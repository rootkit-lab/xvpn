package auth

import "testing"

func TestSafeReturnURL(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"https://xvpn.ihuull.com/admin", "https://xvpn.ihuull.com/admin"},
		{"https://xchat.corp.ihuull.com/social/messages", "https://xchat.corp.ihuull.com/social/messages"},
		{"https://vpn.ihuull.com/social", "https://xvpn.ihuull.com/social"},
		{"https://xchat.corp.ihuull.com/admin", PanelOrigin + "/admin"},
		{"https://evil.example/phish", ""},
		{"javascript:alert(1)", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := SafeReturnURL(tc.in); got != tc.want {
			t.Fatalf("SafeReturnURL(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}
