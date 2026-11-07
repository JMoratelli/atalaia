package main

import (
	"fmt"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// Colar várias linhas num shell sem colagem protegida (bracketed paste)
// RODA cada linha assim que ela chega — copiar um bloco de um chamado ou
// de uma página e colar no terminal de um PDV executa tudo, inclusive o
// que não se queria. Shell novo pede a colagem protegida e não tem esse
// problema; o de PDV antigo, não. Então, quando o programa não pediu e o
// texto tem mais de uma linha, o app pergunta antes, mostrando o começo do
// que vai — como fazem o Windows Terminal e o iTerm2.

// linhasDoColar conta as linhas que vão virar Enter: o texto com a quebra
// do fim aparada (a última linha sem Enter não executa sozinha).
func linhasDoColar(texto string) int {
	t := strings.TrimRight(strings.ReplaceAll(texto, "\r\n", "\n"), "\n")
	if t == "" {
		return 0
	}
	return strings.Count(t, "\n") + 1
}

type dlgColarLinhas struct {
	texto    string
	n        int
	colar    func()
	btnColar widget.Clickable
	btnCanc  widget.Clickable
}

func (d *dlgColarLinhas) Titulo() string   { return fmt.Sprintf("Colar %d linhas?", d.n) }
func (d *dlgColarLinhas) Largura() unit.Dp { return 520 }

func (d *dlgColarLinhas) Corpo(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if d.btnCanc.Clicked(gtx) {
		fecharDialogo()
	}
	if d.btnColar.Clicked(gtx) {
		fecharDialogo()
		d.colar()
	}
	linhas := strings.Split(strings.TrimRight(strings.ReplaceAll(d.texto, "\r\n", "\n"), "\n"), "\n")
	mostrar := linhas
	if len(mostrar) > 6 {
		mostrar = append(append([]string{}, linhas[:6]...), fmt.Sprintf("… e mais %d", len(linhas)-6))
	}
	filhos := []layout.FlexChild{
		layout.Rigid(rotulo(th, fonteSans, spRotulo,
			"Este programa não pediu colagem protegida: no shell, cada linha roda como comando assim que chega.", tema.Sec)),
		espaco(10),
	}
	for _, l := range mostrar {
		filhos = append(filhos, layout.Rigid(rotuloLinha(th, fonteMono, spCorpo, l, tema.Texto)))
	}
	filhos = append(filhos,
		espaco(14),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Dimensions{Size: gtx.Constraints.Min}
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return botaoNeutro(gtx, th, &d.btnCanc, "Cancelar")
				}),
				layout.Rigid(layout.Spacer{Width: 8}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return botaoPrimario(gtx, th, &d.btnColar, fmt.Sprintf("Colar %d linhas", d.n))
				}),
			)
		}),
	)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, filhos...)
}
