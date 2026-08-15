package marketplace

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// ErrSSRFBlocked é devolvido quando a URL do asset aponta (direta ou via
// DNS) para um endereço que o servidor não deve buscar por conta própria
// — loopback, privado, link-local etc. Sem essa guarda, o manifesto
// viraria um proxy SSRF para 127.0.0.1:8080 ou 10.66.66.1 (PLAN.md §6.10.3).
var ErrSSRFBlocked = errors.New("url do asset rejeitada pela guarda anti-SSRF")

// ErrSHA256Mismatch é devolvido quando o conteúdo baixado não bate com o
// hash esperado no manifesto — integridade, não só transporte.
var ErrSHA256Mismatch = errors.New("sha256 do asset não confere com o manifesto")

const fetchTimeout = 2 * time.Minute

// FetchAndPut baixa url via HTTPS (com guarda anti-SSRF), confere o
// SHA-256 esperado e grava o blob no Store content-addressed. filename
// é uma sugestão derivada do path da URL (o caller pode sobrescrever).
func FetchAndPut(ctx context.Context, store *Store, rawURL, expectedSHA256 string) (PutResult, string, error) {
	expectedSHA256 = strings.ToLower(strings.TrimSpace(expectedSHA256))
	if len(expectedSHA256) != 64 {
		return PutResult{}, "", fmt.Errorf("sha256 esperado inválido")
	}
	if _, err := hex.DecodeString(expectedSHA256); err != nil {
		return PutResult{}, "", fmt.Errorf("sha256 esperado inválido: %w", err)
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return PutResult{}, "", fmt.Errorf("url inválida: %w", err)
	}
	if err := assertHTTPSPublicURL(parsed); err != nil {
		return PutResult{}, "", err
	}

	client := &http.Client{
		Timeout: fetchTimeout,
		Transport: &http.Transport{
			Proxy: nil, // nunca herdar HTTP_PROXY — o proxy poderia reintroduzir SSRF
			DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
				host, port, err := net.SplitHostPort(address)
				if err != nil {
					return nil, err
				}
				ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
				if err != nil {
					return nil, fmt.Errorf("resolvendo %s: %w", host, err)
				}
				var lastErr error
				for _, ipa := range ips {
					if !isPublicIP(ipa.IP) {
						lastErr = fmt.Errorf("%w: %s resolve para %s", ErrSSRFBlocked, host, ipa.IP)
						continue
					}
					dialer := &net.Dialer{Timeout: 30 * time.Second}
					conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ipa.IP.String(), port))
					if err != nil {
						lastErr = err
						continue
					}
					return conn, nil
				}
				if lastErr == nil {
					lastErr = fmt.Errorf("%w: nenhum IP público para %s", ErrSSRFBlocked, host)
				}
				return nil, lastErr
			},
			DisableKeepAlives: true,
		},
		// GitHub Releases responde 302 → objects.githubusercontent.com.
		// Seguimos só https; o DialContext continua bloqueando IPs
		// privados/loopback em cada hop (anti-SSRF).
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("muitos redirects ao baixar asset")
			}
			return assertHTTPSPublicURL(req.URL)
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return PutResult{}, "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, ErrSSRFBlocked) || strings.Contains(err.Error(), ErrSSRFBlocked.Error()) {
			return PutResult{}, "", err
		}
		return PutResult{}, "", fmt.Errorf("baixando asset: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return PutResult{}, "", fmt.Errorf("baixando asset: HTTP %d", resp.StatusCode)
	}

	result, err := store.Put(resp.Body)
	if err != nil {
		return PutResult{}, "", err
	}

	if result.SHA256 != expectedSHA256 {
		_ = store.Remove(result.RelPath)
		return PutResult{}, "", fmt.Errorf("%w: esperado %s, obtido %s", ErrSHA256Mismatch, expectedSHA256, result.SHA256)
	}

	filename := path.Base(parsed.Path)
	if filename == "" || filename == "." || filename == "/" {
		filename = expectedSHA256[:16]
	}
	return result, filename, nil
}

// assertHTTPSPublicURL rejeita schemes não-https e hosts vazios. A
// resolução DNS + isPublicIP fica no DialContext (cada hop do redirect).
func assertHTTPSPublicURL(u *url.URL) error {
	if u == nil {
		return fmt.Errorf("%w: url ausente", ErrSSRFBlocked)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("%w: só https é aceito", ErrSSRFBlocked)
	}
	if u.Host == "" {
		return fmt.Errorf("%w: host ausente", ErrSSRFBlocked)
	}
	return nil
}

func isPublicIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
			return false
		}
	}
	return true
}
