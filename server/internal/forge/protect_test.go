package forge

import "testing"

func TestMatchProtected(t *testing.T) {
	cases := []struct {
		patterns []string
		ref      string
		want     bool
	}{
		{[]string{"main"}, "refs/heads/main", true},
		{[]string{"master"}, "refs/heads/main", false},
		{[]string{"release/*"}, "refs/heads/release/1.0", true},
		{[]string{"release/*"}, "refs/heads/main", false},
		{[]string{"refs/heads/main"}, "refs/heads/main", true},
		{[]string{"main"}, "refs/tags/v1", false},
		{[]string{"refs/tags/*"}, "refs/tags/v1", true},
		{[]string{"../etc"}, "refs/heads/main", false},
	}
	for _, tc := range cases {
		if got := MatchProtected(tc.patterns, tc.ref); got != tc.want {
			t.Fatalf("MatchProtected(%v, %q)=%v, want %v", tc.patterns, tc.ref, got, tc.want)
		}
	}
}

func TestValidBranchPattern(t *testing.T) {
	if !ValidBranchPattern("main") || !ValidBranchPattern("release/*") {
		t.Fatal("padrões válidos rejeitados")
	}
	if ValidBranchPattern("../etc") || ValidBranchPattern("main;rm") || ValidBranchPattern("") {
		t.Fatal("padrão perigoso aceito")
	}
}

func TestNormalizeSlug(t *testing.T) {
	if NormalizeSlug("XChat.git") != "xchat" {
		t.Fatalf("got %q", NormalizeSlug("XChat.git"))
	}
}
