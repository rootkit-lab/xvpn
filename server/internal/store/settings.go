package store

// PanelSettings é a linha singleton (ID=1) com valores editáveis pelo
// painel (Fase 15). WireGuard/rede continua só via env — mudar sub-rede
// ou porta em runtime sem restart quebraria peers e o firewall.
//
// Zero nos campos de TTL significa "usar o default do env/config" (ainda
// não editado pelo admin).
type PanelSettings struct {
	ID                    uint `gorm:"primaryKey"`
	InviteTokenTTLMinutes int  `gorm:"not null;default:0"`
	JWTTokenTTLMinutes    int  `gorm:"not null;default:0"`
}

// ForgeSettings é a linha singleton (ID=1) do XGIT (Fase 43 — UI GitLab).
type ForgeSettings struct {
	ID                uint          `gorm:"primaryKey"`
	DefaultVisibility AppVisibility `gorm:"not null;default:global"`
	DefaultNetwork    AppNetwork    `gorm:"not null;default:vpn"`
	AllowMemberCreate bool          `gorm:"not null;default:false"`
}
