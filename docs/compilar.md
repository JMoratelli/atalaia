# Compilar

## Flatpak (Linux)

```bash
./build.sh              # constrói e gera build/org.jj.Acessos-<versão>.flatpak
./build.sh --instalar   # constrói, gera o pacote e instala ou atualiza para o usuário
./build.sh --limpar     # apaga build/ inteiro
```

O build não usa a rede: as dependências Go vêm de `vendor/` (que o script
regenera), e o Go oficial e as duas bibliotecas C, libvncserver e FreeRDP 3,
são baixados pelo `flatpak-builder` a partir dos pacotes com sha256 fixo no
manifesto (`flatpak/org.jj.Acessos.yml`). A versão sai da primeira entrada do
`flatpak/org.jj.Acessos.metainfo.xml`.

## Desenvolvimento

Para rodar sem empacotar, com as bibliotecas do sistema (`libvncclient`,
`freerdp3`, `wayland-client`, `xkbcommon`):

```bash
go build -o acessos ./cmd/acessos && ./acessos
```

Opções úteis:

| Opção | O que faz |
|---|---|
| `-ini <arquivo>` | usa outro inventário |
| `-tema claro\|escuro` | força o tema, por cima do que está no inventário |
| `-filtro <termo>` | abre o Painel já filtrado |
| `-conn type=ssh,host=...,user=...` | abre uma conexão direto (repetível); `type` aceita `vnc`, `rdp`, `ssh` e `sftp` |
| `-regua` | desenha a régua de alinhamento por cima da interface (`Ctrl+G` liga e desliga) |

Depois de mexer em `third_party/gio` (ou em qualquer `replace` local do
`go.mod`), rode `go mod vendor` antes de compilar, senão o build usa a cópia
velha em `vendor/`.

Os testes rodam com `go test ./...`.

## Windows (a partir do Linux)

O mesmo código gera o executável e o instalador do Windows, no Linux, numa
passada só:

```bash
scripts/build-windows.sh               # build/win/dist/ (acessos.exe e DLLs)
scripts/build-windows.sh --instalador  # build/win/AcessosSetup-<versão>.exe
scripts/build-windows.sh --limpar      # apaga build/win/
```

Nada é instalado no sistema e nada pede root. O script:

1. mede o runtime C do cross-compiler e monta um sysroot MinGW com os pacotes
   binários do MSYS2 do mesmo sabor (`scripts/sysroot-msys2.py`, repositório
   `ucrt64` ou `mingw64`);
2. gera o ícone a partir do mesmo SVG do Linux (`icones/acessos.svg`);
3. compila o `libvncclient.dll` a partir do código da LibVNCServer, com os
   mesmos patches de segurança do Flatpak (`flatpak/patches/libvncserver/`),
   no lugar do pacote do MSYS2, que não os tem;
4. compila o app com cgo, com VNC e RDP ligados;
5. confere que o `.exe` e as DLLs ficaram no mesmo runtime C
   (`scripts/crt-windows.sh`);
6. resolve recursivamente as DLLs de que o `.exe` depende
   (`scripts/dlls-windows.py`);
7. no `--instalador`, compila `scripts/instalador.iss` com o Inno Setup rodando
   num prefixo Wine próprio, em `build/win/wine`; o `~/.wine` do usuário fica
   intocado.

A conferência do runtime C não é zelo: os dois sabores têm heaps separados, e
o pacote da 2.7.1 saiu com o `.exe` num e as DLLs no outro. Toda sessão VNC
morria ao conectar, num `free()` que devolvia ao heap errado a memória
alocada dentro da `libvncclient`. Hoje o build falha se isso se repetir.

O instalador não pede administrador: instala em `%LOCALAPPDATA%\Acessos`, com
atalho no Menu Iniciar e, se escolhido, na Área de Trabalho. O inventário fica
em `%APPDATA%\acessos\conexoes.ini`.

Diferenças em relação ao Linux: a sonda de vida usa `IcmpSendEcho` no lugar do
socket ICMP sem privilégio, e o app grava `log.txt` e `freerdp.log` ao lado do
`conexoes.ini`, já que um aplicativo gráfico do Windows não tem saída de
terminal.

## Organização do código

```
cmd/acessos/          o aplicativo: interface, abas, diálogos
internal/
  vnc/ rdp/           clientes próprios, em cgo sobre a libvncclient e a libfreerdp3
  telaproc/           o canal com o processo-filho que hospeda cada sessão de tela
  cgoregistry/        a ponte entre os callbacks em C e os objetos em Go
  grab/               teclado, área de transferência e captura de atalhos,
                      falando Wayland direto
  conexoes/ iniutil/  leitura e gravação cirúrgica do conexoes.ini e dos demais .ini
  cofre/ chaveiro/    senhas cifradas e credenciais reutilizáveis
  massa/              execução em massa
  hostkey/ vida/      identidade do servidor SSH e sonda de vida
  atualizador/        busca e instalação de versões novas
scripts/              build do Windows: sysroot MSYS2, DLLs, instalador
third_party/gio/      o Gio com os patches do projeto (ver PATCH.md)
flatpak/              manifesto, .desktop e metainfo
icones/               ícone do app e banners do README
screenshots/          as imagens do README e da loja
```

Os demais comandos em `cmd/` são ferramentas de teste manual, cada uma de um
protocolo ou de um problema específico (captura de teclado, clipboard, tela
cheia, cursores), usadas para medir em janela de verdade o que um `go test`
não alcança.
