package main

import "testing"

// abaTravas é uma sessão RDP de mentira: a conexão é trocável e o que a
// fila entregaria fica gravado.
type abaTravas struct {
	abaDeTeste
	conexao any
}

func (a *abaTravas) ConexaoTravas() any         { return a.conexao }
func (a *abaTravas) HandleTravas(travas uint32) {}

type envioTravas struct {
	aba    Tab
	travas uint32
}

// passo roda um quadro de sincTravas e devolve o que ele mandou.
func passo(s *sincTravas, alvo Tab, travas uint32, lidas *int) []envioTravas {
	var out []envioTravas
	s.atualizar(alvo, func() uint32 { *lidas++; return travas },
		func(t Tab, v uint32) { out = append(out, envioTravas{t, v}) })
	return out
}

func TestTravasSoVaoQuandoMudam(t *testing.T) {
	var s sincTravas
	var lidas int
	rdp := &abaTravas{conexao: new(int)}

	if got := passo(&s, rdp, travaNum, &lidas); len(got) != 1 || got[0].travas != travaNum {
		t.Fatalf("primeiro quadro com a sessão: mandou %v, queria Num Lock", got)
	}
	for i := 0; i < 3; i++ {
		if got := passo(&s, rdp, travaNum, &lidas); len(got) != 0 {
			t.Fatalf("estado igual reenviado no quadro %d: %v", i, got)
		}
	}
	// apertou Num Lock (e Caps): vai o estado novo
	if got := passo(&s, rdp, travaCaps, &lidas); len(got) != 1 || got[0].travas != travaCaps {
		t.Fatalf("mudança não foi: %v", got)
	}
}

// Religar é outro processo-filho, que nasce com as travas do servidor.
func TestTravasVaoDeNovoAoReligar(t *testing.T) {
	var s sincTravas
	var lidas int
	rdp := &abaTravas{conexao: new(int)}
	passo(&s, rdp, travaNum, &lidas)

	rdp.conexao = nil // caiu
	if got := passo(&s, rdp, travaNum, &lidas); len(got) != 0 {
		t.Fatalf("sem conexão não há a quem mandar, e mandou %v", got)
	}
	rdp.conexao = new(int) // religou
	if got := passo(&s, rdp, travaNum, &lidas); len(got) != 1 {
		t.Fatalf("religou e o estado não foi: %v", got)
	}
}

// Sair da sessão (painel, campo, janela sem foco) e voltar manda de novo,
// mesmo com o estado igual: lá fora o servidor pode ter mudado o dele — é o
// "sincroniza ao ganhar o foco" do mstsc.
func TestTravasVaoDeNovoNaVoltaDoFoco(t *testing.T) {
	var s sincTravas
	var lidas int
	rdp := &abaTravas{conexao: new(int)}
	passo(&s, rdp, travaNum, &lidas)

	antes := lidas
	if got := passo(&s, nil, travaNum, &lidas); len(got) != 0 {
		t.Fatalf("sem ninguém com o teclado, mandou %v", got)
	}
	if lidas != antes {
		t.Fatal("leu as travas sem sessão com o teclado — no Windows isso é uma ida à thread da janela por quadro")
	}
	if got := passo(&s, rdp, travaNum, &lidas); len(got) != 1 {
		t.Fatalf("voltou o foco e o estado não foi: %v", got)
	}
}

func TestTravasTrocandoDeSessao(t *testing.T) {
	var s sincTravas
	var lidas int
	a := &abaTravas{conexao: new(int)}
	b := &abaTravas{conexao: new(int)}
	passo(&s, a, travaNum, &lidas)
	if got := passo(&s, b, travaNum, &lidas); len(got) != 1 || got[0].aba != b {
		t.Fatalf("trocou para a outra sessão e ela não recebeu: %v", got)
	}
	if got := passo(&s, a, travaNum, &lidas); len(got) != 1 || got[0].aba != a {
		t.Fatalf("voltou para a primeira e ela não recebeu: %v", got)
	}
}

// SSH e VNC não têm o que fazer com as travas: vai o caractere.
func TestTravasSoParaQuemSincroniza(t *testing.T) {
	var s sincTravas
	var lidas int
	if got := passo(&s, &abaDeTeste{}, travaNum, &lidas); len(got) != 0 || lidas != 0 {
		t.Fatalf("aba sem sincronizaTravas recebeu %v (lidas=%d)", got, lidas)
	}
}
