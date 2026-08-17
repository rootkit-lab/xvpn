// Package backup roda restic/rclone para destinos off-site (Fase 44).
package backup

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var (
	ErrNoRestic   = errors.New("restic não encontrado no PATH")
	ErrNoPaths    = errors.New("nada selecionado para o backup")
	ErrBadKind    = errors.New("destino de backup inválido")
	ErrNoPassword = errors.New("senha do repositório restic ausente")
)

// Secret é o JSON gravado em BackupDestination.Secret (nunca no GET).
type Secret struct {
	Password    string `json:"password,omitempty"`
	SFTPUser    string `json:"sftp_user,omitempty"`
	SFTPHost    string `json:"sftp_host,omitempty"`
	SFTPPath    string `json:"sftp_path,omitempty"`
	SFTPKey     string `json:"sftp_key,omitempty"`
	B2AccountID string `json:"b2_account_id,omitempty"`
	B2Key       string `json:"b2_key,omitempty"`
	S3Endpoint  string `json:"s3_endpoint,omitempty"`
	S3Access    string `json:"s3_access,omitempty"`
	S3Secret    string `json:"s3_secret,omitempty"`
	S3Bucket    string `json:"s3_bucket,omitempty"`
	S3Region    string `json:"s3_region,omitempty"`
	WebDAVURL   string `json:"webdav_url,omitempty"`
	WebDAVUser  string `json:"webdav_user,omitempty"`
	WebDAVPass  string `json:"webdav_pass,omitempty"`
	RcloneConf  string `json:"rclone_conf,omitempty"`
}

func ParseSecret(raw string) Secret {
	var s Secret
	_ = json.Unmarshal([]byte(raw), &s)
	return s
}

func (s Secret) Encode() string {
	raw, _ := json.Marshal(s)
	return string(raw)
}

// Include escolhe o que entra no snapshot.
type Include struct {
	MongoURI       string
	MarketplaceDir string
	GitDir         string
	SocialDir      string
	Mongo          bool
	Marketplace    bool
	Git            bool
	Social         bool
}

// Dest descreve o repositório (sem persistir o secret no log).
type Dest struct {
	Kind     string
	Endpoint string
	Path     string
	Secret   Secret
}

// Result é o resumo de um job.
type Result struct {
	SnapshotID string
	Bytes      int64
	Log        string
}

// Runner executa restic. LookPath pode ser sobrescrito nos testes.
type Runner struct {
	LookPath func(string) (string, error)
	Run      func(ctx context.Context, name string, env []string, args ...string) ([]byte, error)
	Now      func() time.Time
}

func (r *Runner) look(bin string) (string, error) {
	if r != nil && r.LookPath != nil {
		return r.LookPath(bin)
	}
	return exec.LookPath(bin)
}

func (r *Runner) run(ctx context.Context, name string, env []string, args ...string) ([]byte, error) {
	if r != nil && r.Run != nil {
		return r.Run(ctx, name, env, args...)
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	return out, err
}

func (r *Runner) now() time.Time {
	if r != nil && r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

// Backup monta o snapshot. Dry-run não grava no repositório.
func (r *Runner) Backup(ctx context.Context, dest Dest, inc Include, staging string, dryRun bool) (Result, error) {
	paths, err := collectPaths(inc)
	if err != nil {
		return Result{}, err
	}
	if dest.Kind == "xdriver" {
		return r.copyXDriver(ctx, dest, paths, dryRun)
	}
	bin, err := r.look("restic")
	if err != nil {
		return Result{}, ErrNoRestic
	}
	repo, env, err := resticRepo(dest, staging)
	if err != nil {
		return Result{}, err
	}
	if dest.Secret.Password == "" {
		return Result{}, ErrNoPassword
	}
	env = append(env, "RESTIC_REPOSITORY="+repo, "RESTIC_PASSWORD="+dest.Secret.Password)
	baseEnv := append(os.Environ(), env...)

	if _, err := r.run(ctx, bin, baseEnv, "cat", "config"); err != nil {
		if out, initErr := r.run(ctx, bin, baseEnv, "init"); initErr != nil {
			return Result{}, fmt.Errorf("restic init: %s", strings.TrimSpace(string(out)))
		}
	}
	args := []string{"backup", "--json"}
	if dryRun {
		args = append(args, "--dry-run")
	}
	args = append(args, paths...)
	out, err := r.run(ctx, bin, baseEnv, args...)
	res := Result{Log: strings.TrimSpace(string(out))}
	if err != nil {
		return res, fmt.Errorf("restic backup: %s", res.Log)
	}
	res.SnapshotID, res.Bytes = parseResticSummary(out)
	if dryRun {
		res.SnapshotID = "dry-run"
	}
	return res, nil
}

func collectPaths(inc Include) ([]string, error) {
	var paths []string
	if inc.Marketplace && inc.MarketplaceDir != "" {
		if st, err := os.Stat(inc.MarketplaceDir); err == nil && st.IsDir() {
			paths = append(paths, inc.MarketplaceDir)
		}
	}
	if inc.Git && inc.GitDir != "" {
		if st, err := os.Stat(inc.GitDir); err == nil && st.IsDir() {
			paths = append(paths, inc.GitDir)
		}
	}
	if inc.Social && inc.SocialDir != "" {
		if st, err := os.Stat(inc.SocialDir); err == nil && st.IsDir() {
			paths = append(paths, inc.SocialDir)
		}
	}
	if inc.Mongo && inc.MongoURI != "" {
		if dump, err := dumpMongo(inc.MongoURI); err == nil && dump != "" {
			paths = append(paths, dump)
		}
	}
	if len(paths) == 0 {
		return nil, ErrNoPaths
	}
	return paths, nil
}

func dumpMongo(uri string) (string, error) {
	bin, err := exec.LookPath("mongodump")
	if err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp("", "xvpn-mongodump-*")
	if err != nil {
		return "", err
	}
	cfg := filepath.Join(dir, "mongodump.yaml")
	body := "uri: " + strconv.Quote(uri) + "\n"
	if err := os.WriteFile(cfg, []byte(body), 0o600); err != nil {
		_ = os.RemoveAll(dir)
		return "", err
	}
	outDir := filepath.Join(dir, "dump")
	cmd := exec.Command(bin, "--config="+cfg, "--out="+outDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(dir)
		return "", fmt.Errorf("mongodump: %s", strings.TrimSpace(string(out)))
	}
	_ = os.Remove(cfg)
	return outDir, nil
}

func resticRepo(dest Dest, staging string) (string, []string, error) {
	sec := dest.Secret
	switch dest.Kind {
	case "local":
		p := dest.Path
		if p == "" {
			p = filepath.Join(staging, "restic-local")
		}
		if err := os.MkdirAll(p, 0o750); err != nil {
			return "", nil, err
		}
		return p, nil, nil
	case "sftp":
		user := sec.SFTPUser
		host := sec.SFTPHost
		if host == "" {
			host = dest.Endpoint
		}
		path := sec.SFTPPath
		if path == "" {
			path = dest.Path
		}
		if user == "" || host == "" || path == "" {
			return "", nil, ErrBadKind
		}
		return fmt.Sprintf("sftp:%s@%s:%s", user, host, path), nil, nil
	case "b2":
		if sec.B2AccountID == "" || sec.B2Key == "" || dest.Path == "" {
			return "", nil, ErrBadKind
		}
		return "b2:" + dest.Path, []string{
			"B2_ACCOUNT_ID=" + sec.B2AccountID,
			"B2_ACCOUNT_KEY=" + sec.B2Key,
		}, nil
	case "s3":
		bucket := sec.S3Bucket
		if bucket == "" {
			bucket = dest.Path
		}
		if sec.S3Access == "" || sec.S3Secret == "" || bucket == "" {
			return "", nil, ErrBadKind
		}
		repo := "s3:" + bucket
		if dest.Path != "" && dest.Path != bucket {
			repo = "s3:" + bucket + "/" + strings.TrimPrefix(dest.Path, "/")
		}
		env := []string{
			"AWS_ACCESS_KEY_ID=" + sec.S3Access,
			"AWS_SECRET_ACCESS_KEY=" + sec.S3Secret,
		}
		if sec.S3Endpoint != "" {
			env = append(env, "AWS_S3_ENDPOINT="+sec.S3Endpoint)
		}
		if sec.S3Region != "" {
			env = append(env, "AWS_DEFAULT_REGION="+sec.S3Region)
		}
		return repo, env, nil
	case "webdav", "drive":
		conf, err := rcloneConfig(dest.Kind, sec)
		if err != nil {
			return "", nil, err
		}
		path := dest.Path
		if path == "" {
			path = "xvpn"
		}
		if !safeRclonePath(path) {
			return "", nil, ErrBadKind
		}
		cfg := filepath.Join(staging, "rclone.conf")
		if err := os.MkdirAll(staging, 0o750); err != nil {
			return "", nil, err
		}
		if err := os.WriteFile(cfg, []byte(conf), 0o600); err != nil {
			return "", nil, err
		}
		return "rclone:" + dest.Kind + ":" + path, []string{"RCLONE_CONFIG=" + cfg}, nil
	default:
		return "", nil, ErrBadKind
	}
}

func (r *Runner) copyXDriver(ctx context.Context, dest Dest, paths []string, dryRun bool) (Result, error) {
	if dest.Path == "" {
		return Result{}, ErrBadKind
	}
	if dryRun {
		return Result{SnapshotID: "dry-run", Log: "xdriver rsync --dry-run → " + dest.Path}, nil
	}
	bin, err := r.look("rsync")
	if err != nil {
		return Result{}, fmt.Errorf("rsync: %w", err)
	}
	if err := os.MkdirAll(dest.Path, 0o750); err != nil {
		return Result{}, err
	}
	var log bytes.Buffer
	for _, p := range paths {
		out, err := r.run(ctx, bin, os.Environ(), "-a", "--delete", p+"/", filepath.Join(dest.Path, filepath.Base(p))+"/")
		log.Write(out)
		if err != nil {
			return Result{Log: log.String()}, fmt.Errorf("rsync: %s", strings.TrimSpace(string(out)))
		}
	}
	return Result{SnapshotID: r.now().UTC().Format("20060102T150405Z"), Log: log.String()}, nil
}

func safeINIValue(s string) bool {
	return !strings.ContainsAny(s, "\n\r=")
}

func safeRclonePath(p string) bool {
	p = strings.TrimSpace(p)
	if p == "" || strings.ContainsAny(p, "\n\r:") || strings.Contains(p, "..") {
		return false
	}
	return true
}

func rcloneConfig(kind string, sec Secret) (string, error) {
	switch kind {
	case "webdav":
		if !safeINIValue(sec.WebDAVURL) || !safeINIValue(sec.WebDAVUser) || !safeINIValue(sec.WebDAVPass) {
			return "", ErrBadKind
		}
		if sec.WebDAVURL == "" {
			return "", ErrBadKind
		}
		return fmt.Sprintf("[webdav]\ntype = webdav\nvendor = other\nurl = %s\nuser = %s\npass = %s\n",
			sec.WebDAVURL, sec.WebDAVUser, sec.WebDAVPass), nil
	case "drive":
		tok := strings.TrimSpace(sec.RcloneConf)
		if tok == "" || !strings.HasPrefix(tok, "{") || strings.ContainsAny(tok, "\n\r=") {
			return "", ErrBadKind
		}
		return fmt.Sprintf("[drive]\ntype = drive\ntoken = %s\n", tok), nil
	default:
		return "", ErrBadKind
	}
}

func parseResticSummary(out []byte) (string, int64) {
	var snap string
	var total int64
	for _, line := range bytes.Split(out, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var ev struct {
			MessageType string `json:"message_type"`
			SnapshotID  string `json:"snapshot_id"`
			TotalBytes  int64  `json:"total_bytes_processed"`
			DataAdded   int64  `json:"data_added"`
		}
		if json.Unmarshal(line, &ev) != nil {
			continue
		}
		if ev.SnapshotID != "" {
			snap = ev.SnapshotID
		}
		if ev.TotalBytes > 0 {
			total = ev.TotalBytes
		}
		if ev.DataAdded > 0 && total == 0 {
			total = ev.DataAdded
		}
	}
	return snap, total
}
