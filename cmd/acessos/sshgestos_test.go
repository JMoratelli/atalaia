package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"image"
	"net"
	"strings"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"github.com/hinshun/vt10x"
	"golang.org/x/crypto/ssh"
)

func TestLimitesPalavraPegaCaminhoEIP(t *testing.T) {
	linha := []rune("suporte@srv:~$ tail -n 5 /var/log/backup.log de 10.20.1.12")
	casos := []struct {
		x    int
		quer string
	}{
		{30, "/var/log/backup.log"},
		{2, "suporte@srv:~"},
		{50, "10.20.1.12"},
	}
	for _, c := range casos {
		ini, fim, ok := limitesPalavra(linha, c.x)
		if !ok || string(linha[ini:fim+1]) != c.quer {
			t.Errorf("x=%d: %q (ok=%v), quer %q", c.x, string(linha[ini:fim+1]), ok, c.quer)
		}
	}
	if _, _, ok := limitesPalavra(linha, 14); ok {
		t.Error("clicar no espaço não pode marcar palavra")
	}
}

func TestLinhasDoColar(t *testing.T) {
	casos := map[string]int{"ls": 1, "ls\n": 1, "ls\npwd": 2, "a\r\nb\r\nc\r\n": 3, "": 0}
	for texto, quer := range casos {
		if got := linhasDoColar(texto); got != quer {
			t.Errorf("linhasDoColar(%q) = %d, quer %d", texto, got, quer)
		}
	}
}

// abaGestosDeTeste monta a aba e um quadro, para as células terem medida.
func abaGestosDeTeste(t *testing.T) (*sshTab, *bufFechavel, func(col, lin int) f32.Point) {
	t.Helper()
	tema = temaClaro
	th := material.NewTheme()
	th.Shaper = shaperDoApp()
	buf := &bufFechavel{}
	tab := &sshTab{th: th, cols: 80, rows: 24, corpo: 13, entrada: buf}
	tab.term = vt10x.New(vt10x.WithSize(80, 24))
	var r input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops: &ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(900, 500)), Source: r.Source(),
	}
	tab.Layout(gtx)
	r.Frame(gtx.Ops)
	tab.mu.Lock()
	margem, avanco, ch := tab.margem, tab.avanco, tab.alturaCel
	tab.mu.Unlock()
	em := func(col, lin int) f32.Point {
		return f32.Pt(float32(float64(margem)+avanco*(float64(col)+0.5)), float32(margem+ch*lin+ch/2))
	}
	return tab, buf, em
}

func TestDuploCliqueMarcaPalavra(t *testing.T) {
	tab, _, em := abaGestosDeTeste(t)
	tab.term.Write([]byte("erro em /var/log/pdv/agente.log linha 12"))
	clique := func() {
		tab.HandlePointer(pointer.Event{Kind: pointer.Press, Buttons: pointer.ButtonPrimary,
			Position: em(12, 0), Source: pointer.Mouse}, image.Pt(900, 500))
		tab.HandlePointer(pointer.Event{Kind: pointer.Release, Position: em(12, 0), Source: pointer.Mouse}, image.Pt(900, 500))
	}
	clique()
	clique()
	if got, ok := tab.textoSelecionado(); !ok || got != "/var/log/pdv/agente.log" {
		t.Fatalf("duplo clique marcou %q (ok=%v)", got, ok)
	}
	clique()
	if got, ok := tab.textoSelecionado(); !ok || got != "erro em /var/log/pdv/agente.log linha 12" {
		t.Fatalf("triplo clique marcou %q (ok=%v)", got, ok)
	}
}

func TestRodaEmTelaCheiaViraSeta(t *testing.T) {
	tab, buf, em := abaGestosDeTeste(t)
	tab.term.Write([]byte("\x1b[?1049h"))
	tab.HandlePointer(pointer.Event{Kind: pointer.Scroll, Scroll: f32.Pt(0, 1),
		Position: em(5, 5), Source: pointer.Mouse}, image.Pt(900, 500))
	if got, quer := buf.String(), strings.Repeat(setaBaixo, 3); got != quer {
		t.Fatalf("roda para baixo no nano mandou %q, esperado %q", got, quer)
	}
	// No shell (tela normal) a roda continua rolando o histórico.
	tab2, buf2, em2 := abaGestosDeTeste(t)
	tab2.HandlePointer(pointer.Event{Kind: pointer.Scroll, Scroll: f32.Pt(0, -1),
		Position: em2(5, 5), Source: pointer.Mouse}, image.Pt(900, 500))
	if buf2.Len() != 0 {
		t.Fatalf("no shell a roda não pode mandar nada: %q", buf2.String())
	}
}

// parSSH monta cliente e servidor SSH em memória. responde=false faz o
// servidor nunca responder pedido global (rede morta do ponto de vista do
// sinal de vida).
func parSSH(t *testing.T, responde bool) *ssh.Client {
	t.Helper()
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	// TCP local de verdade, e não net.Pipe: o aperto de mão do SSH escreve
	// dos dois lados antes de ler, e o cano sem buffer trava os dois.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	cfgS := &ssh.ServerConfig{NoClientAuth: true}
	cfgS.AddHostKey(signer)
	go func() {
		b, err := ln.Accept()
		if err != nil {
			return
		}
		_, chans, reqs, err := ssh.NewServerConn(b, cfgS)
		if err != nil {
			return
		}
		go func() {
			for nc := range chans {
				nc.Reject(ssh.Prohibited, "teste")
			}
		}()
		if responde {
			ssh.DiscardRequests(reqs)
			return
		}
		for r := range reqs {
			_ = r // nunca responde: o pedido fica sem resposta
		}
	}()
	a, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	cc, chans, reqs, err := ssh.NewClientConn(a, "teste", &ssh.ClientConfig{User: "x", HostKeyCallback: ssh.InsecureIgnoreHostKey()})
	if err != nil {
		t.Fatal(err)
	}
	return ssh.NewClient(cc, chans, reqs)
}

func TestSinalDeVida(t *testing.T) {
	salvoI, salvoE := intervaloVivo, esperaVivo
	intervaloVivo, esperaVivo = 20*time.Millisecond, 30*time.Millisecond
	defer func() { intervaloVivo, esperaVivo = salvoI, salvoE }()

	vivo := parSSH(t, true)
	go manterVivaSSH(vivo)
	fechou := make(chan struct{})
	go func() { vivo.Wait(); close(fechou) }()
	select {
	case <-fechou:
		t.Fatal("servidor que responde não pode ter a conexão fechada")
	case <-time.After(300 * time.Millisecond):
	}
	vivo.Close()

	mudo := parSSH(t, false)
	go manterVivaSSH(mudo)
	fechouMudo := make(chan struct{})
	go func() { mudo.Wait(); close(fechouMudo) }()
	select {
	case <-fechouMudo:
	case <-time.After(2 * time.Second):
		t.Fatal("três sinais sem resposta deviam fechar a conexão")
	}
}
