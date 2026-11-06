//go:build !linux

package main

import (
	"gioui.org/io/key"
)

// Tradução do teclado do Windows para o que as sessões remotas esperam.
//
// São DOIS caminhos, e a divisão é o que faz Shift, ABNT2 e acento
// funcionarem:
//
//   - RDP repassa a tecla por POSIÇÃO (keycode X11 → scancode RDP, ver
//     rs_tecla em internal/rdp/rdpshim.c). Quem decide o caractere é o
//     layout da máquina remota, então Shift, acento e AltGr saem certos
//     sozinhos — desde que a posição seja a de verdade. Por isso o keycode
//     sai do Scancode do key.Event (patch no fork do Gio), e não do Name:
//     o Name junta teclas diferentes ("+" do numérico e "=") e não tem as
//     que o Gio não nomeia (CapsLock, Insert, a "/?" do ABNT2).
//
//   - SSH e VNC querem o CARACTERE (bytes no terminal, keysym no VNC). O
//     Name é a tecla física sem layout — o Windows manda o mesmo "1" com e
//     sem Shift — e montar o caractere a partir dele era assumir teclado
//     US: Shift+1 saía "1" em vez de "!", Shift+; saía ";" em vez de ":",
//     e no ABNT2 o "ç" e os acentos não saíam. O caractere certo é o que o
//     próprio Windows produz (WM_CHAR, que chega como key.EditEvent), já
//     com layout, Shift, CapsLock, AltGr e tecla morta resolvidos. A tecla
//     que produz texto, então, NÃO segue pelo key.Event para essas abas —
//     ver produzTexto e entrada_outros.go.

// keysimDoNome traduz key.Name para o keysym X11 que o VNC espera, nas
// teclas que não são texto (setas, F1-F12, Enter...) e na pontuação, que
// só segue por aqui com Ctrl ou Alt (Alt+. no bash, por exemplo).
var keysimDoNome = map[key.Name]uint32{
	key.NameLeftArrow:      0xff51,
	key.NameUpArrow:        0xff52,
	key.NameRightArrow:     0xff53,
	key.NameDownArrow:      0xff54,
	key.NameHome:           0xff50,
	key.NameEnd:            0xff57,
	key.NamePageUp:         0xff55,
	key.NamePageDown:       0xff56,
	key.NameReturn:         0xff0d,
	key.NameEscape:         0xff1b,
	key.NameDeleteBackward: 0xff08, // Backspace
	key.NameDeleteForward:  0xffff, // Delete
	key.NameTab:            0xff09,
	key.NameSpace:          0x0020,
	key.NameCtrl:           0xffe3,
	key.NameShift:          0xffe1,
	key.NameAlt:            0xffe9,
	key.NameSuper:          0xffeb,
	key.NameF1:             0xffbe,
	key.NameF2:             0xffbf,
	key.NameF3:             0xffc0,
	key.NameF4:             0xffc1,
	key.NameF5:             0xffc2,
	key.NameF6:             0xffc3,
	key.NameF7:             0xffc4,
	key.NameF8:             0xffc5,
	key.NameF9:             0xffc6,
	key.NameF10:            0xffc7,
	key.NameF11:            0xffc8,
	key.NameF12:            0xffc9,
	";":                    0x3b,
	"+":                    0x2b,
	",":                    0x2c,
	"-":                    0x2d,
	".":                    0x2e,
	"/":                    0x2f,
	"*":                    0x2a,
	"`":                    0x60,
	"[":                    0x5b,
	"\\":                   0x5c,
	"]":                    0x5d,
	"'":                    0x27,
}

// keysymDoScancode cobre o que o Name não diz: as teclas sem nome no Gio e
// o lado DIREITO dos modificadores (o Name é o mesmo dos dois lados).
var keysymDoScancode = map[uint32]uint32{
	0x36:   0xffe2, // Shift direito
	0xe01d: 0xffe4, // Ctrl direito
	0xe038: 0xffea, // Alt direito (AltGr)
	0xe05c: 0xffec, // Super direito
	0xe052: 0xff63, // Insert
	0xe05d: 0xff67, // Menu (tecla de contexto)
	0x3a:   0xffe5, // CapsLock
	0xe045: 0xff7f, // NumLock
	0x46:   0xff14, // ScrollLock
	0x45:   0xff13, // Pause
	0xe037: 0xff61, // PrintScreen
}

// keycodeX11DoNome traduz para o keycode X11 (evdev+8) que o RDP espera
// (ver internal/rdp e rdpshim.c, rs_tecla). É a reserva para quando o
// key.Event vem sem Scancode — o caminho normal é keycodeX11DoScancode.
// Os valores batem com os que teclas.go usa nos atalhos fixos
// (Ctrl+Alt+Del etc.) — mesma tabela evdev+8, conferida ali.
var keycodeX11DoNome = map[key.Name]uint32{
	key.NameLeftArrow:      113,
	key.NameUpArrow:        111,
	key.NameRightArrow:     114,
	key.NameDownArrow:      116,
	key.NameHome:           110,
	key.NameEnd:            115,
	key.NamePageUp:         112,
	key.NamePageDown:       117,
	key.NameReturn:         36,
	key.NameEscape:         9,
	key.NameDeleteBackward: 22,
	key.NameDeleteForward:  119,
	key.NameTab:            23,
	key.NameSpace:          65,
	key.NameCtrl:           37,
	key.NameShift:          50,
	key.NameAlt:            64,
	key.NameSuper:          133,
	key.NameF1:             67,
	key.NameF2:             68,
	key.NameF3:             69,
	key.NameF4:             70,
	key.NameF5:             71,
	key.NameF6:             72,
	key.NameF7:             73,
	key.NameF8:             74,
	key.NameF9:             75,
	key.NameF10:            76,
	key.NameF11:            95,
	key.NameF12:            96,
	";":                    47,
	"+":                    21,
	",":                    59,
	"-":                    20,
	".":                    60,
	"/":                    61,
	"`":                    49,
	"[":                    34,
	"\\":                   51,
	"]":                    35,
	"'":                    48,
}

// keycodeX11DoAlfanumerico cobre letras e dígitos na reserva por nome:
// evdev não numera por ordem alfabética, segue a posição física da tecla
// no teclado (linha QWERTY, depois ASDF, depois ZXCV) — por isso a tabela
// explícita, em vez de derivar de 'A'-'Z'.
var keycodeX11DoAlfanumerico = map[byte]uint32{
	'Q': 24, 'W': 25, 'E': 26, 'R': 27, 'T': 28, 'Y': 29, 'U': 30, 'I': 31, 'O': 32, 'P': 33,
	'A': 38, 'S': 39, 'D': 40, 'F': 41, 'G': 42, 'H': 43, 'J': 44, 'K': 45, 'L': 46,
	'Z': 52, 'X': 53, 'C': 54, 'V': 55, 'B': 56, 'N': 57, 'M': 58,
	'0': 19, '1': 10, '2': 11, '3': 12, '4': 13, '5': 14, '6': 15, '7': 16, '8': 17, '9': 18,
}

// keycodeX11DoScancodeEstendido: as teclas estendidas (prefixo E0) não
// seguem a regra "evdev = scancode", cada uma tem o seu código.
var keycodeX11DoScancodeEstendido = map[uint32]uint32{
	0x1c: 104, // Enter do numérico
	0x1d: 105, // Ctrl direito
	0x35: 106, // "/" do numérico
	0x37: 107, // PrintScreen
	0x38: 108, // Alt direito (AltGr)
	0x45: 77,  // NumLock — o Windows o marca como estendido; o Pause não
	0x46: 127, // Ctrl+Break
	0x47: 110, // Home
	0x48: 111, // seta para cima
	0x49: 112, // PageUp
	0x4b: 113, // seta para a esquerda
	0x4d: 114, // seta para a direita
	0x4f: 115, // End
	0x50: 116, // seta para baixo
	0x51: 117, // PageDown
	0x52: 118, // Insert
	0x53: 119, // Delete
	0x5b: 133, // Super esquerdo
	0x5c: 134, // Super direito
	0x5d: 135, // Menu
}

// keycodeX11DoScancode converte o scancode do PC (conjunto 1, com 0xE000
// nas estendidas — ver key.Event.Scancode) no keycode X11 (evdev+8). No
// bloco principal o código evdev É o scancode (Esc=1 ... F12=88), o que
// cobre de uma vez letras, dígitos, pontuação, numérico, CapsLock e a
// tecla ao lado do Z (0x56). Zero quando não há tradução — é o caso das
// teclas "falsas" que o driver injeta (o Shift estendido que acompanha o
// numérico com NumLock), que não devem chegar ao outro lado.
func keycodeX11DoScancode(sc uint32) uint32 {
	if sc&0xe000 == 0xe000 {
		return keycodeX11DoScancodeEstendido[sc&0xff]
	}
	switch sc {
	case 0x45:
		return 127 // Pause (sem E0; o NumLock vem com)
	case 0x54:
		return 107 // SysRq (Alt+PrintScreen)
	case 0x73:
		return 97 // "/?" do ABNT2 (VK_ABNT_C1, evdev KEY_RO)
	case 0x7e:
		return 129 // "." do numérico do ABNT2 (VK_ABNT_C2, evdev KEY_KPCOMMA)
	}
	if sc >= 0x01 && sc <= 0x58 {
		return sc + 8
	}
	return 0
}

// traduzirTecla devolve (keysym X11, keycode X11) para um key.Event — os
// mesmos dois valores que grab.Start entrega no Linux. Qualquer um dos
// dois pode vir zero: RDP usa só o keycode e VNC/SSH só o keysym, e cada
// lado descarta o que vier sem o seu (ver roteadorTeclado.tecla).
//
// Letras são caso especial no keysym: o Windows manda o MESMO Name (VK,
// sempre maiúsculo) esteja Shift apertado ou não. Só chega aqui letra com
// Ctrl ou Alt junto (a sem modificador é texto e vem pelo EditEvent), e aí
// o Shift decide a caixa — Ctrl+Shift+C/V do terminal depende disso.
func traduzirTecla(ke key.Event) (keysym, keycodeX11 uint32) {
	nome := ke.Name
	if len(nome) == 1 {
		c := nome[0]
		switch {
		case c >= 'A' && c <= 'Z':
			keysym = uint32(c)
			if !ke.Modifiers.Contain(key.ModShift) {
				keysym += 'a' - 'A'
			}
		case c >= '0' && c <= '9':
			keysym = uint32(c)
		}
	}
	if keysym == 0 {
		keysym = keysimDoNome[nome]
	}
	if ks, ok := keysymDoScancode[ke.Scancode]; ok {
		keysym = ks
	}

	if ke.Scancode != 0 {
		keycodeX11 = keycodeX11DoScancode(ke.Scancode)
	} else if len(nome) == 1 {
		if kc, ok := keycodeX11DoAlfanumerico[nome[0]]; ok {
			keycodeX11 = kc
		} else {
			keycodeX11 = keycodeX11DoNome[nome]
		}
	} else {
		keycodeX11 = keycodeX11DoNome[nome]
	}
	return keysym, keycodeX11
}

// produzTexto diz se a tecla, com os modificadores que vieram, vira
// CARACTERE no Windows — e portanto chega também como EditEvent. Para
// SSH e VNC essas teclas não seguem pelo key.Event, senão o caractere
// sairia duas vezes (e errado na primeira).
//
// Ctrl OU Alt sozinho faz da tecla um atalho (Ctrl+C, Alt+.), que o
// Windows não transforma em caractere imprimível: segue como tecla. Ctrl
// E Alt juntos é o AltGr (é assim que o Windows o reporta), que produz
// caractere — "/" e "?" no ABNT2 sem a tecla dedicada, por exemplo.
func produzTexto(ke key.Event) bool {
	return teclaDeTexto(ke) && modsDeTexto(ke.Modifiers)
}

func modsDeTexto(m key.Modifiers) bool {
	return m.Contain(key.ModCtrl) == m.Contain(key.ModAlt)
}

// teclaDeTexto: letras, dígitos, pontuação e numérico têm Name de UM
// caractere ASCII ("A", "1", ";", "+"); Espaço tem nome próprio; as duas
// teclas do ABNT2 que o Gio não nomeia vêm com Name vazio e só o scancode.
//
// ASCII, e não "uma rune": Enter, Esc, Backspace, Delete, setas, Home/End
// e PgUp/PgDn TAMBÉM têm nome de uma rune só no Gio ("⏎", "⎋", "⌫", "←",
// "⇱"...), e não são texto.
func teclaDeTexto(ke key.Event) bool {
	switch {
	case ke.Name == key.NameSpace:
		return true
	case len(ke.Name) == 1:
		return ke.Name[0] > ' ' && ke.Name[0] < 0x7f
	case ke.Name == "":
		return ke.Scancode == 0x73 || ke.Scancode == 0x7e
	}
	return false
}
