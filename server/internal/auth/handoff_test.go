package auth

import (
	"strings"
	"testing"
)

func TestHandoffContinueHTML_Escapes(t *testing.T) {
	got := HandoffContinueHTML(`https://xvpn.ihuull.com/api/auth/session`, `tok"&`, `https://xvpn.ihuull.com/?q="`)
	if strings.Contains(got, `tok"&`) {
		t.Fatal("aspas cruas no HTML")
	}
	if !strings.Contains(got, "tok") || !strings.Contains(got, "method=\"post\"") {
		t.Fatalf("form incompleto: %s", got)
	}
}
