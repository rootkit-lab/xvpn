package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/rootkit-lab/xvpn/server/internal/api"
	"github.com/rootkit-lab/xvpn/server/internal/auth"
	"github.com/rootkit-lab/xvpn/server/internal/config"
	"github.com/rootkit-lab/xvpn/server/internal/logging"
	"github.com/rootkit-lab/xvpn/server/internal/marketplace"
	"github.com/rootkit-lab/xvpn/server/internal/store"
	"github.com/rootkit-lab/xvpn/server/internal/userprovision"
	"github.com/rootkit-lab/xvpn/server/internal/wireguard"
)

func main() {
	logging.Setup()
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	// Produção: GIN_MODE=release no EnvironmentFile do systemd (ver
	// deploy/xvpn-server.env.example). Sem isso o Gin fica em debug.
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	db, err := store.Open(cfg.DBPath)
	if err != nil {
		return err
	}

	if err := bootstrapAdmin(db, cfg); err != nil {
		return err
	}

	privateKey, err := wireguard.ReadPrivateKey(cfg.WireGuardPrivateKeyPath)
	if err != nil {
		return err
	}

	wgManager, err := wireguard.NewManager(cfg.WireGuardInterface)
	if err != nil {
		return err
	}
	defer wgManager.Close()

	if err := wgManager.EnsureInterface(privateKey, cfg.WireGuardListenPort, cfg.WireGuardAddress); err != nil {
		return err
	}

	if err := reconcilePeersFromDB(db, wgManager); err != nil {
		return err
	}

	marketplaceStore, err := marketplace.NewStore(cfg.MarketplaceDataDir)
	if err != nil {
		return err
	}

	// UserProvisioner (Fase 13, PLAN.md §6.9): cliente do binário
	// privilegiado xvpn-user-provision. Se o binário não existe no
	// caminho configurado (XVPN_USER_PROVISION_BIN), o client ainda é
	// criado — a ausência só é detectada na primeira chamada real
	// (ErrBinaryMissing), e o handler devolve 503. Em produção o
	// binário é instalado pelo deploy da Fase 13 (ver ROADMAP.md).
	userProvisioner := userprovision.New(cfg.UserProvisionBinaryPath)

	tokens := auth.NewTokenManager(cfg.JWTSecret, time.Duration(cfg.JWTTokenTTLMinutes)*time.Minute)

	app := &api.App{
		Store:           db,
		WG:              wgManager,
		Tokens:          tokens,
		Config:          cfg,
		Marketplace:     marketplaceStore,
		UserProvisioner: userProvisioner,
		ServerPublicKey: privateKey.PublicKey().String(),
	}
	router := api.NewRouter(app)

	// Reconcile de contas Unix (Fase 13, PLAN.md §6.9): converte o
	// estado do DB para o sistema. Best-effort — não bloqueia o boot;
	// se falhar, o admin vê no log e pode re-rodar (ou corrigir à mão).
	// Ver api.ReconcileUnixAccounts pra detalhes e limitações.
	if err := app.ReconcileUnixAccounts(context.Background()); err != nil {
		slog.Error("reconcile de contas Unix falhou (servidor continua subindo; ver log)",
			"err", err.Error())
	} else {
		slog.Info("unix accounts reconciled from database")
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("listening",
			"addr", cfg.HTTPAddr,
			"wireguard_interface", cfg.WireGuardInterface,
		)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case sig := <-sigCh:
		slog.Info("shutdown signal", "signal", sig.String())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}

func reconcilePeersFromDB(db *store.Store, wg *wireguard.Manager) error {
	var devices []store.Device
	if err := db.DB.Find(&devices).Error; err != nil {
		return err
	}
	specs := make([]wireguard.PeerSpec, 0, len(devices))
	for _, d := range devices {
		specs = append(specs, wireguard.PeerSpec{PublicKey: d.PublicKey, AllowedIP: d.AllowedIP})
	}
	if err := wg.ReconcilePeers(specs); err != nil {
		return err
	}
	slog.Info("peers reconciled from database", "count", len(specs))
	return nil
}

func bootstrapAdmin(db *store.Store, cfg *config.Config) error {
	count, err := db.CountUsers()
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	username := cfg.AdminBootstrapUsername
	if username == "" {
		username = "admin"
	}
	password := cfg.AdminBootstrapPassword
	generated := false
	if password == "" {
		password, err = auth.GenerateRandomPassword()
		if err != nil {
			return err
		}
		generated = true
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}

	// O bootstrap sempre vira super_admin (Fase 10, PLAN.md §6.7): é o
	// único usuário criado com a tabela vazia, então não há papel "mais
	// antigo" para herdar — precisa nascer com o papel mais alto para
	// poder promover/gerenciar qualquer conta futura.
	if err := db.DB.Create(&store.User{Username: username, PasswordHash: hash, Role: store.RoleSuperAdmin}).Error; err != nil {
		return err
	}

	if generated {
		// Senha gerada: única exceção proposital a "não logar segredos" —
		// é o bootstrap one-shot do primeiro admin (ver PLAN.md). slog
		// estruturado ainda ajuda o journalctl -u a achar a linha.
		slog.Warn("bootstrap admin created with generated password",
			"username", username,
			"password", password,
			"hint", "copie agora — não será exibida de novo; troque pelo painel",
		)
	} else {
		slog.Info("bootstrap admin created from env", "username", username)
	}

	return nil
}
