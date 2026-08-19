package provision

import (
	"fmt"
	"strings"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/corpdns"
)

func TestValidDemoName(t *testing.T) {
	n, err := ValidDemoName("Vite")
	if err != nil || n != "vite" {
		t.Fatalf("got %q %v", n, err)
	}
	n, err = ValidDemoName("demo-app")
	if err != nil || n != "app" {
		t.Fatalf("got %q %v", n, err)
	}
	if DemoHostname("app") != "demo-app.corp.ihuull.com" {
		t.Fatal(DemoHostname("app"))
	}
	if DefaultDemoName("3ceec32ad760") != "cs-3ceec32ad760" {
		t.Fatal(DefaultDemoName("3ceec32ad760"))
	}
	if DemoHostname(DefaultDemoName("3ceec32ad760")) != "demo-cs-3ceec32ad760.corp.ihuull.com" {
		t.Fatal(DemoHostname(DefaultDemoName("3ceec32ad760")))
	}
	if _, err := ValidDemoName("x.git"); err == nil {
		t.Fatal("rótulo com ponto")
	}
	if _, err := ValidDemoName(""); err == nil {
		t.Fatal("vazio")
	}
}

func TestValidContainerIP(t *testing.T) {
	ip, err := validContainerIP("172.17.0.2")
	if err != nil || ip != "172.17.0.2" {
		t.Fatalf("%q %v", ip, err)
	}
	for _, bad := range []string{"10.66.66.1", "127.0.0.1", "172.17.0.1", "8.8.8.8", "172.18.0.2"} {
		if _, err := validContainerIP(bad); err == nil {
			t.Fatalf("aceitou %s", bad)
		}
	}
}

func TestApplyCodespace_DemoWritesDnsmasq(t *testing.T) {
	root := t.TempDir()
	csRoot := root + "/codespaces"
	gitRoot := root + "/git"
	id := "aabbccddeeff"
	ws := csRoot + "/alice/lab/" + id + "/workspace"
	payload := `{
		"action":"demo","id":"` + id + `","workspace":"` + ws + `",
		"demo_name":"vite"
	}`
	f := newFakeCs()
	if err := ApplyCodespace(f, strings.NewReader(payload), csRoot, gitRoot); err != nil {
		t.Fatal(err)
	}
	hosts := f.writes[corpdns.DemoHostsPath]
	if !strings.Contains(hosts, "10.66.66.254 demo-vite.corp.ihuull.com") {
		t.Fatalf("demo.hosts: %q", hosts)
	}
	conf := f.writes[demoDnsmasqConf]
	if strings.Contains(conf, "address=/") {
		t.Fatalf("address=/ perde pro catch-all: %q", conf)
	}
	joined := ""
	for _, c := range f.host {
		joined += strings.Join(c, " ") + "\n"
	}
	if !strings.Contains(joined, "DNAT") || !strings.Contains(joined, "172.17.0.2") {
		t.Fatalf("iptables: %s", joined)
	}
	if !strings.Contains(joined, "multiport") || !strings.Contains(joined, "80,443") {
		t.Fatalf("DNAT deve reservar 80/443 ao nginx offline: %s", joined)
	}
	if strings.Contains(joined, "-s 172.17.0.2 -d 10.66.66.0/24") {
		t.Fatal("FORWARD não deve permitir egress livre do container para a VPN")
	}
	if !strings.Contains(joined, "RELATED,ESTABLISHED") {
		t.Fatalf("FORWARD deve aceitar só respostas conntrack: %s", joined)
	}
	if !strings.Contains(f.writes[demoNginxSnippet], "demo-vite.corp.ihuull.com") {
		t.Fatalf("nginx demo: %q", f.writes[demoNginxSnippet])
	}
	if strings.Contains(joined, "10.66.66.1") && strings.Contains(joined, "DNAT") && strings.Contains(joined, "-d 10.66.66.1") {
		t.Fatal("DNAT não pode usar o IP do Samba/Nginx")
	}
}

func TestReloadDemoDnsmasq_RestartsWhenAddnHostsMissing(t *testing.T) {
	f := newFakeCs()
	f.writes[dnsmasqMainPath] = "listen-address=10.66.66.1\naddn-hosts=/etc/xvpn/dnsmasq-records.hosts\n"
	if err := reloadDemoDnsmasq(f); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.writes[dnsmasqMainPath], "addn-hosts=/etc/xvpn/demo.hosts") {
		t.Fatalf("main: %s", f.writes[dnsmasqMainPath])
	}
	joined := ""
	for _, c := range f.host {
		joined += strings.Join(c, " ") + "\n"
	}
	if !strings.Contains(joined, "systemctl restart dnsmasq") {
		t.Fatalf("esperava restart: %s", joined)
	}
}

func TestEnsureDemoVIP_Idempotent(t *testing.T) {
	f := newFakeCs()
	f.hostFail = map[string]error{
		"ip addr add 10.66.66.254/32 dev wg0": fmt.Errorf("Error: ipv4: Address already assigned"),
	}
	if err := ensureDemoVIP(f); err != nil {
		t.Fatal(err)
	}
}
