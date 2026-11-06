package main

// Num Lock, Caps Lock e Scroll Lock entre esta máquina e a sessão remota.
//
// O RDP manda a tecla por POSIÇÃO, e quem decide se o "1" do numérico é
// "1" ou "End" é o Num Lock DO SERVIDOR. Ninguém sincronizava os dois: o
// servidor ficava com o estado dele (o da tela de logon, o do perfil) e o
// teclado do operador com outro — luz acesa e o numérico andando o cursor.
// Apertar Num Lock invertia os dois juntos, e eles continuavam trocados.
//
// O protocolo tem a peça para isso, o Synchronize Event, que DIZ ao
// servidor o estado das travas (ver rs_travas, em internal/rdp/rdpshim.c).
// Este arquivo decide QUANDO mandar: sempre que a aba que está com o
// teclado mudar, a conexão dela mudar (conectou, religou) ou o estado
// local mudar. Quem chama passa nil como aba quando o teclado não é da
// sessão (painel, diálogo, campo de texto, janela sem foco), e aí o
// próximo envio sai na volta, com o estado lido de novo — o operador pode
// ter mexido nas travas enquanto estava fora.
//
// O VNC fica de fora: lá vai o caractere (o keysym), já resolvido pelo
// estado local.

// Bits do TS_SYNC_EVENT do RDP (KBD_SYNC_* em freerdp/input.h). A leitura
// local, nas duas plataformas, já devolve nesta forma.
const (
	travaScroll uint32 = 1 << 0
	travaNum    uint32 = 1 << 1
	travaCaps   uint32 = 1 << 2
)

// sincronizaTravas é a aba que repassa ao servidor o estado das travas
// (RDP).
type sincronizaTravas interface {
	// ConexaoTravas identifica a conexão de agora, ou nil sem sessão
	// conectada. Mudou, manda de novo: a sessão nova nasce com as travas
	// do servidor.
	ConexaoTravas() any
	// HandleTravas manda o estado. Chamado pela fila de teclas
	// (filateclas.go), nunca do quadro: escreve no socket do filho.
	HandleTravas(travas uint32)
}

// sincTravas lembra o que já foi mandado, para só mandar quando mudar.
type sincTravas struct {
	aba     Tab
	conexao any
	travas  uint32
}

// atualizar roda uma vez por quadro. t é a aba que está com o teclado
// agora, ou nil quando nenhuma está; travas é o estado local, que só é
// lido quando t != nil.
//
// A entrega vai pela MESMA fila das teclas, e não direto: o Num Lock
// apertado dentro da sessão vai como tecla (o servidor inverte o dele) e
// em seguida como estado. Na ordem inversa, o estado chegaria antes da
// tecla e a tecla o desfaria.
func (s *sincTravas) atualizar(t Tab, travas func() uint32, entregar func(Tab, uint32)) {
	st, ok := t.(sincronizaTravas)
	if !ok {
		*s = sincTravas{}
		return
	}
	c := st.ConexaoTravas()
	if c == nil {
		*s = sincTravas{}
		return
	}
	v := travas()
	if s.aba == t && s.conexao == c && s.travas == v {
		return
	}
	*s = sincTravas{aba: t, conexao: c, travas: v}
	entregar(t, v)
}
