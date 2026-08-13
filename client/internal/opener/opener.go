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
// arquivos padrão do SO — Explorer via caminho UNC no Windows,
// GVFS/Nautilus/Dolphin via URI smb:// no Linux.
func OpenSMBShare(host, share string) error {
	return openSMBShare(host, share)
}
