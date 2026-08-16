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
		{"http://xvpn.ihuull.com/admin", ""},
		{"http://localhost/admin", "http://localhost/admin"},
		{"javascript:alert(1)", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := SafeReturnURL(tc.in); got != tc.want {
			t.Fatalf("SafeReturnURL(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestTrustedHandoffOrigin(t *testing.T) {
	if !TrustedHandoffOrigin("https://xauth.ihuull.com") {
		t.Fatal("xauth https deveria passar")
	}
	if TrustedHandoffOrigin("https://evil.example") {
		t.Fatal("origem estranha")
	}
	if TrustedHandoffOrigin("http://xvpn.ihuull.com") {
		t.Fatal("http em produção")
	}
	if !TrustedHandoffOrigin("http://localhost:5173") {
		t.Fatal("http localhost é o dev")
	}
	if TrustedHandoffOrigin("null") {
		t.Fatal("Origin null")
	}
}

func TestHandoffAllowed(t *testing.T) {
	if !HandoffAllowed("https://xauth.ihuull.com", "", "", "xvpn.ihuull.com") {
		t.Fatal("Origin xauth")
	}
	if HandoffAllowed("https://evil.example", "", "cross-site", "xvpn.ihuull.com") {
		t.Fatal("Origin estranha")
	}
	if !HandoffAllowed("", "", "same-site", "xvpn.ihuull.com") {
		t.Fatal("POST same-site sem Origin (Referrer-Policy do xauth)")
	}
	if !HandoffAllowed("", "", "same-origin", "xvpn.ihuull.com") {
		t.Fatal("same-origin")
	}
	if HandoffAllowed("", "", "cross-site", "xvpn.ihuull.com") {
		t.Fatal("cross-site sem Origin")
	}
	if HandoffAllowed("", "", "", "xvpn.ihuull.com") {
		t.Fatal("sem Origin nem Sec-Fetch-Site")
	}
	if HandoffAllowed("", "", "same-site", "evil.example") {
		t.Fatal("same-site em host estranho")
	}
	if HandoffAllowed("null", "", "same-site", "xvpn.ihuull.com") {
		t.Fatal("Origin null (iframe sandbox) não é same-site confiável")
	}
}
