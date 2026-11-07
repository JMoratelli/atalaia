package main

// Desenho e gestos da aba SFTP (2.9.2). Até a 2.9.1 ela era a única tela
// que não tinha passado pela repaginação: duas listas soltas na folha,
// botões empilhados no meio, o caminho num campo de texto e um arrasto
// que só levava o que já estivesse marcado — apertar num arquivo e
// arrastar não levava nada, que é exatamente o gesto que todo cliente de
// SFTP ensina.
//
// Agora cada lado é um cartão de vidro, como os do Painel: cabeçalho com
// as ferramentas daquele lado, o caminho em migalhas (clicar no vazio ao
// lado abre a edição), o filtro com a lupa, colunas e linhas de altura
// fixa. O arrasto leva o item apertado (ou a seleção, se ele fizer parte
// dela), mostra o que vai junto preso ao ponteiro e acende o painel e a
// pasta de destino; soltar em cima de uma pasta manda para dentro dela.
// O botão direito traz as ações do item, e as mesmas ações valem dos dois
// lados — antes criar pasta e excluir só existiam no remoto.

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gio.tools/icons"
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
	sftpLinhaH   = unit.Dp(30) // altura FIXA da linha: o arrasto acha a pasta pela conta
	sftpCentroL  = unit.Dp(72) // coluna das setas entre os painéis
	sftpPainelR  = unit.Dp(14) // raio do cartão
	sftpPad      = unit.Dp(12) // respiro interno do cartão
	sftpColTam   = unit.Dp(84)
	sftpColData  = unit.Dp(112)
	sftpMargemX  = unit.Dp(16)
	sftpMargemY  = unit.Dp(12)
	sftpArrastoD = unit.Dp(8) // quanto andar antes de virar arrasto
)

func (t *sftpTab) Layout(gtx layout.Context) layout.Dimensions {
	t.aplicarListagens()
	// O que a lista anota a cada quadro (item apertado, item sob o
	// ponteiro) vale só para este quadro.
	t.pressPainel, t.pressNome = nil, ""
	t.sobPainel, t.sobNome = nil, ""
	t.tratarEventos(gtx)
	defer t.arrasto(gtx)

	mx, my := gtx.Dp(sftpMargemX), gtx.Dp(sftpMargemY)
	return layout.Inset{Top: sftpMargemY, Bottom: 10, Left: sftpMargemX, Right: sftpMargemX}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return t.painel(gtx, &t.local, image.Pt(mx, my))
					}),
					layout.Rigid(t.colunaCentral),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						x := mx + t.local.area.Dx() + gtx.Dp(sftpCentroL)
						return t.painel(gtx, &t.remoto, image.Pt(x, my))
					}),
				)
			}),
			layout.Rigid(t.rodape),
		)
	})
}

// ------------------------------------------------------------- eventos

func (t *sftpTab) tratarEventos(gtx layout.Context) {
	for _, p := range []*painelArquivos{&t.local, &t.remoto} {
		p := p
		itensVisiveis := p.visiveis()
		if p.btnAcima.Clicked(gtx) {
			t.ativo = p
			t.subir(p)
		}
		if p.btnAtual.Clicked(gtx) {
			t.ativo = p
			t.recarregar(p)
		}
		if p.btnNovaDir.Clicked(gtx) {
			t.ativo = p
			t.criarPasta(p)
		}
		if p.btnRenomear.Clicked(gtx) {
			t.ativo = p
			t.renomear(p)
		}
		if p.btnExcluir.Clicked(gtx) {
			t.ativo = p
			t.excluir(p)
		}
		if p.btnEditar.Clicked(gtx) {
			p.editando, p.focouCam = true, false
		}
		for i := range p.btnMigalha {
			if p.btnMigalha[i].Clicked(gtx) {
				m := migalhas(p.caminho.Text(), p.remoto)
				if i < len(m) {
					p.caminho.SetText(m[i].caminho)
					t.recarregar(p)
				}
			}
		}
		// Enter no campo de caminho: vai para o caminho TAL QUAL foi
		// digitado e volta às migalhas.
		for {
			ev, ok := p.caminho.Update(gtx)
			if !ok {
				break
			}
			if _, submeteu := ev.(widget.SubmitEvent); submeteu {
				p.editando = false
				t.recarregar(p)
			}
		}
		for i := range p.btnItem {
			if i >= len(itensVisiveis) {
				continue
			}
			for {
				ev, ok := p.btnItem[i].Update(gtx)
				if !ok {
					break
				}
				t.ativo = p
				it := itensVisiveis[i]
				if ev.NumClicks >= 2 {
					// Dois cliques: pasta entra, arquivo vai para o outro
					// lado (o mesmo gesto dos clientes de SFTP de duas
					// colunas).
					if it.dir {
						t.navegar(p, it.nome)
					} else {
						t.iniciarTransferencia([]itemArquivo{it}, !p.remoto)
					}
					break
				}
				t.clicarItem(p, itensVisiveis, i, comCtrl(ev), shiftPressionado())
			}
		}
	}

	if t.btnEnviar.Clicked(gtx) {
		t.iniciarTransferencia(selecionados(&t.local), true)
	}
	if t.btnBaixar.Clicked(gtx) {
		t.iniciarTransferencia(selecionados(&t.remoto), false)
	}
	t.teclas(gtx)
}

// teclas são as de gerenciador de arquivos, no último painel clicado. Os
// filtros não têm foco: só chega aqui a tecla que nenhum campo consumiu
// (Delete no filtro apaga letra, não arquivo).
func (t *sftpTab) teclas(gtx layout.Context) {
	for {
		ev, ok := gtx.Event(
			key.Filter{Name: key.NameDeleteForward},
			key.Filter{Name: key.NameF2},
			key.Filter{Name: key.NameF5},
			key.Filter{Name: key.NameReturn},
			key.Filter{Name: key.NameDeleteBackward},
			key.Filter{Name: "A", Optional: key.ModCtrl | key.ModShortcut | key.ModShift},
		)
		if !ok {
			break
		}
		e, ok := ev.(key.Event)
		if !ok || e.State != key.Press || focoEmCampo.Load() {
			continue
		}
		p := t.ativo
		if p == nil {
			p = &t.local
		}
		if p.remoto && t.cliente() == nil {
			continue
		}
		switch e.Name {
		case key.NameDeleteForward:
			t.excluir(p)
		case key.NameF2:
			t.renomear(p)
		case key.NameF5:
			t.recarregar(p)
		case key.NameDeleteBackward:
			t.subir(p)
		case key.NameReturn:
			if sel := selecionados(p); len(sel) == 1 && sel[0].dir {
				t.navegar(p, sel[0].nome)
			} else if len(sel) > 0 {
				t.iniciarTransferencia(sel, !p.remoto)
			}
		case "A":
			// Ctrl+A marca tudo o que está à vista (o filtro manda). No
			// Linux o Modifiers do Gio chega vazio; o Ctrl vem do grab.
			if ctrlPressionado() || e.Modifiers.Contain(key.ModShortcut) {
				for _, it := range p.visiveis() {
					p.sel[it.nome] = true
				}
			}
		}
	}
}

// clicarItem aplica a seleção do clique simples: só ele, Ctrl soma e tira,
// Shift marca a faixa desde o último clicado.
func (t *sftpTab) clicarItem(p *painelArquivos, vis []itemArquivo, i int, ctrl, shift bool) {
	it := vis[i]
	switch {
	case shift && p.ancora != "":
		de := -1
		for k, v := range vis {
			if v.nome == p.ancora {
				de = k
			}
		}
		if de < 0 {
			de = i
		}
		if !ctrl {
			clear(p.sel)
		}
		a, b := min(de, i), max(de, i)
		for k := a; k <= b; k++ {
			p.sel[vis[k].nome] = true
		}
		return // o ancora fica: Shift+clique de novo refaz a faixa a partir dele
	case ctrl:
		p.sel[it.nome] = !p.sel[it.nome]
	default:
		clear(p.sel)
		p.sel[it.nome] = true
	}
	p.ancora = it.nome
}

func (t *sftpTab) recarregar(p *painelArquivos) {
	if p.remoto {
		go t.listarRemoto()
		return
	}
	t.listarLocal()
}

// ---------------------------------------------------------- os painéis

// painel desenha um lado. origem é o canto do cartão em coordenadas da
// aba, guardado para o arrasto saber onde cada painel e cada linha estão.
func (t *sftpTab) painel(gtx layout.Context, p *painelArquivos, origem image.Point) layout.Dimensions {
	gtx.Constraints.Min = gtx.Constraints.Max
	size := gtx.Constraints.Max
	p.area = image.Rectangle{Min: origem, Max: origem.Add(size)}
	p.linhaH = gtx.Dp(sftpLinhaH)

	alvo := t.arrastando && t.origem != nil && t.origem != p && t.ladoSob(t.posAtual) == p
	// Sombra como a dos cards do Painel: no claro é ela que separa o
	// cartão branco da folha clara (no escuro a borda de luz do vidro já
	// fazia isso, e a falta dela só aparecia no claro).
	sombra(gtx, size, sftpPainelR)
	superficieVidro(gtx, size, tema.PainelTinta, tema.CardBorda, sftpPainelR)
	if alvo {
		// O painel que vai receber acende como campo focado: anel azul.
		r := gtx.Dp(sftpPainelR)
		rr := clip.UniformRRect(image.Rectangle{Max: size}, r)
		paint.FillShape(gtx.Ops, tema.AzulCheio, clip.Stroke{Path: rr.Path(gtx.Ops), Width: float32(gtx.Dp(2))}.Op())
	}

	// Altura acumulada dos itens fixos, para saber onde a lista começa.
	y := gtx.Dp(sftpPad)
	medir := func(w layout.Widget) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			d := w(gtx)
			y += d.Size.Y
			return d
		}
	}
	vao := func(h unit.Dp) layout.FlexChild {
		return layout.Rigid(medir(layout.Spacer{Height: h}.Layout))
	}

	return layout.UniformInset(sftpPad).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(medir(func(gtx layout.Context) layout.Dimensions { return t.cabecalhoPainel(gtx, p) })),
			vao(10),
			layout.Rigid(medir(func(gtx layout.Context) layout.Dimensions { return t.caminhoPainel(gtx, p) })),
			vao(8),
			layout.Rigid(medir(func(gtx layout.Context) layout.Dimensions { return t.filtroPainel(gtx, p) })),
			vao(10),
			layout.Rigid(medir(func(gtx layout.Context) layout.Dimensions { return t.colunas(gtx) })),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				p.listaY = origem.Y + y
				gtx.Constraints.Min = gtx.Constraints.Max
				// O splash só cobre o painel REMOTO: o local não depende
				// de rede e continua usável enquanto se espera a conexão.
				if p.remoto && t.cliente() == nil {
					icSftp, corSftp, _ := t.Selo()
					return desenharSplash(gtx, t.th, t.splash,
						fmt.Sprintf("%s@%s:%d", t.user, t.host, t.porta), icSftp, corSftp,
						&t.btnSplash, func() { go t.conectar() })
				}
				if p.erro != "" {
					return layout.Center.Layout(gtx, rotulo(t.th, fonteMono, unit.Sp(11.5), p.erro, tema.ErroFg))
				}
				return t.listaArquivos(gtx, p)
			}),
		)
	})
}

func (t *sftpTab) cabecalhoPainel(gtx layout.Context, p *painelArquivos) layout.Dimensions {
	th := t.th
	ic, cor, titulo := icons.HardwareComputer, tema.Azul, "Esta máquina"
	sub := ""
	if p.remoto {
		ic, cor, titulo = icons.ActionDescription, tema.AtencaoFg, t.titulo
		sub = fmt.Sprintf("%s@%s · ", t.user, t.host)
	}
	n, total, marcados := len(p.visiveis()), len(p.itens), len(selecionados(p))
	switch {
	case n != total:
		sub += fmt.Sprintf("%d de %d itens", n, total)
	default:
		sub += fmt.Sprintf("%d %s", total, plural(total, "item", "itens"))
	}
	if marcados > 0 {
		sub += fmt.Sprintf(" · %d %s", marcados, plural(marcados, "marcado", "marcados"))
	}
	conectado := !p.remoto || t.cliente() != nil
	um, algum := marcados == 1, marcados > 0

	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lado := gtx.Dp(32)
			superficie(gtx, image.Pt(lado, lado), tinta(cor, 0.16), transparente, 9)
			gtx.Constraints = layout.Exact(image.Pt(lado, lado))
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return icone(gtx, ic, cor, 18)
			})
		}),
		layout.Rigid(layout.Spacer{Width: 10}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					l := negrito(txt(th, fonteTitulo, unit.Sp(14), titulo, tema.Texto))
					l.MaxLines = 1
					return l.Layout(gtx)
				}),
				layout.Rigid(rotuloLinha(th, fonteMono, unit.Sp(10.5), sub, tema.Fraco)),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return capsula(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ferramentaSe(gtx, &p.btnAcima, icons.NavigationArrowUpward, tema.Sec, conectado, "sftp-acima", "Pasta de cima")
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ferramentaSe(gtx, &p.btnAtual, icons.NavigationRefresh, tema.Sec, conectado, "sftp-atual", "Atualizar")
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return divisoria(gtx, tema.Borda) }),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ferramentaSe(gtx, &p.btnNovaDir, icons.FileCreateNewFolder, tema.Sec, conectado, "sftp-nova", "Nova pasta")
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ferramentaSe(gtx, &p.btnRenomear, icons.EditorModeEdit, tema.Sec, conectado && um, "sftp-renomear", "Renomear")
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ferramentaSe(gtx, &p.btnExcluir, icons.ActionDelete, tema.ErroFg, conectado && algum, "sftp-excluir", "Excluir os marcados")
				}),
			)
		}),
	)
}

// ferramentaSe é a ferramenta da cápsula que pode estar desligada: sem
// clique e com o ícone apagado. A dica sai sob o mouse quando ligada.
func ferramentaSe(gtx layout.Context, btn *widget.Clickable, ic *widget.Icon, cor color.NRGBA, ligada bool, chave, dica string) layout.Dimensions {
	if !ligada {
		size := image.Pt(gtx.Dp(30), gtx.Dp(26))
		gtx.Constraints = layout.Exact(size)
		layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return icone(gtx, ic, comAlfa(tema.Fraco, 0.5), 17)
		})
		return layout.Dimensions{Size: size}
	}
	d := ferramenta(gtx, btn, ic, cor)
	if btn.Hovered() {
		pedirDica(chave, dica)
	}
	return d
}

// migalha é um pedaço do caminho, com o caminho inteiro até ele.
type migalha struct{ nome, caminho string }

// migalhas quebra o caminho nos pedaços clicáveis. O remoto é sempre
// POSIX; o local segue o sistema (no Windows a raiz é o volume, "C:\").
func migalhas(caminho string, remoto bool) []migalha {
	if caminho == "" {
		return nil
	}
	if remoto || runtime.GOOS != "windows" {
		limpo := path.Clean(strings.ReplaceAll(caminho, "\\", "/"))
		if !remoto {
			limpo = filepath.ToSlash(filepath.Clean(caminho))
		}
		out := []migalha{{"/", "/"}}
		if !strings.HasPrefix(limpo, "/") {
			out = nil // caminho relativo ("."): mostra como veio
		}
		acum := ""
		for _, parte := range strings.Split(strings.Trim(limpo, "/"), "/") {
			if parte == "" {
				continue
			}
			if strings.HasPrefix(limpo, "/") {
				acum += "/" + parte
			} else if acum == "" {
				acum = parte
			} else {
				acum += "/" + parte
			}
			out = append(out, migalha{parte, acum})
		}
		return out
	}
	limpo := filepath.Clean(caminho)
	vol := filepath.VolumeName(limpo)
	raiz := vol + string(filepath.Separator)
	out := []migalha{{raiz, raiz}}
	acum := raiz
	for _, parte := range strings.Split(strings.TrimPrefix(limpo, raiz), string(filepath.Separator)) {
		if parte == "" {
			continue
		}
		acum = filepath.Join(acum, parte)
		out = append(out, migalha{parte, acum})
	}
	return out
}

// caminhoPainel: as migalhas num campo, ou o próprio campo de texto
// quando se clica no vazio ao lado delas (ou no lápis).
func (t *sftpTab) caminhoPainel(gtx layout.Context, p *painelArquivos) layout.Dimensions {
	th := t.th
	altura := gtx.Dp(34)
	if p.editando {
		if p.focouCam && !gtx.Focused(&p.caminho) {
			p.editando = false // saiu do campo sem Enter: volta às migalhas
		} else {
			gtx.Constraints.Min.Y = altura
			d := caixaEditor(gtx, th, &p.caminho, "caminho", 0)
			// O foco é pedido DEPOIS do campo desenhado: o alvo tem de
			// estar registrado neste quadro (ver buscapop.go).
			if !p.focouCam {
				gtx.Execute(key.FocusCmd{Tag: &p.caminho})
				p.focouCam = true
			}
			return d
		}
	}
	m := migalhas(p.caminho.Text(), p.remoto)
	for len(p.btnMigalha) < len(m) {
		p.btnMigalha = append(p.btnMigalha, widget.Clickable{})
	}
	// Caminho fundo: os primeiros pedaços somem e fica um "…" no lugar,
	// porque o que interessa é onde se está, não de onde se partiu.
	primeiro := 0
	if len(m) > 5 {
		primeiro = len(m) - 4
	}
	gtx.Constraints.Min = image.Pt(gtx.Constraints.Max.X, altura)
	gtx.Constraints.Max.Y = altura
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			fundoCampo(gtx, gtx.Constraints.Min, false)
			return layout.Dimensions{Size: gtx.Constraints.Min}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			return layout.Inset{Left: 4, Right: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				var filhos []layout.FlexChild
				add := func(w layout.Widget) { filhos = append(filhos, layout.Rigid(w)) }
				for i := range m {
					if i > 0 && i < primeiro {
						continue
					}
					if i == primeiro && primeiro > 0 {
						add(func(gtx layout.Context) layout.Dimensions {
							return centroV(gtx, rotulo(th, fonteMono, unit.Sp(12), "…", tema.Fraco))
						})
						add(func(gtx layout.Context) layout.Dimensions { return centroV(gtx, seta) })
					}
					i := i
					ultimo := i == len(m)-1
					add(func(gtx layout.Context) layout.Dimensions {
						return centroV(gtx, func(gtx layout.Context) layout.Dimensions {
							return t.migalha(gtx, &p.btnMigalha[i], m[i].nome, ultimo)
						})
					})
					if !ultimo {
						add(func(gtx layout.Context) layout.Dimensions { return centroV(gtx, seta) })
					}
				}
				filhos = append(filhos,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						// O vazio depois das migalhas abre a edição: é onde
						// se clica num gerenciador de arquivos para digitar.
						return p.btnEditar.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							pointer.CursorText.Add(gtx.Ops)
							return layout.Dimensions{Size: gtx.Constraints.Max}
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return centroV(gtx, func(gtx layout.Context) layout.Dimensions {
							return icone(gtx, icons.EditorModeEdit, tema.Fraco, 14)
						})
					}),
					layout.Rigid(layout.Spacer{Width: 6}.Layout),
				)
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, filhos...)
			})
		}),
	)
}

func seta(gtx layout.Context) layout.Dimensions {
	return icone(gtx, icons.NavigationChevronRight, tema.Fraco, 14)
}

func (t *sftpTab) migalha(gtx layout.Context, btn *widget.Clickable, nome string, ultimo bool) layout.Dimensions {
	return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		cor := tema.Sec
		if ultimo || btn.Hovered() {
			cor = tema.Texto
		}
		l := txt(t.th, fonteMono, unit.Sp(12), nome, cor)
		if ultimo {
			l = negrito(l)
		}
		rec := op.Record(gtx.Ops)
		d := layout.Inset{Top: 4, Bottom: 4, Left: 6, Right: 6}.Layout(gtx, l.Layout)
		desenho := rec.Stop()
		if btn.Hovered() {
			superficie(gtx, d.Size, tinta(tema.Texto, 0.08), transparente, 7)
		}
		desenho.Add(gtx.Ops)
		return d
	})
}

// filtroPainel: a busca DENTRO da pasta aberta, com a lupa.
func (t *sftpTab) filtroPainel(gtx layout.Context, p *painelArquivos) layout.Dimensions {
	th := t.th
	marcarFoco(gtx.Focused(&p.filtro))
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			fundoCampo(gtx, gtx.Constraints.Min, gtx.Focused(&p.filtro))
			return layout.Dimensions{Size: gtx.Constraints.Min}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Inset{Top: 8, Bottom: 8, Left: 10, Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return icone(gtx, icons.ActionSearch, tema.Fraco, 16)
					}),
					layout.Rigid(layout.Spacer{Width: 8}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						es := material.Editor(th, &p.filtro, "Filtrar nesta pasta")
						es.Font = fonteSans
						es.TextSize = unit.Sp(12.5)
						es.Color = tema.Texto
						es.HintColor = tema.Fraco
						return es.Layout(gtx)
					}),
				)
			})
		}),
	)
}

// colunas é o cabeçalho da lista. Os tamanhos são os mesmos das linhas.
func (t *sftpTab) colunas(gtx layout.Context) layout.Dimensions {
	th := t.th
	cab := func(s string) layout.Widget {
		return rotuloLinha(th, fonteMono, unit.Sp(9.5), s, tema.Fraco)
	}
	direita := func(w unit.Dp, s string) layout.FlexChild {
		return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints = layout.Exact(image.Pt(gtx.Dp(w), gtx.Dp(16)))
			return layout.E.Layout(gtx, cab(s))
		})
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: 36, Right: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, cab("NOME")),
					direita(sftpColTam, "TAMANHO"),
					direita(sftpColData, "MODIFICADO"),
				)
			})
		}),
		layout.Rigid(layout.Spacer{Height: 6}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			sz := image.Pt(gtx.Constraints.Max.X, gtx.Dp(1))
			paint.FillShape(gtx.Ops, tema.Borda, clip.Rect{Max: sz}.Op())
			return layout.Dimensions{Size: sz}
		}),
		layout.Rigid(layout.Spacer{Height: 4}.Layout),
	)
}

func (t *sftpTab) listaArquivos(gtx layout.Context, p *painelArquivos) layout.Dimensions {
	itens := p.visiveis()
	if len(itens) == 0 {
		msg := "Pasta vazia"
		if len(p.itens) > 0 {
			msg = "Nada com esse nome nesta pasta"
		}
		return layout.Center.Layout(gtx, rotulo(t.th, fonteSans, unit.Sp(12.5), msg, tema.Fraco))
	}
	// A pasta de destino do arrasto, se o ponteiro estiver sobre uma.
	destino := ""
	if t.arrastando && t.origem != nil && t.origem != p && t.ladoSob(t.posAtual) == p {
		destino = t.pastaSob(p, t.posAtual)
	}
	return material.List(t.th, &p.lista).Layout(gtx, len(itens), func(gtx layout.Context, i int) layout.Dimensions {
		return t.linhaArquivo(gtx, p, i, itens[i], destino)
	})
}

func (t *sftpTab) linhaArquivo(gtx layout.Context, p *painelArquivos, i int, it itemArquivo, destino string) layout.Dimensions {
	th := t.th
	h := gtx.Dp(sftpLinhaH)
	gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, h))
	btn := &p.btnItem[i]
	if btn.Pressed() {
		t.pressPainel, t.pressNome = p, it.nome
	}
	if btn.Hovered() && !t.arrastando {
		t.sobPainel, t.sobNome = p, it.nome
	}
	marcado := p.sel[it.nome]
	ehDestino := it.dir && destino != "" && juntarLado(p.caminho.Text(), it.nome, p.remoto) == destino

	// Fundo FORA do Clickable: ele recorta a pintura na área de clique.
	size := gtx.Constraints.Max
	switch {
	case ehDestino:
		superficie(gtx, size, tema.AzulFraco, tema.AzulCheio, 8)
	case marcado:
		superficie(gtx, size, tema.AzulFraco, comAlfa(tema.Azul, 0.55), 8)
	case btn.Hovered() && !t.arrastando:
		superficie(gtx, size, tinta(tema.Texto, 0.06), transparente, 8)
	}
	return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		// A linha INTEIRA é a área de clique (o Clickable registra o
		// tamanho que o conteúdo devolve): sem isto só a faixa do texto,
		// no alto da linha, respondia ao aperto.
		gtx.Constraints.Min = gtx.Constraints.Max
		return layout.Inset{Left: 10, Right: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			ic, cor := icons.ActionDescription, tema.Sec
			if it.dir {
				ic, cor = icons.FileFolder, tema.AtencaoFg
			}
			coluna := func(w unit.Dp, s string, c color.NRGBA) layout.FlexChild {
				return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X, gtx.Constraints.Max.X = gtx.Dp(w), gtx.Dp(w)
					return layout.E.Layout(gtx, rotuloLinha(th, fonteMono, unit.Sp(10.5), s, c))
				})
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return centroV(gtx, func(gtx layout.Context) layout.Dimensions { return icone(gtx, ic, cor, 16) })
				}),
				layout.Rigid(layout.Spacer{Width: 10}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					// Nome, tamanho e data na MESMA linha de base: a mono e
					// a Space Grotesk têm caixas de altura diferentes, e
					// centralizar as caixas deixava a data mais alta que o
					// nome.
					return centroV(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
							layout.Flexed(1, rotuloLinha(th, fonteSans, unit.Sp(12.5), it.nome, tema.Texto)),
							coluna(sftpColTam, tamanhoHumano(it), tema.Sec),
							coluna(sftpColData, it.mtime.Format("02/01/06 15:04"), tema.Fraco),
						)
					})
				}),
			)
		})
	})
}

// --------------------------------------------------------- coluna central

func (t *sftpTab) colunaCentral(gtx layout.Context) layout.Dimensions {
	larg := gtx.Dp(sftpCentroL)
	gtx.Constraints = layout.Exact(image.Pt(larg, gtx.Constraints.Max.Y))
	temLocal := len(selecionados(&t.local)) > 0
	temRemoto := len(selecionados(&t.remoto)) > 0 && t.cliente() != nil
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return t.botaoSeta(gtx, &t.btnEnviar, icons.NavigationArrowForward, "enviar", temLocal,
					"Enviar os marcados desta máquina para o remoto")
			}),
			layout.Rigid(layout.Spacer{Height: 18}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return t.botaoSeta(gtx, &t.btnBaixar, icons.NavigationArrowBack, "baixar", temRemoto,
					"Baixar os marcados do remoto para esta máquina")
			}),
		)
	})
}

func (t *sftpTab) botaoSeta(gtx layout.Context, btn *widget.Clickable, ic *widget.Icon, rot string, ligado bool, dica string) layout.Dimensions {
	lado := gtx.Dp(44)
	return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			sz := image.Pt(lado, lado)
			fundo, cor := tinta(tema.Texto, 0.07), tinta(tema.Texto, 0.35)
			if ligado {
				fundo, cor = tema.AzulCheio, hex(0xffffff)
				if btn.Hovered() {
					fundo = tema.AzulH
				}
			}
			superficie(gtx, sz, fundo, transparente, 12)
			return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints = layout.Exact(sz)
				if ligado {
					pointer.CursorPointer.Add(gtx.Ops)
				}
				if btn.Hovered() {
					pedirDica("sftp-"+rot, dica)
				}
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return icone(gtx, ic, cor, 22)
				})
			})
		}),
		layout.Rigid(layout.Spacer{Height: 4}.Layout),
		layout.Rigid(rotulo(t.th, fonteMono, unit.Sp(10), rot, tema.Fraco)),
	)
}

// ----------------------------------------------------------------- rodapé

func (t *sftpTab) rodape(gtx layout.Context) layout.Dimensions {
	th := t.th
	t.mu.Lock()
	msg, erro := t.msg, t.msgErro
	feito, total, arq, i, n, inicio := t.progFeito, t.progTotal, t.progArquivo, t.progI, t.progN, t.progInicio
	ocupado := t.ocupado
	t.mu.Unlock()

	return layout.Inset{Top: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.Y = gtx.Dp(28)
		if ocupado && n > 0 {
			frac := float32(0)
			if total > 0 {
				frac = float32(feito) / float32(total)
			}
			velocidade := ""
			if s := time.Since(inicio).Seconds(); s > 0.5 {
				velocidade = " · " + tamanhoBytes(int64(float64(feito)/s)) + "/s"
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return centroV(gtx, func(gtx layout.Context) layout.Dimensions {
						return icone(gtx, icons.FileFileUpload, tema.Azul, 16)
					})
				}),
				layout.Rigid(layout.Spacer{Width: 8}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Max.X = gtx.Dp(360)
					return centroV(gtx, rotuloLinha(th, fonteSans, unit.Sp(12), fmt.Sprintf("Transferindo %d de %d · %s", i, n, arq), tema.Texto))
				}),
				layout.Rigid(layout.Spacer{Width: 12}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return centroV(gtx, func(gtx layout.Context) layout.Dimensions {
						return barraProgresso(gtx, frac)
					})
				}),
				layout.Rigid(layout.Spacer{Width: 12}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return centroV(gtx, rotuloLinha(th, fonteMono, unit.Sp(10.5),
						fmt.Sprintf("%s de %s%s", tamanhoBytes(feito), tamanhoBytes(total), velocidade), tema.Sec))
				}),
			)
		}
		texto, cor := msg, tema.Sec
		if erro {
			cor = tema.ErroFg
		}
		if t.arrastando && t.origem != nil {
			texto, cor = t.textoArrasto(), tema.Azul
		}
		if texto == "" {
			texto = "Arraste entre os painéis para transferir · dois cliques num arquivo também mandam · Ctrl+clique soma, Shift+clique marca uma faixa · botão direito: mais ações"
			cor = tema.Fraco
		}
		return centroV(gtx, rotuloLinha(th, fonteSans, unit.Sp(11.5), texto, cor))
	})
}

func barraProgresso(gtx layout.Context, frac float32) layout.Dimensions {
	sz := image.Pt(gtx.Constraints.Max.X, gtx.Dp(6))
	r := gtx.Dp(3)
	paint.FillShape(gtx.Ops, tinta(tema.Texto, 0.10), clip.UniformRRect(image.Rectangle{Max: sz}, r).Op(gtx.Ops))
	if frac > 0 {
		w := int(float32(sz.X) * min(frac, 1))
		paint.FillShape(gtx.Ops, tema.AzulCheio, clip.UniformRRect(image.Rect(0, 0, max(w, 2*r), sz.Y), r).Op(gtx.Ops))
	}
	return layout.Dimensions{Size: sz}
}

// ------------------------------------------------------------- o arrasto

// ladoSob diz para qual painel o ponteiro aponta. Pela metade da coluna
// do meio, e não pelo cartão: soltar no vão entre os dois também vale.
func (t *sftpTab) ladoSob(pos image.Point) *painelArquivos {
	if t.local.area.Empty() || t.remoto.area.Empty() {
		return nil
	}
	meio := (t.local.area.Max.X + t.remoto.area.Min.X) / 2
	if pos.X < meio {
		return &t.local
	}
	return &t.remoto
}

// pastaSob devolve a pasta (caminho completo) sob o ponteiro na lista do
// painel p, ou "" se ele não estiver sobre uma pasta. A conta vale porque
// a linha tem altura fixa.
func (t *sftpTab) pastaSob(p *painelArquivos, pos image.Point) string {
	if p.linhaH <= 0 || pos.Y < p.listaY || pos.Y >= p.area.Max.Y {
		return ""
	}
	idx := p.lista.Position.First + (pos.Y-p.listaY+p.lista.Position.Offset)/p.linhaH
	vis := p.visiveis()
	if idx < 0 || idx >= len(vis) || !vis[idx].dir {
		return ""
	}
	return juntarLado(p.caminho.Text(), vis[idx].nome, p.remoto)
}

func (t *sftpTab) textoArrasto() string {
	o := t.origem
	alvo := t.ladoSob(t.posAtual)
	oque := fmt.Sprintf("%d itens", len(t.itensArr))
	if len(t.itensArr) == 1 {
		oque = t.itensArr[0].nome
	}
	if alvo == nil || alvo == o {
		return "Arraste até o outro painel para transferir " + oque
	}
	pasta := t.pastaSob(alvo, t.posAtual)
	if pasta == "" {
		pasta = alvo.caminho.Text()
	}
	verbo := "enviar"
	if o.remoto {
		verbo = "baixar"
	}
	return fmt.Sprintf("Solte para %s %s para %s", verbo, oque, pasta)
}

// arrasto trata o ponteiro numa área por cima da aba inteira, com PassOp:
// por baixo ela não receberia nada, e com PassOp os cliques das listas e
// dos botões continuam funcionando. Também é daqui que sai o menu do botão
// direito, e é aqui que se desenha o que vai preso ao ponteiro.
func (t *sftpTab) arrasto(gtx layout.Context) {
	for {
		ev, ok := gtx.Source.Event(pointer.Filter{
			Target: t.tagArrasto,
			Kinds:  pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel,
		})
		if !ok {
			break
		}
		pe, isP := ev.(pointer.Event)
		if !isP {
			continue
		}
		pos := image.Pt(int(pe.Position.X), int(pe.Position.Y))
		switch pe.Kind {
		case pointer.Press:
			if pe.Buttons.Contain(pointer.ButtonSecondary) {
				t.menuDeContexto(pos)
				continue
			}
			if !pe.Buttons.Contain(pointer.ButtonPrimary) {
				continue
			}
			t.arrastando, t.itensArr = false, nil
			t.posArrasto, t.posAtual = pos, pos
			t.origem = nil
			// Só vira arrasto quem apertou numa LINHA: apertar no vazio,
			// num botão ou num campo não leva nada.
			if t.pressPainel != nil && t.pressNome != "" {
				t.origem = t.pressPainel
			}
		case pointer.Drag:
			t.posAtual = pos
			if t.arrastando || t.origem == nil {
				continue
			}
			if abs(pos.X-t.posArrasto.X)+abs(pos.Y-t.posArrasto.Y) <= gtx.Dp(sftpArrastoD) {
				continue
			}
			p := t.origem
			if p.remoto && t.cliente() == nil {
				t.origem = nil
				continue
			}
			// O item apertado vai junto da seleção se fizer parte dela;
			// se não fizer, vai SÓ ele (e passa a ser a seleção), como em
			// qualquer gerenciador de arquivos.
			if t.pressNome != "" && !p.sel[t.pressNome] {
				clear(p.sel)
				p.sel[t.pressNome] = true
				p.ancora = t.pressNome
			}
			t.itensArr = selecionados(p)
			t.arrastando = len(t.itensArr) > 0
		case pointer.Release:
			if t.arrastando {
				alvo := t.ladoSob(pos)
				if alvo != nil && alvo != t.origem {
					t.iniciarTransferenciaPara(t.itensArr, !t.origem.remoto, t.pastaSob(alvo, pos))
				}
			}
			t.arrastando, t.origem, t.itensArr = false, nil, nil
		case pointer.Cancel:
			t.arrastando, t.origem, t.itensArr = false, nil, nil
		}
	}

	pass := pointer.PassOp{}.Push(gtx.Ops)
	area := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
	event.Op(gtx.Ops, t.tagArrasto)
	if t.arrastando {
		pointer.CursorGrabbing.Add(gtx.Ops)
	}
	area.Pop()
	pass.Pop()

	if t.arrastando {
		t.desenharFantasma(gtx)
	}
}

// desenharFantasma é o que vai preso ao ponteiro: o ícone e o nome (ou a
// contagem), num chip com sombra, um pouco abaixo e à direita da seta.
func (t *sftpTab) desenharFantasma(gtx layout.Context) {
	oque := fmt.Sprintf("%d itens", len(t.itensArr))
	ic, cor := icons.ActionDescription, tema.Sec
	if len(t.itensArr) == 1 {
		oque = t.itensArr[0].nome
		if t.itensArr[0].dir {
			ic, cor = icons.FileFolder, tema.AtencaoFg
		}
	} else {
		ic, cor = icons.ContentContentCopy, tema.Azul
	}
	defer op.Offset(t.posAtual.Add(image.Pt(gtx.Dp(14), gtx.Dp(12)))).Push(gtx.Ops).Pop()
	gtx.Constraints.Min = image.Point{}
	gtx.Constraints.Max.X = gtx.Dp(280)
	rec := op.Record(gtx.Ops)
	d := layout.Inset{Top: 7, Bottom: 7, Left: 10, Right: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return icone(gtx, ic, cor, 16) }),
			layout.Rigid(layout.Spacer{Width: 8}.Layout),
			layout.Rigid(rotuloLinha(t.th, fonteSans, unit.Sp(12.5), oque, tema.Texto)),
		)
	})
	conteudo := rec.Stop()
	sombraVazada(gtx, d.Size, 10, &elevBusca)
	superficie(gtx, d.Size, tema.Elevado, tema.Contorno, 10)
	conteudo.Add(gtx.Ops)
}

// ------------------------------------------------- menu e ações de arquivo

func (t *sftpTab) menuDeContexto(pos image.Point) {
	p := t.ladoSob(pos)
	if p == nil || (p.remoto && t.cliente() == nil) {
		return
	}
	t.ativo = p
	if t.sobPainel == p && t.sobNome != "" && !p.sel[t.sobNome] {
		clear(p.sel)
		p.sel[t.sobNome] = true
		p.ancora = t.sobNome
	}
	sel := selecionados(p)
	var itens []*itemMenu
	if len(sel) == 1 && sel[0].dir {
		nome := sel[0].nome
		itens = append(itens, &itemMenu{rotulo: "Abrir", acao: func() { t.navegar(p, nome) }})
	}
	if len(sel) > 0 {
		rot := "Enviar para o remoto"
		if p.remoto {
			rot = "Baixar para esta máquina"
		}
		itens = append(itens, &itemMenu{rotulo: rot, acao: func() { t.iniciarTransferencia(sel, !p.remoto) }})
	}
	if len(sel) == 1 {
		itens = append(itens, &itemMenu{rotulo: "Renomear…", acao: func() { t.renomear(p) }})
	}
	if len(sel) > 0 {
		itens = append(itens, &itemMenu{rotulo: "Copiar caminho", acao: func() {
			var caminhos []string
			for _, it := range sel {
				caminhos = append(caminhos, juntarLado(p.caminho.Text(), it.nome, p.remoto))
			}
			gtxClipboard(t.w, strings.Join(caminhos, "\n"))
		}})
	}
	itens = append(itens,
		&itemMenu{rotulo: "Nova pasta…", acao: func() { t.criarPasta(p) }},
		&itemMenu{rotulo: "Atualizar", acao: func() { t.recarregar(p) }},
	)
	if len(sel) > 0 {
		itens = append(itens, &itemMenu{rotulo: "Excluir…", perigo: true, acao: func() { t.excluir(p) }})
	}
	// A posição do ponteiro na JANELA: o deslocamento da área de conteúdo
	// não conta a barra da sessão que fica em cima desta aba.
	abrirMenu(t.w, ultimaPosPonteiro(), itens)
	t.invalidar()
}

func (t *sftpTab) criarPasta(p *painelArquivos) {
	cli := t.cliente()
	if p.remoto && cli == nil {
		return
	}
	base := p.caminho.Text()
	abrirDialogo(&dlgTexto{
		w: t.w, titulo: "Nova pasta", dica: "nome da pasta",
		ok: func(nome string) error {
			nome = strings.TrimSpace(nome)
			if nome == "" {
				return fmt.Errorf("o nome não pode ficar vazio")
			}
			if p.remoto {
				if err := cli.Mkdir(path.Join(base, nome)); err != nil {
					return err
				}
			} else if err := os.Mkdir(filepath.Join(base, nome), 0o755); err != nil {
				return err
			}
			t.recarregar(p)
			return nil
		},
	})
}

func (t *sftpTab) renomear(p *painelArquivos) {
	sel := selecionados(p)
	if len(sel) != 1 {
		return
	}
	cli := t.cliente()
	if p.remoto && cli == nil {
		return
	}
	antigo := sel[0].nome
	base := p.caminho.Text()
	d := &dlgTexto{
		w: t.w, titulo: "Renomear", dica: "novo nome",
		ok: func(novo string) error {
			novo = strings.TrimSpace(novo)
			if novo == "" || novo == antigo {
				return nil
			}
			if strings.ContainsAny(novo, `/\`) {
				return fmt.Errorf("o nome não pode ter barra")
			}
			de, para := juntarLado(base, antigo, p.remoto), juntarLado(base, novo, p.remoto)
			var err error
			if p.remoto {
				if _, e := cli.Stat(para); e == nil {
					return fmt.Errorf("já existe %q nesta pasta", novo)
				}
				err = cli.Rename(de, para)
			} else {
				if _, e := os.Stat(para); e == nil {
					return fmt.Errorf("já existe %q nesta pasta", novo)
				}
				err = os.Rename(de, para)
			}
			if err != nil {
				return err
			}
			clear(p.sel)
			p.sel[novo] = true
			t.recarregar(p)
			return nil
		},
	}
	d.valor.SetText(antigo)
	abrirDialogo(d)
}

// excluir pede confirmação com a contagem (recursiva) do que vai sair, nos
// dois lados — não tem desfazer em nenhum deles.
func (t *sftpTab) excluir(p *painelArquivos) {
	itens := selecionados(p)
	if len(itens) == 0 {
		return
	}
	if p.remoto && t.cliente() == nil {
		return
	}
	arquivos, pastas := t.levantar(p, itens)
	var lista []string
	for _, it := range itens {
		lista = append(lista, it.nome)
	}
	onde := fmt.Sprintf("na MÁQUINA REMOTA (%s)", t.host)
	titulo := "Excluir no remoto"
	if !p.remoto {
		onde, titulo = "NESTA MÁQUINA", "Excluir nesta máquina"
	}
	lista = append(lista, fmt.Sprintf("total: %d arquivo(s) e %d pasta(s) — %s", arquivos, pastas, onde))
	// A contagem regressiva é para LOTE: apagar um arquivo solto com três
	// segundos de espera irrita sem proteger nada.
	emLote := len(itens) > 1 || pastas > 0
	confirmarDestrutivoEm(t.w, titulo, lista, "Excluir definitivamente", emLote, func() {
		if p.remoto {
			go t.excluirRemoto(itens)
			return
		}
		t.excluirLocal(itens)
	})
}

func plural(n int, um, varios string) string {
	if n == 1 {
		return um
	}
	return varios
}
