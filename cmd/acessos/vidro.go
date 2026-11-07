package main

import (
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

// VIDRO FOSCO
//
// O Gio não tem blur. Translucidez sem blur deixa o fundo ATRAVESSAR o
// cartão com forma e tudo — a faixa diagonal cortando o card, a palavra
// "REMOTO" do canto aparecendo atrás do nome da máquina —, e por isso o
// card tinha de ser quase opaco (Vidro2 0,78) para continuar legível.
//
// O fundo da folha, porém, não depende do conteúdo: é função só do
// tamanho da janela, do tema e da escala (a Pagina com a luz no canto, ver
// renderFosco). Então dá para refazê-lo na CPU, uma vez, em baixa
// resolução, desfocar, e pintar essa imagem atrás de cada cartão. O que o
// patch do fork (paint.ImageOp.Tela) acrescenta é a âncora: a imagem fica
// presa à JANELA e não ao cartão, então cada cartão mostra exatamente o
// pedaço do fundo que está atrás dele — com rolagem, com hover, sem o
// cartão saber onde está na tela. Desde a 2.9 a mesma imagem é o fundo da
// própria folha (fundoFolha) e da aba ativa, que se funde a ela.
//
// Vale só para superfícies que ficam DIRETO sobre o fundo da janela
// (cards do Painel). Diálogo e menu ficam por cima de outros cartões,
// que esta imagem não tem — neles o vidro mostraria o fundo "por trás"
// dos cards, como raio X.

const (
	// foscoEscala: cada texel cobre foscoEscala×foscoEscala pixels da
	// janela. O desfoque apaga o detalhe de qualquer jeito; em 1/8 a
	// imagem de uma janela 1920×1080 tem 33 mil texels e sai em poucos
	// milissegundos, o bastante para refazer a cada passo de um resize.
	foscoEscala = 8
	// foscoSigma é o raio do desfoque, em pixels da janela.
	foscoSigma = 22
)

var fosco struct {
	tam    image.Point
	escuro bool
	pxDp   float32
	topo   int
	img    paint.ImageOp
	ok     bool
}

// atualizarFosco refaz a imagem desfocada quando o fundo muda. Chamada
// por fundoJanela, então só a janela principal tem fosco — e é só nela
// que há cards sobre o fundo.
func atualizarFosco(gtx layout.Context, size image.Point) {
	if size.X <= 0 || size.Y <= 0 {
		return
	}
	px := gtx.Metric.PxPerDp
	topo := gtx.Dp(topBarAltura)
	if fosco.ok && fosco.tam == size && fosco.escuro == tema.Escuro && fosco.pxDp == px && fosco.topo == topo {
		return
	}
	img := renderFosco(size, px, topo)
	fosco.img = paint.NewImageOp(img)
	fosco.img.Filter = paint.FilterLinear
	fosco.img.Tela = foscoEscala
	fosco.tam, fosco.escuro, fosco.pxDp, fosco.topo, fosco.ok = size, tema.Escuro, px, topo, true
}

// superficieVidro: vidro fosco. Por baixo, o fundo da janela desfocado;
// por cima, a tinta (CardTinta nos cards), a mesma luz de superficieRelevo
// e a borda. Com borda transparente, a borda é de LUZ: clara em cima,
// apagando para baixo, como a quina de um vidro iluminado de cima — que
// é de onde a sombra (sombra.go) diz que a luz vem.
func superficieVidro(gtx layout.Context, size image.Point, tinta, borda color.NRGBA, raioDp unit.Dp) {
	var fundo *paint.ImageOp
	if fosco.ok {
		fundo = &fosco.img
	}
	pintarVidro(gtx, size, tinta, borda, raioDp, fundo)
}

func pintarVidro(gtx layout.Context, size image.Point, tinta, borda color.NRGBA, raioDp unit.Dp, fundo *paint.ImageOp) {
	if size.X <= 0 || size.Y <= 0 {
		return
	}
	rr := clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(raioDp))
	molde := rr.Push(gtx.Ops)
	if fundo != nil {
		fundo.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
	}
	paint.ColorOp{Color: tinta}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	molde.Pop()
	brilhoRelevo(gtx, size, raioDp)

	cima, baixo := rgba(0xffffff, 0.85), rgba(0x11161a, 0.10)
	if tema.Escuro {
		cima, baixo = rgba(0xffffff, 0.11), rgba(0xffffff, 0.03)
	}
	bordaDeLuz(gtx, rr, size, cima, baixo, borda)
}

// bordaDeLuz: traço de 1px em degradê (cima → baixo), com uma borda
// chapada opcional por cima — é assim que o hover do card ACRESCENTA a
// Borda2 aos poucos sem a borda de luz sumir de uma vez.
func bordaDeLuz(gtx layout.Context, rr clip.RRect, size image.Point, cima, baixo, borda color.NRGBA) {
	traco := clip.Stroke{Path: rr.Path(gtx.Ops), Width: 1}.Op().Push(gtx.Ops)
	paint.LinearGradientOp{
		Stop1: f32.Pt(0, 0), Color1: cima,
		Stop2: f32.Pt(0, float32(size.Y)), Color2: baixo,
	}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	if borda.A > 0 {
		paint.ColorOp{Color: borda}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
	}
	traco.Pop()
}

// superficieCartao: superfície OPACA (diálogo, menu, dica, splash) com a
// mesma linguagem do vidro — brilho do topo e borda de luz —, mas sem
// fundo desfocado: ela fica por cima de outros cards, e o fundo fosco
// mostraria a janela "por trás" deles. No claro a borda é a Borda que
// escurece para baixo (branco sobre branco não teria quina); no escuro,
// luz no topo apagando.
func superficieCartao(gtx layout.Context, size image.Point, raioDp unit.Dp) {
	if size.X <= 0 || size.Y <= 0 {
		return
	}
	rr := clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(raioDp))
	paint.FillShape(gtx.Ops, tema.Cartao, rr.Op(gtx.Ops))
	brilhoRelevo(gtx, size, raioDp)
	cima, baixo := comAlfa(tema.Borda, 0.55), tema.Borda2
	if tema.Escuro {
		cima, baixo = rgba(0xffffff, 0.14), rgba(0xffffff, 0.05)
	}
	bordaDeLuz(gtx, rr, size, cima, baixo, transparente)
}

// ------------------------------------------------------------ CPU

// corLin é cor PRÉ-MULTIPLICADA em espaço linear — o mesmo espaço em que
// o Gio compõe. Refazer o fundo em sRGB daria um tom diferente do da
// janela, e o cartão viraria uma mancha de outra cor.
type corLin struct{ r, g, b, a float32 }

func paraLin(c color.NRGBA) corLin {
	a := float32(c.A) / 255
	return corLin{srgbLin(c.R) * a, srgbLin(c.G) * a, srgbLin(c.B) * a, a}
}

var tabSrgbLin = func() (t [256]float32) {
	for i := range t {
		c := float64(i) / 255
		if c <= 0.04045 {
			t[i] = float32(c / 12.92)
		} else {
			t[i] = float32(math.Pow((c+0.055)/1.055, 2.4))
		}
	}
	return
}()

func srgbLin(v uint8) float32 { return tabSrgbLin[v] }

// tabLinSrgb: a volta, por tabela — math.Pow por canal era metade do
// tempo de renderFosco. 16 mil degraus bastam: perto do preto, onde a
// curva do sRGB é mais íngreme, um degrau vale menos de um nível de 8 bits.
const degrausLin = 1 << 14

var tabLinSrgb = func() (t [degrausLin + 1]uint8) {
	for i := range t {
		c := float64(i) / degrausLin
		if c < 0.0031308 {
			c *= 12.92
		} else {
			c = 1.055*math.Pow(c, 1/2.4) - 0.055
		}
		t[i] = uint8(math.Round(c * 255))
	}
	return
}()

func linSrgb(c float32) uint8 {
	if c <= 0 {
		return 0
	} else if c >= 1 {
		return 255
	}
	return tabLinSrgb[int(c*degrausLin+0.5)]
}

// sobre compõe s por cima de d (Porter-Duff "over", pré-multiplicado).
func sobre(d, s corLin) corLin {
	k := 1 - s.a
	return corLin{s.r + d.r*k, s.g + d.g*k, s.b + d.b*k, s.a + d.a*k}
}

func (c corLin) mul(k float32) corLin { return corLin{c.r * k, c.g * k, c.b * k, c.a * k} }

func misturar(a, b corLin, t float32) corLin {
	return corLin{a.r + (b.r-a.r)*t, a.g + (b.g-a.g)*t, a.b + (b.b-a.b)*t, a.a + (b.a-a.a)*t}
}

// renderFosco desenha, em 1/foscoEscala da resolução, a folha como ela
// fica na janela: a Pagina, a luz no canto superior direito (o
// .gt-page-glow da central do ChamadoJá: elipse de 900×520 presa ao canto
// de cima da folha, transparente a 72% do raio) e, no tema claro, o véu
// claro no meio (700×420, transparente a 75%). Tudo em pixels da janela,
// com a folha começando abaixo do topo (topo, em px). A luz é suave de
// propósito, então o desfoque quase não muda nada nela — ele fica porque a
// mesma imagem é o fundo dos cards de vidro.
func renderFosco(size image.Point, pxDp float32, topo int) *image.RGBA {
	const e = foscoEscala
	w, h := float32(size.X), float32(size.Y)
	lw, lh := size.X/e+2, size.Y/e+2

	base := paraLin(tema.Pagina)
	brilho := paraLin(tema.Brilho)
	veu := paraLin(tema.BrilhoVeu)
	t0 := float32(topo)
	type elipse struct {
		cx, cy, rx, ry, fim float32
		cor                 corLin
	}
	luzes := []elipse{
		{cx: w, cy: t0, rx: 900 * pxDp, ry: 520 * pxDp, fim: 0.72, cor: brilho},
	}
	if veu.a > 0 {
		luzes = append(luzes, elipse{cx: w * 0.45, cy: t0 + (h-t0)*0.45, rx: 700 * pxDp, ry: 420 * pxDp, fim: 0.75, cor: veu})
	}

	buf := make([]corLin, lw*lh)
	for j := range lh {
		for i := range lw {
			cx, cy := (float32(i)+0.5)*e, (float32(j)+0.5)*e
			c := base
			for _, l := range luzes {
				dx, dy := (cx-l.cx)/l.rx, (cy-l.cy)/l.ry
				d := float32(math.Sqrt(float64(dx*dx+dy*dy))) / l.fim
				if d < 1 {
					c = sobre(c, l.cor.mul(1-d))
				}
			}
			buf[j*lw+i] = c
		}
	}

	desfocar(buf, lw, lh, foscoSigma/float64(e))

	img := image.NewRGBA(image.Rect(0, 0, lw, lh))
	for j := range lh {
		for i := range lw {
			c := buf[j*lw+i]
			o := img.PixOffset(i, j)
			img.Pix[o+0] = linSrgb(c.r)
			img.Pix[o+1] = linSrgb(c.g)
			img.Pix[o+2] = linSrgb(c.b)
			img.Pix[o+3] = 255
		}
	}
	return img
}

// desfocar aplica um desfoque gaussiano aproximado por três passadas de
// média móvel em cada eixo (a soma de três caixas já é indistinguível de
// uma gaussiana a olho), com a borda replicada.
func desfocar(buf []corLin, w, h int, sigma float64) {
	if sigma <= 0 {
		return
	}
	tmp := make([]corLin, len(buf))
	for _, r := range raiosCaixa(sigma, 3) {
		caixa(buf, tmp, w, r, 1, w) // horizontal: linhas de w, vizinho ao lado
		caixa(tmp, buf, h, r, w, 1) // vertical: colunas de h, vizinho embaixo
	}
}

// caixa: média móvel de raio r ao longo de cada linha de comprimento m;
// passo é a distância entre vizinhos na linha, salto entre linhas.
func caixa(src, dst []corLin, m, r, passo, salto int) {
	linhas := len(src) / m
	inv := 1 / float32(2*r+1)
	for l := range linhas {
		ini := l * salto
		at := func(i int) corLin {
			if i < 0 {
				i = 0
			} else if i >= m {
				i = m - 1
			}
			return src[ini+i*passo]
		}
		var s corLin
		for i := -r; i <= r; i++ {
			c := at(i)
			s = corLin{s.r + c.r, s.g + c.g, s.b + c.b, s.a + c.a}
		}
		for i := range m {
			dst[ini+i*passo] = s.mul(inv)
			a, b := at(i+r+1), at(i-r)
			s = corLin{s.r + a.r - b.r, s.g + a.g - b.g, s.b + a.b - b.b, s.a + a.a - b.a}
		}
	}
}

// raiosCaixa: raios de n caixas cuja soma aproxima uma gaussiana de
// desvio sigma (receita de Kovesi).
func raiosCaixa(sigma float64, n int) []int {
	wIdeal := math.Sqrt(12*sigma*sigma/float64(n) + 1)
	wl := int(math.Floor(wIdeal))
	if wl%2 == 0 {
		wl--
	}
	wu := wl + 2
	mIdeal := (12*sigma*sigma - float64(n*wl*wl) - 4*float64(n*wl) - 3*float64(n)) / (-4*float64(wl) - 4)
	m := int(math.Round(mIdeal))
	r := make([]int, n)
	for i := range r {
		if i < m {
			r[i] = (wl - 1) / 2
		} else {
			r[i] = (wu - 1) / 2
		}
	}
	return r
}
