package helper

import (
	"strconv"
	"strings"
	"testing"
)

func TestRingBuffer_CapsAtMaxLines(t *testing.T) {
	b := newRingBuffer(5)
	for i := 0; i < 12; i++ {
		if _, err := b.Write([]byte("linha-" + strconv.Itoa(i) + "\n")); err != nil {
			t.Fatalf("erro escrevendo no ring: %v", err)
		}
	}

	lines := b.Lines()
	want := []string{"linha-7", "linha-8", "linha-9", "linha-10", "linha-11"}
	if strings.Join(lines, ",") != strings.Join(want, ",") {
		t.Fatalf("esperava as %d linhas mais recentes em ordem %v, obtido %v", len(want), want, lines)
	}
}

func TestRingBuffer_SplitsMultipleLinesInOneWrite(t *testing.T) {
	b := newRingBuffer(10)
	if _, err := b.Write([]byte("a\nb\nc\n")); err != nil {
		t.Fatalf("erro escrevendo no ring: %v", err)
	}
	lines := b.Lines()
	if strings.Join(lines, ",") != "a,b,c" {
		t.Fatalf("esperava [a b c], obtido %v", lines)
	}
}

func TestRingBuffer_LinesReturnsCopy(t *testing.T) {
	b := newRingBuffer(5)
	if _, err := b.Write([]byte("original\n")); err != nil {
		t.Fatalf("erro escrevendo no ring: %v", err)
	}
	lines := b.Lines()
	lines[0] = "adulterada"

	fresh := b.Lines()
	if fresh[0] != "original" {
		t.Fatalf("Lines() deveria devolver uma cópia — mutação externa vazou para o estado interno: %v", fresh)
	}
}
