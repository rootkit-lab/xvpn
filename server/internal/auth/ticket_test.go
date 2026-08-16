package auth

import (
	"testing"
	"time"
)

func TestTicketStore_RedeemOnce(t *testing.T) {
	s := NewTicketStore()
	id, err := s.Issue("jwe", time.Minute)
	if err != nil || id == "" {
		t.Fatalf("issue: %v %q", err, id)
	}
	got, ok := s.Redeem(id)
	if !ok || got != "jwe" {
		t.Fatalf("redeem: %q %v", got, ok)
	}
	if _, ok := s.Redeem(id); ok {
		t.Fatal("ticket deveria ser uso único")
	}
}

func TestTicketStore_Expired(t *testing.T) {
	s := NewTicketStore()
	id, err := s.Issue("jwe", -time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Redeem(id); ok {
		t.Fatal("ticket expirado")
	}
}

func TestIsDocumentNavigation(t *testing.T) {
	if !IsDocumentNavigation("document", "navigate") {
		t.Fatal("location.replace")
	}
	if IsDocumentNavigation("empty", "cors") {
		t.Fatal("fetch XSS")
	}
	if IsDocumentNavigation("", "") {
		t.Fatal("sem metadata")
	}
}
