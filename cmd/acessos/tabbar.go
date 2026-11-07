package main

import (
	"image"
	"time"

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

// As medidas da aba (altura, raio, curvas) estão em moldura.go: desde a
// 2.9 a tira de abas mora dentro do topo, e a aba ativa se funde à folha.
const (
	abaEspaco = unit.Dp(4) // entre uma aba e a seguinte
)

// tabBar é a tira de abas-documento (padrão B). Desde a 2.9 mora dentro do
// topo: aba em repouso sem fundo, sob o mouse um véu com os cantos de cima
// redondos, e a ativa pintada com a folha e contornada (ver layoutAba).
type tabBar struct {
	// tagRolagem/acumulado: trocar de aba com a rodinha. Mouse de alta
	// resolução manda dezenas de eventos fracionários por volta do
	// dente — somamos até completar um "click" inteiro e só então
	// andamos UMA aba, senão uma rodada varre a tira toda.
	tagRolagem    *int
	acumulado     float32
	ultimaRolagem time.Time
	largurasAba   []int

	// chaves identifica O QUE cada aba é (protocolo + máquina) para não
	// abrir duas vezes a mesma coisa. Fica FORA do rótulo porque o rótulo
	// de uma conexão salva mostra só o nome da máquina — o protocolo já
	// está no selo colorido ao lado.
	chaves    map[Tab]string
	tabs      []Tab
	selectBtn []widget.Clickable
	closeBtn  []widget.Clickable
	idx       int
}

// Rolagem da tira: UM passo por gesto.
//
// Roda de alta definição não manda "um dente": manda uma enxurrada de
// deltas fracionários, dezenas por volta. Só somar e dividir por um passo
// fazia uma rodada varrer a tira inteira. Então valem as duas coisas: um
// mínimo de deslocamento (para o toque leve não contar) E um intervalo
// mínimo entre trocas (para a enxurrada contar como um gesto só).
const (
	passoRolagem     = 8                      // pixels que já valem um passo
	intervaloRolagem = 140 * time.Millisecond // uma aba por gesto
)

func newTabBar(tabs []Tab) *tabBar {
	return &tabBar{
		tagRolagem: new(int),
		tabs:       tabs,
		selectBtn:  make([]widget.Clickable, len(tabs)),
		closeBtn:   make([]widget.Clickable, len(tabs)),
	}
}

// append adiciona uma aba no fim e já foca nela (é o que se espera ao
// clicar num card do painel).
// appendFundo adiciona a aba SEM focar nela (botão do meio no card).
func (b *tabBar) appendFundo(t Tab, chave string) {
	idx := b.idx
	b.appendCom(t, chave)
	b.idx = idx
}

// appendCom adiciona a aba com uma chave de identidade própria.
func (b *tabBar) appendCom(t Tab, chave string) {
	b.append(t)
	if b.chaves == nil {
		b.chaves = map[Tab]string{}
	}
	b.chaves[t] = chave
}

// chave devolve a identidade da aba (cai no rótulo quando não há uma).
func (b *tabBar) chave(t Tab) string {
	if c, ok := b.chaves[t]; ok {
		return c
	}
	return t.Title()
}

func (b *tabBar) append(t Tab) {
	b.tabs = append(b.tabs, t)
	b.selectBtn = append(b.selectBtn, widget.Clickable{})
	b.closeBtn = append(b.closeBtn, widget.Clickable{})
	b.idx = len(b.tabs) - 1
}

// prependPinned insere t no início (índice 0) e seleciona-o — usado só
// para a aba Painel, na montagem inicial.
func (b *tabBar) prependPinned(t Tab) {
	b.tabs = append([]Tab{t}, b.tabs...)
	b.selectBtn = append([]widget.Clickable{{}}, b.selectBtn...)
	b.closeBtn = append([]widget.Clickable{{}}, b.closeBtn...)
	b.idx = 0
}

func (b *tabBar) active() Tab {
	if b.idx < 0 || b.idx >= len(b.tabs) {
		return nil
	}
	return b.tabs[b.idx]
}

func (b *tabBar) selectIndex(i int) {
	if i >= 0 && i < len(b.tabs) {
		b.idx = i
	}
}

// closeActive fecha a aba selecionada agora (Ctrl+W) — sem efeito na aba
// fixa (Painel).
func (b *tabBar) closeActive() { b.fechar(b.idx) }

func (b *tabBar) fechar(i int) {
	if i < 0 || i >= len(b.tabs) || b.tabs[i].Pinned() {
		return
	}
	b.tabs[i].Close()
	delete(b.chaves, b.tabs[i])
	b.tabs = append(b.tabs[:i], b.tabs[i+1:]...)
	b.selectBtn = append(b.selectBtn[:i], b.selectBtn[i+1:]...)
	b.closeBtn = append(b.closeBtn[:i], b.closeBtn[i+1:]...)
	if b.idx >= len(b.tabs) {
		b.idx = len(b.tabs) - 1
	}
}

// retirar tira a aba da tira SEM fechá-la, devolvendo-a com a chave de
// identidade. É o que separa "destacar" de "fechar": fechar chama
// t.Close(), que derruba a sessão e mata o processo-filho; destacar só
// muda quem desenha a aba, e a sessão nem percebe.
//
// A aba fixa (Painel) não sai — ela não tem sessão para destacar, e tirá-la
// deixaria a tira no estado "zero abas" que ela existe para impedir.
func (b *tabBar) retirar(i int) (Tab, string, bool) {
	if i < 0 || i >= len(b.tabs) || b.tabs[i].Pinned() {
		return nil, "", false
	}
	t := b.tabs[i]
	chave := b.chave(t)
	delete(b.chaves, t)
	b.tabs = append(b.tabs[:i], b.tabs[i+1:]...)
	b.selectBtn = append(b.selectBtn[:i], b.selectBtn[i+1:]...)
	b.closeBtn = append(b.closeBtn[:i], b.closeBtn[i+1:]...)
	if b.idx >= len(b.tabs) {
		b.idx = len(b.tabs) - 1
	}
	return t, chave, true
}

// indiceDe acha a aba na tira. -1 quando ela não está lá (destacada, por
// exemplo).
func (b *tabBar) indiceDe(t Tab) int {
	for i := range b.tabs {
		if b.tabs[i] == t {
			return i
		}
	}
	return -1
}

func (b *tabBar) update(gtx layout.Context) {
	for i := range b.tabs {
		if b.selectBtn[i].Clicked(gtx) {
			b.idx = i
		}
	}
	for i := range b.tabs {
		if !b.tabs[i].Pinned() && b.closeBtn[i].Clicked(gtx) {
			b.fechar(i)
			break // as fatias mudaram de tamanho; o resto espera o próximo frame
		}
	}
}

// layout desenha a tira de abas na altura toda do topo, com as abas
// encostadas embaixo: a ativa, pintada com a folha, cobre o fio da base do
// topo e desce até a folha (formaAbaAtiva). largurasAba guarda a largura
// de cada uma, com o espaço depois dela, para o clique do meio achar a aba.
// colada: a lateral está aberta e a primeira aba é a ativa — ela começa
// exatamente na borda da folha e desce reta (ver formaAbaAtiva).
func (b *tabBar) layout(gtx layout.Context, th *material.Theme, colada bool) layout.Dimensions {
	b.update(gtx)
	b.rolagem(gtx)

	h := gtx.Constraints.Max.Y
	gtx.Constraints.Min.Y = h

	if len(b.largurasAba) != len(b.tabs) {
		b.largurasAba = make([]int, len(b.tabs))
	}
	// Largura máxima por aba: até 220dp, e menos quando não cabem todas
	// (mínimo 110dp) — o título encolhe com reticências em vez de as
	// últimas abas sumirem cortadas, inalcançáveis.
	maxAba := gtx.Dp(220)
	if n := len(b.tabs); n > 0 {
		cabe := (gtx.Constraints.Max.X - n*gtx.Dp(abaEspaco)) / n
		maxAba = min(maxAba, max(cabe, gtx.Dp(110)))
	}
	children := make([]layout.FlexChild, 0, len(b.tabs)+1)
	for i := range b.tabs {
		i := i
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			var fechar *widget.Clickable
			if !b.tabs[i].Pinned() {
				fechar = &b.closeBtn[i]
			}
			d := layoutAba(gtx, th, b.tabs[i], i == b.idx, colada && i == 0, &b.selectBtn[i], fechar, maxAba)
			d.Size.X += gtx.Dp(abaEspaco)
			b.largurasAba[i] = d.Size.X
			return d
		}))
	}
	dims := layout.Flex{Axis: layout.Horizontal, Alignment: layout.End}.Layout(gtx, children...)

	// Rolagem do mouse e clique do meio: o alvo cobre a tira toda, mas
	// deixa o evento passar (PassOp) para o arrastar da janela por baixo.
	pass := pointer.PassOp{}.Push(gtx.Ops)
	area := clip.Rect{Max: image.Pt(gtx.Constraints.Max.X, h)}.Push(gtx.Ops)
	event.Op(gtx.Ops, b.tagRolagem)
	area.Pop()
	pass.Pop()
	return dims
}

// layoutAba desenha uma aba: o selo do tipo (ícone na cor do protocolo),
// o título e o X. A ativa é pintada com a folha e ganha as curvas da base;
// a que está sob o mouse, um véu discreto com os cantos de cima redondos.
// O fundo fica FORA do Clickable: ele recorta a pintura no tamanho da aba,
// e as curvas moram justamente fora dela.
func layoutAba(gtx layout.Context, th *material.Theme, t Tab, ativa, colada bool,
	sel *widget.Clickable, fechar *widget.Clickable, maxAba int) layout.Dimensions {

	alt := gtx.Dp(abaAltura) + 1 // +1: cobre o fio da base do topo
	gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = alt, alt
	gtx.Constraints.Min.X = 0

	conteudo := op.Record(gtx.Ops)
	d := sel.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		cor := tema.TopoSec
		f := fonteSansMedia
		if ativa {
			cor = tema.Texto
			f = fonteSans
			f.Weight = font.SemiBold
		} else if sel.Hovered() {
			cor = tema.TopoTxt
		}
		dir := unit.Dp(12)
		if fechar != nil {
			dir = 5
		}
		// o que sobra para o título depois do ícone, dos respiros e do X
		resto := gtx.Dp(11) + gtx.Dp(16) + gtx.Dp(8) + gtx.Dp(dir)
		if fechar != nil {
			resto += gtx.Dp(6) + gtx.Dp(20)
		}
		maxTitulo := max(maxAba-resto, gtx.Dp(24))
		// W: alinha à esquerda e CENTRALIZA na altura da aba. O Flex passa
		// o mínimo de altura adiante, e o rótulo, recebendo a altura toda,
		// desenhava o texto colado no alto.
		return centroV(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: 11, Right: dir}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						ic, corIc, _ := t.Selo()
						if ic == nil {
							return layout.Dimensions{}
						}
						return icone(gtx, ic, corIc, 16)
					}),
					layout.Rigid(layout.Spacer{Width: 8}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, maxTitulo)
						return txt(th, f, unit.Sp(12), t.Title(), cor).Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if fechar == nil {
							return layout.Dimensions{}
						}
						return layout.Inset{Left: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return fechar.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								lado := gtx.Dp(20)
								gtx.Constraints.Min = image.Pt(lado, lado)
								gtx.Constraints.Max = gtx.Constraints.Min
								pointer.CursorPointer.Add(gtx.Ops)
								c := tema.TopoSec // na moldura; o Fraco ali fica abaixo de 3:1
								if ativa {
									c = tema.Fraco // na folha
								}
								if fechar.Hovered() {
									// fechar ganha vermelho SÓ no hover (ver
									// ui-pesos-botoes): aviso leve de "isto vai sumir".
									c = tema.ErroFg
									superficie(gtx, gtx.Constraints.Min, comAlfa(tema.ErroFg, 0.12), transparente, 6)
								}
								return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return icone(gtx, iconeFechar, c, 14)
								})
							})
						})
					}),
				)
			})
		})
	})
	desenho := conteudo.Stop()

	switch {
	case ativa:
		forma := clip.Outline{Path: formaAbaAtiva(gtx, d.Size, colada)}.Op().Push(gtx.Ops)
		pintarFolha(gtx)
		forma.Pop()
		paint.FillShape(gtx.Ops, tema.Contorno,
			clip.Stroke{Path: contornoAbaAtiva(gtx, d.Size, colada), Width: 1}.Op())
	case sel.Hovered():
		r := gtx.Dp(abaRaio)
		rr := clip.RRect{Rect: image.Rectangle{Max: image.Pt(d.Size.X, d.Size.Y-1)}, NW: r, NE: r}
		paint.FillShape(gtx.Ops, tinta(tema.TopoTxt, 0.06), rr.Op(gtx.Ops))
	}
	desenho.Add(gtx.Ops)
	return d
}

// rolagem troca de aba com a rodinha do mouse sobre a tira.
//
// O cuidado aqui é com mouse de alta resolução: ele não manda "um dente",
// manda uma chuva de deltas fracionários. Somando tudo e só trocando de
// aba a cada passoRolagem pixels, uma volta da rodinha anda algumas abas,
// e não a tira inteira.
func (b *tabBar) rolagem(gtx layout.Context) {
	for {
		ev, ok := gtx.Source.Event(pointer.Filter{
			Target:  b.tagRolagem,
			Kinds:   pointer.Scroll | pointer.Press,
			ScrollY: pointer.ScrollRange{Min: -1000, Max: 1000},
		})
		if !ok {
			break
		}
		pe, isP := ev.(pointer.Event)
		if !isP {
			continue
		}
		// clique do MEIO na tira fecha a aba sob o cursor — mesmo gesto do
		// navegador, e o que o app original faz.
		if pe.Kind == pointer.Press && pe.Buttons.Contain(pointer.ButtonTertiary) {
			if i := b.abaEm(pe.Position.X); i >= 0 {
				b.fechar(i)
			}
			continue
		}
		b.acumulado += pe.Scroll.Y
		if b.acumulado > -passoRolagem && b.acumulado < passoRolagem {
			continue
		}
		// dentro do intervalo, a enxurrada é o MESMO gesto: só descarta
		// o acumulado e espera o próximo.
		if time.Since(b.ultimaRolagem) < intervaloRolagem {
			b.acumulado = 0
			continue
		}
		passo := 1
		if b.acumulado < 0 {
			passo = -1
		}
		b.acumulado = 0
		b.ultimaRolagem = time.Now()
		b.selectIndex(b.idx + passo)
	}
}

// abaEm diz qual aba está sob a coordenada X da tira. As larguras são
// guardadas durante o desenho: a tira é um Flex, e sem isso não há como
// saber onde cada aba começa.
func (b *tabBar) abaEm(x float32) int {
	acc := 0
	for i, w := range b.largurasAba {
		if int(x) >= acc && int(x) < acc+w {
			return i
		}
		acc += w
	}
	return -1
}
