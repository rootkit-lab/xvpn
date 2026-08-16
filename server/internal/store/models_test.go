package store

import (
	"errors"
	"testing"
)

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

func TestHasProduct(t *testing.T) {
	tests := []struct {
		role    Role
		stored  []Product
		want    Product
		allowed bool
	}{
		{RoleSuperAdmin, nil, ProductMarketplace, true},
		{RoleSuperAdmin, []Product{ProductCore}, ProductMarketplace, true},
		{RoleAdmin, nil, ProductMarketplace, true},
		{RoleAdmin, []Product{}, ProductCore, true},
		{RoleAdmin, []Product{ProductMarketplace}, ProductMarketplace, true},
		{RoleAdmin, []Product{ProductMarketplace}, ProductCore, false},
		{RoleAdmin, []Product{ProductCore, ProductXDriver}, ProductXGroup, false},
		{RoleViewer, nil, ProductMarketplace, false},
		{RoleMember, []Product{ProductMarketplace}, ProductMarketplace, false},
	}
	for _, tt := range tests {
		if got := HasProduct(tt.role, tt.stored, tt.want); got != tt.allowed {
			t.Errorf("HasProduct(%s, %v, %s) = %v, esperado %v", tt.role, tt.stored, tt.want, got, tt.allowed)
		}
	}
}

func TestResolveAssignedProducts(t *testing.T) {
	got, err := ResolveAssignedProducts(RoleSuperAdmin, nil, []Product{ProductMarketplace}, RoleAdmin)
	if err != nil || len(got) != 1 || got[0] != ProductMarketplace {
		t.Fatalf("super_admin deveria persistir o pedido, got %v err %v", got, err)
	}

	got, err = ResolveAssignedProducts(RoleAdmin, nil, nil, RoleAdmin)
	if err != nil || got != nil {
		t.Fatalf("admin irrestrito sem pedido deve gravar vazio (irrestrito), got %v err %v", got, err)
	}

	got, err = ResolveAssignedProducts(RoleAdmin, []Product{ProductMarketplace}, nil, RoleAdmin)
	if err != nil || len(got) != 1 || got[0] != ProductMarketplace {
		t.Fatalf("admin restrito sem pedido herda o próprio escopo, got %v err %v", got, err)
	}

	_, err = ResolveAssignedProducts(RoleAdmin, []Product{ProductMarketplace}, []Product{ProductCore}, RoleAdmin)
	if !errors.Is(err, ErrProductEscalation) {
		t.Fatalf("admin da loja não concede core, err=%v", err)
	}

	got, err = ResolveAssignedProducts(RoleAdmin, []Product{ProductMarketplace}, []Product{"loja"}, RoleAdmin)
	if !errors.Is(err, ErrInvalidProduct) {
		t.Fatalf("produto desconhecido deveria falhar, got %v err %v", got, err)
	}

	got, err = ResolveAssignedProducts(RoleAdmin, []Product{ProductMarketplace}, []Product{ProductMarketplace}, RoleMember)
	if err != nil || got != nil {
		t.Fatalf("member ignora products, got %v err %v", got, err)
	}
}
