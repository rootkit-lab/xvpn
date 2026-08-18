package store

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MaxProjectEnvs          = 32
	MaxProjectEnvValueBytes = 4096
	projectEnvNameMaxRunes  = 64
)

var projectEnvNameRe = regexp.MustCompile(`^[A-Z][A-Z0-9_]{1,63}$`)

// ProjectEnv é um ENV do codespace gravado no Settings do repo (Fase 51.5).
// Não vai para o bare Git, XGROUP nem log de CI.
type ProjectEnv struct {
	ID        uint   `gorm:"primaryKey"`
	ProjectID uint   `gorm:"uniqueIndex:idx_project_env;not null"`
	Name      string `gorm:"uniqueIndex:idx_project_env;not null;size:64"`
	Value     string `gorm:"type:text" json:"-"`
	Secret    bool   `gorm:"not null;default:false"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func ValidProjectEnvName(name string) bool {
	if name == "" || utf8.RuneCountInString(name) > projectEnvNameMaxRunes {
		return false
	}
	return projectEnvNameRe.MatchString(name)
}

func BlockedProjectEnvName(name string) bool {
	switch name {
	case "PATH", "HOME":
		return true
	}
	for _, p := range []string{"LD_", "SSH_", "DOCKER_"} {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

func IsLLMProjectEnv(name string) bool {
	return strings.HasPrefix(name, "XCS_LLM_")
}

func ValidProjectEnvValue(value string) bool {
	if len(value) > MaxProjectEnvValueBytes {
		return false
	}
	return !strings.ContainsAny(value, "\x00\n\r")
}
