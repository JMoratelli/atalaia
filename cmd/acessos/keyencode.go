package main

import (
	"fmt"
	"strings"
	"unicode"
)

// Tradução de tecla física (o que o internal/grab entrega, direto do
// Wayland) para os bytes que um terminal espera. O grab dá keysym X11 já
// resolvido pelo layout ativo, então acento e pontuação saem certos sem
// tabela de teclado nossa.
//
// Escopo desta versão: uso normal de shell. Fora: teclado morto/composição
// (o keysym vem 0), modo de aplicação do teclado numérico e relatório de
// mouse.

// keysyms de modificador — o grab entrega apertar E soltar de cada um, que
// é justamente o que o key.Event do Gio não dá nesta pilha.
const (
	ksShiftL = 0xffe1
	ksShiftR = 0xffe2
	ksCtrlL  = 0xffe3
	ksCtrlR  = 0xffe4
	ksAltL   = 0xffe9
	ksAltR   = 0xffea
)

// modificadores acompanha o estado de Ctrl/Shift/Alt.
type modificadores struct{ ctrl, shift, alt bool }

// anotar registra o aperta/solta de um modificador. Devolve true se a
// tecla ERA um modificador (e portanto não gera bytes).
func (m *modificadores) anotar(keysym uint32, pressed bool) bool {
	switch keysym {
	case ksShiftL, ksShiftR:
		m.shift = pressed
		return true
	case ksCtrlL, ksCtrlR:
		m.ctrl = pressed
		return true
	case ksAltL, ksAltR:
		m.alt = pressed
		return true
	}
	return false
}

// especiais: keysym -> sequência. Só as que independem de modificador.
var especiais = map[uint32]string{
	0xff0d: "\r",      // Return
	0xff8d: "\r",      // KP_Enter
	0xff09: "\t",      // Tab
	0xff1b: "\x1b",    // Escape
	0xff08: "\x7f",    // BackSpace (DEL, como todo terminal moderno)
	0xffff: "\x1b[3~", // Delete
	0xff63: "\x1b[2~", // Insert
	0xff50: "\x1b[H",  // Home
	0xff57: "\x1b[F",  // End
	0xff55: "\x1b[5~", // Page Up
	0xff56: "\x1b[6~", // Page Down
	0xff52: "\x1b[A",  // Up
	0xff54: "\x1b[B",  // Down
	0xff53: "\x1b[C",  // Right
	0xff51: "\x1b[D",  // Left
	0xffbe: "\x1bOP",  // F1
	0xffbf: "\x1bOQ",  // F2
	0xffc0: "\x1bOR",  // F3
	0xffc1: "\x1bOS",  // F4
	0xffc2: "\x1b[15~",
	0xffc3: "\x1b[17~",
	0xffc4: "\x1b[18~",
	0xffc5: "\x1b[19~",
	0xffc6: "\x1b[20~",
	0xffc7: "\x1b[21~",
	0xffc8: "\x1b[23~",
	0xffc9: "\x1b[24~", // F12
}

// csiModificavel: teclas de cursor, edição e função que, com Shift, Alt ou
// Ctrl, mudam de sequência na convenção do xterm — a que gnome-terminal,
// Konsole e o Windows Terminal seguem: "\x1b[<n>;<mod><final>". Sem
// modificador continuam saindo como em especiais.
var csiModificavel = map[uint32]struct {
	n     string
	final byte
}{
	0xff52: {"1", 'A'}, // Up
	0xff54: {"1", 'B'}, // Down
	0xff53: {"1", 'C'}, // Right
	0xff51: {"1", 'D'}, // Left
	0xff50: {"1", 'H'}, // Home
	0xff57: {"1", 'F'}, // End
	0xff63: {"2", '~'}, // Insert
	0xffff: {"3", '~'}, // Delete
	0xff55: {"5", '~'}, // Page Up
	0xff56: {"6", '~'}, // Page Down
	0xffbe: {"1", 'P'}, // F1
	0xffbf: {"1", 'Q'}, // F2
	0xffc0: {"1", 'R'}, // F3
	0xffc1: {"1", 'S'}, // F4
	0xffc2: {"15", '~'},
	0xffc3: {"17", '~'},
	0xffc4: {"18", '~'},
	0xffc5: {"19", '~'},
	0xffc6: {"20", '~'},
	0xffc7: {"21", '~'},
	0xffc8: {"23", '~'},
	0xffc9: {"24", '~'}, // F12
}

// ksISOLeftTab é o keysym que o xkb dá para Shift+Tab no Linux.
const ksISOLeftTab = 0xfe20

// bytesDaTecla traduz um keysym (com os modificadores correntes) para os
// bytes a mandar. Devolve nil quando a tecla não produz nada.
func bytesDaTecla(keysym uint32, m modificadores) []byte {
	// Shift+Tab é "tabulação para trás" (CBT), que menus de terminal
	// (nmtui, mc, dialog) usam para voltar um campo. No Linux ele chega
	// como ISO_Left_Tab, que não estava em especiais e não mandava nada;
	// no Windows chega como Tab com Shift, e mandava um Tab comum.
	if keysym == ksISOLeftTab || keysym == 0xff09 && m.shift {
		return []byte("\x1b[Z")
	}
	// Seta, Home/End, PgUp/PgDn, Insert/Delete e F1-F12 com modificador:
	// a forma CSI do xterm, com o parâmetro 1 + Shift + 2·Alt + 4·Ctrl.
	// É o que faz Ctrl+←/→ pular palavra no bash (o inputrc das
	// distribuições amarra "\e[1;5C"/"\e[1;5D"), Shift+seta marcar texto
	// e Ctrl+Home/End ir ao começo/fim do arquivo no nano. Antes o
	// modificador era ignorado e a seta saía pura. Ctrl+Delete ("\x1b[3;5~",
	// "chopwordright" no nano, kill-word no readline ≥ 8.1) é um caso disto.
	if c, ok := csiModificavel[keysym]; ok {
		if p := parametroModificador(m); p > 1 {
			return []byte(fmt.Sprintf("\x1b[%s;%d%c", c.n, p, c.final))
		}
	}
	if s, ok := especiais[keysym]; ok {
		if m.alt {
			return append([]byte{0x1b}, s...)
		}
		return []byte(s)
	}

	r := runaDoKeysym(keysym)
	if r == 0 {
		return nil
	}

	if m.ctrl {
		// Ctrl+letra = o caractere de controle correspondente. Vale para
		// @ [ \ ] ^ _ e espaço também, que é a faixa 0x40-0x5f da tabela.
		c := r
		if c >= 'a' && c <= 'z' {
			c -= 32
		}
		if c == ' ' {
			c = '@'
		}
		if c >= '@' && c <= '_' {
			b := []byte{byte(c) & 0x1f}
			if m.alt {
				return append([]byte{0x1b}, b...)
			}
			return b
		}
		return nil
	}

	b := []byte(string(r))
	if m.alt {
		return append([]byte{0x1b}, b...)
	}
	return b
}

// parametroModificador é o parâmetro de modificador do xterm: 1 sem
// nenhum, somando 1 para Shift, 2 para Alt e 4 para Ctrl.
func parametroModificador(m modificadores) int {
	p := 1
	if m.shift {
		p++
	}
	if m.alt {
		p += 2
	}
	if m.ctrl {
		p += 4
	}
	return p
}

// runaDoKeysym converte um keysym X11 imprimível em rune. Latin-1 é
// identidade; o resto usa a convenção Unicode do X (0x01000000 + ponto de
// código).
func runaDoKeysym(keysym uint32) rune {
	switch {
	case keysym >= 0x20 && keysym <= 0x7e:
		return rune(keysym)
	case keysym >= 0xa0 && keysym <= 0xff:
		return rune(keysym)
	case keysym >= 0x01000100 && keysym <= 0x0110ffff:
		return rune(keysym - 0x01000000)
	// teclado numérico com Num Lock: keysyms próprios, mesmo resultado
	case keysym >= 0xffb0 && keysym <= 0xffb9:
		return rune('0' + (keysym - 0xffb0))
	case keysym == 0xffaa:
		return '*'
	case keysym == 0xffab:
		return '+'
	case keysym == 0xffad:
		return '-'
	case keysym == 0xffae:
		return '.'
	case keysym == 0xffaf:
		return '/'
	}
	return 0
}

// keysymDaRuna é o inverso de runaDoKeysym: Latin-1 é o próprio código, o
// resto vai pela convenção Unicode do X (0x01000000 + ponto de código).
// Zero para o que não é caractere imprimível. Quem usa é o texto que o
// Windows já resolveu (HandleTexto, no VNC).
func keysymDaRuna(r rune) uint32 {
	switch {
	case !unicode.IsPrint(r):
		return 0
	case r >= 0x20 && r <= 0x7e, r >= 0xa0 && r <= 0xff:
		return uint32(r)
	default:
		return 0x01000000 + uint32(r)
	}
}

// textoImprimivel tira do texto os caracteres de controle. O EditEvent só
// traz o que o Windows já filtrou como imprimível, mas o texto vai direto
// para um terminal remoto, e um ESC que escapasse viraria sequência de
// comando do outro lado.
func textoImprimivel(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsPrint(r) {
			return r
		}
		return -1
	}, s)
}
