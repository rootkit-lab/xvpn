package store

import "testing"

func TestRole_Valid(t *testing.T) {
	valid := []Role{RoleSuperAdmin, RoleAdmin, RoleViewer, RoleMember}
	for _, r := range valid {
		if !r.Valid() {
			t.Errorf("esperava %q válido", r)
		}
	}

	invalid := []Role{"", "root", "SUPER_ADMIN", "administrator"}
	for _, r := range invalid {
		if r.Valid() {
			t.Errorf("esperava %q inválido", r)
		}
	}
}

func TestRole_Rank(t *testing.T) {
	if !(RoleSuperAdmin.Rank() > RoleAdmin.Rank() && RoleAdmin.Rank() > RoleViewer.Rank() && RoleViewer.Rank() > RoleMember.Rank()) {
		t.Fatalf("hierarquia de rank inesperada: super_admin=%d admin=%d viewer=%d member=%d",
			RoleSuperAdmin.Rank(), RoleAdmin.Rank(), RoleViewer.Rank(), RoleMember.Rank())
	}

	// Papel desconhecido nunca rankeia acima de um papel válido — do
	// contrário um valor corrompido/vazio no banco poderia, por engano,
	// "gerenciar" papéis reais.
	if Role("desconhecido").Rank() >= RoleMember.Rank() {
		t.Fatalf("papel desconhecido não deveria rankear >= member")
	}
}

func TestRole_CanManage(t *testing.T) {
	tests := []struct {
		actor  Role
		target Role
		want   bool
	}{
		{RoleSuperAdmin, RoleSuperAdmin, true},
		{RoleSuperAdmin, RoleAdmin, true},
		{RoleSuperAdmin, RoleViewer, true},
		{RoleSuperAdmin, RoleMember, true},

		{RoleAdmin, RoleSuperAdmin, false},
		{RoleAdmin, RoleAdmin, true},
		{RoleAdmin, RoleViewer, true},
		{RoleAdmin, RoleMember, true},

		{RoleViewer, RoleAdmin, false},
		{RoleViewer, RoleViewer, true},
		{RoleViewer, RoleMember, true},

		{RoleMember, RoleViewer, false},
		{RoleMember, RoleMember, true},
	}
	for _, tt := range tests {
		if got := tt.actor.CanManage(tt.target); got != tt.want {
			t.Errorf("%s.CanManage(%s) = %v, esperado %v", tt.actor, tt.target, got, tt.want)
		}
	}
}
