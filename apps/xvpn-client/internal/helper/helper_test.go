package helper

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/rootkit-lab/xvpn/client/internal/apiclient"
	"github.com/rootkit-lab/xvpn/client/internal/config"
	"github.com/rootkit-lab/xvpn/client/internal/tunnel"
)

// fakeEngine é um tunnel.Engine em memória, usado para testar o helper sem
// precisar de TUN/rotas/DNS reais (ver internal/platform/{linux,windows}).
type fakeEngine struct {
	mu        sync.Mutex
	connected bool
	// lastConfig guarda a última config aplicada, para os testes que
	// conferem o que de fato chegou ao motor (ex.: o MTU reaplicado).
	lastConfig tunnel.Config

	// connectDelay, se não-nulo, faz Connect bloquear até o canal ser
	// fechado — simula uma chamada lenta ao motor real (netlink/rotas/
	// DNS) num teste específico.
	connectDelay  chan struct{}
	connectErr    error
	disconnectErr error
}

func (f *fakeEngine) Connect(cfg tunnel.Config) error {
	if f.connectDelay != nil {
		<-f.connectDelay
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.connectErr != nil {
		return f.connectErr
	}
	f.connected = true
	f.lastConfig = cfg
	return nil
}

func (f *fakeEngine) LastConfig() tunnel.Config {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastConfig
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

// newTestHelper monta um Helper isolado do mundo real: a gravação do
// estado não toca o arquivo root-only de /var/lib e a identidade não
// depende de rede (GET /api/me só responde de dentro de um túnel de
// verdade — ver refreshIdentity). Cada teste sobrescreve fetchIdentity/
// saveState quando quer exercitar esses caminhos.
func newTestHelper(t *testing.T, engine tunnel.Engine, state *config.DeviceState) *Helper {
	t.Helper()
	return &Helper{
		engine:    engine,
		state:     state,
		logs:      newRingBuffer(10),
		saveState: func(*config.DeviceState) error { return nil },
		fetchIdentity: func(context.Context) (*apiclient.MeResult, error) {
			return nil, errors.New("sem túnel neste teste")
		},
	}
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
	h := newTestHelper(t, &fakeEngine{}, testDeviceState(t))

	if _, err := h.handleConnect(nil); err != nil {
		t.Fatalf("esperava sucesso, obtido erro: %v", err)
	}
	if !h.desiredConnected {
		t.Fatal("esperava desiredConnected=true após Connect bem-sucedido")
	}
}

func TestHandleConnect_RequiresEnrollment(t *testing.T) {
	h := newTestHelper(t, &fakeEngine{}, nil)

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
	h := newTestHelper(t, engine, testDeviceState(t))

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
	h := newTestHelper(t, &fakeEngine{}, testDeviceState(t))
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

// TestHandleConnect_RefreshesIdentity cobre o caminho que resolve os dois
// bugs da Fase 14: quem é o usuário e se o Samba está liberado só se
// descobre perguntando ao servidor de dentro do túnel, depois de conectar.
func TestHandleConnect_RefreshesIdentity(t *testing.T) {
	state := testDeviceState(t)
	h := newTestHelper(t, &fakeEngine{}, state)
	var saved *config.DeviceState
	h.saveState = func(s *config.DeviceState) error {
		saved = s
		return nil
	}
	h.fetchIdentity = func(context.Context) (*apiclient.MeResult, error) {
		return &apiclient.MeResult{Username: "rootkit", SambaEnabled: true, SFTPEnabled: false}, nil
	}

	if _, err := h.handleConnect(nil); err != nil {
		t.Fatalf("esperava sucesso, obtido erro: %v", err)
	}

	status, err := h.handleStatus(nil)
	if err != nil {
		t.Fatalf("erro consultando status: %v", err)
	}
	resp, ok := status.(StatusResponse)
	if !ok {
		t.Fatalf("esperava StatusResponse, obtido %T", status)
	}
	if resp.Username != "rootkit" || !resp.SambaEnabled || resp.SFTPEnabled {
		t.Fatalf("identidade não propagou para o status: %+v", resp)
	}
	if saved == nil || saved.Username != "rootkit" {
		t.Fatal("esperava a identidade persistida no estado do dispositivo")
	}
}

// TestHandleConnect_IdentityFailureDoesNotFailConnect: o túnel já está de
// pé quando a identidade é consultada, então uma falha ali é só um aviso
// no log — nunca um erro de conexão na cara do usuário.
func TestHandleConnect_IdentityFailureDoesNotFailConnect(t *testing.T) {
	h := newTestHelper(t, &fakeEngine{}, testDeviceState(t))
	h.fetchIdentity = func(context.Context) (*apiclient.MeResult, error) {
		return nil, errors.New("servidor não respondeu")
	}

	if _, err := h.handleConnect(nil); err != nil {
		t.Fatalf("esperava sucesso mesmo com /api/me falhando, obtido erro: %v", err)
	}
	if !h.desiredConnected {
		t.Fatal("esperava desiredConnected=true após Connect bem-sucedido")
	}
}

func TestHandleSetMTU_RejectsOutOfRange(t *testing.T) {
	for _, mtu := range []int{1, 1279, 1501, 9000, -1} {
		h := newTestHelper(t, &fakeEngine{}, testDeviceState(t))
		_, err := h.handleSetMTU(json.RawMessage(`{"mtu":` + strconv.Itoa(mtu) + `}`))
		if err == nil {
			t.Fatalf("esperava erro para MTU %d", mtu)
		}
		if !strings.Contains(err.Error(), "1280") || !strings.Contains(err.Error(), "1500") {
			t.Fatalf("esperava erro citando a faixa aceita, obtido: %v", err)
		}
		if h.state.MTU != 0 {
			t.Fatalf("MTU inválido não podia ter sido gravado (obtido %d)", h.state.MTU)
		}
	}
}

func TestHandleSetMTU_AcceptsAutomaticAndValidRange(t *testing.T) {
	for _, mtu := range []int{0, 1280, 1400, 1500} {
		h := newTestHelper(t, &fakeEngine{}, testDeviceState(t))
		if _, err := h.handleSetMTU(json.RawMessage(`{"mtu":` + strconv.Itoa(mtu) + `}`)); err != nil {
			t.Fatalf("esperava aceitar MTU %d, obtido erro: %v", mtu, err)
		}
	}
}

// TestHandleSetMTU_PersistsAndReappliesWhenConnected: quem mexe no MTU
// costuma estar tentando destravar uma conexão em curso, então o valor
// novo tem que chegar ao motor sem exigir reconectar à mão.
func TestHandleSetMTU_PersistsAndReappliesWhenConnected(t *testing.T) {
	engine := &fakeEngine{}
	h := newTestHelper(t, engine, testDeviceState(t))
	var saved *config.DeviceState
	h.saveState = func(s *config.DeviceState) error {
		saved = s
		return nil
	}
	if _, err := h.handleConnect(nil); err != nil {
		t.Fatalf("setup: erro conectando: %v", err)
	}

	result, err := h.handleSetMTU(json.RawMessage(`{"mtu":1380}`))
	if err != nil {
		t.Fatalf("esperava sucesso, obtido erro: %v", err)
	}
	if setting, ok := result.(MTUSetting); !ok || setting.MTU != 1380 {
		t.Fatalf("esperava MTUSetting{1380}, obtido %#v", result)
	}
	if saved == nil || saved.MTU != 1380 {
		t.Fatal("esperava o MTU persistido no estado do dispositivo")
	}
	if got := engine.LastConfig().MTU; got != 1380 {
		t.Fatalf("esperava o MTU reaplicado no motor com o túnel conectado, obtido %d", got)
	}
}

func TestHandleGetMTU_ReturnsSavedValue(t *testing.T) {
	state := testDeviceState(t)
	state.MTU = 1420
	h := newTestHelper(t, &fakeEngine{}, state)

	result, err := h.handleGetMTU(nil)
	if err != nil {
		t.Fatalf("esperava sucesso, obtido erro: %v", err)
	}
	if setting, ok := result.(MTUSetting); !ok || setting.MTU != 1420 {
		t.Fatalf("esperava MTUSetting{1420}, obtido %#v", result)
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
	h := newTestHelper(t, engine, state)
	h.desiredConnected = true

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
