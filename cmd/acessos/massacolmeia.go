package main

// A cara da execução em massa (2.9.2): o cartão da execução no alto, com a
// COLMEIA — um favo por máquina, na cor do estado, pulsando enquanto roda
// —, e o mesmo favo pequeno no começo de cada linha. É o favo do
// "conectando" das abas levado para o lote: dá para ver 200 caixas de
// relance, o que a lista sozinha não deixa.

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"time"

	"acessos-go/internal/massa/model"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
)

// favoPath é um hexágono de ponta para cima, centro (x, y), raio r.
func favoPath(gtx layout.Context, x, y, r float32) clip.PathSpec {
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

// classeEstado agrupa os estados do runner nas quatro famílias que a tela
// distingue (e que o cartão conta).
type classeEstado int

const (
	estAguardando classeEstado = iota
	estRodando
	estOK
	estFalha
	estSemConexao
	estPulado
)

func classeDe(s model.StatusExec) classeEstado {
	switch s {
	case model.StatusConectando, model.StatusExecutando:
		return estRodando
	case model.StatusOK:
		return estOK
	case model.StatusErro, model.StatusTimeout:
		return estFalha
	case model.StatusFalhaConexao:
		return estSemConexao
	case model.StatusPulado:
		return estPulado
	}
	return estAguardando
}

func corClasse(c classeEstado) color.NRGBA {
	switch c {
	case estRodando:
		return tema.Azul
	case estOK:
		return tema.Verde
	case estFalha:
		return tema.ErroFg
	case estSemConexao:
		return tema.AtencaoFg
	}
	return tema.Fraco
}

// desenharFavo pinta UM favo do estado c em (x, y). agora pulsa quem roda.
func desenharFavo(gtx layout.Context, x, y, r float32, c classeEstado, agora float64) {
	cor := corClasse(c)
	switch c {
	case estAguardando:
		paint.FillShape(gtx.Ops, tinta(tema.Texto, 0.06), clip.Outline{Path: favoPath(gtx, x, y, r)}.Op())
		paint.FillShape(gtx.Ops, tinta(tema.Texto, 0.22), clip.Stroke{Path: favoPath(gtx, x, y, r), Width: float32(gtx.Dp(1))}.Op())
		return
	case estPulado:
		paint.FillShape(gtx.Ops, tinta(tema.Texto, 0.10), clip.Outline{Path: favoPath(gtx, x, y, r)}.Op())
		b := r * 0.38
		var p clip.Path
		p.Begin(gtx.Ops)
		p.MoveTo(f32.Pt(x-b, y))
		p.LineTo(f32.Pt(x+b, y))
		paint.FillShape(gtx.Ops, tema.Fraco, clip.Stroke{Path: p.End(), Width: float32(gtx.Dp(1.6))}.Op())
		return
	case estRodando:
		// O pulso do favo do "conectando": acende e apaga suave, 1,4s.
		k := 0.5 + 0.5*math.Sin(agora*2*math.Pi/1.4)
		k = suave(k)
		paint.FillShape(gtx.Ops, comAlfa(cor, 0.18+0.5*k), clip.Outline{Path: favoPath(gtx, x, y, r*float32(0.86+0.14*k))}.Op())
		paint.FillShape(gtx.Ops, cor, clip.Stroke{Path: favoPath(gtx, x, y, r), Width: float32(gtx.Dp(1.2))}.Op())
		return
	}
	paint.FillShape(gtx.Ops, comAlfa(cor, 0.88), clip.Outline{Path: favoPath(gtx, x, y, r)}.Op())
	branco := hex(0xffffff)
	if !tema.Escuro && c == estSemConexao {
		branco = hex(0xffffff)
	}
	var p clip.Path
	p.Begin(gtx.Ops)
	b := r * 0.36
	if c == estOK {
		p.MoveTo(f32.Pt(x-b, y+b*0.05))
		p.LineTo(f32.Pt(x-b*0.25, y+b*0.75))
		p.LineTo(f32.Pt(x+b*1.05, y-b*0.7))
	} else {
		p.MoveTo(f32.Pt(x-b, y-b))
		p.LineTo(f32.Pt(x+b, y+b))
		p.MoveTo(f32.Pt(x+b, y-b))
		p.LineTo(f32.Pt(x-b, y+b))
	}
	paint.FillShape(gtx.Ops, branco, clip.Stroke{Path: p.End(), Width: float32(gtx.Dp(1.8))}.Op())
}

// seloEstado é o favo pequeno do começo da linha.
func seloEstado(gtx layout.Context, s model.StatusExec) layout.Dimensions {
	lado := gtx.Dp(22)
	c := classeDe(s)
	desenharFavo(gtx, float32(lado)/2, float32(lado)/2, float32(lado)/2-1, c, float64(gtx.Now.UnixMilli())/1000)
	if c == estRodando {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(40 * time.Millisecond)})
	}
	return layout.Dimensions{Size: image.Pt(lado, lado)}
}

// contagem do lote, na ordem do cartão.
type contagemLote struct{ ok, falha, semConexao, aguardando, rodando, pulado int }

func (t *massaTab) contar() contagemLote {
	t.mu.Lock()
	defer t.mu.Unlock()
	var c contagemLote
	for _, cx := range t.cxs {
		switch classeDe(t.estados[cx.Nome].status) {
		case estOK:
			c.ok++
		case estFalha:
			c.falha++
		case estSemConexao:
			c.semConexao++
		case estRodando:
			c.rodando++
		case estPulado:
			c.pulado++
		default:
			c.aguardando++
		}
	}
	return c
}

// cartaoExecucao: o que roda, onde está e o lote inteiro em favos.
func (t *massaTab) cartaoExecucao(gtx layout.Context) layout.Dimensions {
	th := t.th
	n := len(t.cxs)
	c := t.contar()
	t.mu.Lock()
	rodando, inicio, fim := t.rodando, t.inicio, t.fim
	filaExec := t.filaExecutada
	t.mu.Unlock()
	pausa := t.pausaAtual()

	ritmo := "1 POR VEZ"
	if t.paralelo {
		ritmo = "EM PARALELO"
	}
	sobre := fmt.Sprintf("EXECUÇÃO EM MASSA · %d %s · %s", n, plural(n, "MÁQUINA", "MÁQUINAS"), ritmo)
	titulo := "Escolha um snippet ou escreva um comando"
	fila := filaExec
	if len(fila) == 0 {
		fila = t.fila
	}
	if len(fila) > 0 {
		titulo = fila[0].rotulo
		if titulo == "digitado" {
			titulo = primeiraLinha(fila[0].texto)
		}
		if len(fila) > 1 {
			titulo += fmt.Sprintf(" e mais %d", len(fila)-1)
		}
	}
	var linha string
	feitos := c.ok + c.falha + c.semConexao + c.pulado
	switch {
	case pausa != nil && pausa.tipo == pausaCanario:
		linha = fmt.Sprintf("Testado em 3: confira antes de seguir com as outras %d", c.aguardando)
	case pausa != nil:
		linha = "Parado numa falha: escolha como seguir"
	case rodando:
		linha = fmt.Sprintf("Rodando · %d de %d", feitos, n)
	case !fim.IsZero() && feitos > 0:
		linha = fmt.Sprintf("Concluída em %s · %d ok", duracao(fim.Sub(inicio)), c.ok)
		if c.falha > 0 {
			linha += fmt.Sprintf(" · %d com falha", c.falha)
		}
		if c.semConexao > 0 {
			linha += fmt.Sprintf(" · %d sem conexão", c.semConexao)
		}
	default:
		linha = fmt.Sprintf("Pronto para rodar em %d %s, primeiro em 3 de teste", n, plural(n, "máquina", "máquinas"))
	}

	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			// O mesmo cartão do Painel: vidro, sombra e a luz do acento
			// entrando pela direita (a malha de lá não vem junto: aqui
			// quem faz o papel dos favos é a colmeia).
			sz := gtx.Constraints.Min
			const raio = 16
			sombra(gtx, sz, raio)
			superficieVidro(gtx, sz, tema.CardTinta, transparente, raio)
			rr := clip.UniformRRect(image.Rectangle{Max: sz}, gtx.Dp(raio))
			molde := rr.Push(gtx.Ops)
			brilho := 0.15
			if tema.Escuro {
				brilho = 0.16
			}
			gradCSS(gtx, sz, 270, comAlfa(tema.AzulCheio, brilho), 0, transparente, 0.65)
			molde.Pop()
			paint.FillShape(gtx.Ops, tema.CardBorda, clip.Stroke{Path: rr.Path(gtx.Ops), Width: 1}.Op())
			return layout.Dimensions{Size: sz}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Inset{Top: 16, Bottom: 16, Left: 20, Right: 16}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
									layout.Rigid(negrito(txt(th, fonteMono, unit.Sp(10.5), sobre, tema.Fraco)).Layout),
									layout.Rigid(layout.Spacer{Height: 5}.Layout),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										l := negrito(txt(th, fonteTitulo, unit.Sp(19), titulo, tema.Texto))
										l.MaxLines = 1
										return l.Layout(gtx)
									}),
									layout.Rigid(layout.Spacer{Height: 4}.Layout),
									layout.Rigid(txt(th, fonteSans, unit.Sp(12.5), linha, tema.Sec).Layout),
								)
							}),
							layout.Rigid(layout.Spacer{Width: 12}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return t.numerosLote(gtx, c)
							}),
						)
					}),
					layout.Rigid(layout.Spacer{Height: 14}.Layout),
					layout.Rigid(t.colmeia),
				)
			})
		}),
	)
}

// numerosLote: os mesmos quadros de número do cartão do Painel.
func (t *massaTab) numerosLote(gtx layout.Context, c contagemLote) layout.Dimensions {
	th := t.th
	caixa := func(n int, rot string, cor color.NRGBA) layout.FlexChild {
		return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Dp(74)
				rec := op.Record(gtx.Ops)
				d := layout.Inset{Top: 8, Bottom: 8, Left: 12, Right: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(negrito(txt(th, fonteTitulo, unit.Sp(20), fmt.Sprint(n), tema.Texto)).Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									lado := gtx.Dp(7)
									paint.FillShape(gtx.Ops, cor, clip.UniformRRect(image.Rect(0, 0, lado, lado), gtx.Dp(2)).Op(gtx.Ops))
									return layout.Dimensions{Size: image.Pt(lado, lado)}
								}),
								layout.Rigid(layout.Spacer{Width: 5}.Layout),
								layout.Rigid(rotuloLinha(th, fonteMono, unit.Sp(9.5), rot, tema.Sec)),
							)
						}),
					)
				})
				desenho := rec.Stop()
				superficie(gtx, d.Size, tinta(tema.Texto, 0.04), tema.CardBorda, 10)
				desenho.Add(gtx.Ops)
				return d
			})
		})
	}
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		caixa(c.ok, "OK", tema.Verde),
		caixa(c.falha, "FALHA", tema.ErroFg),
		caixa(c.semConexao, "SEM CONEXÃO", tema.AtencaoFg),
		caixa(c.aguardando+c.rodando, "FALTAM", tema.Azul),
	)
}

// colmeia: um favo por máquina, em fileiras encaixadas como num favo de
// mel. Passar o mouse diz quem é; clicar abre a saída dela.
func (t *massaTab) colmeia(gtx layout.Context) layout.Dimensions {
	n := len(t.cxs)
	if n == 0 {
		return layout.Dimensions{}
	}
	R := float32(gtx.Dp(11))
	folga := float32(gtx.Dp(3))
	passoX := R*float32(math.Sqrt(3)) + folga
	passoY := R*1.5 + folga*0.87
	larg := float32(gtx.Constraints.Max.X)
	porLinha := max(1, int((larg-passoX/2)/passoX))
	linhas := (n + porLinha - 1) / porLinha
	alt := int(float32(linhas-1)*passoY + 2*R + 2)
	for len(t.btnFavo) < n {
		t.btnFavo = append(t.btnFavo, widget.Clickable{})
	}
	agora := float64(gtx.Now.UnixMilli()) / 1000
	anima := false
	for i, cx := range t.cxs {
		t.mu.Lock()
		e := *t.estados[cx.Nome]
		t.mu.Unlock()
		lin, col := i/porLinha, i%porLinha
		x := R + float32(col)*passoX
		if lin%2 == 1 {
			x += passoX / 2
		}
		y := R + 1 + float32(lin)*passoY
		c := classeDe(e.status)
		if c == estRodando {
			anima = true
		}
		desenharFavo(gtx, x, y, R, c, agora)
		// área de clique do favo (um quadrado em volta dele basta)
		btn := &t.btnFavo[i]
		if btn.Clicked(gtx) {
			abrirDialogo(&dlgSaida{titulo: cx.Nome + " · " + cx.Host, texto: textoCompleto(e, t.filaExecutada)})
		}
		area := image.Rect(int(x-R), int(y-R), int(x+R), int(y+R))
		st := op.Offset(area.Min).Push(gtx.Ops)
		cgtx := gtx
		cgtx.Constraints = layout.Exact(area.Size())
		btn.Layout(cgtx, func(gtx layout.Context) layout.Dimensions {
			if btn.Hovered() {
				pedirDica("massa-favo-"+cx.Nome, fmt.Sprintf("%s · %s · %s", cx.Nome, cx.Host, e.status))
			}
			return layout.Dimensions{Size: gtx.Constraints.Max}
		})
		st.Pop()
	}
	if anima {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(40 * time.Millisecond)})
	}
	return layout.Dimensions{Size: image.Pt(int(larg), alt)}
}
