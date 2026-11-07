package main

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// A barra de sessão é a faixa logo abaixo das abas, presente só nas abas
// de conexão: à esquerda o estado (chip + destino), à direita os controles
// da sessão e a geometria. É a mesma barra do app original (a `self.barra`
// de AbaBase em acessos.py): o operador precisa ver, sem abrir menu
// nenhum, se a sessão está viva, para onde ela aponta e em que resolução.
const barraSessaoAltura = unit.Dp(32)

// estadoSessao é o que a barra mostra à esquerda e à direita. Tipo é uma
// das chaves de corChip.
type estadoSessao struct {
	Chip  string // "ATIVO", "AGUARDE", "CAIU"
	Tipo  string // ok | atencao | erro | neutro
	Texto string // destino: 192.168.8.132:5900
	Geo   string // "1920x1080 1:1", "80x24"…
	// Aviso é uma nota passageira ("clipboard recebido da sessão"),
	// vazia quase sempre. Sai ao lado do destino, em verde.
	Aviso string
}

// abaSessao é implementada pelas abas que têm sessão remota. As que não
// têm (Painel) simplesmente não ganham a barra.
type abaSessao interface {
	EstadoSessao() estadoSessao
	ControlesSessao(gtx layout.Context, th *material.Theme) layout.Dimensions
}

func corChip(tipo string) (fundo, texto color.NRGBA) {
	switch tipo {
	case "ok":
		return tema.OkBg, tema.OkFg
	case "erro":
		return tema.ErroBg, tema.ErroFg
	case "atencao":
		return tema.AtencaoBg, tema.AtencaoFg
	}
	return tema.Vidro2, tema.Sec
}

// layoutBarraSessao desenha a barra da sessão. Desde a 2.9 ela não tem
// fundo próprio: fica na folha, logo abaixo da aba ativa, que desce direto
// nela — a aba e a barra são uma peça só. Da esquerda para a direita: o
// estado (pílula), o destino, a geometria e, encostados à direita, os
// controles da sessão e os extras (tela cheia).
func layoutBarraSessao(gtx layout.Context, th *material.Theme, a abaSessao,
	extras ...layout.Widget) layout.Dimensions {
	h := gtx.Dp(barraSessaoAltura)
	gtx.Constraints.Min.Y = h
	gtx.Constraints.Max.Y = h

	e := a.EstadoSessao()
	return layout.Inset{Left: 14, Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return centroV(gtx, func(gtx layout.Context) layout.Dimensions {
			return itensBarraSessao(gtx, th, a, e, extras)
		})
	})
}

func itensBarraSessao(gtx layout.Context, th *material.Theme, a abaSessao, e estadoSessao,
	extras []layout.Widget) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return chipEstado(gtx, th, e.Chip, e.Tipo)
		}),
		layout.Rigid(layout.Spacer{Width: 12}.Layout),
		// Destino e geometria ficam na parte que ENCOLHE (Flexed é medido
		// depois dos Rigid): numa barra estreita é o texto que se corta,
		// nunca os botões da direita — entre eles os de janela, o canto que
		// o operador procura para fechar.
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(txt(th, fonteMono, unit.Sp(12), e.Texto, tema.Texto).Layout),
				layout.Rigid(layout.Spacer{Width: 12}.Layout),
				layout.Rigid(txt(th, fonteMono, unit.Sp(11.5), e.Geo, tema.Fraco).Layout),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if e.Aviso == "" {
				return layout.Dimensions{}
			}
			return layout.Inset{Right: 12}.Layout(gtx,
				txt(th, fonteMono, spSecundario, e.Aviso, tema.OkFg).Layout)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.ControlesSessao(gtx, th)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if len(extras) == 0 {
				return layout.Dimensions{}
			}
			filhos := make([]layout.FlexChild, 0, len(extras)*2)
			for _, x := range extras {
				filhos = append(filhos,
					layout.Rigid(layout.Spacer{Width: 6}.Layout),
					layout.Rigid(x))
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.
				Layout(gtx, filhos...)
		}),
	)
}

// chipEstado é a pílula do estado da sessão: ponto e rótulo na cor do
// estado, sobre a mesma cor bem clara.
func chipEstado(gtx layout.Context, th *material.Theme, texto, tipo string) layout.Dimensions {
	fundo, cor := corChip(tipo)
	return layout.Background{}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			sz := gtx.Constraints.Min
			paint.FillShape(gtx.Ops, fundo, clip.UniformRRect(image.Rectangle{Max: sz}, sz.Y/2).Op(gtx.Ops))
			return layout.Dimensions{Size: sz}
		},
		func(gtx layout.Context) layout.Dimensions {
			alt := gtx.Dp(20)
			gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = alt, alt
			return layout.Inset{Left: 8, Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return centroV(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							lado := gtx.Dp(7)
							paint.FillShape(gtx.Ops, cor, clip.Ellipse{Max: image.Pt(lado, lado)}.Op(gtx.Ops))
							return layout.Dimensions{Size: image.Pt(lado, lado)}
						}),
						layout.Rigid(layout.Spacer{Width: 7}.Layout),
						layout.Rigid(negrito(txt(th, fonteMono, unit.Sp(10), texto, cor)).Layout),
					)
				})
			})
		},
	)
}

// botaoSessao: o botão de texto da barra da sessão ("Reconectar",
// "Snippets"…), no vidro da folha com borda, 24 de altura — a barra é
// fina de propósito (quem quiser maior usa o A+, que escala tudo).
func botaoSessao(gtx layout.Context, th *material.Theme, btn *widget.Clickable, rot string) layout.Dimensions {
	return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		alt := gtx.Dp(24)
		gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = alt, alt
		return layout.Background{}.Layout(gtx,
			func(gtx layout.Context) layout.Dimensions {
				fundo := tema.CardTinta
				if btn.Hovered() {
					fundo = tema.CardTintaH
				}
				superficie(gtx, gtx.Constraints.Min, fundo, tema.CardBorda, 8)
				return layout.Dimensions{Size: gtx.Constraints.Min}
			},
			func(gtx layout.Context) layout.Dimensions {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: 10, Right: 10}.Layout(gtx,
						txt(th, fonteSansMedia, unit.Sp(12), rot, tema.Texto).Layout)
				})
			},
		)
	})
}

// toggleSessao: liga/desliga só de ícone (reconectar sozinho, área de
// transferência). Ligado, ganha o fundo e o ícone na cor dele.
func toggleSessao(gtx layout.Context, th *material.Theme, btn *widget.Clickable,
	ic *widget.Icon, ligado bool, corLigado color.NRGBA) layout.Dimensions {

	return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		size := image.Pt(gtx.Dp(28), gtx.Dp(24))
		gtx.Constraints.Min, gtx.Constraints.Max = size, size

		fundo, cor := transparente, tema.Fraco
		if ligado {
			fundo, cor = comAlfa(corLigado, 0.15), corLigado
		}
		if btn.Hovered() && !ligado {
			fundo, cor = tema.Elevado, tema.Sec
		}
		superficie(gtx, size, fundo, transparente, 8)
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return icone(gtx, ic, cor, 15)
		})
	})
}

// segmentado é a escolha exclusiva entre rótulos (modo do VNC, opções dos
// diálogos): o trilho da repaginação (moldura.go), com o escolhido em
// relevo.
func segmentado(gtx layout.Context, th *material.Theme, btns []*widget.Clickable,
	rotulos []string, escolhido int) layout.Dimensions {

	return trilho(gtx, len(rotulos), escolhido, btns, func(gtx layout.Context, i int, ativo, hover bool) layout.Dimensions {
		cor := tema.Sec
		f := fonteSansMedia
		if ativo {
			cor = tema.Texto
			f = fonteSans
			f.Weight = font.SemiBold
		} else if hover {
			cor = tema.Texto
		}
		return layout.Inset{Top: 3, Bottom: 3, Left: 10, Right: 10}.Layout(gtx,
			txt(th, f, unit.Sp(12), rotulos[i], cor).Layout)
	})
}
