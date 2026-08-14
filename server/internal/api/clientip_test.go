package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// doProxiedJSON reproduz o caminho real de produção: o Nginx local
// (127.0.0.1) repassando a requisição para o xvpn-server. O forwardedFor
// deve vir no formato que `$proxy_add_x_forwarded_for` produz — o que o
// cliente mandou, seguido do $remote_addr real dele no fim da cadeia.
func doProxiedJSON(t *testing.T, router http.Handler, method, path string, body any, forwardedFor string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("erro codificando corpo da requisição: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:54321"
	req.Header.Set("X-Forwarded-For", forwardedFor)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// TestTrustedProxies_ListIsValid protege o panic de NewRouter: se alguém
// editar trustedProxies para um CIDR inválido, o Gin zera a lista de
// proxies confiáveis e o ClientIP de todo mundo vira 127.0.0.1.
func TestTrustedProxies_ListIsValid(t *testing.T) {
	if err := gin.New().SetTrustedProxies(trustedProxies); err != nil {
		t.Fatalf("lista trustedProxies inválida: %v", err)
	}
}

// TestClientIP_ResolvesRealClientBehindNginx cobre a resolução de
// c.ClientIP() — a chave do rate limit dos endpoints públicos e o campo
// client_ip do log de requisições — nos cenários que importam em
// produção, incluindo cabeçalhos forjados pelo cliente.
func TestClientIP_ResolvesRealClientBehindNginx(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)

	// Rota exclusiva do teste, montada sobre o router real para herdar a
	// configuração de proxies confiáveis feita em NewRouter.
	router.GET("/__test/client-ip", func(c *gin.Context) {
		c.String(http.StatusOK, c.ClientIP())
	})

	const realClient = "203.0.113.7"

	tests := []struct {
		name       string
		remoteAddr string
		forwarded  string
		realIP     string
		want       string
	}{
		{
			name:       "Nginx com $proxy_add_x_forwarded_for, cliente sem XFF",
			remoteAddr: "127.0.0.1:54321",
			forwarded:  realClient,
			realIP:     realClient,
			want:       realClient,
		},
		{
			name:       "cliente forja um XFF, Nginx anexa o IP real dele",
			remoteAddr: "127.0.0.1:54321",
			forwarded:  "198.51.100.1, " + realClient,
			realIP:     realClient,
			want:       realClient,
		},
		{
			name:       "cliente forja uma cadeia inteira de XFF",
			remoteAddr: "127.0.0.1:54321",
			forwarded:  "198.51.100.1, 198.51.100.2, 198.51.100.3, " + realClient,
			realIP:     realClient,
			want:       realClient,
		},
		{
			name:       "cliente forja o próprio loopback no XFF",
			remoteAddr: "127.0.0.1:54321",
			forwarded:  "127.0.0.1, " + realClient,
			realIP:     realClient,
			want:       realClient,
		},
		{
			name:       "cliente forja um X-Real-IP, sobrescrito pelo Nginx",
			remoteAddr: "127.0.0.1:54321",
			forwarded:  realClient,
			realIP:     realClient,
			want:       realClient,
		},
		{
			// Defesa em profundidade: se o Nginx passar a usar
			// $remote_addr (que descarta o header do cliente em vez de
			// anexar), o resultado precisa continuar correto.
			name:       "Nginx com $remote_addr em vez de $proxy_add_x_forwarded_for",
			remoteAddr: "127.0.0.1:54321",
			forwarded:  realClient,
			realIP:     "",
			want:       realClient,
		},
		{
			// Sem XFF nenhum, o Gin cai para o X-Real-IP — que o Nginx
			// preenche com $remote_addr, então também é confiável.
			name:       "sem XFF, cai para o X-Real-IP do Nginx",
			remoteAddr: "127.0.0.1:54321",
			forwarded:  "",
			realIP:     realClient,
			want:       realClient,
		},
		{
			name:       "conexão direta ao servidor, sem proxy, com headers forjados",
			remoteAddr: realClient + ":44444",
			forwarded:  "198.51.100.1",
			realIP:     "198.51.100.1",
			want:       realClient,
		},
		{
			// Cenário da Fase 14 (autenticação de dispositivo pelo IP de
			// origem dentro de 10.66.66.0/24): um peer que fala direto com
			// o servidor pela wg0 não pode forjar o próprio IP.
			name:       "conexão direta pela wg0 com headers forjados",
			remoteAddr: "10.66.66.5:44444",
			forwarded:  "10.66.66.9",
			realIP:     "10.66.66.9",
			want:       "10.66.66.5",
		},
		{
			name:       "Nginx falando IPv6 no loopback",
			remoteAddr: "[::1]:54321",
			forwarded:  realClient,
			realIP:     realClient,
			want:       realClient,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/__test/client-ip", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.forwarded != "" {
				req.Header.Set("X-Forwarded-For", tt.forwarded)
			}
			if tt.realIP != "" {
				req.Header.Set("X-Real-IP", tt.realIP)
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if got := rec.Body.String(); got != tt.want {
				t.Fatalf("ClientIP() = %q, esperado %q", got, tt.want)
			}
		})
	}
}

// TestLoginRateLimit_ForgedXForwardedForCannotBypass é o teste de
// regressão da falha: um atacante trocando o X-Forwarded-For a cada
// tentativa zerava o contador do loginLimiter e podia forçar bruta a
// senha de admin à vontade.
func TestLoginRateLimit_ForgedXForwardedForCannotBypass(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUser(t, app, "alice", "senha-forte-123")
	router := NewRouter(app)

	const attacker = "203.0.113.7"

	var lastCode int
	for i := 0; i <= loginRateLimitMax; i++ {
		forged := fmt.Sprintf("198.51.100.%d, %s", i+1, attacker)
		rec := doProxiedJSON(t, router, http.MethodPost, "/api/auth/login",
			loginRequest{Username: "alice", Password: "senha-errada"}, forged)
		lastCode = rec.Code
	}

	if lastCode != http.StatusTooManyRequests {
		t.Fatalf("X-Forwarded-For forjado furou o rate limit do login: esperava 429 na tentativa %d, obtido %d",
			loginRateLimitMax+1, lastCode)
	}
}

// TestEnrollRateLimit_ForgedXForwardedForCannotBypass: mesma falha no
// endpoint de enrollment, onde o que estava exposto a força bruta era o
// código de convite.
func TestEnrollRateLimit_ForgedXForwardedForCannotBypass(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)

	const attacker = "203.0.113.7"

	var lastCode int
	for i := 0; i <= enrollRateLimitMax; i++ {
		forged := fmt.Sprintf("198.51.100.%d, %s", i+1, attacker)
		rec := doProxiedJSON(t, router, http.MethodPost, "/api/devices/enroll",
			enrollRequest{InviteToken: "convite-invalido", PublicKey: testPublicKey, DeviceName: "chute"}, forged)
		lastCode = rec.Code
	}

	if lastCode != http.StatusTooManyRequests {
		t.Fatalf("X-Forwarded-For forjado furou o rate limit do enrollment: esperava 429 na tentativa %d, obtido %d",
			enrollRateLimitMax+1, lastCode)
	}
}

// TestLoginRateLimit_DistinctClientsCountedSeparately é a outra metade da
// correção: confiar só no loopback não pode fazer o ClientIP de todos os
// usuários colapsar em 127.0.0.1 — isso transformaria o rate limit num
// DoS coletivo, em que um atacante derruba o login de todo mundo.
func TestLoginRateLimit_DistinctClientsCountedSeparately(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUser(t, app, "alice", "senha-forte-123")
	router := NewRouter(app)

	for i := 0; i <= loginRateLimitMax; i++ {
		doProxiedJSON(t, router, http.MethodPost, "/api/auth/login",
			loginRequest{Username: "alice", Password: "senha-errada"}, "203.0.113.7")
	}

	rec := doProxiedJSON(t, router, http.MethodPost, "/api/auth/login",
		loginRequest{Username: "alice", Password: "senha-errada"}, "203.0.113.7")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("esperava 429 para o cliente que estourou a cota, obtido %d", rec.Code)
	}

	rec = doProxiedJSON(t, router, http.MethodPost, "/api/auth/login",
		loginRequest{Username: "alice", Password: "senha-forte-123"}, "198.51.100.42")
	if rec.Code != http.StatusOK {
		t.Fatalf("um cliente distinto atrás do mesmo Nginx não deveria herdar o rate limit do vizinho: esperava 200, obtido %d: %s",
			rec.Code, rec.Body.String())
	}
}
