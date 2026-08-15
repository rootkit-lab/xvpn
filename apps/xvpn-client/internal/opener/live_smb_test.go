//go:build linux && live

package opener

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Rode com: go test -tags=live ./internal/opener/ -count=1 -v -run Live
func TestLiveAnonymousMountOpenShared(t *testing.T) {
	if _, err := os.Stat("/sys/class/net/xvpn0"); err != nil {
		t.Skip("VPN (xvpn0) não está up")
	}

	_ = exec.Command("gio", "mount", "-u", "smb://10.66.66.1/shared").Run()
	_ = exec.Command("gio", "mount", "-u", "smb://10.66.66.1/home-rootkit").Run()
	time.Sleep(300 * time.Millisecond)

	for _, share := range []string{"shared", "home-rootkit"} {
		if err := ensureSMBMounted("10.66.66.1", share); err != nil {
			t.Fatalf("ensureSMBMounted(%s): %v", share, err)
		}
		path := resolveGVFSMount("10.66.66.1", share)
		if path == "" {
			t.Fatalf("resolveGVFSMount(%s) vazio", share)
		}
		wantBase := "smb-share:server=10.66.66.1,share=" + share
		if filepath.Base(path) != wantBase {
			t.Fatalf("esperava mount anônimo %q, obtido %q", wantBase, filepath.Base(path))
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			t.Fatalf("ReadDir %s: %v", path, err)
		}
		t.Logf("%s OK (%d entradas) path=%s", share, len(entries), path)
		for _, e := range entries {
			t.Logf("  - %s", e.Name())
		}
	}

	shared := resolveGVFSMount("10.66.66.1", "shared")
	if _, err := os.Stat(filepath.Join(shared, "contas.txt")); err != nil {
		t.Fatalf("contas.txt no shared: %v", err)
	}

	if err := openSMBShare("10.66.66.1", "shared"); err != nil {
		t.Fatalf("openSMBShare: %v", err)
	}
	time.Sleep(600 * time.Millisecond)

	if processArgsContain(shared) || processArgsContain("share=shared") {
		t.Log("file manager args contêm o path GVFS — destino correto")
	} else {
		t.Log("não achei path nos args do file manager (janela reusada?); mount+contas.txt OK")
	}
}

func processArgsContain(substr string) bool {
	ents, err := os.ReadDir("/proc")
	if err != nil {
		return false
	}
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		cmdline, err := os.ReadFile(filepath.Join("/proc", e.Name(), "cmdline"))
		if err != nil || len(cmdline) == 0 {
			continue
		}
		s := string(cmdline)
		if !strings.Contains(s, "cosmic-files") && !strings.Contains(s, "gio") {
			continue
		}
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}
