package provision

import "testing"

func TestSetDiskQuotaMB(t *testing.T) {
	r := newFakeRunner()
	if err := SetDiskQuotaMB(r, "alice", 100); err != nil {
		t.Fatal(err)
	}
	if got := r.calls[len(r.calls)-1]; got != "SetUserQuota(alice,102400)" {
		t.Fatalf("call=%q", got)
	}
	if err := SetDiskQuotaMB(r, "alice", 0); err != nil {
		t.Fatal(err)
	}
	if got := r.calls[len(r.calls)-1]; got != "SetUserQuota(alice,0)" {
		t.Fatalf("clear call=%q", got)
	}
}

func TestSetDiskQuotaMB_InvalidUser(t *testing.T) {
	r := newFakeRunner()
	if err := SetDiskQuotaMB(r, "ROOT", 10); err == nil {
		t.Fatal("esperava ErrInvalidUsername")
	}
}
