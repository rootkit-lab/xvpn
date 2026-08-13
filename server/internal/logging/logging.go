// Package logging configura o logger estruturado padrão do processo
// (log/slog em JSON). Usado pelo xvpn-server para journalctl/systemd
// parseável — ver ROADMAP Fase 8.
package logging

import (
	"log/slog"
	"os"
	"strings"
)

// Setup inicializa slog como logger padrão do processo.
// XVPN_LOG_FORMAT=text força texto (útil em desenvolvimento local);
// o padrão em produção é JSON.
func Setup() *slog.Logger {
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
	var handler slog.Handler
	if strings.EqualFold(os.Getenv("XVPN_LOG_FORMAT"), "text") {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	logger := slog.New(handler).With("component", "xvpn-server")
	slog.SetDefault(logger)
	return logger
}
