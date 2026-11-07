package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	"acessos-go/internal/atualizador"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// dlgSobre mostra o que o metainfo embutido já sabe: mesma fonte de
// verdade da versão (versao.go), para nome, resumo, licença e link do
// repositório nunca divergirem do que a loja/Flathub mostra.
type dlgSobre struct {
	btnFechar   widget.Clickable
	btnHomepage widget.Clickable
	btnAvisos   widget.Clickable
}

var (
	reResumo   = regexp.MustCompile(`<summary>([^<]+)</summary>`)
	reHomepage = regexp.MustCompile(`<url type="homepage">([^<]+)</url>`)
	reLicenca  = regexp.MustCompile(`<project_license>([^<]+)</project_license>`)
)

func abrirSobre() {
	abrirDialogo(&dlgSobre{})
}

func (d *dlgSobre) Titulo() string   { return "Sobre" }
func (d *dlgSobre) Largura() unit.Dp { return 400 }

func (d *dlgSobre) Corpo(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if d.btnFechar.Clicked(gtx) {
		fecharDialogo()
	}
	homepage := extraiMeta(reHomepage)
	if d.btnHomepage.Clicked(gtx) {
		abrirNoNavegador(homepage)
	}
	// Os avisos de terceiros vão também dentro do pacote (ao lado do .exe
	// no Windows, em share/licenses no Flatpak); o link é o caminho curto.
	avisos := strings.TrimSuffix(homepage, "/") + "/blob/master/THIRD-PARTY-NOTICES.md"
	if d.btnAvisos.Clicked(gtx) {
		abrirNoNavegador(avisos)
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// a marca grande, o nome como no topo e a versão
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return marcaApp(gtx, gtx.Dp(52))
				}),
				layout.Rigid(layout.Spacer{Width: 14}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
								layout.Rigid(negrito(txt(th, fonteTitulo, unit.Sp(20), "Atalaia", tema.Texto)).Layout),
								layout.Rigid(layout.Spacer{Width: 6}.Layout),
								layout.Rigid(txt(th, fonteSansMedia, unit.Sp(18), "Acessos", tema.Sec).Layout),
							)
						}),
						layout.Rigid(layout.Spacer{Height: 2}.Layout),
						layout.Rigid(rotulo(th, fonteMono, unit.Sp(12), "versão "+versaoInstalada(), tema.Sec)),
					)
				}),
			)
		}),
		espaco(14),
		layout.Rigid(rotulo(th, fonteSans, spRotulo, extraiMeta(reResumo), tema.Sec)),
		espaco(14),
		layout.Rigid(rotulo(th, fonteMono, unit.Sp(11), "Licença: "+extraiMeta(reLicenca), tema.Fraco)),
		espaco(4),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return d.btnHomepage.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				cor := tema.Azul
				if d.btnHomepage.Hovered() {
					cor = tema.AzulH
				}
				return rotulo(th, fonteMono, unit.Sp(11), homepage, cor)(gtx)
			})
		}),
		espaco(4),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return d.btnAvisos.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				cor := tema.Azul
				if d.btnAvisos.Hovered() {
					cor = tema.AzulH
				}
				return rotulo(th, fonteMono, unit.Sp(11), "Componentes de terceiros e licenças", cor)(gtx)
			})
		}),
		espaco(16),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Dimensions{Size: gtx.Constraints.Min}
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return botaoNeutro(gtx, th, &d.btnFechar, "Fechar")
				}),
			)
		}),
	)
}

// abrirNoNavegador chama o abridor de URL do sistema. No Flatpak, o
// xdg-open de dentro do sandbox não existe — precisa ir pelo host, como
// o atualizador já faz para o flatpak install (atualizador.noHost).
func abrirNoNavegador(url string) {
	var cmd *exec.Cmd
	switch {
	case runtime.GOOS == "windows":
		cmd = exec.Command("cmd", "/c", "start", "", url)
	case atualizador.EmFlatpak():
		cmd = exec.Command("flatpak-spawn", "--host", "xdg-open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "abrir navegador:", err)
	}
}

func extraiMeta(re *regexp.Regexp) string {
	m := re.FindStringSubmatch(metainfoXML)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}
