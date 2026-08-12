package auth

import (
	"testing"
	"time"
)

func TestTokenManager_IssueAndParse(t *testing.T) {
	tm := NewTokenManager("um-segredo-de-teste-com-pelo-menos-32-bytes", time.Hour)

	token, err := tm.Issue(42, "alice")
	if err != nil {
		t.Fatalf("erro inesperado ao emitir token: %v", err)
	}

	claims, err := tm.Parse(token)
	if err != nil {
		t.Fatalf("erro inesperado ao validar token: %v", err)
	}
	if claims.UserID != 42 || claims.Username != "alice" {
		t.Fatalf("claims inesperadas: %+v", claims)
	}
}

func TestTokenManager_RejectsExpiredToken(t *testing.T) {
	tm := NewTokenManager("um-segredo-de-teste-com-pelo-menos-32-bytes", -time.Hour)

	token, err := tm.Issue(1, "bob")
	if err != nil {
		t.Fatalf("erro inesperado ao emitir token: %v", err)
	}

	if _, err := tm.Parse(token); err == nil {
		t.Fatalf("esperava erro para token expirado, obteve nil")
	}
}

func TestTokenManager_RejectsWrongSecret(t *testing.T) {
	tm1 := NewTokenManager("segredo-um-com-pelo-menos-32-bytes-aqui", time.Hour)
	tm2 := NewTokenManager("segredo-dois-com-pelo-menos-32-bytes-aqui", time.Hour)

	token, err := tm1.Issue(1, "carol")
	if err != nil {
		t.Fatalf("erro inesperado ao emitir token: %v", err)
	}

	if _, err := tm2.Parse(token); err == nil {
		t.Fatalf("esperava erro ao validar token com segredo diferente, obteve nil")
	}
}
