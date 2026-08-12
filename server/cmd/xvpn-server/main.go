// Comando xvpn-server: control-plane do XVPN — API HTTP, integração
// WireGuard via wgctrl e (a partir da Fase 3) o painel web embutido.
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rootkit-lab/xvpn/server/internal/api"
	"github.com/rootkit-lab/xvpn/server/internal/auth"
	"github.com/rootkit-lab/xvpn/server/internal/config"
	"github.com/rootkit-lab/xvpn/server/internal/store"
	"github.com/rootkit-lab/xvpn/server/internal/wireguard"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("xvpn-server: %v", err)
	}
}

func run() error {
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

	tokens := auth.NewTokenManager(cfg.JWTSecret, time.Duration(cfg.JWTTokenTTLMinutes)*time.Minute)

	app := &api.App{
		Store:           db,
		WG:              wgManager,
		Tokens:          tokens,
		Config:          cfg,
		ServerPublicKey: privateKey.PublicKey().String(),
	}
	router := api.NewRouter(app)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("xvpn-server: escutando em %s (interface WireGuard: %s)", cfg.HTTPAddr, cfg.WireGuardInterface)
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
		log.Printf("xvpn-server: sinal %s recebido, desligando...", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}

// reconcilePeersFromDB sincroniza o conjunto de peers da interface com o
// que está persistido no banco, garantindo consistência mesmo após um
// restart do serviço (ver internal/wireguard.ReconcilePeers).
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
	log.Printf("xvpn-server: %d peer(s) sincronizado(s) a partir do banco de dados", len(specs))
	return nil
}

// bootstrapAdmin cria o primeiro usuário admin se a tabela de usuários
// estiver vazia. Usa XVPN_ADMIN_USERNAME/XVPN_ADMIN_PASSWORD se definidos;
// caso contrário, gera uma senha aleatória e a loga uma única vez (o
// operador deve copiá-la e trocá-la depois via painel).
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
		password, err = generateRandomPassword()
		if err != nil {
			return err
		}
		generated = true
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}

	if err := db.DB.Create(&store.User{Username: username, PasswordHash: hash}).Error; err != nil {
		return err
	}

	if generated {
		log.Printf("=== primeiro usuário admin criado ===")
		log.Printf("usuário: %s", username)
		log.Printf("senha (gerada, copie agora — não será exibida de novo): %s", password)
		log.Printf("troque essa senha assim que possível pelo painel/API")
	} else {
		log.Printf("xvpn-server: usuário admin %q criado a partir de XVPN_ADMIN_USERNAME/PASSWORD", username)
	}

	return nil
}

func generateRandomPassword() (string, error) {
	buf := make([]byte, 18)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
