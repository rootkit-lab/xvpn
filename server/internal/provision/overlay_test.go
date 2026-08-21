package provision

import (
	"strings"
	"testing"
)

func TestRenderOverlayNft_DropsCrossNetByDefault(t *testing.T) {
	script := RenderOverlayNft(OverlaySpec{
		Networks: []OverlayNetSpec{
			{ID: 1, CIDR: "10.66.66.0/24"},
			{ID: 2, CIDR: "10.66.80.0/24"},
		},
		Rules: []OverlayRuleSpec{
			{SrcCIDR: "10.66.80.0/24", DstCIDR: "10.66.66.0/24", Action: "allow", Proto: "tcp", Ports: []int{443, 53}},
		},
	})
	if !strings.Contains(script, "destroy table inet xvpn-overlay") {
		t.Fatal(script)
	}
	if strings.Contains(script, "flush table") {
		t.Fatal("flush falha na primeira apply")
	}
	if !strings.Contains(script, "drop") {
		t.Fatal(script)
	}
	if !strings.Contains(script, "tcp dport { 443, 53 } accept") {
		t.Fatal(script)
	}
	if strings.Contains(script, "27017") {
		t.Fatal("mongo não entra no nft")
	}
}

func TestParseOverlaySpec_RequiresNetworks(t *testing.T) {
	if _, err := ParseOverlaySpec([]byte(`{"networks":[]}`)); err == nil {
		t.Fatal("vazio")
	}
}
