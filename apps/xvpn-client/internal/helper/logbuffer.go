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
		b.append(line)
	}
	return len(p), nil
}

// append mantém b.lines com capacidade fixa em maxLines: uma vez cheio,
// desloca em memória já alocada em vez de re-fatiar pela frente (o que
// reduziria a capacidade a cada linha e forçaria realocação completa a
// cada ~maxLines chamadas — ver ROADMAP.md Fase 9 e applog.appendRing,
// mesmo padrão).
func (b *ringBuffer) append(line string) {
	if len(b.lines) < b.maxLines {
		b.lines = append(b.lines, line)
		return
	}
	copy(b.lines, b.lines[1:])
	b.lines[len(b.lines)-1] = line
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
