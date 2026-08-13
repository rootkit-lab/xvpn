package applog_test

import (
	"strconv"
	"testing"

	"github.com/rootkit-lab/xvpn/client/internal/applog"
)

func TestRecentRing(t *testing.T) {
	logger := applog.Setup("test")
	logger.Info("hello-ring")
	lines := applog.Recent()
	if len(lines) == 0 {
		t.Fatal("esperava pelo menos uma linha no ring após Info")
	}
	found := false
	for _, line := range lines {
		if contains(line, "hello-ring") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("linha hello-ring não encontrada em %v", lines)
	}
}

// TestRecentRingCapsAtFixedSize cobre a troca de "re-fatiar pela frente"
// (que ia encolhendo a capacidade do slice a cada linha) por um deslocamento
// em memória fixa — o comportamento observável (capacidade limitada, mais
// antigas descartadas primeiro) deve continuar o mesmo (ver ROADMAP.md
// Fase 9).
func TestRecentRingCapsAtFixedSize(t *testing.T) {
	logger := applog.Setup("test")
	const total = 500
	for i := 0; i < total; i++ {
		logger.Info("linha-de-teste-ring", "seq", strconv.Itoa(i))
	}

	lines := applog.Recent()
	if len(lines) == 0 || len(lines) >= total {
		t.Fatalf("esperava ring de tamanho fixo bem menor que %d, obtido %d linhas", total, len(lines))
	}

	for _, line := range lines {
		if contains(line, "seq=0") {
			t.Fatalf("linha mais antiga (seq=0) ainda presente após %d gravações — overflow não descartou nada", total)
		}
	}
	lastMarker := "seq=" + strconv.Itoa(total-1)
	if !contains(lines[len(lines)-1], lastMarker) {
		t.Fatalf("esperava que a última linha do ring fosse a mais recente (%s), obtido %q", lastMarker, lines[len(lines)-1])
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		(func() bool {
			for i := 0; i+len(sub) <= len(s); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		})())
}
