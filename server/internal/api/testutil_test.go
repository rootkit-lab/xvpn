package api

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/rootkit-lab/xvpn/server/internal/auth"
	"github.com/rootkit-lab/xvpn/server/internal/config"
	"github.com/rootkit-lab/xvpn/server/internal/marketplace"
	"github.com/rootkit-lab/xvpn/server/internal/store"
	"github.com/rootkit-lab/xvpn/server/internal/wireguard"
)

// fakePeerManager é um wireguard.PeerManager em memória, usado para testar
// os handlers HTTP sem precisar de CAP_NET_ADMIN/kernel real.
type fakePeerManager struct {
	mu    sync.Mutex
	peers map[string]wireguard.PeerStatus

	// failNextAdd/failNextRemove permitem simular falha do kernel/wgctrl
	// num teste específico.
	failNextAdd    bool
	failNextRemove bool

	// listPeersCalls conta chamadas reais a ListPeers — usado para
	// verificar o cache de GET /api/status (ver status_handler_test.go).
	listPeersCalls int
}

func newFakePeerManager() *fakePeerManager {
	return &fakePeerManager{peers: make(map[string]wireguard.PeerStatus)}
}

func (f *fakePeerManager) AddPeer(spec wireguard.PeerSpec) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failNextAdd {
		f.failNextAdd = false
		return assertErr("falha simulada em AddPeer")
	}
	f.peers[spec.PublicKey] = wireguard.PeerStatus{
		PublicKey:  spec.PublicKey,
		AllowedIPs: []string{spec.AllowedIP},
	}
	return nil
}

func (f *fakePeerManager) RemovePeer(publicKey string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failNextRemove {
		f.failNextRemove = false
		return assertErr("falha simulada em RemovePeer")
	}
	delete(f.peers, publicKey)
	return nil
}

func (f *fakePeerManager) setHandshake(publicKey string, t time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := f.peers[publicKey]
	p.PublicKey = publicKey
	p.LastHandshake = &t
	f.peers[publicKey] = p
}

func (f *fakePeerManager) ListPeers() ([]wireguard.PeerStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listPeersCalls++
	out := make([]wireguard.PeerStatus, 0, len(f.peers))
	for _, p := range f.peers {
		out = append(out, p)
	}
	return out, nil
}

type simpleError string

func (e simpleError) Error() string { return string(e) }

func assertErr(msg string) error { return simpleError(msg) }

func newTestApp(t *testing.T) (*App, *fakePeerManager) {
	t.Helper()

	// DSN em memória com nome único por teste: "cache=shared" faz várias
	// conexões da mesma pool enxergarem o mesmo banco, mas um nome único
	// evita que testes diferentes rodando no mesmo processo compartilhem
	// dados entre si (username/token únicos colidindo entre testes).
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", sanitizeDSNName(t.Name()))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("erro abrindo sqlite em memória: %v", err)
	}
	if err := db.AutoMigrate(
		&store.User{}, &store.Device{}, &store.InviteToken{}, &store.AuditLog{}, &store.WaitlistEntry{},
		&store.App{}, &store.AppVersion{}, &store.AppAsset{}, &store.AppAccess{},
		&store.PanelSettings{}, &store.ForgeSettings{},
		&store.DNSSettings{}, &store.DNSRecord{},
		&store.CloudflareAccount{}, &store.PublicZone{}, &store.PublicRecord{},
		&store.SocialProfile{}, &store.Follow{}, &store.SocialGroup{}, &store.SocialGroupMember{},
		&store.DirectThread{}, &store.DirectThreadMember{}, &store.Message{}, &store.MessageReceipt{},
		&store.SocialAttachment{}, &store.Story{}, &store.StoryView{},
		&store.SocialPost{}, &store.SocialPostStar{}, &store.SocialPostComment{},
		&store.Project{}, &store.ProjectMember{}, &store.ProjectStar{}, &store.ProtectedBranch{}, &store.MergeRequest{}, &store.MergeRequestReview{}, &store.Issue{}, &store.Milestone{}, &store.WorkProject{}, &store.WorkItem{}, &store.CiJob{},
		&store.MeshServer{}, &store.ServerGroup{}, &store.ServerAccess{}, &store.BitLaunchAccount{},
		&store.ServiceInstance{},
	); err != nil {
		t.Fatalf("erro migrando schema: %v", err)
	}
	if err := store.SeedIntranetDNS(db); err != nil {
		t.Fatalf("erro semeando DNS: %v", err)
	}

	fakeWG := newFakePeerManager()
	cfg := &config.Config{
		WireGuardAllowedSubnet: "10.66.66.0/24",
		WireGuardEndpoint:      "203.0.113.10:51820",
		InviteTokenTTLMinutes:  15,
	}

	marketplaceStore, err := marketplace.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("erro criando marketplace store de teste: %v", err)
	}

	socialStore, err := marketplace.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("erro criando social media store de teste: %v", err)
	}

	app := &App{
		Store:           &store.Store{DB: db},
		WG:              fakeWG,
		Tokens:          auth.NewTokenManager("segredo-de-teste-com-pelo-menos-32-bytes", time.Hour),
		Config:          cfg,
		Marketplace:     marketplaceStore,
		SocialMedia:     socialStore,
		ServerPublicKey: "test-server-public-key=",
	}
	return app, fakeWG
}

func sanitizeDSNName(name string) string {
	replacer := strings.NewReplacer("/", "_", " ", "_")
	return replacer.Replace(name)
}
