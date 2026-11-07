package main

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"os/user"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"acessos-go/internal/conexoes"
	"acessos-go/internal/vida"

	"gio.tools/icons"
	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

const (
	// O card deixou de ser quadrado na 2.9: mais baixo, sem o vão no
	// meio, e esticado para a grade fechar a linha (ver grade).
	cardLargura = unit.Dp(178) // largura MÍNIMA
	cardAltura  = unit.Dp(144)
	cardRaio    = unit.Dp(14)
	icoRaio     = unit.Dp(8)  // segmento do trilho de protocolos
	icoAltura   = unit.Dp(30) // idem
)

// acaoAbrir é o que o painel pede quando alguém clica num ícone de
// protocolo do card: main.go abre a aba correspondente.
type acaoAbrir func(cx conexoes.Conexao, p conexoes.Protocolo)

// protocolo reúne o que muda entre VNC/SSH/RDP/SFTP num lugar só — ícone,
// cor e cor de fundo tênue. O MESMO glifo e a MESMA cor aparecem no card e
// na aba: um protocolo tem uma cor só em toda a interface (gtk.md §6).
type protoEstilo struct {
	p      conexoes.Protocolo
	ic     *widget.Icon
	cor    func() color.NRGBA
	fundo  func() color.NRGBA
	rotulo string
}

var protocolos = []protoEstilo{
	{conexoes.VNC, icons.HardwareDesktopWindows, func() color.NRGBA { return tema.Azul }, func() color.NRGBA { return tema.AzulFraco }, "tela"},
	{conexoes.SSH, icons.ActionCode, func() color.NRGBA { return tema.Verde }, func() color.NRGBA { return tema.VerdeFraco }, "shell"},
	{conexoes.RDP, icons.HardwareComputer, func() color.NRGBA { return tema.Roxo }, func() color.NRGBA { return tema.RoxoFraco }, "rdp"},
	// Glifo de TRAÇO, como os outros três: os cheios (a pasta sólida)
	// pintam o quadrado inteiro e encostam na borda da caixinha, tanto no
	// card quanto no selo da aba.
	{conexoes.SFTP, icons.ActionDescription, func() color.NRGBA { return tema.AtencaoFg }, func() color.NRGBA { return tema.AtencaoBg }, "sftp"},
}

func estiloDe(p conexoes.Protocolo) protoEstilo {
	for _, e := range protocolos {
		if e.p == p {
			return e
		}
	}
	return protocolos[0]
}

// dashTab é a aba FIXA "Painel": hero, filtro, árvore de grupos e os cards
// de vidro com um botão por protocolo.
type dashTab struct {
	th      *material.Theme
	arq     *conexoes.Arquivo
	arvore  []*conexoes.Grupo
	abrir   acaoAbrir
	caminho string
	abertas func() int

	// DOIS campos de busca, um na lateral e um no painel, com o texto
	// espelhado a cada frame: o Gio não deixa desenhar o MESMO
	// widget.Editor duas vezes no mesmo frame (o estado de foco/seleção é
	// um só), e a busca foi pedida nos dois lugares.
	filtro        widget.Editor
	filtroLateral widget.Editor
	ultimoTermo   string
	lista         widget.List
	expandir      map[string]bool
	cliques       map[string]*widget.Clickable
	btnRecolher   widget.Clickable
	btnExpandir   widget.Clickable
	btnNova       widget.Clickable
	btnMassa      widget.Clickable
	btnLimparSel  widget.Clickable
	btnExcluirSel widget.Clickable
	btnSobre      widget.Clickable
	btnRecarregar widget.Clickable

	// seleção para execução em massa. A chave é grupo|nome, a mesma dos
	// Clickables — o card é identificado pelo par, porque nome se repete
	// entre lojas.
	selecao        map[string]conexoes.Conexao
	aoExecMassa    func([]conexoes.Conexao)
	aoExcluirMassa func([]conexoes.Conexao)
	recarregar     func()
	aoInvalidar    func()

	// menu de contexto: onde o ponteiro está (para ancorar o menu) e a
	// tag de área do botão direito de cada card.
	ultimaPos      image.Point
	sobCursor      *conexoes.Conexao  // card sob o ponteiro (hover)
	protoSobCursor conexoes.Protocolo // ícone de protocolo sob o ponteiro
	grupoSobCursor string             // cabeçalho de grupo sob o ponteiro
	tagPainel      *int

	// Sonda de vida: trilho verde = respondeu, vermelho = não, cinza =
	// ainda não checado. Só os cards que estão SENDO DESENHADOS entram na
	// fila — sondar 274 máquinas de uma vez inunda a rede e não serve
	// para nada, porque o operador está olhando uma loja de cada vez.
	vidaMu      sync.Mutex
	vida        map[string]vida.Estado
	naFila      map[string]bool
	ultimoAviso time.Time
	fila        chan conexoes.Conexao
	travaAbrir  int // quadros em que clique não abre (gesto do botão direito)
	aoMenuCard  func(cx conexoes.Conexao, pos image.Point)
	aoNova      func(grupo string)
	// buscaNaLateral diz se o filtro está sendo desenhado na lateral
	// (aberta): aí a barra do Painel não desenha o dela.
	buscaNaLateral func() bool
	aoEfemera      func(destino string)
	// aoRascunho abre um destino NÃO cadastrado (o card temporário da
	// busca): quem trata pergunta as credenciais antes de conectar.
	aoRascunho func(cx conexoes.Conexao, p conexoes.Protocolo)
}

func newDashTab(th *material.Theme, arq *conexoes.Arquivo, caminho string, abrir acaoAbrir, abertas func() int) *dashTab {
	d := &dashTab{
		th:        th,
		arq:       arq,
		arvore:    arq.Arvore(),
		abrir:     abrir,
		caminho:   caminho,
		abertas:   abertas,
		expandir:  map[string]bool{},
		cliques:   map[string]*widget.Clickable{},
		selecao:   map[string]conexoes.Conexao{},
		tagPainel: new(int),
		vida:      map[string]vida.Estado{},
		naFila:    map[string]bool{},
		fila:      make(chan conexoes.Conexao, 512),
	}
	d.filtro.SingleLine = true
	d.filtro.Submit = true
	d.filtroLateral.SingleLine = true
	d.filtroLateral.Submit = true
	d.filtro.SetText(filtroInicial)
	d.filtroLateral.SetText(filtroInicial)
	d.ultimoTermo = filtroInicial
	d.lista.Axis = layout.Vertical
	// Poucos trabalhadores e prazo curto: isto é diagnóstico de fundo e
	// não pode competir com a sessão remota que o operador está usando —
	// nem com a CPU do próprio desenho.
	for i := 0; i < 3; i++ {
		go d.sondador()
	}
	return d
}

func (d *dashTab) sondador() {
	for cx := range d.fila {
		porta := 0
		if cx.Tem(conexoes.VNC) {
			porta = cx.VNC.Porta
		} else if cx.Tem(conexoes.SSH) {
			porta = cx.SSH.Porta
		}
		e := vida.Checar(cx.Host, porta, 400*time.Millisecond)
		d.vidaMu.Lock()
		d.vida[cx.Host] = e
		delete(d.naFila, cx.Host)
		// Um Invalidate por resposta fazia a janela redesenhar dezenas de
		// vezes por segundo com o parque inteiro na tela — e redesenhar
		// 40 cards com sombra não é de graça. Agrupa: no máximo um pedido
		// de quadro a cada 250ms.
		agora := time.Now()
		pedir := agora.Sub(d.ultimoAviso) > 250*time.Millisecond
		if pedir {
			d.ultimoAviso = agora
		}
		d.vidaMu.Unlock()
		if pedir && d.aoInvalidar != nil {
			d.aoInvalidar()
		}
	}
}

// estadoVida devolve o que se sabe do host e enfileira a checagem quando
// ainda não se sabe nada.
// vidaConhecida devolve o estado de vida que já se sabe, SEM pôr a máquina
// na fila de verificação — é o que a lateral usa: quem pergunta é o
// Painel, e a lateral desenhando a árvore não pode disparar checagem de
// rede em cada máquina que aparece nela.
func (d *dashTab) vidaConhecida(cx conexoes.Conexao) vida.Estado {
	d.vidaMu.Lock()
	defer d.vidaMu.Unlock()
	if e, sabido := d.vida[cx.Host]; sabido {
		return e
	}
	return vida.Desconhecido
}

func (d *dashTab) estadoVida(cx conexoes.Conexao) vida.Estado {
	d.vidaMu.Lock()
	e, sabido := d.vida[cx.Host]
	pendente := d.naFila[cx.Host]
	if !sabido && !pendente {
		d.naFila[cx.Host] = true
		pendente = true
		defer func() {
			select {
			case d.fila <- cx:
			default: // fila cheia: fica para o próximo desenho
				d.vidaMu.Lock()
				delete(d.naFila, cx.Host)
				d.vidaMu.Unlock()
			}
		}()
	}
	d.vidaMu.Unlock()
	if !sabido {
		return vida.Desconhecido
	}
	return e
}

// recarregarCom troca o arquivo exibido mantendo o que está expandido —
// recarregar não deveria fechar tudo que o operador abriu.
func (d *dashTab) recarregarCom(arq *conexoes.Arquivo) {
	d.arq = arq
	d.arvore = arq.Arvore()
}

func (d *dashTab) Title() string { return "Painel" }

func (d *dashTab) Selo() (*widget.Icon, color.NRGBA, color.NRGBA) {
	return icons.ActionDashboard, tema.Azul, transparente
}
func (d *dashTab) Pinned() bool                  { return true }
func (d *dashTab) Close()                        {}
func (d *dashTab) HandleKey(_, _ uint32, _ bool) {}

// HandlePointer não é usado pelo Painel: os widgets do Gio ficam POR CIMA
// da área que main.go registra, então esses eventos nunca chegam aqui. A
// posição do ponteiro vem de rastrearPonteiro, com PassOp.
func (d *dashTab) HandlePointer(_ pointer.Event, _ image.Point) {}

// rastrearPonteiro é uma área do tamanho do painel, com PassOp, registrada
// POR CIMA de tudo (daí o defer em Layout). Ela faz duas coisas: guarda
// onde o ponteiro está, para ancorar o menu, e detecta o clique com o
// botão direito.
//
// Por que não uma área por card: no hit-test do Gio, ao encontrar uma área
// SEM pass o percurso salta para o nó PAI, e com isso as áreas irmãs
// registradas antes ficam inalcançáveis. Uma área por card, por baixo dos
// widgets do card, nunca recebia nada — foram duas tentativas. Aqui a
// área é uma só, fica por cima, e QUAL card está sob o cursor sai do
// hover, que o Gio já mantém.
func (d *dashTab) rastrearPonteiro(gtx layout.Context) {
	for {
		ev, ok := gtx.Source.Event(pointer.Filter{
			Target: d.tagPainel,
			Kinds:  pointer.Move | pointer.Press | pointer.Drag,
		})
		if !ok {
			break
		}
		pe, isP := ev.(pointer.Event)
		if !isP {
			continue
		}
		d.ultimaPos = image.Pt(int(pe.Position.X), int(pe.Position.Y))
		if pe.Kind != pointer.Press {
			continue
		}
		switch {
		case pe.Buttons.Contain(pointer.ButtonSecondary):
			// trava o gesto: nenhum clique de abertura vale enquanto o
			// menu está sendo aberto por este mesmo movimento.
			d.travaAbrir = 2
			if d.sobCursor != nil && d.aoMenuCard != nil {
				d.aoMenuCard(*d.sobCursor, d.ultimaPos)
			} else if d.grupoSobCursor != "" {
				d.aoNovaGrupo(d.grupoSobCursor)
			}
		case pe.Buttons.Contain(pointer.ButtonTertiary):
			// Botão do meio abre em segundo plano. Se o cursor está sobre
			// um ÍCONE de protocolo, é aquele protocolo que abre — antes
			// ele caía sempre no padrão do card, então clicar no shell
			// abria a tela.
			if d.sobCursor != nil {
				p, ok := d.protoSobCursor, d.protoSobCursor != ""
				if !ok {
					p, ok = protocoloPadrao(*d.sobCursor)
				}
				if ok {
					d.abrirEm(*d.sobCursor, p, false)
				}
			}
		}
	}
	pass := pointer.PassOp{}.Push(gtx.Ops)
	area := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
	event.Op(gtx.Ops, d.tagPainel)
	area.Pop()
	pass.Pop()
}

// clique devolve (criando na primeira vez) o Clickable daquela chave —
// recriar a cada frame zeraria o clique antes de ele ser lido.
func (d *dashTab) clique(chave string) *widget.Clickable {
	c, ok := d.cliques[chave]
	if !ok {
		c = &widget.Clickable{}
		d.cliques[chave] = c
	}
	return c
}

// sincronizarBusca copia o texto do campo que mudou para o outro, pra os
// dois espelharem o mesmo filtro.
func (d *dashTab) sincronizarBusca() string {
	painel, lateral := d.filtro.Text(), d.filtroLateral.Text()
	switch {
	case painel != d.ultimoTermo:
		d.ultimoTermo = painel
		d.filtroLateral.SetText(painel)
	case lateral != d.ultimoTermo:
		d.ultimoTermo = lateral
		d.filtro.SetText(lateral)
	}
	return strings.ToLower(strings.TrimSpace(d.ultimoTermo))
}

func (d *dashTab) Layout(gtx layout.Context) layout.Dimensions {
	termo := d.sincronizarBusca()
	// quem está sob o cursor é recalculado a cada quadro pelo hover
	d.sobCursor, d.grupoSobCursor, d.protoSobCursor = nil, "", ""
	if d.travaAbrir > 0 {
		d.travaAbrir--
	}
	defer d.rastrearPonteiro(gtx)

	// O trecho solto da malha de hexágonos fica ATRÁS de tudo, preso ao
	// canto inferior esquerdo da folha (não rola com a lista). O de cima
	// mora no cartão de boas-vindas (cabecalho).
	malhaHex(gtx)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(d.cabecalho),
		layout.Rigid(d.barraFiltro),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return d.cardRascunho(gtx, termo)
		}),
		layout.Rigid(d.barraSelecao),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return d.lista.Layout(gtx, len(d.arvore), func(gtx layout.Context, i int) layout.Dimensions {
				return layout.Inset{Left: margemPainel, Right: margemPainel}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return d.grupo(gtx, d.arvore[i], 0, termo)
				})
			})
		}),
	)
}

// margemPainel é o recuo lateral de tudo o que mora na folha do Painel.
const margemPainel = unit.Dp(28)

// spMetaCard é o resumo dos protocolos no card. Não é o spCardMeta (9),
// que outras telas usam para nota de rodapé: no card novo ele subiu.
const spMetaCard = unit.Sp(10.5)

// cabecalho é o cartão de boas-vindas, como o do painel da central do
// ChamadoJá: saudação e resumo à esquerda, os números à direita e a malha
// de hexágonos DENTRO do cartão, recortada nele e acesa à direita. Até a
// 2.8 era um banner escuro; na primeira versão da 2.9, título e números
// soltos na folha, com um vão vazio entre eles.
func (d *dashTab) cabecalho(gtx layout.Context) layout.Dimensions {
	return layout.Inset{Top: 20, Left: margemPainel, Right: margemPainel}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		corpo := op.Record(gtx.Ops)
		dims := layout.Inset{Top: 20, Bottom: 20, Left: 24, Right: 20}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(negrito(txt(d.th, fonteMono, unit.Sp(11), d.sobretitulo(), tema.Fraco)).Layout),
						layout.Rigid(layout.Spacer{Height: 6}.Layout),
						layout.Rigid(negrito(txt(d.th, fonteTitulo, unit.Sp(26), saudacao(gtx.Now), tema.Texto)).Layout),
						layout.Rigid(layout.Spacer{Height: 6}.Layout),
						layout.Rigid(txt(d.th, fonteSans, unit.Sp(13.5), d.resumo(), tema.Sec).Layout),
					)
				}),
				layout.Rigid(layout.Spacer{Width: 24}.Layout),
				layout.Rigid(d.estatisticas),
			)
		})
		desenho := corpo.Stop()

		sz := dims.Size
		const raio = 16
		sombra(gtx, sz, raio)
		superficieVidro(gtx, sz, tema.CardTinta, transparente, raio)
		rr := clip.UniformRRect(image.Rectangle{Max: sz}, gtx.Dp(raio))
		molde := rr.Push(gtx.Ops)
		// a luz do acento entrando pela direita, e a malha por cima dela
		brilho := 0.15
		if tema.Escuro {
			brilho = 0.16
		}
		gradCSS(gtx, sz, 270, comAlfa(tema.AzulCheio, brilho), 0, transparente, 0.65)
		lm := min(gtx.Dp(640), sz.X*62/100)
		malhaCartao.desenharEm(gtx, lm, sz.Y, image.Pt(sz.X-lm, 0))
		molde.Pop()
		paint.FillShape(gtx.Ops, tema.CardBorda, clip.Stroke{Path: rr.Path(gtx.Ops), Width: 1}.Op())
		desenho.Add(gtx.Ops)
		return dims
	})
}

// saudacao: "Bom dia" / "Boa tarde" / "Boa noite", com o primeiro nome de
// quem usa o computador quando o sistema o tem.
func saudacao(agora time.Time) string {
	s := "Boa noite"
	switch h := agora.Hour(); {
	case h >= 5 && h < 12:
		s = "Bom dia"
	case h >= 12 && h < 18:
		s = "Boa tarde"
	}
	if n := primeiroNome(); n != "" {
		s += ", " + n
	}
	return s
}

// primeiroNome é o nome da saudação. Começa pelo login da variável de
// ambiente (instantâneo) e é trocado pelo nome completo quando
// carregarPrimeiroNome termina — numa goroutine, porque user.Current() pode
// consultar o controlador de domínio no Windows (ou o LDAP no Linux) e,
// fora da VPN, travaria o primeiro quadro do Painel por segundos.
var nomeSaudacao atomic.Pointer[string]

func primeiroNome() string {
	if p := nomeSaudacao.Load(); p != nil {
		return *p
	}
	login := os.Getenv("USER")
	if login == "" {
		login = os.Getenv("USERNAME")
	}
	return maiusculaInicial(login)
}

// carregarPrimeiroNome lê o usuário do sistema (o nome completo — o campo
// GECOS no Linux, o nome de exibição no Windows — quando houver, senão o
// login sem domínio) e chama pronto para a janela redesenhar.
func carregarPrimeiroNome(pronto func()) {
	go func() {
		n := lerPrimeiroNome()
		if n == "" {
			return
		}
		nomeSaudacao.Store(&n)
		if pronto != nil {
			pronto()
		}
	}()
}

func lerPrimeiroNome() string {
	u, err := user.Current()
	if err != nil {
		return ""
	}
	nome := strings.TrimSpace(strings.Split(u.Name, ",")[0])
	if campos := strings.Fields(nome); len(campos) > 0 {
		nome = campos[0]
	} else {
		nome = u.Username
		if i := strings.LastIndexAny(nome, `\/`); i >= 0 {
			nome = nome[i+1:]
		}
	}
	return maiusculaInicial(nome)
}

// sobretitulo é a linha miúda acima da saudação. O nome da empresa vem do
// [geral] empresa do inventário, quando existe; sem ele, só o nome do
// painel. Na 2.9.0 o nome de uma empresa saiu FIXO aqui, para todo mundo.
func (d *dashTab) sobretitulo() string {
	const painel = "PAINEL DE ACESSOS"
	if d.arq == nil {
		return painel
	}
	if empresa := strings.TrimSpace(d.arq.Geral["empresa"]); empresa != "" {
		return strings.ToUpper(empresa) + " · " + painel
	}
	return painel
}

func maiusculaInicial(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	return strings.ToUpper(string(r[0])) + string(r[1:])
}

// resumo é a linha de baixo da saudação: o inventário e as sessões.
func (d *dashTab) resumo() string {
	sessoes := "nenhuma sessão aberta"
	switch n := d.abertas(); {
	case n == 1:
		sessoes = "1 sessão aberta"
	case n > 1:
		sessoes = fmt.Sprintf("%d sessões abertas", n)
	}
	maquinas := fmt.Sprintf("%d máquinas", len(d.arq.Conexoes))
	if len(d.arq.Conexoes) == 1 {
		maquinas = "1 máquina"
	}
	grupos := fmt.Sprintf("%d grupos", d.contarGrupos())
	if d.contarGrupos() == 1 {
		grupos = "1 grupo"
	}
	return maquinas + " em " + grupos + " · " + sessoes
}

// contarGrupos conta os grupos de cima e os subgrupos diretos — a mesma
// conta do número "GRUPOS".
func (d *dashTab) contarGrupos() int {
	n := 0
	for _, g := range d.arvore {
		n += 1 + len(g.Filhos)
	}
	return n
}

// nomeArquivo é só o nome do .ini, para a barra de status; o caminho
// inteiro (que pode trazer até e-mail, quando a pasta é sincronizada) fica
// nos Ajustes.
func nomeArquivo(caminho string) string {
	limpo := strings.ReplaceAll(caminho, "\\", "/")
	return limpo[strings.LastIndex(limpo, "/")+1:]
}

// estatisticas: os números do inventário num cartão de vidro, um por
// célula. O quadradinho de cor de tela/shell/rdp é a MESMA cor do ícone do
// protocolo nos cards — serve de legenda sem precisar de uma.
func (d *dashTab) estatisticas(gtx layout.Context) layout.Dimensions {
	var maquinas, tela, shell, rdp int
	for _, cx := range d.arq.Conexoes {
		maquinas++
		if cx.Tem(conexoes.VNC) {
			tela++
		}
		if cx.Tem(conexoes.SSH) {
			shell++
		}
		if cx.Tem(conexoes.RDP) {
			rdp++
		}
	}
	grupos := d.contarGrupos()
	type numero struct {
		n   int
		rot string
		cor color.NRGBA
	}
	nums := []numero{
		{maquinas, "MÁQUINAS", tema.Borda2},
		{tela, "TELA", tema.Azul},
		{shell, "SHELL", tema.Verde},
		{rdp, "RDP", tema.Roxo},
		{grupos, "GRUPOS", tema.Borda2},
		{d.abertas(), "ABERTAS", tema.AzulCheio},
	}
	var filhos []layout.FlexChild
	for i, n := range nums {
		i, n := i, n
		filhos = append(filhos, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Stack{}.Layout(gtx,
				layout.Expanded(func(gtx layout.Context) layout.Dimensions {
					if i > 0 { // fio entre as células
						paint.FillShape(gtx.Ops, tema.CardBorda, clip.Rect{Max: image.Pt(1, gtx.Constraints.Min.Y)}.Op())
					}
					return layout.Dimensions{Size: gtx.Constraints.Min}
				}),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: 10, Bottom: 11, Left: 18, Right: 18}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(negrito(txt(d.th, fonteTitulo, unit.Sp(24), fmt.Sprint(n.n), tema.Texto)).Layout),
							layout.Rigid(layout.Spacer{Height: 3}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										lado := gtx.Dp(8)
										rr := clip.UniformRRect(image.Rectangle{Max: image.Pt(lado, lado)}, gtx.Dp(3))
										paint.FillShape(gtx.Ops, n.cor, rr.Op(gtx.Ops))
										return layout.Dimensions{Size: image.Pt(lado, lado)}
									}),
									layout.Rigid(layout.Spacer{Width: 6}.Layout),
									layout.Rigid(txt(d.th, fonteMono, unit.Sp(10.5), n.rot, tema.Sec).Layout),
								)
							}),
						)
					})
				}),
			)
		}))
	}
	corpo := op.Record(gtx.Ops)
	dims := layout.Flex{Axis: layout.Horizontal}.Layout(gtx, filhos...)
	desenho := corpo.Stop()
	// dentro do cartão de boas-vindas: mais sólido que ele, para os
	// números não ficarem em cima da malha
	superficie(gtx, dims.Size, tema.CardTintaH, tema.CardBorda, 14)
	desenho.Add(gtx.Ops)
	return dims
}

// CampoBusca é o campo de filtro, desenhado ou na lateral ou no próprio
// painel — nunca nos dois ao mesmo tempo (um widget.Editor só pode ser
// desenhado uma vez por frame). Quem decide é main.go, olhando o estado
// da lateral.
func (d *dashTab) CampoBusca(gtx layout.Context) layout.Dimensions {
	return d.caixaBusca(gtx, &d.filtroLateral)
}

// barraFiltro é a barra do Painel: o filtro (quando ele não está na
// lateral), recolher/expandir e, à direita, as duas ações sobre o
// inventário — "Nova conexão", a principal da tela, e "Recarregar INI".
// As duas moravam no topo até a 2.8, miúdas e do mesmo peso de tudo lá.
func (d *dashTab) barraFiltro(gtx layout.Context) layout.Dimensions {
	for d.btnRecolher.Clicked(gtx) {
		d.expandir = map[string]bool{}
	}
	for d.btnExpandir.Clicked(gtx) {
		d.expandirTudo()
	}
	if d.btnNova.Clicked(gtx) && d.aoNova != nil {
		d.aoNova("")
	}
	if d.btnRecarregar.Clicked(gtx) && d.recarregar != nil {
		d.recarregar()
	}
	naLateral := d.buscaNaLateral != nil && d.buscaNaLateral()
	return layout.Inset{Top: 20, Bottom: 12, Left: margemPainel, Right: margemPainel}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if naLateral {
					return layout.Dimensions{}
				}
				// encolhe com a folha: numa janela estreita é ela que cede,
				// não o "Nova conexão", o último da linha
				larg := min(gtx.Dp(440), gtx.Constraints.Max.X*35/100)
				gtx.Constraints.Min.X, gtx.Constraints.Max.X = larg, larg
				return layout.Inset{Right: 10}.Layout(gtx, d.campoFiltro)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				btns := []*widget.Clickable{&d.btnRecolher, &d.btnExpandir}
				rots := []string{"Recolher tudo", "Expandir tudo"}
				return trilho(gtx, 2, -1, btns, func(gtx layout.Context, i int, _, hover bool) layout.Dimensions {
					cor := tema.Sec
					if hover {
						cor = tema.Texto
					}
					// 3 + 34 + 3 = alturaBarra
					gtx.Constraints.Min.Y = gtx.Dp(alturaBarra) - gtx.Dp(6)
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: 12, Right: 12}.Layout(gtx,
							txt(d.th, fonteSansMedia, unit.Sp(13), rots[i], cor).Layout)
					})
				})
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: image.Pt(gtx.Constraints.Min.X, 0)}
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return botaoAcao(gtx, d.th, &d.btnRecarregar, icons.NavigationRefresh, "Recarregar INI", false)
			}),
			layout.Rigid(layout.Spacer{Width: 10}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return botaoAcao(gtx, d.th, &d.btnNova, icons.ContentAdd, "Nova conexão", true)
			}),
		)
	})
}

// expandirTudo abre todos os grupos e subgrupos de uma vez.
func (d *dashTab) expandirTudo() {
	var caminhar func(g *conexoes.Grupo)
	caminhar = func(g *conexoes.Grupo) {
		d.expandir[strings.Join(g.Caminho, ";")] = true
		for _, f := range g.Filhos {
			caminhar(f)
		}
	}
	for _, g := range d.arvore {
		caminhar(g)
	}
}

func (d *dashTab) campoFiltro(gtx layout.Context) layout.Dimensions {
	return d.caixaBusca(gtx, &d.filtro)
}

func (d *dashTab) caixaBusca(gtx layout.Context, ed *widget.Editor) layout.Dimensions {
	focado := gtx.Focused(ed)
	marcarFoco(focado)
	// Enter no campo: se o que está escrito não achou nada, trata como
	// DESTINO e conecta sem cadastrar (ver efemera.go). É o caminho de
	// "me passaram um IP no chat".
	for {
		ev, ok := ed.Update(gtx)
		if !ok {
			break
		}
		if _, sub := ev.(widget.SubmitEvent); sub && d.aoEfemera != nil {
			d.aoEfemera(ed.Text())
		}
	}
	// Na lateral o campo é mais baixo e sem borda (a moldura em volta já o
	// separa); no Painel ele é o vidro da folha, com borda.
	naLateral := ed == &d.filtroLateral
	alt, raio := gtx.Dp(alturaBarra), unit.Dp(12)
	fundo, borda := tema.CardTinta, tema.CardBorda
	tam := unit.Sp(14)
	if naLateral {
		alt, raio = gtx.Dp(36), 11
		fundo, borda = tinta(tema.TopoTxt, 0.06), transparente
		tam = 13
	}
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			size := gtx.Constraints.Min
			if focado {
				// anel de foco de 3dp na cor de acento, por fora
				anel := gtx.Dp(3)
				rr := clip.UniformRRect(image.Rect(-anel, -anel, size.X+anel, size.Y+anel), gtx.Dp(raio)+anel)
				paint.FillShape(gtx.Ops, comAlfa(tema.AzulCheio, 0.2), rr.Op(gtx.Ops))
				fundo, borda = tema.CardTintaH, comAlfa(tema.AzulCheio, 0.55)
			}
			superficie(gtx, size, fundo, borda, raio)
			return layout.Dimensions{Size: size}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			gtx.Constraints.Min.Y = alt
			return layout.Inset{Left: 12, Right: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return centroV(gtx, func(gtx layout.Context) layout.Dimensions {
					return d.linhaBusca(gtx, ed, naLateral, tam)
				})
			})
		}),
	)
}

func (d *dashTab) linhaBusca(gtx layout.Context, ed *widget.Editor, naLateral bool, tam unit.Sp) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			cor := tema.Fraco
			if naLateral {
				cor = tema.TopoSec // na moldura o Fraco fica abaixo de 3:1
			}
			return icone(gtx, icons.ActionSearch, cor, 17)
		}),
		layout.Rigid(layout.Spacer{Width: 10}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			dica := "Filtrar máquinas, hosts ou grupos"
			if naLateral {
				dica = "Filtrar máquinas"
			}
			es := material.Editor(d.th, ed, dica)
			es.Font = fonteSans
			es.TextSize = tam
			es.Color = tema.Texto
			es.HintColor = tema.Fraco
			if naLateral {
				es.HintColor = tema.TopoSec
			}
			return es.Layout(gtx)
		}),
	)
}

// barraStatus é a faixa de baixo da janela, na moldura, quando o Painel
// está à vista: versão (clicar abre o Sobre), o arquivo, e as contagens.
func (d *dashTab) barraStatus(gtx layout.Context) layout.Dimensions {
	if d.btnSobre.Clicked(gtx) {
		abrirSobre()
	}
	alt := gtx.Dp(statusAltura)
	gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = alt, alt
	larg := gtx.Constraints.Max.X
	paint.FillShape(gtx.Ops, tema.MolduraLinha, clip.Rect{Max: image.Pt(larg, 1)}.Op())
	arq := nomeArquivo(d.caminho)
	return layout.Inset{Left: 14, Right: 14}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return centroV(gtx, func(gtx layout.Context) layout.Dimensions {
			return d.itensStatus(gtx, arq)
		})
	})
}

func (d *dashTab) itensStatus(gtx layout.Context, arq string) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return d.btnSobre.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				pointer.CursorPointer.Add(gtx.Ops)
				cor := tema.TopoSec
				if d.btnSobre.Hovered() {
					cor = tema.TopoTxt
				}
				return rotulo(d.th, fonteMono, spRodape, nomeApp+" "+versaoInstalada(), cor)(gtx)
			})
		}),
		layout.Rigid(layout.Spacer{Width: 14}.Layout),
		layout.Rigid(rotulo(d.th, fonteMono, spRodape, arq, tema.TopoSec)),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Pt(gtx.Constraints.Min.X, 0)}
		}),
		layout.Rigid(rotulo(d.th, fonteMono, spRodape,
			fmt.Sprintf("%d máquinas · %d abertas", len(d.arq.Conexoes), d.abertas()), tema.TopoSec)),
		layout.Rigid(layout.Spacer{Width: 14}.Layout),
		layout.Rigid(rotulo(d.th, fonteMono, spRodape, "@JJMoratelli", tema.TopoSec)),
	)
}

// grupo desenha a linha do grupo e, se aberto, os filhos e os cards — os
// filhos recuados, com um fio vertical à esquerda ligando-os ao grupo
// (o "ramo"), em vez do recuo solto de antes.
func (d *dashTab) grupo(gtx layout.Context, g *conexoes.Grupo, nivel int, termo string) layout.Dimensions {
	chave := strings.Join(g.Caminho, ";")
	visiveis := filtrar(g.Conexoes, termo)

	aberto := d.expandir[chave]
	if termo != "" {
		// Filtro ativo: grupo sem nada que case some, e o que sobra já
		// aparece aberto — procurar não deveria exigir mais um clique.
		if len(visiveis) == 0 && !temDescendente(g, termo) {
			return layout.Dimensions{}
		}
		aberto = true
	}

	btn := d.clique("grupo:" + chave)
	if btn.Clicked(gtx) {
		d.expandir[chave] = !d.expandir[chave]
	}
	if btn.Hovered() {
		d.grupoSobCursor = chave
	}

	var filhos []layout.FlexChild
	if aberto {
		for _, f := range g.Filhos {
			f := f
			filhos = append(filhos, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return d.grupo(gtx, f, nivel+1, termo)
			}))
		}
		if len(visiveis) > 0 {
			filhos = append(filhos, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 4, Bottom: 14}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return d.grade(gtx, visiveis)
				})
			}))
		}
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return d.linhaGrupo(gtx, g, nivel, aberto, btn.Hovered())
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if len(filhos) == 0 {
				return layout.Dimensions{}
			}
			return layout.Inset{Left: 7}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Stack{}.Layout(gtx,
					layout.Expanded(func(gtx layout.Context) layout.Dimensions {
						paint.FillShape(gtx.Ops, tema.CardBorda, clip.Rect{Max: image.Pt(1, gtx.Constraints.Min.Y)}.Op())
						return layout.Dimensions{Size: gtx.Constraints.Min}
					}),
					layout.Stacked(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: 18}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx, filhos...)
						})
					}),
				)
			})
		}),
	)
}

// linhaGrupo: caixa de seleção do grupo, seta, a marca de cor (hexágono,
// só no grupo de cima), o nome e o contador. Sob o mouse o nome ganha a
// cor de acento, sem fundo — o fundo antigo piscava a linha inteira.
func (d *dashTab) linhaGrupo(gtx layout.Context, g *conexoes.Grupo, nivel int, aberto, hover bool) layout.Dimensions {
	seta := setaExpansor(aberto)
	corNome := tema.Texto
	if hover {
		corNome = tema.Azul
	}
	nome := negrito(txt(d.th, fonteTitulo, spGrupoTitulo, g.Nome, corNome))
	alt := gtx.Dp(40)
	if nivel > 0 {
		f := fonteSans
		f.Weight = font.SemiBold
		nome = txt(d.th, f, unit.Sp(14.5), g.Nome, corNome)
		alt = gtx.Dp(36)
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	gtx.Constraints.Min.Y = alt
	pointer.CursorPointer.Add(gtx.Ops)
	return centroV(gtx, func(gtx layout.Context) layout.Dimensions {
		return d.linhaGrupoItens(gtx, g, nivel, seta, nome)
	})
}

func (d *dashTab) linhaGrupoItens(gtx layout.Context, g *conexoes.Grupo, nivel int,
	seta *widget.Icon, nome material.LabelStyle) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		// Selecionar o grupo inteiro de uma vez: com 40 caixas por loja,
		// marcar uma a uma não é opção. O estado PARCIAL existe porque
		// "algumas selecionadas" não é nem marcado nem desmarcado.
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return d.caixaGrupo(gtx, g)
		}),
		layout.Rigid(layout.Spacer{Width: 10}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return icone(gtx, seta, tema.Sec, 17)
		}),
		layout.Rigid(layout.Spacer{Width: 8}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if nivel > 0 {
				return layout.Dimensions{}
			}
			return layout.Inset{Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return hexagono(gtx, gtx.Dp(14), corCategoria(g.Caminho[0]))
			})
		}),
		layout.Rigid(nome.Layout),
		layout.Rigid(layout.Spacer{Width: 10}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return contador(gtx, d.th, fmt.Sprint(g.Total()))
		}),
	)
}

// grade quebra os cards em linhas conforme a largura disponível. Os cards
// ESTICAM para ocupar a linha inteira (como o auto-fill de uma grade CSS),
// com largura mínima cardLargura: a borda direita da grade fica alinhada
// com a da barra de cima, em vez da sobra irregular de antes.
func (d *dashTab) grade(gtx layout.Context, lista []conexoes.Conexao) layout.Dimensions {
	minimo := gtx.Dp(cardLargura)
	esp := gtx.Dp(12)
	total := gtx.Constraints.Max.X
	porLinha := max(1, (total+esp)/(minimo+esp))
	// A divisão inteira joga fora até porLinha-1 px; eles vão, um a um,
	// para os primeiros cards da linha, senão a borda direita da grade
	// fica alguns px aquém da das ações acima dela.
	util := total - esp*(porLinha-1)
	larg, resto := util/porLinha, util%porLinha
	alt := gtx.Dp(cardAltura)

	var linhas []layout.FlexChild
	for i := 0; i < len(lista); i += porLinha {
		fatia := lista[i:min(i+porLinha, len(lista))]
		if i > 0 {
			linhas = append(linhas, layout.Rigid(layout.Spacer{Height: 12}.Layout))
		}
		linhas = append(linhas, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			var cols []layout.FlexChild
			for j, cx := range fatia {
				cx := cx
				if j > 0 {
					cols = append(cols, layout.Rigid(layout.Spacer{Width: 12}.Layout))
				}
				l := larg
				if j < resto {
					l++
				}
				cols = append(cols, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min = image.Pt(l, alt)
					gtx.Constraints.Max = image.Pt(l, alt)
					return d.card(gtx, cx)
				}))
			}
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, cols...)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, linhas...)
}

// card: vidro fosco (superficieVidro) sobre a folha, com borda; sob o mouse
// a tinta engrossa e a borda puxa para o azul. Dois blocos ancorados:
// identidade em cima (nome e caixa de seleção; o host com o ponto de
// vida), e embaixo o resumo dos protocolos e o trilho com os quatro.
func (d *dashTab) card(gtx layout.Context, cx conexoes.Conexao) layout.Dimensions {
	hov := d.clique("card:" + cx.GrupoStr() + "|" + cx.Nome)

	if hov.Hovered() {
		cx := cx
		d.sobCursor = &cx
		// A chave do hover já identifica o card (grupo + nome), então
		// serve de identidade para a dica sem inventar outra.
		pedirDica("card:"+cx.GrupoStr()+"|"+cx.Nome, cx.Descricao)
	}
	marcado := d.selecionada(cx)

	// A sombra fica FORA do Clickable: ele recorta o que desenha no
	// tamanho do card (é o recorte da área de clique), e a sombra, que
	// mora justamente fora do card, sumia inteira.
	corpo := op.Record(gtx.Ops)
	dims := hov.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				size := gtx.Constraints.Min
				// Hover em transição (anima.go): a tinta engrossa e a borda
				// puxa para o azul aos poucos.
				h := progressoHover(gtx, hov, hov.Hovered())
				borda := misturarCor(tema.CardBorda, misturarCor(tema.CardBorda, tema.AzulCheio, 0.45), h)
				if marcado {
					borda = tema.AzulCheio
				}
				superficieVidro(gtx, size, misturarCor(tema.CardTinta, tema.CardTintaH, h), borda, cardRaio)
				return layout.Dimensions{Size: size}
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				// O conteúdo OCUPA o card inteiro, para o rodapé (resumo e
				// trilho) ficar sempre encostado embaixo.
				gtx.Constraints.Min = gtx.Constraints.Max
				return layout.Inset{Top: 12, Bottom: 10, Left: 14, Right: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									l := negrito(txt(d.th, fonteTitulo, spCardNome, cx.Nome, tema.Texto))
									l.MaxLines = 1
									return l.Layout(gtx)
								}),
								layout.Rigid(layout.Spacer{Width: 8}.Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return d.caixaSelecao(gtx, cx)
								}),
							)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							// O ponto de vida vem ANTES do host, que é o dado
							// a que ele se refere. Verde respondeu, vermelho
							// não; só o contorno = ainda não perguntei.
							cv, mostra := corDaVida(d.estadoVida(cx))
							return layout.Inset{Top: 5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return pontoVida(gtx, cv, mostra)
									}),
									layout.Rigid(layout.Spacer{Width: 5}.Layout),
									layout.Flexed(1, rotulo(d.th, fonteMono, spCardHost, cx.Host, tema.Sec)),
								)
							})
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Dimensions{Size: image.Pt(gtx.Constraints.Min.X, gtx.Constraints.Min.Y)}
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return d.meta(gtx, cx)
						}),
						layout.Rigid(layout.Spacer{Height: 10}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return d.botoes(gtx, cx)
						}),
					)
				})
			}),
		)
	})
	desenho := corpo.Stop()
	sombra(gtx, dims.Size, cardRaio)
	desenho.Add(gtx.Ops)

	return dims
}

// cardRascunho é o card TEMPORÁRIO que aparece na busca quando o texto
// digitado tem cara de destino e não de filtro ("10.1.1.99",
// "zanthus@caixa02", "rdp serv-ad").
//
// Existe porque conectar num destino avulso já era possível — bastava dar
// Enter na busca — mas ninguém descobre um atalho invisível. Como card,
// com a MESMA fileira de ícones dos cards salvos, a ação fica onde o olho
// já está: clica no protocolo e abre, sem cadastrar nada.
func (d *dashTab) cardRascunho(gtx layout.Context, termo string) layout.Dimensions {
	// Um nome cru ("fc52002-lj06") só vira destino quando a busca não
	// achou nada — enquanto houver máquina casando, o texto é filtro.
	cx, ok := alvoRascunho(termo, !algumCasa(d.arq.Conexoes, termo))
	if !ok {
		return layout.Dimensions{}
	}
	larg, alt := gtx.Dp(cardLargura)+gtx.Dp(12), gtx.Dp(cardAltura)
	return layout.Inset{Left: margemPainel, Right: margemPainel, Bottom: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = image.Pt(larg, alt)
		gtx.Constraints.Max = image.Pt(larg, alt)
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				size := gtx.Constraints.Min
				sombra(gtx, size, cardRaio)
				// borda na cor de acento (e não a neutra dos cards salvos):
				// é o que diz, sem legenda, que este card não está no
				// inventário.
				superficieVidro(gtx, size, tema.CardTintaH, tema.AzulCheio, cardRaio)
				return layout.Dimensions{Size: size}
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min = gtx.Constraints.Max
				return layout.Inset{Top: 12, Bottom: 10, Left: 14, Right: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							l := negrito(txt(d.th, fonteTitulo, spCardNome, cx.Host, tema.Texto))
							l.MaxLines = 1
							return l.Layout(gtx)
						}),
						layout.Rigid(layout.Spacer{Height: 4}.Layout),
						layout.Rigid(rotulo(d.th, fonteMono, spCardHost, "não cadastrado", tema.Sec)),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Dimensions{Size: gtx.Constraints.Min}
						}),
						layout.Rigid(rotulo(d.th, fonteMono, spMetaCard,
							"sessão temporária", tema.CardFraco)),
						layout.Rigid(layout.Spacer{Height: 10}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return d.botoesCom(gtx, cx, func(p conexoes.Protocolo) {
								if d.aoRascunho != nil {
									d.aoRascunho(cx, p)
								}
							})
						}),
					)
				})
			}),
		)
	})
}

// meta são as linhas apagadas com o resumo de cada protocolo configurado
// ("tela 5900 · encaixar · auto"), como no card do app original. No
// máximo duas: o card tem altura fixa, e a terceira empurraria o trilho.
func (d *dashTab) meta(gtx layout.Context, cx conexoes.Conexao) layout.Dimensions {
	var linhas []layout.FlexChild
	add := func(s string) {
		if len(linhas) == 2 {
			return
		}
		linhas = append(linhas, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			l := txt(d.th, fonteMono, spMetaCard, s, tema.CardFraco)
			l.MaxLines = 1
			return l.Layout(gtx)
		}))
	}
	if cx.Tem(conexoes.VNC) {
		s := fmt.Sprintf("tela %d · %s", cx.VNC.Porta, cx.VNC.Modo)
		if cx.VNC.Auto {
			s += " · auto"
		}
		if cx.VNC.Ronly {
			s += " · só ver"
		}
		add(s)
	}
	if cx.Tem(conexoes.SSH) {
		s := fmt.Sprintf("ssh %s@%d", cx.SSH.Usuario, cx.SSH.Porta)
		if cx.SSH.Auto {
			s += " · auto"
		}
		add(s)
	}
	if cx.Tem(conexoes.RDP) {
		s := fmt.Sprintf("rdp %s@%d · %s", cx.RDP.Usuario, cx.RDP.Porta, cx.RDP.Tela)
		add(s)
	}
	if len(linhas) == 0 {
		add("sem protocolo configurado")
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, linhas...)
}

// botoes é a fileira de protocolos do card: indicador e botão no mesmo
// controle. Slots SEMPRE presentes, mesmo desligados (apagados e inertes):
// ícone que aparece e some obriga a reler o cartão toda vez; posição fixa
// se aprende e para de ser lida (gtk.md §6).
func (d *dashTab) botoes(gtx layout.Context, cx conexoes.Conexao) layout.Dimensions {
	return d.botoesCom(gtx, cx, nil)
}

// botoesCom é o trilho de protocolos do card. aoClicar != nil troca o
// que o clique faz — é o que o card temporário usa para passar antes pelo
// diálogo de credenciais, já que um destino não cadastrado não tem senha
// guardada em lugar nenhum.
//
// Desde a 2.9 é um trilho afundado com os quatro segmentos: o protocolo
// configurado fica EM RELEVO, com o ícone na cor dele; o que não está
// configurado fica rente ao trilho, apagado. Antes cada configurado tinha
// um fundo colorido próprio, e uma fileira de cards virava um confete.
func (d *dashTab) botoesCom(gtx layout.Context, cx conexoes.Conexao,
	aoClicar func(conexoes.Protocolo)) layout.Dimensions {
	var filhos []layout.FlexChild
	for i, e := range protocolos {
		e := e
		ligado := cx.Tem(e.p)
		btn := d.clique(fmt.Sprintf("%s|%s|%s", cx.GrupoStr(), cx.Nome, e.p))
		if ligado && btn.Hovered() {
			d.protoSobCursor = e.p
		}
		if ligado && btn.Clicked(gtx) {
			if d.travaAbrir == 0 {
				if aoClicar != nil {
					aoClicar(e.p)
				} else {
					d.abrirEm(cx, e.p, true)
				}
			}
		}

		conteudo := func(gtx layout.Context) layout.Dimensions {
			// largura = a fatia inteira que o Flexed deu, pra os quatro
			// segmentos cobrirem o trilho.
			size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(icoAltura))
			gtx.Constraints.Min = size
			cor := comAlfa(tema.Fraco, 0.45)
			if ligado {
				cor = e.cor()
				fundo := tema.Elevado
				if btn.Hovered() {
					fundo = misturarCor(tema.Elevado, e.cor(), 0.14)
				}
				superficie(gtx, size, fundo, transparente, icoRaio)
				pointer.CursorPointer.Add(gtx.Ops)
			}
			layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return icone(gtx, e.ic, cor, 17)
			})
			return layout.Dimensions{Size: size}
		}

		if i > 0 {
			filhos = append(filhos, layout.Rigid(layout.Spacer{Width: 3}.Layout))
		}
		filhos = append(filhos, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if !ligado {
				return conteudo(gtx)
			}
			return btn.Layout(gtx, conteudo)
		}))
	}
	return layout.Background{}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			superficie(gtx, gtx.Constraints.Min, tema.Afundado, transparente, trilhoRaio)
			return layout.Dimensions{Size: gtx.Constraints.Min}
		},
		func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(3).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, filhos...)
			})
		},
	)
}

// ---------------------------------------------------------------- apoio

func filtrar(lista []conexoes.Conexao, termo string) []conexoes.Conexao {
	if termo == "" {
		return lista
	}
	var out []conexoes.Conexao
	for _, cx := range lista {
		if casaComTermo(cx, termo) {
			out = append(out, cx)
		}
	}
	return out
}

// casaComTermo é o teste de UMA conexão, num lugar só — o filtro e o
// "tem alguma?" abaixo têm de concordar sempre.
func casaComTermo(cx conexoes.Conexao, termo string) bool {
	return strings.Contains(strings.ToLower(cx.Nome), termo) ||
		strings.Contains(strings.ToLower(cx.Host), termo) ||
		strings.Contains(strings.ToLower(cx.GrupoStr()), termo) ||
		// A descrição entra no casamento, e não só na tela: ela existe
		// para dizer o que o nome não diz ("impressora fiscal", "troca em
		// outubro"), e é por esse detalhe que se procura quando o nome
		// não é lembrado. Vale aqui, num lugar só, então o filtro do
		// Painel e a busca do atalho global concordam de graça.
		strings.Contains(strings.ToLower(cx.Descricao), termo)
}

// algumCasa responde "tem alguma?" sem montar a lista das que casam, e
// para no primeiro achado.
//
// Quem só quer a resposta sim/não estava pagando a lista inteira A CADA
// QUADRO: o cardRascunho montava uma fatia de até 274 conexões (~90 KB)
// só para testar se ela estava vazia, enquanto a pessoa digita; e o
// temDescendente fazia o mesmo por grupo, recursivamente, na taxa de
// quadro — inclusive a da lateral, que redesenha junto com a sessão
// RDP/VNC ativa. O comentário de baixo, sobre o "travadinho ao marcar
// vários", conta o que montar fatia por quadro já custou aqui uma vez.
func algumCasa(lista []conexoes.Conexao, termo string) bool {
	if termo == "" {
		return len(lista) > 0
	}
	for _, cx := range lista {
		if casaComTermo(cx, termo) {
			return true
		}
	}
	return false
}

func temDescendente(g *conexoes.Grupo, termo string) bool {
	for _, f := range g.Filhos {
		if algumCasa(f.Conexoes, termo) || temDescendente(f, termo) {
			return true
		}
	}
	return false
}

// corCategoria dá uma cor estável por grupo de topo. Cor de categoria
// NUNCA significa erro — por isso a cor de erro fica fora da lista.
func corCategoria(nome string) color.NRGBA {
	paleta := []color.NRGBA{tema.Azul, tema.Verde, tema.Roxo, tema.AtencaoFg, tema.OkFg}
	var soma int
	for _, r := range nome {
		soma += int(r)
	}
	return paleta[soma%len(paleta)]
}

// ---------------------------------------------------- seleção em massa

// chaveCx identifica uma conexão: nome se repete entre lojas, o par
// grupo+nome não.
func chaveCx(cx conexoes.Conexao) string { return cx.GrupoStr() + "|" + cx.Nome }

func (d *dashTab) selecionada(cx conexoes.Conexao) bool {
	_, ok := d.selecao[chaveCx(cx)]
	return ok
}

func (d *dashTab) alternarSelecao(cx conexoes.Conexao) {
	k := chaveCx(cx)
	if _, ok := d.selecao[k]; ok {
		delete(d.selecao, k)
		return
	}
	d.selecao[k] = cx
}

// barraSelecao só existe quando há seleção — barra de ações em massa
// aparece com a seleção e traz o contador (regra da skill). Sem seleção
// ela não ocupa altura nenhuma.
func (d *dashTab) barraSelecao(gtx layout.Context) layout.Dimensions {
	if len(d.selecao) == 0 {
		return layout.Dimensions{}
	}
	if d.btnLimparSel.Clicked(gtx) {
		d.selecao = map[string]conexoes.Conexao{}
	}
	if d.btnMassa.Clicked(gtx) && d.aoExecMassa != nil {
		d.aoExecMassa(d.listaSelecionada())
	}
	if d.btnExcluirSel.Clicked(gtx) && d.aoExcluirMassa != nil {
		d.aoExcluirMassa(d.listaSelecionada())
	}

	return layout.Inset{Top: 0, Bottom: 12, Left: margemPainel, Right: margemPainel}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Background{}.Layout(gtx,
			func(gtx layout.Context) layout.Dimensions {
				superficie(gtx, gtx.Constraints.Min, tema.AzulFraco, tema.Azul, 12)
				return layout.Dimensions{Size: gtx.Constraints.Min}
			},
			func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.Inset{Top: 6, Bottom: 6, Left: 10, Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(negrito(txt(d.th, fonteSans, spRotulo,
							fmt.Sprintf("%d máquina(s) selecionada(s)", len(d.selecao)), tema.Texto)).Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Dimensions{Size: gtx.Constraints.Min}
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return botaoIconePerigo(gtx, &d.btnExcluirSel, icons.ActionDelete)
						}),
						layout.Rigid(layout.Spacer{Width: 6}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return botaoBarra(gtx, d.th, &d.btnLimparSel, "limpar seleção")
						}),
						layout.Rigid(layout.Spacer{Width: 6}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return botaoPrimario(gtx, d.th, &d.btnMassa,
								fmt.Sprintf("Executar em %d", len(d.selecao)))
						}),
					)
				})
			},
		)
	})
}

// listaSelecionada devolve a seleção em ordem estável (a do arquivo), pra
// a aba de massa não embaralhar a lista a cada execução.
func (d *dashTab) listaSelecionada() []conexoes.Conexao {
	var out []conexoes.Conexao
	for _, cx := range d.arq.Conexoes {
		if d.selecionada(cx) {
			out = append(out, cx)
		}
	}
	return out
}

// caixaSelecao é a marca de lote do card: discreta, some no fundo até ser
// marcada — não deve competir com o nome da máquina (tema.py).
func (d *dashTab) caixaSelecao(gtx layout.Context, cx conexoes.Conexao) layout.Dimensions {
	btn := d.clique("sel:" + chaveCx(cx))
	if btn.Clicked(gtx) {
		d.alternarSelecao(cx)
	}
	marcada := d.selecionada(cx)
	return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		lado := gtx.Dp(16)
		gtx.Constraints.Min = image.Pt(lado, lado)
		gtx.Constraints.Max = image.Pt(lado, lado)
		pointer.CursorPointer.Add(gtx.Ops)
		fundo, borda := tema.Elevado, tema.Borda2
		if btn.Hovered() {
			borda = comAlfa(tema.AzulCheio, 0.6)
		}
		if marcada {
			fundo, borda = tema.AzulCheio, tema.AzulCheio
		}
		superficie(gtx, image.Pt(lado, lado), fundo, borda, 5)
		if marcada {
			layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return icone(gtx, icons.NavigationCheck, hex(0xffffff), 11)
			})
		}
		return layout.Dimensions{Size: image.Pt(lado, lado)}
	})
}

// todasDoGrupo devolve as conexões do grupo e de todos os subgrupos.
func todasDoGrupo(g *conexoes.Grupo) []conexoes.Conexao {
	out := append([]conexoes.Conexao{}, g.Conexoes...)
	for _, f := range g.Filhos {
		out = append(out, todasDoGrupo(f)...)
	}
	return out
}

// contarSelecao percorre a árvore SEM alocar: a caixa de cada grupo
// precisa desse número a cada quadro, e montar uma fatia com as 54
// conexões da loja 60 vezes por segundo era o "travadinho" ao marcar
// vários.
func (d *dashTab) contarSelecao(g *conexoes.Grupo) (marcadas, total int) {
	for _, cx := range g.Conexoes {
		total++
		if _, ok := d.selecao[chaveCx(cx)]; ok {
			marcadas++
		}
	}
	for _, f := range g.Filhos {
		m, t := d.contarSelecao(f)
		marcadas += m
		total += t
	}
	return
}

// caixaGrupo marca/desmarca o grupo inteiro. Três estados: nenhuma,
// algumas (traço) e todas — o traço é o "indeterminate" que a skill pede,
// sem ele um grupo meio selecionado parece não selecionado.
func (d *dashTab) caixaGrupo(gtx layout.Context, g *conexoes.Grupo) layout.Dimensions {
	marcadas, total := d.contarSelecao(g)
	btn := d.clique("selg:" + strings.Join(g.Caminho, ";"))
	if btn.Clicked(gtx) {
		// só aqui a fatia é montada: no clique, não a cada quadro.
		lista := todasDoGrupo(g)
		if marcadas == total {
			for _, cx := range lista {
				delete(d.selecao, chaveCx(cx))
			}
		} else {
			for _, cx := range lista {
				d.selecao[chaveCx(cx)] = cx
			}
		}
		marcadas, total = d.contarSelecao(g)
	}
	return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		lado := gtx.Dp(16)
		gtx.Constraints.Min = image.Pt(lado, lado)
		gtx.Constraints.Max = image.Pt(lado, lado)
		pointer.CursorPointer.Add(gtx.Ops)
		fundo, borda := tema.Elevado, tema.Borda2
		if btn.Hovered() {
			borda = comAlfa(tema.AzulCheio, 0.6)
		}
		if marcadas > 0 {
			fundo, borda = tema.AzulCheio, tema.AzulCheio
		}
		superficie(gtx, image.Pt(lado, lado), fundo, borda, 5)
		switch {
		case marcadas == total && marcadas > 0:
			layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return icone(gtx, icons.NavigationCheck, hex(0xffffff), 11)
			})
		case marcadas > 0:
			// traço do estado parcial
			larg, alt := gtx.Dp(8), gtx.Dp(2)
			off := image.Pt((lado-larg)/2, (lado-alt)/2)
			defer op.Offset(off).Push(gtx.Ops).Pop()
			paint.FillShape(gtx.Ops, hex(0xffffff), clip.Rect{Max: image.Pt(larg, alt)}.Op())
		}
		return layout.Dimensions{Size: image.Pt(lado, lado)}
	})
}

// aoNovaGrupo abre o menu de um grupo: por enquanto, só criar conexão
// dentro dele (renomear/remover grupo mexe em N seções e fica para
// quando houver desfazer).
func (d *dashTab) aoNovaGrupo(caminhoGrupo string) {
	if d.aoNova == nil {
		return
	}
	abrirMenu(janelaPrincipal, d.ultimaPos.Add(offsetConteudo), []*itemMenu{
		{rotulo: "Nova conexão aqui…", acao: func() { d.aoNova(caminhoGrupo) }},
	})
}

// protocoloPadrao é o que abre ao clicar no card sem mirar num ícone:
// tela primeiro, senão shell, senão RDP — a mesma ordem do app original.
func protocoloPadrao(cx conexoes.Conexao) (conexoes.Protocolo, bool) {
	for _, p := range []conexoes.Protocolo{conexoes.VNC, conexoes.SSH, conexoes.RDP} {
		if cx.Tem(p) {
			return p, true
		}
	}
	return "", false
}

// abrirEm abre a conexão; focar=false deixa a aba em segundo plano.
func (d *dashTab) abrirEm(cx conexoes.Conexao, p conexoes.Protocolo, focar bool) {
	if d.abrir == nil {
		return
	}
	abrirEmSegundoPlano = !focar
	d.abrir(cx, p)
	abrirEmSegundoPlano = false
}

// corDaVida: cinza não é pintado (fica só o trilho da categoria), porque
// "não sei" não deve gritar nada.
func corDaVida(e vida.Estado) (color.NRGBA, bool) {
	switch e {
	case vida.Viva:
		return tema.Verde, true
	case vida.Morta:
		return tema.ErroFg, true
	}
	return color.NRGBA{}, false
}
