package helper

import (
	"context"
	"sync"
	"testing"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/rootkit-lab/xvpn/client/internal/config"
	"github.com/rootkit-lab/xvpn/client/internal/tunnel"
)

// fakeEngine é um tunnel.Engine em memória, usado para testar o helper sem
// precisar de TUN/rotas/DNS reais (ver internal/platform/{linux,windows}).
type fakeEngine struct {
	mu        sync.Mutex
	connected bool

	// connectDelay, se não-nulo, faz Connect bloquear até o canal ser
	// fechado — simula uma chamada lenta ao motor real (netlink/rotas/
	// DNS) num teste específico.
	connectDelay  chan struct{}
	connectErr    error
	disconnectErr error
}

func (f *fakeEngine) Connect(_ tunnel.Config) error {
	if f.connectDelay != nil {
		<-f.connectDelay
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.connectErr != nil {
		return f.connectErr
	}
	f.connected = true
	return nil
}

func (f *fakeEngine) Disconnect() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.connected = false
	return f.disconnectErr
}

func (f *fakeEngine) Status() (tunnel.Status, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return tunnel.Status{Connected: f.connected}, nil
}

func testDeviceState(t *testing.T) *config.DeviceState {
	t.Helper()
	key, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		t.Fatalf("erro gerando chave de teste: %v", err)
	}
	return &config.DeviceState{
		DeviceName:      "device-teste",
		PrivateKey:      key.String(),
		PublicKey:       key.PublicKey().String(),
		AssignedIP:      "10.66.66.9/32",
		ServerPublicKey: key.PublicKey().String(),
		ServerEndpoint:  "203.0.113.10:51820",
		AllowedIPs:      []string{"0.0.0.0/0", "::/0"},
	}
}

func TestHandleConnect_Success(t *testing.T) {
	h := &Helper{engine: &fakeEngine{}, state: testDeviceState(t), logs: newRingBuffer(10)}

	if _, err := h.handleConnect(nil); err != nil {
		t.Fatalf("esperava sucesso, obtido erro: %v", err)
	}
	if !h.desiredConnected {
		t.Fatal("esperava desiredConnected=true após Connect bem-sucedido")
	}
}

func TestHandleConnect_RequiresEnrollment(t *testing.T) {
	h := &Helper{engine: &fakeEngine{}, logs: newRingBuffer(10)}

	if _, err := h.handleConnect(nil); err == nil {
		t.Fatal("esperava erro sem enrollment (state == nil)")
	}
}

// TestHandleConnect_DoesNotBlockUnrelatedIPCCalls é a regressão do bug de
// mutex único do ROADMAP.md Fase 9: antes desta correção, handleConnect
// segurava Helper.mu durante toda a chamada (potencialmente lenta) ao
// motor de túnel, bloqueando outras chamadas IPC que não dependem do
// motor (preferências, is_enrolled, logs) pelo mesmo tempo.
func TestHandleConnect_DoesNotBlockUnrelatedIPCCalls(t *testing.T) {
	engine := &fakeEngine{connectDelay: make(chan struct{})}
	h := &Helper{engine: engine, state: testDeviceState(t), logs: newRingBuffer(10)}

	connectDone := make(chan struct{})
	go func() {
		_, _ = h.handleConnect(nil)
		close(connectDone)
	}()

	// Dá tempo do goroutine acima entrar em handleConnect e ficar
	// bloqueado dentro de engine.Connect (engineMu travado, h.mu livre).
	time.Sleep(100 * time.Millisecond)

	prefsDone := make(chan struct{})
	go func() {
		_, _ = h.handleGetPreferences(nil)
		close(prefsDone)
	}()

	select {
	case <-prefsDone:
		// ok — handleGetPreferences não esperou o Connect terminar.
	case <-time.After(2 * time.Second):
		t.Fatal("handleGetPreferences ficou bloqueado enquanto handleConnect ainda estava em andamento")
	}

	close(engine.connectDelay)
	select {
	case <-connectDone:
	case <-time.After(2 * time.Second):
		t.Fatal("handleConnect não terminou depois do engine liberar")
	}
}

func TestHandleDisconnect_StopsMonitor(t *testing.T) {
	h := &Helper{engine: &fakeEngine{}, state: testDeviceState(t), logs: newRingBuffer(10)}
	if _, err := h.handleConnect(nil); err != nil {
		t.Fatalf("setup: erro conectando: %v", err)
	}

	if _, err := h.handleDisconnect(nil); err != nil {
		t.Fatalf("esperava sucesso, obtido erro: %v", err)
	}

	h.mu.Lock()
	desired := h.desiredConnected
	cancel := h.monitorCancel
	h.mu.Unlock()
	if desired {
		t.Fatal("esperava desiredConnected=false após Disconnect")
	}
	if cancel != nil {
		t.Fatal("esperava monitorCancel nulo após Disconnect (monitor parado)")
	}
}

// TestCheckConnection_ReconnectDoesNotBlockUnrelatedIPCCalls cobre a mesma
// regressão do mutex único, mas no caminho de reconexão automática
// (reconnect.go) em vez do Connect manual via IPC.
func TestCheckConnection_ReconnectDoesNotBlockUnrelatedIPCCalls(t *testing.T) {
	engine := &fakeEngine{connectDelay: make(chan struct{})}
	state := testDeviceState(t)
	state.Preferences.AutoReconnect = true
	// desiredConnected=true + engine reportando desconectado é o estado
	// "caiu e precisa reconectar" que dispara o fluxo de retry.
	h := &Helper{engine: engine, state: state, logs: newRingBuffer(10), desiredConnected: true}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	checkDone := make(chan struct{})
	go func() {
		h.checkConnection(ctx)
		close(checkDone)
	}()

	// backoffDuration(0) == 1s; dá tempo de passar o backoff e entrar em
	// engine.Connect (bloqueado em connectDelay, engineMu travado).
	time.Sleep(1200 * time.Millisecond)

	prefsDone := make(chan struct{})
	go func() {
		_, _ = h.handleGetPreferences(nil)
		close(prefsDone)
	}()
	select {
	case <-prefsDone:
	case <-time.After(2 * time.Second):
		t.Fatal("handleGetPreferences ficou bloqueado durante reconexão automática em andamento")
	}

	close(engine.connectDelay)
	select {
	case <-checkDone:
	case <-time.After(2 * time.Second):
		t.Fatal("checkConnection não terminou depois do engine liberar")
	}
}
