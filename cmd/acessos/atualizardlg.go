package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	"acessos-go/internal/atualizador"

	"gioui.org/app"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// dlgAtualizar avisa que há versão nova e instala sob confirmação.
//
// A confirmação existe porque atualizar FECHA as sessões abertas: o app
// sai e volta. Quem está no meio de um atendimento decide a hora.
type dlgAtualizar struct {
	w   *app.Window
	rel *atualizador.Release

	mu sync.Mutex
	// baixando cobre da confirmação até o pacote estar no disco;
	// instalando é o que vem depois. No Linux instalando dura o
	// flatpak install; no Windows vai até o instalador fechar o app.
	baixando   bool
	instalando bool
	fracao     float64
	erro       string
	btnAgora   widget.Clickable
	btnDepois  widget.Clickable
}

func oferecerAtualizacao(w *app.Window, r *atualizador.Release) {
	abrirDialogo(&dlgAtualizar{w: w, rel: r})
}

func (d *dlgAtualizar) Titulo() string   { return "Atualização disponível" }
func (d *dlgAtualizar) Largura() unit.Dp { return 460 }

func (d *dlgAtualizar) Corpo(gtx layout.Context, th *material.Theme) layout.Dimensions {
	d.mu.Lock()
	baixando, instalando, fracao, erro := d.baixando, d.instalando, d.fracao, d.erro
	d.mu.Unlock()

	if !instalando {
		if d.btnDepois.Clicked(gtx) {
			fecharDialogo()
		}
		if !baixando && d.btnAgora.Clicked(gtx) {
			d.instalar()
		}
	}

	filhos := []layout.FlexChild{
		layout.Rigid(rotulo(th, fonteSans, spCorpo,
			fmt.Sprintf("A versão %s está disponível (você tem a %s).",
				d.rel.Tag, versaoInstalada()), tema.Texto)),
		espaco(6),
		layout.Rigid(rotulo(th, fonteSans, spNota,
			"O app será fechado e reaberto — as sessões abertas se encerram.", tema.Sec)),
	}
	if baixando {
		filhos = append(filhos, espaco(10),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				b := material.ProgressBar(th, float32(fracao))
				b.Color = tema.Azul
				b.TrackColor = tema.Vidro2
				return b.Layout(gtx)
			}))
	}
	if instalando {
		// A barra continua onde estava, cheia: ela mede o DOWNLOAD, que
		// de fato acabou. A linha abaixo é só para dizer o que está
		// acontecendo agora, já que a instalação não tem progresso para
		// ler. No Linux ela pisca por poucos segundos e a janela nova
		// sobe; no Windows fica até o instalador fechar este processo, que
		// é o tempo em que a tela ficaria sem nada para mostrar.
		filhos = append(filhos, espaco(8),
			layout.Rigid(rotulo(th, fonteSans, spNota,
				"Instalando a atualização…", tema.Sec)))
	}
	if erro != "" {
		filhos = append(filhos, espaco(8),
			layout.Rigid(rotulo(th, fonteSans, spRotulo, erro, tema.ErroFg)))
	}
	if !instalando {
		filhos = append(filhos, espaco(14), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Dimensions{Size: gtx.Constraints.Min}
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return botaoNeutro(gtx, th, &d.btnDepois, "Depois")
				}),
				layout.Rigid(layout.Spacer{Width: 8}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return botao(gtx, th, &d.btnAgora, "Atualizar agora", pesoPrimario, baixando)
				}),
			)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, filhos...)
}

func (d *dlgAtualizar) instalar() {
	d.mu.Lock()
	d.baixando, d.erro = true, ""
	d.mu.Unlock()

	go func() {
		err := atualizador.Instalar(d.rel,
			func(f float64) {
				d.mu.Lock()
				d.fracao = f
				d.mu.Unlock()
				d.w.Invalidate()
			},
			func() {
				d.mu.Lock()
				d.instalando = true
				d.mu.Unlock()
				d.w.Invalidate()
			})
		if err != nil {
			d.mu.Lock()
			d.baixando, d.instalando, d.erro = false, false, err.Error()
			d.mu.Unlock()
			d.w.Invalidate()
			return
		}
		// A instância nova já está subindo; esta sai. No Windows é raro
		// chegar aqui: o instalador costuma fechar este processo antes.
		//
		// O fechamento vai pela fila do laço da janela, e não direto daqui:
		// no Wayland o Perform roda na goroutine de quem chama, e o pedido
		// feito desta goroutine ficava parado até a janela receber algum
		// evento. Foi assim na passagem da 2.8.3 para a 2.9.0: a versão nova
		// abriu e a velha continuou na tela, parecendo que nada tinha
		// mudado. Se mesmo assim a janela não fechar, o processo sai: a
		// instância nova já está de pé e duas abertas só confundem.
		naJanelaPrincipalInsistindo(d.w, func() { d.w.Perform(system.ActionClose) })
		time.Sleep(15 * time.Second)
		fmt.Fprintln(os.Stderr, "atualização: a janela antiga não fechou; saindo")
		os.Exit(0)
	}()
}

// checarAtualizacao roda no start, em segundo plano. Silenciosa: sem
// rede, sem release nova ou fora do Flatpak, ninguém fica sabendo — aviso
// de atualização que aparece para dizer "está tudo certo" vira ruído.
func checarAtualizacao(w *app.Window) {
	if !atualizador.Suportado() {
		return
	}
	atual := versaoInstalada()
	if atual == "" {
		return
	}
	go func() {
		rel, err := atualizador.Checar(atual)
		if err != nil || rel == nil {
			return
		}
		reg("atualização disponível: %s", rel.Tag)
		oferecerAtualizacao(w, rel)
		w.Invalidate()
	}()
}
