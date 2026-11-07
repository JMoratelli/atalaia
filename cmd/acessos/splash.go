package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// splash é o cartão central mostrado no lugar da tela remota (RDP/VNC),
// do terminal (SSH) ou do painel remoto (SFTP) enquanto não há sessão
// viva — a mesma peça para as quatro abas de protocolo, para "atender
// todas as telas com os passos de conexão" em vez de cada uma inventar o
// próprio jeito de dizer "conectando".
//
// Inspirado no Termius: uma lista de passos que vai se marcando conforme
// a conexão avança, e um botão de ação que aparece assim que a espera já
// foi longa demais ou a tentativa terminou em erro — em vez de deixar
// girando pra sempre sem dar ao operador nenhum jeito de agir.
type splash struct {
	// janela é lida a CADA uso, não guardada: a aba pode ter sido
	// destacada para outra janela depois que o splash nasceu, e um
	// ponteiro fixo aqui pedia quadro para a janela antiga — o cartão de
	// reconexão congelava na primeira imagem enquanto a janela errada era
	// acordada à toa.
	janela func() *app.Window
	// disparo é o mesmo invalidador de invalidar.go: chamar w.Invalidate()
	// cru direto da goroutine do ticker (ver animar) caía na mesma guarda
	// mayInvalidate do Gio documentada ali — o pedido podia morrer calado
	// bem no instante em que outro quadro qualquer estivesse de passagem,
	// e sem mais nada para gerar quadro (a aba parada, "conectando") o
	// halo ficava com a primeira aparência e nunca mais se movia.
	disparo invalidador

	mu        sync.Mutex
	visivel   bool // false só enquanto a sessão está viva (ver concluir)
	passos    []string
	atual     int       // índice do passo em andamento (ou último alcançado)
	desde     time.Time // quando o passo ATUAL começou
	erro      string    // por que a última tentativa parou; "" = nenhuma
	erroEm    time.Time // quando o erro apareceu (o favo vira "×" a partir daí)
	proximaEm time.Time // reconexão automática agendada; zero = nenhuma

	tickando atomic.Bool
}

// Todos os métodos abaixo (e desenharSplash) toleram s == nil como "sem
// splash nenhum, não desenha nada": testes que montam uma aba com um
// literal de struct em vez do construtor (ver sftpDeTeste em
// sftpsel_test.go) nunca chamam novoSplash.

func novoSplash(janela func() *app.Window) *splash { return &splash{janela: janela} }

// iniciar reseta os PASSOS para o começo de uma tentativa nova e garante
// que o cartão esteja visível. É seguro chamar de novo a cada tentativa,
// inclusive a primeira.
func (s *splash) iniciar(passos []string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.visivel = true
	s.passos = passos
	s.atual = 0
	s.desde = time.Now()
	s.erro = ""
	s.proximaEm = time.Time{}
	s.mu.Unlock()
	s.disparo.disparar(s.janela())
	s.animar()
}

// avancar marca o passo i como o atual. i pode pular o do meio: em RDP e
// VNC o caminho de sucesso não observa nada entre "pedi a conexão" e "o
// servidor confirmou".
func (s *splash) avancar(i int) {
	if s == nil {
		return
	}
	s.mu.Lock()
	if i > s.atual {
		s.atual = i
		s.desde = time.Now()
	}
	s.mu.Unlock()
	s.disparo.disparar(s.janela())
}

// concluir esconde o cartão: a sessão está viva. Barato de chamar de
// novo (RDP/VNC chamam isto a cada quadro recebido, não só no primeiro).
func (s *splash) concluir() {
	if s == nil {
		return
	}
	s.mu.Lock()
	jaEscondido := !s.visivel
	s.visivel = false
	s.atual = len(s.passos)
	s.mu.Unlock()
	if !jaEscondido {
		s.disparo.disparar(s.janela())
	}
}

// setErro registra por que a última tentativa parou — inclusive uma
// sessão que ESTAVA viva e caiu — e torna o cartão visível de novo.
//
// O "de novo" importa: concluir() já pode ter escondido o cartão (a
// sessão chegou a conectar). Sem reafirmar visivel aqui, uma sessão que
// caiu depois de conectada ficava com o cartão escondido bem no momento
// em que mais precisava aparecer — durante a espera de reconexão, que é
// justamente quando falhou()/aguardar() são chamados.
func (s *splash) setErro(msg string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.visivel = true
	if s.erro == "" && msg != "" {
		s.erroEm = time.Now()
	}
	s.erro = msg
	s.mu.Unlock()
	s.disparo.disparar(s.janela())
	s.animar()
}

// aguardar anota quando a reconexão automática vai acontecer, para o
// cartão mostrar a contagem regressiva — só faz sentido chamar isto no
// caminho de backoff automático; a espera por um clique manual não tem
// prazo, então não chama aguardar nenhum.
func (s *splash) aguardar(proximaEm time.Time) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.visivel = true
	s.proximaEm = proximaEm
	s.mu.Unlock()
	s.disparo.disparar(s.janela())
	s.animar()
}

type splashFoto struct {
	passos    []string
	atual     int
	desde     time.Time
	erro      string
	erroEm    time.Time
	proximaEm time.Time
	visivel   bool
}

func (s *splash) foto() splashFoto {
	if s == nil {
		return splashFoto{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return splashFoto{
		passos: s.passos, atual: s.atual, desde: s.desde,
		erro: s.erro, erroEm: s.erroEm, proximaEm: s.proximaEm,
		visivel: s.visivel,
	}
}

// animar mantém o quadro vivo enquanto o cartão precisar de algo que só
// o tempo muda: o halo pulsando, o botão que aparece depois de
// travadoApos, e a contagem regressiva de reconexão. Sem isto o cartão
// ficava parado entre um evento de rede e o próximo, que podem levar
// vários segundos — na prática, o halo desenhava só a primeira aparência
// e congelava ali. No máximo uma goroutine de cada vez.
func (s *splash) animar() {
	if !s.tickando.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer s.tickando.Store(false)
		// 30ms (~33 quadros/s): o favo pulsa em ciclos de 3s, e a 200ms
		// o pulso andava aos solavancos. Só enquanto o cartão está à vista.
		tk := time.NewTicker(30 * time.Millisecond)
		defer tk.Stop()
		for range tk.C {
			if !s.foto().visivel {
				return
			}
			s.disparo.disparar(s.janela())
		}
	}()
}

// origemAnimacao é o zero da fase de qualquer animação do splash — usar
// time.Since(origemAnimacao) em vez de time.Now() cru mantém o número
// pequeno (segundos desde que o app abriu, não desde 1970), sem motivo
// nenhum pra precisão em nanossegundo de um valor gigante entrar na
// conta de um seno.
var origemAnimacao = time.Now()

// travadoApos é quanto tempo um passo fica "em andamento" antes do
// cartão oferecer o botão de agir — mesmo sem erro nenhum ainda. Rede de
// PDV lenta existe; oito segundos dá tempo sem deixar a impressão de
// travado.
const travadoApos = 8 * time.Second

// desenharSplash desenha o cartão: emblema do protocolo com um halo
// pulsando, a trilha de progresso (bolinhas, não uma lista de texto) e
// só a legenda do passo ATUAL — o resto do histórico já está dito pela
// trilha, escrever tudo de novo em texto seria repetir a mesma
// informação duas vezes. Quando a espera já é longa ou uma tentativa
// termina, aparece o botão de agir agora.
func desenharSplash(gtx layout.Context, th *material.Theme, s *splash,
	titulo string, ic *widget.Icon, corAccent color.NRGBA,
	btn *widget.Clickable, agir func(),
) layout.Dimensions {
	f := s.foto()
	if !f.visivel {
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
	if btn.Clicked(gtx) && agir != nil {
		agir()
	}

	travado := f.erro != "" || time.Since(f.desde) > travadoApos
	agora := time.Since(origemAnimacao).Seconds()

	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		larg := gtx.Dp(240)
		if maxLarg := gtx.Constraints.Max.X - gtx.Dp(24); larg > maxLarg {
			larg = maxLarg
		}
		gtx.Constraints.Min.X = larg
		gtx.Constraints.Max.X = larg
		gtx.Constraints.Min.Y = 0

		macro := op.Record(gtx.Ops)
		dims := layout.UniformInset(20).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			var filhos []layout.FlexChild
			filhos = append(filhos,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						desdeErro := -1.0
						if f.erro != "" {
							desdeErro = time.Since(f.erroEm).Seconds()
						}
						return favoConexao(gtx, ic, corAccent, agora, desdeErro)
					})
				}),
				espaco(14),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, negrito(txt(th, fonteMono, spCardHost, titulo, tema.Texto)).Layout)
				}),
			)

			if len(f.passos) > 1 {
				filhos = append(filhos, espaco(16),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return trilhaPassos(gtx, len(f.passos), f.atual, corAccent)
					}))
			}

			texto, cor := statusDoSplash(f)
			if texto != "" {
				filhos = append(filhos, espaco(12),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Center.Layout(gtx, rotulo(th, fonteMono, spCardMeta, texto, cor))
					}))
			}
			if !f.proximaEm.IsZero() {
				restante := time.Until(f.proximaEm).Round(time.Second)
				if restante < 0 {
					restante = 0
				}
				filhos = append(filhos, espaco(4),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Center.Layout(gtx, rotulo(th, fonteMono, spCardMeta,
							fmt.Sprintf("nova tentativa em %s", restante), tema.AtencaoFg))
					}))
			}
			if travado {
				rot := "Conectar agora"
				filhos = append(filhos, espaco(16),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return botaoSessao(gtx, th, btn, rot)
						})
					}))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, filhos...)
		})
		conteudo := macro.Stop()
		sombraElev(gtx, dims.Size, 14, &elevDialogo)
		superficieCartao(gtx, dims.Size, 14)
		conteudo.Add(gtx.Ops)
		return dims
	})
}

// statusDoSplash resume o estado atual numa frase só: o erro, quando
// houver, vence — é a informação mais importante no instante em que
// existe. Sem erro, é só o nome do passo em andamento.
func statusDoSplash(f splashFoto) (string, color.NRGBA) {
	if f.erro != "" {
		return f.erro, tema.ErroFg
	}
	if f.atual >= 0 && f.atual < len(f.passos) {
		return f.passos[f.atual] + "…", tema.Sec
	}
	return "", tema.Sec
}

// suave é a curva smoothstep: começa e termina devagar, sem degrau.
func suave(k float64) float64 { return k * k * (3 - 2*k) }

// favoConexao é o favo de carregamento da central do ChamadoJá, com o
// ícone da conexão no hexágono do meio: seis hexágonos em volta acendem um
// depois do outro (0,22s de diferença, em roda) num ciclo de 3s — aparecem
// crescendo, ficam, e somem encolhendo um pouco —, na cor do protocolo.
// Vale para as quatro abas (tela, shell, rdp, arquivos): todas desenham o
// "conectando" por desenharSplash, cada uma com o próprio ícone e cor.
// Com erro (desdeErro >= 0, segundos desde que ele apareceu) o favo para
// e cada célula vira um "×" vermelho, desenhado do centro para fora em
// cascata rápida; o hexágono do meio também fica vermelho.
// Substitui o círculo com halo de antes, para o "conectando" falar a mesma
// língua da malha de hexágonos do Painel.
func favoConexao(gtx layout.Context, ic *widget.Icon, cor color.NRGBA, agora, desdeErro float64) layout.Dimensions {
	R := float32(gtx.Dp(17)) // raio de cada célula (ponta para cima)
	passo := R * float32(math.Sqrt(3))
	lado := int(passo*3 + 2)
	alt := int(R*5 + 2)
	cx, cy := float32(lado)/2, float32(alt)/2

	hexa := func(x, y, r float32) clip.PathSpec {
		var p clip.Path
		p.Begin(gtx.Ops)
		for k := 0; k < 6; k++ {
			a := math.Pi/3*float64(k) - math.Pi/2
			pt := f32.Pt(x+r*float32(math.Cos(a)), y+r*float32(math.Sin(a)))
			if k == 0 {
				p.MoveTo(pt)
			} else {
				p.LineTo(pt)
			}
		}
		p.Close()
		return p.End()
	}

	r := R * 0.9 // folga entre as células
	for i := 0; i < 6; i++ {
		// vizinhas a 30° + 60°·i, no sentido do relógio a partir de cima
		a := math.Pi/6 + math.Pi/3*float64(i) - math.Pi/2
		x := cx + passo*float32(math.Cos(a))
		y := cy + passo*float32(math.Sin(a))
		if desdeErro >= 0 {
			// a célula avermelha e o "×" se desenha do centro para fora
			k := suave(min(max((desdeErro-0.06*float64(i))/0.4, 0), 1))
			paint.FillShape(gtx.Ops, misturarCor(tinta(tema.Texto, 0.07), comAlfa(tema.ErroFg, 0.12), float32(k)),
				clip.Outline{Path: hexa(x, y, r)}.Op())
			if k > 0 {
				b := r * 0.42 * float32(k)
				var p clip.Path
				p.Begin(gtx.Ops)
				p.MoveTo(f32.Pt(x-b, y-b))
				p.LineTo(f32.Pt(x+b, y+b))
				p.MoveTo(f32.Pt(x+b, y-b))
				p.LineTo(f32.Pt(x-b, y+b))
				paint.FillShape(gtx.Ops, comAlfa(tema.ErroFg, 0.9*k),
					clip.Stroke{Path: p.End(), Width: float32(gtx.Dp(2.2))}.Op())
			}
			continue
		}
		paint.FillShape(gtx.Ops, tinta(tema.Texto, 0.07), clip.Outline{Path: hexa(x, y, r)}.Op())
		t := math.Mod(agora-0.22*float64(i), 3) / 3
		if t < 0 {
			t += 1
		}
		// Curvas suaves (smoothstep) nas duas pontas, e entrada e saída
		// mais longas que as do favo da central: a célula acende e apaga
		// sem degrau, e a cascata (0,22s entre vizinhas) corre em roda.
		var o, esc float64
		switch {
		case t < 0.28: // aparece crescendo
			k := suave(t / 0.28)
			o, esc = k, 0.55+0.45*k
		case t < 0.58:
			o, esc = 1, 1
		case t < 0.92: // some encolhendo um pouco
			k := suave((t - 0.58) / 0.34)
			o, esc = 1-k, 1-0.12*k
		default:
			o, esc = 0, 0.88
		}
		if o > 0 {
			paint.FillShape(gtx.Ops, comAlfa(cor, 0.55*o), clip.Outline{Path: hexa(x, y, r*float32(esc))}.Op())
		}
	}
	// o do meio: o ícone da conexão, fixo (vermelho com erro)
	if desdeErro >= 0 {
		cor = misturarCor(cor, tema.ErroFg, float32(suave(min(desdeErro/0.4, 1))))
	}
	fundo := comAlfa(cor, 0.16)
	paint.FillShape(gtx.Ops, fundo, clip.Outline{Path: hexa(cx, cy, r)}.Op())
	paint.FillShape(gtx.Ops, cor, clip.Stroke{Path: hexa(cx, cy, r), Width: float32(gtx.Dp(1.2))}.Op())
	tam := gtx.Dp(20)
	off := image.Pt(int(cx)-tam/2, int(cy)-tam/2)
	st := op.Offset(off).Push(gtx.Ops)
	icone(gtx, ic, cor, 20)
	st.Pop()
	return layout.Dimensions{Size: image.Pt(lado, alt)}
}

// trilhaPassos é a barra de progresso com uma bolinha por passo — feito
// (preenchida), atual (preenchida, maior) e pendente (só o contorno) —
// no lugar da lista de nomes em texto: o nome de cada passo já concluído
// não importa mais depois que ele passou, só QUANTO falta.
func trilhaPassos(gtx layout.Context, n, atual int, corAtiva color.NRGBA) layout.Dimensions {
	larg := gtx.Constraints.Max.X
	altura := gtx.Dp(16)
	raioCheia := gtx.Dp(5)
	raioVazia := gtx.Dp(4)
	espessura := gtx.Dp(2)
	y := altura / 2

	bolinha := func(cx, raio int, cor color.NRGBA, contornoSo bool) {
		r := image.Rect(cx-raio, y-raio, cx+raio, y+raio)
		if contornoSo {
			paint.FillShape(gtx.Ops, tema.Cartao, clip.Ellipse(r).Op(gtx.Ops))
			paint.FillShape(gtx.Ops, cor, clip.Stroke{Path: clip.Ellipse(r).Path(gtx.Ops), Width: float32(gtx.Dp(1.3))}.Op())
			return
		}
		paint.FillShape(gtx.Ops, cor, clip.Ellipse(r).Op(gtx.Ops))
	}

	if n <= 1 {
		bolinha(larg/2, raioCheia, corAtiva, false)
		return layout.Dimensions{Size: image.Pt(larg, altura)}
	}

	passo := float64(larg-2*raioCheia) / float64(n-1)
	xDe := func(i int) int { return raioCheia + int(passo*float64(i)+0.5) }

	linha := func(x0, x1 int, cor color.NRGBA) {
		if x1 <= x0 {
			return
		}
		paint.FillShape(gtx.Ops, cor,
			clip.Rect{Min: image.Pt(x0, y-espessura/2), Max: image.Pt(x1, y+espessura/2)}.Op())
	}
	fimFeito := xDe(atual)
	if atual >= n-1 {
		fimFeito = larg - raioCheia
	}
	linha(raioCheia, larg-raioCheia, tema.Borda)
	linha(raioCheia, fimFeito, corAtiva)

	for i := 0; i < n; i++ {
		x := xDe(i)
		switch {
		case i < atual:
			bolinha(x, raioVazia, corAtiva, false)
		case i == atual:
			bolinha(x, raioCheia, corAtiva, false)
		default:
			bolinha(x, raioVazia, tema.Borda, true)
		}
	}
	return layout.Dimensions{Size: image.Pt(larg, altura)}
}
