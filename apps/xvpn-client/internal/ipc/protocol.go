// Package ipc implementa o canal JSON-RPC entre a GUI (processo sem
// privilégio) e o helper (processo privilegiado que manipula TUN/rotas) —
// ver .cursor/rules/go-client.mdc. Transporte: Unix Domain Socket no Linux,
// Named Pipe no Windows (arquivos com build tag _linux.go/_windows.go);
// este arquivo contém só o protocolo, independente de plataforma.
package ipc

import "encoding/json"

// Request e Response seguem o formato JSON-RPC 2.0 (sem batching, sem
// notificações — não há necessidade disso aqui), uma requisição por linha
// (newline-delimited JSON) sobre a conexão.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      uint64          `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      uint64          `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// Métodos RPC expostos pelo helper — ver internal/helper.
const (
	MethodStatus     = "status"
	MethodEnroll     = "enroll"
	MethodConnect    = "connect"
	MethodDisconnect = "disconnect"
	MethodIsEnrolled = "is_enrolled"

	// MethodGetPreferences/MethodSetPreferences — preferências de conexão
	// (kill switch, split-tunnel, reconexão automática), ver
	// ROADMAP.md Fase 6. Persistidas em config.DeviceState, por isso só o
	// helper as lê/escreve.
	MethodGetPreferences = "get_preferences"
	MethodSetPreferences = "set_preferences"

	// MethodGetLogs devolve as últimas linhas do log do helper (buffer em
	// memória, ver internal/helper/logbuffer.go) — usado pela página de
	// diagnóstico da GUI.
	MethodGetLogs = "get_logs"
)
