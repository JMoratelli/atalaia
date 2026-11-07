<div align="center">

<!-- o banner da marca: o escuro no tema escuro do GitHub, o claro no claro -->
<picture>
  <source media="(prefers-color-scheme: dark)" srcset="icones/banner-escuro.png">
  <img src="icones/banner-claro.png" alt="Atalaia Acessos: VNC, RDP, SSH e SFTP em abas, numa janela só">
</picture>

**Acesso remoto às máquinas da empresa numa janela só: tela, Windows, terminal e arquivos em abas,<br>
comando em centenas de máquinas de uma vez e busca de qualquer lugar. Livre, gratuito, Linux e Windows.**

<!-- os selos do projeto: a versão sai sozinha da última release; testes contados com
     grep '^func Test' em cmd/ e internal/ — atualizar a cada versão -->
<p>
  <a href="https://github.com/JMoratelli/atalaia/releases/latest"><img alt="versão" src="https://img.shields.io/github/v/release/JMoratelli/atalaia?style=flat-square&label=vers%C3%A3o&color=5AA2FF"></a>
  <a href="#instalar"><img alt="Linux e Windows" src="https://img.shields.io/badge/sistemas-Linux%20%C2%B7%20Windows-1C211F?style=flat-square"></a>
  <a href="docs/compilar.md"><img alt="226 testes" src="https://img.shields.io/badge/testes-226%20casos-38D9A9?style=flat-square"></a>
  <a href="LICENSE"><img alt="licença GPLv3" src="https://img.shields.io/badge/licen%C3%A7a-GPLv3-B197FC?style=flat-square"></a>
  <a href="#comparação"><img alt="preço: R$ 0" src="https://img.shields.io/badge/pre%C3%A7o-R%24%200-E0B458?style=flat-square"></a>
</p>

<!-- a stack, com o logo de cada uma (shields.io); mudou uma versão maior, muda aqui -->
<p>
  <img alt="Go 1.27" src="https://img.shields.io/badge/Go-1.27-00ADD8?style=flat-square&logo=go&logoColor=white">
  <img alt="Gio 0.10, interface na GPU" src="https://img.shields.io/badge/Gio-0.10%20%C2%B7%20GPU-1C211F?style=flat-square">
  <img alt="OpenGL ES e Vulkan" src="https://img.shields.io/badge/OpenGL%20ES%20%C2%B7%20Vulkan-Linux-5586A4?style=flat-square&logo=vulkan&logoColor=white">
  <img alt="Direct3D 11" src="https://img.shields.io/badge/Direct3D%2011-Windows-0078D4?style=flat-square&logo=windows&logoColor=white">
  <img alt="FreeRDP 3.32" src="https://img.shields.io/badge/FreeRDP-3.32-D94A3B?style=flat-square">
  <img alt="LibVNC 0.9.15" src="https://img.shields.io/badge/LibVNC-0.9.15-4A6FA5?style=flat-square">
  <img alt="SSH e SFTP" src="https://img.shields.io/badge/SSH%20%C2%B7%20SFTP-x%2Fcrypto%20%C2%B7%20pkg%2Fsftp-2E8B57?style=flat-square">
  <img alt="Wayland" src="https://img.shields.io/badge/Wayland-nativo-FFBC00?style=flat-square&logo=wayland&logoColor=black">
  <img alt="Flatpak" src="https://img.shields.io/badge/Flatpak-25.08-4A90D9?style=flat-square&logo=flatpak&logoColor=white">
  <img alt="Inno Setup" src="https://img.shields.io/badge/Windows-Inno%20Setup-264B99?style=flat-square">
</p>

[**Instalar**](#instalar) · [**Documentação**](#documentação) · [**Novidades**](https://github.com/JMoratelli/atalaia/releases) · [**Avisos de terceiros**](THIRD-PARTY-NOTICES.md)

[O que faz](#o-que-ele-faz) · [Telas](#telas) · [Desempenho](#desempenho-e-estabilidade) · [Comparação](#comparação) · [Primeiros passos](#primeiros-passos) · [Licença](#licença)

</div>

---

O Atalaia Acessos junta numa janela só o que o suporte usa para chegar nas
máquinas: a tela (VNC), a área de trabalho do Windows (RDP), o terminal (SSH)
e os arquivos (SFTP). Cada máquina abre numa aba, o Painel mostra quem está
respondendo, e um atalho de teclado acha qualquer máquina sem precisar ir até
o programa.

Roda no Linux (Flatpak) e no Windows, a partir do mesmo código.

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="screenshots/principal.png">
  <img src="screenshots/principal-claro.png" alt="Painel com as máquinas agrupadas por loja, o cartão de boas-vindas e os números do inventário">
</picture>

<sub>As imagens usam um inventário de demonstração: empresa, máquinas, pessoas e endereços são fictícios.</sub>

## O que ele faz

- **Painel com todas as máquinas**, em grupos e subgrupos (lojas, setores,
  servidores), cada uma com um ponto que diz se ela está respondendo. O
  filtro procura por nome, endereço ou grupo, e clicar no cartão abre a
  máquina no protocolo escolhido.
- **Sessões em abas.** VNC e RDP embutidos, com a imagem ajustada ao tamanho
  da aba; terminal SSH com a biblioteca de comandos ao lado; SFTP com a sua
  máquina de um lado e a remota do outro, arrastando arquivos entre as duas.
- **RDP em janela própria**, inclusive em tela cheia, sem derrubar a sessão.
- **Execução em massa**: o mesmo comando em dezenas de máquinas, testado
  antes em três, com o resto esperando a sua confirmação.
- **Busca de qualquer lugar** com `Ctrl+Shift+F12`: uma caixa aparece por
  cima do que estiver na tela, você digita parte do nome e a máquina abre
  numa aba.
- **Destino avulso**: um endereço que não está cadastrado (`10.1.1.99`,
  `suporte@10.1.1.99`, `rdp servidor`) vira conexão na hora, sem cadastro.
- **Senhas protegidas**: o cofre cifra as senhas salvas com uma senha mestra,
  e o chaveiro guarda credenciais que várias máquinas usam, chamadas por um
  apelido.
- **Identidade do servidor conferida**: chave SSH e certificado RDP novos ou
  trocados aparecem para você decidir, e nunca são aceitos sozinhos.
- **Atualização pelo próprio app**: quando sai versão nova ele avisa e
  instala por cima, na hora que você escolher.
- Tema claro e escuro, letra maior com um clique (`A+`) e barra lateral que
  recolhe com `F12`.

## Telas

**Terminal SSH**, com os snippets ao lado: um clique manda o comando para a
sessão. Clicar no texto leva o cursor até ali (no nano e no vim também),
duplo clique marca a palavra e o botão direito copia e cola.

![Terminal SSH num servidor de arquivos, com a lista de snippets à direita](screenshots/ssh.png)

**Tela remota (VNC)**, aqui no caixa de uma loja.

![Sessão VNC mostrando a tela de um caixa de supermercado](screenshots/vnc.png)

**Arquivos (SFTP)**: esta máquina à esquerda, a remota à direita. Arraste
de um lado para o outro (soltando em cima de uma pasta, vai para dentro
dela), ou dê dois cliques no arquivo.

![Dois arquivos sendo arrastados para a pasta backups do servidor](screenshots/sftp.png)

**Execução em massa.** A colmeia mostra o lote inteiro de relance, um favo
por máquina na cor do estado. O comando roda primeiro em três máquinas; só depois
que você confere o resultado ele segue para as outras, uma por vez ou em
paralelo.

![Teste inicial em três caixas, esperando a confirmação para seguir](screenshots/canario.png)

![Resultado nos dez caixas, com um que não respondeu](screenshots/execucao-massa.png)

**Biblioteca de snippets**, os comandos que você usa sempre, com a
plataforma e a indicação de quem precisa de root.

![Editor de snippets](screenshots/snippets.png)

**`Ctrl+Shift+F12`**, de qualquer lugar: a caixa aparece por cima do que
estiver na tela, e a máquina escolhida abre numa aba.

![Caixa de busca do atalho global sobre o Painel](screenshots/busca.png)

Enquanto uma conexão abre, os favos pulsam em volta do ícone do protocolo;
se ela falha, viram um × vermelho.

<img src="screenshots/conectando.png" width="600" alt="Aba RDP conectando, com o favo roxo pulsando">

## Desempenho e estabilidade

- **Tudo na GPU.** A interface inteira é desenhada pela placa de vídeo, com o
  [Gio](https://gioui.org): OpenGL ES ou Vulkan no Linux, Direct3D 11 no
  Windows. As telas remotas (VNC e RDP) e o terminal também são compostos na
  GPU; a CPU fica para decodificar o que chega da rede.
- **Abre rápido e não gasta parado.** Do comando até a janela pronta, com 24
  máquinas no Painel: cerca de **0,25 s**. Parado, **0% de CPU**: o app só
  redesenha quando algo muda, e uma aba escondida não redesenha nem quando a
  sessão dela recebe dados.
- **Leve.** Cerca de **200 MB** de memória com o Painel aberto (app e serviço
  do atalho, contando o driver de vídeo), e **215 MB** com uma sessão VNC,
  uma SSH e uma SFTP abertas. O pacote Flatpak tem 7 MB; o instalador do
  Windows, 49 MB, com o FreeRDP e tudo de que ele precisa.
- **Uma sessão não derruba as outras.** Cada tela remota (VNC ou RDP) roda num
  processo próprio: se uma travar ou cair, o app e as outras sessões seguem.
  Conexão que a rede derrubou sem avisar é detectada pelo sinal de vida e
  oferece a reconexão (ou reconecta sozinha, se a máquina estiver marcada
  para isso).
- **Testado.** 226 testes automatizados rodam antes de cada versão, mais
  ferramentas de teste em janela de verdade para o que um teste comum não
  alcança (teclado, área de transferência, tela cheia, várias janelas).

<sub>Medido em 07/11/2026 num Ryzen 5 5600 com Radeon RX 7600, KDE Plasma 6 em
Wayland, com o inventário de demonstração. Memória como PSS (a parte de cada
processo nas bibliotecas compartilhadas, sem contar em dobro).</sub>

## Comparação

| | Linux | Windows | VNC, RDP, SSH e SFTP embutidos | Mesmo comando em várias máquinas | Atalho global para achar a máquina | Preço |
|---|---|---|---|---|---|---|
| **Atalaia Acessos** | sim | sim | sim | em lote: testa em 3, para na falha e mostra o resultado de cada máquina | sim, também no Linux e com o app fechado | **R$ 0**, livre (GPLv3), sem limite |
| Remmina | sim | não | sim | não encontramos | não encontramos | grátis, livre (GPLv2+) |
| MobaXterm | não | sim | sim | replica a digitação (MultiExec) | não encontramos | Home grátis até 12 sessões; Professional US$ 69 por usuário |
| mRemoteNG | não | sim | VNC, RDP e SSH; SFTP só para enviar arquivo | replica a digitação no SSH | não encontramos | grátis, livre (GPLv2); última versão estável de 2019 |
| Royal TS | não | sim | sim | sequências de teclas e tarefas em várias conexões | sim, no Windows | grátis até 10 conexões; 49 € por usuário |
| Devolutions RDM | sim | sim | sim | replica a digitação no SSH | no Windows e no macOS; no Linux só com o app em foco | grátis para uso individual; equipe US$ 400 por usuário por ano |
| Termius | sim | sim | só SSH e SFTP | snippets em vários hosts | não encontramos | Starter grátis; Pro US$ 10 por mês |

O que o Atalaia junta e os outros não juntam: **os quatro protocolos
embutidos, livre e nos dois sistemas**, sem limite de sessões nem licença por
pessoa. Na execução em massa, em vez de repetir a digitação em várias abas,
ele roda o comando como um lote: primeiro em três máquinas, espera você
conferir, para na falha perguntando o que fazer e mostra o resultado de cada
máquina na colmeia. E foi feito para o suporte de lojas: Painel com quem está
respondendo, destino avulso digitado na busca, cofre com senha mestra e
credenciais compartilhadas por apelido.

<sub>Recursos e preços conferidos nas páginas oficiais em 07/11/2026; preços sem
impostos, como cada fabricante divulga. "Não encontramos" quer dizer que a
documentação oficial não menciona, não que seja impossível. Fontes:
[Remmina](https://remmina.org/remmina-features/),
[MobaXterm](https://mobaxterm.mobatek.net/download.html),
[mRemoteNG](https://github.com/mRemoteNG/mRemoteNG/releases),
[Royal TS](https://www.royalapps.com/ts/buy),
[Devolutions RDM](https://devolutions.net/pricing/),
[Termius](https://termius.com/pricing).</sub>

## Instalar

Os pacotes ficam na página de
[versões](https://github.com/JMoratelli/atalaia/releases/latest).

**Linux.** Baixe o `org.jj.Acessos-<versão>.flatpak` e instale para o seu
usuário:

```bash
flatpak install --user org.jj.Acessos-2.9.0.flatpak
```

O pacote usa o runtime `org.freedesktop.Platform` 25.08, que o Flatpak busca
no Flathub se ainda não estiver instalado. Depois é só abrir pelo menu de
aplicativos ou com `flatpak run org.jj.Acessos`.

**Windows.** Baixe o `AcessosSetup-<versão>.exe` e execute. Ele instala só
para o seu usuário, sem pedir administrador, e cria o atalho no Menu Iniciar.

Daí em diante as atualizações chegam pelo próprio app.

## Primeiros passos

Na primeira vez o app cria um inventário de exemplo. As máquinas são
cadastradas pelo botão **Nova conexão** no Painel, ou direto no
`conexoes.ini`, que é um texto simples com uma seção por máquina:

```ini
[CAIXA-01]
grupo = Loja Centro;Caixas
host = 10.20.1.11
descricao = Caixa 1, entrada
vnc = 1
ssh = 1
ssh_usuario = suporte
```

| Onde o inventário fica | |
|---|---|
| Linux (Flatpak) | `~/.var/app/org.jj.Acessos/config/acessos/conexoes.ini` |
| Linux (compilado à mão) | `~/.config/acessos/conexoes.ini` |
| Windows | `%APPDATA%\acessos\conexoes.ini` |

Para guardar o inventário numa pasta sincronizada, aponte para ela com
`caminho` na seção `[geral]`. Para salvar senhas cifradas, clique no cadeado
no topo da janela e crie a senha mestra.

## Documentação

- [Inventário e preferências](docs/inventario.md): o `conexoes.ini` campo a
  campo, grupos e a seção `[geral]`.
- [Uso no dia a dia](docs/uso.md): Painel, abas, sessões, destino avulso e
  atalhos de teclado.
- [Execução em massa e snippets](docs/execucao-em-massa.md)
- [Senhas e identidade dos servidores](docs/senhas.md): cofre, chaveiro,
  chaves SSH e certificados RDP.
- [Atalho global](docs/atalho-global.md): o `Ctrl+Shift+F12` e o serviço que
  o mantém valendo com o app fechado.
- [Compilar](docs/compilar.md): Flatpak, build de desenvolvimento, Windows a
  partir do Linux e a organização do código.

## A versão em Python (1.x)

Até a 1.x o app era escrito em Python. Desde a 2.0 ele é escrito em Go, com
a interface desenhada na GPU, e lê os mesmos arquivos de antes
(`conexoes.ini`, `chaveiro.ini`, `snippets.ini`), no mesmo lugar: atualizar
da 1.x não pede migração. O código antigo continua acessível:

```bash
git checkout v1.2.2     # a última versão publicada em Python
git checkout 1.x        # a linha de manutenção
```

## Licença

GNU GPLv3. Veja [LICENSE](LICENSE).

O app usa trabalho de muita gente: o Gio, o FreeRDP, a LibVNC, as bibliotecas
Go de SSH e SFTP, as fontes IBM Plex, Sora e Space Grotesk, entre outros. A
lista completa, com a versão e a licença de cada um, está em
[THIRD-PARTY-NOTICES.md](THIRD-PARTY-NOTICES.md), que também vai dentro dos
pacotes e abre pelo **Sobre** do app.

## Sobre o desenvolvimento

Desenvolvido com IA, revisado por mim
  Este repositório utiliza inteligência artificial no processo criativo (vibecoding). No entanto, todo o código é revisado, testado e mantido 100% manualmente por mim, garantindo cuidado em cada detalhe como único mantenedor do projeto (por enquanto).

AI-Assisted, Human-Crafted
  This repository leverages artificial intelligence during the creative process (vibecoding). However, all code is 100% manually reviewed, tested, and maintained by me, ensuring care and quality as a solo maintainer (for now).
