package gitssh

import (
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/forge"
)

func TestParseOriginalCommand(t *testing.T) {
	cases := []struct {
		in      string
		service string
		repo    string
		ok      bool
	}{
		{"git-upload-pack '/rootkit-lab/evilsuite-v3.git'", "upload-pack", "rootkit-lab/evilsuite-v3", true},
		{"git-receive-pack '/xcorp/xvpn.git'", "receive-pack", "xcorp/xvpn", true},
		{"git-upload-pack rootkit-lab/foo.git", "", "", false},
		{"rm -rf /", "", "", false},
	}
	for _, tc := range cases {
		got, err := ParseOriginalCommand(tc.in)
		if tc.ok && err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("%q: esperava erro", tc.in)
		}
		if tc.ok {
			if got.Service != tc.service || got.Repo != tc.repo {
				t.Fatalf("%q: got %+v", tc.in, got)
			}
		}
	}
}

func TestParseOriginalCommand_SplitRepo(t *testing.T) {
	_, err := ParseOriginalCommand("git-upload-pack '/../escape.git'")
	if err == nil {
		t.Fatal("esperava rejeição de path traversal")
	}
	_, _, err2 := forge.SplitRepo("bad")
	if err2 == nil {
		t.Fatal("split deveria falhar")
	}
}
