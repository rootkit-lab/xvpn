// Package opener abre URLs/compartilhamentos de rede no aplicativo padrão
// do SO (navegador, gerenciador de arquivos). Roda inteiramente no processo
// GUI sem privilégio — não toca rede/TUN/rotas, então não se sujeita à
// restrição de isolar runtime.GOOS do .cursor/rules/go-client.mdc; ainda
// assim, cada SO tem sua própria implementação por clareza e consistência
// com o resto do pacote internal/platform.
package opener

// OpenURL abre uma URL (http/https) no navegador padrão do sistema.
func OpenURL(url string) error {
	return openURL(url)
}

// OpenSMBShare abre o compartilhamento SMB especificado no gerenciador de
// arquivos padrão do SO — Explorer via caminho UNC no Windows; no Linux
// prefere o mount CIFS do helper em ~/XVPN e cai no GVFS anônimo (Cosmic
// Files não trata smb:// como pasta — ver opener_linux.go).
func OpenSMBShare(host, share string) error {
	return openSMBShare(host, share)
}

// EnsureSMBMounted prepara o share no SO (no Linux: CIFS se já montado,
// senão gio mount --anonymous) sem abrir o gerenciador. Chamado após
// Connect para os mounts já existirem quando o usuário clicar.
func EnsureSMBMounted(host, share string) error {
	return ensureSMBMounted(host, share)
}

// UnmountServerSMBShares desmonta todos os shares SMB desse host no SO
// (GVFS no Linux), remove atalhos ~/XVPN e restos de ícones/pastas no
// Desktop. Chamado no Disconnect e quando a bandeja detecta queda do túnel.
func UnmountServerSMBShares(host string) error {
	return unmountServerSMBShares(host)
}

// OpenPath abre um arquivo ou pasta local no aplicativo/gerenciador de
// arquivos padrão do SO (ex.: um instalador baixado do marketplace, Fase
// 12 — ROADMAP.md, ou a própria pasta de Downloads). Reaproveita o mesmo
// mecanismo de OpenURL (xdg-open no Linux e "start" no Windows tratam
// caminhos de arquivo/pasta locais e URLs de forma idêntica) só com um
// nome que não confunde quem lê o código chamador com um caminho local.
func OpenPath(path string) error {
	return openURL(path)
}
