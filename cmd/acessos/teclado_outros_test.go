//go:build !linux

package main

import (
	"fmt"
	"reflect"
	"testing"

	"gioui.org/io/key"
)

// Testes do teclado do Windows: a tradução (teclado_outros.go) e o
// roteador por janela (estadoTeclado, em entrada_outros.go). Nenhum abre
// janela: o roteador recebe key.Event montado à mão — com o Scancode que o
// patch do Gio passa a preencher — e a entrega é trocada por um gravador.

// abaTextoTeste é uma aba que quer o caractere (como SSH e VNC). A
// abaDeTeste pura faz o papel do RDP, que quer a posição.
type abaTextoTeste struct{ abaDeTeste }

func (a *abaTextoTeste) HandleTexto(string) {}

// gravador anota o que o roteador entregou, na ordem.
type gravador struct{ log []string }

func (g *gravador) roteador() *estadoTeclado {
	return &estadoTeclado{
		tecla: func(t Tab, ks, kc uint32, p bool) {
			v := "solta"
			if p {
				v = "aperta"
			}
			g.log = append(g.log, fmt.Sprintf("%s %s ks=%#x kc=%d", t.Title(), v, ks, kc))
		},
		texto: func(t Tab, s string) {
			g.log = append(g.log, fmt.Sprintf("%s texto %q", t.Title(), s))
		},
	}
}

func (g *gravador) confere(t *testing.T, quer ...string) {
	t.Helper()
	if quer == nil {
		quer = []string{}
	}
	got := g.log
	if got == nil {
		got = []string{}
	}
	if !reflect.DeepEqual(got, quer) {
		t.Fatalf("entregue:\n  %q\nqueria:\n  %q", got, quer)
	}
}

// Scancodes de um teclado de verdade (conjunto 1), os que os testes usam.
const (
	scShiftEsq = 0x2a
	scCtrlEsq  = 0x1d
	scAltGr    = 0xe038
	sc1        = 0x02
	scA        = 0x1e
	scC        = 0x2e
	scQ        = 0x10
	scEspaco   = 0x39
	scSetaDir  = 0xe04d
)

func ev(nome key.Name, sc uint32, mods key.Modifiers, aperta bool) key.Event {
	st := key.Release
	if aperta {
		st = key.Press
	}
	return key.Event{Name: nome, Scancode: sc, Modifiers: mods, State: st}
}

// O bug do Shift: no SSH, Shift+1 tem de chegar como "!", e chega pelo
// texto do Windows — a tecla "1" em si não pode ir (sairia "1"). O Shift
// vai como tecla, apertando e soltando em volta do texto.
func TestShiftNoTerminalVaiPeloTexto(t *testing.T) {
	g := &gravador{}
	k := g.roteador()
	ssh := &abaTextoTeste{abaDeTeste{"ssh"}}

	k.teclaDe(ssh, ev(key.NameShift, scShiftEsq, key.ModShift, true))
	k.teclaDe(ssh, ev("1", sc1, key.ModShift, true))
	k.textoDe(ssh, "!")
	k.teclaDe(ssh, ev("1", sc1, key.ModShift, false))
	k.teclaDe(ssh, ev(key.NameShift, scShiftEsq, 0, false))

	g.confere(t,
		"ssh aperta ks=0xffe1 kc=50",
		`ssh texto "!"`,
		"ssh solta ks=0xffe1 kc=50",
	)
}

// No RDP é o contrário: vai a POSIÇÃO da tecla e o layout remoto decide o
// caractere. O texto do Windows não pode ir junto.
func TestRDPVaiPelaPosicaoENaoPeloTexto(t *testing.T) {
	g := &gravador{}
	k := g.roteador()
	rdp := &abaDeTeste{"rdp"}

	k.teclaDe(rdp, ev(key.NameShift, scShiftEsq, key.ModShift, true))
	k.teclaDe(rdp, ev("1", sc1, key.ModShift, true))
	k.textoDe(rdp, "!")
	k.teclaDe(rdp, ev("1", sc1, key.ModShift, false))
	k.teclaDe(rdp, ev(key.NameShift, scShiftEsq, 0, false))

	g.confere(t,
		"rdp aperta ks=0xffe1 kc=50",
		"rdp aperta ks=0x31 kc=10",
		"rdp solta ks=0x31 kc=10",
		"rdp solta ks=0xffe1 kc=50",
	)
}

// Ctrl+C no terminal é atalho (SIGINT): vai como tecla. Ctrl+Espaço gera
// também um WM_CHAR de espaço, que não pode virar um espaço a mais.
func TestCtrlNoTerminalVaiComoTecla(t *testing.T) {
	g := &gravador{}
	k := g.roteador()
	ssh := &abaTextoTeste{abaDeTeste{"ssh"}}

	k.teclaDe(ssh, ev(key.NameCtrl, scCtrlEsq, key.ModCtrl, true))
	k.teclaDe(ssh, ev("C", scC, key.ModCtrl, true))
	k.teclaDe(ssh, ev("C", scC, key.ModCtrl, false))
	k.teclaDe(ssh, ev(key.NameSpace, scEspaco, key.ModCtrl, true))
	k.textoDe(ssh, " ")
	k.teclaDe(ssh, ev(key.NameSpace, scEspaco, key.ModCtrl, false))
	k.teclaDe(ssh, ev(key.NameCtrl, scCtrlEsq, 0, false))

	g.confere(t,
		"ssh aperta ks=0xffe3 kc=37",
		"ssh aperta ks=0x63 kc=54",
		"ssh solta ks=0x63 kc=54",
		"ssh aperta ks=0x20 kc=65",
		"ssh solta ks=0x20 kc=65",
		"ssh solta ks=0xffe3 kc=37",
	)
}

// AltGr é Ctrl+Alt para o Windows. O caractere dele ("/" no AltGr+Q do
// ABNT2) é texto, e Ctrl e Alt saem do lado remoto durante o caractere —
// senão o VNC o recebe como Ctrl+Alt+/.
func TestAltGrSoltaCtrlAltEmVoltaDoTexto(t *testing.T) {
	g := &gravador{}
	k := g.roteador()
	vnc := &abaTextoTeste{abaDeTeste{"vnc"}}
	ctrlAlt := key.ModCtrl | key.ModAlt

	k.teclaDe(vnc, ev(key.NameCtrl, scCtrlEsq, key.ModCtrl, true))
	k.teclaDe(vnc, ev(key.NameAlt, scAltGr, ctrlAlt, true))
	k.teclaDe(vnc, ev("Q", scQ, ctrlAlt, true))
	k.textoDe(vnc, "/")

	g.confere(t,
		"vnc aperta ks=0xffe3 kc=37",
		"vnc aperta ks=0xffea kc=108",
		"vnc solta ks=0xffe3 kc=37",
		"vnc solta ks=0xffea kc=108",
		`vnc texto "/"`,
		"vnc aperta ks=0xffea kc=108",
		"vnc aperta ks=0xffe3 kc=37",
	)
}

// Perder o foco (Alt+Tab, atalho global) solta do lado remoto o que estava
// segurado — tecla comum antes, modificador por último.
func TestPerderFocoSoltaTudoModificadorPorUltimo(t *testing.T) {
	g := &gravador{}
	k := g.roteador()
	rdp := &abaDeTeste{"rdp"}

	k.teclaDe(rdp, ev(key.NameCtrl, scCtrlEsq, key.ModCtrl, true))
	k.teclaDe(rdp, ev(key.NameShift, scShiftEsq, key.ModCtrl|key.ModShift, true))
	k.teclaDe(rdp, ev("A", scA, key.ModCtrl|key.ModShift, true))
	g.log = nil

	k.soltarTudo()
	g.confere(t,
		"rdp solta ks=0x41 kc=38",
		"rdp solta ks=0xffe3 kc=37",
		"rdp solta ks=0xffe1 kc=50",
	)
	// e não solta de novo
	g.log = nil
	k.soltarTudo()
	g.confere(t)
}

// O soltar leva os MESMOS códigos do apertar: no VNC o keysym da letra
// muda com o Shift, e soltar "A" depois de apertar "a" deixa o "a" preso
// no servidor.
func TestSoltarUsaOsCodigosDoApertar(t *testing.T) {
	g := &gravador{}
	k := g.roteador()
	vnc := &abaTextoTeste{abaDeTeste{"vnc"}}

	k.teclaDe(vnc, ev(key.NameCtrl, scCtrlEsq, key.ModCtrl, true))
	k.teclaDe(vnc, ev("A", scA, key.ModCtrl, true))
	k.teclaDe(vnc, ev(key.NameShift, scShiftEsq, key.ModCtrl|key.ModShift, true))
	k.teclaDe(vnc, ev("A", scA, key.ModCtrl|key.ModShift, false))

	g.confere(t,
		"vnc aperta ks=0xffe3 kc=37",
		"vnc aperta ks=0x61 kc=38",
		"vnc aperta ks=0xffe1 kc=50",
		"vnc solta ks=0x61 kc=38",
	)
}

// Trocar de aba com a tecla segurada: o soltar vai para a aba que recebeu
// o apertar, não para a nova.
func TestTrocarDeAbaSoltaNaAnterior(t *testing.T) {
	g := &gravador{}
	k := g.roteador()
	um, dois := &abaDeTeste{"um"}, &abaDeTeste{"dois"}

	k.teclaDe(um, ev(key.NameShift, scShiftEsq, key.ModShift, true))
	k.soltarForaDe(um)
	k.soltarForaDe(dois)
	k.teclaDe(dois, ev(key.NameShift, scShiftEsq, 0, false))

	g.confere(t,
		"um aperta ks=0xffe1 kc=50",
		"um solta ks=0xffe1 kc=50",
		"dois solta ks=0xffe1 kc=50",
	)
}

// Repetição automática: o Windows repete o WM_KEYDOWN. Cada repetição vai,
// com os códigos do primeiro, e um soltar só fecha tudo.
func TestRepeticaoAutomatica(t *testing.T) {
	g := &gravador{}
	k := g.roteador()
	ssh := &abaTextoTeste{abaDeTeste{"ssh"}}

	for range 3 {
		k.teclaDe(ssh, ev(key.NameRightArrow, scSetaDir, 0, true))
	}
	k.teclaDe(ssh, ev(key.NameRightArrow, scSetaDir, 0, false))

	g.confere(t,
		"ssh aperta ks=0xff53 kc=114",
		"ssh aperta ks=0xff53 kc=114",
		"ssh aperta ks=0xff53 kc=114",
		"ssh solta ks=0xff53 kc=114",
	)
	if len(k.apertadas) != 0 {
		t.Fatalf("sobrou tecla apertada: %v", k.apertadas)
	}
}

// Teclas que o Name não distingue ou nem tem, e que o RDP precisa na
// posição certa — o motivo do Scancode no key.Event.
func TestTraduzirTeclaPeloScancode(t *testing.T) {
	casos := []struct {
		desc   string
		ke     key.Event
		ks, kc uint32
	}{
		{"\"+\" do numérico (não é o \"=\")", key.Event{Name: "+", Scancode: 0x4e}, 0x2b, 86},
		{"\"=\" que o Gio chama de \"+\"", key.Event{Name: "+", Scancode: 0x0d}, 0x2b, 21},
		{"\"*\" do numérico", key.Event{Name: "*", Scancode: 0x37}, 0x2a, 63},
		{"7 do numérico (não é o 7 de cima)", key.Event{Name: "7", Scancode: 0x47}, '7', 79},
		{"\"/\" do numérico", key.Event{Name: "/", Scancode: 0xe035}, 0x2f, 106},
		{"Enter do numérico", key.Event{Name: key.NameReturn, Scancode: 0xe01c}, 0xff0d, 104},
		{"tecla ao lado do Z (VK_OEM_102)", key.Event{Name: "\\", Scancode: 0x56}, 0x5c, 94},
		{"\"/?\" do ABNT2, sem nome no Gio", key.Event{Scancode: 0x73}, 0, 97},
		{"CapsLock", key.Event{Scancode: 0x3a}, 0xffe5, 66},
		{"Insert", key.Event{Scancode: 0xe052}, 0xff63, 118},
		{"NumLock", key.Event{Scancode: 0xe045}, 0xff7f, 77},
		{"Pause", key.Event{Scancode: 0x45}, 0xff13, 127},
		{"Ctrl direito", key.Event{Name: key.NameCtrl, Scancode: 0xe01d}, 0xffe4, 105},
		{"AltGr", key.Event{Name: key.NameAlt, Scancode: 0xe038}, 0xffea, 108},
		{"Shift direito", key.Event{Name: key.NameShift, Scancode: 0x36}, 0xffe2, 62},
		{"Delete", key.Event{Name: key.NameDeleteForward, Scancode: 0xe053}, 0xffff, 119},
		{"Shift falso do numérico", key.Event{Name: key.NameShift, Scancode: 0xe02a}, 0xffe1, 0},
		{"letra com Shift", key.Event{Name: "A", Scancode: scA, Modifiers: key.ModShift}, 'A', 38},
		{"letra sem Shift", key.Event{Name: "A", Scancode: scA}, 'a', 38},
		{"sem scancode, pelo nome", key.Event{Name: "Q"}, 'q', 24},
		{"sem scancode, seta", key.Event{Name: key.NameUpArrow}, 0xff52, 111},
	}
	for _, c := range casos {
		ks, kc := traduzirTecla(c.ke)
		if ks != c.ks || kc != c.kc {
			t.Errorf("%s: ks=%#x kc=%d, queria ks=%#x kc=%d", c.desc, ks, kc, c.ks, c.kc)
		}
	}
}

// As duas tabelas de keycode — pelo scancode (o caminho normal) e pelo
// nome (a reserva) — têm de concordar em toda tecla que as duas conhecem.
// O scancode aqui é o da tecla com aquele nome num teclado US.
func TestScancodeConcordaComTabelaPorNome(t *testing.T) {
	scancodeUS := map[key.Name]uint32{
		key.NameLeftArrow: 0xe04b, key.NameUpArrow: 0xe048,
		key.NameRightArrow: 0xe04d, key.NameDownArrow: 0xe050,
		key.NameHome: 0xe047, key.NameEnd: 0xe04f,
		key.NamePageUp: 0xe049, key.NamePageDown: 0xe051,
		key.NameReturn: 0x1c, key.NameEscape: 0x01,
		key.NameDeleteBackward: 0x0e, key.NameDeleteForward: 0xe053,
		key.NameTab: 0x0f, key.NameSpace: 0x39,
		key.NameCtrl: 0x1d, key.NameShift: 0x2a, key.NameAlt: 0x38, key.NameSuper: 0xe05b,
		key.NameF1: 0x3b, key.NameF2: 0x3c, key.NameF3: 0x3d, key.NameF4: 0x3e,
		key.NameF5: 0x3f, key.NameF6: 0x40, key.NameF7: 0x41, key.NameF8: 0x42,
		key.NameF9: 0x43, key.NameF10: 0x44, key.NameF11: 0x57, key.NameF12: 0x58,
		";": 0x27, "+": 0x0d, ",": 0x33, "-": 0x0c, ".": 0x34, "/": 0x35,
		"`": 0x29, "[": 0x1a, "\\": 0x2b, "]": 0x1b, "'": 0x28,
	}
	for nome, kcNome := range keycodeX11DoNome {
		sc, ok := scancodeUS[nome]
		if !ok {
			t.Errorf("%q: falta o scancode US no teste", nome)
			continue
		}
		if kc := keycodeX11DoScancode(sc); kc != kcNome {
			t.Errorf("%q: pelo scancode %#x dá %d, pelo nome dá %d", nome, sc, kc, kcNome)
		}
	}
	// letras e dígitos: scancodes reais das fileiras do teclado US
	alfa := map[byte]uint32{'0': 0x0b}
	for i, c := range "123456789" {
		alfa[byte(c)] = 0x02 + uint32(i)
	}
	for _, fileira := range []struct {
		teclas string
		inicio uint32
	}{{"QWERTYUIOP", 0x10}, {"ASDFGHJKL", 0x1e}, {"ZXCVBNM", 0x2c}} {
		for i, c := range fileira.teclas {
			alfa[byte(c)] = fileira.inicio + uint32(i)
		}
	}
	for c, kcNome := range keycodeX11DoAlfanumerico {
		if kc := keycodeX11DoScancode(alfa[c]); kc != kcNome {
			t.Errorf("%q: pelo scancode %#x dá %d, pelo nome dá %d", c, alfa[c], kc, kcNome)
		}
	}
	// os atalhos fixos do menu Teclas (teclas.go) também
	for ks, kc := range keycodeDoKeysym {
		if kc < 9 || kc > 135 {
			t.Errorf("keysym %#x: keycode %d fora da faixa evdev+8", ks, kc)
		}
	}
}

// Tecla de texto, para SSH e VNC, é a que vira caractere com aqueles
// modificadores — nem mais, nem menos.
func TestProduzTexto(t *testing.T) {
	casos := []struct {
		ke   key.Event
		quer bool
	}{
		{key.Event{Name: "A"}, true},
		{key.Event{Name: "A", Modifiers: key.ModShift}, true},
		{key.Event{Name: "1", Modifiers: key.ModShift}, true},
		{key.Event{Name: key.NameSpace}, true},
		{key.Event{Name: "Q", Modifiers: key.ModCtrl | key.ModAlt}, true}, // AltGr
		{key.Event{Scancode: 0x73}, true},                                 // "/?" do ABNT2
		{key.Event{Name: "C", Modifiers: key.ModCtrl}, false},
		{key.Event{Name: ".", Modifiers: key.ModAlt}, false},
		// Os nomes do Gio para estas são UMA rune ("⏎", "←"...), e não são
		// texto — tratadas como texto, sumiam do SSH.
		{key.Event{Name: key.NameReturn}, false},
		{key.Event{Name: key.NameEscape}, false},
		{key.Event{Name: key.NameDeleteBackward}, false},
		{key.Event{Name: key.NameDeleteForward}, false},
		{key.Event{Name: key.NameLeftArrow}, false},
		{key.Event{Name: key.NameUpArrow, Modifiers: key.ModShift}, false},
		{key.Event{Name: key.NameHome}, false},
		{key.Event{Name: key.NamePageDown}, false},
		{key.Event{Name: key.NameTab}, false},
		{key.Event{Name: key.NameShift, Modifiers: key.ModShift}, false},
		{key.Event{Scancode: 0x3a}, false}, // CapsLock
	}
	for _, c := range casos {
		if got := produzTexto(c.ke); got != c.quer {
			t.Errorf("produzTexto(%+v) = %v, queria %v", c.ke, got, c.quer)
		}
	}
}

// As teclas de trava fazem o quadro ler o estado de novo (travas.go).
// Scancodes como o Windows entrega (ver tecladotest).
func TestTeclaDeTrava(t *testing.T) {
	casos := []struct {
		sc   uint32
		quer bool
	}{
		{0x3a, true},    // Caps Lock
		{0xe045, true},  // Num Lock (vem estendido)
		{0x46, true},    // Scroll Lock
		{0x45, true},    // Pause: falso positivo inofensivo, só relê
		{0x4f, false},   // "1"/End do numérico
		{0xe01d, false}, // Ctrl direito
		{0, false},
	}
	for _, c := range casos {
		if got := teclaDeTrava(key.Event{Scancode: c.sc}); got != c.quer {
			t.Errorf("teclaDeTrava(%#x) = %v, queria %v", c.sc, got, c.quer)
		}
	}
}
