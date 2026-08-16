// Package vpngate recusa API do xchat se a VPN não estiver no ar ou se
// *.corp não resolver para o gateway do túnel (PLAN.md §5 / Fase 23).
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
)

var (
	ErrVPNDisconnected = errors.New("xvpn desconectado — conecte a VPN antes de usar o xchat")
	ErrCorpDNS         = errors.New("DNS de *.corp.ihuull.com não aponta para 10.66.66.1 — a intranet só existe dentro do túnel")
)

type Status struct {
	Connected bool `json:"connected"`
}

// Check exige helper conectado (quando o socket existe) e DNS corp = 10.66.66.1.
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
	ip, err := lookupCorp()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrCorpDNS, err)
	}
	if ip != CorpIP {
		return fmt.Errorf("%w (obteve %s)", ErrCorpDNS, ip)
	}
	return nil
}

func lookupCorp() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip4", CorpHost)
	if err != nil {
		return "", err
	}
	if len(ips) == 0 {
		return "", errors.New("sem A")
	}
	return ips[0].String(), nil
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
