package bitlaunch

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientListCreateDestroyRebuild(t *testing.T) {
	var lastMethod, lastPath, lastAuth string
	var lastBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastMethod, lastPath, lastAuth = r.Method, r.URL.Path, r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		lastBody = nil
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &lastBody)
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/servers":
			_ = json.NewEncoder(w).Encode([]Server{{ID: "bl-1", Name: "edge", IPv4: "203.0.113.9"}})
		case r.Method == http.MethodPost && r.URL.Path == "/servers":
			_ = json.NewEncoder(w).Encode(Server{ID: "bl-new", Name: "mesh-a", Status: "launching"})
		case r.Method == http.MethodDelete && r.URL.Path == "/servers/bl-1":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && r.URL.Path == "/servers/bl-1/rebuild":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/user":
			_ = json.NewEncoder(w).Encode(Account{Email: "ops@ihuull.com", Balance: 30000, Used: 1, Limit: 5, CostPerHr: 21})
		case r.Method == http.MethodPost && r.URL.Path == "/transactions":
			_ = json.NewEncoder(w).Encode(Transaction{
				ID: "tx-1", Address: "bc1qtest", CryptoSymbol: "BTC", AmountUSD: 20, AmountCrypto: "0.002", Status: "Pending",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	c := New("tok-secret")
	c.BaseURL = srv.URL
	c.HTTPClient = srv.Client()

	list, err := c.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].ID != "bl-1" || lastAuth != "Bearer tok-secret" {
		t.Fatalf("list: %+v auth=%s", list, lastAuth)
	}

	created, err := c.Create(CreateOpts{
		Name: "mesh-a", HostID: 4, HostImageID: "img", SizeID: "sz", RegionID: "ams",
		SSHKeys: []string{"k1"}, InitScript: "#!/bin/bash",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ID != "bl-new" || lastMethod != http.MethodPost {
		t.Fatalf("create: %+v method=%s", created, lastMethod)
	}
	server, _ := lastBody["server"].(map[string]any)
	if server["initscript"] != "#!/bin/bash" || server["name"] != "mesh-a" {
		t.Fatalf("create body: %+v", lastBody)
	}

	if err := c.Destroy("bl-1"); err != nil {
		t.Fatalf("destroy: %v", err)
	}
	if lastPath != "/servers/bl-1" || lastMethod != http.MethodDelete {
		t.Fatalf("destroy path=%s method=%s", lastPath, lastMethod)
	}

	if err := c.Rebuild("bl-1", RebuildOpts{HostImageID: "img2"}); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if lastPath != "/servers/bl-1/rebuild" {
		t.Fatalf("rebuild path=%s", lastPath)
	}

	acc, err := c.Account()
	if err != nil || acc.Balance != 30000 || lastPath != "/user" {
		t.Fatalf("account: %+v err=%v path=%s", acc, err, lastPath)
	}
	tx, err := c.CreateTransaction(TopUpOpts{AmountUSD: 20, CryptoSymbol: "BTC"})
	if err != nil || tx.Address != "bc1qtest" || lastPath != "/transactions" {
		t.Fatalf("topup: %+v err=%v path=%s", tx, err, lastPath)
	}
}

func TestClientRejectsEmptyToken(t *testing.T) {
	c := New("")
	if _, err := c.List(); err == nil {
		t.Fatal("token vazio deveria falhar sem rede")
	}
}
