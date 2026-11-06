//go:build windows

package main

import "gioui.org/app"

var procGetKeyState = user32.NewProc("GetKeyState")

// lerTravasLocais devolve Num/Caps/Scroll Lock desta máquina, nos bits de
// travas.go.
//
// Lê NA THREAD DA JANELA (w.Run), e não daqui: o GetKeyState do Windows é
// por thread — devolve o estado das teclas segundo as mensagens que AQUELA
// thread já tirou da fila —, e quem tira as mensagens de teclado é a
// thread do laço da janela do Gio (os_windows.go), não a goroutine do
// quadro. Lido daqui, o estado sairia velho. É a mesma leitura que o
// wfreerdp faz ao ganhar o foco.
//
// w.Run espera a função rodar lá. Chamado de dentro do quadro é seguro: a
// thread da janela está parada esperando este quadro, e a espera dela
// (eventLoop.deliverEvent) atende pedidos de Run.
func lerTravasLocais(w *app.Window) uint32 {
	var travas uint32
	w.Run(func() {
		ligada := func(vk uintptr) bool {
			r, _, _ := procGetKeyState.Call(vk)
			return r&1 != 0 // bit baixo: a trava está ligada
		}
		const vkCapital, vkNumLock, vkScroll = 0x14, 0x90, 0x91
		if ligada(vkScroll) {
			travas |= travaScroll
		}
		if ligada(vkNumLock) {
			travas |= travaNum
		}
		if ligada(vkCapital) {
			travas |= travaCaps
		}
	})
	return travas
}
