package store

import (
	"net"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func overlayTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Device{}, &MeshServer{}, &OverlayNetwork{}, &NetworkMember{}, &NetworkRule{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestSeedOverlayNetworks_CreatesInfraUsersAndRules(t *testing.T) {
	db := overlayTestDB(t)
	if err := SeedOverlayNetworks(db); err != nil {
		t.Fatal(err)
	}
	infra, err := NetworkByKind(db, NetworkKindInfra)
	if err != nil || infra.CIDR != InfraCIDR || infra.Exit {
		t.Fatalf("infra: %+v %v", infra, err)
	}
	users, err := NetworkByKind(db, NetworkKindUsers)
	if err != nil || users.CIDR != UsersCIDR || !users.Exit {
		t.Fatalf("users: %+v %v", users, err)
	}
	var n int64
	if err := db.Model(&NetworkRule{}).Count(&n).Error; err != nil || n != 3 {
		t.Fatalf("regras: %d %v", n, err)
	}
}

func TestSeedOverlayNetworks_MovesUserOffInfra(t *testing.T) {
	db := overlayTestDB(t)
	u := User{Username: "alice", PasswordHash: "x"}
	if err := db.AutoMigrate(&User{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&u).Error; err != nil {
		t.Fatal(err)
	}
	d := Device{UserID: u.ID, Name: "note", PublicKey: "k1", AllowedIP: "10.66.66.9/32"}
	if err := db.Create(&d).Error; err != nil {
		t.Fatal(err)
	}
	if err := SeedOverlayNetworks(db); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&d, d.ID).Error; err != nil {
		t.Fatal(err)
	}
	users, _ := NetworkByKind(db, NetworkKindUsers)
	if d.NetworkID != users.ID || !CIDRContainsIP(UsersCIDR, d.AllowedIP) {
		t.Fatalf("user deveria ir para users, obtido net=%d ip=%s", d.NetworkID, d.AllowedIP)
	}
}

func TestSeedOverlayNetworks_KeepsMeshOnInfra(t *testing.T) {
	db := overlayTestDB(t)
	if err := db.AutoMigrate(&User{}); err != nil {
		t.Fatal(err)
	}
	u := User{Username: "ops", PasswordHash: "x"}
	if err := db.Create(&u).Error; err != nil {
		t.Fatal(err)
	}
	d := Device{UserID: u.ID, Name: "mesh-data", PublicKey: "k2", AllowedIP: "10.66.66.8/32"}
	if err := db.Create(&d).Error; err != nil {
		t.Fatal(err)
	}
	s := MeshServer{BitLaunchID: "m", Name: "data", Hostname: "data", Role: ServerRoleMesh, DeviceID: &d.ID}
	if err := db.Create(&s).Error; err != nil {
		t.Fatal(err)
	}
	if err := SeedOverlayNetworks(db); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&d, d.ID).Error; err != nil {
		t.Fatal(err)
	}
	infra, _ := NetworkByKind(db, NetworkKindInfra)
	if d.NetworkID != infra.ID || !CIDRContainsIP(InfraCIDR, d.AllowedIP) {
		t.Fatalf("mesh deveria ficar na infra, obtido net=%d ip=%s", d.NetworkID, d.AllowedIP)
	}
}

func TestValidateOverlayCIDR(t *testing.T) {
	if err := ValidateOverlayCIDR(NetworkKindCustom, "10.10.1.0/24", nil); err == nil {
		t.Fatal("10.10 deveria ser recusado")
	}
	if err := ValidateOverlayCIDR(NetworkKindCustom, "10.66.66.0/24", nil); err == nil {
		t.Fatal("overlap infra")
	}
	if err := ValidateOverlayCIDR(NetworkKindCustom, "10.66.80.0/24", nil); err == nil {
		t.Fatal("overlap users")
	}
	if err := ValidateOverlayCIDR(NetworkKindCustom, "10.66.81.0/24", nil); err != nil {
		t.Fatal(err)
	}
	if err := ValidateOverlayCIDR(NetworkKindInfra, "10.66.66.0/24", nil); err != nil {
		t.Fatal(err)
	}
}

func TestClientAllowedIPs_MeshIsInfraOnly(t *testing.T) {
	db := overlayTestDB(t)
	if err := SeedOverlayNetworks(db); err != nil {
		t.Fatal(err)
	}
	infra, _ := NetworkByKind(db, NetworkKindInfra)
	d := Device{UserID: 1, Name: "mesh", PublicKey: "k", AllowedIP: "10.66.66.8/32", NetworkID: infra.ID}
	got, err := ClientAllowedIPs(db, d)
	if err != nil || got != InfraCIDR {
		t.Fatalf("got %q %v", got, err)
	}
}

func TestClientAllowedIPs_UsersExitIsFullTunnel(t *testing.T) {
	db := overlayTestDB(t)
	if err := SeedOverlayNetworks(db); err != nil {
		t.Fatal(err)
	}
	users, _ := NetworkByKind(db, NetworkKindUsers)
	d := Device{UserID: 1, Name: "note", PublicKey: "k", AllowedIP: "10.66.80.2/32", NetworkID: users.ID}
	got, err := ClientAllowedIPs(db, d)
	if err != nil || got != "0.0.0.0/0, ::/0" {
		t.Fatalf("got %q %v", got, err)
	}
}

func TestClientAllowedIPs_RequiresNetwork(t *testing.T) {
	db := overlayTestDB(t)
	_, err := ClientAllowedIPs(db, Device{Name: "x", PublicKey: "k", AllowedIP: "10.66.66.2/32"})
	if err == nil {
		t.Fatal("sem NetworkID deveria falhar")
	}
}

func TestOverlayContainsIP(t *testing.T) {
	db := overlayTestDB(t)
	if err := SeedOverlayNetworks(db); err != nil {
		t.Fatal(err)
	}
	if !OverlayContainsIP(db, net.ParseIP("10.66.80.4")) {
		t.Fatal("users")
	}
	if !OverlayContainsIP(db, net.ParseIP("10.66.66.1")) {
		t.Fatal("infra")
	}
	if OverlayContainsIP(db, net.ParseIP("8.8.8.8")) {
		t.Fatal("público")
	}
}

func TestForwardAllowed_UserCannotReachMongo(t *testing.T) {
	db := overlayTestDB(t)
	if err := SeedOverlayNetworks(db); err != nil {
		t.Fatal(err)
	}
	users, _ := NetworkByKind(db, NetworkKindUsers)
	infra, _ := NetworkByKind(db, NetworkKindInfra)
	var rules []NetworkRule
	_ = db.Find(&rules).Error
	pairs, _ := MembershipPairs(db)
	if !ForwardAllowed(rules, pairs, users.ID, infra.ID, "tcp", 443) {
		t.Fatal("443 corp")
	}
	if ForwardAllowed(rules, pairs, users.ID, infra.ID, "tcp", 27017) {
		t.Fatal("27017 não pode estar no seed")
	}
	if !ForwardAllowed(rules, pairs, infra.ID, infra.ID, "tcp", 27017) {
		t.Fatal("infra↔infra")
	}
}

func TestNextCustomCIDR(t *testing.T) {
	c, err := NextCustomCIDR([]OverlayNetwork{{CIDR: UsersCIDR}})
	if err != nil || c != "10.66.81.0/24" {
		t.Fatalf("got %s %v", c, err)
	}
}
