package helper

import (
	"os/user"
	"path/filepath"
	"strconv"
	"testing"
)

func TestResolveSMBMount_RejectsBadInput(t *testing.T) {
	if _, err := resolveSMBMount(MountSMBRequest{Host: "1.2.3.4", Share: "shared"}, 1, 1); err == nil {
		t.Fatal("host estranho deveria falhar")
	}
	if _, err := resolveSMBMount(MountSMBRequest{Host: serverVPNAddress, Share: "../etc"}, 1, 1); err == nil {
		t.Fatal("share path traversal")
	}
	if _, err := resolveSMBMount(MountSMBRequest{Host: serverVPNAddress, Share: "shared"}, 0, 0); err == nil {
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
		t.Skip("gid")
	}
	got, err := resolveSMBMount(MountSMBRequest{Host: serverVPNAddress, Share: "shared"}, uid, gid)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(u.HomeDir, "XVPN", "Compartilhado")
	if got.Mountpoint != want {
		t.Fatalf("mountpoint %q want %q", got.Mountpoint, want)
	}
	home, err := resolveSMBMount(MountSMBRequest{Host: serverVPNAddress, Share: "home-rootkit"}, uid, gid)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(home.Mountpoint) != "Meus arquivos" {
		t.Fatalf("home folder %q", home.Mountpoint)
	}
}

func TestUnescapeProcMount(t *testing.T) {
	got := unescapeProcMount(`/home/wiz/XVPN/Meus\040arquivos`)
	if got != "/home/wiz/XVPN/Meus arquivos" {
		t.Fatalf("got %q", got)
	}
}

func TestPathUnderHome(t *testing.T) {
	if !pathUnderHome("/home/wiz/XVPN/Compartilhado", "/home/wiz") {
		t.Fatal("deveria aceitar filho do home")
	}
	if pathUnderHome("/tmp/evil", "/home/wiz") {
		t.Fatal("não deveria aceitar path fora do home")
	}
	if pathUnderHome("/home/wiz/../root", "/home/wiz") {
		t.Fatal("não deveria aceitar ..")
	}
}
