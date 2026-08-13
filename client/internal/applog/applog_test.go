package applog_test

import (
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
