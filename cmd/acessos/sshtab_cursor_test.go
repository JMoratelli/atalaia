package main

import (
	"strings"
	"testing"

	"github.com/hinshun/vt10x"
)

func abaCursorDeTeste(t *testing.T) (*sshTab, *bufFechavel) {
	t.Helper()
	buf := &bufFechavel{}
	tab := &sshTab{cols: 80, rows: 24, entrada: buf}
	tab.term = vt10x.New(vt10x.WithSize(80, 24))
	return tab, buf
}

// No editor (tela alternativa) o clique leva o cursor: primeiro a linha,
// depois a coluna, com as setas que o teclado manda.
func TestCliqueMoveCursorNoEditor(t *testing.T) {
	tab, buf := abaCursorDeTeste(t)
	tab.term.Write([]byte("\x1b[?1049h\x1b[2;3H")) // tela cheia, cursor na linha 2, coluna 3 (1-based)
	alvo := celula{x: 10, y: 5}
	if !tab.podeMoverCursor(alvo) {
		t.Fatal("no editor o clique devia poder mover o cursor")
	}
	tab.moverCursorPara(alvo, func() {})
	quer := strings.Repeat(setaBaixo, 4) + strings.Repeat(setaDireita, 8)
	if got := buf.String(); got != quer {
		t.Fatalf("mandou %q, esperado %q", got, quer)
	}
}

// No shell, seta para cima puxa o histórico: só anda na linha do cursor.
func TestCliqueNoShellSoNaLinhaDoCursor(t *testing.T) {
	tab, buf := abaCursorDeTeste(t)
	tab.term.Write([]byte("$ ls -la /var/log\x1b[1;5H"))
	if tab.podeMoverCursor(celula{x: 3, y: 4}) {
		t.Fatal("no shell, clicar em OUTRA linha não pode mandar seta")
	}
	alvo := celula{x: 12, y: 0}
	if !tab.podeMoverCursor(alvo) {
		t.Fatal("no shell, clicar na linha do cursor devia mover na horizontal")
	}
	tab.moverCursorPara(alvo, func() {})
	if got, quer := buf.String(), strings.Repeat(setaDireita, 8); got != quer {
		t.Fatalf("mandou %q, esperado %q", got, quer)
	}
}

// Com modo de mouse ligado o clique é do programa; com o cursor escondido
// (menu) seta mudaria a opção marcada. Nos dois casos, nada de seta.
func TestCliqueNaoMexeComMouseOuCursorEscondido(t *testing.T) {
	tab, _ := abaCursorDeTeste(t)
	tab.term.Write([]byte("\x1b[?1049h\x1b[?1000h\x1b[?1006h"))
	if tab.podeMoverCursor(celula{x: 5, y: 5}) {
		t.Fatal("com o relato de mouse ligado, o clique não pode virar seta")
	}
	tab2, _ := abaCursorDeTeste(t)
	tab2.term.Write([]byte("\x1b[?1049h\x1b[?25l"))
	if tab2.podeMoverCursor(celula{x: 5, y: 5}) {
		t.Fatal("com o cursor escondido, o clique não pode virar seta")
	}
}
