// Package vpngate recusa API do xchat se a VPN não estiver no ar ou se
// o gateway da intranet (10.66.66.1:443) não responder (PLAN.md §5 / Fase 23).
//
// Não usa o DNS do sistema: systemd-resolved e DoH frequentemente ignoram
// /etc/hosts e o dnsmasq do túnel, o que gerava "no such host" com a VPN
// já conectada. HTTP/WS discam 10.66.66.1 e mantêm o SNI *.corp.
package vpngate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"time"
)

const (
	HelperSocket = "/run/xvpn-client/helper.sock"
	CorpHost     = "xchat.corp.ihuull.com"
	CorpIP       = "10.66.66.1"
	CorpPort     = "443"
)

var (
	ErrVPNDisconnected = errors.New("xvpn desconectado — conecte a VPN antes de usar o xchat")
	ErrCorpUnreachable = errors.New("intranet não alcançável em 10.66.66.1:443 — confirme que o túnel xvpn está no ar")
	// ErrCorpDNS is kept for callers that still match the old gate error.
	ErrCorpDNS = ErrCorpUnreachable
)

type Status struct {
	Connected bool `json:"connected"`
}

// Check exige helper conectado (quando o socket existe) e TCP no gateway corp.
// Sem helper (CI, testes, web) o gate não bloqueia. XVPN_SKIP_VPNGATE=1 libera sempre.
func Check() error {
	if os.Getenv("XVPN_SKIP_VPNGATE") == "1" {
		return nil
	}
	if _, err := os.Stat(HelperSocket); err != nil {
		return nil
	}
	st, err := helperStatus()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrVPNDisconnected, err)
	}
	if !st.Connected {
		return ErrVPNDisconnected
	}
	if err := probeGateway(); err != nil {
		return fmt.Errorf("%w: %v", ErrCorpUnreachable, err)
	}
	return nil
}

func probeGateway() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, err := DialContext(ctx, "tcp", net.JoinHostPort(CorpIP, CorpPort))
	if err != nil {
		return err
	}
	return conn.Close()
}

func helperStatus() (Status, error) {
	conn, err := net.DialTimeout("unix", HelperSocket, 2*time.Second)
	if err != nil {
		return Status{}, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if err := json.NewEncoder(conn).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "status"}); err != nil {
		return Status{}, err
	}
	var resp struct {
		Result json.RawMessage `json:"result"`
		Error  string          `json:"error"`
	}
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return Status{}, err
	}
	if resp.Error != "" {
		return Status{}, errors.New(resp.Error)
	}
	var st Status
	if err := json.Unmarshal(resp.Result, &st); err != nil {
		return Status{}, err
	}
	return st, nil
}
