// Package config persiste o estado do dispositivo (par de chaves, IP
// atribuído, dados do servidor) obtido no enrollment. Só o helper
// privilegiado lê/escreve esse arquivo — nunca a GUI, ver
// .cursor/rules/go-client.mdc (chave privada nunca sai do helper).
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DeviceState é o que sobra depois de um enrollment bem-sucedido, o
// suficiente para reconectar sem precisar de um novo código de convite.
type DeviceState struct {
	ServerBaseURL       string   `json:"server_base_url"`
	DeviceName          string   `json:"device_name"`
	PrivateKey          string   `json:"private_key"`
	PublicKey           string   `json:"public_key"`
	AssignedIP          string   `json:"assigned_ip"`
	ServerPublicKey     string   `json:"server_public_key"`
	ServerEndpoint      string   `json:"server_endpoint"`
	AllowedIPs          []string `json:"allowed_ips"`
	PersistentKeepalive int      `json:"persistent_keepalive_seconds"`
	DNS                 []string `json:"dns,omitempty"`
	// MTU sobrescreve o padrão da plataforma (1420) quando != 0. Alguns
	// usuários (outra VPN ativa, CGNAT, rede móvel restritiva) precisam de
	// um valor menor para evitar um "black hole" de PMTU — ver
	// ROADMAP.md Fase 1 (achado de MTU) e Fase 4.
	MTU        int       `json:"mtu,omitempty"`
	EnrolledAt time.Time `json:"enrolled_at"`
}

// Load lê o estado persistido. Devolve (nil, nil) — não um erro — se o
// dispositivo ainda não fez enrollment, para o caller distinguir
// "não enrolled" de uma falha real de leitura.
func Load() (*DeviceState, error) {
	raw, err := os.ReadFile(defaultStatePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("lendo estado do dispositivo: %w", err)
	}
	var state DeviceState
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, fmt.Errorf("estado do dispositivo corrompido em %q: %w", defaultStatePath(), err)
	}
	return &state, nil
}

// Save grava o estado com permissão restrita (só o helper, que roda com
// privilégio, deve conseguir ler — a chave privada mora aqui).
func Save(state *DeviceState) error {
	path := defaultStatePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("criando diretório de estado %q: %w", filepath.Dir(path), err)
	}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("codificando estado do dispositivo: %w", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return fmt.Errorf("gravando estado do dispositivo em %q: %w", path, err)
	}
	return nil
}

// Clear remove o estado persistido (usado por um futuro fluxo de
// "desconectar e esquecer este dispositivo").
func Clear() error {
	err := os.Remove(defaultStatePath())
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removendo estado do dispositivo: %w", err)
	}
	return nil
}
