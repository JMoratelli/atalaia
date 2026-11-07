package main

import (
	"testing"

	"acessos-go/internal/chaveiro"
	"acessos-go/internal/conexoes"
)

// TestCredencialDigitadaResolveAlias cobre o bug relatado: um operador
// digitando "!Nome" num campo de usuário/senha da aba Massa (o app ensina
// esse alias em outros lugares — ver conexaodlg.go) tinha o texto "!Nome"
// enviado cru pro servidor em vez da credencial de verdade guardada no
// chaveiro.
func TestCredencialDigitadaResolveAlias(t *testing.T) {
	antigo := chaveiroAtual
	t.Cleanup(func() { chaveiroAtual = antigo })

	chaveiroAtual = &chaveiro.Arquivo{
		Credenciais: map[string]chaveiro.Credencial{
			"Zanthus": {Usuario: "suporte", Senha: "S3nh@Real"},
		},
		Ordem: []string{"Zanthus"},
	}

	t.Run("texto livre passa direto", func(t *testing.T) {
		c, err := credencialDigitada("fulano", "minhasenha")
		if err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		if c.Usuario != "fulano" || c.Senha != "minhasenha" {
			t.Fatalf("got %+v, esperado usuário/senha inalterados", c)
		}
	})

	t.Run("alias resolve pra credencial de verdade", func(t *testing.T) {
		c, err := credencialDigitada("!Zanthus", "!Zanthus")
		if err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		if c.Usuario != "suporte" || c.Senha != "S3nh@Real" {
			t.Fatalf("got %+v, esperado a credencial resolvida — não o alias cru", c)
		}
	})

	t.Run("alias quebrado na senha vira erro, não texto cru", func(t *testing.T) {
		_, err := credencialDigitada("qualquer", "!NaoExiste")
		if err == nil {
			t.Fatal("esperava erro de credencial inexistente")
		}
	})

	t.Run("sem chaveiro carregado, nada quebra", func(t *testing.T) {
		chaveiroAtual = nil
		c, err := credencialDigitada("fulano", "minhasenha")
		if err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		if c.Usuario != "fulano" || c.Senha != "minhasenha" {
			t.Fatalf("got %+v", c)
		}
	})
}

// TestPrecisaCofre cobre o outro relato da aba Massa: com o cofre trancado
// e senhas guardadas nele, ela seguia com a credencial da tela em vez de
// pedir a senha mestra, como abrir uma sessão faz.
func TestPrecisaCofre(t *testing.T) {
	antigoCofre, antigoChaveiro := cofreAberto, chaveiroAtual
	t.Cleanup(func() { cofreAberto, chaveiroAtual = antigoCofre, antigoChaveiro })
	cofreAberto = nil
	chaveiroAtual = &chaveiro.Arquivo{
		Credenciais: map[string]chaveiro.Credencial{
			"Cifrada": {Usuario: "zanthus", Senha: "enc:v1:qualquercoisa"},
			"Clara":   {Usuario: "zanthus", Senha: "texto"},
		},
		Ordem: []string{"Cifrada", "Clara"},
	}
	ssh := func(senha string) conexoes.Conexao {
		return conexoes.Conexao{Nome: "CAIXA", SSH: conexoes.AcessoSSH{Ligado: true, Porta: 22, Usuario: "zanthus", Senha: senha}}
	}

	casos := []struct {
		nome string
		cxs  []conexoes.Conexao
		quer bool
	}{
		{"senha em texto claro", []conexoes.Conexao{ssh("texto")}, false},
		{"senha cifrada", []conexoes.Conexao{ssh("enc:v1:abc")}, true},
		{"credencial do chaveiro cifrada", []conexoes.Conexao{ssh("!Cifrada")}, true},
		{"credencial do chaveiro em claro", []conexoes.Conexao{ssh("!Clara")}, false},
		{"uma cifrada no meio de claras", []conexoes.Conexao{ssh("a"), ssh("enc:v1:x"), ssh("b")}, true},
	}
	for _, c := range casos {
		if got := precisaCofre(c.cxs); got != c.quer {
			t.Errorf("%s: precisaCofre = %v, queria %v", c.nome, got, c.quer)
		}
	}
}
