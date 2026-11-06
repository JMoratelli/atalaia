//go:build windows

// Command tecladotest confere, numa janela de verdade, o que o Gio do fork
// entrega ao app quando se digita no Windows: o key.Event (com o Scancode
// do patch de 2026-11-06) e o texto do WM_CHAR (key.EditEvent), que é de
// onde SSH e VNC tiram o caractere — ver cmd/acessos/teclado_outros.go.
//
// Digita sozinho, por SendInput com SCANCODE, para o resultado passar pelo
// layout ativo como passaria um teclado físico: Shift+1 tem de chegar como
// "!", a tecla do ";" com Shift como "Ç" no ABNT2, e a "/?" do ABNT2 e o
// Insert, que o Gio não nomeia, têm de chegar mesmo assim.
//
// SEGURANÇA: SendInput vai para a janela em PRIMEIRO PLANO, seja qual for.
// Antes de cada sequência a ferramenta confere que a janela em primeiro
// plano é a dela e, se não for, para sem digitar nada — senão as teclas
// cairiam no editor de quem rodou. Cada sequência vai num SendInput só,
// com o apertar e o soltar juntos, para nenhum Shift ficar preso.
//
// Sai 0 se tudo bateu, 1 se alguma sequência falhou, 2 se não conseguiu o
// primeiro plano (aí não testou nada). Nada disso aparece em go test: não
// há janela, nem layout, nem WM_CHAR.
package main

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"gioui.org/app"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/system"
	"gioui.org/op"
)

var (
	user32                   = syscall.NewLazyDLL("user32.dll")
	procSendInput            = user32.NewProc("SendInput")
	procGetForegroundWindow  = user32.NewProc("GetForegroundWindow")
	procGetWindowThreadPID   = user32.NewProc("GetWindowThreadProcessId")
	procGetKeyboardLayout    = user32.NewProc("GetKeyboardLayout")
	procSetForegroundWindow  = user32.NewProc("SetForegroundWindow")
	procAttachThreadInput    = user32.NewProc("AttachThreadInput")
	procGetCurrentThreadID   = syscall.NewLazyDLL("kernel32.dll").NewProc("GetCurrentThreadId")
	procBringWindowToTop     = user32.NewProc("BringWindowToTop")
	procGetAsyncKeyState     = user32.NewProc("GetAsyncKeyState")
	procGetKeyState          = user32.NewProc("GetKeyState")
	procGetKeyboardLayoutNam = user32.NewProc("GetKeyboardLayoutNameW")
)

const (
	inputKeyboard     = 1
	keyeventExtended  = 0x0001
	keyeventKeyup     = 0x0002
	keyeventScancode  = 0x0008
	vkShift, vkCtrl   = 0x10, 0x11
	vkMenu            = 0x12
	esperaPorSequenc  = 250 * time.Millisecond
	esperaPrimeiroPla = 4 * time.Second
)

// input é o INPUT do Win32 com o KEYBDINPUT no union: 40 bytes no x64.
type input struct {
	typ   uint32
	_     uint32
	vk    uint16
	scan  uint16
	flags uint32
	time  uint32
	_     uint32
	extra uintptr
	_     [8]byte
}

// tecla é uma tecla física: scancode do conjunto 1, com 0xE000 nas
// estendidas (a mesma convenção do key.Event.Scancode).
type tecla uint32

func (t tecla) evento(solta bool) input {
	in := input{typ: inputKeyboard, scan: uint16(t & 0xff), flags: keyeventScancode}
	if t&0xe000 == 0xe000 {
		in.flags |= keyeventExtended
	}
	if solta {
		in.flags |= keyeventKeyup
	}
	return in
}

// combinacao: segura as primeiras e bate a última; solta na ordem inversa.
func combinacao(ts ...tecla) []input {
	var out []input
	for _, t := range ts {
		out = append(out, t.evento(false))
	}
	for i := len(ts) - 1; i >= 0; i-- {
		out = append(out, ts[i].evento(true))
	}
	return out
}

func emSequencia(grupos ...[]input) []input {
	var out []input
	for _, g := range grupos {
		out = append(out, g...)
	}
	return out
}

const (
	scShift  tecla = 0x2a
	scCtrlD  tecla = 0xe01d
	scAltGr  tecla = 0xe038
	sc1      tecla = 0x02
	sc2      tecla = 0x03
	scPontoV tecla = 0x27 // ";" no US, "ç" no ABNT2
	scIgual  tecla = 0x0d
	scKPMais tecla = 0x4e
	scKPVezs tecla = 0x37
	scInsert tecla = 0xe052
	scABNTC1 tecla = 0x73 // "/?" do ABNT2
	scOEM102 tecla = 0x56 // ao lado do Z
	scAgudo  tecla = 0x1a // "´" (morta) no ABNT2, "[" no US
	scA      tecla = 0x1e
	scQ      tecla = 0x10
	scSetaE  tecla = 0xe04b
	// O Num Lock do teclado é 0x45 PURO (o E1 1D 45 é o Pause): manda-se
	// scNumLk, e o Gio do fork o entrega como scNumLkWin — o teclado físico
	// o marca como estendido e o SendInput não, e o fork iguala os dois pela
	// tecla virtual (scancodeDoLParam). Mandar 0xE045 não liga nem desliga
	// nada.
	scNumLk    tecla = 0x45
	scNumLkWin       = 0xe045
	vkNumLk          = 0x90
)

// esperado: um key.Event com este nome e scancode (State Press) e, se
// texto != "", o EditEvent com esse texto.
type esperado struct {
	nome  key.Name
	sc    uint32
	texto string
}

type sequencia struct {
	desc   string
	inputs []input
	quer   []esperado
	soABNT bool
}

func sequencias() []sequencia {
	return []sequencia{
		{"Shift+1 vira \"!\"", combinacao(scShift, sc1),
			[]esperado{{"1", 0x02, "!"}}, false},
		{"Shift+2 vira \"@\"", combinacao(scShift, sc2),
			[]esperado{{"2", 0x03, "@"}}, false},
		{"\"=\" (o Gio chama de \"+\") com scancode 0x0D", combinacao(scIgual),
			[]esperado{{"+", 0x0d, "="}}, false},
		{"\"+\" do numérico com scancode 0x4E", combinacao(scKPMais),
			[]esperado{{"+", 0x4e, "+"}}, false},
		{"\"*\" do numérico com scancode 0x37", combinacao(scKPVezs),
			[]esperado{{"*", 0x37, "*"}}, false},
		{"Insert (sem nome no Gio) chega com scancode", combinacao(scInsert),
			[]esperado{{"", 0xe052, ""}}, false},
		{"Ctrl direito com scancode estendido", combinacao(scCtrlD),
			[]esperado{{key.NameCtrl, 0xe01d, ""}}, false},
		{"seta com scancode estendido", combinacao(scSetaE),
			[]esperado{{key.NameLeftArrow, 0xe04b, ""}}, false},
		{"ABNT2: Shift+ç vira \"Ç\"", combinacao(scShift, scPontoV),
			[]esperado{{";", 0x27, "Ç"}}, true},
		{"ABNT2: \"/?\" (sem nome no Gio) chega e escreve \"/\"", combinacao(scABNTC1),
			[]esperado{{"", 0x73, "/"}}, true},
		{"ABNT2: tecla ao lado do Z escreve \"\\\"", combinacao(scOEM102),
			[]esperado{{"\\", 0x56, "\\"}}, true},
		{"ABNT2: acento morto ´ + a vira \"á\"", emSequencia(combinacao(scAgudo), combinacao(scA)),
			[]esperado{{"A", 0x1e, "á"}}, true},
		{"ABNT2: AltGr+Q vira \"/\"", combinacao(scAltGr, scQ),
			[]esperado{{"Q", 0x10, "/"}}, true},
	}
}

// registro guarda o que a janela recebeu.
type registro struct {
	mu     sync.Mutex
	teclas []key.Event
	textos []string

	// travas são pedidos de leitura do Num Lock, atendidos DENTRO de um
	// quadro — ver numLockNaJanela.
	travas chan chan bool
}

func (r *registro) zerar() {
	r.mu.Lock()
	r.teclas, r.textos = nil, nil
	r.mu.Unlock()
}

func (r *registro) copia() ([]key.Event, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]key.Event(nil), r.teclas...), strings.Join(r.textos, "")
}

func main() {
	w := new(app.Window)
	w.Option(app.Title("tecladotest"), app.Size(480, 160))
	reg := &registro{travas: make(chan chan bool, 1)}
	hwndC := make(chan uintptr, 1)
	go func() {
		os.Exit(dirigir(w, reg, hwndC))
	}()
	go laco(w, reg, hwndC)
	app.Main()
}

// laco é o laço da janela: pede o foco para um alvo e grava tudo o que
// chega — o mesmo par de filtros de cmd/acessos/entrada_outros.go.
func laco(w *app.Window, reg *registro, hwndC chan<- uintptr) {
	var ops op.Ops
	alvo := new(int)
	const qualquer = key.ModCtrl | key.ModShift | key.ModAlt | key.ModSuper | key.ModCommand
	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return
		case app.Win32ViewEvent:
			if e.Valid() {
				select {
				case hwndC <- e.HWND:
				default:
				}
			}
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			event.Op(gtx.Ops, alvo)
			gtx.Execute(key.FocusCmd{Tag: alvo})
			for {
				ev, ok := gtx.Event(
					key.FocusFilter{Target: alvo},
					key.Filter{Focus: alvo, Name: "", Optional: qualquer},
				)
				if !ok {
					break
				}
				reg.mu.Lock()
				switch ev := ev.(type) {
				case key.Event:
					reg.teclas = append(reg.teclas, ev)
				case key.EditEvent:
					reg.textos = append(reg.textos, ev.Text)
				}
				reg.mu.Unlock()
			}
			select {
			case resp := <-reg.travas:
				resp <- lerNumLock(w)
			default:
			}
			e.Frame(gtx.Ops)
		}
	}
}

func dirigir(w *app.Window, reg *registro, hwndC <-chan uintptr) int {
	var hwnd uintptr
	select {
	case hwnd = <-hwndC:
	case <-time.After(5 * time.Second):
		fmt.Println("a janela não abriu")
		return 2
	}
	if !trazerParaFrente(w, hwnd) {
		fmt.Println("não consegui o primeiro plano — nada foi digitado")
		return 2
	}
	tid, _, _ := procGetWindowThreadPID.Call(hwnd, 0)
	hkl, _, _ := procGetKeyboardLayout.Call(tid)
	abnt := hkl&0xffff == 0x0416
	fmt.Printf("layout da janela: HKL %#x (ABNT2/pt-BR: %v), %s\n", hkl, abnt, nomeLayout())
	time.Sleep(300 * time.Millisecond)

	falhas := 0
	for _, s := range sequencias() {
		if s.soABNT && !abnt {
			fmt.Printf("PULA  %s (layout não é pt-BR)\n", s.desc)
			continue
		}
		if fg, _, _ := procGetForegroundWindow.Call(); fg != hwnd {
			fmt.Println("PERDEU O PRIMEIRO PLANO — parando sem digitar mais nada")
			return 2
		}
		reg.zerar()
		if n, _, err := procSendInput.Call(uintptr(len(s.inputs)),
			uintptr(unsafe.Pointer(&s.inputs[0])), unsafe.Sizeof(s.inputs[0])); int(n) != len(s.inputs) {
			fmt.Printf("FALHA %s: SendInput entregou %d de %d (%v)\n", s.desc, n, len(s.inputs), err)
			falhas++
			continue
		}
		time.Sleep(esperaPorSequenc)
		teclas, texto := reg.copia()
		if msg := conferir(s.quer, teclas, texto); msg != "" {
			fmt.Printf("FALHA %s: %s\n", s.desc, msg)
			falhas++
		} else {
			fmt.Printf("ok    %s\n", s.desc)
		}
		fmt.Printf("      recebido: %s | texto %q\n", resumo(teclas), texto)
	}
	falhas += testarNumLock(w, hwnd, reg)
	if presa := modificadorPreso(); presa != "" {
		fmt.Printf("ATENÇÃO: %s ficou apertado no sistema\n", presa)
		falhas++
	}
	if falhas > 0 {
		fmt.Printf("%d falha(s)\n", falhas)
		return 1
	}
	fmt.Println("tudo certo")
	return 0
}

// testarNumLock confere a leitura das travas do app (lerTravasLocais, em
// cmd/acessos/travas_windows.go): GetKeyState NA THREAD DA JANELA, por
// w.Run. Duas situações:
//
//  1. Num Lock apertado na própria janela: a leitura muda, e a tecla chega
//     com o scancode que o app usa para saber que é hora de reler.
//  2. Num Lock apertado com OUTRA janela na frente (uma segunda janela
//     daqui mesmo, que tem thread e fila de entrada próprias — faz o papel
//     de outro programa sem digitar em programa de ninguém): de volta à
//     primeira, a leitura tem de mostrar o estado novo. É o caso do
//     operador que mexeu no Num Lock fora do app.
//
// As duas trocas se desfazem: o Num Lock termina como começou.
func testarNumLock(w *app.Window, hwnd uintptr, reg *registro) (falhas int) {
	inicio, ok := numLockNaJanela(w, reg)
	if !ok {
		fmt.Println("FALHA ler as travas de dentro do quadro TRAVOU a janela")
		return 1
	}
	fmt.Printf("Num Lock no início: %v (lido de dentro do quadro, como o app)\n", inicio)
	travou := func() int {
		fmt.Println("FALHA ler as travas de dentro do quadro TRAVOU a janela")
		return falhas + 1
	}
	defer func() {
		// se algo parou no meio, devolve o Num Lock como estava — só com a
		// janela daqui na frente
		if v, ok := numLockNaJanela(w, reg); ok && v != inicio && trazerParaFrente(w, hwnd) {
			enviar(hwnd, combinacao(scNumLk))
			time.Sleep(esperaPorSequenc)
		}
	}()

	reg.zerar()
	if !enviar(hwnd, combinacao(scNumLk)) {
		fmt.Println("PERDEU O PRIMEIRO PLANO — Num Lock não testado")
		return 1
	}
	time.Sleep(esperaPorSequenc)
	teclas, _ := reg.copia()
	if msg := conferir([]esperado{{"", scNumLkWin, ""}}, teclas, ""); msg != "" {
		fmt.Printf("FALHA Num Lock chega com o scancode: %s\n", msg)
		falhas++
	} else {
		fmt.Println("ok    Num Lock chega com o scancode 0xe045")
	}
	fmt.Printf("      recebido: %s\n", resumo(teclas))
	if v, ok := numLockNaJanela(w, reg); !ok {
		return travou()
	} else if v == inicio {
		fmt.Println("FALHA Num Lock apertado na janela: a leitura não mudou")
		falhas++
	} else {
		fmt.Println("ok    Num Lock apertado na janela: a leitura mudou")
	}

	w2 := new(app.Window)
	w2.Option(app.Title("tecladotest — outra janela"), app.Size(320, 120))
	hwnd2C := make(chan uintptr, 1)
	go laco(w2, &registro{}, hwnd2C)
	defer w2.Perform(system.ActionClose)
	var hwnd2 uintptr
	select {
	case hwnd2 = <-hwnd2C:
	case <-time.After(5 * time.Second):
		fmt.Println("FALHA a segunda janela não abriu")
		return falhas + 1
	}
	if !trazerParaFrente(w2, hwnd2) || !enviar(hwnd2, combinacao(scNumLk)) {
		fmt.Println("FALHA não consegui apertar Num Lock na outra janela")
		return falhas + 1
	}
	time.Sleep(esperaPorSequenc)
	if !trazerParaFrente(w, hwnd) {
		fmt.Println("FALHA não consegui voltar para a primeira janela")
		return falhas + 1
	}
	time.Sleep(esperaPorSequenc)
	if got, ok := numLockNaJanela(w, reg); !ok {
		return travou()
	} else if got != inicio {
		fmt.Printf("FALHA Num Lock mudado em outra janela: de volta, a leitura diz %v, o certo é %v\n", got, inicio)
		falhas++
	} else {
		fmt.Println("ok    Num Lock mudado em outra janela: de volta, a leitura acompanha")
	}
	return falhas
}

// numLockNaJanela pede a leitura ao laço da janela, que a faz DENTRO de um
// quadro — o mesmo lugar de onde o app lê (tratarTecladoFrame). ok=false é
// a leitura que não voltou: w.Run de dentro do quadro travou a janela.
func numLockNaJanela(w *app.Window, reg *registro) (ligado, ok bool) {
	resp := make(chan bool, 1)
	reg.travas <- resp
	w.Invalidate()
	select {
	case v := <-resp:
		return v, true
	case <-time.After(3 * time.Second):
		return false, false
	}
}

// lerNumLock é a leitura de cmd/acessos/travas_windows.go: GetKeyState na
// thread da janela, por w.Run.
func lerNumLock(w *app.Window) bool {
	var ligado bool
	w.Run(func() {
		r, _, _ := procGetKeyState.Call(vkNumLk)
		ligado = r&1 != 0
	})
	return ligado
}

// enviar digita ins só se a janela da frente for hwnd.
func enviar(hwnd uintptr, ins []input) bool {
	if fg, _, _ := procGetForegroundWindow.Call(); fg != hwnd {
		return false
	}
	n, _, _ := procSendInput.Call(uintptr(len(ins)),
		uintptr(unsafe.Pointer(&ins[0])), unsafe.Sizeof(ins[0]))
	return int(n) == len(ins)
}

func conferir(quer []esperado, teclas []key.Event, texto string) string {
	var textoQuer string
	for _, q := range quer {
		achou := false
		for _, k := range teclas {
			if k.State == key.Press && k.Name == q.nome && k.Scancode == q.sc {
				achou = true
				break
			}
		}
		if !achou {
			return fmt.Sprintf("faltou key.Event{Name:%q Scancode:%#x}", q.nome, q.sc)
		}
		textoQuer += q.texto
	}
	if texto != textoQuer {
		return fmt.Sprintf("texto %q, queria %q", texto, textoQuer)
	}
	return ""
}

func resumo(ts []key.Event) string {
	var b strings.Builder
	for i, k := range ts {
		if i > 0 {
			b.WriteString(" ")
		}
		v := "↓"
		if k.State == key.Release {
			v = "↑"
		}
		fmt.Fprintf(&b, "%s%q/%#x[%v]", v, k.Name, k.Scancode, k.Modifiers)
	}
	return b.String()
}

// trazerParaFrente pede o primeiro plano pelo Gio e, se o Windows recusar
// (ele só cede o primeiro plano a quem recebeu a última entrada), tenta
// de novo ligando a fila de entrada à da janela que está na frente.
func trazerParaFrente(w *app.Window, hwnd uintptr) bool {
	w.Perform(system.ActionRaise)
	prazo := time.Now().Add(esperaPrimeiroPla)
	tentouAnexar := false
	for time.Now().Before(prazo) {
		fg, _, _ := procGetForegroundWindow.Call()
		if fg == hwnd {
			return true
		}
		if !tentouAnexar && time.Until(prazo) < esperaPrimeiroPla/2 {
			tentouAnexar = true
			fgTid, _, _ := procGetWindowThreadPID.Call(fg, 0)
			eu, _, _ := procGetCurrentThreadID.Call()
			procAttachThreadInput.Call(eu, fgTid, 1)
			procBringWindowToTop.Call(hwnd)
			procSetForegroundWindow.Call(hwnd)
			procAttachThreadInput.Call(eu, fgTid, 0)
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

func modificadorPreso() string {
	for vk, nome := range map[uintptr]string{vkShift: "Shift", vkCtrl: "Ctrl", vkMenu: "Alt"} {
		if s, _, _ := procGetAsyncKeyState.Call(vk); int16(s) < 0 {
			return nome
		}
	}
	return ""
}

func nomeLayout() string {
	var buf [9]uint16
	procGetKeyboardLayoutNam.Call(uintptr(unsafe.Pointer(&buf[0])))
	return "KLID " + syscall.UTF16ToString(buf[:])
}
