package main

import (
	"fmt"
	"image"
	"image/color"
	"strings"

	"acessos-go/internal/conexoes"

	"gio.tools/icons"
	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
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
	lateralLarga = unit.Dp(196)
	// lateralLarga é também a coluna da marca no topo (topbar.go): as abas
	// começam na borda dela, abra ou feche a lateral. UMA calha só pra toda
	// a coluna: busca, itens, árvore e o pé começam na mesma vertical.
	padColuna     = unit.Dp(12)
	lateralTrilho = unit.Dp(60) // recolhida vira TRILHO de ícones, nunca some
)

// itemLateral é uma entrada do menu. sid identifica a seção; o clique abre
// (ou foca) a aba correspondente — padrão B: o item do menu ABRE uma aba
// que permanece aberta e fechável.
type itemLateral struct {
	sid    string
	rotulo string
	ic     *widget.Icon
	btn    widget.Clickable
}

// estadoLateral: larga -> trilho de ícones -> oculta. A skill diz pra
// nunca sumir por completo; o terceiro estado entra porque foi pedido
// explicitamente (tela cheia de operação, onde cada pixel conta). O menu
// da barra de topo continua trazendo a lateral de volta, então não há
// beco sem saída.
type estadoLateral int

const (
	lateralAberta estadoLateral = iota
	lateralRecolhida
	lateralOculta
)

// sidebar é o menu lateral, SEMPRE vertical (quando visível). O botão de
// recolher mora na própria lateral, no rodapé — nunca na barra de abas,
// que rola (gtk.md, padrão B).
type sidebar struct {
	estado      estadoLateral
	itens       []*itemLateral
	collapseBtn widget.Clickable

	// A lista de máquinas mora aqui, como no app original: grupo ->
	// subgrupo -> host, e o clique no host abre a conexão no protocolo
	// escolhido na tira de baixo (Tela/Shell/RDP/Arquivos).
	painel   *dashTab
	lista    widget.List
	expandir map[string]bool
	cliques  map[string]*widget.Clickable
	proto    conexoes.Protocolo
	btnProto [4]widget.Clickable

	// menu de contexto e clique do meio: mesma técnica do Painel — uma
	// área só, por cima, com PassOp, e quem está sob o cursor sai do
	// hover (ver dashtab.rastrearPonteiro).
	tagPonteiro *int
	ultimaPos   image.Point
	sobCursor   *conexoes.Conexao
	grupoSob    *conexoes.Grupo
	aoMenuHost  func(cx conexoes.Conexao, pos image.Point)
	// abaColada: a aba ativa do topo começa na borda direita desta lateral
	// e desce reta (ver formaAbaAtiva) — aí a borda da folha sobe reta até
	// ela, sem o canto arredondado.
	abaColada   bool
	aoMenuGrupo func(g *conexoes.Grupo, pos image.Point)
}

func newSidebar() *sidebar {
	s := &sidebar{
		itens: []*itemLateral{
			{sid: "painel", rotulo: "Painel", ic: icons.ActionDashboard},
		},
		expandir:    map[string]bool{},
		cliques:     map[string]*widget.Clickable{},
		proto:       conexoes.VNC,
		tagPonteiro: new(int),
	}
	s.lista.Axis = layout.Vertical
	// Nasce como estava quando o app foi fechado: quem trabalha com a
	// lateral escondida não quer reabri-la toda manhã. Mesma chave (e
	// mesmo significado) do app original: [geral] lateral = 0 | 1.
	if lateralInicialOculta {
		s.estado = lateralOculta
	}
	return s
}

// lateralInicialOculta é lido do .ini no start (ver main.go).
var lateralInicialOculta bool

// lembrarLateral grava o estado no [geral] do inventário. Grava na hora
// do clique, e não ao sair: fechar o app pelo botão da janela — ou uma
// queda — não pode custar a preferência.
func lembrarLateral(oculta bool) {
	lembrarGeral("lateral", simNao(!oculta))
}

func (s *sidebar) clique(chave string) *widget.Clickable {
	c, ok := s.cliques[chave]
	if !ok {
		c = &widget.Clickable{}
		s.cliques[chave] = c
	}
	return c
}

// ciclar alterna aberta <-> oculta. É UM clique: a lateral aqui não é um
// menu de seções (o app tem uma seção só, o Painel) e sim a LISTA das
// máquinas — um trilho de ícones no meio do caminho não mostraria nada
// útil e só atrasaria quem quer a tela cheia.
func (s *sidebar) ciclar() {
	if s.estado == lateralAberta {
		s.estado = lateralOculta
	} else {
		s.estado = lateralAberta
	}
	lembrarLateral(s.estado == lateralOculta)
}

func (s *sidebar) recolhida() bool { return s.estado == lateralRecolhida }
func (s *sidebar) oculta() bool    { return s.estado == lateralOculta }

func (s *sidebar) largura(gtx layout.Context) int {
	switch s.estado {
	case lateralOculta:
		return 0
	case lateralRecolhida:
		return gtx.Dp(lateralTrilho)
	}
	return gtx.Dp(lateralLarga)
}

// layout desenha a lateral. aoAbrir recebe o sid do item clicado.
//
// Desde a 2.9 a lateral não tem fundo próprio: ela é a MOLDURA (pintada na
// janela inteira), do mesmo tom do topo, e a folha começa logo à direita
// dela, com o canto de cima arredondado. O item ativo corre até essa borda
// e se funde à folha, como o menu da central do ChamadoJá. A marca do app
// subiu para o topo, que alinha a coluna dela com a desta lateral.
func (s *sidebar) layout(gtx layout.Context, th *material.Theme, ativo string,
	aoAbrir func(sid string), busca layout.Widget) layout.Dimensions {

	if s.oculta() {
		return layout.Dimensions{}
	}
	for _, it := range s.itens {
		if it.btn.Clicked(gtx) {
			aoAbrir(it.sid)
			gtx.Execute(key.FocusCmd{Tag: nil})
		}
	}
	for s.collapseBtn.Clicked(gtx) {
		s.ciclar()
		gtx.Execute(key.FocusCmd{Tag: nil})
	}

	s.sobCursor, s.grupoSob = nil, nil
	defer s.rastrear(gtx)

	w := s.largura(gtx)
	gtx.Constraints.Min.X = w
	gtx.Constraints.Max.X = w
	// A borda da folha corre na última coluna desta lateral, POR BAIXO dos
	// itens: o item ativo a cobre e se abre para a folha.
	raio := gtx.Dp(folhaRaio)
	if s.abaColada {
		raio = 0
	}
	contornoFolhaLateral(gtx, w, gtx.Constraints.Max.Y, raio)

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if s.recolhida() || busca == nil {
				return layout.Spacer{Height: 12}.Layout(gtx)
			}
			return layout.Inset{Top: 12, Left: padColuna, Right: padColuna}.Layout(gtx, busca)
		}),
		layout.Rigid(layout.Spacer{Height: 14}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			var filhos []layout.FlexChild
			for _, it := range s.itens {
				it := it
				filhos = append(filhos, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return s.item(gtx, th, it, it.sid == ativo)
				}))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, filhos...)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if s.recolhida() || s.painel == nil {
				return layout.Dimensions{}
			}
			return layout.Inset{Top: 20, Bottom: 6, Left: padColuna + 2}.Layout(gtx,
				txt(th, fonteMono, unit.Sp(10.5), "MÁQUINAS", tema.TopoSec).Layout)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return s.arvore(gtx, th)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			// o pé da lateral (protocolo do clique e recolher), separado
			// da árvore por um fio da moldura
			return layout.Inset{Left: padColuna, Right: padColuna}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				paint.FillShape(gtx.Ops, tema.MolduraLinha, clip.Rect{Max: image.Pt(gtx.Constraints.Max.X, 1)}.Op())
				return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 1)}
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return s.tiraProtocolo(gtx, th) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return s.botaoRecolher(gtx, th) }),
	)
}

// item é uma entrada de navegação da lateral (hoje só "Painel"). O ativo
// é pintado com a folha e vai até a borda direita, onde se funde a ela
// (formaItemAtivo); o fundo fica fora do Clickable, que recortaria as
// curvas.
func (s *sidebar) item(gtx layout.Context, th *material.Theme, it *itemLateral, ativo bool) layout.Dimensions {
	esq := padColuna
	if s.recolhida() {
		esq = 8
	}
	return layout.Inset{Left: esq}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		conteudo := op.Record(gtx.Ops)
		d := it.btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			pointer.CursorPointer.Add(gtx.Ops)
			alt := gtx.Dp(40)
			gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = alt, alt
			corIc, cor := tema.TopoSec, tema.TopoSec
			f := fonteSansMedia
			switch {
			case ativo:
				corIc, cor = tema.Azul, tema.Texto
				f = fonteSans
				f.Weight = font.SemiBold
			case it.btn.Hovered():
				corIc, cor = tema.TopoTxt, tema.TopoTxt
			}
			if s.recolhida() {
				return centroV(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: 13}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return icone(gtx, it.ic, corIc, 20)
					})
				})
			}
			return centroV(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return icone(gtx, it.ic, corIc, 19)
						}),
						layout.Rigid(layout.Spacer{Width: 10}.Layout),
						layout.Rigid(txt(th, f, unit.Sp(14), it.rotulo, cor).Layout),
					)
				})
			})
		})
		desenho := conteudo.Stop()
		switch {
		case ativo:
			forma := clip.Outline{Path: formaItemAtivo(gtx, d.Size)}.Op().Push(gtx.Ops)
			pintarFolha(gtx)
			forma.Pop()
			paint.FillShape(gtx.Ops, tema.Contorno,
				clip.Stroke{Path: contornoItemAtivo(gtx, d.Size), Width: 1}.Op())
		case it.btn.Hovered():
			r := gtx.Dp(12)
			rr := clip.RRect{Rect: image.Rectangle{Max: image.Pt(d.Size.X-gtx.Dp(padColuna), d.Size.Y)}, NW: r, SW: r, NE: r, SE: r}
			paint.FillShape(gtx.Ops, tinta(tema.TopoTxt, 0.06), rr.Op(gtx.Ops))
		}
		desenho.Add(gtx.Ops)
		return d
	})
}

func (s *sidebar) botaoRecolher(gtx layout.Context, th *material.Theme) layout.Dimensions {
	ic := icons.NavigationChevronLeft
	rot := "Recolher a lateral   F12"
	if s.recolhida() {
		ic = icons.NavigationChevronRight
		rot = ""
	}
	cor := tema.TopoSec
	if s.collapseBtn.Hovered() {
		cor = tema.TopoTxt
	}

	return layout.Inset{Top: 4, Bottom: 8, Left: padColuna, Right: padColuna}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return s.collapseBtn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			pointer.CursorPointer.Add(gtx.Ops)
			return layout.Stack{}.Layout(gtx,
				layout.Expanded(func(gtx layout.Context) layout.Dimensions {
					size := gtx.Constraints.Min
					if s.collapseBtn.Hovered() {
						superficie(gtx, size, tinta(tema.TopoTxt, 0.06), transparente, 9)
					}
					return layout.Dimensions{Size: size}
				}),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					if s.recolhida() {
						return layout.Inset{Top: 7, Bottom: 7}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return icone(gtx, ic, cor, 18)
							})
						})
					}
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return layout.Inset{Top: 7, Bottom: 7, Left: 8, Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return icone(gtx, ic, cor, 18)
							}),
							layout.Rigid(layout.Spacer{Width: 8}.Layout),
							layout.Rigid(txt(th, fonteSansMedia, unit.Sp(12.5), rot, cor).Layout),
						)
					})
				}),
			)
		})
	})
}

var _ color.NRGBA

// ------------------------------------------------ lista de máquinas

const (
	linhaLateralAlt = unit.Dp(13) // recuo por nível da árvore
)

// arvore desenha grupo -> subgrupo -> host, rolável, filtrada pelo mesmo
// termo do campo de busca (o do painel e o da lateral são espelhados).
func (s *sidebar) arvore(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if s.painel == nil || s.recolhida() {
		return layout.Dimensions{Size: gtx.Constraints.Min}
	}
	termo := strings.ToLower(strings.TrimSpace(s.painel.ultimoTermo))
	raizes := s.painel.arvore
	return material.List(th, &s.lista).Layout(gtx, len(raizes), func(gtx layout.Context, i int) layout.Dimensions {
		return s.ramo(gtx, th, raizes[i], 0, termo)
	})
}

func (s *sidebar) ramo(gtx layout.Context, th *material.Theme, g *conexoes.Grupo, nivel int, termo string) layout.Dimensions {
	chave := strings.Join(g.Caminho, ";")
	visiveis := filtrar(g.Conexoes, termo)

	aberto := s.expandir[chave]
	if termo != "" {
		if len(visiveis) == 0 && !temDescendente(g, termo) {
			return layout.Dimensions{}
		}
		aberto = true
	}

	btn := s.clique("g:" + chave)
	if btn.Clicked(gtx) {
		s.expandir[chave] = !s.expandir[chave]
	}
	if btn.Hovered() {
		s.grupoSob = g
	}

	filhos := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return s.linhaGrupo(gtx, th, g, nivel, aberto, btn.Hovered())
			})
		}),
	}
	if aberto {
		for _, f := range g.Filhos {
			f := f
			filhos = append(filhos, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return s.ramo(gtx, th, f, nivel+1, termo)
			}))
		}
		for _, cx := range visiveis {
			cx := cx
			filhos = append(filhos, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return s.linhaHost(gtx, th, cx, nivel+1)
			}))
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, filhos...)
}

func (s *sidebar) linhaGrupo(gtx layout.Context, th *material.Theme, g *conexoes.Grupo, nivel int, aberto, hover bool) layout.Dimensions {
	seta := setaExpansor(aberto)
	cor := tema.TopoTxt
	f := fonteSans
	f.Weight = font.SemiBold
	if nivel > 0 {
		cor = tema.TopoSec
		if aberto {
			cor = tema.TopoTxt
		}
	}
	if hover {
		cor = tema.TopoTxt
	}
	return layout.Inset{Left: padColuna, Right: padColuna}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				if hover {
					superficie(gtx, gtx.Constraints.Min, tinta(tema.TopoTxt, 0.06), transparente, 9)
				}
				return layout.Dimensions{Size: gtx.Constraints.Min}
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				alt := gtx.Dp(30)
				gtx.Constraints.Min.Y = alt
				return layout.Inset{
					Right: 6,
					Left:  unit.Dp(6) + unit.Dp(float32(nivel))*linhaLateralAlt,
				}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return centroV(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return icone(gtx, seta, cor, 15)
							}),
							layout.Rigid(layout.Spacer{Width: 6}.Layout),
							layout.Flexed(1, rotulo(th, f, unit.Sp(13), g.Nome, cor)),
							layout.Rigid(layout.Spacer{Width: 6}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return contador(gtx, th, fmt.Sprint(g.Total()))
							}),
						)
					})
				})
			}),
		)
	})
}

// linhaHost é a máquina na árvore da lateral: o ponto de vida (o mesmo do
// card), o nome e, à direita, o protocolo que o clique vai abrir — na cor
// dele quando a máquina tem o escolhido na tira de baixo, apagado quando o
// clique vai cair no primeiro que ela tiver.
func (s *sidebar) linhaHost(gtx layout.Context, th *material.Theme, cx conexoes.Conexao, nivel int) layout.Dimensions {
	btn := s.clique("h:" + cx.GrupoStr() + "|" + cx.Nome)
	p := s.proto
	if !cx.Tem(p) {
		p = conexoes.Protocolo("")
		for _, e := range protocolos {
			if cx.Tem(e.p) {
				p = e.p
				break
			}
		}
	}
	if btn.Clicked(gtx) && s.painel != nil && p != "" {
		s.painel.abrir(cx, p)
	}
	if btn.Hovered() {
		cx := cx
		s.sobCursor = &cx
		pedirDica("lateral:"+cx.GrupoStr()+"|"+cx.Nome, cx.Descricao)
	}
	var rot string
	corTag := tema.TopoSec
	if p != "" {
		e := estiloDe(p)
		rot = e.rotulo
		if p == s.proto {
			corTag = e.cor()
		}
	}
	cor := tema.TopoSec
	if btn.Hovered() {
		cor = tema.TopoTxt
	}
	var corVida color.NRGBA
	mostraVida := false
	if s.painel != nil {
		corVida, mostraVida = corDaVida(s.painel.vidaConhecida(cx))
	}

	return layout.Inset{Left: padColuna, Right: padColuna}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			pointer.CursorPointer.Add(gtx.Ops)
			return layout.Stack{}.Layout(gtx,
				layout.Expanded(func(gtx layout.Context) layout.Dimensions {
					if btn.Hovered() {
						superficie(gtx, gtx.Constraints.Min, tinta(tema.TopoTxt, 0.06), transparente, 9)
					}
					return layout.Dimensions{Size: gtx.Constraints.Min}
				}),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					gtx.Constraints.Min.Y = gtx.Dp(28)
					return layout.Inset{
						Right: 8,
						Left:  unit.Dp(10) + unit.Dp(float32(nivel))*linhaLateralAlt,
					}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return centroV(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return pontoVida(gtx, corVida, mostraVida)
								}),
								layout.Rigid(layout.Spacer{Width: 8}.Layout),
								layout.Flexed(1, rotulo(th, fonteSans, unit.Sp(13), cx.Nome, cor)),
								layout.Rigid(layout.Spacer{Width: 6}.Layout),
								layout.Rigid(rotulo(th, fonteMono, unit.Sp(10.5), rot, corTag)),
							)
						})
					})
				}),
			)
		})
	})
}

// tiraProtocolo escolhe o que o clique numa máquina da lateral abre. É um
// trilho (moldura.go) com o ícone de cada protocolo; o escolhido fica em
// relevo, na cor dele.
func (s *sidebar) tiraProtocolo(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if s.recolhida() || s.painel == nil {
		return layout.Dimensions{}
	}
	escolhido := 0
	btns := make([]*widget.Clickable, len(protocolos))
	for i, e := range protocolos {
		btns[i] = &s.btnProto[i]
		if s.btnProto[i].Clicked(gtx) {
			s.proto = e.p
		}
		if s.proto == e.p {
			escolhido = i
		}
	}
	return layout.Inset{Top: 10, Bottom: 2, Left: padColuna, Right: padColuna}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: 2, Bottom: 6}.Layout(gtx,
					txt(th, fonteSansMedia, unit.Sp(11.5), "Clique na máquina abre em", tema.TopoSec).Layout)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				// os quatro segmentos dividem a largura toda
				larg := (gtx.Constraints.Max.X - gtx.Dp(6) - 3*gtx.Dp(3)) / len(protocolos)
				return trilho(gtx, len(protocolos), escolhido, btns, func(gtx layout.Context, i int, ativo, hover bool) layout.Dimensions {
					e := protocolos[i]
					size := image.Pt(larg, gtx.Dp(28))
					gtx.Constraints.Min, gtx.Constraints.Max = size, size
					cor := tema.TopoSec
					if ativo {
						cor = e.cor()
					} else if hover {
						cor = tema.TopoTxt
					}
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return icone(gtx, e.ic, cor, 17)
					})
				})
			}),
		)
	})
}

// rastrear registra a área por cima da lateral (PassOp) para saber onde
// está o ponteiro e detectar os cliques do meio e direito.
func (s *sidebar) rastrear(gtx layout.Context) {
	for {
		ev, ok := gtx.Source.Event(pointer.Filter{
			Target: s.tagPonteiro,
			Kinds:  pointer.Move | pointer.Press | pointer.Drag,
		})
		if !ok {
			break
		}
		pe, isP := ev.(pointer.Event)
		if !isP {
			continue
		}
		s.ultimaPos = image.Pt(int(pe.Position.X), int(pe.Position.Y))
		if pe.Kind != pointer.Press {
			continue
		}
		meio := pe.Buttons.Contain(pointer.ButtonTertiary)
		dir := pe.Buttons.Contain(pointer.ButtonSecondary)
		if !meio && !dir {
			continue
		}
		pos := s.ultimaPos.Add(image.Pt(0, alturaTopoAtual))
		switch {
		case s.sobCursor != nil && dir && s.aoMenuHost != nil:
			s.aoMenuHost(*s.sobCursor, pos)
		case s.sobCursor != nil && meio:
			// clique do meio na máquina: abre o primeiro protocolo em
			// segundo plano, como no app original.
			if p, ok := protocoloPadrao(*s.sobCursor); ok && s.painel != nil {
				s.painel.abrirEm(*s.sobCursor, p, false)
			}
		case s.grupoSob != nil && (meio || dir) && s.aoMenuGrupo != nil:
			s.aoMenuGrupo(s.grupoSob, pos)
		}
	}
	pass := pointer.PassOp{}.Push(gtx.Ops)
	area := clip.Rect{Max: image.Pt(s.largura(gtx), gtx.Constraints.Max.Y)}.Push(gtx.Ops)
	event.Op(gtx.Ops, s.tagPonteiro)
	area.Pop()
	pass.Pop()
}
