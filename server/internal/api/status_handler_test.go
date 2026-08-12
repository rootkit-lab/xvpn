package api

import (
	"encoding/json"
	"net/http"
	"testing"
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
