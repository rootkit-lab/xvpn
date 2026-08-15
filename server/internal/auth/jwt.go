package auth

import (
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

// Claims são as informações carregadas no token de sessão do painel. Role
// (Fase 10) é o que os middlewares de autorização checam — mudar o papel de
// um usuário só tem efeito prático no próximo login (o token antigo carrega
// o papel de quando foi emitido, até expirar).
type Claims struct {
	UserID   uint       `json:"uid"`
	Username string     `json:"username"`
	Role     store.Role `json:"role"`
	jwt.RegisteredClaims
}

// TokenManager emite e valida JWTs assinados com HMAC-SHA256.
type TokenManager struct {
	mu     sync.Mutex
	secret []byte
	ttl    time.Duration
}

// NewTokenManager cria um TokenManager. secret deve ter pelo menos 32 bytes
// (validado em config.Load).
func NewTokenManager(secret string, ttl time.Duration) *TokenManager {
	return &TokenManager{secret: []byte(secret), ttl: ttl}
}

// SetTTL atualiza a validade usada em Issue (Fase 15 — edição via painel).
// Tokens já emitidos mantêm a expiração original.
func (t *TokenManager) SetTTL(ttl time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ttl = ttl
}

// Issue gera um novo JWT válido por t.ttl para o usuário informado.
func (t *TokenManager) Issue(userID uint, username string, role store.Role) (string, error) {
	t.mu.Lock()
	ttl := t.ttl
	t.mu.Unlock()
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			Subject:   username,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(t.secret)
}

// Parse valida a assinatura/expiração de um token e retorna suas claims.
// Nunca logar o token completo nem o conteúdo retornado (ver go-backend.mdc).
func (t *TokenManager) Parse(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("método de assinatura inesperado: %v", token.Header["alg"])
		}
		return t.secret, nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, fmt.Errorf("token inválido")
	}
	return claims, nil
}
