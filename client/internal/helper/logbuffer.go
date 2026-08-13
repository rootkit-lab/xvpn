package helper

import (
	"strings"
	"sync"
)

// ringBuffer é um io.Writer que guarda só as últimas maxLines linhas
// gravadas nele — usado para expor os logs recentes do helper à página
// de diagnóstico da GUI (ver ROADMAP.md Fase 6) sem depender de
// journalctl/Visualizador de Eventos, que o processo GUI sem privilégio
// não tem acesso direto de qualquer forma.
type ringBuffer struct {
	mu       sync.Mutex
	lines    []string
	maxLines int
}

func newRingBuffer(maxLines int) *ringBuffer {
	return &ringBuffer{maxLines: maxLines}
}

func (b *ringBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, line := range strings.Split(strings.TrimRight(string(p), "\n"), "\n") {
		if line == "" {
			continue
		}
		b.lines = append(b.lines, line)
	}
	if overflow := len(b.lines) - b.maxLines; overflow > 0 {
		b.lines = b.lines[overflow:]
	}
	return len(p), nil
}

// Lines devolve uma cópia das linhas atuais (mais antiga primeiro) —
// cópia evita expor o slice interno a mutação por quem chamou.
func (b *ringBuffer) Lines() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]string, len(b.lines))
	copy(out, b.lines)
	return out
}
