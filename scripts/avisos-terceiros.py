#!/usr/bin/env python3
"""Gera o THIRD-PARTY-NOTICES.md: o que de terceiros vai nos pacotes do
Atalaia Acessos, com versão, licença e os textos das licenças.

Rodar da raiz do repositório DEPOIS dos dois builds (./build.sh e
scripts/build-windows.sh), porque é deles que saem as listas de verdade:

    python3 scripts/avisos-terceiros.py

Fontes de cada parte:
  - módulos Go: vendor/modules.txt (o que o build usa), com o LICENSE de
    cada um em vendor/ (ou em third_party/, os dois forks);
  - o próprio Go: o LICENSE do GOROOT;
  - fontes: cmd/acessos/fontes/OFL-*.txt;
  - bibliotecas C do Flatpak: os tarballs no cache do flatpak-builder
    (.flatpak-builder/downloads), os mesmos do manifesto;
  - DLLs do Windows: os pacotes do MSYS2 em build/win/.cache, cruzados
    com as DLLs que de fato foram para build/win/dist. Os textos dessas
    licenças vão no próprio instalador (pasta licencas/), copiados do
    sysroot pelo build-windows.sh.
"""
import collections
import glob
import os
import re
import subprocess
import sys
import tarfile

RAIZ = os.getcwd()
SAIDA = os.path.join(RAIZ, "THIRD-PARTY-NOTICES.md")


def ler(caminho):
    with open(caminho, encoding="utf-8", errors="replace") as f:
        return f.read().replace("\r\n", "\n").strip("\n")


def modulos_go():
    """(módulo, versão, pasta) de cada módulo que entra no binário do app,
    no Linux OU no Windows (go list -deps nos dois), com o replace."""
    usados = set()
    for goos in ("linux", "windows"):
        amb = dict(os.environ, GOOS=goos, GOARCH="amd64", CGO_ENABLED="1", GOWORK="off", GOFLAGS="-mod=vendor")
        r = subprocess.run(["go", "list", "-deps", "-f", "{{with .Module}}{{.Path}}{{end}}", "./cmd/acessos"],
                           capture_output=True, text=True, env=amb, cwd=RAIZ)
        usados |= {l for l in r.stdout.split() if l and l != "acessos-go"}
    mods = []
    for linha in open(os.path.join(RAIZ, "vendor/modules.txt"), encoding="utf-8"):
        m = re.match(r"# (\S+) (\S+)(?: => (\S+))?", linha)
        if not m or m.group(2) == "=>" or m.group(1) not in usados:
            continue
        nome, versao, troca = m.group(1), m.group(2), m.group(3)
        pasta = os.path.join(RAIZ, "vendor", nome)
        if troca and troca.startswith("./"):
            pasta = os.path.join(RAIZ, troca[2:])
            versao = "fork local (" + versao + ")"
        mods.append((nome, versao, pasta))
    return mods


def licenca_em(pasta):
    for nome in sorted(os.listdir(pasta)):
        if re.match(r"(?i)^(licen[cs]e|copying|unlicense)(\.(md|txt))?$", nome):
            return os.path.join(pasta, nome)
    return None


def tipo_licenca(texto):
    t = texto.lower()
    mit = re.search(r"\bmit\b", t) is not None
    bsd = re.search(r"\bbsd\b", t) is not None
    if "unlicense" in t and mit:
        return "Unlicense ou MIT"
    if "unlicense" in t and bsd:
        return "Unlicense ou BSD-3-Clause"
    if "public domain" in t or "unencumbered" in t:
        return "Unlicense (domínio público)"
    if "apache license" in t:
        return "Apache-2.0"
    if "permission is hereby granted, free of charge" in t:
        return "MIT"
    if "redistribution and use in source and binary forms" in t:
        return "BSD-3-Clause" if "neither the name" in t else "BSD-2-Clause"
    return "ver o texto"


def tarball(sha, nome):
    achados = glob.glob(os.path.join(RAIZ, ".flatpak-builder/downloads", sha, nome))
    return achados[0] if achados else None


def de_tarball(caminho, sufixo):
    with tarfile.open(caminho) as t:
        for m in t.getmembers():
            if m.name.count("/") == 1 and m.name.endswith(sufixo):
                return t.extractfile(m).read().decode("utf-8", "replace").replace("\r\n", "\n").strip("\n")
    return None


def pacotes_windows():
    """{(pacote, versão, licença, url): [dlls]} das DLLs em build/win/dist."""
    cache = os.path.join(RAIZ, "build/win/.cache")
    dist = os.path.join(RAIZ, "build/win/dist")
    if not os.path.isdir(cache) or not os.path.isdir(dist):
        return None
    dlls = {os.path.basename(p).lower() for p in glob.glob(dist + "/*.dll") + glob.glob(dist + "/*/*.dll")}
    pacotes = {}
    for f in sorted(glob.glob(cache + "/*.pkg.tar.zst")):
        info = subprocess.run(["tar", "--zstd", "-xOf", f, ".PKGINFO"], capture_output=True, text=True).stdout
        campos = collections.defaultdict(list)
        for l in info.splitlines():
            m = re.match(r"(\w+) = (.*)", l)
            if m:
                campos[m.group(1)].append(m.group(2))
        if campos["pkgname"]:
            pacotes[campos["pkgname"][0]] = (f, campos)  # o último (versão maior) fica
    achados = {}
    for nome, (f, c) in pacotes.items():
        for l in subprocess.run(["tar", "--zstd", "-tf", f], capture_output=True, text=True).stdout.splitlines():
            b = os.path.basename(l).lower()
            if b.endswith(".dll") and b in dlls and b not in achados:
                lic = " / ".join(x.replace("spdx:", "") for x in c["license"])
                achados[b] = (nome.replace("mingw-w64-ucrt-x86_64-", "").replace("mingw-w64-x86_64-", ""),
                              c["pkgver"][0], lic, (c["url"] or [""])[0])
    por = collections.defaultdict(list)
    for dll, chave in achados.items():
        por[chave].append(dll)
    return por


def main():
    goroot = subprocess.run(["go", "env", "GOROOT"], capture_output=True, text=True).stdout.strip()
    # Sem o sufixo de distribuição ("go1.27.3-X:nodwarf5" no Arch): a
    # versão é a mesma do Go oficial que o manifesto do Flatpak usa.
    goversao = subprocess.run(["go", "env", "GOVERSION"], capture_output=True, text=True).stdout.strip().split("-")[0]
    textos = []  # (título, texto)
    out = []
    w = out.append

    w("# Avisos de terceiros")
    w("")
    w("O Atalaia Acessos é software livre, sob a GNU GPLv3 (ver [LICENSE](LICENSE)), e é")
    w("feito em cima do trabalho de muita gente. Esta é a lista do que de terceiros vai")
    w("nos pacotes, com a versão e a licença de cada um, e no fim os textos das")
    w("licenças. Ela é gerada por `scripts/avisos-terceiros.py` a partir do que os")
    w("builds de fato empacotam; não edite à mão.")
    w("")
    w("## No aplicativo (Linux e Windows)")
    w("")
    w("| Componente | Versão | Licença | Para que serve |")
    w("|---|---|---|---|")
    w(f"| [Go](https://go.dev) (compilador e biblioteca padrão) | {goversao} | BSD-3-Clause | a linguagem do app |")
    # O LICENSE do Go: no GOROOT (instalação oficial), em /usr/share/licenses
    # (o pacote do Arch move para lá) ou no tarball oficial do cache do Flatpak.
    licgo = next((c for c in (os.path.join(goroot, "LICENSE"), "/usr/share/licenses/go/LICENSE") if os.path.exists(c)), None)
    if licgo:
        textos.append(("Go", ler(licgo)))
    else:
        for tb in glob.glob(os.path.join(RAIZ, ".flatpak-builder/downloads/*/go1.*.linux-*.tar.gz")):
            with tarfile.open(tb) as t:
                textos.append(("Go", t.extractfile("go/LICENSE").read().decode().strip("\n")))
            break
    uso = {
        "gioui.org": "a interface, desenhada na GPU (fork com os patches do projeto)",
        "gioui.org/x": "extensões do Gio (seletor de arquivos)",
        "gioui.org/shader": "shaders do Gio",
        "gio.tools/icons": "os ícones (Material Design Icons, do Google, Apache-2.0)",
        "github.com/go-text/typesetting": "composição de texto",
        "github.com/godbus/dbus/v5": "portais do Linux (atalho global, segundo plano)",
        "github.com/hinshun/vt10x": "o emulador de terminal do SSH (fork com patches)",
        "github.com/pkg/sftp": "transferência de arquivos (SFTP)",
        "github.com/kr/fs": "usado pelo sftp",
        "golang.org/x/crypto": "o cliente SSH",
        "golang.org/x/net": "sonda de vida (ICMP)",
        "golang.org/x/sys": "chamadas de sistema",
        "golang.org/x/text": "texto e codificações",
        "golang.org/x/term": "terminal",
        "golang.org/x/image": "imagens (usado pela composição de texto)",
        "golang.org/x/exp/shiny": "usado pelo Gio",
        "git.wow.st/gmp/jni": "usado pelo Gio (Android, não entra no binário de desktop)",
        "github.com/go-ole/go-ole": "usado pelo Gio no Windows",
    }
    for nome, versao, pasta in modulos_go():
        arq = licenca_em(pasta) if os.path.isdir(pasta) else None
        texto = ler(arq) if arq else ""
        lic = tipo_licenca(texto) if texto else "não encontrada"
        link = "https://pkg.go.dev/" + nome
        w(f"| [{nome}]({link}) | {versao} | {lic} | {uso.get(nome, '')} |")
        if texto:
            textos.append((nome, texto))
    fontes = [
        ("IBM Plex Sans, Sans Condensed e Mono", "OFL-IBMPlex.txt", "https://github.com/IBM/plex", "texto de apoio e terminal"),
        ("Sora", "OFL-Sora.txt", "https://github.com/sora-xor/sora-font", "títulos"),
        ("Space Grotesk", "OFL-SpaceGrotesk.txt", "https://github.com/floriankarsten/space-grotesk", "texto da interface"),
    ]
    for nome, arq, link, para in fontes:
        caminho = os.path.join(RAIZ, "cmd/acessos/fontes", arq)
        w(f"| [{nome}]({link}) (fonte) | | OFL-1.1 | {para} |")
        if os.path.exists(caminho):
            textos.append((nome, ler(caminho)))
    w("")

    w("## Bibliotecas C do pacote Linux (Flatpak)")
    w("")
    w("Compiladas junto com o app pelo manifesto (`flatpak/org.jj.Acessos.yml`). O resto")
    w("do que o app usa no Linux vem do runtime `org.freedesktop.Platform`, instalado à")
    w("parte pelo Flatpak, e não é distribuído por nós.")
    w("")
    w("| Componente | Versão | Licença | Para que serve |")
    w("|---|---|---|---|")
    frdp = tarball("9ccb8357bf0188044a460de775c7ea289ca7d2a2a917eca0f0a7c4dc9c8b08ea", "3.32.2.tar.gz")
    vnc = tarball("62352c7795e231dfce044beb96156065a05a05c974e5de9e023d688d8ff675d7", "LibVNCServer-0.9.15.tar.gz")
    w("| [FreeRDP](https://www.freerdp.com) | 3.32.2 | Apache-2.0 | o cliente RDP |")
    w("| [LibVNCServer / LibVNCClient](https://libvnc.github.io) | 0.9.15, com três correções de segurança (ver `flatpak/patches/libvncserver/`) | GPL-2.0-or-later | o cliente VNC |")
    if frdp:
        textos.append(("FreeRDP", de_tarball(frdp, "/LICENSE")))
    if vnc:
        textos.append(("LibVNCServer / LibVNCClient", de_tarball(vnc, "/COPYING")))
    w("")

    w("## Bibliotecas do instalador do Windows")
    w("")
    por = pacotes_windows()
    if por is None:
        w("(Gerar depois do `scripts/build-windows.sh`: a lista sai das DLLs que ele empacota.)")
    else:
        w("Vêm prontas do [MSYS2](https://www.msys2.org) (repositório UCRT64), menos o")
        w("`libvncclient.dll`, que o build compila a partir do código da LibVNCServer com as")
        w("mesmas correções de segurança do Flatpak. A maior parte é dependência do FreeRDP.")
        w("Os textos completos destas licenças vão no próprio instalador, na pasta")
        w("`licencas`, ao lado do `acessos.exe`.")
        w("")
        w("| Pacote | Versão | Licença (como o MSYS2 declara) | DLLs |")
        w("|---|---|---|---|")
        for (nome, ver, lic, url), dlls in sorted(por.items()):
            if nome == "libvncserver":
                ver += " + correções (compilado pelo build)"
            ligado = f"[{nome}]({url})" if url else nome
            w(f"| {ligado} | {ver} | {lic} | {', '.join(sorted(dlls))} |")
    w("")

    w("## Textos das licenças")
    w("")
    for titulo, texto in textos:
        if not texto:
            continue
        w(f"### {titulo}")
        w("")
        w("```text")
        w(texto)
        w("```")
        w("")

    with open(SAIDA, "w", encoding="utf-8") as f:
        f.write("\n".join(out).rstrip("\n") + "\n")
    print(f"{SAIDA}: {len(textos)} textos de licença")


if __name__ == "__main__":
    sys.exit(main())
