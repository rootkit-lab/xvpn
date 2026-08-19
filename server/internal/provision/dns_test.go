package provision

import (
	"strings"
	"testing"
)

func TestApplyDNSWritesBindAndReloads(t *testing.T) {
	r := newFakeRunner()
	payload := `{"forwarders":["8.8.8.8"],"cache_size":100,"catch_all":true,"records":[{"hostname":"xchat.corp.ihuull.com","ipv4":"10.66.66.1"}]}`
	if err := ApplyDNS(r, strings.NewReader(payload)); err != nil {
		t.Fatal(err)
	}
	main := r.files[dnsmasqMainPath]
	if !strings.Contains(main, "listen-address=10.66.66.1") {
		t.Fatalf("main: %s", main)
	}
	if strings.Contains(main, "listen-address=0.0.0.0") {
		t.Fatal("não pode bindar 0.0.0.0")
	}
	if !strings.Contains(main, "addn-hosts=/etc/xvpn/dnsmasq-records.hosts") {
		t.Fatalf("hosts fora de dnsmasq.d: %s", main)
	}
	if !strings.Contains(r.files[dnsmasqHostsPath], "xchat.corp.ihuull.com") {
		t.Fatalf("hosts: %s", r.files[dnsmasqHostsPath])
	}
	found := false
	for _, c := range r.calls {
		if c == "ReloadDnsmasq()" {
			found = true
		}
	}
	if !found {
		t.Fatalf("ReloadDnsmasq não chamado: %v", r.calls)
	}
}

func TestApplyDNSRejectsBadForwarder(t *testing.T) {
	r := newFakeRunner()
	err := ApplyDNS(r, strings.NewReader(`{"forwarders":["0.0.0.0"],"cache_size":1,"records":[]}`))
	if err == nil {
		t.Fatal("esperava erro")
	}
}
