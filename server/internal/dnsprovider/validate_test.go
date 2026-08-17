package dnsprovider

import "testing"

func TestNormalizeZoneRejectsLandpagesAndCorp(t *testing.T) {
	if _, err := NormalizeZone("ldpops.appapisip.com"); err == nil {
		t.Fatal("ldpops deveria ser recusado")
	}
	if _, err := NormalizeZone("corp.ihuull.com"); err == nil {
		t.Fatal("corp não é zona pública")
	}
	z, err := NormalizeZone("App.Ihuull.COM.")
	if err != nil || z != "app.ihuull.com" {
		t.Fatalf("got %q %v", z, err)
	}
}

func TestNormalizeRecordNameBlocksCorp(t *testing.T) {
	if _, err := NormalizeRecordName("ihuull.com", "corp"); err == nil {
		t.Fatal("corp.ihuull.com não pode ser público")
	}
	if _, err := NormalizeRecordName("ihuull.com", "xchat.corp"); err == nil {
		t.Fatal("xchat.corp.ihuull.com não pode ser público")
	}
	n, err := NormalizeRecordName("ihuull.com", "lab")
	if err != nil || n != "lab.ihuull.com" {
		t.Fatalf("got %q %v", n, err)
	}
}

func TestValidatePublicContentRejectsPrivateA(t *testing.T) {
	if err := ValidatePublicContent("A", "10.66.66.1"); err == nil {
		t.Fatal("RFC1918 no A público")
	}
	if err := ValidatePublicContent("A", "206.189.224.72"); err != nil {
		t.Fatal(err)
	}
}
