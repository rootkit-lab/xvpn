package provision

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	svcDataRoot = "/opt/xvpn/data/svc"
	svcUnitDir  = "/etc/systemd/system"
	wgListenIP  = "10.66.66.1"
	loopbackIP  = "127.0.0.1"
	corpNetCIDR = "10.66.66.0/24"
)

// Control-plane Mongo and other host listeners we never steal.
var reservedPorts = map[uint16]string{
	22:    "ssh",
	53:    "dns",
	80:    "http",
	139:   "nmbd",
	443:   "https",
	445:   "samba",
	8080:  "xvpn-api",
	27017: "control-plane-mongo",
	51820: "wireguard",
}

// SvcSpec é o JSON do subcomando svc-apply (stdin).
type SvcSpec struct {
	Action   string   `json:"action"`
	Slug     string   `json:"slug"`
	Kind     string   `json:"kind"`
	Bind     string   `json:"bind"`
	Port     uint16   `json:"port"`
	Password string   `json:"password"`
	Backends []string `json:"backends,omitempty"`
}

// SvcRunner isola apt/systemctl/arquivos para ApplyService ser testável
// sem root e sem instalar Redis no CI.
type SvcRunner interface {
	LookPath(bin string) (string, error)
	InstallPackage(pkg string) error
	WriteFile(path, content string, perm os.FileMode) error
	MkdirAll(path string, perm os.FileMode) error
	Chmod(path string, perm os.FileMode) error
	FileExists(path string) (bool, error)
	RemoveFile(path string) error
	Systemctl(args ...string) error
}

type osSvcRunner struct{}

// NewSvcRunner devolve o runner de produção (root via sudoers).
func NewSvcRunner() SvcRunner { return osSvcRunner{} }

func (osSvcRunner) LookPath(bin string) (string, error) {
	return exec.LookPath(bin)
}

func (osSvcRunner) InstallPackage(pkg string) error {
	if !validPkgName(pkg) {
		return fmt.Errorf("pacote inválido")
	}
	cmd := exec.Command("apt-get", "install", "-y", "--no-install-recommends", pkg)
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("instalando %s: %w: %s", pkg, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (osSvcRunner) WriteFile(path, content string, perm os.FileMode) error {
	return os.WriteFile(path, []byte(content), perm)
}

func (osSvcRunner) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (osSvcRunner) Chmod(path string, perm os.FileMode) error {
	return os.Chmod(path, perm)
}

func (osSvcRunner) FileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (osSvcRunner) RemoveFile(path string) error {
	return os.Remove(path)
}

func (osSvcRunner) Systemctl(args ...string) error {
	cmd := exec.Command("systemctl", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func validPkgName(pkg string) bool {
	if pkg == "" || len(pkg) > 64 {
		return false
	}
	for _, c := range pkg {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '.' || c == '+' {
			continue
		}
		return false
	}
	return true
}

// ParseSvcSpec valida o JSON antes de qualquer syscall.
func ParseSvcSpec(raw []byte) (SvcSpec, error) {
	var spec SvcSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return SvcSpec{}, fmt.Errorf("payload de serviço inválido")
	}
	spec.Action = strings.ToLower(strings.TrimSpace(spec.Action))
	spec.Slug = strings.ToLower(strings.TrimSpace(spec.Slug))
	spec.Kind = strings.ToLower(strings.TrimSpace(spec.Kind))
	spec.Bind = strings.TrimSpace(spec.Bind)
	if spec.Action != "apply" && spec.Action != "stop" {
		return SvcSpec{}, fmt.Errorf("action deve ser apply ou stop")
	}
	if !validSvcSlug(spec.Slug) {
		return SvcSpec{}, fmt.Errorf("slug inválido")
	}
	switch spec.Kind {
	case "mongo", "redis", "rabbitmq", "lb":
	default:
		return SvcSpec{}, fmt.Errorf("kind inválido")
	}
	if err := validateSvcBind(spec.Bind); err != nil {
		return SvcSpec{}, err
	}
	if spec.Port == 0 {
		return SvcSpec{}, fmt.Errorf("porta obrigatória")
	}
	if reason, ok := reservedPorts[spec.Port]; ok {
		return SvcSpec{}, fmt.Errorf("porta %d reservada (%s)", spec.Port, reason)
	}
	if spec.Kind == "mongo" && spec.Port == 27017 {
		return SvcSpec{}, fmt.Errorf("mongo gerenciado não usa 27017 (control-plane)")
	}
	if spec.Kind == "lb" && spec.Action == "apply" {
		if len(spec.Backends) == 0 {
			return SvcSpec{}, fmt.Errorf("lb exige pelo menos um backend")
		}
		for _, b := range spec.Backends {
			if err := validateBackend(b); err != nil {
				return SvcSpec{}, err
			}
		}
	}
	return spec, nil
}

func validSvcSlug(s string) bool {
	if len(s) < 2 || len(s) > 20 {
		return false
	}
	if s[0] == '-' || s[len(s)-1] == '-' {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			continue
		}
		return false
	}
	return true
}

func validateSvcBind(raw string) error {
	ip := net.ParseIP(raw)
	if ip == nil || ip.To4() == nil {
		return fmt.Errorf("bind deve ser um IPv4")
	}
	ip = ip.To4()
	if ip.Equal(net.IPv4zero) || ip.IsUnspecified() {
		return fmt.Errorf("bind 0.0.0.0 é proibido")
	}
	if ip.String() == loopbackIP {
		return nil
	}
	_, cidr, err := net.ParseCIDR(corpNetCIDR)
	if err != nil {
		return err
	}
	if !cidr.Contains(ip) {
		return fmt.Errorf("bind deve ser 127.0.0.1 ou %s", corpNetCIDR)
	}
	return nil
}

func validateBackend(raw string) error {
	host, port, err := net.SplitHostPort(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("backend inválido: %s", raw)
	}
	ip := net.ParseIP(host)
	if ip == nil || ip.To4() == nil {
		return fmt.Errorf("backend deve ser IPv4:porta")
	}
	if ip.Equal(net.IPv4zero) || ip.IsUnspecified() {
		return fmt.Errorf("backend 0.0.0.0 é proibido")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return fmt.Errorf("porta de backend inválida")
	}
	return nil
}

// ApplyService lê o JSON do stdin e aplica ou para a unit no host.
func ApplyService(r SvcRunner, stdin io.Reader) error {
	raw, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("lendo payload de serviço: %w", err)
	}
	spec, err := ParseSvcSpec(raw)
	if err != nil {
		return err
	}
	if spec.Action == "stop" {
		return stopService(r, spec)
	}
	return applyService(r, spec)
}

func svcDir(slug string) string {
	return filepath.Join(svcDataRoot, slug)
}

func unitName(slug string) string {
	return "xvpn-svc-" + slug + ".service"
}

func unitPath(slug string) string {
	return filepath.Join(svcUnitDir, unitName(slug))
}

func applyService(r SvcRunner, spec SvcSpec) error {
	dir := svcDir(spec.Slug)
	if err := r.MkdirAll(filepath.Join(dir, "data"), 0o750); err != nil {
		return fmt.Errorf("criando dir do serviço: %w", err)
	}
	bin, conf, unit, pkg, err := renderKind(spec, dir)
	if err != nil {
		return err
	}
	path, lookErr := r.LookPath(bin)
	if lookErr != nil {
		if err := r.InstallPackage(pkg); err != nil {
			return err
		}
		path, lookErr = r.LookPath(bin)
		if lookErr != nil {
			return fmt.Errorf("binário %s não encontrado após instalar %s", bin, pkg)
		}
	}
	if err := r.WriteFile(filepath.Join(dir, conf.name), conf.body, 0o640); err != nil {
		return fmt.Errorf("gravando config: %w", err)
	}
	unitBody := strings.ReplaceAll(unit, "{{BIN}}", path)
	if err := r.WriteFile(unitPath(spec.Slug), unitBody, 0o644); err != nil {
		return fmt.Errorf("gravando unit: %w", err)
	}
	if err := r.Systemctl("daemon-reload"); err != nil {
		return err
	}
	if err := r.Systemctl("enable", "--now", unitName(spec.Slug)); err != nil {
		return err
	}
	return nil
}

func stopService(r SvcRunner, spec SvcSpec) error {
	_ = r.Systemctl("disable", "--now", unitName(spec.Slug))
	_ = r.RemoveFile(unitPath(spec.Slug))
	_ = r.Systemctl("daemon-reload")
	return nil
}

type svcFile struct {
	name string
	body string
}

func renderKind(spec SvcSpec, dir string) (bin string, conf svcFile, unit, pkg string, err error) {
	switch spec.Kind {
	case "redis":
		return renderRedis(spec, dir)
	case "mongo":
		return renderMongo(spec, dir)
	case "rabbitmq":
		return renderRabbit(spec, dir)
	case "lb":
		return renderLB(spec, dir)
	default:
		return "", svcFile{}, "", "", fmt.Errorf("kind inválido")
	}
}

func renderRedis(spec SvcSpec, dir string) (string, svcFile, string, string, error) {
	pass := strings.ReplaceAll(spec.Password, "\n", "")
	body := fmt.Sprintf(`# Gerado por xvpn-user-provision svc-apply. Não edite.
bind %s
port %d
protected-mode yes
daemonize no
dir %s
pidfile %s
logfile ""
`, spec.Bind, spec.Port, filepath.Join(dir, "data"), filepath.Join(dir, "redis.pid"))
	if pass != "" {
		body += "requirepass " + pass + "\n"
	}
	unit := fmt.Sprintf(`[Unit]
Description=xvpn managed redis (%s)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart={{BIN}} %s
Restart=on-failure
RestartSec=3

[Install]
WantedBy=multi-user.target
`, spec.Slug, filepath.Join(dir, "redis.conf"))
	return "redis-server", svcFile{name: "redis.conf", body: body}, unit, "redis-server", nil
}

func renderMongo(spec SvcSpec, dir string) (string, svcFile, string, string, error) {
	if spec.Port == 27017 {
		return "", svcFile{}, "", "", fmt.Errorf("mongo gerenciado não usa 27017")
	}
	body := fmt.Sprintf(`# Gerado por xvpn-user-provision svc-apply. Não edite.
storage:
  dbPath: %s
net:
  bindIp: %s
  port: %d
processManagement:
  fork: false
`, filepath.Join(dir, "data"), spec.Bind, spec.Port)
	unit := fmt.Sprintf(`[Unit]
Description=xvpn managed mongo (%s)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart={{BIN}} --config %s
Restart=on-failure
RestartSec=3

[Install]
WantedBy=multi-user.target
`, spec.Slug, filepath.Join(dir, "mongod.conf"))
	return "mongod", svcFile{name: "mongod.conf", body: body}, unit, "mongodb", nil
}

func renderRabbit(spec SvcSpec, dir string) (string, svcFile, string, string, error) {
	pass := strings.ReplaceAll(spec.Password, "\n", "")
	if pass == "" {
		pass = "changeme"
	}
	body := fmt.Sprintf(`# Gerado por xvpn-user-provision svc-apply. Não edite.
listeners.tcp.local = %s:%d
loopback_users.guest = false
default_user = xvpn
default_pass = %s
`, spec.Bind, spec.Port, pass)
	unit := fmt.Sprintf(`[Unit]
Description=xvpn managed rabbitmq (%s)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
Environment=RABBITMQ_CONFIG_FILE=%s
Environment=RABBITMQ_NODENAME=xvpn-%s
Environment=RABBITMQ_MNESIA_BASE=%s
ExecStart={{BIN}} --foreground
Restart=on-failure
RestartSec=3

[Install]
WantedBy=multi-user.target
`, spec.Slug, strings.TrimSuffix(filepath.Join(dir, "rabbitmq.conf"), ".conf"), spec.Slug, filepath.Join(dir, "data"))
	return "rabbitmq-server", svcFile{name: "rabbitmq.conf", body: body}, unit, "rabbitmq-server", nil
}

func renderLB(spec SvcSpec, dir string) (string, svcFile, string, string, error) {
	var ups strings.Builder
	for _, b := range spec.Backends {
		ups.WriteString("        server " + b + ";\n")
	}
	body := fmt.Sprintf(`# Gerado por xvpn-user-provision svc-apply. Não edite.
# Intranet only — sem listen em 0.0.0.0/80/443.
pid %s;
error_log %s;
events { worker_connections 64; }
http {
    server {
        listen %s:%d;
        location / {
            proxy_pass http://xvpn_backends;
        }
    }
    upstream xvpn_backends {
%s    }
}
`, filepath.Join(dir, "nginx.pid"), filepath.Join(dir, "error.log"), spec.Bind, spec.Port, ups.String())
	unit := fmt.Sprintf(`[Unit]
Description=xvpn managed lb (%s)
After=network-online.target
Wants=network-online.target

[Service]
Type=forking
PIDFile=%s
ExecStart={{BIN}} -c %s
ExecReload={{BIN}} -s reload
Restart=on-failure

[Install]
WantedBy=multi-user.target
`, spec.Slug, filepath.Join(dir, "nginx.pid"), filepath.Join(dir, "nginx.conf"))
	return "nginx", svcFile{name: "nginx.conf", body: body}, unit, "nginx", nil
}
