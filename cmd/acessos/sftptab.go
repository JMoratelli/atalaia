package main

import (
	"fmt"
	"image"
	"image/color"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gio.tools/icons"
	"gioui.org/app"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"acessos-go/internal/hostkey"
	"acessos-go/internal/massa/executor"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// sftpTab é o navegador de arquivos: dois painéis lado a lado, LOCAL e
// REMOTO, com transferência nos dois sentidos.
//
// Duas decisões que vêm do app original e não são detalhe:
//   - conflito é verificado ANTES de começar a transferir, e o operador
//     escolhe sobrescrever, pular os existentes ou cancelar. Descobrir no
//     meio do caminho já custou arquivo sobrescrito.
//   - excluir no remoto levanta a árvore inteira primeiro, diz quantos
//     arquivos e pastas serão apagados e só então oferece o botão, com
//     contagem — não tem desfazer do outro lado.
type sftpTab struct {
	th *material.Theme
	w  *app.Window
	invalidador
	titulo string
	host   string
	porta  int
	user   string
	senha  string

	mu         sync.Mutex
	cli        *sftp.Client
	ssh        *ssh.Client
	estado     string
	fechado    bool
	conectando atomic.Bool // evita duas conectar() concorrentes (clique duplo, botão do splash)

	splash    *splash
	btnSplash widget.Clickable

	local  painelArquivos
	remoto painelArquivos

	btnEnviar, btnBaixar widget.Clickable
	btnRec               widget.Clickable
	btnHost              widget.Clickable
	trocarHost           func()

	progresso string
	ocupado   bool
	msg       string
	msgErro   bool
	// Números da transferência em curso, para a barra do rodapé (o
	// progresso em texto continua indo para a barra da sessão).
	progFeito, progTotal int64
	progArquivo          string
	progI, progN         int
	progInicio           time.Time

	// Arrastar e soltar entre os painéis (ver sftppainel.go). O Gio não
	// traz "arrastar item de lista" pronto; o que existe é o evento de
	// ponteiro, numa área por cima da aba inteira.
	tagArrasto *int
	arrastando bool
	origem     *painelArquivos // de onde o arrasto saiu
	posArrasto image.Point     // onde o botão desceu
	posAtual   image.Point     // onde o ponteiro está agora
	itensArr   []itemArquivo   // o que vai junto
	// O item sob o botão no momento do aperto, anotado pela lista: é ele
	// que vai quando o arrasto começa fora da seleção (como em qualquer
	// gerenciador de arquivos), e não a seleção antiga.
	pressPainel *painelArquivos
	pressNome   string
	// O item sob o ponteiro (sem botão), para o menu do botão direito.
	sobPainel *painelArquivos
	sobNome   string
	// ativo é o último painel em que se clicou: é nele que o Delete, o F2
	// e o Enter agem.
	ativo *painelArquivos
}

// listagem é o resultado de ler uma pasta. Ela NÃO é aplicada por quem
// leu: fica aqui até o laço de quadro aplicar (ver aplicarListagens).
//
// Dois motivos, os dois vividos: a leitura remota roda em goroutine e
// escrevia itens/btnItem/sel enquanto a interface os lia, e a leitura
// local, por ser síncrona, trocava o btnItem NO MEIO do laço que estava
// percorrendo o btnItem antigo — entrar numa pasta com menos arquivos
// que a anterior derrubava o app com "index out of range".
type listagem struct {
	itens []itemArquivo
	erro  string
}

// painelArquivos é um lado do navegador.
type painelArquivos struct {
	remoto   bool
	pend     atomic.Pointer[listagem]
	caminho  widget.Editor
	filtro   widget.Editor // busca dentro da pasta atual (nunca recursiva)
	itens    []itemArquivo
	sel      map[string]bool
	lista    widget.List
	btnItem  []widget.Clickable
	btnAcima widget.Clickable
	btnAtual widget.Clickable
	erro     string

	// Ferramentas do cabeçalho do painel (ver sftppainel.go).
	btnNovaDir, btnRenomear, btnExcluir widget.Clickable
	// Caminho em migalhas; clicar no vazio ao lado abre a edição.
	btnMigalha []widget.Clickable
	btnEditar  widget.Clickable
	editando   bool
	focouCam   bool
	// ancora é o último item clicado sem Shift: Shift+clique marca do
	// ancora até o item clicado.
	ancora string
	// Onde o painel e a lista ficaram no último quadro, em coordenadas da
	// aba: o arrasto usa isso para saber sobre qual painel (e sobre qual
	// pasta) o ponteiro está.
	area   image.Rectangle
	listaY int
	linhaH int
}

type itemArquivo struct {
	nome  string
	dir   bool
	tam   int64
	mtime time.Time
}

func newSFTPTab(w *app.Window, spec map[string]string) (Tab, error) {
	host := spec["host"]
	if host == "" {
		return nil, fmt.Errorf("sftp: falta host")
	}
	if spec["user"] == "" {
		return nil, fmt.Errorf("sftp: falta usuário")
	}
	porta := 22
	if p, err := strconv.Atoi(spec["port"]); err == nil && p > 0 {
		porta = p
	}
	t := &sftpTab{
		th: temaApp, w: w,
		titulo: rotuloAba(spec, "SFTP", host),
		host:   host, porta: porta,
		user: spec["user"], senha: spec["pass"],
		estado: "conectando…",
	}
	t.splash = novoSplash(func() *app.Window { return w })
	t.local.sel = map[string]bool{}
	t.remoto.sel = map[string]bool{}
	t.remoto.remoto = true
	t.local.caminho.SingleLine = true
	t.remoto.caminho.SingleLine = true
	// Submit: sem isto o Enter no campo de caminho vira um '\n' que o
	// próprio Editor, por ser SingleLine, troca por um ESPAÇO (ver
	// widget.Editor.processKey) — o campo nunca soltava um SubmitEvent
	// para a aba entrar na pasta digitada.
	t.local.caminho.Submit = true
	t.remoto.caminho.Submit = true
	t.local.filtro.SingleLine = true
	t.remoto.filtro.SingleLine = true
	if lar, err := os.UserHomeDir(); err == nil {
		t.local.caminho.SetText(lar)
	}
	t.local.lista.Axis = layout.Vertical
	t.remoto.lista.Axis = layout.Vertical
	t.tagArrasto = new(int)
	t.listarLocal()
	go t.conectar()
	return t, nil
}

func (t *sftpTab) Title() string { return t.titulo }
func (t *sftpTab) Pinned() bool  { return false }
func (t *sftpTab) Selo() (*widget.Icon, color.NRGBA, color.NRGBA) {
	return icons.ActionDescription, tema.AtencaoFg, tema.AtencaoBg
}
func (t *sftpTab) HandleKey(_, _ uint32, _ bool)                {}
func (t *sftpTab) HandlePointer(_ pointer.Event, _ image.Point) {}

func (t *sftpTab) Close() {
	t.mu.Lock()
	t.fechado = true
	cli, sc := t.cli, t.ssh
	t.cli, t.ssh = nil, nil
	t.mu.Unlock()
	if cli != nil {
		cli.Close()
	}
	if sc != nil {
		sc.Close()
	}
}

// passosSFTP: o SFTP não tem laço de reconexão automática (só o botão
// Reconectar) e faz só duas chamadas bloqueantes sequenciais — Dial e
// NewClient — então dois passos bastam.
var passosSFTP = []string{"Conectando", "Abrindo SFTP"}

func (t *sftpTab) conectar() {
	if !t.conectando.CompareAndSwap(false, true) {
		return // já há uma tentativa em andamento (clique duplo, botão do splash)
	}
	defer t.conectando.Store(false)

	t.setEstado("conectando…")
	t.splash.iniciar(passosSFTP)
	cfg := &ssh.ClientConfig{
		User: t.user,
		Auth: []ssh.AuthMethod{ssh.Password(t.senha)},
		// mesmo motivo do sshtab: PDV antigo só oferece algoritmo que o
		// Go corta por padrão, e o aperto de mão falha com a rede boa.
		Config:          executor.AlgoritmosLegado(),
		HostKeyCallback: hostkey.Callback(),
		Timeout:         8 * time.Second,
	}
	sc, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", t.host, t.porta), cfg)
	if err != nil {
		if ec := erroDeChave(err); ec != nil {
			t.setEstado(ec.Error())
			t.splash.setErro(ec.Error())
			pedirConfiancaHostKey(t.w, ec, func() { go t.conectar() })
			return
		}
		t.setEstado(err.Error())
		t.splash.setErro(err.Error())
		return
	}
	t.splash.avancar(1)
	cli, err := sftp.NewClient(sc)
	if err != nil {
		sc.Close()
		t.setEstado(err.Error())
		t.splash.setErro(err.Error())
		return
	}
	t.mu.Lock()
	if t.fechado {
		t.mu.Unlock()
		cli.Close()
		sc.Close()
		return
	}
	t.cli, t.ssh, t.estado = cli, sc, ""
	t.mu.Unlock()
	t.splash.concluir()

	inicio := "."
	if lar, err := cli.Getwd(); err == nil {
		inicio = lar
	}
	t.remoto.caminho.SetText(inicio)
	t.listarRemoto()
	t.invalidar()
}

func (t *sftpTab) setEstado(s string) {
	t.mu.Lock()
	t.estado = s
	t.mu.Unlock()
	t.invalidar()
}

func (t *sftpTab) encerrada() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.fechado
}

func (t *sftpTab) cliente() *sftp.Client {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.cli
}

// ------------------------------------------------------------ listagem

func ordenar(itens []itemArquivo) {
	// pastas primeiro, depois alfabético — é como todo navegador de
	// arquivos ordena, e procurar pasta no meio de 300 arquivos é pior.
	sort.Slice(itens, func(i, j int) bool {
		if itens[i].dir != itens[j].dir {
			return itens[i].dir
		}
		return strings.ToLower(itens[i].nome) < strings.ToLower(itens[j].nome)
	})
}

func (t *sftpTab) listarLocal() {
	dir := t.local.caminho.Text()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.local.pend.Store(&listagem{erro: err.Error()})
		return
	}
	var itens []itemArquivo
	for _, e := range ents {
		info, err := e.Info()
		if err != nil {
			continue
		}
		itens = append(itens, itemArquivo{nome: e.Name(), dir: e.IsDir(), tam: info.Size(), mtime: info.ModTime()})
	}
	ordenar(itens)
	t.local.pend.Store(&listagem{itens: itens})
}

func (t *sftpTab) listarRemoto() {
	cli := t.cliente()
	if cli == nil {
		return
	}
	dir := t.remoto.caminho.Text()
	ents, err := cli.ReadDir(dir)
	if err != nil {
		t.remoto.pend.Store(&listagem{erro: err.Error()})
		t.invalidar()
		return
	}
	var itens []itemArquivo
	for _, e := range ents {
		itens = append(itens, itemArquivo{nome: e.Name(), dir: e.IsDir(), tam: e.Size(), mtime: e.ModTime()})
	}
	ordenar(itens)
	t.remoto.pend.Store(&listagem{itens: itens})
	t.invalidar()
}

func (t *sftpTab) navegar(p *painelArquivos, nome string) {
	base := p.caminho.Text()
	var novo string
	if p.remoto {
		novo = path.Join(base, nome)
	} else {
		novo = filepath.Join(base, nome)
	}
	p.caminho.SetText(novo)
	if p.remoto {
		go t.listarRemoto()
	} else {
		t.listarLocal()
	}
}

func (t *sftpTab) subir(p *painelArquivos) {
	base := p.caminho.Text()
	var novo string
	if p.remoto {
		novo = path.Dir(base)
	} else {
		novo = filepath.Dir(base)
	}
	if novo == base {
		return
	}
	p.caminho.SetText(novo)
	if p.remoto {
		go t.listarRemoto()
	} else {
		t.listarLocal()
	}
}

// visiveis aplica o filtro da pasta ATUAL. Nunca recursivo: procurar
// dentro de subpasta por acidente, num diretório remoto grande, custa uma
// eternidade de rede.
func (p *painelArquivos) visiveis() []itemArquivo {
	termo := strings.ToLower(strings.TrimSpace(p.filtro.Text()))
	if termo == "" {
		return p.itens
	}
	var out []itemArquivo
	for _, it := range p.itens {
		if strings.Contains(strings.ToLower(it.nome), termo) {
			out = append(out, it)
		}
	}
	return out
}

func selecionados(p *painelArquivos) []itemArquivo {
	var out []itemArquivo
	for _, it := range p.itens {
		if p.sel[it.nome] {
			out = append(out, it)
		}
	}
	return out
}

// ------------------------------------------------------- transferência

// conflitos devolve os nomes que já existem no destino. Verificar ANTES é
// o ponto: no meio da fila, "já existe" vira ou sobrescrita silenciosa ou
// uma pergunta por arquivo, e as duas coisas são ruins.
func (t *sftpTab) conflitos(itens []itemArquivo, destino string, paraRemoto bool) []string {
	var out []string
	cli := t.cliente()
	for _, it := range itens {
		if it.dir {
			// pasta é merge: o conflito que importa é arquivo a arquivo,
			// e ele é resolvido pela mesma pergunta, com a lista do que
			// vai ser substituído.
			continue
		}
		if paraRemoto {
			if cli == nil {
				continue
			}
			if _, err := cli.Stat(path.Join(destino, it.nome)); err == nil {
				out = append(out, it.nome)
			}
			continue
		}
		if _, err := os.Stat(filepath.Join(destino, it.nome)); err == nil {
			out = append(out, it.nome)
		}
	}
	return out
}

// transferir roda numa goroutine própria: copiar 200MB não pode segurar a
// interface, e o progresso precisa aparecer enquanto anda.
func (t *sftpTab) transferir(itens []itemArquivo, paraRemoto bool, pular map[string]bool, pasta string) {
	cli := t.cliente()
	if cli == nil {
		return
	}
	origem, destino := t.local.caminho.Text(), t.remoto.caminho.Text()
	if !paraRemoto {
		origem, destino = t.remoto.caminho.Text(), t.local.caminho.Text()
	}
	if pasta != "" {
		destino = pasta // soltou em cima de uma pasta do outro lado
	}

	t.mu.Lock()
	t.ocupado = true
	t.mu.Unlock()
	defer func() {
		t.mu.Lock()
		t.ocupado, t.progresso = false, ""
		t.progFeito, t.progTotal, t.progArquivo, t.progI, t.progN = 0, 0, "", 0, 0
		t.mu.Unlock()
		if paraRemoto {
			t.listarRemoto()
		} else {
			t.listarLocal()
		}
		t.invalidar()
	}()

	// Total a transferir, para a barra saber a fração. Pasta entra
	// inteira: a transferência de pasta é MERGE — cria o que falta e
	// mantém o que já existe, sem apagar a estrutura do destino.
	var total, feito int64
	arquivos := t.planejar(itens, origem, destino, paraRemoto, pular)
	for _, a := range arquivos {
		total += a.tam
	}

	t.mu.Lock()
	t.progTotal, t.progN, t.progInicio = total, len(arquivos), time.Now()
	t.mu.Unlock()

	enviados := 0
	for i, a := range arquivos {
		if t.encerrada() {
			return
		}
		t.mu.Lock()
		t.progresso = fmt.Sprintf("%s — %s de %s", a.rel, tamanhoBytes(feito), tamanhoBytes(total))
		t.progArquivo, t.progI, t.progFeito = a.rel, i+1, feito
		t.mu.Unlock()
		t.invalidar()

		var err error
		andou := func(n int64) {
			feito += n
			t.mu.Lock()
			t.progresso = fmt.Sprintf("%s — %s de %s", a.rel, tamanhoBytes(feito), tamanhoBytes(total))
			t.progFeito = feito
			t.mu.Unlock()
		}
		if paraRemoto {
			err = copiarParaRemoto(cli, a.origem, a.destino, andou)
		} else {
			err = copiarParaLocal(cli, a.origem, a.destino, andou)
		}
		if err != nil {
			t.definirMsg(fmt.Sprintf("%s: %v", a.rel, err), true)
			return
		}
		enviados++
	}
	t.definirMsg(fmt.Sprintf("%d arquivo(s) transferido(s)", enviados), false)
}

func copiarParaRemoto(cli *sftp.Client, origem, destino string, andou func(int64)) error {
	f, err := os.Open(origem)
	if err != nil {
		return err
	}
	defer f.Close()
	d, err := cli.Create(destino)
	if err != nil {
		return err
	}
	defer d.Close()
	if _, err := io.Copy(d, &leitorContado{r: f, andou: andou}); err != nil {
		return err
	}
	// Fechar aqui, e conferir o erro, e não só no defer: é no flush final
	// que um destino sem espaço (disco/share cheio bem no último bloco)
	// costuma falhar — depois que io.Copy já retornou nil. Sem isto o app
	// dizia "transferido" com o arquivo truncado do outro lado.
	return d.Close()
}

func copiarParaLocal(cli *sftp.Client, origem, destino string, andou func(int64)) error {
	f, err := cli.Open(origem)
	if err != nil {
		return err
	}
	defer f.Close()
	d, err := os.Create(destino)
	if err != nil {
		return err
	}
	defer d.Close()
	if _, err := io.Copy(d, &leitorContado{r: f, andou: andou}); err != nil {
		return err
	}
	// Ver o comentário em copiarParaRemoto: mesma corrida, destino local.
	return d.Close()
}

// leitorContado avisa quanto já passou. Transferir 200MB sem dizer em que
// pé está é o mesmo que travar, do ponto de vista de quem espera.
type leitorContado struct {
	r     io.Reader
	andou func(int64)
}

func (l *leitorContado) Read(p []byte) (int, error) {
	n, err := l.r.Read(p)
	if n > 0 && l.andou != nil {
		l.andou(int64(n))
	}
	return n, err
}

func (t *sftpTab) definirMsg(s string, erro bool) {
	t.mu.Lock()
	t.msg, t.msgErro = s, erro
	t.mu.Unlock()
	t.invalidar()
}

// levantar conta o que será apagado, recursivamente. O número é o que a
// confirmação mostra: "3 arquivos" e "1 pasta com 400 dentro" são coisas
// muito diferentes para quem vai clicar.
func (t *sftpTab) levantar(p *painelArquivos, itens []itemArquivo) (arquivos, pastas int) {
	cli := t.cliente()
	base := p.caminho.Text()
	var anda func(dir string)
	anda = func(dir string) {
		var ents []os.FileInfo
		if p.remoto {
			if cli == nil {
				return
			}
			l, err := cli.ReadDir(dir)
			if err != nil {
				return
			}
			ents = l
		} else {
			l, err := os.ReadDir(dir)
			if err != nil {
				return
			}
			for _, e := range l {
				// Link para pasta NÃO é pasta para esta conta: apagar
				// segue o os.RemoveAll, que remove o link e não entra.
				if info, err := e.Info(); err == nil {
					ents = append(ents, info)
				}
			}
		}
		for _, e := range ents {
			if e.IsDir() {
				pastas++
				anda(juntarLado(dir, e.Name(), p.remoto))
				continue
			}
			arquivos++
		}
	}
	for _, it := range itens {
		if it.dir {
			pastas++
			anda(juntarLado(base, it.nome, p.remoto))
			continue
		}
		arquivos++
	}
	return
}

// juntarLado junta caminho com a separação do lado: o remoto é sempre
// POSIX, o local segue o sistema.
func juntarLado(base, nome string, remoto bool) string {
	if remoto {
		return path.Join(base, nome)
	}
	return filepath.Join(base, nome)
}

// excluirLocal apaga na máquina local, depois da mesma confirmação com
// contagem que o remoto pede.
func (t *sftpTab) excluirLocal(itens []itemArquivo) {
	base := t.local.caminho.Text()
	for _, it := range itens {
		if err := os.RemoveAll(filepath.Join(base, it.nome)); err != nil {
			t.definirMsg(err.Error(), true)
			break
		}
	}
	t.listarLocal()
	t.invalidar()
}

func (t *sftpTab) excluirRemoto(itens []itemArquivo) {
	cli := t.cliente()
	if cli == nil {
		return
	}
	base := t.remoto.caminho.Text()
	var apagarArvore func(p string) error
	apagarArvore = func(p string) error {
		ents, err := cli.ReadDir(p)
		if err != nil {
			return err
		}
		for _, e := range ents {
			alvo := path.Join(p, e.Name())
			if e.IsDir() {
				if err := apagarArvore(alvo); err != nil {
					return err
				}
				continue
			}
			if err := cli.Remove(alvo); err != nil {
				return err
			}
		}
		// de baixo para cima: pasta só sai vazia
		return cli.RemoveDirectory(p)
	}
	for _, it := range itens {
		alvo := path.Join(base, it.nome)
		var err error
		if it.dir {
			err = apagarArvore(alvo)
		} else {
			err = cli.Remove(alvo)
		}
		if err != nil {
			t.definirMsg(err.Error(), true)
			break
		}
	}
	t.listarRemoto()
}

// ------------------------------------------------------------- desenho

func (t *sftpTab) EstadoSessao() estadoSessao {
	t.mu.Lock()
	viva, estado, prog := t.cli != nil, t.estado, t.progresso
	t.mu.Unlock()
	e := estadoSessao{
		Chip: "AGUARDE", Tipo: "neutro",
		Texto: fmt.Sprintf("%s@%s:%d", t.user, t.host, t.porta),
		Geo:   prog,
	}
	switch {
	case viva:
		e.Chip, e.Tipo = "ATIVO", "ok"
	case estado != "" && estado != "conectando…":
		e.Chip, e.Tipo = "ERRO", "erro"
	}
	return e
}

func (t *sftpTab) ControlesSessao(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if t.btnRec.Clicked(gtx) {
		go t.conectar()
	}
	if t.btnHost.Clicked(gtx) && t.trocarHost != nil {
		t.trocarHost()
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			// trocar de máquina sem fechar a aba: o painel local (caminho,
			// filtro, seleção) continua onde estava, que é o ponto de
			// mandar o mesmo arquivo para várias caixas.
			return botaoSessao(gtx, th, &t.btnHost, "Trocar máquina")
		}),
		layout.Rigid(layout.Spacer{Width: 6}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return botaoSessao(gtx, th, &t.btnRec, "Reconectar")
		}),
	)
}

// ApontarPara troca o destino desta aba e reconecta.
//
// Fecha a conexão ANTERIOR em segundo plano, não aqui: isto é chamado do
// laço de quadro (o clique na lista de "trocar máquina" dispara direto
// dali), e cli.Close()/sc.Close() são chamadas de rede — se a máquina
// antiga estiver com a conexão pendurada (inalcançável, mas o TCP ainda
// não percebeu), a interface inteira congelava até a chamada retornar.
func (t *sftpTab) ApontarPara(nome, host string, porta int, usuario, senha string) {
	t.mu.Lock()
	cli, sc := t.cli, t.ssh
	t.cli, t.ssh = nil, nil
	t.fechado = false
	t.host, t.porta, t.user, t.senha = host, porta, usuario, senha
	t.titulo = nome
	t.estado = "conectando…"
	t.mu.Unlock()
	if cli != nil {
		go cli.Close()
	}
	if sc != nil {
		go sc.Close()
	}
	go t.conectar()
}

// aplicarListagens troca o conteúdo dos painéis, no laço de quadro e só
// aqui. Depois disto, nada mais mexe em itens/btnItem/sel durante o
// quadro — que é a condição para o laço de cliques poder confiar nos
// índices que está percorrendo.
// invalidar existe para a aba poder ser exercitada sem janela (testes).
// A insistência contra a guarda mayInvalidate do Gio — e o limite de uma
// rajada de retries por vez, mesmo com a goroutine de transferência
// chamando isto a cada pedaço copiado — está em invalidador (ver
// invalidar.go, compartilhado com sshTab).
func (t *sftpTab) invalidar() {
	t.disparar(t.w)
}

func (t *sftpTab) aplicarListagens() {
	for _, p := range []*painelArquivos{&t.local, &t.remoto} {
		l := p.pend.Swap(nil)
		if l == nil {
			continue
		}
		p.erro = l.erro
		if l.erro != "" {
			continue // erro de leitura não apaga o que estava na tela
		}
		p.itens = l.itens
		p.btnItem = make([]widget.Clickable, len(l.itens))
		p.sel = map[string]bool{}
	}
}

// comCtrl diz se o clique veio com Ctrl. São DUAS fontes de propósito: no
// Linux o teclado vem do Wayland cru (internal/grab), e o Modifiers do
// Gio chega vazio nesta pilha; no Windows não há grab nenhum, e o
// Modifiers do Gio é a única fonte. Perguntar aos dois faz o gesto
// funcionar nos dois lugares.
func comCtrl(ev widget.Click) bool {
	return ctrlPressionado() || ev.Modifiers.Contain(key.ModCtrl)
}

func (t *sftpTab) iniciarTransferencia(itens []itemArquivo, paraRemoto bool) {
	t.iniciarTransferenciaPara(itens, paraRemoto, "")
}

// iniciarTransferenciaPara manda para uma pasta escolhida do outro lado
// (soltar em cima dela); pasta vazia é a pasta aberta no outro painel.
func (t *sftpTab) iniciarTransferenciaPara(itens []itemArquivo, paraRemoto bool, pasta string) {
	// Nada aqui pode falhar calado: transferência que "não vai" sem dizer
	// por quê é pior que erro na cara.
	if len(itens) == 0 {
		lado := "nesta máquina"
		if !paraRemoto {
			lado = "no remoto"
		}
		t.definirMsg("selecione os arquivos "+lado+" (clique no nome)", true)
		return
	}
	if t.cliente() == nil {
		t.definirMsg("sem conexão SFTP", true)
		return
	}

	t.mu.Lock()
	ocupado := t.ocupado
	t.mu.Unlock()
	if ocupado {
		t.definirMsg("aguarde a transferência em andamento", true)
		return
	}
	destino := t.remoto.caminho.Text()
	if !paraRemoto {
		destino = t.local.caminho.Text()
	}
	if pasta != "" {
		destino = pasta
	}
	conf := t.conflitos(itens, destino, paraRemoto)
	if len(conf) == 0 {
		go t.transferir(itens, paraRemoto, nil, pasta)
		return
	}
	// três saídas, como no original: sobrescrever, pular os existentes
	// (os demais vão assim mesmo) ou cancelar tudo.
	abrirDialogo(&dlgConflito{
		w: t.w, nomes: conf,
		sobrescrever: func() { go t.transferir(itens, paraRemoto, nil, pasta) },
		pular: func() {
			pular := map[string]bool{}
			for _, n := range conf {
				pular[n] = true
			}
			go t.transferir(itens, paraRemoto, pular, pasta)
		},
	})
}

// tamanhoHumano: pasta não mostra tamanho (o do inode não diz nada).
func tamanhoHumano(it itemArquivo) string {
	if it.dir {
		return ""
	}
	n := float64(it.tam)
	for _, u := range []string{"B", "KB", "MB", "GB"} {
		if n < 1024 {
			return fmt.Sprintf("%.0f %s", n, u)
		}
		n /= 1024
	}
	return fmt.Sprintf("%.1f TB", n)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// arquivoPlano é uma cópia a fazer, já com origem e destino resolvidos.
type arquivoPlano struct {
	origem, destino, rel string
	tam                  int64
}

// planejar percorre a seleção e devolve a lista de ARQUIVOS a copiar,
// criando as pastas do destino pelo caminho. Pasta é copiada inteira, em
// MERGE: o que já existe do outro lado fica, o que falta é criado — nunca
// se apaga a estrutura do destino para depois recriar.
func (t *sftpTab) planejar(itens []itemArquivo, origem, destino string, paraRemoto bool, pular map[string]bool) []arquivoPlano {
	cli := t.cliente()
	var plano []arquivoPlano

	criarDir := func(p string) {
		if paraRemoto {
			cli.MkdirAll(p)
			return
		}
		os.MkdirAll(p, 0o755)
	}

	var anda func(orig, dest, rel string)
	anda = func(orig, dest, rel string) {
		criarDir(dest)
		var nomes []itemArquivo
		if paraRemoto {
			ents, err := os.ReadDir(orig)
			if err != nil {
				return
			}
			for _, e := range ents {
				info, err := e.Info()
				if err != nil {
					continue
				}
				nomes = append(nomes, itemArquivo{nome: e.Name(), dir: e.IsDir(), tam: info.Size()})
			}
		} else {
			ents, err := cli.ReadDir(orig)
			if err != nil {
				return
			}
			for _, e := range ents {
				nomes = append(nomes, itemArquivo{nome: e.Name(), dir: e.IsDir(), tam: e.Size()})
			}
		}
		for _, it := range nomes {
			o, d := juntar(orig, it.nome, paraRemoto, true), juntar(dest, it.nome, paraRemoto, false)
			if it.dir {
				anda(o, d, rel+"/"+it.nome)
				continue
			}
			plano = append(plano, arquivoPlano{origem: o, destino: d, rel: rel + "/" + it.nome, tam: it.tam})
		}
	}

	for _, it := range itens {
		if pular[it.nome] {
			continue
		}
		o, d := juntar(origem, it.nome, paraRemoto, true), juntar(destino, it.nome, paraRemoto, false)
		if it.dir {
			anda(o, d, it.nome)
			continue
		}
		plano = append(plano, arquivoPlano{origem: o, destino: d, rel: it.nome, tam: it.tam})
	}
	return plano
}

// juntar monta caminho com a separação certa de cada lado: o remoto é
// sempre POSIX, o local segue o sistema.
func juntar(base, nome string, paraRemoto, ladoOrigem bool) string {
	remoto := paraRemoto != ladoOrigem // origem remota quando não é envio
	if remoto {
		return path.Join(base, nome)
	}
	return filepath.Join(base, nome)
}

func tamanhoBytes(n int64) string {
	v := float64(n)
	for _, u := range []string{"B", "KB", "MB", "GB"} {
		if v < 1024 {
			return fmt.Sprintf("%.0f %s", v, u)
		}
		v /= 1024
	}
	return fmt.Sprintf("%.1f TB", v)
}
