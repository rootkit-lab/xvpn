package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func TestTokenManager_IssueAndParse(t *testing.T) {
	tm := NewTokenManager("um-segredo-de-teste-com-pelo-menos-32-bytes", time.Hour)

	token, err := tm.Issue(42, "alice", store.RoleAdmin)
	if err != nil {
		t.Fatalf("erro inesperado ao emitir token: %v", err)
	}

	claims, err := tm.Parse(token)
	if err != nil {
		t.Fatalf("erro inesperado ao validar token: %v", err)
	}
	if claims.UserID != 42 || claims.Username != "alice" || claims.Role != store.RoleAdmin {
		t.Fatalf("claims inesperadas: %+v", claims)
	}
}

func TestTokenManager_RejectsExpiredToken(t *testing.T) {
	tm := NewTokenManager("um-segredo-de-teste-com-pelo-menos-32-bytes", -time.Hour)

	token, err := tm.Issue(1, "bob", store.RoleMember)
	if err != nil {
		t.Fatalf("erro inesperado ao emitir token: %v", err)
	}

	if _, err := tm.Parse(token); err == nil {
		t.Fatalf("esperava erro para token expirado, obteve nil")
	}
}

func TestNormalizeAudience(t *testing.T) {
	if got := NormalizeAudience("XADMIN"); got != AudXadmin {
		t.Fatalf("xadmin: %s", got)
	}
	if got := NormalizeAudience("xgit"); got != AudXgit {
		t.Fatalf("xgit: %s", got)
	}
	if got := NormalizeAudience("desconhecido"); got != AudXvpn {
		t.Fatalf("default: %s", got)
	}
}

func TestTokenManager_IssueForAudience(t *testing.T) {
	tm := NewTokenManager("um-segredo-de-teste-com-pelo-menos-32-bytes", time.Hour)
	token, err := tm.IssueFor(7, "bob", store.RoleMember, AudXchat)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(token, ".") != 4 {
		t.Fatalf("JWE compacto deveria ter 5 partes (4 pontos), token=%q", token)
	}
	claims, err := tm.Parse(token)
	if err != nil {
		t.Fatal(err)
	}
	if len(claims.Audience) != 1 || claims.Audience[0] != AudXchat {
		t.Fatalf("aud: %+v", claims.Audience)
	}
	if claims.Issuer != IssuerURL {
		t.Fatalf("iss: %s", claims.Issuer)
	}
}

func TestTokenManager_AcceptsLegacyIssuer(t *testing.T) {
	tm := NewTokenManager("um-segredo-de-teste-com-pelo-menos-32-bytes", time.Hour)
	tm.issuer = LegacyIssuerURL
	token, err := tm.Issue(1, "legacy", store.RoleMember)
	if err != nil {
		t.Fatal(err)
	}
	tm.issuer = IssuerURL
	if _, err := tm.Parse(token); err != nil {
		t.Fatalf("issuer legado deveria ser aceito: %v", err)
	}
}

func TestTokenManager_RejectsHMACJWT(t *testing.T) {
	tm := NewTokenManager("um-segredo-de-teste-com-pelo-menos-32-bytes", time.Hour)
	// Três segmentos = JWT assinado, não JWE.
	legacy := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1aWQiOjF9.sig"
	if _, err := tm.Parse(legacy); err == nil {
		t.Fatal("JWT HMAC não pode ser aceito")
	}
}

func TestTokenManager_RejectsWrongSecret(t *testing.T) {
	tm1 := NewTokenManager("segredo-um-com-pelo-menos-32-bytes-aqui", time.Hour)
	tm2 := NewTokenManager("segredo-dois-com-pelo-menos-32-bytes-aqui", time.Hour)

	token, err := tm1.Issue(1, "carol", store.RoleViewer)
	if err != nil {
		t.Fatalf("erro inesperado ao emitir token: %v", err)
	}

	if _, err := tm2.Parse(token); err == nil {
		t.Fatalf("esperava erro ao validar token com segredo diferente, obteve nil")
	}
}
