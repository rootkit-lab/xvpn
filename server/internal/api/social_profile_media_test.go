package api

import "testing"

func TestNormalizeAvatarRef(t *testing.T) {
	t.Parallel()
	ok, err := normalizeAvatarRef("  attachment:12  ")
	if err != nil || ok != "attachment:12" {
		t.Fatalf("avatar anexo: got %q %v", ok, err)
	}
	if _, err := normalizeAvatarRef("https://evil.example/x"); err == nil {
		t.Fatal("URL externa deveria ser rejeitada")
	}
	if _, err := normalizeAvatarRef("javascript:alert(1)"); err == nil {
		t.Fatal("javascript deveria ser rejeitado")
	}
	if _, err := normalizeAvatarRef("tone:primary"); err == nil {
		t.Fatal("tom de capa não vale no avatar")
	}
	if got, err := normalizeAvatarRef(""); err != nil || got != "" {
		t.Fatalf("vazio deveria limpar, got %q %v", got, err)
	}
}

func TestNormalizeBannerRef(t *testing.T) {
	t.Parallel()
	ok, err := normalizeBannerRef("tone:chart-2")
	if err != nil || ok != "tone:safe" {
		t.Fatalf("tom legado: got %q %v", ok, err)
	}
	if _, err := normalizeBannerRef("tone:rainbow"); err == nil {
		t.Fatal("tom inventado deveria ser rejeitado")
	}
	ok, err = normalizeBannerRef("attachment:3")
	if err != nil || ok != "attachment:3" {
		t.Fatalf("anexo: got %q %v", ok, err)
	}
}

func TestNormalizeTheme(t *testing.T) {
	t.Parallel()
	ok, err := normalizeTheme("xgroup")
	if err != nil || ok != "xgroup" {
		t.Fatalf("tema: got %q %v", ok, err)
	}
	ok, err = normalizeTheme("chart-3")
	if err != nil || ok != "xgroup" {
		t.Fatalf("legado: got %q %v", ok, err)
	}
	if _, err := normalizeTheme("rainbow"); err == nil {
		t.Fatal("tema inventado deveria ser rejeitado")
	}
}
