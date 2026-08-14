package api

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func TestReconcileUnixAccounts_NilProvisionerNoop(t *testing.T) {
	app, _ := newTestApp(t) // UserProvisioner nil
	if err := app.ReconcileUnixAccounts(context.Background()); err != nil {
		t.Fatalf("esperava nil com provisioner nil, obtido %v", err)
	}
}

func TestReconcileUnixAccounts_AppliesEnabledState(t *testing.T) {
	fp := &fakeUserProvisioner{}
	app, _ := withProvisioner(t, fp)
	// Dois usuários: um com SFTP on, outro com Samba on, outro com nada.
	alice := createTestUserWithRole(t, app, "alice", "senha-alice-123", store.RoleMember)
	app.Store.DB.Model(&alice).Updates(map[string]any{
		"sftp_enabled":   true,
		"ssh_public_key": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5 alice",
	})
	bob := createTestUserWithRole(t, app, "bob", "senha-bob-123", store.RoleMember)
	app.Store.DB.Model(&bob).Updates(map[string]any{"samba_enabled": true})
	_ = createTestUserWithRole(t, app, "carol", "senha-carol-123", store.RoleMember) // sem acesso

	if err := app.ReconcileUnixAccounts(context.Background()); err != nil {
		t.Fatalf("reconcile falhou: %v", err)
	}
	// Espera: EnableSFTP(alice) e EnableSamba(bob). Carol não é tocada.
	wantCalls := map[string]bool{
		"EnableSFTP(alice,ssh-ed25519 AAAAC3NzaC1lZDI1NTE5 alice)": true,
		"EnableSamba(bob)": true,
	}
	if len(fp.calls) != len(wantCalls) {
		t.Fatalf("esperava %d calls, obtido %d: %v", len(wantCalls), len(fp.calls), fp.calls)
	}
	for _, c := range fp.calls {
		if !wantCalls[c] {
			t.Errorf("call inesperada: %q", c)
		}
	}
}

func TestReconcileUnixAccounts_SkipsDisabledUsers(t *testing.T) {
	fp := &fakeUserProvisioner{}
	app, _ := withProvisioner(t, fp)
	_ = createTestUserWithRole(t, app, "alice", "senha-alice-123", store.RoleMember) // sem acesso

	if err := app.ReconcileUnixAccounts(context.Background()); err != nil {
		t.Fatalf("reconcile falhou: %v", err)
	}
	if len(fp.calls) != 0 {
		t.Errorf("esperava 0 calls (usuário sem acesso), obtido %v", fp.calls)
	}
}

func TestReconcileUnixAccounts_PartialFailureAggregates(t *testing.T) {
	// alice tem SFTP on mas o provisionador falha em EnableSFTP.
	// bob tem Samba on e sucede. Reconcile devolve erro agregando
	// a falha da alice, mas ainda tenta o bob.
	fp := &fakeUserProvisioner{failOn: "EnableSFTP", err: errors.New("sshd -t falhou")}
	app, _ := withProvisioner(t, fp)
	alice := createTestUserWithRole(t, app, "alice", "senha-alice-123", store.RoleMember)
	app.Store.DB.Model(&alice).Updates(map[string]any{
		"sftp_enabled":   true,
		"ssh_public_key": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5 alice",
	})
	bob := createTestUserWithRole(t, app, "bob", "senha-bob-123", store.RoleMember)
	app.Store.DB.Model(&bob).Updates(map[string]any{"samba_enabled": true})

	err := app.ReconcileUnixAccounts(context.Background())
	if err == nil {
		t.Fatal("esperava erro agregado, obtido nil")
	}
	if !strings.Contains(err.Error(), "alice") {
		t.Errorf("erro deveria mencionar alice: %q", err.Error())
	}
	// bob ainda foi tentado (EnableSamba sucedeu).
	foundBob := false
	for _, c := range fp.calls {
		if c == "EnableSamba(bob)" {
			foundBob = true
		}
	}
	if !foundBob {
		t.Error("bob deveria ter sido reconciliado mesmo com alice falhando")
	}
}

func TestReconcileUnixAccounts_BothEnabledTriesBoth(t *testing.T) {
	fp := &fakeUserProvisioner{}
	app, _ := withProvisioner(t, fp)
	alice := createTestUserWithRole(t, app, "alice", "senha-alice-123", store.RoleMember)
	app.Store.DB.Model(&alice).Updates(map[string]any{
		"sftp_enabled":   true,
		"samba_enabled":  true,
		"ssh_public_key": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5 alice",
	})

	if err := app.ReconcileUnixAccounts(context.Background()); err != nil {
		t.Fatalf("reconcile falhou: %v", err)
	}
	if len(fp.calls) != 2 {
		t.Fatalf("esperava 2 calls (SFTP + Samba), obtido %d: %v", len(fp.calls), fp.calls)
	}
	if fp.calls[0] != "EnableSFTP(alice,ssh-ed25519 AAAAC3NzaC1lZDI1NTE5 alice)" {
		t.Errorf("call[0] = %q, esperava EnableSFTP", fp.calls[0])
	}
	if fp.calls[1] != "EnableSamba(alice)" {
		t.Errorf("call[1] = %q, esperava EnableSamba", fp.calls[1])
	}
}
