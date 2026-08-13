// Package applog configura log/slog no cliente (GUI e helper) e mantém
// um ring buffer em memória das últimas linhas — útil para diagnóstico
// sem gravar segredos em disco. Ver ROADMAP Fase 8.
package applog

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"sync"
)

const ringSize = 200

var (
	mu   sync.Mutex
	ring []string
)

type ringHandler struct {
	inner slog.Handler
}

func (h *ringHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *ringHandler) Handle(ctx context.Context, r slog.Record) error {
	line := r.Message
	attrs := make([]string, 0, r.NumAttrs())
	r.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, a.String())
		return true
	})
	if len(attrs) > 0 {
		line = line + " " + strings.Join(attrs, " ")
	}
	appendRing(r.Level.String() + " " + line)
	return h.inner.Handle(ctx, r)
}

func (h *ringHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ringHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *ringHandler) WithGroup(name string) slog.Handler {
	return &ringHandler{inner: h.inner.WithGroup(name)}
}

func appendRing(line string) {
	mu.Lock()
	defer mu.Unlock()
	if len(ring) < ringSize {
		ring = append(ring, line)
		return
	}
	// Desloca em memória já alocada e escreve por cima do slot mais
	// antigo, em vez de re-fatiar pela frente (ring = ring[1:]): aquele
	// padrão reduz a capacidade do slice a cada linha, forçando uma
	// realocação completa (cópia de ringSize elementos) a cada ~ringSize
	// chamadas em vez de nunca — capacidade fica fixa em ringSize para
	// sempre (ver ROADMAP.md Fase 9).
	copy(ring, ring[1:])
	ring[len(ring)-1] = line
}

// Recent devolve uma cópia das últimas linhas do ring (mais antigas primeiro).
func Recent() []string {
	mu.Lock()
	defer mu.Unlock()
	out := make([]string, len(ring))
	copy(out, ring)
	return out
}

// Setup configura slog JSON (ou texto se XVPN_LOG_FORMAT=text) e o ring.
// component diferencia helper vs gui nos logs.
func Setup(component string) *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(os.Getenv("XVPN_LOG_LEVEL")) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	opts := &slog.HandlerOptions{Level: level}
	var inner slog.Handler
	if strings.EqualFold(os.Getenv("XVPN_LOG_FORMAT"), "text") {
		inner = slog.NewTextHandler(os.Stderr, opts)
	} else {
		inner = slog.NewJSONHandler(os.Stderr, opts)
	}

	logger := slog.New(&ringHandler{inner: inner}).With("component", component)
	slog.SetDefault(logger)
	return logger
}
