package api

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestHandleStatus_NoAuthRequired(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)

	rec := doJSON(t, router, http.MethodGet, "/api/status", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, obtido %d: %s", rec.Code, rec.Body.String())
	}

	var resp statusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("erro decodificando resposta: %v", err)
	}
	if resp.APIVersion != APIVersion {
		t.Fatalf("esperado api_version=%d, obtido %d", APIVersion, resp.APIVersion)
	}
	if resp.TotalPeers != 0 {
		t.Fatalf("esperava 0 peers num servidor recém-criado, obtido %d", resp.TotalPeers)
	}
}

// TestHandleStatus_CachesShortWindow confere que chamadas consecutivas
// dentro de statusCacheTTL não repetem a consulta real ao WG (ver
// ROADMAP.md Fase 9) — sem isso, cada poll do painel/cliente batia direto
// no kernel via wgctrl.
func TestHandleStatus_CachesShortWindow(t *testing.T) {
	app, wg := newTestApp(t)
	router := NewRouter(app)

	rec := doJSON(t, router, http.MethodGet, "/api/status", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, obtido %d", rec.Code)
	}
	rec = doJSON(t, router, http.MethodGet, "/api/status", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, obtido %d", rec.Code)
	}
	if wg.listPeersCalls != 1 {
		t.Fatalf("esperava 1 chamada real a ListPeers (cache deveria evitar a segunda), obtido %d", wg.listPeersCalls)
	}

	// Expira o cache manualmente (sem depender de time.Sleep real) e
	// confirma que uma nova consulta de fato acontece.
	app.statusCacheAt = time.Now().Add(-statusCacheTTL - time.Second)
	rec = doJSON(t, router, http.MethodGet, "/api/status", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, obtido %d", rec.Code)
	}
	if wg.listPeersCalls != 2 {
		t.Fatalf("esperava 2ª chamada real após expirar o cache, obtido %d", wg.listPeersCalls)
	}
}
