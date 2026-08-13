// Package autostart liga/desliga a inicialização automática da GUI do
// XVPN junto com o login do sistema. Roda inteiramente no processo GUI
// sem privilégio (nunca no helper) — não toca TUN/rotas/firewall, só
// escreve um atalho/entrada de registro no espaço do usuário atual, ver
// .cursor/rules/go-client.mdc e ROADMAP.md Fase 6.
package autostart

// IsEnabled reporta se a inicialização automática está configurada para
// o executável atual.
func IsEnabled() (bool, error) {
	removeLegacyAutostartCopy()
	return isEnabled()
}

// SetEnabled liga ou desliga a inicialização automática.
func SetEnabled(enabled bool) error {
	removeLegacyAutostartCopy()
	return setEnabled(enabled)
}
