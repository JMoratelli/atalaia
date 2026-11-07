# Inventário e preferências

Tudo o que o Atalaia Acessos sabe sobre as máquinas mora num arquivo de texto,
o `conexoes.ini`. Dá para cadastrar e editar pela tela (botão **Nova conexão**
no Painel, ou botão direito no cartão → **Editar…**), e dá para editar o
arquivo direto: o app grava só a linha que mudou e mantém comentários, ordem e
o resto do arquivo como estavam.

## Onde fica

| Sistema | Arquivo |
|---|---|
| Linux (Flatpak) | `~/.var/app/org.jj.Acessos/config/acessos/conexoes.ini` |
| Linux (compilado à mão) | `~/.config/acessos/conexoes.ini` |
| Windows | `%APPDATA%\acessos\conexoes.ini` |

Ao lado dele ficam o `snippets.ini` (a biblioteca de comandos) e o
`chaveiro.ini` (cofre e credenciais compartilhadas). Os Ajustes, na
engrenagem do topo, mostram os caminhos em uso.

Para usar outro arquivo uma vez só: `acessos -ini /caminho/para/conexoes.ini`.

## Uma máquina

Cada seção é uma máquina, e o nome da seção é o nome que aparece no Painel:

```ini
[CENTRO-CX02]
grupo = Loja Centro;Caixas
host = 10.20.1.12
descricao = Caixa 2

# tela (VNC)
vnc = 1
porta = 5900
senha = ...
modo = encaixar

# terminal e arquivos (SSH e SFTP)
ssh = 1
ssh_porta = 22
ssh_usuario = suporte
ssh_senha = ...

# área de trabalho do Windows (RDP)
rdp = 0
```

O `grupo` aceita subgrupos separados por ponto e vírgula: `Loja Centro;Caixas`
aparece como **Caixas** dentro de **Loja Centro**, no Painel e na barra
lateral.

### Campos

| Campo | Para que serve | Padrão |
|---|---|---|
| `grupo` | grupo e subgrupos, separados por `;` | sem grupo |
| `host` | endereço IP ou nome da máquina | |
| `descricao` | uma linha de texto livre, mostrada na busca | |
| `windows` | `1` quando a máquina é Windows (a execução em massa usa PowerShell em vez de shell) | `0` |
| `vnc` | liga a tela (VNC) | `1` quando a chave não existe |
| `porta` | porta do VNC | `5900` |
| `usuario`, `senha` | credencial do VNC, quando o servidor pede | |
| `modo` | `encaixar` (a imagem cabe na aba), `1x1` (tamanho real, com rolagem) ou `dinamico` | `encaixar` |
| `ronly` | `1` abre a tela bloqueada, só para ver | `0` |
| `auto` | `1` reconecta a tela sozinho quando a conexão cai | `0` |
| `ssh` | liga o terminal e os arquivos (SSH e SFTP) | ligado quando há `ssh_usuario` |
| `ssh_porta` | porta do SSH | `22` |
| `ssh_usuario`, `ssh_senha` | credencial do SSH | |
| `ssh_auto` | reconecta o terminal sozinho | `0` |
| `rdp` | liga a área de trabalho remota (RDP) | ligado quando há `rdp_usuario` |
| `rdp_porta` | porta do RDP | `3389` |
| `rdp_usuario`, `rdp_senha`, `rdp_dominio` | credencial do RDP | |
| `rdp_tela` | `dinamico` (a resolução acompanha a aba), `cheia` ou `janela` | `dinamico` |
| `rdp_auto` | reconecta o RDP sozinho | `0` |

As senhas podem ficar em texto puro ou cifradas pelo cofre (`enc:v1:...`), e
os campos de usuário e senha aceitam um apelido do chaveiro (`!suporte-loja`).
Os dois estão em [senhas.md](senhas.md).

## A seção `[geral]`

São as preferências do app. Quase todas são gravadas sozinhas quando você muda
alguma coisa na tela; editar à mão também funciona.

| Chave | Valores | O que faz |
|---|---|---|
| `tema` | `claro`, `escuro` | o tema da janela (o botão de contraste no topo troca) |
| `fonte` | `0`, `1`, ... | o nível do botão `A+`; `0` é o tamanho padrão |
| `lateral` | `1`, `0` | barra lateral aberta ou recolhida (`F12` troca) |
| `empresa` | texto | o nome que aparece acima da saudação do Painel |
| `caminho` | uma pasta | muda o inventário, os snippets e o chaveiro para essa pasta |
| `atalho_global` | `1`, `0` | liga ou desliga o `Ctrl+Shift+F12` ([atalho-global.md](atalho-global.md)) |
| `atalho_autostart` | `1`, `0` | se o atalho sobe sozinho no login (Linux) |

### Inventário numa pasta sincronizada

Para várias pessoas ou várias máquinas usarem o mesmo inventário, coloque o
`conexoes.ini` (e o `snippets.ini` e o `chaveiro.ini`, se quiser levá-los
junto) numa pasta sincronizada e aponte para ela no arquivo padrão:

```ini
[geral]
caminho = /home/voce/Nuvem/Suporte
```

O app passa a abrir o inventário de lá. Se a pasta não tiver um
`conexoes.ini`, ele avisa e continua no arquivo padrão.

No Flatpak o app enxerga a pasta pessoal inteira, então qualquer pasta dentro
dela serve.
