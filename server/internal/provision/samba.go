package provision

import (
	"fmt"
	"sort"
	"strings"
)

// sambaIncludeDir é o diretório de includes do smb.conf. O smb.conf
// principal precisa ter uma linha `include = /etc/samba/smb.conf.d/
// xvpn-shares.conf` para que os shares por usuário sejam carregados
// — isso é configurado no deploy da Fase 13 (ver ROADMAP.md), não
// pelo binário.
//
// Nota: o `include` do Samba NÃO suporta glob (*.conf) como o Include
// do sshd — ele inclui um único arquivo. Por isso o binário mantém um
// agregado (xvpn-shares.conf) que lista um `include =` por share
// per-user; o smb.conf inclui só esse agregado.
const sambaIncludeDir = "/etc/samba/smb.conf.d"

// sambaAggregatePath é o arquivo único que o smb.conf inclui. Contém
// uma linha `include = <caminho>` para cada share per-user presente
// no diretório. Regenerado a cada enable/disable (ver
// regenerateSambaAggregate).
const sambaAggregatePath = "/etc/samba/smb.conf.d/xvpn-shares.conf"

// sambaShareIncludePath devolve o caminho do fragmento de smb.conf que
// define o share [home-<username>] apontando para /home/<username>/files.
// Um arquivo por usuário, prefixado "xvpn-home-" para identificação e limpeza.
func sambaShareIncludePath(username string) string {
	return fmt.Sprintf("%s/xvpn-home-%s.conf", sambaIncludeDir, username)
}

// regenerateSambaAggregate reescreve xvpn-shares.conf listando um
// `include =` para cada fragmento xvpn-home-*.conf presente no
// diretório. É a ponte entre os arquivos per-user (fonte da verdade,
// fáceis de adicionar/remover) e o `include` único do Samba (que não
// suporta glob). Idempotente: chamada a cada enable/disable de Samba.
//
// Ordem alfabética pra o arquivo ser determinístico (diffs limpos,
// testes reproduzíveis). Exclui o próprio agregado e qualquer arquivo
// que não case xvpn-home-*.conf.
func regenerateSambaAggregate(r Runner) error {
	names, err := r.ReadDir(sambaIncludeDir)
	if err != nil {
		return fmt.Errorf("lendo %s: %w", sambaIncludeDir, err)
	}
	var shares []string
	for _, n := range names {
		if strings.HasPrefix(n, "xvpn-home-") && strings.HasSuffix(n, ".conf") {
			shares = append(shares, n)
		}
	}
	sort.Strings(shares)
	var b strings.Builder
	b.WriteString("# Gerado por xvpn-user-provision (Fase 13, PLAN.md §6.9).\n")
	b.WriteString("# NÃO editar — regenerado a cada toggle de Samba no painel.\n")
	for _, n := range shares {
		fmt.Fprintf(&b, "include = %s/%s\n", sambaIncludeDir, n)
	}
	if err := r.WriteFile(sambaAggregatePath, b.String(), 0o644); err != nil {
		return fmt.Errorf("escrevendo %s: %w", sambaAggregatePath, err)
	}
	return nil
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
