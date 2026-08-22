package provision

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const overlayNftPath = "/etc/xvpn/overlay.nft"

// OverlaySpec is the FORWARD policy for overlay nets (default deny between CIDRs).
type OverlaySpec struct {
	Networks []OverlayNetSpec  `json:"networks"`
	Rules    []OverlayRuleSpec `json:"rules"`
	Pairs    []OverlayPairSpec `json:"pairs"`
}

type OverlayNetSpec struct {
	ID   uint   `json:"id"`
	CIDR string `json:"cidr"`
	Exit bool   `json:"exit"`
}

type OverlayRuleSpec struct {
	SrcCIDR string `json:"src_cidr"`
	DstCIDR string `json:"dst_cidr"`
	Action  string `json:"action"`
	Proto   string `json:"proto"`
	Ports   []int  `json:"ports"`
}

type OverlayPairSpec struct {
	SrcCIDR string `json:"src_cidr"`
	DstCIDR string `json:"dst_cidr"`
}

func ParseOverlaySpec(raw []byte) (OverlaySpec, error) {
	var s OverlaySpec
	if err := json.Unmarshal(raw, &s); err != nil {
		return OverlaySpec{}, fmt.Errorf("JSON overlay inválido")
	}
	if len(s.Networks) < 2 {
		return OverlaySpec{}, fmt.Errorf("overlay exige ao menos infra e users")
	}
	for _, n := range s.Networks {
		if n.CIDR == "" || n.ID == 0 {
			return OverlaySpec{}, fmt.Errorf("rede overlay incompleta")
		}
	}
	return s, nil
}

func RenderOverlayNft(s OverlaySpec) string {
	var b strings.Builder
	b.WriteString("# Gerado por xvpn-user-provision overlay-apply. Não edite.\n")
	b.WriteString("destroy table inet xvpn-overlay\n")
	b.WriteString("table inet xvpn-overlay {\n")
	b.WriteString("  chain forward {\n")
	b.WriteString("    type filter hook forward priority 10; policy accept;\n")
	for i, n := range s.Networks {
		for j, m := range s.Networks {
			if i == j {
				continue
			}
			b.WriteString("    ip saddr " + n.CIDR + " ip daddr " + m.CIDR + " jump xvpn-x-" + strconv.FormatUint(uint64(n.ID), 10) + "-" + strconv.FormatUint(uint64(m.ID), 10) + ";\n")
		}
	}
	b.WriteString("  }\n")
	type key struct{ src, dst uint }
	idByCIDR := map[string]uint{}
	for _, n := range s.Networks {
		idByCIDR[n.CIDR] = n.ID
	}
	chains := map[key]*strings.Builder{}
	chain := func(srcCIDR, dstCIDR string) *strings.Builder {
		sk := key{idByCIDR[srcCIDR], idByCIDR[dstCIDR]}
		if sk.src == 0 || sk.dst == 0 {
			return nil
		}
		if chains[sk] == nil {
			chains[sk] = &strings.Builder{}
		}
		return chains[sk]
	}
	for _, r := range s.Rules {
		c := chain(r.SrcCIDR, r.DstCIDR)
		if c == nil {
			continue
		}
		if r.Action != "allow" {
			continue
		}
		ports := formatNftPorts(r.Ports)
		proto := strings.ToLower(r.Proto)
		if proto == "" || proto == "any" {
			if ports == "" {
				c.WriteString("    accept\n")
				continue
			}
			c.WriteString("    tcp dport " + ports + " accept\n")
			c.WriteString("    udp dport " + ports + " accept\n")
			continue
		}
		if ports == "" {
			c.WriteString("    meta l4proto " + proto + " accept\n")
			continue
		}
		c.WriteString("    " + proto + " dport " + ports + " accept\n")
	}
	for i, n := range s.Networks {
		for j, m := range s.Networks {
			if i == j {
				continue
			}
			name := "xvpn-x-" + strconv.FormatUint(uint64(n.ID), 10) + "-" + strconv.FormatUint(uint64(m.ID), 10)
			b.WriteString("  chain " + name + " {\n")
			sk := key{n.ID, m.ID}
			if body := chains[sk]; body != nil {
				b.WriteString(body.String())
			}
			b.WriteString("    drop\n")
			b.WriteString("  }\n")
		}
	}
	b.WriteString("}\n")
	var exits []string
	for _, n := range s.Networks {
		if n.Exit {
			exits = append(exits, n.CIDR)
		}
	}
	if len(exits) > 0 {
		b.WriteString("destroy table ip xvpn-overlay-nat\n")
		b.WriteString("table ip xvpn-overlay-nat {\n")
		b.WriteString("  chain postrouting {\n")
		b.WriteString("    type nat hook postrouting priority 100; policy accept;\n")
		for _, c := range exits {
			b.WriteString("    ip saddr " + c + " oif != \"wg0\" masquerade\n")
		}
		b.WriteString("  }\n")
		b.WriteString("}\n")
	}
	return b.String()
}

func formatNftPorts(ports []int) string {
	if len(ports) == 0 {
		return ""
	}
	if len(ports) == 1 {
		return strconv.Itoa(ports[0])
	}
	parts := make([]string, len(ports))
	for i, p := range ports {
		parts[i] = strconv.Itoa(p)
	}
	return "{ " + strings.Join(parts, ", ") + " }"
}

func ApplyOverlay(r Runner, stdin io.Reader) error {
	raw, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("lendo payload overlay: %w", err)
	}
	s, err := ParseOverlaySpec(raw)
	if err != nil {
		return err
	}
	script := RenderOverlayNft(s)
	if err := r.MkdirAll("/etc/xvpn", 0o755); err != nil {
		return fmt.Errorf("mkdir /etc/xvpn: %w", err)
	}
	if err := r.WriteFile(overlayNftPath, script, 0644); err != nil {
		return fmt.Errorf("gravando %s: %w", overlayNftPath, err)
	}
	return r.NftFile(overlayNftPath)
}
