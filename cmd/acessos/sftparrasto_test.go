package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/op"
)

// Até a 2.9.1 o arrasto levava só o que JÁ estivesse marcado: apertar num
// arquivo não marcado e arrastar não levava ele (levava a seleção antiga,
// ou nada). Agora vai o item apertado, que passa a ser a seleção.
func TestArrastarArquivoNaoMarcadoLevaEle(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"a.txt", "b.txt", "c.txt"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ab := sftpDeTeste(t, dir)
	var r input.Router
	var ops op.Ops
	quadroSFTP(ab, &r, &ops)
	quadroSFTP(ab, &r, &ops)
	ab.local.sel["a.txt"] = true // seleção antiga, que NÃO deve ir

	y := float32(ab.local.listaY + 2*ab.local.linhaH + ab.local.linhaH/2) // linha de c.txt
	x := float32(ab.local.area.Min.X + 80)
	destino := float32(ab.remoto.area.Min.X + 120)
	mouse := func(k pointer.Kind, px float32, b pointer.Buttons) pointer.Event {
		return pointer.Event{Kind: k, Position: f32.Pt(px, y), Source: pointer.Mouse, Buttons: b}
	}
	quadroSFTP(ab, &r, &ops, mouse(pointer.Press, x, pointer.ButtonPrimary))
	quadroSFTP(ab, &r, &ops, mouse(pointer.Move, x+40, pointer.ButtonPrimary))
	quadroSFTP(ab, &r, &ops, mouse(pointer.Move, destino, pointer.ButtonPrimary))
	if !ab.arrastando {
		t.Fatal("arrastar a linha não começou o arrasto")
	}
	quadroSFTP(ab, &r, &ops, mouse(pointer.Release, destino, 0))
	quadroSFTP(ab, &r, &ops)

	if got := selecionados(&ab.local); len(got) != 1 || got[0].nome != "c.txt" {
		t.Fatalf("depois do arrasto a seleção é %v, esperado só c.txt", got)
	}
	// Sem cliente SFTP a transferência não anda, mas ela foi PEDIDA com
	// itens: com a lista vazia a mensagem seria a de "selecione".
	if ab.msg != "sem conexão SFTP" {
		t.Fatalf("mensagem %q: a transferência não foi pedida com o item arrastado", ab.msg)
	}
}

func TestShiftCliqueMarcaFaixa(t *testing.T) {
	p := &painelArquivos{sel: map[string]bool{}}
	vis := []itemArquivo{{nome: "a"}, {nome: "b"}, {nome: "c"}, {nome: "d"}, {nome: "e"}}
	ab := &sftpTab{}
	ab.clicarItem(p, vis, 1, false, false) // b
	ab.clicarItem(p, vis, 3, false, true)  // Shift em d: b..d
	quer := map[string]bool{"b": true, "c": true, "d": true}
	if !reflect.DeepEqual(p.sel, quer) {
		t.Fatalf("Shift+clique marcou %v, esperado %v", p.sel, quer)
	}
	ab.clicarItem(p, vis, 0, false, true) // Shift em a, a partir do MESMO ancora: a..b
	if !reflect.DeepEqual(p.sel, map[string]bool{"a": true, "b": true}) {
		t.Fatalf("segundo Shift+clique marcou %v, esperado a e b", p.sel)
	}
	ab.clicarItem(p, vis, 4, true, false) // Ctrl soma e
	if !p.sel["e"] || len(p.sel) != 3 {
		t.Fatalf("Ctrl+clique deveria somar e: %v", p.sel)
	}
}

func TestMigalhasRemotas(t *testing.T) {
	casos := map[string][]migalha{
		"/home/suporte": {{"/", "/"}, {"home", "/home"}, {"suporte", "/home/suporte"}},
		"/":             {{"/", "/"}},
		"/srv//dados/":  {{"/", "/"}, {"srv", "/srv"}, {"dados", "/srv/dados"}},
		".":             {{".", "."}},
	}
	for entrada, quer := range casos {
		if got := migalhas(entrada, true); !reflect.DeepEqual(got, quer) {
			t.Errorf("migalhas(%q) = %v, quer %v", entrada, got, quer)
		}
	}
}
