package main

import (
	"testing"

	"acessos-go/internal/conexoes"
)

// A 2.9.0 saiu com o nome de uma empresa FIXO acima da saudação, para todo
// mundo. Ele só aparece quando o inventário diz qual é ([geral] empresa).
func TestSobretituloSoComEmpresaDoInventario(t *testing.T) {
	casos := []struct {
		geral map[string]string
		quer  string
	}{
		{nil, "PAINEL DE ACESSOS"},
		{map[string]string{"empresa": "  "}, "PAINEL DE ACESSOS"},
		{map[string]string{"empresa": "Rede Horizonte"}, "REDE HORIZONTE · PAINEL DE ACESSOS"},
	}
	for _, c := range casos {
		d := &dashTab{arq: &conexoes.Arquivo{Geral: c.geral}}
		if got := d.sobretitulo(); got != c.quer {
			t.Errorf("empresa=%q: %q, quer %q", c.geral["empresa"], got, c.quer)
		}
	}
	if got := (&dashTab{}).sobretitulo(); got != "PAINEL DE ACESSOS" {
		t.Errorf("sem inventário: %q", got)
	}
}
