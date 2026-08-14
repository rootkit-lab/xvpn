//go:build linux

package linux

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
)

// killSwitchTable é isolada num nome próprio pra nunca colidir com regras
// de firewall que o usuário já tenha configurado por fora — ver
// ROADMAP.md Fase 6.
const killSwitchTable = "xvpn_killswitch"

// enableKillSwitch aplica um bloqueio fail-closed via nftables: toda saída
// é dropada por padrão, exceto loopback, o próprio túnel (ifaceName) e o
// tráfego para o servidor (necessário pro handshake WireGuard conseguir
// reconectar depois de uma queda — sem essa exceção o próprio mecanismo de
// reconexão automática da Fase 6 ficaria travado). Idempotente: sempre
// recria a tabela do zero, então pode ser chamado de novo (ex.: endpoint
// do servidor mudou) sem acumular regras duplicadas.
func enableKillSwitch(serverIP net.IP) error {
	_ = disableKillSwitch()

	ruleset := fmt.Sprintf(`table inet %s {
	chain output {
		type filter hook output priority filter; policy drop;
		oifname "lo" accept
		oifname %q accept
		ip daddr %s accept
	}
}
`, killSwitchTable, ifaceName, serverIP.String())

	cmd := exec.Command("nft", "-f", "-")
	cmd.Stdin = strings.NewReader(ruleset)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("aplicando regras do kill switch (nft): %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// disableKillSwitch remove a tabela do kill switch — no-op (sem erro) se
// ela não existir, para poder ser chamada incondicionalmente em qualquer
// teardown.
func disableKillSwitch() error {
	out, err := exec.Command("nft", "delete", "table", "inet", killSwitchTable).CombinedOutput()
	if err != nil && !strings.Contains(string(out), "No such file") {
		return fmt.Errorf("removendo regras do kill switch (nft): %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}
