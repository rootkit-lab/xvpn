//go:build linux

package config

// defaultStatePath: /var/lib é o local convencional para estado
// persistente de serviço no Linux (não /etc, que é pra config fornecida
// pelo administrador — a chave privada gerada aqui não se encaixa nisso).
// O diretório é criado pelo próprio systemd via StateDirectory=xvpn-client
// (ver deploy/systemd/xvpn-client-helper.service) com o dono correto antes
// do helper subir — ReadWritePaths manual pra um caminho que ainda não
// existe causava falha "226/NAMESPACE" no start, achado ao instalar de
// verdade fora do Docker na Fase 4. Diretório/arquivo ficam 0700/0600: só
// o usuário que roda o helper (xvpn-client-helper) consegue ler; nem a
// GUI, sem privilégio, nem outros usuários locais.
func defaultStatePath() string {
	return "/var/lib/xvpn-client/device.json"
}
