package main

import (
	"strings"
	"unicode"

	"github.com/hinshun/vt10x"
)

// Gestos de mouse do terminal que todo terminal tem e o nosso não tinha
// (2.9.3): duplo clique marca a palavra, triplo marca a linha, e o botão
// direito traz copiar e colar.

// caractereDePalavra: o que o duplo clique leva junto. Além de letra e
// número, o que aparece em caminho, IP, host e usuário@máquina — para um
// duplo clique em /var/log/pdv/sincronia.log ou em 10.20.1.12 pegar o
// endereço inteiro, que é o que se quer copiar.
func caractereDePalavra(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("._-/~:@+=%,", r)
}

// limitesPalavra devolve o começo e o fim da palavra na posição x da
// linha (ok=false se x cair num espaço).
func limitesPalavra(linha []rune, x int) (ini, fim int, ok bool) {
	if x < 0 || x >= len(linha) || !caractereDePalavra(linha[x]) {
		return 0, 0, false
	}
	ini, fim = x, x
	for ini > 0 && caractereDePalavra(linha[ini-1]) {
		ini--
	}
	for fim < len(linha)-1 && caractereDePalavra(linha[fim+1]) {
		fim++
	}
	return ini, fim, true
}

// linhaDaTela lê a linha y da tela como runas (célula vazia vira espaço).
func (t *sshTab) linhaDaTela(y int) []rune {
	t.term.Lock()
	defer t.term.Unlock()
	cols, rows := t.term.Size()
	if y < 0 || y >= rows {
		return nil
	}
	linha := make([]rune, cols)
	for x := range cols {
		c := t.term.Cell(x, y).Char
		if c == 0 {
			c = ' '
		}
		linha[x] = c
	}
	return linha
}

// marcarPorClique: n=2 marca a palavra sob c, n>=3 a linha inteira (até o
// último caractere, sem o espaço da borda).
func (t *sshTab) marcarPorClique(c celula, n int) {
	linha := t.linhaDaTela(c.y)
	if linha == nil {
		return
	}
	var ini, fim int
	if n >= 3 {
		aparada := []rune(strings.TrimRight(string(linha), " "))
		if len(aparada) == 0 {
			return
		}
		fim = len(aparada) - 1
	} else {
		var ok bool
		if ini, fim, ok = limitesPalavra(linha, c.x); !ok {
			return
		}
	}
	t.mu.Lock()
	t.selA, t.selB = celula{x: ini, y: c.y}, celula{x: fim, y: c.y}
	t.selAtiva, t.arrastando = true, false
	t.mu.Unlock()
	t.invalidar()
}

// menuDoTerminal é o menu do botão direito (sem modo de mouse ligado; com
// ele, o clique direito vai para o programa, como sempre foi).
func (t *sshTab) menuDoTerminal() {
	var itens []*itemMenu
	if _, ok := t.textoSelecionado(); ok {
		itens = append(itens, &itemMenu{rotulo: "Copiar", acao: t.copiarSelecao})
	}
	itens = append(itens,
		&itemMenu{rotulo: "Colar", acao: t.colarDoSistema},
		&itemMenu{rotulo: "Copiar a tela inteira", acao: func() {
			// String() do vt10x tranca o mutex por conta própria (ver
			// copiarSelecao): nada de Lock em volta.
			publicarClipboard(t.w, strings.TrimRight(t.term.String(), "\n \t"))
			t.setEstado("tela copiada")
		}},
	)
	abrirMenu(t.w, ultimaPosPonteiro(), itens)
	t.invalidar()
}

// telaCheiaSemMouse: programa de tela cheia (tela alternativa) que não
// pediu relato de mouse, com a tela ao vivo (sem histórico rolado).
func (t *sshTab) telaCheiaSemMouse() bool {
	t.mu.Lock()
	rolagem := t.rolagem
	t.mu.Unlock()
	if rolagem != 0 || t.term == nil {
		return false
	}
	t.term.Lock()
	m := t.term.Mode()
	t.term.Unlock()
	return m&vt10x.ModeAltScreen != 0 && m&vt10x.ModeMouseMask == 0
}
