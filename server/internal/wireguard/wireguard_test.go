package wireguard

import (
	"testing"
)

func TestEnsureReturnRoutes_RejectsInvalidCIDR(t *testing.T) {
	m := &Manager{ifaceName: "wg0"}
	if err := m.EnsureReturnRoutes([]string{"not-a-cidr"}); err == nil {
		t.Fatal("cidr inválido deveria falhar")
	}
}
