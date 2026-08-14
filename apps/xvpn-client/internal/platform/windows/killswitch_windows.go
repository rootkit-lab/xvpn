//go:build windows

// Kill switch para Windows via Windows Filtering Platform (WFP), usando o
// wrapper github.com/tailscale/wf (mesma biblioteca usada em produção
// pelo cliente Windows da Tailscale). O desenho segue de perto o kill
// switch de referência do wireguard-windows oficial
// (tunnel/firewall/blocker.go no repositório WireGuard/wireguard-windows):
// sessão WFP dinâmica (Dynamic: true na Options) — se o processo helper
// morrer ou crashar, o próprio kernel do Windows remove todos os filtros
// automaticamente, então uma falha do helper nunca deixa a máquina sem
// internet travada permanentemente.
//
// AVISO (ver cabeçalho de engine_windows.go): assim como o resto deste
// pacote, nunca foi executado num Windows real — só validado por
// cross-compilation (GOOS=windows). Precisa de teste manual antes de
// confiar nisto em produção, ver ROADMAP.md Fase 6.
package windows

import (
	"fmt"
	"net/netip"

	"github.com/tailscale/wf"
	"golang.org/x/sys/windows"
)

// killSwitch encapsula a sessão WFP — fechá-la remove todos os filtros
// que ela criou (sublayer + regras), não precisa deletar um por um.
type killSwitch struct {
	session *wf.Session
}

// enableKillSwitch abre uma sessão WFP dinâmica e instala, nas camadas
// ALE_AUTH_CONNECT v4/v6 (toda conexão TCP/UDP de saída iniciada por
// qualquer processo da máquina): permitir loopback, permitir a interface
// do túnel (tunLUID), permitir o endpoint do servidor (necessário pro
// próprio handshake WireGuard reconseguir se conectar depois de uma
// queda — ver internal/helper, reconexão automática) e, com peso menor
// que todos os anteriores, bloquear o resto.
func enableKillSwitch(tunLUID uint64, serverIP netip.Addr) (*killSwitch, error) {
	session, err := wf.New(&wf.Options{Name: "XVPN kill switch", Dynamic: true})
	if err != nil {
		return nil, fmt.Errorf("abrindo sessão WFP: %w", err)
	}
	if err := installKillSwitchRules(session, tunLUID, serverIP); err != nil {
		session.Close()
		return nil, err
	}
	return &killSwitch{session: session}, nil
}

// disable fecha a sessão WFP, removendo todas as regras instaladas — é
// seguro chamar em um *killSwitch nil (ex.: kill switch nunca foi
// ativado).
func (k *killSwitch) disable() error {
	if k == nil || k.session == nil {
		return nil
	}
	return k.session.Close()
}

func installKillSwitchRules(session *wf.Session, tunLUID uint64, serverIP netip.Addr) error {
	providerGUID, err := windows.GenerateGUID()
	if err != nil {
		return fmt.Errorf("gerando GUID do provider WFP: %w", err)
	}
	provider := &wf.Provider{ID: wf.ProviderID(providerGUID), Name: "XVPN"}
	if err := session.AddProvider(provider); err != nil {
		return fmt.Errorf("registrando provider WFP: %w", err)
	}

	sublayerGUID, err := windows.GenerateGUID()
	if err != nil {
		return fmt.Errorf("gerando GUID do sublayer WFP: %w", err)
	}
	sublayer := &wf.Sublayer{
		ID:       wf.SublayerID(sublayerGUID),
		Name:     "XVPN kill switch",
		Provider: provider.ID,
		// Weight máximo: nossas regras devem ser avaliadas antes de
		// qualquer sublayer padrão do Windows Firewall.
		Weight: 0xFFFF,
	}
	if err := session.AddSublayer(sublayer); err != nil {
		return fmt.Errorf("registrando sublayer WFP: %w", err)
	}

	for _, layer := range []wf.LayerID{wf.LayerALEAuthConnectV4, wf.LayerALEAuthConnectV6} {
		if err := addKillSwitchRule(session, provider.ID, sublayer.ID, layer, "loopback", 15, wf.ActionPermit,
			[]*wf.Match{{Field: wf.FieldFlags, Op: wf.MatchTypeFlagsAnySet, Value: wf.ConditionFlagIsLoopback}}); err != nil {
			return err
		}
		if err := addKillSwitchRule(session, provider.ID, sublayer.ID, layer, "tunnel interface", 14, wf.ActionPermit,
			[]*wf.Match{{Field: wf.FieldIPLocalInterface, Op: wf.MatchTypeEqual, Value: tunLUID}}); err != nil {
			return err
		}
		if err := addKillSwitchRule(session, provider.ID, sublayer.ID, layer, "server endpoint", 13, wf.ActionPermit,
			[]*wf.Match{{Field: wf.FieldIPRemoteAddress, Op: wf.MatchTypeEqual, Value: serverIP}}); err != nil {
			return err
		}
		if err := addKillSwitchRule(session, provider.ID, sublayer.ID, layer, "block all (kill switch)", 0, wf.ActionBlock, nil); err != nil {
			return err
		}
	}
	return nil
}

func addKillSwitchRule(session *wf.Session, providerID wf.ProviderID, sublayerID wf.SublayerID, layer wf.LayerID, name string, weight uint64, action wf.Action, conditions []*wf.Match) error {
	ruleGUID, err := windows.GenerateGUID()
	if err != nil {
		return fmt.Errorf("gerando GUID da regra %q: %w", name, err)
	}
	rule := &wf.Rule{
		ID:         wf.RuleID(ruleGUID),
		Name:       "XVPN: " + name,
		Layer:      layer,
		Sublayer:   sublayerID,
		Weight:     weight,
		Conditions: conditions,
		Action:     action,
		Provider:   providerID,
	}
	if err := session.AddRule(rule); err != nil {
		return fmt.Errorf("instalando regra WFP %q: %w", name, err)
	}
	return nil
}
