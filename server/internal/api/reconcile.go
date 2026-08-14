package api

import (
	"context"
	"fmt"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

// ReconcileUnixAccounts converte o estado do DB para o sistema na
// inicialização do servidor (Fase 13, PLAN.md §6.9). Para cada
// usuário marcado como SFTPEnabled/SambaEnabled, re-aplica o
// provisionamento correspondente — as operações são idempotentes
// (criam a conta Unix se faltar, reescrevem drop-ins/authorized_keys/
// share include, recarregam sshd/smbd).
//
// Cenários que isto cobre:
//   - Servidor reiniciou no meio de um provisionamento (handler
//     gravou SFTPEnabled=true no DB mas o binário privilegiado não
//     chegou a rodar, ou rodou em parte).
//   - Admin restaurou o DB de um backup.
//   - Alguem apagou um drop-in/authorized_keys à mão na VPS.
//
// Limitação conhecida (fora do escopo do MVP da Fase 13): usuários
// marcados como DESligado no DB mas com config stale no sistema não
// são purgados aqui — confiamos nos Disable* do handler para a
// transição normal. Um futuro subcomando `purge` no binário poderia
// listar drop-ins/shares órfãos e removê-los.
//
// Retorna nil se todos os usuários convergiram, ou um erro agregando
// quantos falharam (o servidor ainda sobe — reconcile é best-effort,
// não bloqueia o boot; o admin vê o erro no log e pode re-rodar).
func (a *App) ReconcileUnixAccounts(ctx context.Context) error {
	if a.UserProvisioner == nil {
		return nil // Fase 13 não configurada — nada a fazer.
	}
	var users []store.User
	if err := a.Store.DB.Find(&users).Error; err != nil {
		return err
	}
	var failed []string
	for _, u := range users {
		if !u.SFTPEnabled && !u.SambaEnabled {
			continue // nada a reconciliar — estado negativo não é purgado.
		}
		if err := a.reconcileUser(ctx, u); err != nil {
			failed = append(failed, u.Username+": "+err.Error())
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("reconcile falhou para %d usuário(s): %s", len(failed), joinComma(failed))
	}
	return nil
}

// reconcileUser aplica o estado de um usuário só. Ordem: SFTP antes
// de Samba (ambos chamam Create internamente, então o segundo Create
// é no-op). Se SFTP falha, ainda tenta Samba — são independentes.
func (a *App) reconcileUser(ctx context.Context, u store.User) error {
	var firstErr error
	if u.SFTPEnabled {
		if err := a.UserProvisioner.EnableSFTP(ctx, u.Username, u.SSHPublicKey); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if u.SambaEnabled {
		if err := a.UserProvisioner.EnableSamba(ctx, u.Username); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// joinComma junta strings com ", ". Helper local pra não importar
// strings só por isso neste arquivo.
func joinComma(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}
