package main

import "testing"

// O que vai para o terminal remoto em cada tecla com modificador. Vale para
// as duas plataformas: o codificador é o mesmo, só a origem do keysym muda.
func TestBytesDaTeclaComModificador(t *testing.T) {
	const (
		ksUp, ksRight, ksHome, ksPgUp = 0xff52, 0xff53, 0xff50, 0xff55
		ksTab, ksReturn, ksF5         = 0xff09, 0xff0d, 0xffc2
	)
	shift := modificadores{shift: true}
	ctrl := modificadores{ctrl: true}
	alt := modificadores{alt: true}
	ctrlShift := modificadores{ctrl: true, shift: true}

	casos := []struct {
		desc string
		ks   uint32
		m    modificadores
		quer string
	}{
		{"seta pura", ksUp, modificadores{}, "\x1b[A"},
		{"Ctrl+→ pula palavra", ksRight, ctrl, "\x1b[1;5C"},
		{"Shift+seta marca", ksUp, shift, "\x1b[1;2A"},
		{"Alt+seta", ksRight, alt, "\x1b[1;3C"},
		{"Ctrl+Shift+→", ksRight, ctrlShift, "\x1b[1;6C"},
		{"Ctrl+Home", ksHome, ctrl, "\x1b[1;5H"},
		{"Shift+PgUp", ksPgUp, shift, "\x1b[5;2~"},
		{"Ctrl+Delete", ksDelete, ctrl, "\x1b[3;5~"},
		{"Shift+F5", ksF5, shift, "\x1b[15;2~"},
		{"Shift+Tab (Windows)", ksTab, shift, "\x1b[Z"},
		{"ISO_Left_Tab (Linux)", ksISOLeftTab, shift, "\x1b[Z"},
		{"Tab puro", ksTab, modificadores{}, "\t"},
		{"Alt+Enter", ksReturn, alt, "\x1b\r"},
		{"Ctrl+C", 'c', ctrl, "\x03"},
		{"Ctrl+Shift+C não é controle", 'C', ctrlShift, "\x03"},
		{"Alt+.", '.', alt, "\x1b."},
	}
	for _, c := range casos {
		if got := string(bytesDaTecla(c.ks, c.m)); got != c.quer {
			t.Errorf("%s: %q, queria %q", c.desc, got, c.quer)
		}
	}
}

func TestKeysymDaRunaEhInversoDeRunaDoKeysym(t *testing.T) {
	for _, r := range "aZ9!:|_?@~çãéÇ€ñ" {
		ks := keysymDaRuna(r)
		if ks == 0 {
			t.Errorf("%q: sem keysym", r)
			continue
		}
		if volta := runaDoKeysym(ks); volta != r {
			t.Errorf("%q → %#x → %q", r, ks, volta)
		}
	}
	for _, r := range "\x1b\r\t\x00" {
		if ks := keysymDaRuna(r); ks != 0 {
			t.Errorf("controle %q virou keysym %#x", r, ks)
		}
	}
}

func TestTextoImprimivelTiraControle(t *testing.T) {
	if got := textoImprimivel("a\x1b[2Jb\r\tç"); got != "a[2Jbç" {
		t.Fatalf("%q", got)
	}
}
