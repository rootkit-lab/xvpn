package corpdns

import (
	"strings"
	"testing"
)

func TestNormalizeHostname(t *testing.T) {
	ok := []string{"corp.ihuull.com", "xchat.corp.ihuull.com", "APP.CORP.IHUULL.COM."}
	for _, h := range ok {
		if _, err := NormalizeHostname(h); err != nil {
			t.Errorf("%q: %v", h, err)
		}
	}
	bad := []string{"", "xchat.ihuull.com", "a.b.corp.ihuull.com", "evil.com", "corp.ihuull.com.evil"}
	for _, h := range bad {
		if _, err := NormalizeHostname(h); err == nil {
			t.Errorf("%q deveria falhar", h)
		}
	}
}

func TestValidateIPv4(t *testing.T) {
	if _, err := ValidateIPv4("10.66.66.1"); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateIPv4("10.66.66.20"); err != nil {
		t.Fatal(err)
	}
	for _, ip := range []string{"8.8.8.8", "127.0.0.1", "10.10.0.1", "not-an-ip"} {
		if _, err := ValidateIPv4(ip); err == nil {
			t.Errorf("%q deveria falhar", ip)
		}
	}
}

func TestParseApplyPayloadAndRender(t *testing.T) {
	raw := []byte(`{"forwarders":["8.8.8.8","1.1.1.1"],"cache_size":500,"catch_all":true,"records":[{"hostname":"xchat.corp.ihuull.com","ipv4":"10.66.66.1"}]}`)
	p, err := ParseApplyPayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	main := RenderMain(p)
	if err := AssertSafeMain(main); err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"listen-address=10.66.66.1", "server=8.8.8.8", "address=/corp.ihuull.com/10.66.66.1"} {
		if !strings.Contains(main, part) {
			t.Fatalf("main sem %q:\n%s", part, main)
		}
	}
	hosts := RenderHosts(p.Records)
	if !strings.Contains(hosts, "10.66.66.1 xchat.corp.ihuull.com") {
		t.Fatalf("hosts: %q", hosts)
	}
}

func TestParseApplyPayloadStackRecords(t *testing.T) {
	raw := []byte(`{"forwarders":["8.8.8.8"],"cache_size":1,"records":[],"stack_records":[{"hostname":"app.example.com","ipv4":"10.66.66.9"}]}`)
	p, err := ParseApplyPayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.StackRecords) != 1 || p.StackRecords[0].Hostname != "app.example.com" {
		t.Fatalf("%+v", p.StackRecords)
	}
}

func TestParseApplyPayloadRejectsPublicBind(t *testing.T) {
	_, err := ParseApplyPayload([]byte(`{"forwarders":["10.66.66.1"],"cache_size":1,"records":[]}`))
	if err == nil {
		t.Fatal("forwarder do gateway deveria falhar")
	}
}
