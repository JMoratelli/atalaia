//go:build ignore

// Conta as linhas de código do projeto para o selo do README:
//
//	go run scripts/linhas-de-codigo.go
//
// Conta só linha com código — sem linha em branco e sem comentário — dos
// arquivos rastreados pelo git, fora third_party/ (o fork do Gio e o vt10x
// não são nossos) e vendor/. O Go passa pelo scanner da própria linguagem,
// então comentário dentro de string não engana a conta; C, Python, shell e
// o script do instalador vão por regra simples de comentário.
package main

import (
	"bytes"
	"fmt"
	"go/scanner"
	"go/token"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

func main() {
	saida, err := exec.Command("git", "ls-files", "-z",
		"*.go", "*.c", "*.h", "*.py", "*.sh", "*.iss",
		":!third_party", ":!vendor").Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, "git ls-files:", err)
		os.Exit(1)
	}
	var app, testes, outros int
	for _, f := range strings.Split(strings.TrimRight(string(saida), "\x00"), "\x00") {
		src, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		switch {
		case strings.HasSuffix(f, "_test.go"):
			testes += linhasGo(f, src)
		case strings.HasSuffix(f, ".go"):
			app += linhasGo(f, src)
		default:
			outros += linhasOutras(f, src)
		}
	}
	fmt.Printf("Go do app: %d\nGo dos testes: %d\nC, Python, shell e instalador: %d\ntotal: %d\n",
		app, testes, outros, app+testes+outros)
}

// linhasGo conta as linhas em que o scanner acha algum token (comentário
// não é token sem ScanComments). String crua de várias linhas conta todas.
func linhasGo(nome string, src []byte) int {
	fs := token.NewFileSet()
	arq := fs.AddFile(nome, -1, len(src))
	var s scanner.Scanner
	s.Init(arq, src, nil, 0)
	linhas := map[int]bool{}
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			return len(linhas)
		}
		ini := fs.Position(pos).Line
		linhas[ini] = true
		if tok == token.STRING {
			for i := 1; i <= strings.Count(lit, "\n"); i++ {
				linhas[ini+i] = true
			}
		}
	}
}

var (
	blocoC = regexp.MustCompile(`(?s)/\*.*?\*/`)
	linhaC = regexp.MustCompile(`//[^\n]*`)
)

func linhasOutras(nome string, src []byte) int {
	marca := "#"
	switch {
	case strings.HasSuffix(nome, ".c"), strings.HasSuffix(nome, ".h"):
		// o comentário de bloco vira as quebras de linha que ele tinha
		src = blocoC.ReplaceAllFunc(src, func(b []byte) []byte {
			return bytes.Repeat([]byte("\n"), bytes.Count(b, []byte("\n")))
		})
		src = linhaC.ReplaceAll(src, nil)
		marca = ""
	case strings.HasSuffix(nome, ".iss"):
		marca = ";"
	}
	n := 0
	for _, l := range strings.Split(string(src), "\n") {
		l = strings.TrimSpace(l)
		if l != "" && (marca == "" || !strings.HasPrefix(l, marca)) {
			n++
		}
	}
	return n
}
