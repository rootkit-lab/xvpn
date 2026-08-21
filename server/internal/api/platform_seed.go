package api

import (
	"errors"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

const dataNodeNotes = "Nó de dados da malha: Mongo, git bare e containers. " +
	"Alivia o control-plane 206.189.224.72. Chave SSH fica no laptop — control-plane não faz SSH. " +
	"Enroll WireGuard gera a chave privada neste host."

// SeedPlatformRepo garante xcorp/xvpn (monorepo da plataforma: server/, shared/,
// painel xadmin, API codespace). Idempotente. Não é inventário de VPS.
func (a *App) SeedPlatformRepo() error {
	if a == nil || a.Store == nil {
		return nil
	}
	if err := store.SeedXcorp(a.Store.DB); err != nil {
		return err
	}
	owner, ok := a.firstProjectOwner()
	if !ok {
		return nil
	}
	org, ok := a.defaultOrganization()
	if !ok {
		return nil
	}
	var existing store.Project
	err := a.Store.DB.Where("organization_id = ? AND slug = ?", org.ID, store.PlatformRepoSlug).First(&existing).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	_, err = a.createProject(owner.ID, org, store.PlatformRepoSlug, "XVPN Platform",
		"Monorepo da plataforma ihuull (server, shared, xadmin, codespace API). Produtos xadmin/xgit/xcodespaces são superfícies deste repo — não VPS.",
		store.AppVisibilityRestricted, store.AppNetworkVPN, nil, false, nil)
	if err != nil {
		return err
	}
	slog.Info("seed XGIT platform repo", "repo", store.DefaultOrgSlug+"/"+store.PlatformRepoSlug)
	return nil
}

// SeedDataNode registra o VPS de dados (66.29.147.100) como MeshServer manual
// pending-enroll. Não cria BitLaunch, não grava chave SSH, não é ProjectHost.
func (a *App) SeedDataNode() error {
	if a == nil || a.Store == nil {
		return nil
	}
	owner, ok := a.firstProjectOwner()
	if !ok {
		return nil
	}
	var existing store.MeshServer
	err := a.Store.DB.Where("ipv4 = ? OR hostname = ?", store.DataNodeIPv4, store.DataHostname).First(&existing).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	token, err := generateInviteToken()
	if err != nil {
		return err
	}
	exp := time.Now().Add(7 * 24 * time.Hour)
	row := store.MeshServer{
		BitLaunchID:     store.ManualBitLaunchID(store.DataHostname),
		Name:            "data",
		Hostname:        store.DataHostname,
		Role:            store.ServerRoleMesh,
		IPv4:            store.DataNodeIPv4,
		Status:          "pending-enroll",
		Labels:          []string{"data", "mongo", "git", "containers"},
		Notes:           dataNodeNotes,
		CreatedByUserID: owner.ID,
		EnrollToken:     token,
		EnrollExpiresAt: &exp,
	}
	if err := a.Store.DB.Create(&row).Error; err != nil {
		return err
	}
	slog.Info("seed Compute data node", "hostname", store.DataHostname, "ipv4", store.DataNodeIPv4)
	return nil
}
