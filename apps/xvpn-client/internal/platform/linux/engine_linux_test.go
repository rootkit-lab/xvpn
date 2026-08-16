//go:build linux

package linux

import (
	"slices"
	"testing"
)

func TestSplitHorizonResolvectlArgs_DoesNotHijackPublicDNS(t *testing.T) {
	dnsArgs, domainArgs, defaultRouteArgs := splitHorizonResolvectlArgs("xvpn0", []string{"10.66.66.1"})

	if !slices.Equal(dnsArgs, []string{"dns", "xvpn0", "10.66.66.1"}) {
		t.Fatalf("dns: %v", dnsArgs)
	}
	if slices.Contains(domainArgs, "~.") {
		t.Fatal("domain não pode incluir ~. — isso sequestra o DNS público (Cursor/apt)")
	}
	if !slices.Equal(domainArgs, []string{"domain", "xvpn0", "~corp.ihuull.com"}) {
		t.Fatalf("domain: %v", domainArgs)
	}
	if !slices.Equal(defaultRouteArgs, []string{"default-route", "xvpn0", "no"}) {
		t.Fatalf("default-route precisa ser no, obtido %v", defaultRouteArgs)
	}
}
