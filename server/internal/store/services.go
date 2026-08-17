package store

import "time"

// ServiceKind é o tipo de instância orquestrada (Fase 43 — PLAN.md §6.18).
type ServiceKind string

const (
	ServiceKindMongo  ServiceKind = "mongo"
	ServiceKindRedis  ServiceKind = "redis"
	ServiceKindRabbit ServiceKind = "rabbitmq"
	ServiceKindLB     ServiceKind = "lb"
)

func (k ServiceKind) Valid() bool {
	switch k {
	case ServiceKindMongo, ServiceKindRedis, ServiceKindRabbit, ServiceKindLB:
		return true
	}
	return false
}

func (k ServiceKind) DefaultPort() uint16 {
	switch k {
	case ServiceKindRedis:
		return 6379
	case ServiceKindRabbit:
		return 5672
	case ServiceKindMongo:
		return 27018
	case ServiceKindLB:
		return 9080
	}
	return 0
}

// ServiceBind é a interface em que o processo escuta.
type ServiceBind string

const (
	ServiceBindWG0      ServiceBind = "wg0"
	ServiceBindLoopback ServiceBind = "loopback"
)

func (b ServiceBind) Valid() bool {
	return b == ServiceBindWG0 || b == ServiceBindLoopback
}

// ServiceHost é onde a instância roda.
type ServiceHost string

const (
	ServiceHostLocal ServiceHost = "local"
	ServiceHostMesh  ServiceHost = "mesh"
)

func (h ServiceHost) Valid() bool {
	return h == ServiceHostLocal || h == ServiceHostMesh
}

// ServiceStatus é o ciclo de vida da instância.
type ServiceStatus string

const (
	SvcPending ServiceStatus = "pending"
	SvcReady   ServiceStatus = "ready"
	SvcError   ServiceStatus = "error"
	SvcStopped ServiceStatus = "stopped"
)

func (s ServiceStatus) Valid() bool {
	switch s {
	case SvcPending, SvcReady, SvcError, SvcStopped:
		return true
	}
	return false
}

// ServiceInstance é um banco/fila/LB orquestrado no node local ou num
// peer da malha. Não é o Mongo do control-plane (127.0.0.1:27017).
type ServiceInstance struct {
	ID           uint          `gorm:"primaryKey"`
	Slug         string        `gorm:"uniqueIndex;not null"`
	Kind         ServiceKind   `gorm:"not null;index"`
	ProjectID    *uint         `gorm:"index"`
	Host         ServiceHost   `gorm:"not null;default:local;index"`
	MeshServerID *uint         `gorm:"index"`
	Bind         ServiceBind   `gorm:"not null;default:wg0"`
	Port         uint16        `gorm:"not null"`
	Status       ServiceStatus `gorm:"not null;default:pending;index"`
	// AuthSecret é a senha do serviço (Redis requirepass, etc.). Fica
	// só no VPS, nunca no GET — o painel mostra uma vez na criação
	// ou rotação, no mesmo padrão do token BitLaunch.
	AuthSecret string   `gorm:"type:text" json:"-"`
	Backends   []string `gorm:"serializer:json"`
	Error      string
	CreatedAt  time.Time
	UpdatedAt  time.Time

	Project    *Project    `gorm:"foreignKey:ProjectID"`
	MeshServer *MeshServer `gorm:"foreignKey:MeshServerID"`
}

// ServiceHostname é o A da intranet: svc-<slug>.corp.ihuull.com.
func ServiceHostname(slug string) string {
	return "svc-" + slug + ".corp.ihuull.com"
}

// ValidServiceSlug reusa a chave do forge (2–20, [a-z0-9-]).
func ValidServiceSlug(s string) bool {
	return ValidProjectSlug(s)
}
