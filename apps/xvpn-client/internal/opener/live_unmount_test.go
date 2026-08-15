//go:build linux && live

package opener

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestLiveUnmountCleansShares(t *testing.T) {
	// Tenta montar se o túnel estiver up; senão só valida a limpeza local.
	if _, err := os.Stat("/sys/class/net/xvpn0"); err == nil {
		_ = ensureSMBMounted("10.66.66.1", "shared")
		_ = ensureSMBMounted("10.66.66.1", "home-rootkit")
	} else {
		// Sem túnel: cria leftovers locais como o Connect deixaria.
		home, _ := os.UserHomeDir()
		dir := filepath.Join(home, "XVPN")
		_ = os.MkdirAll(dir, 0o755)
		_ = os.Symlink("/tmp", filepath.Join(dir, "Compartilhado"))
		desk := desktopDir()
		legacy := filepath.Join(desk, "smb-share:server=10.66.66.1,share=shared,user=xvpntest")
		_ = os.MkdirAll(legacy, 0o755)
	}

	if err := unmountServerSMBShares("10.66.66.1"); err != nil {
		t.Logf("unmount warnings: %v", err)
	}

	if p := resolveGVFSMount("10.66.66.1", "shared"); p != "" {
		t.Fatalf("shared ainda montado: %s", p)
	}
	if p := resolveGVFSMount("10.66.66.1", "home-rootkit"); p != "" {
		t.Fatalf("home ainda montado: %s", p)
	}

	home, _ := os.UserHomeDir()
	if _, err := os.Lstat(filepath.Join(home, "XVPN", "Compartilhado")); !os.IsNotExist(err) {
		t.Fatalf("symlink Compartilhado deveria ter sumido: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(home, "XVPN", "Meus arquivos")); !os.IsNotExist(err) {
		t.Fatalf("symlink Meus arquivos deveria ter sumido: %v", err)
	}

	legacy := filepath.Join(desktopDir(), "smb-share:server=10.66.66.1,share=shared,user=xvpntest")
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("pasta legado no Desktop deveria ter sido removida")
	}

	// Confirma gio sem smb do host.
	out, _ := exec.Command("gio", "mount", "-l").CombinedOutput()
	t.Logf("gio mount -l:\n%s", out)
}
