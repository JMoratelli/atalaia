# Senhas e identidade dos servidores

## Cofre

O cofre cifra as senhas salvas no inventário com uma senha mestra. Sem ele as
senhas ficam em texto puro no `conexoes.ini`, o que serve para testar mas não
para um inventário que circula entre pessoas ou fica numa pasta sincronizada.

Para criar: clique no cadeado no topo da janela e escolha a senha mestra
(**Criar cofre**). A partir daí, tudo o que for gravado pela interface sai
cifrado, e no arquivo a senha aparece como `enc:v1:...`.

A senha mestra é pedida uma vez, na primeira conexão que precisar de uma
senha cifrada (ou ao abrir a execução em massa com máquinas assim), e vale até
o app fechar. Pelo mesmo cadeado dá para **Trocar senha**: todas as senhas são
cifradas de novo com a senha nova e o arquivo é regravado.

Sem a senha mestra não há como recuperar as senhas cifradas. Guarde-a num
gerenciador de senhas.

O cofre mora no `chaveiro.ini`, ao lado do inventário. Copiando os dois
arquivos para outra máquina, as senhas vão junto e abrem com a mesma senha
mestra.

## Chaveiro

Muitas máquinas costumam usar a mesma credencial: o usuário de suporte de
todos os caixas, o administrador do domínio. Em vez de repetir a senha em cada
uma, cadastre a credencial uma vez no chaveiro (o botão da chave, no topo da
janela) com um apelido, e aponte para ela no inventário com `!apelido`:

```ini
[CENTRO-CX01]
ssh_usuario = !suporte-caixas
ssh_senha = !suporte-caixas
```

Trocar a senha no chaveiro troca em todas as máquinas que apontam para ela. Os
apelidos funcionam nos campos de usuário e senha de qualquer protocolo e nos
campos da execução em massa. As senhas do chaveiro também são cifradas pelo
cofre.

No arquivo, cada credencial é uma seção do `chaveiro.ini`:

```ini
[suporte-caixas]
usuario = suporte
senha   = enc:v1:...
```

## Identidade do servidor

Antes de mandar qualquer senha, o app confere se está falando com a máquina
certa. Uma identidade nova ou diferente nunca é aceita sozinha.

**SSH.** Na primeira conexão com uma máquina aparece a impressão digital da
chave dela, com **Confiar e conectar** ou **Não conectar**. A chave aceita vai
para o `~/.ssh/known_hosts`, o mesmo arquivo do comando `ssh`, então o que um
já conhece o outro também conhece. Se a chave mudar depois, o aviso é outro (**A
chave do servidor MUDOU**) e conectar exige aceitar a nova chave de propósito.
Chave que muda sem motivo pode ser máquina reinstalada, mas também pode ser
alguém no meio do caminho.

**RDP.** O certificado do servidor aparece na primeira conexão, com o nome, o
emissor e a impressão digital. **Confiar e salvar** grava o certificado e não
pergunta mais; **Só desta vez** conecta sem gravar. Quando o certificado muda,
o app avisa (**O certificado do servidor MUDOU**) antes de conectar.

A execução em massa é a exceção: ela não confere a chave das máquinas, porque
rodar em dezenas de caixas parando em cada primeira conexão não seria
praticável. Use-a só em rede que você controla.
