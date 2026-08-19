package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/backup"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

type fakeBackupEngine struct {
	lastDry bool
	lastInc backup.Include
	err     error
}

func (f *fakeBackupEngine) Backup(_ context.Context, _ backup.Dest, inc backup.Include, _ string, dryRun bool) (backup.Result, error) {
	f.lastDry = dryRun
	f.lastInc = inc
	if f.err != nil {
		return backup.Result{}, f.err
	}
	return backup.Result{SnapshotID: "snap1", Bytes: 42}, nil
}

func TestBackupSettingsAndDestination(t *testing.T) {
	app, router, adminTok := setupGitApp(t)
	eng := &fakeBackupEngine{}
	app.Backup = eng
	app.Config.BackupDir = t.TempDir()
	app.Config.MarketplaceDataDir = t.TempDir()

	rec := doJSON(t, router, http.MethodGet, "/api/backups/settings", nil, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("settings: %d %s", rec.Code, rec.Body.String())
	}

	days := 14
	rec = doJSON(t, router, http.MethodPatch, "/api/backups/settings", patchBackupSettingsRequest{RetentionDays: &days}, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch settings: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodPost, "/api/backups/destinations", upsertBackupDestRequest{
		Name: "nas", Kind: store.BackupKindSFTP, Endpoint: "nas.example", Path: "/backups/xvpn",
		Secret: &backup.Secret{Password: "repo-pass", SFTPUser: "bk", SFTPHost: "nas.example", SFTPPath: "/backups/xvpn"},
	}, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create dest: %d %s", rec.Code, rec.Body.String())
	}
	var dest backupDestJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &dest); err != nil {
		t.Fatal(err)
	}
	if !dest.HasSecret || dest.Kind != "sftp" || !dest.Offsite {
		t.Fatalf("dest: %+v", dest)
	}
	if strings.Contains(rec.Body.String(), "repo-pass") {
		t.Fatalf("secret leaked: %s", rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodPost, "/api/backups/destinations/"+itoa(dest.ID)+"/run", runBackupRequest{DryRun: true}, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("run: %d %s", rec.Code, rec.Body.String())
	}
	if !eng.lastDry {
		t.Fatal("expected dry-run")
	}
	var job backupJobJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &job); err != nil {
		t.Fatal(err)
	}
	if job.Status != "ok" || job.SnapshotID != "snap1" || !job.DryRun {
		t.Fatalf("job: %+v", job)
	}

	rec = doJSON(t, router, http.MethodGet, "/api/backups/jobs", nil, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("jobs: %d", rec.Code)
	}

	rec = doJSON(t, router, http.MethodPost, "/api/backups/destinations", upsertBackupDestRequest{
		Name: "local", Kind: store.BackupKindLocal,
	}, adminTok)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("local dest should be rejected: %d", rec.Code)
	}

	rec = doJSON(t, router, http.MethodPost, "/api/backups/destinations", upsertBackupDestRequest{
		Name: "drive-extra", Kind: store.BackupKindXDriver, Path: "extra",
	}, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("xdriver dest: %d %s", rec.Code, rec.Body.String())
	}
	var xd backupDestJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &xd); err != nil {
		t.Fatal(err)
	}
	if xd.Offsite {
		t.Fatal("xdriver não é off-site")
	}
	rec = doJSON(t, router, http.MethodPost, "/api/backups/destinations/"+itoa(xd.ID)+"/run", runBackupRequest{}, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("xdriver run: %d %s", rec.Code, rec.Body.String())
	}
	if eng.lastInc.Mongo || eng.lastInc.MongoURI != "" {
		t.Fatalf("xdriver não pode incluir mongo: %+v", eng.lastInc)
	}
}
