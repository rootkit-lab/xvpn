package apiclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMe_DecodesIdentity(t *testing.T) {
	var gotPath, gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"username":"rootkit","sftp_enabled":false,"samba_enabled":true}`)
	}))
	defer server.Close()

	result, err := New(server.URL).Me(context.Background())
	if err != nil {
		t.Fatalf("esperava sucesso, obtido erro: %v", err)
	}
	if gotMethod != http.MethodGet || gotPath != "/api/me" {
		t.Fatalf("esperava GET /api/me, obtido %s %s", gotMethod, gotPath)
	}
	if result.Username != "rootkit" {
		t.Fatalf("esperava username %q, obtido %q", "rootkit", result.Username)
	}
	if result.SFTPEnabled {
		t.Fatal("esperava sftp_enabled=false")
	}
	if !result.SambaEnabled {
		t.Fatal("esperava samba_enabled=true")
	}
}

// TestMe_ServerErrorIsActionable cobre o 404 de "nenhum device registrado
// para esse IP": a mensagem do servidor precisa chegar ao usuário junto
// com a instrução do que fazer, não virar um "erro desconhecido".
func TestMe_ServerErrorIsActionable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":"dispositivo não encontrado"}`)
	}))
	defer server.Close()

	_, err := New(server.URL).Me(context.Background())
	if err == nil {
		t.Fatal("esperava erro para resposta 404")
	}
	if !strings.Contains(err.Error(), "dispositivo não encontrado") {
		t.Fatalf("esperava a mensagem do servidor na cadeia de erro, obtido: %v", err)
	}
	if !strings.Contains(err.Error(), "conecte o túnel") {
		t.Fatalf("esperava instrução acionável em português, obtido: %v", err)
	}
}

func TestRegisterSSHKey_SendsSingleKeyAndDecodesResult(t *testing.T) {
	const publicKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIExemplo xvpn-maquina"
	var gotBody sshKeyRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("corpo da requisição inválido: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"fingerprint":"SHA256:0PfV","sftp_enabled":false,"changed":true}`)
	}))
	defer server.Close()

	result, err := New(server.URL).RegisterSSHKey(context.Background(), publicKey+"\n")
	if err != nil {
		t.Fatalf("esperava sucesso, obtido erro: %v", err)
	}
	if gotBody.PublicKey != publicKey {
		t.Fatalf("esperava a chave sem quebra de linha final, obtido %q", gotBody.PublicKey)
	}
	if result.Fingerprint != "SHA256:0PfV" || !result.Changed {
		t.Fatalf("resultado decodificado errado: %+v", result)
	}
	// A chave é aceita mesmo com o toggle desligado — é isso que permite
	// o admin ligar SFTP depois sem pedir nada ao usuário.
	if result.SFTPEnabled {
		t.Fatal("esperava sftp_enabled=false")
	}
}

func TestRegisterSSHKey_ForbiddenOutsideTunnel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"error":"origem fora da rede da VPN"}`)
	}))
	defer server.Close()

	_, err := New(server.URL).RegisterSSHKey(context.Background(), "ssh-ed25519 AAAA xvpn-maquina")
	if err == nil {
		t.Fatal("esperava erro para resposta 403")
	}
	if !strings.Contains(err.Error(), "origem fora da rede da VPN") {
		t.Fatalf("esperava a mensagem do servidor na cadeia de erro, obtido: %v", err)
	}
	if !strings.Contains(err.Error(), "chave SSH") {
		t.Fatalf("esperava contexto acionável em português, obtido: %v", err)
	}
}

// TestEnroll_ReadsUsername garante o caminho rápido de identidade: o
// username vem já na resposta de enrollment (api_version 2), sem esperar
// a primeira conexão para perguntar em GET /api/me.
func TestEnroll_ReadsUsername(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"assigned_ip":"10.66.66.2/32","server_public_key":"chave","endpoint":"203.0.113.10:51820","allowed_ips":"0.0.0.0/0, ::/0","persistent_keepalive":25,"api_version":2,"username":"rootkit"}`)
	}))
	defer server.Close()

	result, err := New(server.URL).Enroll(context.Background(), "convite", "maquina-teste")
	if err != nil {
		t.Fatalf("esperava sucesso, obtido erro: %v", err)
	}
	if result.Username != "rootkit" {
		t.Fatalf("esperava username %q, obtido %q", "rootkit", result.Username)
	}
	if result.APIVersion != SupportedAPIVersion {
		t.Fatalf("esperava api_version %d, obtido %d", SupportedAPIVersion, result.APIVersion)
	}
}
