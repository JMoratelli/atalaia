package main

import (
	"time"

	"golang.org/x/crypto/ssh"
)

// manterVivaSSH manda um sinal de vida (keepalive@openssh.com, o mesmo do
// ServerAliveInterval do ssh) a cada intervaloVivo enquanto a conexão
// existir.
//
// Dois motivos, os dois de loja: roteador e firewall de filial costumam
// derrubar conexão TCP parada depois de alguns minutos sem avisar nenhum
// dos lados, e o terminal ficava com cara de vivo até a próxima tecla
// sumir no vazio; e quando a máquina cai de verdade (cabo, energia), sem
// tráfego nenhum o TCP pode levar horas para perceber. Três sinais sem
// resposta seguidos fecham a conexão: quem a usa (SSH e SFTP) vê a queda e
// mostra o botão de reconectar, em vez de uma tela parada.
//
// O servidor responder "não conheço esse pedido" (o OpenSSH responde
// assim) também é sinal de vida: o que conta é a RESPOSTA chegar.
func manterVivaSSH(cli *ssh.Client) {
	fechou := make(chan struct{})
	go func() {
		cli.Wait()
		close(fechou)
	}()
	tic := time.NewTicker(intervaloVivo)
	defer tic.Stop()
	semResposta := 0
	for {
		select {
		case <-fechou:
			return
		case <-tic.C:
		}
		resp := make(chan error, 1)
		go func() {
			_, _, err := cli.SendRequest("keepalive@openssh.com", true, nil)
			resp <- err
		}()
		select {
		case <-fechou:
			return
		case err := <-resp:
			if err != nil {
				semResposta++
			} else {
				semResposta = 0
			}
		case <-time.After(esperaVivo):
			semResposta++
		}
		if semResposta >= 3 {
			reg("ssh: %s sem resposta a 3 sinais de vida; fechando a conexão", cli.RemoteAddr())
			cli.Close()
			return
		}
	}
}

var (
	intervaloVivo = 30 * time.Second
	esperaVivo    = 15 * time.Second
)
