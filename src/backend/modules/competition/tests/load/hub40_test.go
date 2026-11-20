//go:build load

// Package load holds the load tests of module competition (go test -tags load). TestHub40 measures the room host side
// of TC-PERF-003 / TC-PERF-023 (F091-AC-16): one hub instance of 40 bots moving at 10 Hz for HUB40_SECONDS of simulated
// play (default 600 = 10 minutes), the real domain tick and the real snapshot encoding of the room host. It reports
// tick p95/p99 against the room tick budget (≤ 5 ms, REALTIME_ARCHITECTURE §8) and the downstream bytes per viewer
// per second against hub_sync_bandwidth (≤ 30 KB/s, PERFORMANCE_BUDGET §10). The WebSocket end-to-end run with bots
// through the gateway belongs to the load harness of the platform (not part of this test).
package load

import (
	"encoding/json"
	"math"
	"math/rand/v2"
	"os"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/hub"
)

const (
	bots         = 40
	hz           = 10
	tickBudget   = 5 * time.Millisecond // room tick budget, hub of 40
	bandwidthMax = 30 * 1024            // hub_sync_bandwidth: ≤ 30 KB/s down per player
)

type snapshotOut struct {
	RoomID   string       `json:"room_id"`
	Tick     int64        `json:"tick"`
	AckSeq   int64        `json:"ack_client_seq"`
	Entities []hub.Entity `json:"entities"`
	Removed  []int        `json:"removed,omitempty"`
}

func TestHub40(t *testing.T) {
	seconds := 600
	if v, err := strconv.Atoi(os.Getenv("HUB40_SECONDS")); err == nil && v > 0 {
		seconds = v
	}
	now := time.Date(2026, 11, 21, 8, 0, 0, 0, time.UTC)
	area := hub.Area{ID: "main_hub", Capacity: 40, MaxSpeedMPS: 6, Min: hub.Vec3{-60, 0, -60}, Max: hub.Vec3{60, 10, 60},
		Bands: []string{"g1_3"}, Active: true}
	p := hub.Partition{TenantID: uuid.New(), SchoolID: uuid.New(), GradeBand: "g1_3"}
	room := uuid.NewString()
	in := hub.NewInstance(room, area, p, 40, now)
	type bot struct {
		id       uuid.UUID
		pos, dir hub.Vec3
		seq      int64
	}
	rng := rand.New(rand.NewPCG(1, 2))
	bs := make([]*bot, bots)
	for i := range bs {
		b := &bot{id: uuid.New(), pos: hub.Vec3{rng.Float64()*100 - 50, 0, rng.Float64()*100 - 50}}
		a := rng.Float64() * 2 * math.Pi
		b.dir = hub.Vec3{math.Cos(a), 0, math.Sin(a)}
		bs[i] = b
		s := hub.Seat{Player: hub.PublicPlayer{UserID: b.id, Nickname: "Bot" + strconv.Itoa(i), Level: 3,
			AvatarConfig: map[string]any{"body": "body_01", "hair": "hair_02"}}, TenantID: p.TenantID, SchoolID: p.SchoolID,
			GradeBand: "g1_3", Node: "gw-" + strconv.Itoa(i%4)}
		if _, _, err := in.Join(s, b.pos, now); err != nil {
			t.Fatal(err)
		}
	}
	ticks := seconds * hz
	durations := make([]time.Duration, 0, ticks)
	bytesPerViewer := map[uuid.UUID]int{}
	corrected := 0
	step := time.Second / hz
	for k := 0; k < ticks; k++ {
		now = now.Add(step)
		for _, b := range bs { // 10 Hz input at 5 m/s (walk/run), turning at the bounds
			next := hub.Vec3{b.pos[0] + b.dir[0]*0.5, 0, b.pos[2] + b.dir[2]*0.5}
			if !area.Contains(next) {
				b.dir = hub.Vec3{-b.dir[0], 0, -b.dir[2]}
				next = hub.Vec3{b.pos[0] + b.dir[0]*0.5, 0, b.pos[2] + b.dir[2]*0.5}
			}
			b.pos = next
			b.seq++
			res, err := in.Move(b.id, hub.Input{ClientSeq: b.seq, P: next, R: math.Atan2(b.dir[2], b.dir[0]), A: "walk"}, now)
			if err != nil {
				t.Fatal(err)
			}
			if res.Corrected {
				corrected++
			}
		}
		start := time.Now()
		snaps := in.Tick() // the room host tick: deltas per viewer + encoding of the payloads
		for _, s := range snaps {
			b, _ := json.Marshal(snapshotOut{RoomID: room, Tick: s.Tick, AckSeq: s.AckSeq, Entities: s.Entities, Removed: s.Removed})
			bytesPerViewer[s.UserID] += len(b) + 64 // + server envelope {v,type,seq,ts}
		}
		durations = append(durations, time.Since(start))
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	p95, p99 := durations[len(durations)*95/100], durations[len(durations)*99/100]
	worst := 0
	for _, n := range bytesPerViewer {
		if n > worst {
			worst = n
		}
	}
	perSecond := worst / seconds
	t.Logf("Hub40: %d s simulated, %d ticks, tick p95 %v p99 %v, worst viewer %d B/s (budget %d B/s), corrections %d",
		seconds, ticks, p95, p99, perSecond, bandwidthMax, corrected)
	if corrected != 0 {
		t.Errorf("%d legitimate moves corrected", corrected)
	}
	if p95 > tickBudget {
		t.Errorf("tick p95 %v over the budget %v", p95, tickBudget)
	}
	if perSecond > bandwidthMax {
		t.Errorf("downstream %d B/s per viewer over hub_sync_bandwidth", perSecond)
	}
}
