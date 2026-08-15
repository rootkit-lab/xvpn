package api

import (
	"encoding/json"
	"testing"
	"time"
)

func TestHub_PresenceSnapshotOmitsInvisible(t *testing.T) {
	h := newHub()
	alice := h.subscribe(1)
	bob := h.subscribe(2)
	carol := h.subscribe(3)
	t.Cleanup(func() {
		h.unsubscribe(1, alice)
		h.unsubscribe(2, bob)
		h.unsubscribe(3, carol)
	})

	h.setStatus(1, "away")
	h.setStatus(2, "invisible")
	h.setStatus(3, "online")

	snap := h.presenceSnapshot()
	got := map[uint]string{}
	for _, row := range snap {
		id, _ := row["user_id"].(uint)
		st, _ := row["status"].(string)
		got[id] = st
	}
	if _, ok := got[2]; ok {
		t.Fatalf("usuário invisível não deve aparecer no snapshot: %+v", snap)
	}
	if got[1] != "away" || got[3] != "online" {
		t.Fatalf("snapshot inesperado: %+v", snap)
	}
	if h.statusOf(2) != "offline" {
		t.Fatalf("status visível de invisível deveria ser offline, obtido %q", h.statusOf(2))
	}
	if h.ownStatus(2) != "invisible" {
		t.Fatalf("status próprio deveria permanecer invisible, obtido %q", h.ownStatus(2))
	}
}

func TestHub_InvisibleLooksLikeDisconnect(t *testing.T) {
	h := newHub()
	alice := h.subscribe(1)
	bob := h.subscribe(2)
	t.Cleanup(func() {
		h.unsubscribe(1, alice)
		h.unsubscribe(2, bob)
	})
	drain(bob)

	h.setStatus(1, "invisible")
	ev := readPresence(t, bob)
	if statusOfPayload(ev) != "offline" {
		t.Fatalf("ficar invisível deve anunciar offline, obtido %+v", ev)
	}
	if id, _ := ev["user_id"].(float64); id != 1 {
		t.Fatalf("presence deveria ser do user 1, obtido %+v", ev)
	}

	h.setStatus(1, "invisible")
	select {
	case raw := <-bob:
		t.Fatalf("reafirmar invisível não deve reanunciar: %s", raw)
	case <-time.After(50 * time.Millisecond):
	}

	carol := h.subscribe(3)
	t.Cleanup(func() { h.unsubscribe(3, carol) })
	h.announceJoin(1)
	select {
	case raw := <-bob:
		t.Fatalf("reconectar/anunciar invisível não deve vazar user_id: %s", raw)
	case <-time.After(50 * time.Millisecond):
	}
}

func drain(ch chan []byte) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

func readPresence(t *testing.T, ch chan []byte) map[string]any {
	t.Helper()
	select {
	case raw := <-ch:
		var ev wsEvent
		if err := json.Unmarshal(raw, &ev); err != nil {
			t.Fatalf("json: %v (%s)", err, raw)
		}
		if ev.Type != "presence" {
			t.Fatalf("esperado presence, obtido %s (%s)", ev.Type, raw)
		}
		b, _ := json.Marshal(ev.Payload)
		var p map[string]any
		_ = json.Unmarshal(b, &p)
		return p
	case <-time.After(2 * time.Second):
		t.Fatal("timeout esperando presence")
		return nil
	}
}

func statusOfPayload(p map[string]any) string {
	s, _ := p["status"].(string)
	return s
}
