package windows

import (
	"net"
	"testing"
)

func TestParseIPv4DefaultGateway(t *testing.T) {
	sample := `
===========================================================================
Active Routes:
Network Destination        Netmask          Gateway       Interface  Metric
          0.0.0.0          0.0.0.0      192.168.1.1     192.168.1.50     25
        127.0.0.0        255.0.0.0         On-link         127.0.0.1    331
===========================================================================
`
	gw, err := parseIPv4DefaultGateway(sample)
	if err != nil {
		t.Fatal(err)
	}
	if !gw.Equal(net.ParseIP("192.168.1.1")) {
		t.Fatalf("gw=%v", gw)
	}
}

func TestParseIPv4DefaultGateway_Missing(t *testing.T) {
	_, err := parseIPv4DefaultGateway("no default here\n")
	if err == nil {
		t.Fatal("esperava erro")
	}
}
