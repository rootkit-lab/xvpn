package main

import "testing"

func TestGreeting(t *testing.T) {
	if got := greeting(""); got != "olá, XCODESPACES" {
		t.Fatalf("got %q", got)
	}
	if got := greeting("ihuull"); got != "olá, ihuull" {
		t.Fatalf("got %q", got)
	}
}
