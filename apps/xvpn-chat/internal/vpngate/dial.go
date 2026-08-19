package vpngate

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// IsCorpHost is true for corp.ihuull.com and any *.corp.ihuull.com label.
// Public marketing hosts (xchat.ihuull.com) stay on the system resolver.
func IsCorpHost(host string) bool {
	h := strings.ToLower(strings.TrimSuffix(host, "."))
	if h == "corp.ihuull.com" {
		return true
	}
	return strings.HasSuffix(h, ".corp.ihuull.com")
}

// RewriteAddr maps intranet host:port to the WireGuard gateway. TLS ServerName
// stays on the original URL host (cert *.corp.ihuull.com).
func RewriteAddr(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		if IsCorpHost(addr) {
			return net.JoinHostPort(CorpIP, "443")
		}
		return addr
	}
	if IsCorpHost(host) {
		return net.JoinHostPort(CorpIP, port)
	}
	return addr
}

func DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	var d net.Dialer
	return d.DialContext(ctx, network, RewriteAddr(addr))
}

func HTTPClient(timeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = DialContext
	return &http.Client{Timeout: timeout, Transport: transport}
}

func WSDialer() *websocket.Dialer {
	d := *websocket.DefaultDialer
	d.NetDialContext = DialContext
	return &d
}
