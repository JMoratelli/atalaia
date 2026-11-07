package main

import (
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// MOLDURA E FOLHA (repaginação da 2.9)
//
// O desenho vem da central do ChamadoJá: o topo, a lateral e a barra de
// status são uma MOLDURA de um tom só (tema.Moldura), e o conteúdo mora
// numa FOLHA de outro tom (tema.Pagina, com a luz no canto — fundoFolha).
// O que liga as duas é a aba ativa: ela é pintada com a própria folha e
// desce até ela com duas curvas côncavas na base, como o item ativo do
// menu da central. Sem isso o topo, a tira de abas e o conteúdo eram três
// faixas soltas, cada uma de um material — a tela "desconexa" de antes.
//
// Aqui ficam as peças que mais de um lugar usa. Cor só por token, como no
// resto do app — fora o branco de ícone e rótulo sobre o azul cheio e as
// cores fixas da marca (marcaApp), que não mudam com o tema.

const (
	abaAltura = unit.Dp(28)
	abaRaio   = unit.Dp(8)
	// abaCurva é o raio das curvas côncavas que ligam a aba ativa (e o
	// item ativo da lateral) à folha. Elas saem PARA FORA da caixa da aba.
	abaCurva     = unit.Dp(8)
	folhaRaio    = unit.Dp(10) // canto da folha encostado na lateral
	statusAltura = unit.Dp(26)
	// alturaBarra: tudo o que fica numa linha de barra (busca, trilho,
	// botões de ação) tem esta altura, para os centros baterem.
	alturaBarra = unit.Dp(40)
	trilhoRaio  = unit.Dp(11)
)

// k da aproximação de um quarto de círculo por uma cúbica.
const kQuarto = 0.5523

// quarto acrescenta ao caminho um quarto de círculo de p até q, com o
// "joelho" em c (o canto do quadrado que o arco contorna).
func quarto(p *clip.Path, de, joelho, ate f32.Point) {
	c1 := de.Add(joelho.Sub(de).Mul(kQuarto))
	c2 := ate.Add(joelho.Sub(ate).Mul(kQuarto))
	p.CubeTo(c1, c2, ate)
}

// formaAbaAtiva: o contorno da aba ativa — cantos de cima arredondados e,
// embaixo, as duas curvas côncavas que a ligam à folha. size é a caixa da
// aba; as curvas saem abaCurva para cada lado dela. colada: a aba começa
// exatamente na borda esquerda da folha (lateral aberta, primeira aba), e
// o lado esquerdo desce reto, virando a própria borda da folha — sem curva.
func formaAbaAtiva(gtx layout.Context, size image.Point, colada bool) clip.PathSpec {
	r, c := float32(gtx.Dp(abaRaio)), float32(gtx.Dp(abaCurva))
	w, h := float32(size.X), float32(size.Y)
	var p clip.Path
	p.Begin(gtx.Ops)
	if colada {
		p.MoveTo(f32.Pt(0, h))
	} else {
		p.MoveTo(f32.Pt(-c, h))
		quarto(&p, f32.Pt(-c, h), f32.Pt(0, h), f32.Pt(0, h-c))
	}
	p.LineTo(f32.Pt(0, r))
	quarto(&p, f32.Pt(0, r), f32.Pt(0, 0), f32.Pt(r, 0))
	p.LineTo(f32.Pt(w-r, 0))
	quarto(&p, f32.Pt(w-r, 0), f32.Pt(w, 0), f32.Pt(w, r))
	p.LineTo(f32.Pt(w, h-c))
	quarto(&p, f32.Pt(w, h-c), f32.Pt(w, h), f32.Pt(w+c, h))
	p.Close()
	return p.End()
}

// contornoAbaAtiva: o traço de 1px em volta da aba ativa — o mesmo
// contorno de formaAbaAtiva, mas ABERTO na base: ele nasce e morre na
// linha da base do topo (que corre a meia altura do último pixel), e a aba
// fica parecendo recortada na moldura, aberta para a folha. Colada, o lado
// esquerdo corre na última coluna da lateral (x = -0,5), continuando a
// borda da folha que a lateral desenha.
func contornoAbaAtiva(gtx layout.Context, size image.Point, colada bool) clip.PathSpec {
	r, c := float32(gtx.Dp(abaRaio)), float32(gtx.Dp(abaCurva))
	w, h := float32(size.X)-0.5, float32(size.Y)-0.5
	var p clip.Path
	p.Begin(gtx.Ops)
	esq := float32(0.5)
	if colada {
		esq = -0.5
		p.MoveTo(f32.Pt(esq, h+0.5))
	} else {
		p.MoveTo(f32.Pt(-c, h))
		quarto(&p, f32.Pt(-c, h), f32.Pt(esq, h), f32.Pt(esq, h-c))
	}
	p.LineTo(f32.Pt(esq, r))
	quarto(&p, f32.Pt(esq, r), f32.Pt(esq, 0.5), f32.Pt(r, 0.5))
	p.LineTo(f32.Pt(w-r, 0.5))
	quarto(&p, f32.Pt(w-r, 0.5), f32.Pt(w, 0.5), f32.Pt(w, r))
	p.LineTo(f32.Pt(w, h-c))
	quarto(&p, f32.Pt(w, h-c), f32.Pt(w, h), f32.Pt(w+c+0.5, h))
	return p.End()
}

// contornoItemAtivo: o traço em volta do item ativo da lateral, aberto à
// direita, onde ele se funde à folha; as pontas caem na borda da lateral
// (meia altura da última coluna), que é onde corre o contorno da folha.
func contornoItemAtivo(gtx layout.Context, size image.Point) clip.PathSpec {
	r, c := float32(gtx.Dp(12)), float32(gtx.Dp(12))
	w, h := float32(size.X)-0.5, float32(size.Y)-0.5
	var p clip.Path
	p.Begin(gtx.Ops)
	p.MoveTo(f32.Pt(w, -c))
	quarto(&p, f32.Pt(w, -c), f32.Pt(w, 0.5), f32.Pt(w-c, 0.5))
	p.LineTo(f32.Pt(r, 0.5))
	quarto(&p, f32.Pt(r, 0.5), f32.Pt(0.5, 0.5), f32.Pt(0.5, r))
	p.LineTo(f32.Pt(0.5, h-r))
	quarto(&p, f32.Pt(0.5, h-r), f32.Pt(0.5, h), f32.Pt(r, h))
	p.LineTo(f32.Pt(w-c, h))
	quarto(&p, f32.Pt(w-c, h), f32.Pt(w, h), f32.Pt(w, h+c+0.5))
	return p.End()
}

// contornoFolhaLateral: com a lateral à vista, a borda esquerda da folha
// e o canto arredondado de cima, desenhados na última coluna da lateral
// (alt é a altura dela; raio, o do canto da folha). Fica POR BAIXO do item
// ativo, que a interrompe; o canto sai da lateral para a quina da folha,
// que é moldura (a folha é recortada ali), e encontra a linha do topo.
func contornoFolhaLateral(gtx layout.Context, larg, alt, raio int) {
	x, r := float32(larg)-0.5, float32(raio)
	var p clip.Path
	p.Begin(gtx.Ops)
	p.MoveTo(f32.Pt(x, float32(alt)))
	if raio == 0 {
		// a aba ativa está colada em cima: a borda sobe reta e o contorno
		// dela continua daqui
		p.LineTo(f32.Pt(x, -0.5))
	} else {
		p.LineTo(f32.Pt(x, r-0.5))
		quarto(&p, f32.Pt(x, r-0.5), f32.Pt(x, -0.5), f32.Pt(x+r+0.5, -0.5))
	}
	paint.FillShape(gtx.Ops, tema.Contorno, clip.Stroke{Path: p.End(), Width: 1}.Op())
}

// formaItemAtivo: o item ativo da lateral, que corre até a borda direita
// dela e se funde à folha — canto arredondado à esquerda e as curvas
// côncavas em cima e embaixo, na borda direita (o .gt-nav-tab da central).
func formaItemAtivo(gtx layout.Context, size image.Point) clip.PathSpec {
	r, c := float32(gtx.Dp(12)), float32(gtx.Dp(12))
	w, h := float32(size.X), float32(size.Y)
	var p clip.Path
	p.Begin(gtx.Ops)
	p.MoveTo(f32.Pt(r, 0))
	p.LineTo(f32.Pt(w-c, 0))
	quarto(&p, f32.Pt(w-c, 0), f32.Pt(w, 0), f32.Pt(w, -c))
	p.LineTo(f32.Pt(w, h+c))
	quarto(&p, f32.Pt(w, h+c), f32.Pt(w, h), f32.Pt(w-c, h))
	p.LineTo(f32.Pt(r, h))
	quarto(&p, f32.Pt(r, h), f32.Pt(0, h), f32.Pt(0, h-r))
	p.LineTo(f32.Pt(0, r))
	quarto(&p, f32.Pt(0, r), f32.Pt(0, 0), f32.Pt(r, 0))
	p.Close()
	return p.End()
}

// tinta é o Texto (ou TopoTxt) a uma fração de opacidade: o hover discreto
// e o fundo de trilho que funcionam sobre qualquer superfície, clara ou
// escura, sem token próprio para cada uma. No escuro a fração cai para
// menos da metade: o Gio compõe em espaço LINEAR, e branco a 6% sobre o
// quase-preto sai muito mais claro do que a conta em sRGB promete (o
// mesmo aviso de Vidro2 e Luz1 em tema.go) — o trilho virava um bloco
// cinza claro.
func tinta(c color.NRGBA, a float64) color.NRGBA {
	if tema.Escuro {
		a *= 0.4
	}
	return comAlfa(c, a)
}

// ferramenta é o botão só-ícone da cápsula de ferramentas do topo e da
// barra da sessão: 30×26, raio 8, e em relevo (Elevado) sob o mouse.
func ferramenta(gtx layout.Context, btn *widget.Clickable, ic *widget.Icon, cor color.NRGBA) layout.Dimensions {
	return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		size := image.Pt(gtx.Dp(30), gtx.Dp(26))
		gtx.Constraints.Min, gtx.Constraints.Max = size, size
		pointer.CursorPointer.Add(gtx.Ops)
		if btn.Hovered() {
			superficie(gtx, size, tema.Elevado, transparente, 8)
			if cor == tema.TopoSec {
				cor = tema.TopoTxt
			}
		}
		layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return icone(gtx, ic, cor, 17)
		})
		return layout.Dimensions{Size: size}
	})
}

// ferramentaTexto: o mesmo botão com um rótulo curto no lugar do ícone
// (o "A+" do tamanho do texto).
func ferramentaTexto(gtx layout.Context, th *material.Theme, btn *widget.Clickable, rot string) layout.Dimensions {
	return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		cor := tema.TopoSec
		if btn.Hovered() {
			cor = tema.TopoTxt
		}
		gtx.Constraints.Min = image.Pt(gtx.Dp(30), gtx.Dp(26))
		conteudo := op.Record(gtx.Ops)
		d := layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: 6, Right: 6}.Layout(gtx,
				negrito(txt(th, fonteSans, unit.Sp(12), rot, cor)).Layout)
		})
		desenho := conteudo.Stop()
		if btn.Hovered() {
			superficie(gtx, d.Size, tema.Elevado, transparente, 9)
		}
		desenho.Add(gtx.Ops)
		return d
	})
}

// capsula agrupa ferramentas num fundo único e discreto (a cápsula do
// topo): quem é do mesmo assunto fica junto, e o topo deixa de ser uma
// fileira de botões soltos do mesmo peso.
func capsula(gtx layout.Context, filhos ...layout.FlexChild) layout.Dimensions {
	return layout.Background{}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			superficie(gtx, gtx.Constraints.Min, tinta(tema.TopoTxt, 0.05), transparente, 12)
			return layout.Dimensions{Size: gtx.Constraints.Min}
		},
		func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(2).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, filhos...)
			})
		},
	)
}

// divisoria: risco vertical curto entre grupos de uma cápsula.
func divisoria(gtx layout.Context, cor color.NRGBA) layout.Dimensions {
	w, h := gtx.Dp(1), gtx.Dp(16)
	sz := image.Pt(w+gtx.Dp(8), h)
	defer op.Offset(image.Pt(gtx.Dp(4), 0)).Push(gtx.Ops).Pop()
	paint.FillShape(gtx.Ops, cor, clip.Rect{Max: image.Pt(w, h)}.Op())
	return layout.Dimensions{Size: sz}
}

// botaoJanela: minimizar e maximizar, 34×26. botaoJanelaFechar fica
// vermelho cheio sob o mouse (ver ui-pesos-botoes: fechar é a exceção).
func botaoJanela(gtx layout.Context, btn *widget.Clickable, ic *widget.Icon) layout.Dimensions {
	return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		size := image.Pt(gtx.Dp(34), gtx.Dp(26))
		gtx.Constraints.Min, gtx.Constraints.Max = size, size
		pointer.CursorPointer.Add(gtx.Ops)
		cor := tema.TopoSec
		if btn.Hovered() {
			superficie(gtx, size, tinta(tema.TopoTxt, 0.08), transparente, 8)
			cor = tema.TopoTxt
		}
		layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return icone(gtx, ic, cor, 16)
		})
		return layout.Dimensions{Size: size}
	})
}

func botaoJanelaFechar(gtx layout.Context, btn *widget.Clickable) layout.Dimensions {
	return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		size := image.Pt(gtx.Dp(34), gtx.Dp(26))
		gtx.Constraints.Min, gtx.Constraints.Max = size, size
		pointer.CursorPointer.Add(gtx.Ops)
		cor := tema.TopoSec
		if btn.Hovered() {
			superficie(gtx, size, hex(0xd13438), transparente, 8)
			cor = hex(0xffffff)
		}
		layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return icone(gtx, iconeFechar, cor, 16)
		})
		return layout.Dimensions{Size: size}
	})
}

// botaoAcao é o botão da barra do Painel ("Nova conexão", "Recarregar
// INI"): 40 de altura — a mesma da busca e do trilho ao lado —, raio 12,
// ícone e rótulo. primario pinta o azul
// cheio (ação principal da tela); senão é a tinta do card com borda.
func botaoAcao(gtx layout.Context, th *material.Theme, btn *widget.Clickable,
	ic *widget.Icon, rot string, primario bool) layout.Dimensions {

	return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		alt := gtx.Dp(alturaBarra)
		gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = alt, alt
		pointer.CursorPointer.Add(gtx.Ops)
		fundo, borda, cor := tema.CardTinta, tema.CardBorda, tema.Texto
		f := fonteSansMedia
		if btn.Hovered() {
			fundo = tema.CardTintaH
		}
		if primario {
			fundo, borda, cor = tema.AzulCheio, transparente, hex(0xffffff)
			f = fonteSans
			f.Weight = font.SemiBold
			if btn.Hovered() {
				fundo = tema.AzulCheioH
			}
		}
		// O fundo vem depois de medir, no tamanho do conteúdo: o mínimo de
		// altura não chega a um filho de Stack, e o botão saía baixinho.
		gtx.Constraints.Min.X = 0
		conteudo := op.Record(gtx.Ops)
		d := centroV(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: 12, Right: 16}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return icone(gtx, ic, cor, 17)
					}),
					layout.Rigid(layout.Spacer{Width: 8}.Layout),
					layout.Rigid(txt(th, f, unit.Sp(13.5), rot, cor).Layout),
				)
			})
		})
		desenho := conteudo.Stop()
		superficie(gtx, d.Size, fundo, borda, 12)
		desenho.Add(gtx.Ops)
		return d
	})
}

// contador: a pílula com um número (total do grupo), 20 de altura.
func contador(gtx layout.Context, th *material.Theme, n string) layout.Dimensions {
	return layout.Background{}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			sz := gtx.Constraints.Min
			paint.FillShape(gtx.Ops, tinta(tema.Texto, 0.10),
				clip.UniformRRect(image.Rectangle{Max: sz}, sz.Y/2).Op(gtx.Ops))
			return layout.Dimensions{Size: sz}
		},
		func(gtx layout.Context) layout.Dimensions {
			alt := gtx.Dp(20)
			gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = alt, alt
			gtx.Constraints.Min.X = gtx.Dp(24)
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: 7, Right: 7}.Layout(gtx,
					negrito(txt(th, fonteSans, unit.Sp(11.5), n, tema.Sec)).Layout)
			})
		},
	)
}

// trilho é o seletor segmentado da repaginação: um fundo afundado e os
// segmentos dentro dele; o escolhido fica em RELEVO (Elevado). Com
// escolhido < 0 nenhum fica marcado — é o par de ações "Recolher tudo /
// Expandir tudo", que são botões e não opções.
func trilho(gtx layout.Context, n, escolhido int, btns []*widget.Clickable,
	conteudo func(gtx layout.Context, i int, ativo, hover bool) layout.Dimensions) layout.Dimensions {

	var filhos []layout.FlexChild
	for i := 0; i < n; i++ {
		i := i
		if i > 0 {
			filhos = append(filhos, layout.Rigid(layout.Spacer{Width: 3}.Layout))
		}
		filhos = append(filhos, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return btns[i].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				pointer.CursorPointer.Add(gtx.Ops)
				ativo, hover := i == escolhido, btns[i].Hovered()
				return layout.Stack{}.Layout(gtx,
					layout.Expanded(func(gtx layout.Context) layout.Dimensions {
						if ativo || hover {
							superficie(gtx, gtx.Constraints.Min, tema.Elevado, transparente, 8)
						}
						return layout.Dimensions{Size: gtx.Constraints.Min}
					}),
					layout.Stacked(func(gtx layout.Context) layout.Dimensions {
						return conteudo(gtx, i, ativo, hover)
					}),
				)
			})
		}))
	}
	return layout.Background{}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			superficie(gtx, gtx.Constraints.Min, tinta(tema.Texto, 0.06), transparente, trilhoRaio)
			return layout.Dimensions{Size: gtx.Constraints.Min}
		},
		func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(3).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, filhos...)
			})
		},
	)
}

// hexagono pinta um hexágono de ponta para cima, inscrito num quadrado de
// lado px — a marca de cor do grupo (no lugar do trilho vertical de antes).
func hexagono(gtx layout.Context, lado int, cor color.NRGBA) layout.Dimensions {
	r := float32(lado) / 2
	cx, cy := r, r
	var p clip.Path
	p.Begin(gtx.Ops)
	for k := 0; k < 6; k++ {
		a := math.Pi/3*float64(k) - math.Pi/2
		pt := f32.Pt(cx+r*float32(math.Cos(a)), cy+r*float32(math.Sin(a)))
		if k == 0 {
			p.MoveTo(pt)
		} else {
			p.LineTo(pt)
		}
	}
	p.Close()
	paint.FillShape(gtx.Ops, cor, clip.Outline{Path: p.End()}.Op())
	return layout.Dimensions{Size: image.Pt(lado, lado)}
}

// corMarca é a cor viva da Atalaia, fixa (não muda com o tema): a marca é
// da família do ChamadoJá — quadrado carvão, UMA cor viva, um objeto e um
// sinal de movimento —, e lá a cor é o lima; aqui, o azul.
var (
	corMarca      = hex(0x5aa2ff)
	carvaoDaMarca = hex(0x1c211f)
)

// marcaApp é a marca da Atalaia num quadrado de lado px: as três linhas de
// movimento chegando pela esquerda e a tela com o prompt ">_" (a opção
// "A5", prompt grande, escolhida por aguentar bem os tamanhos pequenos),
// sem o quadrado carvão de fundo. Medidas numa grade de 100, as mesmas do
// SVG em icones/acessos.svg; o conteúdo vai de x 6,5 a 91 e de y 21 a 79,
// e o desenho escala para ocupar a largura do quadrado.
func marcaApp(gtx layout.Context, lado int) layout.Dimensions {
	k := float32(lado) / 88
	dx, dy := -4*k, (float32(lado)-58*k)/2-21*k
	pt := func(x, y float32) f32.Point { return f32.Pt(x*k+dx, y*k+dy) }
	rr := func(x0, y0, x1, y1, r float32) clip.RRect {
		a, b := pt(x0, y0), pt(x1, y1)
		return clip.UniformRRect(image.Rect(int(a.X+0.5), int(a.Y+0.5), int(b.X+0.5), int(b.Y+0.5)), int(r*k+0.5))
	}
	// linhas de movimento (pontas redondas: retângulos de raio meia altura)
	paint.FillShape(gtx.Ops, corMarca, rr(9.5, 39.5, 19.5, 44.5, 2.5).Op(gtx.Ops))
	paint.FillShape(gtx.Ops, corMarca, rr(6.5, 47.5, 19.5, 52.5, 2.5).Op(gtx.Ops))
	paint.FillShape(gtx.Ops, corMarca, rr(9.5, 55.5, 19.5, 60.5, 2.5).Op(gtx.Ops))
	// a tela
	paint.FillShape(gtx.Ops, corMarca, rr(23, 21, 91, 79, 12).Op(gtx.Ops))
	// o prompt: ">" e o cursor "_"
	var p clip.Path
	p.Begin(gtx.Ops)
	p.MoveTo(pt(36, 37))
	p.LineTo(pt(49, 50))
	p.LineTo(pt(36, 63))
	paint.FillShape(gtx.Ops, carvaoDaMarca, clip.Stroke{Path: p.End(), Width: 8 * k}.Op())
	paint.FillShape(gtx.Ops, carvaoDaMarca, rr(55, 56, 80, 64, 4).Op(gtx.Ops))
	return layout.Dimensions{Size: image.Pt(lado, lado)}
}

// ------------------------------------------------------------ malha

// A malha de hexágonos do Painel — a mesma do cartão de boas-vindas da
// central do ChamadoJá: hexágonos de topo plano, traço finíssimo, que somem
// em direção ao conteúdo, e algumas células acesas na cor de acento. São
// dois trechos: um dentro do cartão de boas-vindas (malhaCartao) e um
// solto na folha, subindo do canto inferior esquerdo e se desfazendo
// (malhaBaixo).
//
// As formas só mudam com o tamanho, o tema ou a escala, então ficam
// gravadas uma vez por trecho e cada quadro só repete a chamada — são
// algumas centenas de traços.
const malhaRaio = 22 // dp

type trechoMalha struct {
	larg, alt unit.Dp
	// deBaixo: o trecho nasce no canto INFERIOR esquerdo e some para cima
	// e para a direita; senão nasce no superior direito e some para a
	// esquerda e para baixo.
	deBaixo bool
	// cartao: o trecho mora dentro do cartão de boas-vindas (recortado
	// nele) e só esmaece para a esquerda — a fórmula do cartão da central:
	// opacidade max(0,08; (x/L − 0,15)/0,7).
	cartao bool
	acesas []celulaAcesa

	ops    op.Ops
	call   op.CallOp
	ok     bool
	tam    image.Point
	escuro bool
}

// celulaAcesa: (coluna, linha) contadas a partir do canto de onde o
// trecho nasce, e o jeito de pintar — 'a' cheia com traço, 'b' só cheia,
// 'c' só traço, 'v' verde.
type celulaAcesa struct {
	c, l int
	tipo byte
}

var (
	malhaCartao = &trechoMalha{cartao: true, acesas: []celulaAcesa{
		{1, 0, 'a'}, {6, 2, 'a'},
		{3, 1, 'b'}, {7, 1, 'b'}, {9, 0, 'b'}, {0, 2, 'b'},
		{5, 0, 'c'}, {11, 1, 'c'},
		{2, 2, 'v'},
	}}
	malhaBaixo = &trechoMalha{larg: 1100, alt: 600, deBaixo: true, acesas: []celulaAcesa{
		{1, 1, 'a'}, {3, 0, 'b'}, {0, 2, 'b'}, {2, 2, 'v'},
		{5, 1, 'c'}, {7, 0, 'b'}, {4, 3, 'b'}, {9, 2, 'a'}, {12, 1, 'b'},
		{2, 5, 'c'}, {6, 4, 'v'}, {15, 0, 'b'}, {1, 7, 'b'},
	}}
)

// malhaHex é o trecho solto na folha, no canto inferior esquerdo. O de
// cima mora dentro do cartão de boas-vindas (malhaCartao, ver
// dashTab.cabecalho): solto na folha, ele deixava um vão em diagonal
// entre o título e o canto, e esticado pela faixa inteira pesava demais.
func malhaHex(gtx layout.Context) {
	w, h := gtx.Dp(malhaBaixo.larg), gtx.Dp(malhaBaixo.alt)
	malhaBaixo.desenharEm(gtx, w, h, image.Pt(0, gtx.Constraints.Max.Y-h))
}

// desenharEm desenha o trecho numa caixa w×h em pos (px), refazendo as
// formas só quando o tamanho ou o tema mudam.
func (m *trechoMalha) desenharEm(gtx layout.Context, w, h int, pos image.Point) {
	tam := image.Pt(w, h)
	if w <= 0 || h <= 0 {
		return
	}
	if !m.ok || m.tam != tam || m.escuro != tema.Escuro {
		m.ops.Reset()
		rec := op.Record(&m.ops)
		m.formas(&m.ops, tam, gtx.Metric.PxPerDp)
		m.call = rec.Stop()
		m.ok, m.tam, m.escuro = true, tam, tema.Escuro
	}
	defer op.Offset(pos).Push(gtx.Ops).Pop()
	m.call.Add(gtx.Ops)
}

func (m *trechoMalha) formas(o *op.Ops, tam image.Point, pxDp float32) {
	R := malhaRaio * pxDp
	dx, dy := 1.5*R, float32(math.Sqrt(3))*R
	w, h := float32(tam.X), float32(tam.Y)
	// (c, l) contados a partir do canto de origem do trecho; o centro é
	// espelhado para o canto certo.
	centro := func(c, l int) f32.Point {
		y := float32(l) * dy
		if c%2 == 1 {
			y += dy / 2
		}
		x := float32(c) * dx
		if m.deBaixo {
			return f32.Pt(x+2*pxDp, h-y-2*pxDp)
		}
		return f32.Pt(w-x-2*pxDp, y+2*pxDp)
	}
	// máscara: 1 no canto de origem, sumindo até 0 na borda oposta, mais
	// devagar no comprido do trecho do que no curto.
	mascara := func(p f32.Point) float32 {
		fx, fy := 1-p.X/w, p.Y/h // distância relativa ao canto de origem
		if m.deBaixo {
			fx, fy = p.X/w, 1-p.Y/h
		}
		if m.cartao {
			return min(max(((1-fx)-0.15)/0.7, 0.08), 1)
		}
		if m.deBaixo {
			// quarto de elipse a partir do canto: a tinta fica mais
			// transparente aos poucos, do canto até zero na borda; só no
			// finzinho os lados começam a faltar (ver dissolver, abaixo)
			d := float32(math.Sqrt(float64(fx*fx + fy*fy)))
			if d >= 1 {
				return 0
			}
			return float32(math.Pow(float64(1-d), 1.4))
		}
		a := (1 - fx) * (1 - fy*fy)
		return min(max(a*1.15, 0), 1)
	}
	hexPath := func(c f32.Point, r float32) clip.PathSpec {
		var p clip.Path
		p.Begin(o)
		for k := 0; k < 6; k++ {
			a := math.Pi / 3 * float64(k)
			pt := f32.Pt(c.X+r*float32(math.Cos(a)), c.Y+r*float32(math.Sin(a)))
			if k == 0 {
				p.MoveTo(pt)
			} else {
				p.LineTo(pt)
			}
		}
		p.Close()
		return p.End()
	}

	// Discreta de propósito: é fundo, e passa atrás de nome de grupo e de
	// número. Mais forte que isto (a primeira versão era 0,13 no claro)
	// atrapalhava a leitura.
	traco, acesa := 0.075, 0.6
	if tema.Escuro {
		traco, acesa = 0.05, 0.6
	}
	if m.cartao {
		// dentro do cartão ela pode aparecer mais: está contida, e não
		// passa atrás de nome de grupo nem de card
		traco, acesa = 0.13, 1.0
		if tema.Escuro {
			traco, acesa = 0.08, 0.85
		}
	}
	if m.deBaixo {
		// mais forte que os outros: este trecho se desfaz (abaixo) e, com
		// a tinta fina do resto, no claro ele quase não aparecia
		traco = 0.22
		if tema.Escuro {
			traco = 0.07
		}
	}
	colunas := int(w/dx) + 2
	linhas := int(h/dy) + 2
	defer clip.Rect{Max: tam}.Push(o).Pop()
	for c := 0; c < colunas; c++ {
		for l := -1; l < linhas; l++ {
			ct := centro(c, l)
			a := mascara(ct)
			if a <= 0.02 {
				continue
			}
			cor := comAlfa(tema.Texto, traco*float64(a))
			if !m.deBaixo {
				paint.FillShape(o, cor, clip.Stroke{Path: hexPath(ct, R), Width: 1.2 * pxDp}.Op())
				continue
			}
			// DISSOLVER: no último trecho antes da borda (d > 0,7), já quase
			// transparente, o hexágono vai perdendo lados até não sobrar
			// nenhum. O sorteio de cada lado é fixo (função da posição),
			// então o desenho não pisca de um quadro para o outro.
			fx, fy := ct.X/w, 1-ct.Y/h
			d := float32(math.Sqrt(float64(fx*fx + fy*fy)))
			chance := min(max((1-d)/0.3, 0), 1)
			var p clip.Path
			p.Begin(o)
			algum := false
			for k := 0; k < 6; k++ {
				if sorteioLado(c, l, k) >= chance {
					continue
				}
				a0, a1 := math.Pi/3*float64(k), math.Pi/3*float64(k+1)
				p.MoveTo(f32.Pt(ct.X+R*float32(math.Cos(a0)), ct.Y+R*float32(math.Sin(a0))))
				p.LineTo(f32.Pt(ct.X+R*float32(math.Cos(a1)), ct.Y+R*float32(math.Sin(a1))))
				algum = true
			}
			lados := p.End()
			if algum {
				paint.FillShape(o, cor, clip.Stroke{Path: lados, Width: 1.2 * pxDp}.Op())
			}
		}
	}
	for _, cel := range m.acesas {
		ct := centro(cel.c, cel.l)
		a := float64(mascara(ct)) * acesa
		if m.deBaixo {
			fx, fy := ct.X/w, 1-ct.Y/h
			if d := math.Sqrt(float64(fx*fx + fy*fy)); d > 0.6 {
				continue
			} else {
				a *= 1 - d/0.6
			}
		}
		r := R - 3*pxDp
		switch cel.tipo {
		case 'a':
			paint.FillShape(o, comAlfa(tema.AzulCheio, 0.22*a), clip.Outline{Path: hexPath(ct, r)}.Op())
			paint.FillShape(o, comAlfa(tema.AzulCheio, 0.5*a), clip.Stroke{Path: hexPath(ct, r), Width: 1.2 * pxDp}.Op())
		case 'b':
			paint.FillShape(o, comAlfa(tema.AzulCheio, 0.11*a), clip.Outline{Path: hexPath(ct, r)}.Op())
		case 'c':
			paint.FillShape(o, comAlfa(tema.AzulCheio, 0.55*a), clip.Stroke{Path: hexPath(ct, r), Width: 2 * pxDp}.Op())
		case 'v':
			paint.FillShape(o, comAlfa(tema.Verde, 0.18*a), clip.Outline{Path: hexPath(ct, r)}.Op())
		}
	}
}

// cartaoTerminal põe o terminal SSH numa folha própria, com canto, borda e
// sombra, recuada da folha do app. Quem desenha dentro recebe a área já
// recuada (e o ponteiro relativo a ela), sem saber do cartão.
func cartaoTerminal(gtx layout.Context, w layout.Widget) layout.Dimensions {
	return layout.Inset{Left: 10, Right: 10, Bottom: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		const raioDp = 12
		sombra(gtx, size, raioDp)
		rr := clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(raioDp))
		paint.FillShape(gtx.Ops, tema.TermBg, rr.Op(gtx.Ops))
		molde := rr.Push(gtx.Ops)
		layout.UniformInset(4).Layout(gtx, w)
		molde.Pop()
		paint.FillShape(gtx.Ops, tema.CardBorda, clip.Stroke{Path: rr.Path(gtx.Ops), Width: 1}.Op())
		return layout.Dimensions{Size: size}
	})
}

// pontoVida é o estado de vida de uma máquina: o ponto de 7dp com um halo
// na cor dele (verde respondeu, vermelho não). Sem estado — ainda não
// perguntei, que é diferente de "está fora" — fica só o contorno.
func pontoVida(gtx layout.Context, cor color.NRGBA, mostra bool) layout.Dimensions {
	lado := gtx.Dp(13)
	c := float32(lado) / 2
	circ := func(r float32) clip.Ellipse {
		return clip.Ellipse{Min: image.Pt(int(c-r+0.5), int(c-r+0.5)), Max: image.Pt(int(c+r+0.5), int(c+r+0.5))}
	}
	r := float32(gtx.Dp(7)) / 2
	if mostra {
		paint.FillShape(gtx.Ops, comAlfa(cor, 0.2), circ(float32(lado)/2).Op(gtx.Ops))
		paint.FillShape(gtx.Ops, cor, circ(r).Op(gtx.Ops))
	} else {
		e := circ(r)
		paint.FillShape(gtx.Ops, tema.Borda2, clip.Stroke{Path: e.Path(gtx.Ops), Width: float32(gtx.Dp(1.5))}.Op())
	}
	return layout.Dimensions{Size: image.Pt(lado, lado)}
}

// centroV encosta w à esquerda e o CENTRALIZA na altura mínima de quem
// chama. O layout.W do Gio não serve sozinho: ele só zera o mínimo de
// largura e repassa o de altura, e um rótulo que recebe a altura toda
// desenha o texto colado no alto da linha.
func centroV(gtx layout.Context, w layout.Widget) layout.Dimensions {
	return layout.W.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.Y = 0
		return w(gtx)
	})
}

// fundoCampo é o fundo de todo campo de texto: Campo com a borda do card
// e, com o foco, o anel de 3dp na cor de acento por fora — o mesmo da
// busca do Painel.
func fundoCampo(gtx layout.Context, size image.Point, focado bool) {
	borda := tema.CardBorda
	if focado {
		a := gtx.Dp(3)
		rr := clip.UniformRRect(image.Rect(-a, -a, size.X+a, size.Y+a), gtx.Dp(10)+a)
		paint.FillShape(gtx.Ops, comAlfa(tema.AzulCheio, 0.2), rr.Op(gtx.Ops))
		borda = comAlfa(tema.AzulCheio, 0.55)
	}
	superficie(gtx, size, tema.Campo, borda, 10)
}

// sorteioLado devolve um número fixo em [0,1) para o lado k do hexágono
// (c, l) — o "acaso" do dissolver da malha, sem estado e sem piscar.
func sorteioLado(c, l, k int) float32 {
	h := uint32(c)*73856093 ^ uint32(l+16)*19349663 ^ uint32(k)*83492791
	h ^= h >> 13
	h *= 0x5bd1e995
	h ^= h >> 15
	return float32(h%1000) / 1000
}
