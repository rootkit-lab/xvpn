package monitor

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rootkit-lab/xvpn/server/internal/store"
	"github.com/rootkit-lab/xvpn/server/internal/wireguard"
	"gorm.io/gorm"
)

const (
	StatusOK       = "ok"
	StatusWarn     = "warn"
	StatusCritical = "critical"
	StatusSkipped  = "skipped"
)

// Result é um probe individual antes de persistir.
type Result struct {
	Slug    string
	Name    string
	Status  string
	Summary string
	Detail  string
}

// Runner executa probes read-only a partir do control-plane.
type Runner struct {
	DB         *gorm.DB
	WG         wireguard.PeerManager
	GitDir     string
	HTTPClient *http.Client
}

func NewRunner(db *gorm.DB, wg wireguard.PeerManager, gitDir string) *Runner {
	return &Runner{
		DB:     db,
		WG:     wg,
		GitDir: strings.TrimSpace(gitDir),
		HTTPClient: &http.Client{
			Timeout: 8 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // probe local nginx/wg0
			},
		},
	}
}

// RunAll executa todos os probes v0 e persiste em monitor_checks.
func (r *Runner) RunAll(ctx context.Context) ([]Result, error) {
	var out []Result
	out = append(out, r.checkCorpHTTP(ctx)...)
	out = append(out, r.checkWireGuard()...)
	out = append(out, r.checkMongoData())
	out = append(out, r.checkRegistryData())
	out = append(out, r.checkGitMount())
	if err := r.persist(out); err != nil {
		return out, err
	}
	return out, nil
}

func (r *Runner) persist(results []Result) error {
	now := time.Now()
	for _, res := range results {
		row := store.MonitorCheck{
			Slug: res.Slug, Name: res.Name, Status: res.Status,
			Summary: res.Summary, Detail: res.Detail, CheckedAt: now,
		}
		var existing store.MonitorCheck
		err := r.DB.Where("slug = ?", res.Slug).First(&existing).Error
		if err == nil {
			row.ID = existing.ID
			if err := r.DB.Save(&row).Error; err != nil {
				return err
			}
			continue
		}
		if err != gorm.ErrRecordNotFound {
			return err
		}
		if err := r.DB.Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *Runner) checkCorpHTTP(ctx context.Context) []Result {
	var hosts []store.DNSRecord
	if err := r.DB.Where("enabled = ?", true).Order("hostname ASC").Find(&hosts).Error; err != nil || len(hosts) == 0 {
		hosts = store.DefaultIntranetHosts
	}
	// data.corp pode existir só no arquivo hosts do dnsmasq.
	seen := map[string]struct{}{}
	for _, h := range hosts {
		seen[h.Hostname] = struct{}{}
	}
	if _, ok := seen["data.corp.ihuull.com"]; !ok {
		hosts = append(hosts, store.DNSRecord{Hostname: "data.corp.ihuull.com", IPv4: "10.66.66.2"})
	}

	out := make([]Result, 0, len(hosts))
	for _, rec := range hosts {
		slug := "corp-http-" + strings.ReplaceAll(rec.Hostname, ".", "-")
		name := "HTTP " + rec.Hostname
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://10.66.66.1/", nil)
		if err != nil {
			out = append(out, Result{Slug: slug, Name: name, Status: StatusCritical, Summary: err.Error()})
			continue
		}
		req.Host = rec.Hostname
		res, err := r.HTTPClient.Do(req)
		if err != nil {
			out = append(out, Result{Slug: slug, Name: name, Status: StatusCritical, Summary: err.Error()})
			continue
		}
		_, _ = io.Copy(io.Discard, res.Body)
		_ = res.Body.Close()
		status := StatusOK
		summary := fmt.Sprintf("HTTP %d", res.StatusCode)
		if res.StatusCode >= 500 {
			status = StatusCritical
		} else if res.StatusCode >= 400 {
			status = StatusWarn
		}
		out = append(out, Result{Slug: slug, Name: name, Status: status, Summary: summary})
	}
	return out
}

func (r *Runner) checkWireGuard() []Result {
	if r.WG == nil {
		return []Result{{Slug: "wg-peers", Name: "WireGuard peers", Status: StatusSkipped, Summary: "sem interface WG"}}
	}
	peers, err := r.WG.ListPeers()
	if err != nil {
		return []Result{{Slug: "wg-peers", Name: "WireGuard peers", Status: StatusCritical, Summary: err.Error()}}
	}
	if len(peers) == 0 {
		return []Result{{Slug: "wg-peers", Name: "WireGuard peers", Status: StatusWarn, Summary: "nenhum peer"}}
	}
	out := make([]Result, 0, len(peers))
	worst := StatusOK
	for _, p := range peers {
		slug := "wg-peer-" + p.PublicKey[:8]
		ip := ""
		if len(p.AllowedIPs) > 0 {
			ip = p.AllowedIPs[0]
		}
		name := "WG " + ip
		status := StatusOK
		summary := "handshake recente"
		if p.LastHandshake == nil {
			status = StatusCritical
			summary = "sem handshake"
		} else if time.Since(*p.LastHandshake) > 3*time.Minute {
			status = StatusWarn
			summary = fmt.Sprintf("handshake há %s", time.Since(*p.LastHandshake).Round(time.Second))
		}
		if status == StatusCritical {
			worst = StatusCritical
		} else if status == StatusWarn && worst != StatusCritical {
			worst = StatusWarn
		}
		out = append(out, Result{Slug: slug, Name: name, Status: status, Summary: summary})
	}
	out = append([]Result{{Slug: "wg-peers", Name: "WireGuard peers", Status: worst, Summary: fmt.Sprintf("%d peer(s)", len(peers))}}, out...)
	return out
}

func (r *Runner) checkMongoData() Result {
	const slug = "mongo-data"
	conn, err := net.DialTimeout("tcp", "10.66.66.2:27017", 2*time.Second)
	if err != nil {
		return Result{Slug: slug, Name: "Mongo no data", Status: StatusSkipped, Summary: "não escutando (SQLite em prod)"}
	}
	_ = conn.Close()
	return Result{Slug: slug, Name: "Mongo no data", Status: StatusOK, Summary: "porta 27017 aberta na infra"}
}

func (r *Runner) checkRegistryData() Result {
	const slug = "registry-data"
	req, err := http.NewRequest(http.MethodGet, "http://10.66.66.2:5000/v2/", nil)
	if err != nil {
		return Result{Slug: slug, Name: "Registry no data", Status: StatusCritical, Summary: err.Error()}
	}
	res, err := r.HTTPClient.Do(req)
	if err != nil {
		return Result{Slug: slug, Name: "Registry no data", Status: StatusCritical, Summary: err.Error()}
	}
	_, _ = io.Copy(io.Discard, res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return Result{Slug: slug, Name: "Registry no data", Status: StatusCritical, Summary: fmt.Sprintf("HTTP %d", res.StatusCode)}
	}
	return Result{Slug: slug, Name: "Registry no data", Status: StatusOK, Summary: "registry:2 responde"}
}

func (r *Runner) checkGitMount() Result {
	const slug = "git-bare"
	dir := r.GitDir
	if dir == "" {
		dir = "/opt/xvpn/data/git"
	}
	info, err := os.Stat(dir)
	if err != nil {
		return Result{Slug: slug, Name: "Git bare", Status: StatusCritical, Summary: err.Error()}
	}
	if !info.IsDir() {
		return Result{Slug: slug, Name: "Git bare", Status: StatusCritical, Summary: "não é diretório"}
	}
	// NFS mount em Linux expõe st_dev diferente; checamos presença de org.
	if _, err := os.Stat(filepath.Join(dir, "xcorp")); err != nil {
		return Result{Slug: slug, Name: "Git bare", Status: StatusWarn, Summary: "sem xcorp/"}
	}
	return Result{Slug: slug, Name: "Git bare", Status: StatusOK, Summary: "xcorp/ acessível"}
}
