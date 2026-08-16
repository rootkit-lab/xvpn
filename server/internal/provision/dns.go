package provision

import (
	"fmt"
	"io"

	"github.com/rootkit-lab/xvpn/server/internal/corpdns"
)

const (
	dnsmasqMainPath  = "/etc/dnsmasq.d/xvpn-corp.conf"
	dnsmasqHostsPath = "/etc/dnsmasq.d/xvpn-records.hosts"
)

// ApplyDNS writes the intranet dnsmasq files from a JSON payload on r
// and reloads the service. Bind is forced to 10.66.66.1.
func ApplyDNS(r Runner, stdin io.Reader) error {
	raw, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("lendo payload DNS: %w", err)
	}
	p, err := corpdns.ParseApplyPayload(raw)
	if err != nil {
		return err
	}
	main := corpdns.RenderMain(p)
	if err := corpdns.AssertSafeMain(main); err != nil {
		return err
	}
	hosts := corpdns.RenderHosts(p.Records)
	if err := r.WriteFile(dnsmasqMainPath, main, 0644); err != nil {
		return fmt.Errorf("gravando %s: %w", dnsmasqMainPath, err)
	}
	if err := r.WriteFile(dnsmasqHostsPath, hosts, 0644); err != nil {
		return fmt.Errorf("gravando %s: %w", dnsmasqHostsPath, err)
	}
	if err := r.ReloadDnsmasq(); err != nil {
		return err
	}
	return nil
}
