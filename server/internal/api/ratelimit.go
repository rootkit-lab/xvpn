package api

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// ipRateLimiter é um limitador simples por IP (janela deslizante em
// memória) — o único uso hoje é o endpoint público POST /api/waitlist
// (ver server.go), a única escrita da API que não exige autenticação.
// Não precisa ser distribuído/persistente: um restart do processo zera os
// contadores, o que é aceitável para o volume esperado (uso pessoal/
// familiar, não um serviço multi-tenant de alto tráfego — ver PLAN.md).
type ipRateLimiter struct {
	mu       sync.Mutex
	hits     map[string][]time.Time
	max      int
	window   time.Duration
	lastSwep time.Time
}

func newIPRateLimiter(max int, window time.Duration) *ipRateLimiter {
	return &ipRateLimiter{
		hits:   make(map[string][]time.Time),
		max:    max,
		window: window,
	}
}

// allow registra uma tentativa para ip e diz se ela deve ser aceita.
func (l *ipRateLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	// Poda oportunista: evita crescimento ilimitado do mapa sem precisar
	// de uma goroutine de limpeza dedicada só para isso.
	if now.Sub(l.lastSwep) > l.window {
		for k, times := range l.hits {
			if len(l.recent(times, now)) == 0 {
				delete(l.hits, k)
			}
		}
		l.lastSwep = now
	}

	recent := l.recent(l.hits[ip], now)
	if len(recent) >= l.max {
		l.hits[ip] = recent
		return false
	}
	recent = append(recent, now)
	l.hits[ip] = recent
	return true
}

func (l *ipRateLimiter) recent(times []time.Time, now time.Time) []time.Time {
	cutoff := now.Add(-l.window)
	out := times[:0]
	for _, t := range times {
		if t.After(cutoff) {
			out = append(out, t)
		}
	}
	return out
}

// rateLimit devolve um middleware Gin que aplica limiter.allow(ClientIP()).
// Depende da lista de proxies confiáveis configurada em NewRouter: sem
// ela o Gin aceita o X-Forwarded-For de qualquer origem e o próprio
// cliente escolhe a chave do limitador.
func rateLimit(limiter *ipRateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !limiter.allow(c.ClientIP()) {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "muitas tentativas, tente novamente mais tarde"})
			c.Abort()
			return
		}
		c.Next()
	}
}
