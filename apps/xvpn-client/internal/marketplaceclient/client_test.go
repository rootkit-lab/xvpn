package marketplaceclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogin_SuccessStoresSession(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/auth/login" || r.Method != http.MethodPost {
			t.Fatalf("requisição inesperada: %s %s", r.Method, r.URL.Path)
		}
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("erro decodificando corpo: %v", err)
		}
		if req.Username != "alice" || req.Password != "senha-123" {
			t.Fatalf("credenciais inesperadas: %+v", req)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token": "jwt-fake",
			"user":  map[string]string{"username": "alice", "role": "member"},
		})
	}))
	defer srv.Close()

	c := New()
	session, err := c.Login(context.Background(), srv.URL, "alice", "senha-123")
	if err != nil {
		t.Fatalf("erro inesperado no login: %v", err)
	}
	if !session.LoggedIn || session.Username != "alice" || session.Role != "member" {
		t.Fatalf("sessão inesperada: %+v", session)
	}

	current := c.Session()
	if !current.LoggedIn || current.Username != "alice" {
		t.Fatalf("Session() não refletiu o login: %+v", current)
	}
}

func TestLogin_InvalidCredentials(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "credenciais inválidas"})
	}))
	defer srv.Close()

	c := New()
	_, err := c.Login(context.Background(), srv.URL, "alice", "errada")
	if err == nil {
		t.Fatal("esperava erro com credenciais inválidas")
	}
	// Um 401 no próprio login é "senha errada", não "sessão expirada":
	// traduzir para ErrNotLoggedIn mandaria o usuário procurar um
	// problema de sessão que não existe.
	if errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("401 no login não deve virar ErrNotLoggedIn, obtido: %v", err)
	}
	if !strings.Contains(err.Error(), "credenciais inválidas") {
		t.Fatalf("esperava a mensagem do servidor no erro, obtido: %v", err)
	}
	if c.Session().LoggedIn {
		t.Fatal("não deveria ter sessão ativa após login falho")
	}
}

func TestUseAPIBase_OverridesListHost(t *testing.T) {
	public := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/auth/login" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"token": "jwt-fake",
				"user":  map[string]string{"username": "alice", "role": "member"},
			})
			return
		}
		t.Fatalf("listagem não deve ir ao host público: %s", r.URL.Path)
	}))
	t.Cleanup(public.Close)

	intranet := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/marketplace/apps" {
			t.Fatalf("rota inesperada no corp: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode([]App{{ID: 7, Name: "xchat"}})
	}))
	t.Cleanup(intranet.Close)

	c := New()
	if _, err := c.Login(context.Background(), public.URL, "alice", "senha-123"); err != nil {
		t.Fatal(err)
	}
	c.UseAPIBase(intranet.URL)
	apps, err := c.ListApps(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 1 || apps[0].Name != "xchat" {
		t.Fatalf("catálogo inesperado: %+v", apps)
	}
}

func TestListApps_RequiresLogin(t *testing.T) {
	c := New()
	if _, err := c.ListApps(context.Background()); err != ErrNotLoggedIn {
		t.Fatalf("esperava ErrNotLoggedIn sem sessão, obtido: %v", err)
	}
}

func TestListApps_SendsAuthHeaderAndParsesCatalog(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/login":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"token": "jwt-fake",
				"user":  map[string]string{"username": "alice", "role": "member"},
			})
		case "/api/marketplace/apps":
			if got := r.Header.Get("Authorization"); got != "Bearer jwt-fake" {
				t.Fatalf("header Authorization inesperado: %q", got)
			}
			_ = json.NewEncoder(w).Encode([]App{
				{
					ID:   1,
					Name: "Ferramenta X",
					Versions: []Version{
						{ID: 1, Version: "1.0.0", Assets: []Asset{
							{ID: 1, Platform: "linux", Filename: "ferramenta.deb", SHA256: "abc"},
						}},
					},
				},
			})
		default:
			t.Fatalf("rota inesperada: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := New()
	if _, err := c.Login(context.Background(), srv.URL, "alice", "senha-123"); err != nil {
		t.Fatalf("erro no login: %v", err)
	}

	apps, err := c.ListApps(context.Background())
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(apps) != 1 || apps[0].Name != "Ferramenta X" || len(apps[0].Versions[0].Assets) != 1 {
		t.Fatalf("catálogo inesperado: %+v", apps)
	}
}

func TestListApps_ExpiredTokenClearsSession(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/login":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"token": "jwt-fake",
				"user":  map[string]string{"username": "alice", "role": "member"},
			})
		case "/api/marketplace/apps":
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "token expirado"})
		}
	}))
	defer srv.Close()

	c := New()
	if _, err := c.Login(context.Background(), srv.URL, "alice", "senha-123"); err != nil {
		t.Fatalf("erro no login: %v", err)
	}

	if _, err := c.ListApps(context.Background()); err != ErrNotLoggedIn {
		t.Fatalf("esperava ErrNotLoggedIn, obtido: %v", err)
	}
	if c.Session().LoggedIn {
		t.Fatal("sessão deveria ter sido limpa após 401")
	}
}

func TestDownloadAsset_VerifiesChecksumAndWritesFile(t *testing.T) {
	content := []byte("conteudo de teste do instalador")
	sum := sha256.Sum256(content)
	expectedHash := hex.EncodeToString(sum[:])

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/login":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"token": "jwt-fake",
				"user":  map[string]string{"username": "alice", "role": "member"},
			})
		case "/api/marketplace/assets/1/download":
			if got := r.Header.Get("Authorization"); got != "Bearer jwt-fake" {
				t.Fatalf("header Authorization inesperado: %q", got)
			}
			_, _ = w.Write(content)
		default:
			t.Fatalf("rota inesperada: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	destDir := t.TempDir()

	c := New()
	if _, err := c.Login(context.Background(), srv.URL, "alice", "senha-123"); err != nil {
		t.Fatalf("erro no login: %v", err)
	}

	result, err := c.downloadAssetTo(context.Background(), destDir, 1, "ferramenta.tar.gz", expectedHash)
	if err != nil {
		t.Fatalf("erro inesperado no download: %v", err)
	}
	if result.SizeBytes != int64(len(content)) {
		t.Fatalf("tamanho inesperado: %d", result.SizeBytes)
	}
	got, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatalf("erro lendo arquivo baixado: %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("conteúdo do arquivo baixado não confere")
	}
}

func TestDownloadAsset_ChecksumMismatchDeletesFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/login":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"token": "jwt-fake",
				"user":  map[string]string{"username": "alice", "role": "member"},
			})
		case "/api/marketplace/assets/1/download":
			_, _ = w.Write([]byte("conteudo adulterado"))
		}
	}))
	defer srv.Close()

	c := New()
	if _, err := c.Login(context.Background(), srv.URL, "alice", "senha-123"); err != nil {
		t.Fatalf("erro no login: %v", err)
	}

	dir := t.TempDir()
	_, err := c.downloadAssetTo(context.Background(), dir, 1, "ferramenta.tar.gz", "hash-que-nunca-vai-bater")
	if err == nil {
		t.Fatal("esperava erro de checksum")
	}

	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		t.Fatalf("erro lendo diretório: %v", readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("arquivo com checksum inválido não deveria ter sobrado em disco: %v", entries)
	}
}

func TestDownloadAsset_RequiresLogin(t *testing.T) {
	c := New()
	if _, err := c.DownloadAsset(context.Background(), 1, "x.bin", "hash"); err != ErrNotLoggedIn {
		t.Fatalf("esperava ErrNotLoggedIn sem sessão, obtido: %v", err)
	}
}

func TestUniqueDestPath_AvoidsCollision(t *testing.T) {
	dir := t.TempDir()
	first := uniqueDestPath(dir, "pacote.deb")
	if err := os.WriteFile(first, []byte("a"), 0o644); err != nil {
		t.Fatalf("erro escrevendo arquivo de teste: %v", err)
	}

	second := uniqueDestPath(dir, "pacote.deb")
	if second == first {
		t.Fatalf("esperava caminho diferente para evitar colisão, obtido o mesmo: %s", second)
	}
	if filepath.Base(second) != "pacote (1).deb" {
		t.Fatalf("nome inesperado para o segundo download: %s", filepath.Base(second))
	}

	if err := os.WriteFile(second, []byte("b"), 0o644); err != nil {
		t.Fatalf("erro escrevendo segundo arquivo de teste: %v", err)
	}
	third := uniqueDestPath(dir, "pacote.deb")
	if filepath.Base(third) != "pacote (2).deb" {
		t.Fatalf("nome inesperado para o terceiro download: %s", filepath.Base(third))
	}
}
