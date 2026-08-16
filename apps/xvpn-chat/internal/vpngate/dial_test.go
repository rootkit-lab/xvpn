package vpngate

import "testing"

func TestRewriteAddr(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"xchat.corp.ihuull.com:443", "10.66.66.1:443"},
		{"xgroup.corp.ihuull.com:443", "10.66.66.1:443"},
		{"xdriver.corp.ihuull.com:80", "10.66.66.1:80"},
		{"corp.ihuull.com:443", "10.66.66.1:443"},
		{"XCHAT.CORP.IHUULL.COM:443", "10.66.66.1:443"},
		{"xchat.corp.ihuull.com.:443", "10.66.66.1:443"},
		{"10.66.66.1:443", "10.66.66.1:443"},
		{"example.com:443", "example.com:443"},
		{"xchat.ihuull.com:443", "xchat.ihuull.com:443"},
		{"marketplace.ihuull.com:443", "marketplace.ihuull.com:443"},
	}
	for _, tc := range cases {
		if got := RewriteAddr(tc.in); got != tc.want {
			t.Errorf("RewriteAddr(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestIsCorpHost(t *testing.T) {
	yes := []string{"corp.ihuull.com", "xchat.corp.ihuull.com", "xgroup.corp.ihuull.com.", "XDRIVER.CORP.IHUULL.COM"}
	no := []string{"xchat.ihuull.com", "ihuull.com", "evilcorp.ihuull.com", "corp.ihuull.com.evil.example"}
	for _, h := range yes {
		if !IsCorpHost(h) {
			t.Errorf("IsCorpHost(%q) = false, want true", h)
		}
	}
	for _, h := range no {
		if IsCorpHost(h) {
			t.Errorf("IsCorpHost(%q) = true, want false", h)
		}
	}
}
