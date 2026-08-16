package auth

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/golang-jwt/jwt/v5"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

const (
	IssuerURL       = "https://xauth.ihuull.com"
	LegacyIssuerURL = "https://xvpn.ihuull.com"

	AudXvpn    = "xvpn"
	AudXchat   = "xchat"
	AudXgroup  = "xgroup"
	AudXdriver = "xdriver"
)

// Claims são as informações do token de sessão (payload do JWE).
type Claims struct {
	UserID   uint       `json:"uid"`
	Username string     `json:"username"`
	Role     store.Role `json:"role"`
	jwt.RegisteredClaims
}

// TokenManager emite e valida só JWE (dir + A256GCM). JWT HMAC não é aceito.
type TokenManager struct {
	mu     sync.Mutex
	secret []byte
	ttl    time.Duration
	issuer string
}

func NewTokenManager(secret string, ttl time.Duration) *TokenManager {
	return &TokenManager{secret: []byte(secret), ttl: ttl, issuer: IssuerURL}
}

func (t *TokenManager) SetTTL(ttl time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ttl = ttl
}

func (t *TokenManager) TTL() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.ttl
}

func (t *TokenManager) aeadKey() []byte {
	if len(t.secret) >= 32 {
		return t.secret[:32]
	}
	return t.secret
}

func (t *TokenManager) Issue(userID uint, username string, role store.Role) (string, error) {
	return t.IssueFor(userID, username, role, AudXvpn)
}

func (t *TokenManager) IssueFor(userID uint, username string, role store.Role, aud string) (string, error) {
	if aud == "" {
		aud = AudXvpn
	}
	t.mu.Lock()
	ttl := t.ttl
	t.mu.Unlock()
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    t.issuer,
			Subject:   username,
			Audience:  jwt.ClaimStrings{aud},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	enc, err := jose.NewEncrypter(jose.A256GCM, jose.Recipient{Algorithm: jose.DIRECT, Key: t.aeadKey()}, (&jose.EncrypterOptions{}).WithType("JWE"))
	if err != nil {
		return "", fmt.Errorf("jwe encrypter: %w", err)
	}
	obj, err := enc.Encrypt(payload)
	if err != nil {
		return "", fmt.Errorf("jwe encrypt: %w", err)
	}
	return obj.CompactSerialize()
}

// Parse valida um JWE compacto. Qualquer outro formato (incl. JWT HS256) é rejeitado.
func (t *TokenManager) Parse(tokenString string) (*Claims, error) {
	if strings.Count(tokenString, ".") != 4 {
		return nil, fmt.Errorf("token não é JWE")
	}
	obj, err := jose.ParseEncrypted(tokenString, []jose.KeyAlgorithm{jose.DIRECT}, []jose.ContentEncryption{jose.A256GCM})
	if err != nil {
		return nil, err
	}
	plain, err := obj.Decrypt(t.aeadKey())
	if err != nil {
		return nil, err
	}
	claims := &Claims{}
	if err := json.Unmarshal(plain, claims); err != nil {
		return nil, err
	}
	if claims.ExpiresAt != nil && time.Now().After(claims.ExpiresAt.Time) {
		return nil, fmt.Errorf("token expirado")
	}
	if claims.Issuer != "" && claims.Issuer != t.issuer && claims.Issuer != LegacyIssuerURL {
		return nil, fmt.Errorf("issuer inesperado")
	}
	return claims, nil
}

func NormalizeAudience(aud string) string {
	switch strings.ToLower(strings.TrimSpace(aud)) {
	case AudXchat:
		return AudXchat
	case AudXgroup:
		return AudXgroup
	case AudXdriver:
		return AudXdriver
	default:
		return AudXvpn
	}
}
