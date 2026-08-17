package store

import "testing"

func TestValidProjectSlug(t *testing.T) {
	ok := []string{"xchat", "xvpn-client", "ab", "a1", "abcdefghijabcdefghij"}
	for _, s := range ok {
		if !ValidProjectSlug(s) {
			t.Errorf("esperava %q válido", s)
		}
	}
	bad := []string{"", "a", "-xchat", "xchat-", "XChat", "x chat", "x_chat", "abcdefghijabcdefghijk"}
	for _, s := range bad {
		if ValidProjectSlug(s) {
			t.Errorf("esperava %q inválido", s)
		}
	}
}

func TestReservedProjectSlug(t *testing.T) {
	if !ReservedProjectSlug("repositories") || !ReservedProjectSlug("stars") ||
		!ReservedProjectSlug("issues") || !ReservedProjectSlug("pulls") || !ReservedProjectSlug("edit") {
		t.Fatal("rotas da home deveriam ser reservadas")
	}
	if ReservedProjectSlug("xvpn") {
		t.Fatal("slug de repo não é reservado")
	}
}

func TestProjectRole_Rank(t *testing.T) {
	if !(ProjectRoleOwner.Rank() > ProjectRoleMaintainer.Rank() &&
		ProjectRoleMaintainer.Rank() > ProjectRoleDeveloper.Rank() &&
		ProjectRoleDeveloper.Rank() > ProjectRoleReporter.Rank() &&
		ProjectRoleReporter.Rank() > ProjectRoleGuest.Rank()) {
		t.Fatalf("hierarquia de ProjectRole inesperada")
	}
	if !ProjectRoleOwner.Valid() || ProjectRole("admin").Valid() {
		t.Fatalf("Valid() inesperado")
	}
}
