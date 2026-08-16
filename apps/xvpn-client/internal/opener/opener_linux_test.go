//go:build linux

package opener

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveGVFSMountPrefersAnonymous(t *testing.T) {
	root := t.TempDir()
	// Simula gvfsRoot via chdir... resolveGVFSMount usa os.Getuid() path.
	// Em vez de patchar, testamos a lógica de escolha com um helper
	// interno espelhado: pickGVFSEntry.
	anon := "smb-share:server=10.66.66.1,share=shared"
	legacy := "smb-share:server=10.66.66.1,share=shared,user=xvpntest"
	if err := os.Mkdir(filepath.Join(root, anon), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	got := pickGVFSEntry(root, "10.66.66.1", "shared")
	want := filepath.Join(root, anon)
	if got != want {
		t.Fatalf("preferia anônimo %q, obtido %q", want, got)
	}
}

func TestResolveGVFSMountFallsBackToUserMount(t *testing.T) {
	root := t.TempDir()
	legacy := "smb-share:server=10.66.66.1,share=shared,user=xvpntest"
	if err := os.Mkdir(filepath.Join(root, legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	got := pickGVFSEntry(root, "10.66.66.1", "shared")
	want := filepath.Join(root, legacy)
	if got != want {
		t.Fatalf("fallback legado %q, obtido %q", want, got)
	}
}

func TestShareLinkName(t *testing.T) {
	if got := shareLinkName("shared"); got != "Compartilhado" {
		t.Fatalf("shared: %q", got)
	}
	if got := shareLinkName("home-rootkit"); got != "Meus arquivos" {
		t.Fatalf("home: %q", got)
	}
}

func TestPickGVFSEntryEmpty(t *testing.T) {
	root := t.TempDir()
	if got := pickGVFSEntry(root, "10.66.66.1", "shared"); got != "" {
		t.Fatalf("esperava vazio, obtido %q", got)
	}
}

func TestUnescapeProcMount(t *testing.T) {
	if got := unescapeProcMount(`/home/wiz/XVPN/Meus\040arquivos`); got != "/home/wiz/XVPN/Meus arquivos" {
		t.Fatalf("got %q", got)
	}
}
