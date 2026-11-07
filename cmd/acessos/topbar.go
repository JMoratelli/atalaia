package main

import (
	"image"

	"gio.tools/icons"
	"gioui.org/app"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

const topBarAltura = unit.Dp(36)

// topBar é a NOSSA titlebar. A decoração do sistema fica desligada
// (app.Decorated(false)) pra não gastar uma faixa inteira de altura só com
// o nome da janela: aqui a mesma faixa leva identidade, as ABAS, as
// ferramentas e os botões de janela.
//
// Desde a 2.9 é uma faixa só, na cor da moldura (ver moldura.go). Antes
// eram três — a barra da janela, a tira de abas e o banner do Painel —,
// cada uma de um material, e a tela lia como pedaços soltos. As abas
// subiram para cá, e "nova" e "INI" desceram para a barra do Painel, que é
// onde se mexe no inventário.
type topBar struct {
	menu widget.Clickable
	// sem botão "início" aqui: a aba fixa do Painel, logo ao lado, JÁ é
	// esse botão.
	snippets  widget.Clickable
	ajustes   widget.Clickable
	cofre     widget.Clickable
	chaveiro  widget.Clickable
	tema      widget.Clickable
	fonte     widget.Clickable
	minimizar widget.Clickable
	maximizar widget.Clickable
	fechar    widget.Clickable
}

type acoesTopo struct {
	menu        func()
	snippets    func()
	ajustes     func()
	abrirCofre  func()
	chaveiro    func()
	trocarTema  func()
	trocarFonte func()
}

// layout desenha o topo. inicioLinha é onde, em px, começa o fio da base:
// 0 sem lateral; com ela, depois do canto arredondado da folha (ou na
// própria borda, quando a aba ativa está colada nela). abas é a tira de
// abas, desenhada no meio da faixa, a partir da coluna da marca.
func (t *topBar) layout(gtx layout.Context, w *app.Window, th *material.Theme, inicioLinha int,
	a acoesTopo, abas layout.Widget) layout.Dimensions {

	for t.menu.Clicked(gtx) {
		a.menu()
	}
	for t.cofre.Clicked(gtx) {
		a.abrirCofre()
	}
	for t.chaveiro.Clicked(gtx) {
		a.chaveiro()
	}
	for t.snippets.Clicked(gtx) {
		a.snippets()
	}
	for t.ajustes.Clicked(gtx) {
		a.ajustes()
	}
	for t.tema.Clicked(gtx) {
		a.trocarTema()
	}
	for t.fonte.Clicked(gtx) {
		a.trocarFonte()
	}
	// ATENÇÃO: minimizar/maximizar/desmaximizar passam por foraDoQuadro.
	// w.Perform aqui é chamada de DENTRO do layout, com um quadro em voo —
	// no Windows isso congela a janela ao restaurar. O porquê inteiro está
	// em acaojanela.go; não volte a chamar w.Perform direto daqui.
	for t.minimizar.Clicked(gtx) {
		foraDoQuadro(func() { w.Perform(system.ActionMinimize) })
	}
	for t.maximizar.Clicked(gtx) {
		// janelaMaximizada vem do app.ConfigEvent (main.go), não de um
		// toggle próprio: encostar a janela numa borda da tela aciona o
		// snap-to-maximize do compositor sem passar por este botão, e um
		// bool só nosso ficava dessincronizado — o ícone continuava
		// mostrando "maximizar" numa janela já maximizada pelo KWin.
		if janelaMaximizada {
			foraDoQuadro(func() { w.Perform(system.ActionUnmaximize) })
		} else {
			foraDoQuadro(func() { w.Perform(system.ActionMaximize) })
		}
	}
	for t.fechar.Clicked(gtx) {
		// Fechar pode ir direto: no Windows ActionClose vira PostMessage,
		// que é assíncrono e não reentra no windowProc. É a única das
		// quatro que não precisa de foraDoQuadro.
		w.Perform(system.ActionClose)
	}

	h := gtx.Dp(topBarAltura)
	gtx.Constraints.Min.Y = h
	gtx.Constraints.Max.Y = h
	largura := gtx.Constraints.Max.X

	// O fundo é a própria moldura, já pintada na janela inteira. Só o fio
	// da base é daqui — e a aba ativa, desenhada depois, passa por cima
	// dele: é ali que a moldura vira folha.
	// Com a lateral à vista a linha começa depois do canto arredondado da
	// folha, que a lateral desenha (contornoFolhaLateral); sobre a coluna
	// da lateral não há linha — topo e lateral são a mesma moldura.
	paint.FillShape(gtx.Ops, tema.Contorno,
		clip.Rect{Min: image.Pt(inicioLinha, h-1), Max: image.Pt(largura, h)}.Op())

	// arrastar a janela pela barra: sem decoração do sistema, é daqui que
	// o compositor recebe o pedido de mover. Os botões e as abas, por
	// cima, ficam com o clique deles.
	area := clip.Rect{Max: image.Pt(largura, h)}.Push(gtx.Ops)
	system.ActionInputOp(system.ActionMove).Add(gtx.Ops)
	area.Pop()

	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		// A marca ocupa a coluna da lateral (aberta): as abas começam junto
		// com a folha e não se mexem quando a lateral abre e fecha.
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			d := naLinhaDasAbas(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ferramenta(gtx, &t.menu, icons.NavigationMenu, tema.TopoSec)
						}),
						layout.Rigid(layout.Spacer{Width: 4}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return marcaApp(gtx, gtx.Dp(20))
						}),
						layout.Rigid(layout.Spacer{Width: 8}.Layout),
						layout.Rigid(negrito(txt(th, fonteTitulo, unit.Sp(14), "Atalaia", tema.TopoTxt)).Layout),
						layout.Rigid(layout.Spacer{Width: 5}.Layout),
						layout.Rigid(txt(th, fonteSansMedia, unit.Sp(13.5), "Acessos", tema.TopoSec).Layout),
					)
				})
			})
			// A coluna da marca tem SEMPRE a largura da lateral aberta, esteja
			// ela aberta, recolhida ou oculta: as abas não pulam de lugar
			// quando o F12 abre e fecha a lateral, e com ela aberta a
			// primeira aba continua em cima da folha. A lateral tem a largura
			// da marca ("Atalaia Acessos"), então com ela oculta a primeira
			// aba não fica perdida longe da marca.
			if col := gtx.Dp(lateralLarga); col > d.Size.X {
				d.Size.X = col
			} else {
				d.Size.X += gtx.Dp(4)
			}
			return d
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.Y = h
			// As abas não passam por baixo das ferramentas quando são
			// muitas: o que não cabe fica cortado aqui.
			// (o recorte começa abaCurva antes: é onde cai a curva esquerda da
			// primeira aba, encostada na coluna da marca)
			defer clip.Rect{Min: image.Pt(-gtx.Dp(abaCurva), 0), Max: image.Pt(gtx.Constraints.Max.X, h)}.Push(gtx.Ops).Pop()
			d := abas(gtx)
			d.Size.X = gtx.Constraints.Max.X
			return d
		}),
		// ferramentas e botões de janela no meio da FAIXA (só a marca vai
		// na linha das abas)
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return noMeioDaFaixa(gtx, func(gtx layout.Context) layout.Dimensions {
				return capsula(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ferramenta(gtx, &t.snippets, icons.ActionCode, tema.TopoSec)
					}),
					// chaveiro e cofre juntos: o cofre é a chave, o chaveiro é
					// o que ela abre. O cadeado é indicador E botão: aberto
					// (verde) = cofre destrancado nesta sessão.
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ferramenta(gtx, &t.chaveiro, icons.CommunicationVPNKey, tema.TopoSec)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						ic, cor := icons.ActionLock, tema.TopoSec
						if cofreAberto != nil {
							ic, cor = icons.ActionLockOpen, tema.Verde
						}
						return ferramenta(gtx, &t.cofre, ic, cor)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return divisoria(gtx, tema.Borda2)
					}),
					// A+ junto do tema porque os dois são a mesma pergunta:
					// como o app se apresenta. O rótulo mostra o nível corrente
					// (A+, A++) para o clique não ser adivinhação.
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ferramentaTexto(gtx, th, &t.fonte, rotuloFonte())
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ferramenta(gtx, &t.tema, icons.ImageBrightness6, tema.TopoSec)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ferramenta(gtx, &t.ajustes, icons.ActionSettings, tema.TopoSec)
					}),
				)
			})
		}),
		layout.Rigid(layout.Spacer{Width: 10}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return noMeioDaFaixa(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return divisoria(gtx, tema.MolduraLinha)
					}),
					layout.Rigid(layout.Spacer{Width: 4}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return botaoJanela(gtx, &t.minimizar, icons.ContentRemove)
					}),
					layout.Rigid(layout.Spacer{Width: 2}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						ic := icons.ToggleCheckBoxOutlineBlank
						if janelaMaximizada {
							ic = icons.ActionFlipToFront
						}
						return botaoJanela(gtx, &t.maximizar, ic)
					}),
					layout.Rigid(layout.Spacer{Width: 2}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return botaoJanelaFechar(gtx, &t.fechar)
					}),
					layout.Rigid(layout.Spacer{Width: 6}.Layout),
				)
			})
		}),
	)
}

// botaoBarra é o botão de texto das barras de ação ("limpar seleção",
// "+ somar à fila"): desde a 2.9 o mesmo desenho do botão da barra da
// sessão (barrasessao.go), para as barras não terem cada uma o seu.
func botaoBarra(gtx layout.Context, th *material.Theme, btn *widget.Clickable, rot string) layout.Dimensions {
	return botaoSessao(gtx, th, btn, rot)
}

// naLinhaDasAbas centraliza w na altura das ABAS, e não na da faixa: as
// abas ficam encostadas embaixo do topo, e a marca e os botões alinhados
// pelo meio da faixa inteira ficavam visivelmente acima delas.
func naLinhaDasAbas(gtx layout.Context, w layout.Widget) layout.Dimensions {
	gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
	return layout.Inset{Top: topBarAltura - abaAltura}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return centroV(gtx, w)
	})
}

// noMeioDaFaixa centraliza w na altura toda do topo.
func noMeioDaFaixa(gtx layout.Context, w layout.Widget) layout.Dimensions {
	gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
	return centroV(gtx, w)
}
