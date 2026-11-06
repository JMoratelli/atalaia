//go:build !linux && !windows

package main

import "gioui.org/app"

// lerTravasLocais: só Linux (grab) e Windows sabem ler. Sem RDP nestas
// plataformas (protocolos_semtela.go), o valor nem é usado.
func lerTravasLocais(*app.Window) uint32 { return 0 }
