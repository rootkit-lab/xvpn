package provision

import (
	"os"
	"strings"
	"testing"
)

type fakeSvcRunner struct {
	bins     map[string]string
	writes   map[string]string
	mkdirs   []string
	syscalls []string
	pkgs     []string
}

func newFakeSvc() *fakeSvcRunner {
	return &fakeSvcRunner{
		bins:   map[string]string{"redis-server": "/usr/bin/redis-server"},
		writes: map[string]string{},
	}
}

func (f *fakeSvcRunner) LookPath(bin string) (string, error) {
	if p, ok := f.bins[bin]; ok {
		return p, nil
	}
	return "", os.ErrNotExist
}
func (f *fakeSvcRunner) InstallPackage(pkg string) error {
	f.pkgs = append(f.pkgs, pkg)
	f.bins[strings.TrimSuffix(pkg, "-server")] = "/usr/bin/" + pkg
	if pkg == "redis-server" {
		f.bins["redis-server"] = "/usr/bin/redis-server"
	}
	if pkg == "mongodb" {
		f.bins["mongod"] = "/usr/bin/mongod"
	}
	if pkg == "rabbitmq-server" {
		f.bins["rabbitmq-server"] = "/usr/sbin/rabbitmq-server"
	}
	if pkg == "nginx" {
		f.bins["nginx"] = "/usr/sbin/nginx"
	}
	return nil
}
func (f *fakeSvcRunner) WriteFile(path, content string, _ os.FileMode) error {
	f.writes[path] = content
	return nil
}
func (f *fakeSvcRunner) MkdirAll(path string, _ os.FileMode) error {
	f.mkdirs = append(f.mkdirs, path)
	return nil
}
func (f *fakeSvcRunner) Chmod(string, os.FileMode) error { return nil }
func (f *fakeSvcRunner) FileExists(path string) (bool, error) {
	_, ok := f.writes[path]
	return ok, nil
}
func (f *fakeSvcRunner) RemoveFile(path string) error {
	delete(f.writes, path)
	return nil
}
func (f *fakeSvcRunner) Systemctl(args ...string) error {
	f.syscalls = append(f.syscalls, strings.Join(args, " "))
	return nil
}

func TestParseSvcSpec_RejectsUnsafeBindAndReservedPort(t *testing.T) {
	cases := []string{
		`{"action":"apply","slug":"cache","kind":"redis","bind":"0.0.0.0","port":6379}`,
		`{"action":"apply","slug":"cache","kind":"redis","bind":"1.2.3.4","port":6379}`,
		`{"action":"apply","slug":"db","kind":"mongo","bind":"10.66.66.1","port":27017}`,
		`{"action":"apply","slug":"api","kind":"lb","bind":"10.66.66.1","port":8080,"backends":["10.66.66.1:9000"]}`,
		`{"action":"apply","slug":"x","kind":"redis","bind":"10.66.66.1","port":6379}`,
	}
	for _, raw := range cases {
		if _, err := ParseSvcSpec([]byte(raw)); err == nil {
			t.Fatalf("deveria rejeitar %s", raw)
		}
	}
}

func TestParseSvcSpec_AcceptsWG0AndLoopback(t *testing.T) {
	for _, bind := range []string{"10.66.66.1", "127.0.0.1", "10.66.66.9"} {
		_, err := ParseSvcSpec([]byte(`{"action":"apply","slug":"cache","kind":"redis","bind":"` + bind + `","port":6379,"password":"s"}`))
		if err != nil {
			t.Fatalf("bind %s: %v", bind, err)
		}
	}
}

func TestApplyService_RedisBindsOnlyGivenIP(t *testing.T) {
	f := newFakeSvc()
	raw := `{"action":"apply","slug":"cache","kind":"redis","bind":"10.66.66.1","port":6379,"password":"segredo"}`
	if err := ApplyService(f, strings.NewReader(raw)); err != nil {
		t.Fatal(err)
	}
	conf := f.writes["/opt/xvpn/data/svc/cache/redis.conf"]
	if !strings.Contains(conf, "bind 10.66.66.1") {
		t.Fatalf("bind ausente: %s", conf)
	}
	if strings.Contains(conf, "0.0.0.0") {
		t.Fatal("config não pode mencionar 0.0.0.0")
	}
	if !strings.Contains(conf, "requirepass segredo") {
		t.Fatal("senha ausente")
	}
	unit := f.writes["/etc/systemd/system/xvpn-svc-cache.service"]
	if !strings.Contains(unit, "/usr/bin/redis-server") {
		t.Fatalf("unit: %s", unit)
	}
	joined := strings.Join(f.syscalls, ";")
	if !strings.Contains(joined, "enable --now xvpn-svc-cache.service") {
		t.Fatalf("systemctl: %v", f.syscalls)
	}
}

func TestApplyService_MongoNever27017(t *testing.T) {
	f := newFakeSvc()
	raw := `{"action":"apply","slug":"appdb","kind":"mongo","bind":"10.66.66.1","port":27018}`
	if err := ApplyService(f, strings.NewReader(raw)); err != nil {
		t.Fatal(err)
	}
	conf := f.writes["/opt/xvpn/data/svc/appdb/mongod.conf"]
	if strings.Contains(conf, "27017") {
		t.Fatal("mongod gerenciado não pode citar 27017")
	}
	if !strings.Contains(conf, "bindIp: 10.66.66.1") {
		t.Fatalf("bind: %s", conf)
	}
}

func TestApplyService_StopRemovesUnit(t *testing.T) {
	f := newFakeSvc()
	f.writes["/etc/systemd/system/xvpn-svc-cache.service"] = "unit"
	if err := ApplyService(f, strings.NewReader(`{"action":"stop","slug":"cache","kind":"redis","bind":"10.66.66.1","port":6379}`)); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.writes["/etc/systemd/system/xvpn-svc-cache.service"]; ok {
		t.Fatal("unit deveria ter sido removida")
	}
}

func TestApplyService_LBRejectsZeroBackend(t *testing.T) {
	f := newFakeSvc()
	err := ApplyService(f, strings.NewReader(`{"action":"apply","slug":"edge","kind":"lb","bind":"10.66.66.1","port":9080,"backends":["0.0.0.0:80"]}`))
	if err == nil {
		t.Fatal("backend 0.0.0.0 deveria falhar")
	}
}
