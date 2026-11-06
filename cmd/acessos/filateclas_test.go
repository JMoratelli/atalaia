package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Testes de ESTRESSE da fila de teclas. Rode sempre com -race:
//
//	go test ./cmd/acessos/ -run 'Tecla|Ordem|Release' -race -count=3
//
// O que eles protegem está escrito por extenso em filateclas.go. Em uma
// linha: quem recebe a tecla roda na goroutine do laço de eventos (o
// callback do grab no Linux, o quadro no Windows), então escrever no
// socket de lá congela a interface inteira.
//
// esperarAte vive em acaojanela_test.go. Este arquivo era só do Linux até
// a fila passar a valer também no Windows (2026-11-06).

// abaLenta é uma aba cujo HandleKey demora — simula o worker engasgado, que
// é o estado em que o bug aparecia.
type abaLenta struct {
	abaFalsa
	atraso   time.Duration
	mu       sync.Mutex
	recebido []teclaPendente

	// segura a PRIMEIRA entrega até o teste liberar, para montar o estado
	// de fila cheia antes de qualquer coisa sair.
	liberar chan struct{}
	uma     sync.Once
}

func (a *abaLenta) HandleKey(ks, kc uint32, p bool) {
	if a.liberar != nil {
		a.uma.Do(func() { <-a.liberar })
	}
	if a.atraso > 0 {
		time.Sleep(a.atraso)
	}
	a.mu.Lock()
	a.recebido = append(a.recebido, teclaPendente{keysym: ks, keycode: kc, pressed: p})
	a.mu.Unlock()
}

func (a *abaLenta) contarRelease() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	n := 0
	for _, k := range a.recebido {
		if !k.pressed {
			n++
		}
	}
	return n
}

// TestTeclaNaoBloqueiaChamador é O teste do bug: com a sessão travada,
// enfileirar NÃO pode segurar quem chamou. Antes da correção cada tecla
// custava até prazoEscrita (5s) na goroutine do laço de eventos.
func TestTeclaNaoBloqueiaChamador(t *testing.T) {
	f := novaFilaTeclas(512)
	go f.rodar()
	aba := &abaLenta{atraso: 2 * time.Second}

	inicio := time.Now()
	for i := 0; i < 100; i++ {
		f.enfileirar(aba, uint32(i), uint32(i), true)
	}
	if d := time.Since(inicio); d > 100*time.Millisecond {
		t.Fatalf("enfileirar bloqueou o chamador por %v — é isso que congela a interface", d)
	}
}

// TestOrdemPreservada: "soltou" antes de "apertou" prende a tecla do lado
// remoto. A fila tem que ser FIFO estrita.
func TestOrdemPreservada(t *testing.T) {
	f := novaFilaTeclas(512)
	go f.rodar()
	aba := &abaLenta{}

	const n = 400
	for i := 0; i < n; i++ {
		f.enfileirar(aba, uint32(i), uint32(i), i%2 == 0)
	}
	esperarAte(t, func() bool {
		aba.mu.Lock()
		defer aba.mu.Unlock()
		return len(aba.recebido) == n
	})

	aba.mu.Lock()
	defer aba.mu.Unlock()
	for i, k := range aba.recebido {
		if k.keysym != uint32(i) || k.pressed != (i%2 == 0) {
			t.Fatalf("fora de ordem na posição %d: %+v", i, k)
		}
	}
}

// TestReleaseNaoUltrapassaPress é o teste de REGRESSÃO do defeito que a
// revisão pegou, e o motivo de a fila ser uma fatia e não um canal.
//
// A versão anterior guardava o "soltou" que não coubesse numa lista à parte,
// drenada logo após CADA entrega. Com o "apertou" da mesma tecla parado no
// meio da fila, o "soltou" saía na frente dele — e a tecla ficava PRESA do
// lado remoto. Reproduzia assim, exatamente:
//
//	entregue: q↓ | A↑ | w↓ | A↓ | e↓      ← A soltou antes de apertar
//
// Se alguém voltar a criar um caminho paralelo à fila, este teste quebra.
func TestReleaseNaoUltrapassaPress(t *testing.T) {
	const teclaA = 0x0061
	f := novaFilaTeclas(4)
	aba := &abaLenta{liberar: make(chan struct{})}
	go f.rodar()

	// enche a fila inteira, com o press de A no MEIO
	f.enfileirar(aba, 0x0071, 100, true)  // q ↓
	f.enfileirar(aba, 0x0077, 101, true)  // w ↓
	f.enfileirar(aba, teclaA, 102, true)  // A ↓  <- o que não pode ser ultrapassado
	f.enfileirar(aba, 0x0065, 103, true)  // e ↓  (lotou)
	f.enfileirar(aba, teclaA, 102, false) // A ↑  <- entra com a fila cheia

	close(aba.liberar)

	esperarAte(t, func() bool {
		aba.mu.Lock()
		defer aba.mu.Unlock()
		p, r := false, false
		for _, k := range aba.recebido {
			if k.keysym == teclaA && k.pressed {
				p = true
			}
			if k.keysym == teclaA && !k.pressed {
				r = true
			}
		}
		return p && r
	})

	aba.mu.Lock()
	defer aba.mu.Unlock()
	posPress, posRelease := -1, -1
	for i, k := range aba.recebido {
		if k.keysym != teclaA {
			continue
		}
		if k.pressed && posPress < 0 {
			posPress = i
		}
		if !k.pressed && posRelease < 0 {
			posRelease = i
		}
	}
	if posRelease < posPress {
		t.Fatalf("release de A (pos %d) entregue ANTES do press (pos %d) — tecla presa no remoto;"+
			" ordem completa: %+v", posRelease, posPress, aba.recebido)
	}
}

// TestReleaseNuncaSePerde: com a fila estourada, "apertou" pode cair, mas
// TODO "soltou" tem que chegar — release perdido é modificador preso.
//
// Este é o teste que reprovou a primeira tentativa de correção (segurar o
// chamador por 50ms), que entregava ~10% dos releases. Não troque o
// mecanismo por um timeout de novo.
func TestReleaseNuncaSePerde(t *testing.T) {
	f := novaFilaTeclas(4) // minúscula de propósito: estoura fácil
	aba := &abaLenta{atraso: 5 * time.Millisecond}
	go f.rodar()

	var releases atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			pressed := i%2 == 0
			if !pressed {
				releases.Add(1)
			}
			f.enfileirar(aba, uint32(i), uint32(i), pressed)
		}(i)
	}
	wg.Wait()

	esperarAte(t, func() bool { return aba.contarRelease() == int(releases.Load()) })
	if got := aba.contarRelease(); got != int(releases.Load()) {
		t.Fatalf("faltaram %d release(s) — tecla fica presa na sessão",
			int(releases.Load())-got)
	}
	t.Logf("%d releases entregues, %d apertou(s) sacrificados pela fila cheia",
		aba.contarRelease(), f.descartadas.Load())
}

// TestTeclasDeVariasThreads: o listener do wl_keyboard e a thread rep_loop
// (repetição) enfileiram ao mesmo tempo. Só faz sentido com -race.
func TestTeclasDeVariasThreads(t *testing.T) {
	f := novaFilaTeclas(512)
	go f.rodar()
	aba := &abaLenta{}

	var wg sync.WaitGroup
	for th := 0; th < 8; th++ {
		wg.Add(1)
		go func(th int) {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				f.enfileirar(aba, uint32(th), uint32(i), i%3 != 0)
			}
		}(th)
	}
	wg.Wait()
}

// abaTextoFila grava teclas e textos na ordem em que a fila os entrega.
type abaTextoFila struct {
	abaDeTeste
	mu  sync.Mutex
	log []string
	fim chan struct{}
}

func (a *abaTextoFila) HandleKey(ks, kc uint32, p bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if p {
		a.log = append(a.log, "aperta")
	} else {
		a.log = append(a.log, "solta")
	}
}

func (a *abaTextoFila) HandleTexto(s string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.log = append(a.log, s)
	if s == "fim" {
		close(a.fim)
	}
}

// HandleTravas grava o estado das travas na mesma linha do tempo.
func (a *abaTextoFila) ConexaoTravas() any { return a }

func (a *abaTextoFila) HandleTravas(travas uint32) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.log = append(a.log, fmt.Sprintf("travas=%d", travas))
}

// O estado das travas vai DEPOIS do Num Lock que acabou de ir como tecla:
// na ordem inversa, o servidor ficaria com o estado novo e em seguida a
// tecla o inverteria de volta.
func TestTravasNaOrdemDasTeclas(t *testing.T) {
	f := novaFilaTeclas(64)
	go f.rodar()
	aba := &abaTextoFila{fim: make(chan struct{})}

	f.enfileirar(aba, 0xff7f, 77, true)
	f.enfileirarTravas(aba, travaNum)
	f.enfileirar(aba, 0xff7f, 77, false)
	f.enfileirarTexto(aba, "fim")

	select {
	case <-aba.fim:
	case <-time.After(2 * time.Second):
		t.Fatal("a fila não entregou")
	}
	aba.mu.Lock()
	defer aba.mu.Unlock()
	quer := []string{"aperta", "travas=2", "solta", "fim"}
	if fmt.Sprint(aba.log) != fmt.Sprint(quer) {
		t.Fatalf("entregue %q, queria %q", aba.log, quer)
	}
}

// Com a fila estourada, o estado das travas não cai: é estado, e perdê-lo
// deixa a sessão trocada como um "soltou" perdido deixa a tecla presa.
func TestTravasNaoCaemComFilaCheia(t *testing.T) {
	f := novaFilaTeclas(2) // sem goroutine de escrita: nada sai
	aba := &abaTextoFila{}
	f.enfileirarTravas(aba, travaNum)
	for i := 0; i < 10; i++ {
		f.enfileirar(aba, uint32(i), uint32(i), true)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.fila) == 0 || !f.fila[0].ehTravas {
		t.Fatalf("o estado das travas foi descartado; fila: %+v", f.fila)
	}
}

// O texto vai na MESMA fila das teclas, e na ordem: Shift apertado, "!",
// Shift solto. Se o texto passasse na frente do Shift, o VNC receberia o
// "!" com o Shift ainda solto do lado de lá.
func TestTextoNaOrdemDasTeclas(t *testing.T) {
	f := novaFilaTeclas(64)
	go f.rodar()
	aba := &abaTextoFila{fim: make(chan struct{})}

	f.enfileirar(aba, 0xffe1, 50, true)
	f.enfileirarTexto(aba, "!")
	f.enfileirar(aba, 0xffe1, 50, false)
	f.enfileirarTexto(aba, "")
	f.enfileirarTexto(aba, "fim")

	select {
	case <-aba.fim:
	case <-time.After(2 * time.Second):
		t.Fatal("a fila não entregou")
	}
	aba.mu.Lock()
	defer aba.mu.Unlock()
	quer := []string{"aperta", "!", "solta", "fim"}
	if len(aba.log) != len(quer) {
		t.Fatalf("entregue %q, queria %q", aba.log, quer)
	}
	for i := range quer {
		if aba.log[i] != quer[i] {
			t.Fatalf("entregue %q, queria %q", aba.log, quer)
		}
	}
}
