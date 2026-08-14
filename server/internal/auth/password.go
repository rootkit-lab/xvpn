// Package auth implementa hash de senha (Argon2id) e emissão/validação de
// JWT para o painel administrativo do xvpn-server.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Parâmetros do Argon2id. Seguem as recomendações da OWASP (2024) para uso
// interativo: memória alta o bastante para dificultar ataques de GPU/ASIC,
// mas viável num VPS pequeno com poucos logins simultâneos (painel admin,
// não uma API de alto tráfego).
const (
	argonTime    = 3
	argonMemory  = 64 * 1024 // 64 MB
	argonThreads = 2
	argonKeyLen  = 32
	saltLen      = 16
)

// HashPassword gera um hash Argon2id codificado no formato PHC-like usado
// por várias libs de referência: $argon2id$v=19$m=...,t=...,p=...$salt$hash
func HashPassword(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("gerando salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	encoded := fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argonMemory,
		argonTime,
		argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	)
	return encoded, nil
}

// randomPasswordBytes é o tamanho (em bytes, antes da codificação) das
// senhas aleatórias geradas pelo servidor — bootstrap do primeiro admin
// (cmd/xvpn-server/main.go) e reset de senha pelo admin (Fase 10, ver
// api/users_handler.go). 18 bytes vira 24 caracteres em base64url, entropia
// bem acima do mínimo de 8 caracteres exigido de senhas escolhidas por
// humano.
const randomPasswordBytes = 18

// GenerateRandomPassword gera uma senha aleatória criptograficamente segura,
// codificada em base64url (sem padding, sem caracteres que exigem escape em
// URL/JSON). Usada sempre que o servidor precisa criar uma senha sem que o
// operador tenha fornecido uma — ela é devolvida ao chamador uma única vez;
// o servidor nunca a guarda em texto puro.
func GenerateRandomPassword() (string, error) {
	buf := make([]byte, randomPasswordBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("gerando senha: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// VerifyPassword confere se password bate com o hash codificado gerado por
// HashPassword. Usa comparação em tempo constante para evitar timing attacks.
func VerifyPassword(encoded, password string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, fmt.Errorf("formato de hash desconhecido")
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, fmt.Errorf("versão do hash inválida: %w", err)
	}

	var memory uint32
	var time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return false, fmt.Errorf("parâmetros do hash inválidos: %w", err)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("salt inválido: %w", err)
	}
	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("hash inválido: %w", err)
	}

	computedHash := argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(expectedHash)))

	if subtle.ConstantTimeCompare(computedHash, expectedHash) == 1 {
		return true, nil
	}
	return false, nil
}
