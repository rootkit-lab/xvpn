package helper

import (
	"context"
	"log"
	"time"
)

// monitorInterval é de quanto em quanto tempo o monitor confere se o
// túnel ainda está de pé enquanto desiredConnected == true.
const monitorInterval = 5 * time.Second

// maxBackoffAttempt limita o crescimento exponencial — a partir daqui o
// intervalo já bateu o teto (ver backoffDuration).
const maxBackoffAttempt = 6

// backoffDuration cresce exponencialmente (1s, 2s, 4s, ... até 60s) a
// cada tentativa de reconexão falhada, pra não bater no servidor (ou na
// bateria/CPU do dispositivo) em loop apertado quando a queda não é
// passageira — ver ROADMAP.md Fase 6.
func backoffDuration(attempt int) time.Duration {
	if attempt > maxBackoffAttempt {
		attempt = maxBackoffAttempt
	}
	d := time.Second << uint(attempt)
	if d > 60*time.Second {
		d = 60 * time.Second
	}
	return d
}

// startMonitor (re)inicia o goroutine que observa o túnel depois de um
// Connect bem-sucedido. Assume h.mu já travado pelo caller.
func (h *Helper) startMonitor() {
	h.stopMonitorLocked()
	ctx, cancel := context.WithCancel(context.Background())
	h.monitorCancel = cancel
	go h.monitorLoop(ctx)
}

// stopMonitorLocked cancela o monitor atual, se houver, e zera o estado
// de reconexão. Assume h.mu já travado pelo caller — ver handleDisconnect.
func (h *Helper) stopMonitorLocked() {
	if h.monitorCancel != nil {
		h.monitorCancel()
		h.monitorCancel = nil
	}
	h.reconnecting = false
	h.reconnectAttempt = 0
}

func (h *Helper) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(monitorInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.checkConnection(ctx)
		}
	}
}

// checkConnection confere se o túnel caiu inesperadamente e, se sim (e a
// preferência AutoReconnect estiver ligada), tenta reconectar com
// backoff. Ver engine_linux.go/engine_windows.go: o kill switch (se
// ativo) continua bloqueando tráfego fora do túnel durante essa janela —
// esta função só cuida de tentar restabelecer o túnel em si.
func (h *Helper) checkConnection(ctx context.Context) {
	h.mu.Lock()
	if !h.desiredConnected || h.reconnecting || h.state == nil {
		h.mu.Unlock()
		return
	}
	status, err := h.engine.Status()
	dropped := err != nil || !status.Connected
	if !dropped {
		h.reconnectAttempt = 0
		h.mu.Unlock()
		return
	}
	if !h.state.Preferences.AutoReconnect {
		h.mu.Unlock()
		return
	}
	h.reconnecting = true
	attempt := h.reconnectAttempt
	h.mu.Unlock()

	backoff := backoffDuration(attempt)
	log.Printf("xvpn-client-helper: túnel caiu, tentando reconectar em %s (tentativa %d)", backoff, attempt+1)

	select {
	case <-ctx.Done():
		return
	case <-time.After(backoff):
	}

	h.mu.Lock()
	h.reconnecting = false
	if !h.desiredConnected || h.state == nil {
		h.mu.Unlock()
		return
	}
	cfg, err := h.buildTunnelConfig()
	h.mu.Unlock()
	if err != nil {
		log.Printf("xvpn-client-helper: reconexão automática abortada: %v", err)
		return
	}

	// engineMu (não h.mu) serializa a chamada ao motor — mantém
	// status/preferências/logs respondendo via IPC mesmo durante uma
	// reconexão automática lenta (ver ROADMAP.md Fase 9).
	h.engineMu.Lock()
	err = h.engine.Connect(cfg)
	h.engineMu.Unlock()

	h.mu.Lock()
	defer h.mu.Unlock()
	if err != nil {
		h.reconnectAttempt = attempt + 1
		log.Printf("xvpn-client-helper: tentativa de reconexão automática falhou: %v", err)
		return
	}
	h.reconnectAttempt = 0
	applyIntranetHosts(h.state.IntranetHosts)
	log.Printf("xvpn-client-helper: túnel reconectado automaticamente")
}
