package main

import (
	"strings"
	"time"

	"github.com/hinshun/vt10x"
)

// Clique move o cursor (2.9.2).
//
// Num terminal o clique só vira coisa do PROGRAMA quando ele pede relato
// de mouse (htop, mc, o nano com "set mouse"); o nano de fábrica não pede,
// e aí clicar no meio do arquivo não fazia nada além de limpar a seleção —
// é assim também no Konsole e no PuTTY, mas não é o que se espera de um
// editor. Sem modo de mouse, um clique SIMPLES (sem arrastar) passa a mover
// o cursor até a célula clicada com as mesmas setas que o teclado manda,
// como o "Option+clique" do iTerm2.
//
// Os cuidados, porque seta não é inofensiva em todo lugar:
//   - no shell (tela normal) só anda na HORIZONTAL e só na linha do cursor:
//     seta para cima no bash puxa o histórico, e clicar numa saída antiga
//     trocaria o comando que se está digitando;
//   - na vertical só em programa de tela cheia (tela alternativa: nano,
//     vim, mc), e só com o cursor visível — cursor escondido é menu, onde
//     a seta muda a opção marcada;
//   - nada com o histórico rolado (a tela não é a de agora) nem com modo de
//     mouse ligado (aí o clique vai para o programa, como sempre foi);
//   - primeiro a vertical, depois a horizontal medida de novo: ao trocar de
//     linha o editor pode pôr o cursor noutra coluna (linha mais curta), e
//     tab ou acento ocupam um número de células diferente do de teclas.
//     Por isso a horizontal confere e corrige até três vezes, e para se o
//     cursor não andar.

const maxSetas = 300 // um clique não pode virar uma enxurrada

const (
	setaCima     = "\x1b[A"
	setaBaixo    = "\x1b[B"
	setaDireita  = "\x1b[C"
	setaEsquerda = "\x1b[D"
)

func (t *sshTab) cursorAtual() vt10x.Cursor {
	t.term.Lock()
	defer t.term.Unlock()
	return t.term.Cursor()
}

// podeMoverCursor aplica os cuidados acima antes de mandar qualquer seta.
func (t *sshTab) podeMoverCursor(alvo celula) bool {
	if t.term == nil {
		return false
	}
	t.mu.Lock()
	rolagem := t.rolagem
	t.mu.Unlock()
	if rolagem != 0 {
		return false
	}
	t.term.Lock()
	cur, modo, visivel := t.term.Cursor(), t.term.Mode(), t.term.CursorVisible()
	t.term.Unlock()
	if !visivel || modo&vt10x.ModeMouseMask != 0 {
		return false
	}
	if modo&vt10x.ModeAltScreen == 0 && alvo.y != cur.Y {
		return false
	}
	return alvo.x != cur.X || alvo.y != cur.Y
}

// cliqueMoveCursor dispara o movimento em segundo plano: esperar o
// programa redesenhar entre a vertical e a horizontal não pode segurar o
// quadro.
func (t *sshTab) cliqueMoveCursor(alvo celula) {
	if !t.podeMoverCursor(alvo) {
		return
	}
	go t.moverCursorPara(alvo, t.esperarCursorParar)
}

func (t *sshTab) moverCursorPara(alvo celula, esperar func()) {
	cur := t.cursorAtual()
	if dy := alvo.y - cur.Y; dy != 0 {
		seta := setaBaixo
		if dy < 0 {
			seta = setaCima
		}
		t.enviar([]byte(strings.Repeat(seta, min(abs(dy), maxSetas))))
		esperar()
	}
	for range 3 {
		antes := t.cursorAtual()
		dx := alvo.x - antes.X
		if dx == 0 {
			return
		}
		seta := setaDireita
		if dx < 0 {
			seta = setaEsquerda
		}
		t.enviar([]byte(strings.Repeat(seta, min(abs(dx), maxSetas))))
		esperar()
		if c := t.cursorAtual(); c.X == antes.X && c.Y == antes.Y {
			return // o programa não andou (fim da linha, prompt): não insistir
		}
	}
}

// esperarCursorParar dá tempo de o remoto responder às setas: espera o
// cursor mudar e ficar parado por duas leituras seguidas, com teto, para
// um remoto lento ou mudo não prender a goroutine.
func (t *sshTab) esperarCursorParar() {
	time.Sleep(30 * time.Millisecond)
	anterior := t.cursorAtual()
	parado := 0
	for limite := time.Now().Add(400 * time.Millisecond); time.Now().Before(limite); {
		time.Sleep(20 * time.Millisecond)
		c := t.cursorAtual()
		if c.X == anterior.X && c.Y == anterior.Y {
			parado++
			if parado >= 2 {
				return
			}
			continue
		}
		anterior, parado = c, 0
	}
}
