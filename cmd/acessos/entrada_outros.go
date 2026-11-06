//go:build !linux

package main

import (
	"io"
	"sort"
	"strings"

	"gioui.org/app"
	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/transfer"
	"gioui.org/layout"
)

// Fora do Linux não há grab por Wayland. No Windows o teclado e a área de
// transferência vêm pelo próprio Gio (ver os_windows.go): key.Event já traz
// Modifiers confiável (diferente do Wayland) e clipboard.ReadCmd/WriteCmd já
// resolvem sozinhos contra a API do Windows. tratarTecladoFrame e
// tratarClipboardFrame são a ponte que faltava — ver BACKLOG.md, itens 1 e 2.
func tratarEventoPlataforma(est *estadoJanela, w *app.Window, e event.Event, activeTab func() Tab) {
}

// tagTecladoOutros é o alvo do foco de teclado que reivindicamos quando a
// aba ativa é uma sessão remota — ver tratarTecladoFrame. Um só para todas
// as janelas: cada janela tem o próprio roteador do Gio, e o alvo só
// existe dentro dele.
var tagTecladoOutros = new(int)

// devolverTecladoASessao é o que o clique na área de uma sessão remota
// chama (conteudoaba.go): tira o teclado de um campo do app que estivesse
// com ele (a busca da lateral, por exemplo) e o devolve à sessão. Aqui é
// pedir o foco direto para o alvo da sessão — pedir "foco em ninguém"
// faria o alvo perder o foco por um quadro, e perder o foco solta as
// teclas apertadas do lado remoto (ver estadoTeclado), o que quebraria o
// Shift+clique e o Ctrl+clique dentro da sessão.
func devolverTecladoASessao(gtx layout.Context) {
	gtx.Execute(key.FocusCmd{Tag: tagTecladoOutros})
}

// tratarTecladoFrame roda uma vez por quadro (chamada do main.go e da
// janela destacada, com o gtx já criado) e faz o que o grab faz no Linux:
// decide se a aba ativa quer o teclado, reivindica o foco quando quer e
// entrega cada tecla à aba — pela fila (filateclas.go), nunca direto,
// porque a entrega escreve no socket do processo-filho e pode bloquear.
//
// O foco só é reivindicado quando a aba ativa É uma sessão remota e não há
// diálogo nem campo de texto do app com o teclado: diferente do Linux,
// aqui pedimos o foco do PRÓPRIO Gio (key.FocusCmd), que outros widgets
// também disputam.
//
// Os atalhos do app (F12, Ctrl+W, Ctrl+G) são filtros SEM Focus: valem com
// o foco em qualquer lugar, inclusive em lugar nenhum, que é o caso do
// Painel. Antes eles só eram lidos pelo filtro do alvo da sessão, ou seja,
// só existiam com uma sessão aberta — e Ctrl+W e Ctrl+G, que só valem FORA
// de uma sessão, não funcionavam nunca no Windows.
func tratarTecladoFrame(est *estadoJanela, gtx layout.Context, activeTab func() Tab) {
	iniciarFilaTeclas()
	kb := &est.teclado

	// event.Op é o que faz o roteador do Gio criar (e manter "visível")
	// a entrada de tagTecladoOutros em handlers — sem isto, o foco pedido
	// logo abaixo (key.FocusCmd) é de um alvo que o roteador não conhece,
	// e keyQueue.Frame() zera de volta ao fim do quadro.
	event.Op(gtx.Ops, tagTecladoOutros)

	t := activeTab()
	remoto := t != nil && querTeclado(t)
	// focoEmCampo é o do quadro ANTERIOR: main.go só o zera depois desta
	// chamada. Zerado antes, como era, ele dizia sempre "não", e com uma
	// sessão aberta o campo de busca da lateral perdia o foco a cada
	// quadro — não dava para digitar nele.
	campo := temDialogo() || focoEmCampo.Load()
	podeTeclado := remoto && !campo
	if podeTeclado {
		gtx.Execute(key.FocusCmd{Tag: tagTecladoOutros})
		kb.soltarForaDe(t)
	} else {
		// trocou para o Painel, abriu um diálogo ou foi para um campo com
		// alguma tecla segurada: solta do lado remoto.
		kb.soltarTudo()
	}

	for {
		// key.FocusFilter PRECISA ser registrado toda vez: é o que marca
		// tagTecladoOutros como "focusable" pro Gio — sem ele, o próprio
		// keyQueue.Frame() (rodado ao fim deste quadro) acha que o foco
		// que acabamos de pedir com FocusCmd é de um alvo inexistente e
		// zera de volta, e nenhuma tecla chega no quadro seguinte. Ele
		// também é o que traz o key.EditEvent (o texto) e o FocusEvent.
		// Optional precisa listar TODOS os modificadores: um key.Filter
		// sem Required/Optional só combina com teclas SEM NENHUM
		// modificador (ver keyFilterMatch em io/input/key.go).
		const qualquerModificador = key.ModCtrl | key.ModShift | key.ModAlt | key.ModSuper | key.ModCommand
		e, ok := gtx.Event(
			key.FocusFilter{Target: tagTecladoOutros},
			key.Filter{Focus: tagTecladoOutros, Name: "", Optional: qualquerModificador},
			key.Filter{Name: key.NameF12},
			key.Filter{Name: "W", Required: key.ModCtrl},
			key.Filter{Name: "G", Required: key.ModCtrl},
		)
		if !ok {
			break
		}
		switch e := e.(type) {
		case key.FocusEvent:
			// A janela perdeu o foco (Alt+Tab, o atalho global abrindo
			// a busca) ou o foco foi para um campo: o "soltou" das teclas
			// seguradas vai para quem ganhou o foco, e do lado remoto
			// elas ficariam apertadas para sempre — o Ctrl ou o Shift
			// preso que transforma tudo o que se digita depois. É o
			// mesmo que o teclado_leave faz no Linux (grab_wayland.c).
			if !e.Focus {
				kb.soltarTudo()
				ctrlDown.Store(false)
			}
			// Ganhou ou perdeu, as travas são lidas de novo: fora da
			// janela o operador pode ter mexido nelas.
			kb.comFoco = e.Focus
			kb.travasLidas = false
		case key.EditEvent:
			if podeTeclado {
				kb.textoDe(t, e.Text)
			}
		case key.Event:
			if e.Name == key.NameCtrl {
				ctrlDown.Store(e.State == key.Press)
			}
			if teclaDeTrava(e) {
				kb.travasLidas = false
			}
			if atalhoDoApp(est, e, remoto, campo) {
				continue
			}
			if podeTeclado {
				kb.teclaDe(t, e)
			}
		}
	}

	// Depois das teclas deste quadro, para o estado sair na fila atrás do
	// Num Lock que acabou de ir como tecla — ver travas.go.
	var alvo Tab
	if podeTeclado && kb.comFoco {
		alvo = t
	} else {
		kb.travasLidas = false
	}
	kb.sinc.atualizar(alvo, func() uint32 {
		if !kb.travasLidas {
			kb.travas = lerTravasLocais(est.w)
			kb.travasLidas = true
		}
		return kb.travas
	}, kb.entregarTravas)
}

// teclaDeTrava: Caps Lock, Num Lock e Scroll Lock, pelo scancode. O Num
// Lock vem marcado como estendido e o Scroll Lock não, mas aceitar as duas
// formas não custa nada e não depende desse detalhe.
func teclaDeTrava(ke key.Event) bool {
	switch ke.Scancode &^ 0xe000 {
	case 0x3a, 0x45, 0x46:
		return true
	}
	return false
}

// atalhoDoApp trata F12, Ctrl+W e Ctrl+G e diz se a tecla era um deles.
// Mesmas regras do grab do Linux (entrada_linux.go):
//
//   - Só na janela principal: na destacada eles mexeriam no estado da
//     outra janela, e F12 é tecla de trabalho numa sessão Windows.
//   - F12 LIMPO, sem modificador nenhum: o atalho global do sistema é
//     Ctrl+Shift+F12.
//   - Ctrl+W/Ctrl+G só fora de uma sessão e fora de campo de texto: dentro
//     de uma sessão são do programa rodando lá (apagar palavra e abortar no
//     readline, Where Is e Get Help no nano).
func atalhoDoApp(est *estadoJanela, ke key.Event, remoto, campo bool) bool {
	if !est.principal || ke.State != key.Press {
		return false
	}
	if ke.Name == key.NameF12 && ke.Modifiers == 0 {
		pendingToggleSidebar.Store(true)
		est.w.Invalidate()
		return true
	}
	if remoto || campo || ke.Modifiers != key.ModCtrl {
		return false
	}
	switch ke.Name {
	case "W":
		pendingCloseActive.Store(true)
		est.w.Invalidate()
		return true
	case "G":
		reguaLigada = !reguaLigada
		est.w.Invalidate()
		return true
	}
	return false
}

// estadoTeclado é o roteador de teclas de UMA janela. Só o laço de quadro
// dela mexe aqui, daí não precisar de trava.
type estadoTeclado struct {
	// apertadas é o que foi mandado a uma sessão como "apertou" e ainda
	// não teve o "soltou", com a aba que recebeu e os códigos usados. O
	// soltar sai com os MESMOS códigos do apertar: no VNC o keysym da
	// letra muda com o Shift, e soltar "a" depois de apertar "A" deixa o
	// "A" preso no servidor.
	apertadas map[idTecla]teclaApertada
	// mods são os modificadores do último key.Event. O EditEvent não traz
	// modificador, e o WM_CHAR chega logo depois do WM_KEYDOWN da mesma
	// tecla — então estes são os dele.
	mods key.Modifiers

	// comFoco: o alvo da sessão tem o foco de teclado e a janela também —
	// o último key.FocusEvent dele. travas é o estado lido por
	// lerTravasLocais, válido enquanto travasLidas; sinc lembra o que já
	// foi para a sessão. Ver travas.go.
	comFoco     bool
	travas      uint32
	travasLidas bool
	sinc        sincTravas

	// entrega; nil é a fila de produção (filateclas.go). Os testes trocam.
	tecla      func(t Tab, keysym, keycode uint32, pressed bool)
	texto      func(t Tab, s string)
	travasPara func(t Tab, travas uint32)
}

// idTecla identifica a tecla FÍSICA: o scancode quando há, o nome quando
// não. Não pode ser o keysym (muda com o Shift) nem só o nome (o "+" do
// numérico e o "=" têm o mesmo).
type idTecla struct {
	sc   uint32
	nome key.Name
}

func idDe(ke key.Event) idTecla {
	if ke.Scancode != 0 {
		return idTecla{sc: ke.Scancode}
	}
	return idTecla{nome: ke.Name}
}

type teclaApertada struct {
	aba     Tab
	keysym  uint32
	keycode uint32
}

func (k *estadoTeclado) entregar(t Tab, keysym, keycode uint32, pressed bool) {
	if k.tecla != nil {
		k.tecla(t, keysym, keycode, pressed)
		return
	}
	enfileirarTecla(t, keysym, keycode, pressed)
}

func (k *estadoTeclado) entregarTexto(t Tab, s string) {
	if k.texto != nil {
		k.texto(t, s)
		return
	}
	enfileirarTexto(t, s)
}

func (k *estadoTeclado) entregarTravas(t Tab, travas uint32) {
	if k.travasPara != nil {
		k.travasPara(t, travas)
		return
	}
	enfileirarTravas(t, travas)
}

// valeParaAba: o RDP repassa por posição e precisa do keycode; SSH e VNC
// (receptorTexto) precisam do keysym. Tecla sem o código que a aba usa
// não tem como ir.
func valeParaAba(querTexto bool, keysym, keycode uint32) bool {
	if querTexto {
		return keysym != 0
	}
	return keycode != 0
}

// teclaDe entrega um key.Event à aba t.
func (k *estadoTeclado) teclaDe(t Tab, ke key.Event) {
	k.mods = ke.Modifiers
	id := idDe(ke)
	_, querTexto := t.(receptorTexto)

	if ke.State != key.Press {
		if ap, ok := k.apertadas[id]; ok {
			delete(k.apertadas, id)
			k.entregar(ap.aba, ap.keysym, ap.keycode, false)
			return
		}
		// Soltou sem ter apertado aqui: tecla que já vinha segurada quando
		// o foco chegou (Alt+Tab de volta), ou tecla de texto, cujo apertar
		// foi pelo EditEvent. A de texto não tem o que soltar; a outra
		// segue — soltar o que não está apertado é inofensivo do outro
		// lado, e é o que destrava um modificador preso por outro caminho.
		if querTexto && teclaDeTexto(ke) {
			return
		}
		ks, kc := traduzirTecla(ke)
		if valeParaAba(querTexto, ks, kc) {
			k.entregar(t, ks, kc, false)
		}
		return
	}

	// SSH e VNC recebem o caractere pelo EditEvent (ver teclado_outros.go):
	// mandar também a tecla faria o caractere sair duas vezes.
	if querTexto && produzTexto(ke) {
		return
	}
	ks, kc := traduzirTecla(ke)
	if !valeParaAba(querTexto, ks, kc) {
		return
	}
	if ap, repetida := k.apertadas[id]; repetida {
		// repetição automática: mesmos códigos do primeiro apertar.
		k.entregar(t, ap.keysym, ap.keycode, true)
		return
	}
	if k.apertadas == nil {
		k.apertadas = map[idTecla]teclaApertada{}
	}
	k.apertadas[id] = teclaApertada{aba: t, keysym: ks, keycode: kc}
	k.entregar(t, ks, kc, true)
}

// textoDe entrega à aba o texto que o Windows já resolveu (WM_CHAR).
func (k *estadoTeclado) textoDe(t Tab, s string) {
	if _, ok := t.(receptorTexto); !ok {
		return
	}
	// Ctrl ou Alt sozinho: a tecla é atalho e já foi como tecla (Ctrl+
	// Espaço, por exemplo, gera também um WM_CHAR de espaço).
	if !modsDeTexto(k.mods) {
		return
	}
	s = textoImprimivel(s)
	if s == "" {
		return
	}
	// AltGr é Ctrl+Alt para o Windows, e os dois já foram mandados como
	// apertados. Com eles apertados do outro lado o caractere viraria
	// Ctrl+Alt+caractere no servidor VNC: solta os dois antes e aperta de
	// novo depois — o mesmo que os clientes VNC fazem.
	var segurados []teclaApertada
	if k.mods.Contain(key.ModCtrl) && k.mods.Contain(key.ModAlt) {
		for _, ap := range k.ordenadas() {
			if ap.aba == t && ehCtrlOuAlt(ap.keysym) {
				segurados = append(segurados, ap)
			}
		}
	}
	for _, ap := range segurados {
		k.entregar(t, ap.keysym, ap.keycode, false)
	}
	k.entregarTexto(t, s)
	for i := len(segurados) - 1; i >= 0; i-- {
		ap := segurados[i]
		k.entregar(t, ap.keysym, ap.keycode, true)
	}
}

func ehCtrlOuAlt(keysym uint32) bool {
	switch keysym {
	case 0xffe3, 0xffe4, 0xffe9, 0xffea:
		return true
	}
	return false
}

// ehModificador: Shift, Ctrl, CapsLock, Meta, Alt e Super (0xffe1-0xffee).
func ehModificador(keysym uint32) bool {
	return keysym >= 0xffe1 && keysym <= 0xffee
}

// ordenadas devolve as teclas apertadas numa ordem fixa: as comuns antes,
// os modificadores por último — é a ordem de um teclado de verdade ao
// soltar (Ctrl+C: solta o C, depois o Ctrl), e soltar o Ctrl primeiro
// faria o servidor ver um "c" solto sem Ctrl.
func (k *estadoTeclado) ordenadas() []teclaApertada {
	ids := make([]idTecla, 0, len(k.apertadas))
	for id := range k.apertadas {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		a, b := k.apertadas[ids[i]], k.apertadas[ids[j]]
		if ma, mb := ehModificador(a.keysym), ehModificador(b.keysym); ma != mb {
			return mb
		}
		if ids[i].sc != ids[j].sc {
			return ids[i].sc < ids[j].sc
		}
		return ids[i].nome < ids[j].nome
	})
	out := make([]teclaApertada, len(ids))
	for i, id := range ids {
		out[i] = k.apertadas[id]
	}
	return out
}

// soltarTudo solta do lado remoto tudo o que está apertado.
func (k *estadoTeclado) soltarTudo() {
	k.soltarSe(func(teclaApertada) bool { return true })
}

// soltarForaDe solta o que foi apertado numa aba que não é mais t — trocou
// de aba com a tecla segurada.
func (k *estadoTeclado) soltarForaDe(t Tab) {
	k.soltarSe(func(ap teclaApertada) bool { return ap.aba != t })
}

func (k *estadoTeclado) soltarSe(cond func(teclaApertada) bool) {
	if len(k.apertadas) == 0 {
		return
	}
	for _, ap := range k.ordenadas() {
		if cond(ap) {
			k.entregar(ap.aba, ap.keysym, ap.keycode, false)
		}
	}
	for id, ap := range k.apertadas {
		if cond(ap) {
			delete(k.apertadas, id)
		}
	}
}

// tagClipboardOutros identifica, pro Gio, quem está pedindo o clipboard do
// sistema — ver clipboard.ReadCmd.
var tagClipboardOutros = new(int)

// clipboardLidoOutros é o último texto que NÓS lemos do clipboard do
// sistema (ou escrevemos nele) — mesmo papel do wl_data_source no Linux:
// sem isto, aplicar o clipboard remoto no clipboard do sistema faria a
// PRÓXIMA leitura achar que "o clipboard mudou" e mandar de volta pro
// remoto, ida e volta pra sempre.
var clipboardLidoOutros clipboardSync

// tratarClipboardFrame substitui, fora do Linux, tanto o lado "recebe do
// sistema" (grab.Start's onClipboardFn) quanto o lado "publica no sistema"
// (entregarClipboard, que no Windows não fazia nada — currentGrab é
// sempre nil). O motor dos dois lados (VNC/RDP) já manda e recebe texto
// pelos próprios canais; só faltava esta ponte (BACKLOG.md, item 2).
func tratarClipboardFrame(est *estadoJanela, gtx layout.Context, activeTab func() Tab) {
	gtx.Execute(clipboard.ReadCmd{Tag: tagClipboardOutros})
	for {
		e, ok := gtx.Event(transfer.TargetFilter{Target: tagClipboardOutros, Type: "application/text"})
		if !ok {
			break
		}
		de, ok := e.(transfer.DataEvent)
		if !ok {
			continue
		}
		r := de.Open()
		if r == nil {
			continue
		}
		b, _ := io.ReadAll(r)
		r.Close()
		texto := string(b)
		if !clipboardLidoOutros.checkAndSet(texto) {
			continue
		}
		registrarClipboardSistema(texto)
		if h, ok := activeTab().(clipboardReceiver); ok {
			h.OnLocalClipboard(texto)
		}
	}

	// Só o que é DESTA janela, e tirado com CompareAndSwap — mesma regra
	// do lado Linux (ver entregarClipboard, em clipboard.go): com a sessão
	// em janela própria há duas goroutines de quadro passando por aqui, e
	// quem não é a dona não pode consumir a fila da outra.
	if p := clipPendente.Load(); p != nil && p.w == est.w &&
		clipPendente.CompareAndSwap(p, nil) {
		// Pré-marca como já conhecido: sem isto, o próximo ReadCmd lia de
		// volta o que acabamos de escrever e ecoava pro remoto nesse
		// instante como se fosse uma cópia local nova.
		clipboardLidoOutros.checkAndSet(p.texto)
		gtx.Execute(clipboard.WriteCmd{
			Type: "application/text",
			Data: io.NopCloser(strings.NewReader(p.texto)),
		})
	}
}
