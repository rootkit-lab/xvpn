package intranet

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyRevert_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hosts")
	original := "127.0.0.1 localhost\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Apply(path); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	if !strings.Contains(text, "xdriver.corp.ihuull.com") || !strings.Contains(text, ServerIP) {
		t.Fatalf("bloco ausente: %q", text)
	}
	if !strings.HasPrefix(text, original) {
		t.Fatalf("perdeu o conteúdo original: %q", text)
	}
	if err := Apply(path); err != nil {
		t.Fatal(err)
	}
	again, _ := os.ReadFile(path)
	if strings.Count(string(again), MarkerBegin) != 1 {
		t.Fatalf("Apply deve ser idempotente, blocos=%d", strings.Count(string(again), MarkerBegin))
	}
	if err := Revert(path); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if string(after) != original {
		t.Fatalf("Revert: got %q want %q", after, original)
	}
}

func TestApplyEntries_CustomRecords(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hosts")
	if err := ApplyEntries(path, []HostEntry{{Hostname: "lab.corp.ihuull.com", IPv4: "10.66.66.9"}}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "lab.corp.ihuull.com") || !strings.Contains(string(got), "10.66.66.9") {
		t.Fatalf("entrada custom ausente: %q", got)
	}
	if !strings.Contains(string(got), "xchat.corp.ihuull.com") {
		t.Fatalf("oficiais têm que continuar no bloco: %q", got)
	}
}
