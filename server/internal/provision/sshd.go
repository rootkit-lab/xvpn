package provision

import "fmt"

// sshdDropInPath é o caminho do fragmento de sshd_config que habilita
// SFTP chrootado para um usuário. Um arquivo por usuário, sob o
// diretório de includes do sshd (ver /etc/ssh/sshd_config.d/). O nome
// prefixa com "xvpn-" para ficar claro quem criou e facilitar a limpeza
// no disable — nunca colidindo com fragmentos de outros pacotes.
func sshdDropInPath(username string) string {
	return fmt.Sprintf("/etc/ssh/sshd_config.d/xvpn-sftp-%s.conf", username)
}

// SSHDMatchConfig devolve o conteúdo do fragmento de sshd_config que
// força o usuário a SFTP-only chrootado, sem shell e sem senha (só
// chave pública — ver PLAN.md §6.9). Função pura: dado o username,
// devolve o texto exato que vai em disco. Testável sem root.
//
// AuthorizedKeysFile aponta para o caminho absoluto dentro do que será
// a raiz do chroot; o sshd lê esse arquivo ANTES de aplicar o chroot
// (a autenticação acontece pré-chroot), então o caminho absoluto
// /home/<username>/.ssh/authorized_keys funciona mesmo com o usuário
// sem poder escrever no próprio /home/<username>/ (raiz root:root,
// exigência do chroot).
func SSHDMatchConfig(username string) string {
	return fmt.Sprintf(`# Gerado por xvpn-user-provision (Fase 13, PLAN.md §6.9).
# NÃO editar manualmente — use o painel para ligar/desligar o acesso.
Match User %s
    ForceCommand internal-sftp
    ChrootDirectory /home/%s
    PubkeyAuthentication yes
    PasswordAuthentication no
    AuthorizedKeysFile /home/%s/.ssh/authorized_keys
`, username, username, username)
}

// authorizedKeysPath devolve o caminho do authorized_keys do usuário,
// dentro da raiz do chroot (root:root, escrita pelo binário privilegiado
// a partir da chave pública colada no painel — o usuário nunca toca
// nesse arquivo).
func authorizedKeysPath(username string) string {
	return fmt.Sprintf("/home/%s/.ssh/authorized_keys", username)
}
