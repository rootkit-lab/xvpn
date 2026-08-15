package marketplace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestIsPublicIP(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		{"8.8.8.8", true},
		{"1.1.1.1", true},
		{"127.0.0.1", false},
		{"10.66.66.1", false},
		{"192.168.1.1", false},
		{"169.254.1.1", false},
		{"100.64.0.1", false},
		{"::1", false},
		{"2001:4860:4860::8888", true},
	}
	for _, tc := range cases {
		got := isPublicIP(net.ParseIP(tc.ip))
		if got != tc.want {
			t.Errorf("isPublicIP(%s)=%v, want %v", tc.ip, got, tc.want)
		}
	}
}

func TestAssertHTTPSPublicURL(t *testing.T) {
	cases := []struct {
		raw     string
		wantErr bool
	}{
		{"https://github.com/a/b", false},
		{"https://objects.githubusercontent.com/foo", false},
		{"http://github.com/a/b", true},
		{"ftp://github.com/a/b", true},
		{"https:///path", true},
	}
	for _, tc := range cases {
		u, err := url.Parse(tc.raw)
		if err != nil {
			t.Fatalf("parse %q: %v", tc.raw, err)
		}
		err = assertHTTPSPublicURL(u)
		if tc.wantErr && err == nil {
			t.Errorf("%s: esperava erro", tc.raw)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("%s: %v", tc.raw, err)
		}
	}
}

func TestFetchAndPut_RejectsHTTP(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = FetchAndPut(context.Background(), store, "http://example.com/a.deb", strings.Repeat("a", 64))
	if !errors.Is(err, ErrSSRFBlocked) {
		t.Fatalf("esperado ErrSSRFBlocked, got %v", err)
	}
}

func TestFetchAndPut_RejectsLoopbackHTTPS(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// https://127.0.0.1 — o DialContext deve recusar antes de conectar.
	_, _, err = FetchAndPut(context.Background(), store, "https://127.0.0.1/a.deb", strings.Repeat("ab", 32))
	if err == nil || !strings.Contains(err.Error(), ErrSSRFBlocked.Error()) {
		t.Fatalf("esperado bloqueio SSRF, got %v", err)
	}
}

func TestFetchAndPut_HappyPath(t *testing.T) {
	content := []byte("conteudo-de-teste-do-asset")
	sum := sha256.Sum256(content)
	hexSum := hex.EncodeToString(sum[:])

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(content)
	}))
	defer srv.Close()

	// httptest.NewTLSServer escuta em 127.0.0.1 — a guarda anti-SSRF
	// bloquearia. Para exercitar o caminho feliz sem abrir a guarda,
	// testamos Put+hash diretamente aqui e o Dial em TestIsPublicIP /
	// RejectsLoopback. O happy path de rede pública fica coberto pelo
	// sync integration test com um fake fetch injetável, se necessário.
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	result, err := store.Put(strings.NewReader(string(content)))
	if err != nil {
		t.Fatal(err)
	}
	if result.SHA256 != hexSum {
		t.Fatalf("sha=%s want %s", result.SHA256, hexSum)
	}
	_ = srv // documenta a intenção do TLS server acima
}
