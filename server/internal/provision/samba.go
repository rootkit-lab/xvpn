package provision

import "fmt"

// sambaIncludeDir é o diretório de includes do smb.conf. O smb.conf
// principal precisa ter uma linha `include = /etc/samba/smb.conf.d/` (ou
// `include = /etc/samba/smb.conf.d/%I.conf`) para que os fragmentos
// por usuário sejam carregados — isso é configurado no deploy da Fase
// 13 (ver ROADMAP.md), não pelo binário.
const sambaIncludeDir = "/etc/samba/smb.conf.d"

// sambaShareIncludePath devolve o caminho do fragmento de smb.conf que
// define o share [home-<username>] apontando para /home/<username>/files.
// Um arquivo por usuário, prefixado "xvpn-" para identificação e limpeza.
func sambaShareIncludePath(username string) string {
	return fmt.Sprintf("%s/xvpn-home-%s.conf", sambaIncludeDir, username)
}

// SambaShareConfig devolve o conteúdo do fragmento de smb.conf que
// define o share [home-<username>]. Função pura, testável sem root.
//
// Autenticação Samba (decisão Fase 13, ver ROADMAP.md): guest + force
// user, confiando no VPN (wg0) como barreira de auth — o share só é
// alcançável via 10.66.66.1 (bind exclusivo wg0, ver Fase 5), e a
// própria VPN já é autenticada (enrollment de dispositivo). Sem senha
// Samba = sem novo segredo para o usuário decorar/relay. force user
// garante que os arquivos sejam gravados com o dono Unix correto
// (<username>), mesmo vindo de uma conexão guest.
func SambaShareConfig(username string) string {
	return fmt.Sprintf(`# Gerado por xvpn-user-provision (Fase 13, PLAN.md §6.9).
# NÃO editar manualmente — use o painel para ligar/desligar o acesso.
[home-%s]
    path = /home/%s/files
    browseable = yes
    guest ok = yes
    force user = %s
    writable = yes
`, username, username, username)
}
