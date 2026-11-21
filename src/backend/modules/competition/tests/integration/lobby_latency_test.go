//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"testing"
	"time"

	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/realtime"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// T015 (integration form, SC-001, AC-PTY-003): a party of 4 with the real Valkey adapters; each Ready change reaches the
// connection channel of another member (ew:rt:user:<id>, what the gateway subscribes) — p95 ≤ 1 s over 24 changes. The E2E
// form through the gateway with 4 headless clients needs the compose stack (no E2E harness yet, F090).
func TestLobbyReadyLatency(t *testing.T) {
	store, vk := valkeyLobby(t)
	e := setupParty(t, popts{lobby: store, realtime: realtime.PartyPublisher{P: rt.Publisher{C: vk}}})
	a, b, c, d := e.kidA(), e.kidA(), e.kidA(), e.kidA()
	id := e.party(a, b, c, d)
	for _, k := range []kid{a, b, c, d} {
		e.online.set(k.id, true)
	}
	got := make(chan time.Time, 64)
	dedicated, release := vk.Dedicate()
	defer release()
	dedicated.SetPubSubHooks(valkey.PubSubHooks{OnMessage: func(m valkey.PubSubMessage) {
		var env rt.Envelope
		if json.Unmarshal([]byte(m.Message), &env) == nil && env.Message != nil && env.Message.Type == "party.member.ready_changed" {
			got <- time.Now()
		}
	}})
	if err := dedicated.Do(context.Background(), dedicated.B().Subscribe().Channel(rt.UserChannel(a.id)).Build()).Error(); err != nil {
		t.Fatal(err)
	}
	var lat []time.Duration
	senders := []kid{b, c, d}
	for i := 0; i < 24; i++ {
		k := senders[i%3]
		start := time.Now()
		if r := e.ready(k, id, i%2 == 0); r.status != http.StatusNoContent {
			t.Fatalf("ready %d: %d %s", i, r.status, r.raw)
		}
		select {
		case at := <-got:
			lat = append(lat, at.Sub(start))
		case <-time.After(3 * time.Second):
			t.Fatalf("change %d not delivered", i)
		}
		time.Sleep(120 * time.Millisecond) // party_lobby_cmd_rate 5/s per student
	}
	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })
	p95 := lat[int(float64(len(lat))*0.95)-1]
	t.Logf("ready_changed latency p50 %v p95 %v max %v (n=%d)", lat[len(lat)/2], p95, lat[len(lat)-1], len(lat))
	if p95 > time.Second {
		t.Fatalf("p95 %v > 1 s", p95)
	}
}
