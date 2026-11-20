package unit

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/hub"
)

// T020 (F091-AC-12, AC-13): a move within max_speed × Δt × 1.2 and the bounds is accepted; a 50 m jump in 100 ms or a
// position out of the bounds keeps the last valid position and counts a violation; more than 20 violations in a minute
// make one integrity signal (not more than one per minute), never a sanction.
func TestMovementValidator(t *testing.T) {
	now := time.Unix(1_763_190_000, 0)
	area := testArea() // 6 m/s
	p := hub.Partition{TenantID: uuid.New(), SchoolID: uuid.New(), GradeBand: "g1_3"}
	in := hub.NewInstance("r1", area, p, 40, now)
	user := uuid.New()
	if _, _, err := in.Join(hub.Seat{Player: hub.PublicPlayer{UserID: user, Nickname: "A"}, TenantID: p.TenantID, SchoolID: p.SchoolID,
		GradeBand: "g1_3"}, hub.Vec3{0, 0, 0}, now); err != nil {
		t.Fatal(err)
	}
	seq := int64(0)
	move := func(dt time.Duration, to hub.Vec3) hub.MoveResult {
		t.Helper()
		now = now.Add(dt)
		seq++
		r, err := in.Move(user, hub.Input{ClientSeq: seq, P: to, R: 1, A: "run"}, now)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	// After a full budget (1 s at rest), sustained running at max speed is accepted at 10 Hz.
	now = now.Add(time.Second)
	x := 0.0
	for i := 0; i < 50; i++ {
		x += 0.6 // 6 m/s × 100 ms
		if r := move(100*time.Millisecond, hub.Vec3{x, 0, 0}); r.Corrected {
			t.Fatalf("step %d corrected: %+v", i, r)
		}
	}
	// Sustained 1.5× speed is caught once the 1 s slack is spent.
	corrected := false
	for i := 0; i < 60 && !corrected; i++ { // the 1 s slack (7.2 m) is spent after 40 steps
		x += 0.9
		corrected = move(100*time.Millisecond, hub.Vec3{x, 0, 0}).Corrected
	}
	if !corrected {
		t.Fatal("1.5× max speed never corrected")
	}
	m, _ := in.Member(user)
	last := m.Mover.Pos
	// Teleport 50 m in 100 ms → speed, position unchanged.
	r := move(100*time.Millisecond, hub.Vec3{last[0] - 50, 0, 0})
	if !r.Corrected || r.Reason != hub.ReasonSpeed || r.Valid != last {
		t.Fatalf("teleport: %+v", r)
	}
	// Out of bounds.
	r = move(time.Second, hub.Vec3{last[0], 0, 100.5})
	if !r.Corrected || r.Reason != hub.ReasonOutOfBounds || r.Valid != last {
		t.Fatalf("bounds: %+v", r)
	}
	if r := move(time.Second, hub.Vec3{last[0], 0, math.Inf(1)}); !r.Corrected || r.Reason != hub.ReasonOutOfBounds {
		t.Fatalf("inf: %+v", r)
	}
	// Stale input (lower client_seq) ignored.
	now = now.Add(time.Second)
	if r, _ := in.Move(user, hub.Input{ClientSeq: 1, P: hub.Vec3{0, 0, 0}}, now); r.Corrected || m.Mover.Pos != last {
		t.Fatalf("stale input applied: %+v", r)
	}
	// > 20 violations per minute → one signal per minute.
	signals := 0
	for i := 0; i < 25; i++ {
		if r := move(100*time.Millisecond, hub.Vec3{last[0] - 50, 0, 0}); r.Signal {
			signals++
			if r.Violations <= hub.ViolationLimit {
				t.Fatalf("signal at %d violations", r.Violations)
			}
		}
	}
	if signals != 1 {
		t.Fatalf("signals %d, want 1", signals)
	}
	if _, err := in.Move(uuid.New(), hub.Input{ClientSeq: 1}, now); !errors.Is(err, hub.ErrNotMember) {
		t.Fatalf("stranger: %v", err)
	}
}

// F091-AC-18: an emote must be on the avatar's wheel and respect emote_rate.
func TestEmote(t *testing.T) {
	now := time.Now()
	p := hub.Partition{TenantID: uuid.New(), SchoolID: uuid.New(), GradeBand: "g1_3"}
	in := hub.NewInstance("r1", testArea(), p, 40, now)
	user := uuid.New()
	player := hub.PublicPlayer{UserID: user, Nickname: "A", AvatarConfig: map[string]any{"emote_slots": []any{"emote_wave", "emote_clap"}}}
	if _, _, err := in.Join(hub.Seat{Player: player, TenantID: p.TenantID, SchoolID: p.SchoolID, GradeBand: "g1_3"}, hub.Vec3{}, now); err != nil {
		t.Fatal(err)
	}
	if _, owned, wait, _ := in.Emote(user, "emote_wave", now, 2*time.Second); !owned || wait != 0 {
		t.Fatal("owned emote refused")
	}
	if _, owned, wait, _ := in.Emote(user, "emote_clap", now.Add(time.Second), 2*time.Second); !owned || wait != time.Second {
		t.Fatalf("rate: wait %v", wait)
	}
	if _, owned, _, _ := in.Emote(user, "emote_dance", now.Add(3*time.Second), 2*time.Second); owned {
		t.Fatal("emote not on the wheel accepted")
	}
	if _, _, _, err := in.Emote(uuid.New(), "emote_wave", now, 2*time.Second); !errors.Is(err, hub.ErrNotMember) {
		t.Fatal("stranger emote")
	}
}
