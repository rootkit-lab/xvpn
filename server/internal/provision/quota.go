package provision

import "fmt"

// MaxDiskQuotaMB evita overflow ao converter MB→KiB (uint64) e quotas
// absurdamente grandes no painel. 1 TiB é folga demais pro VPS atual.
const MaxDiskQuotaMB = 1024 * 1024 // 1 TiB

// SetDiskQuotaMB aplica a quota de disco do usuário (Fase 15). quotaMB=0
// remove o limite. Exige usrquota montado no filesystem (ver
// server/deploy/quota/README.md). Idempotente.
func SetDiskQuotaMB(r Runner, username string, quotaMB uint64) error {
	if !ValidUsername(username) {
		return ErrInvalidUsername
	}
	if quotaMB > MaxDiskQuotaMB {
		return fmt.Errorf("quota acima do máximo permitido (%d MiB)", MaxDiskQuotaMB)
	}
	blocksKB := quotaMB * 1024
	return r.SetUserQuota(username, blocksKB)
}
