package helper

import (
	"os/user"
	"path/filepath"
	"strconv"
	"testing"
)

func TestResolveSMBMount_RejectsBadInput(t *testing.T) {
	if _, err := resolveSMBMount(MountSMBRequest{Host: "1.2.3.4", Share: "shared", UID: 1, GID: 1}); err == nil {
		t.Fatal("host estranho deveria falhar")
	}
	if _, err := resolveSMBMount(MountSMBRequest{Host: serverVPNAddress, Share: "../etc", UID: 1, GID: 1}); err == nil {
		t.Fatal("share path traversal")
	}
	if _, err := resolveSMBMount(MountSMBRequest{Host: serverVPNAddress, Share: "shared", UID: 0, GID: 0}); err == nil {
		t.Fatal("uid 0")
	}
}

func TestResolveSMBMount_CurrentUser(t *testing.T) {
	u, err := user.Current()
	if err != nil {
		t.Skip(err)
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil || uid < 1 {
		t.Skip("uid")
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		t.Skip(err)
	}
	got, err := resolveSMBMount(MountSMBRequest{Host: serverVPNAddress, Share: "shared", UID: uid, GID: gid})
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(u.HomeDir, "XVPN", "Compartilhado")
	if got.Mountpoint != want {
		t.Fatalf("mountpoint %q want %q", got.Mountpoint, want)
	}
	home, err := resolveSMBMount(MountSMBRequest{Host: serverVPNAddress, Share: "home-rootkit", UID: uid, GID: gid})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(home.Mountpoint) != "Meus arquivos" {
		t.Fatalf("home folder %q", home.Mountpoint)
	}
}
