package main

import (
	"image"
	"image/color"
	"math"
	"os"
	"sync"
	"sync/atomic"

	"gio.tools/icons"
	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// Tema é o dicionário de tokens — mesma ideia do TEMAS do tema.py: cor
// NUNCA aparece literal num componente, só o token. Trocar de tema é
// trocar o dicionário inteiro, sem folha duplicada.
//
// Os valores são os MESMOS do tema.py (calibrados em tela real, não
// derivados no papel).
type Tema struct {
	Fundo, Cartao, Borda, Borda2 color.NRGBA
	Texto, Sec, Fraco            color.NRGBA
	// CardFraco é o Fraco QUANDO o texto está por cima do vidro do card
	// (VidroTinta/VidroTintaH sobre o fundo desfocado), não do fundo da
	// janela — ver comentário no valor de cada tema.
	CardFraco              color.NRGBA
	Fundo1, Fundo2, Fundo3 color.NRGBA
	Luz1, Luz2             color.NRGBA
	Vidro1, Vidro2, Vidro3 color.NRGBA
	// VidroTinta/VidroTintaH são a tinta do vidro FOSCO (superficieVidro):
	// por baixo dela já vai o fundo da janela desfocado, então ela pode
	// ser bem mais fina que Vidro2/Vidro3 sem o fundo "atravessar" o
	// cartão — o que atravessa é só cor, sem forma.
	VidroTinta, VidroTintaH color.NRGBA
	// BuscaVidro é o fundo da caixa de busca do atalho global, que
	// flutua sobre o DESKTOP e não sobre o fundo do app — por isso não
	// reaproveita Vidro*: aqui o que está atrás é imprevisível.
	BuscaVidro color.NRGBA
	// BuscaFosco é o mesmo fundo quando o compositor DESFOCA o desktop
	// atrás da caixa (ver buscapop.go): pode ser bem mais fino, porque o
	// que atravessa é só cor, sem forma.
	BuscaFosco                                  color.NRGBA
	LuzB                                        color.NRGBA
	Barra, BarraFosca                           color.NRGBA
	HeroLuz1, HeroLuz2                          color.NRGBA
	Topo1, Topo2, Titulo1, Titulo2, TituloBorda color.NRGBA
	Abas1, Abas2                                color.NRGBA
	TopoTxt, TopoSec                            color.NRGBA
	Vidro, VidroH, VidroB                       color.NRGBA
	Palco, TermBg, TermFg, TermSel              color.NRGBA
	// Ansi é a paleta de 16 cores do terminal (índices 0-15 do ANSI
	// clássico). Cor SÓLIDA, sem alfa nenhum — o terminal já tratava
	// seleção/cursor com transparência calculada em cima de um fundo
	// fixo; virar tema deixaria essa conta errada (o mesmo alfa sobre um
	// fundo bem mais claro lava quase invisível). Cada tema define a
	// paleta inteira já calibrada pro próprio TermBg.
	Ansi                     [16]color.NRGBA
	Hover, Campo             color.NRGBA
	OkBg, OkFg               color.NRGBA
	ErroBg, ErroFg, ErroH    color.NRGBA
	AtencaoBg, AtencaoFg     color.NRGBA
	Azul, AzulH, Verde, Roxo color.NRGBA
	// AzulCheio/AzulCheioH: o azul quando ele é FUNDO de algo com conteúdo
	// branco por cima (botão primário, caixinha marcada). No claro é o
	// próprio Azul; no escuro o Azul (#748ffc) é claro demais para isso —
	// rótulo branco dava 2,97:1.
	AzulCheio, AzulCheioH color.NRGBA
	AzulFraco, VerdeFraco color.NRGBA
	RoxoFraco             color.NRGBA
	Hero1, Hero2, HeroTxt color.NRGBA
	HeroSec               color.NRGBA
	// Repaginação da 2.9 (a "moldura e folha" da central do ChamadoJá):
	// Moldura é o tom do topo, da lateral e da barra de status; Pagina é a
	// FOLHA onde o conteúdo mora. A aba ativa e o item ativo da lateral são
	// pintados com a folha e se fundem a ela — é o que liga o cromo ao
	// conteúdo. MolduraLinha é o fio entre a moldura e a folha.
	Moldura, MolduraLinha, Pagina color.NRGBA
	// Contorno é o traço que DESENHA a folha: corre na base do topo e na
	// borda da lateral, e contorna a aba ativa e o item ativo da lateral
	// (que interrompem a linha e se abrem para a folha) — o "recorte" do
	// menu da central do ChamadoJá, que deixa óbvio qual está aberta.
	Contorno color.NRGBA
	// Brilho é a luz no canto superior direito da folha (ver fundoFolha);
	// BrilhoVeu, o véu claro no meio dela (só no tema claro).
	Brilho, BrilhoVeu color.NRGBA
	// Elevado: o que fica EM RELEVO dentro de um trilho (segmento
	// escolhido, protocolo configurado no card, botão sob o mouse);
	// Afundado: o trilho em si.
	Elevado, Afundado color.NRGBA
	// CardTinta/CardTintaH: a tinta do card sobre a folha (fosca por
	// baixo), em repouso e sob o mouse; CardBorda, a borda dele.
	CardTinta, CardTintaH, CardBorda color.NRGBA
	// Escuro diz qual paleta é esta: o Gio compõe overlay translúcido
	// mais forte sobre fundo escuro (espaço linear, não sRGB) e quem pinta
	// translucidez precisa de alfa bem mais baixo ali.
	Escuro bool
}

var temaClaro = Tema{
	Fundo: hex(0xd9e2ee), Cartao: hex(0xffffff), Borda: hex(0xc7cfd8), Borda2: hex(0xacb6c2),
	// Sec e Fraco escureceram na revisão de contraste (2026-10-18), no
	// MESMO tom: Fraco #94a1ad dava 2,6:1 no branco (abaixo do mínimo de
	// 3 até para dica), e Sec sumia na base da janela, onde o gradiente
	// é mais escuro (2,4:1). Agora Fraco 3,9:1 e Sec 7,1:1 no Cartao.
	Texto: hex(0x1b232b), Sec: hex(0x4e5a65), Fraco: hex(0x738494),
	// O vidro do card deixa passar o tom do fundo, que embaixo é um
	// azul-acinzentado escuro: o Fraco de antes caía a 1,9:1 ali.
	// #61717f fica em 3,7:1 no pior card e 4,8:1 no do topo.
	CardFraco: hex(0x61717f),
	// Fundo1/Fundo3 são as pontas do gradiente da janela (fundoJanela) —
	// eram próximos demais (diff de ~40 por canal) e o degradê saía
	// quase plano num teste real. Vão mais afastados, do jeito
	// acentuado que o tema escuro já tinha.
	Fundo1: hex(0xeef2f9), Fundo2: hex(0xd9e2ee), Fundo3: hex(0x9aabc4),
	Luz1: rgba(0x4c6ef5, 0.10), Luz2: rgba(0x0ca678, 0.07),
	// Vidro2 (card em repouso) desceu de 0,86 para 0,78: com 0,86 o card
	// era praticamente branco opaco e o degradê da janela não passava —
	// o painel inteiro lia como blocos chapados sobre um fundo bonito.
	Vidro1: rgba(0xffffff, 0.70), Vidro2: rgba(0xffffff, 0.78), Vidro3: rgba(0xffffff, 1.00),
	VidroTinta: rgba(0xffffff, 0.52), VidroTintaH: rgba(0xffffff, 0.80),
	// Busca no tom da Moldura (2.9): a caixa do atalho é um pedaço do
	// topo do app flutuando, e não mais um vidro cinza à parte. Os alfas
	// da 2.9.0 (0,90 e 0,70) deixavam a caixa praticamente opaca, e ela
	// tem de continuar lendo como vidro sobre o desktop (2.9.1).
	BuscaVidro: rgba(0xd9e1ec, 0.84), BuscaFosco: rgba(0xd9e1ec, 0.56),
	// Borda do card em repouso: era 11%, quase se perdia contra o fundo —
	// ver o relato de bordas "dissolvendo" no tema claro.
	LuzB:  rgba(0x11161a, 0.17),
	Barra: rgba(0xe8edf3, 0.86),
	// BarraFosca: a mesma barra quando o fundo desfocado vai por baixo
	// (faixaFosca) — o que atravessa é só cor, então afina um pouco.
	BarraFosca: rgba(0xe8edf3, 0.78),
	HeroLuz1:   rgba(0x748ffc, 0.30), HeroLuz2: rgba(0x38d9a9, 0.20),
	Topo1: hex(0xf2f5f9), Topo2: hex(0xe6ecf3),
	// A faixa de janela (minimizar/maximizar/fechar) é MAIS ESCURA que a
	// tira de abas de propósito: são duas faixas coladas, e sem diferença
	// de tom elas viram um bloco só com um degrau de cor no meio sem
	// explicação. Aqui a hierarquia fica: janela (mais escura) > abas >
	// conteúdo.
	Titulo1: hex(0xdbe3ee), Titulo2: hex(0xcbd6e4), TituloBorda: rgba(0x11161a, 0.22),
	Abas1: hex(0xf5f8fc), Abas2: hex(0xeaf0f7),
	TopoTxt: hex(0x11161a), TopoSec: hex(0x4e5a65),
	// Sobre cromo CLARO quem modula é PRETO translúcido — branco sobre
	// barra clara resolve para a própria cor da barra e some (tema.py).
	Vidro: rgba(0x11161a, 0.05), VidroH: rgba(0x11161a, 0.11), VidroB: rgba(0x11161a, 0.12),
	Palco: hex(0x0b0f14),
	// Terminal claro: branco cheio (mesmo valor de Cartao, de propósito —
	// é a "proposta A" aprovada), sem o vidro translúcido do resto da UI,
	// que ficaria ilegível em cima de texto de shell. Paleta ANSI e
	// seleção vêm recalibradas pro fundo claro, não é a paleta escura
	// com o alfa tirado: branco (15) deixa de ser #ffffff (some no
	// fundo), e o amarelo (3/11) — o pior caso de contraste em terminal
	// claro — escurece bem mais que o resto.
	TermBg: hex(0xffffff), TermFg: hex(0x1b232b),
	TermSel: hex(0xd8dffd), // = Azul a ~22% sobre branco, pré-calculado (sem alfa em tempo de desenho)
	Ansi: [16]color.NRGBA{
		hex(0x383f47), hex(0xc2334a), hex(0x128a63), hex(0xa87909),
		hex(0x2f4cd6), hex(0x8b3fc9), hex(0x0f8a8f), hex(0x6b7684),
		hex(0x8a97a3), hex(0xd94f5c), hex(0x1aa877), hex(0xc99a1f),
		hex(0x4c6ef5), hex(0x9a5fdb), hex(0x159aa0), hex(0xcfd6dc),
	},
	// Hover era #eef2f5: 1,1:1 contra o Cartao, e o item do menu sob o
	// mouse mal mudava. #e6ecf4 marca sem virar seleção.
	Hover: hex(0xe6ecf4), Campo: hex(0xffffff),
	OkBg: hex(0xdcf5ec), OkFg: hex(0x0a735d),
	ErroBg: hex(0xfde4e4), ErroFg: hex(0xb02a37), ErroH: hex(0x8d1f2a),
	AtencaoBg: hex(0xfdf1d8), AtencaoFg: hex(0x8a5a00),
	Azul: hex(0x4c6ef5), AzulH: hex(0x3b5bdb), Verde: hex(0x0ca678), Roxo: hex(0x6b40d0),
	AzulCheio: hex(0x4c6ef5), AzulCheioH: hex(0x3b5bdb),
	AzulFraco: rgba(0x4c6ef5, 0.13), VerdeFraco: rgba(0x0ca678, 0.13), RoxoFraco: rgba(0x6b40d0, 0.13),
	Hero1: hex(0x171c22), Hero2: hex(0x2b3a4a), HeroTxt: hex(0xffffff),
	// O hero é escuro NOS DOIS temas (é ele que ancora a página), então o
	// texto secundário dele tem que vir da paleta ESCURA. Usar o topo_sec
	// do tema claro (#5b6976) sobre um hero quase preto é o mesmo erro da
	// "faixa preta acidental" que a skill descreve, só que invertido.
	HeroSec: hex(0x9aa6b2),
	Moldura: hex(0xd9e1ec), MolduraLinha: hex(0xc3cdd9), Pagina: hex(0xedf1f6),
	Contorno: hex(0x9faab8),
	Brilho:   rgba(0x4263eb, 0.15), BrilhoVeu: rgba(0xffffff, 0.55),
	Elevado: hex(0xffffff), Afundado: hex(0xdfe5ee),
	// Mais opaco e com borda mais firme que na primeira versão da 2.9:
	// no claro os cards liam "sem peso", quase da cor da folha.
	CardTinta: rgba(0xffffff, 0.90), CardTintaH: rgba(0xffffff, 1.0), CardBorda: hex(0xc9d2dd),
	Escuro: false,
}

var temaEscuro = Tema{
	Fundo: hex(0x10141a), Cartao: hex(0x171b21), Borda: hex(0x242a32), Borda2: hex(0x333b45),
	// Revisão de contraste (2026-10-18): Sec e Fraco um degrau mais
	// claros. O pior caso é texto sobre um card que está em cima de uma
	// faixa diagonal do fundo — ali o Sec antigo dava 2,8:1.
	Texto: hex(0xd6dde5), Sec: hex(0x9eaab6), Fraco: hex(0x75838e),
	// Fraco foi calibrado contra Fundo (quase preto). O preenchimento
	// translúcido do card (Vidro2/Vidro3) sai mais claro que a conta em
	// sRGB prevê — o Gio compõe em linear, mesmo aviso do Luz1/Luz2 mais
	// abaixo — e Fraco quase desaparecia em cima dele, pior ainda no
	// hover/selecionado (Vidro3, translúcido a mais ainda). CardFraco é
	// um degrau mais claro, só pra texto que fica sobre o card.
	CardFraco: hex(0x9da7b2),
	Fundo1:    hex(0x1a212b), Fundo2: hex(0x10141a), Fundo3: hex(0x080b0f),
	// As luzes vão bem abaixo do alfa do tema.py (0.16/0.10 lá). O Gio
	// compõe em espaço LINEAR, o GTK/cairo em sRGB: a mesma rgba() sobre
	// um fundo quase preto sai bem mais forte aqui. Uma primeira redução
	// (~55% do original) ainda deixava o verde tomar a metade da tela
	// num teste real — cortado bem mais forte agora.
	Luz1: rgba(0x748ffc, 0.045), Luz2: rgba(0x38d9a9, 0.02),
	// Preenchimento do card: DE VOLTA ao valor original. Subir o alfa
	// aqui pareceu seguro pela conta em sRGB, mas o Gio compõe em linear
	// (mesmo aviso do Luz1/Luz2 acima) — o resultado real saiu bem mais
	// claro que a conta previa, um cinza lavado que atropelava até o
	// contraste do texto secundário em cima do card.
	Vidro1: rgba(0xffffff, 0.045), Vidro2: rgba(0xffffff, 0.075), Vidro3: rgba(0xffffff, 0.13),
	VidroTinta: rgba(0xffffff, 0.030), VidroTintaH: rgba(0xffffff, 0.075),
	BuscaVidro: rgba(0x0b0e13, 0.74), BuscaFosco: rgba(0x0b0e13, 0.46),
	// A borda do card em repouso era branco puro translúcido — sobre fundo
	// quase preto isso lê como BRILHO, não como borda de material. Um
	// azul-acinzentado neutro em vez de branco resolve isso sem mudar o
	// matiz frio do resto da paleta escura.
	LuzB:       rgba(0x8fa0b8, 0.16),
	Barra:      rgba(0x10141a, 0.86),
	BarraFosca: rgba(0x10141a, 0.76),
	HeroLuz1:   rgba(0x748ffc, 0.13), HeroLuz2: rgba(0x38d9a9, 0.08),
	Topo1: hex(0x080a0d), Topo2: hex(0x12171d),
	Titulo1: hex(0x0e131b), Titulo2: hex(0x03050a), TituloBorda: rgba(0x000000, 0.75),
	Abas1: hex(0x18202b), Abas2: hex(0x1e2733),
	TopoTxt: hex(0xffffff), TopoSec: hex(0x9aa6b2),
	Vidro: rgba(0xffffff, 0.06), VidroH: rgba(0xffffff, 0.13), VidroB: rgba(0xffffff, 0.10),
	Palco: hex(0x06080a), TermBg: hex(0x0d1117), TermFg: hex(0xd7dee6),
	// = tema.Azul (0x748ffc) a 35% sobre TermBg (0x0d1117), pré-calculado
	// — era isso que a seleção pintava por cima do fundo antes de virar
	// cor sólida.
	TermSel: hex(0x313d67),
	Ansi: [16]color.NRGBA{
		hex(0x1b1f24), hex(0xd2414f), hex(0x2fa87c), hex(0xd79a28),
		hex(0x4c6ef5), hex(0x9a6ee0), hex(0x1ba8a0), hex(0xc6ccd4),
		hex(0x5c6570), hex(0xe05561), hex(0x38d9a9), hex(0xe0b458),
		hex(0x748ffc), hex(0xb197fc), hex(0x3bc9db), hex(0xffffff),
	},
	Hover: hex(0x262d36), Campo: hex(0x171b21),
	OkBg: hex(0x0d2f28), OkFg: hex(0x4fd1b0),
	ErroBg: hex(0x341a1e), ErroFg: hex(0xc9414d), ErroH: hex(0xe05561),
	AtencaoBg: hex(0x332810), AtencaoFg: hex(0xe0b458),
	Azul: hex(0x748ffc), AzulH: hex(0x91a7ff), Verde: hex(0x38d9a9), Roxo: hex(0xb197fc),
	AzulCheio: hex(0x4263eb), AzulCheioH: hex(0x4c6ef5),
	AzulFraco: rgba(0x748ffc, 0.16), VerdeFraco: rgba(0x38d9a9, 0.16), RoxoFraco: rgba(0xb197fc, 0.16),
	Hero1: hex(0x0b0e12), Hero2: hex(0x1c2733), HeroTxt: hex(0xffffff),
	HeroSec: hex(0x9aa6b2),
	Moldura: hex(0x0b0e13), MolduraLinha: hex(0x1d242d), Pagina: hex(0x151a21),
	Contorno: hex(0x343e4b),
	Brilho:   rgba(0x4263eb, 0.13), BrilhoVeu: transparente,
	Elevado: hex(0x2a323d), Afundado: hex(0x11161c),
	CardTinta: rgba(0x1e242d, 0.74), CardTintaH: rgba(0x1c222a, 0.96), CardBorda: hex(0x29313b),
	Escuro: true,
}

// paletaMaterial põe a paleta do app no material.Theme da janela. É dela
// que os widgets do Gio tiram as cores que o app não passa à mão: a
// barra de rolagem das listas (Fg) e a seleção de texto dos campos
// (ContrastBg). Sem isto valiam os padrões do Material — rolagem PRETA
// nos dois temas, invisível no escuro, e seleção índigo (#3f51b5), fora
// da paleta. Chamada no começo de cada quadro, em cada janela (cada uma
// tem o próprio Theme): são quatro atribuições, e a troca de tema vale
// no quadro seguinte sem ninguém precisar avisar.
func paletaMaterial(th *material.Theme) {
	th.Palette = material.Palette{
		Fg: tema.Texto, Bg: tema.Cartao,
		ContrastBg: tema.Azul, ContrastFg: hex(0xffffff),
	}
}

// Foco em campo de texto.
//
// O teclado do Wayland é roteado direto para a aba remota (ver
// internal/grab): sem isto, digitar na busca da lateral com uma sessão
// VNC aberta mandava as letras para a máquina remota e o campo ficava
// vazio. Todo campo de texto do app passa pelos helpers caixaEditor /
// caixaBusca, então marcar ali cobre a interface inteira.
var focoEmCampo atomic.Bool

// marcarFoco é chamado pelos helpers de campo a cada quadro.
func marcarFoco(focado bool) {
	if focado {
		focoEmCampo.Store(true)
	}
}

// temaApp é o material.Theme do app (fonte/shaper), preenchido no main.
var temaApp *material.Theme

// temaBusca é o material.Theme SÓ da caixa de busca aberta pelo app.
//
// Tem de ser OUTRO, e isto não é zelo: a caixa é uma segunda janela e roda
// o laço dela numa goroutine própria (buscapop.go), e o text.Shaper de um
// Theme é um cache sem trava. O próprio Gio diz as duas coisas — em
// text/shaper.go, que duas goroutines no mesmo shaper dão panic, e em
// widget/material/theme.go, que é um Theme por janela de topo. Panic de
// mapa não devolve erro: derruba o processo inteiro, com as sessões
// abertas junto, e só aconteceria com a caixa na tela por cima de uma aba
// recebendo dados — ou seja, no uso normal do atalho global.
//
// No serviço do atalho (servico_linux.go) o problema não existe: lá a
// caixa é a única janela do processo, e o Theme dela já é exclusivo.
var temaBusca *material.Theme

// tema é o tema em uso. Trocar aqui troca o app inteiro.
var tema = temaClaro

// Tipografia — mesmas famílias e tamanhos do tema.py. As famílias IBM Plex
// não estão instaladas nesta máquina, então o original já cai nos mesmos
// substitutos que pedimos aqui (o Gio consulta as fontes do sistema).
// CANTARELL FICA DE FORA DE PROPÓSITO. Ela está instalada (é a fonte que o
// app GTK original acaba usando aqui), mas o shaper do Gio não consegue
// carregá-la: devolve run de largura ZERO, e — pior — um nome assim no
// meio da lista zera a lista INTEIRA em vez de cair pro próximo. O sintoma
// era todo texto em sans sumir da tela, com o layout se fechando em volta
// do nada. Fira Sans é o substituto humanista mais próximo que carrega.
var (
	fonteMono = font.Font{Typeface: "IBM Plex Mono, DejaVu Sans Mono, monospace"}
	// fonteMonoNegrito é o negrito do terminal SSH (SGR 1): a Plex Mono
	// SemiBold já vem embutida (ver fontes.go), então o avanço é o mesmo
	// da regular e a grade não se mexe.
	fonteMonoNegrito = font.Font{Typeface: "IBM Plex Mono, DejaVu Sans Mono, monospace", Weight: font.SemiBold}
	// Desde a repaginação da 2.9 o texto da interface é Space Grotesk e
	// os títulos são Sora, as fontes da central do ChamadoJá (ver
	// fontes.go). A Plex Sans segue na lista como reserva de glifo.
	fonteSans = font.Font{Typeface: "Space Grotesk, IBM Plex Sans, Fira Sans, DejaVu Sans, sans-serif"}
	// fonteSansMedia é o peso 500: rótulo de aba, botão e item de lista.
	fonteSansMedia = font.Font{Typeface: "Space Grotesk, IBM Plex Sans, Fira Sans, DejaVu Sans, sans-serif", Weight: font.Medium}
	// fonteTitulo: nome do app, título de página, nome de card e de grupo,
	// números. Quase sempre em negrito() (SemiBold).
	fonteTitulo = font.Font{Typeface: "Sora, IBM Plex Sans Condensed, Fira Sans, DejaVu Sans, sans-serif"}
)

// Tamanhos, um por classe do CSS original — assim o que muda aqui muda no
// lugar certo, em vez de números soltos no meio do layout.
const (
	spGrupoTitulo = unit.Sp(18) // .grupo-titulo
	spSubgrupo    = unit.Sp(15) // .subgrupo-titulo
	spCardNome    = unit.Sp(15) // .card-nome (17 até a 2.8; o card ficou mais baixo)
	spCardHost    = unit.Sp(12) // .card-host
	spCardMeta    = unit.Sp(9)  // .card-meta
	spRodape      = unit.Sp(11) // .rodape-info
	spSecundario  = unit.Sp(11) // .secundario
	spCorpo       = unit.Sp(12) // .mono / corpo
	// spRotulo: rótulo de formulário, aviso e texto de explicação em
	// diálogo, em Space Grotesk (desde a 2.9; antes era a mono de 11).
	spRotulo = unit.Sp(13)
	// spNota: a nota curta de rodapé de diálogo ("Enter confirma", "o app
	// será reaberto"), também em Space Grotesk.
	spNota = unit.Sp(12)
)

func hex(v uint32) color.NRGBA {
	return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xff}
}

func rgba(v uint32, alpha float64) color.NRGBA {
	c := hex(v)
	c.A = uint8(math.Round(alpha * 255))
	return c
}

// ------------------------------------------------------------- texto

func txt(th *material.Theme, f font.Font, tam unit.Sp, s string, cor color.NRGBA) material.LabelStyle {
	l := material.Label(th, tam, s)
	l.Font = f
	l.Color = cor
	l.MaxLines = 1
	return l
}

// paragrafo é texto que PODE quebrar em várias linhas: explicação de
// diálogo, não rótulo de lista. O txt() padrão corta em UMA linha com
// reticências, que é o certo numa tabela e errado numa frase — a frase
// simplesmente sumia pela direita.
func paragrafo(th *material.Theme, f font.Font, tam unit.Sp, s string, cor color.NRGBA) layout.Widget {
	l := material.Label(th, tam, s)
	l.Font = f
	l.Color = cor
	return l.Layout
}

// rotuloLinha é rótulo de UMA linha só, cortado com reticências. Comando é
// linear: quebrar um comando longo em duas linhas muda o que a pessoa lê e
// desalinha a lista inteira.
func rotuloLinha(th *material.Theme, f font.Font, tam unit.Sp, s string, cor color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		l := txt(th, f, tam, s, cor)
		l.MaxLines = 1
		return l.Layout(gtx)
	}
}

func rotulo(th *material.Theme, f font.Font, tam unit.Sp, s string, cor color.NRGBA) layout.Widget {
	return txt(th, f, tam, s, cor).Layout
}

func negrito(l material.LabelStyle) material.LabelStyle {
	l.Font.Weight = font.SemiBold
	return l
}

// ---------------------------------------------------- gradientes (CSS)

// gradCSS pinta um gradiente linear com a MESMA convenção de ângulo do
// CSS (0° = para cima, 90° = para a direita), já que os ângulos vieram
// copiados do tema.py. O Gio só faz gradiente de duas paradas; onde o CSS
// tinha três, a do meio fica bem perto da interpolação linear das pontas
// (diferença de ~3/255 por canal no fundo), então não vale o custo de
// emular com clip rotacionado.
func gradCSS(gtx layout.Context, size image.Point, anguloGraus float64,
	c1 color.NRGBA, pos1 float64, c2 color.NRGBA, pos2 float64) {

	if size.X <= 0 || size.Y <= 0 {
		return
	}
	rad := anguloGraus * math.Pi / 180
	dx, dy := math.Sin(rad), -math.Cos(rad)
	w, h := float64(size.X), float64(size.Y)
	comp := math.Abs(w*dx) + math.Abs(h*dy) // comprimento da linha do gradiente
	cx, cy := w/2, h/2

	ponto := func(p float64) f32.Point {
		d := (p - 0.5) * comp
		return f32.Pt(float32(cx+dx*d), float32(cy+dy*d))
	}

	paint.LinearGradientOp{
		Stop1: ponto(pos1), Color1: c1,
		Stop2: ponto(pos2), Color2: c2,
	}.Add(gtx.Ops)
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	paint.PaintOp{}.Add(gtx.Ops)
}

var transparente = color.NRGBA{}

// fundoJanela pinta a MOLDURA na janela inteira: topo, lateral e barra de
// status ficam direto sobre ela, e a folha (fundoFolha) é pintada por cima,
// só na área do conteúdo. É também quem refaz, quando preciso, a imagem da
// folha (vidro.go), que serve de fundo à própria folha e ao vidro dos cards.
//
// Até a 2.8 o fundo era um gradiente na janela inteira com faixas
// diagonais e dois blocos de texto no canto ("CONECTE / GERENCIE…"). Na
// repaginação da 2.9 saíram: brigavam com os cards e davam à tela a cara
// de "quebrada". O desenho agora é o da central do ChamadoJá — moldura de
// um tom, folha de outro, luz no canto e a malha de hexágonos discreta no
// cabeçalho do Painel (malhaHex).
func fundoJanela(gtx layout.Context, size image.Point) {
	paint.FillShape(gtx.Ops, tema.Moldura, clip.Rect{Max: size}.Op())
	atualizarFosco(gtx, size)
}

// fundoFolha pinta a folha no recorte corrente: a imagem da folha (Pagina
// com a luz no canto) ANCORADA NA JANELA — o mesmo pedaço que o vidro dos
// cards mostra atrás deles e que a aba ativa usa para se fundir à folha,
// então card, aba e folha nunca brigam de tom.
func fundoFolha(gtx layout.Context, size image.Point) {
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	pintarFolha(gtx)
}

// pintarFolha pinta a folha no recorte corrente, sem recorte próprio —
// para quem já empurrou o molde (a aba ativa, o item ativo da lateral).
func pintarFolha(gtx layout.Context) {
	if fosco.ok {
		fosco.img.Add(gtx.Ops)
	} else {
		paint.ColorOp{Color: tema.Pagina}.Add(gtx.Ops)
	}
	paint.PaintOp{}.Add(gtx.Ops)
}

// comAlfa devolve c com a alfa recalculada a partir de uma fração
// 0..1. a é sempre travado nesse intervalo: sem isso, uma fração maior
// que 1 (um multiplicador aplicado sem pensar, por exemplo) estoura o
// uint8 em silêncio — dá a volta em vez de saturar — e produz uma cor
// errada sem erro nenhum pra rastrear.
func comAlfa(c color.NRGBA, a float64) color.NRGBA {
	if a < 0 {
		a = 0
	} else if a > 1 {
		a = 1
	}
	c.A = uint8(math.Round(a * 255))
	return c
}

// ------------------------------------------------------------ formas

// superficie pinta um retângulo arredondado com borda de 1px. A borda é
// desenhada por cima, como traço: a caixa tem o MESMO tamanho com e sem
// borda, então estado nunca muda dimensão (regra dura da skill).
func superficie(gtx layout.Context, size image.Point, fundo, borda color.NRGBA, raioDp unit.Dp) {
	if size.X <= 0 || size.Y <= 0 {
		return
	}
	rr := clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(raioDp))
	paint.FillShape(gtx.Ops, fundo, rr.Op(gtx.Ops))
	if borda.A > 0 {
		paint.FillShape(gtx.Ops, borda, clip.Stroke{Path: rr.Path(gtx.Ops), Width: 1}.Op())
	}
}

// superficieRelevo é a superficie com um pouco de luz: um brilho que
// desce do topo e some antes do meio, e um fio de 1px de luz na borda
// de cima. É o que tira o "chapado" de cartão, diálogo, menu e botão —
// a cor base continua a mesma (o alfa do brilho é baixo), só deixa de
// ser um bloco de cor único. Campo de texto, chip e item de lista não
// usam: neles o brilho vira ruído.
func superficieRelevo(gtx layout.Context, size image.Point, fundo, borda color.NRGBA, raioDp unit.Dp) {
	if size.X <= 0 || size.Y <= 0 {
		return
	}
	superficie(gtx, size, fundo, borda, raioDp)
	brilhoRelevo(gtx, size, raioDp)
}

// brilhoRelevo é a luz de superficieRelevo sem o fundo: o degradê que
// desce do topo e o fio de 1px na borda de cima. Separado para o vidro
// fosco (vidro.go) pôr a mesma luz por cima da tinta dele.
func brilhoRelevo(gtx layout.Context, size image.Point, raioDp unit.Dp) {
	rr := clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(raioDp))
	molde := rr.Push(gtx.Ops)
	luz := rgba(0xffffff, 0.09)
	if tema.Escuro {
		// Bem mais baixo que no claro: o Gio compõe em linear e branco
		// translúcido sobre fundo escuro sai muito mais claro que a
		// conta em sRGB (mesmo aviso de Vidro2, acima) — a 0,05 o card
		// virava um bloco cinza lavado.
		luz = rgba(0xffffff, 0.018)
	}
	alt := size.Y / 2
	if alt < gtx.Dp(12) {
		alt = size.Y
	}
	paint.LinearGradientOp{
		Stop1: f32.Pt(0, 0), Color1: luz,
		Stop2: f32.Pt(0, float32(alt)), Color2: transparente,
	}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	fio := rgba(0xffffff, 0.45)
	if tema.Escuro {
		fio = rgba(0xffffff, 0.05)
	}
	paint.FillShape(gtx.Ops, fio, clip.Rect{
		Min: image.Pt(gtx.Dp(raioDp), 0), Max: image.Pt(size.X-gtx.Dp(raioDp), 1),
	}.Op())
	molde.Pop()
}

// icone desenha um ícone vetorial Material. Ícone de verdade, não glifo de
// fonte: a fonte não tem emoji e desenha "tofu" (□).
// iconeMu serializa a RASTERIZAÇÃO dos ícones entre as janelas.
//
// Os ícones do gio.tools/icons são *widget.Icon de PACOTE — um objeto
// global por ícone, compartilhado pelo app inteiro. Dentro dele há um
// cache sem trava (src, op, imgSize, imgColor, em widget/icon.go): quando
// o tamanho ou a cor pedidos diferem do que está guardado, o Layout
// rasteriza e ESCREVE nesses campos.
//
// Com uma janela só isso nunca importou. Com a sessão em janela própria
// passam a existir duas goroutines de quadro desenhando os MESMOS objetos
// — o ícone de reconectar da barrinha, o emblema do splash, o de tela
// cheia — e duas escritas concorrentes num paint.ImageOp (struct de várias
// palavras, com ponteiro) rasgam o valor: textura errada, e corrida de
// dados de verdade sob -race.
//
// É a mesma armadilha do text.Shaper, resolvida de outro jeito: o Theme dá
// para ter um por janela (ver tha, em rdptab.go), o ícone global não —
// então serializa-se aqui. Custa quase nada: depois da primeira vez o
// cache acerta e o Layout só lê.
//
// ESTE É O ÚNICO ic.Layout do app, de propósito. Desenhar ícone por fora
// daqui recria o problema sem passar pela trava.
var iconeMu sync.Mutex

func icone(gtx layout.Context, ic *widget.Icon, cor color.NRGBA, tam unit.Dp) layout.Dimensions {
	gtx.Constraints.Min = image.Point{}
	gtx.Constraints.Max = image.Pt(gtx.Dp(tam), gtx.Dp(tam))
	iconeMu.Lock()
	defer iconeMu.Unlock()
	return ic.Layout(gtx, cor)
}

// alternarTema troca claro <-> escuro. O tema é um dicionário só: trocar
// a variável troca o app inteiro.
//
// A GRAVAÇÃO mora aqui, colada na troca, e não no botão que chama isto:
// ela já ficou de fora uma vez — o botão trocava o tema, a tela toda
// mudava, e o app reabria no claro porque ninguém tinha escrito
// [geral] tema no .ini (relatado no Windows; valia nos dois sistemas).
// Enquanto as duas linhas forem a mesma função, não dá para esquecer
// uma. Ver tema_lembrar_test.go.
func alternarTema() {
	if tema.Fundo == temaClaro.Fundo {
		tema = temaEscuro
	} else {
		tema = temaClaro
	}
	lembrarGeral("tema", nomeDoTema())
}

// setaExpansor é o chevron de tudo que abre e fecha: grupo do Painel,
// grupo da lateral, bloco do editor de conexão. Mesmo desenho nos três,
// porque é o mesmo gesto.
//
// É ÍCONE, e não o texto "▸"/"▾" que o editor usava: as fontes embutidas
// (IBM Plex, ver fontes.go) não têm esses dois glifos. No Linux o shaper
// cai numa fonte do sistema e ninguém percebe; no Windows não há em quem
// cair e o que aparece é o quadradinho vazio — foi assim que os "ícones
// que não renderizam" do editor foram parar no relato de teste. Ícone do
// gio.tools/icons é vetor desenhado pelo próprio Gio: não depende de
// fonte nenhuma, nem do sistema. Ver glifos_test.go.
func setaExpansor(aberto bool) *widget.Icon {
	if aberto {
		return icons.NavigationExpandMore
	}
	return icons.NavigationChevronRight
}

// ------------------------------------------------------ régua de alinhamento

// janelaRaio é o arredondamento dos cantos da janela quando ela NÃO está
// maximizada e a decoração é nossa (CSD).
const janelaRaio = unit.Dp(10)

// reguaLigada mostra a régua virtual (-regua na linha de comando, Ctrl+G
// em tempo de execução). É uma FERRAMENTA DE AJUSTE, não parte da
// interface: linhas a cada 8dp, mais fortes a cada 32dp, e uma guia forte
// na borda da lateral — que é a coluna com a qual o resto tem que alinhar.
var reguaLigada bool

// depurarLayout imprime as restrições de cada aba (ACESSOS_DBG_LAYOUT=1).
var depurarLayout = os.Getenv("ACESSOS_DBG_LAYOUT") == "1"

const reguaPasso = unit.Dp(8)

func regua(gtx layout.Context, colunaLateral int) {
	if !reguaLigada {
		return
	}
	size := gtx.Constraints.Max
	passo := gtx.Dp(reguaPasso)
	if passo <= 0 {
		return
	}
	fina := color.NRGBA{R: 0xff, G: 0x00, B: 0x88, A: 0x22}
	grossa := color.NRGBA{R: 0xff, G: 0x00, B: 0x88, A: 0x55}
	guia := color.NRGBA{R: 0x00, G: 0xc8, B: 0xff, A: 0xaa}

	linhaV := func(x int, c color.NRGBA) {
		paint.FillShape(gtx.Ops, c, clip.Rect{Min: image.Pt(x, 0), Max: image.Pt(x+1, size.Y)}.Op())
	}
	linhaH := func(y int, c color.NRGBA) {
		paint.FillShape(gtx.Ops, c, clip.Rect{Min: image.Pt(0, y), Max: image.Pt(size.X, y+1)}.Op())
	}
	for i, x := 0, 0; x < size.X; i, x = i+1, x+passo {
		c := fina
		if i%4 == 0 {
			c = grossa
		}
		linhaV(x, c)
	}
	for i, y := 0, 0; y < size.Y; i, y = i+1, y+passo {
		c := fina
		if i%4 == 0 {
			c = grossa
		}
		linhaH(y, c)
	}
	// as guias que importam: a coluna da lateral e o meio da janela.
	if colunaLateral > 0 {
		linhaV(colunaLateral, guia)
	}
	linhaV(size.X/2, guia)
	linhaH(size.Y/2, guia)
}

// clarear devolve a mesma cor um pouco mais clara — só para o hover dos
// botões cheios, onde a paleta não tem um token próprio. Fora daí, hover
// usa cor FIXA da paleta, nunca calculada (regra dura da skill).
func clarear(c color.NRGBA) color.NRGBA {
	sobe := func(v uint8) uint8 {
		n := int(v) + 22
		if n > 255 {
			n = 255
		}
		return uint8(n)
	}
	return color.NRGBA{R: sobe(c.R), G: sobe(c.G), B: sobe(c.B), A: c.A}
}
