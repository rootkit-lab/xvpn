package store

import "errors"

// Product é um produto da plataforma ao qual um admin pode ser limitado
// (Fase 33 — PLAN.md §6.13). super_admin ignora a lista. Admin com lista
// vazia permanece irrestrito (mesma superfície da Fase 10) para não
// quebrar a matriz RBAC existente.
type Product string

const (
	ProductCore        Product = "core"
	ProductMarketplace Product = "marketplace"
	ProductXGroup      Product = "xgroup"
	ProductXDriver     Product = "xdriver"
)

// AllProducts é o conjunto canônico de escopos. IAM (users/roles/audit)
// não é produto — viewer+ continua vendo essa seção.
var AllProducts = []Product{ProductCore, ProductMarketplace, ProductXGroup, ProductXDriver}

// Valid reporta se p é um escopo de produto conhecido.
func (p Product) Valid() bool {
	switch p {
	case ProductCore, ProductMarketplace, ProductXGroup, ProductXDriver:
		return true
	default:
		return false
	}
}

// NormalizeProducts deduplica e rejeita ids desconhecidos. Entrada
// vazia/nil permanece vazia (admin irrestrito).
func NormalizeProducts(in []Product) ([]Product, error) {
	if len(in) == 0 {
		return nil, nil
	}
	seen := make(map[Product]struct{}, len(in))
	out := make([]Product, 0, len(in))
	for _, p := range in {
		if !p.Valid() {
			return nil, ErrInvalidProduct
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out, nil
}

// ErrInvalidProduct é devolvido quando a lista contém um id desconhecido.
var ErrInvalidProduct = errors.New("produto inválido")

// ErrProductEscalation é devolvido quando um admin restrito tenta
// conceder um produto que ele mesmo não tem.
var ErrProductEscalation = errors.New("seu escopo não pode conceder esses produtos")

// HasProduct reporta se role+stored autoriza escrita em want.
// super_admin sempre passa. admin com lista vazia passa em todos.
// viewer/member nunca passam (não escrevem seções de admin).
func HasProduct(role Role, stored []Product, want Product) bool {
	if role == RoleSuperAdmin {
		return true
	}
	if role != RoleAdmin {
		return false
	}
	if len(stored) == 0 {
		return true
	}
	for _, p := range stored {
		if p == want {
			return true
		}
	}
	return false
}

// ProductScoped reporta se o admin tem lista explícita (restrita).
func ProductScoped(role Role, stored []Product) bool {
	return role == RoleAdmin && len(stored) > 0
}

// IsSubset reporta se todo item de need está em have.
func IsSubset(need, have []Product) bool {
	if len(need) == 0 {
		return true
	}
	set := make(map[Product]struct{}, len(have))
	for _, p := range have {
		set[p] = struct{}{}
	}
	for _, p := range need {
		if _, ok := set[p]; !ok {
			return false
		}
	}
	return true
}

// ResolveAssignedProducts decide quais produtos persistir no alvo.
// Member/viewer ignoram o campo. Admin restrito sem pedido herda a
// própria lista — não cria admin irrestrito por omissão.
func ResolveAssignedProducts(actorRole Role, actorStored, requested []Product, targetRole Role) ([]Product, error) {
	if targetRole != RoleAdmin {
		return nil, nil
	}
	normalized, err := NormalizeProducts(requested)
	if err != nil {
		return nil, err
	}
	if ProductScoped(actorRole, actorStored) {
		if len(normalized) == 0 {
			return append([]Product(nil), actorStored...), nil
		}
		if !IsSubset(normalized, actorStored) {
			return nil, ErrProductEscalation
		}
		return normalized, nil
	}
	return normalized, nil
}
