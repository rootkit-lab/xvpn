package forge

import (
	"bytes"
	"io"
	"testing"
)

func TestParseReceivePack(t *testing.T) {
	// 0000 flush after one update. SHA-1 hex is 40 chars.
	old := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	newSHA := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	line := old + " " + newSHA + " refs/heads/main\n"
	pkt := pkt(line) + "0000PACK"
	updates, rest, err := ParseReceivePack(bytes.NewReader([]byte(pkt)))
	if err != nil {
		t.Fatal(err)
	}
	if len(updates) != 1 || updates[0].Ref != "refs/heads/main" || updates[0].NewHex != newSHA {
		t.Fatalf("updates: %+v", updates)
	}
	body, err := io.ReadAll(rest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(body, []byte(pkt)) {
		t.Fatalf("body reconstruído diverge: %q", body)
	}
}

func pkt(s string) string {
	n := len(s) + 4
	return sprintf4(n) + s
}

func sprintf4(n int) string {
	const hexdigits = "0123456789abcdef"
	out := [4]byte{}
	for i := 3; i >= 0; i-- {
		out[i] = hexdigits[n&0xf]
		n >>= 4
	}
	return string(out[:])
}
