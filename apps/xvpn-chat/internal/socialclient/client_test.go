package socialclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoginStoresTokenInMemory(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/auth/login" {
			t.Fatalf("path inesperado: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token": "jwt-fake",
			"user":  map[string]any{"id": 1, "username": "alice", "role": "member"},
		})
	}))
	t.Cleanup(srv.Close)

	c := New()
	sess, err := c.Login(context.Background(), srv.URL, "alice", "senha-123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if !sess.LoggedIn || sess.Username != "alice" {
		t.Fatalf("sessão: %+v", sess)
	}
	if c.Session().LoggedIn != true {
		t.Fatal("token deveria ficar em memória")
	}
}

func TestListPeopleRequiresLogin(t *testing.T) {
	c := New()
	_, err := c.ListPeople(context.Background(), 1, "")
	if err != ErrNotLoggedIn {
		t.Fatalf("esperado ErrNotLoggedIn, obtido %v", err)
	}
}

func TestWSURLNeverPutsTokenInQuery(t *testing.T) {
	sawQuery := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/login":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"token": "jwt-secret",
				"user":  map[string]any{"id": 1, "username": "alice", "role": "member"},
			})
		case "/api/social/people":
			if r.URL.RawQuery != "" && r.URL.Query().Get("token") != "" {
				sawQuery = true
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}, "total": 0, "page": 1, "per_page": 25})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	c := New()
	if _, err := c.Login(context.Background(), srv.URL, "alice", "x"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ListPeople(context.Background(), 1, ""); err != nil {
		t.Fatal(err)
	}
	if sawQuery {
		t.Fatal("token não pode ir na query")
	}
}
