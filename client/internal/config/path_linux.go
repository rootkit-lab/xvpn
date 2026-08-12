//go:build linux

package config

// defaultStatePath: /etc é o local convencional para estado de serviço de
// sistema no Linux; o diretório e o arquivo ficam 0700/0600 (só o usuário
// que roda o helper, xvpn-client-helper — ver
// deploy/systemd/xvpn-client-helper.service — consegue ler; nem a GUI, que
// roda sem privilégio, nem outros usuários locais).
func defaultStatePath() string {
	return "/etc/xvpn-client/device.json"
}
