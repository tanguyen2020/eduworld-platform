package unit

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/hub"
)

// T011 (F091-AC-03, AC-04): friends first, then the most crowded instance below 90 %, else a new one; an instance
// with a blocked student is avoided; a full instance is never chosen.
func TestChoose(t *testing.T) {
	cases := []struct {
		name  string
		cands []hub.Candidate
		want  string
	}{
		{"empty partition → new", nil, ""},
		{"friends win over crowd", []hub.Candidate{{RoomID: "a", Members: 30, Capacity: 40}, {RoomID: "b", Members: 5, Capacity: 40, Friends: 1}}, "b"},
		{"most friends", []hub.Candidate{{RoomID: "a", Members: 10, Capacity: 40, Friends: 1}, {RoomID: "b", Members: 5, Capacity: 40, Friends: 2}}, "b"},
		{"friends tie → more members", []hub.Candidate{{RoomID: "a", Members: 10, Capacity: 40, Friends: 1}, {RoomID: "b", Members: 12, Capacity: 40, Friends: 1}}, "b"},
		{"friend's instance full → crowd rule", []hub.Candidate{{RoomID: "a", Members: 40, Capacity: 40, Friends: 3}, {RoomID: "b", Members: 20, Capacity: 40}}, "b"},
		{"friend's instance ≥ 90 % but has a seat", []hub.Candidate{{RoomID: "a", Members: 38, Capacity: 40, Friends: 1}}, "a"},
		{"most crowded below 90 %", []hub.Candidate{{RoomID: "a", Members: 10, Capacity: 40}, {RoomID: "b", Members: 35, Capacity: 40}, {RoomID: "c", Members: 36, Capacity: 40}}, "b"},
		{"all ≥ 90 % → new", []hub.Candidate{{RoomID: "a", Members: 36, Capacity: 40}, {RoomID: "b", Members: 39, Capacity: 40}}, ""},
		{"full → new (AC-04)", []hub.Candidate{{RoomID: "a", Members: 40, Capacity: 40}}, ""},
		{"blocked avoided", []hub.Candidate{{RoomID: "a", Members: 30, Capacity: 40, Friends: 2, Blocked: true}, {RoomID: "b", Members: 3, Capacity: 40}}, "b"},
		{"blocked only → new", []hub.Candidate{{RoomID: "a", Members: 3, Capacity: 40, Blocked: true}}, ""},
		{"smaller area capacity", []hub.Candidate{{RoomID: "a", Members: 9, Capacity: 10}, {RoomID: "b", Members: 8, Capacity: 10}}, "b"},
	}
	for _, c := range cases {
		if got := hub.Choose(c.cands); got.RoomID != c.want || got.New() != (c.want == "") {
			t.Errorf("%s: got %q, want %q", c.name, got.RoomID, c.want)
		}
	}
}

// F091-AC-05: every refusal but a full instance is the same PERMISSION_DENIED outcome.
func TestFollow(t *testing.T) {
	if hub.Follow(true, false, true, true, true) != hub.FollowOK {
		t.Fatal("friend ok")
	}
	if hub.Follow(true, false, true, true, false) != hub.FollowFull {
		t.Fatal("full")
	}
	for i, c := range [][5]bool{{false, false, true, true, true}, {true, true, true, true, true}, {true, false, false, true, true},
		{true, false, true, false, true}, {false, false, false, false, false}} {
		if hub.Follow(c[0], c[1], c[2], c[3], c[4]) != hub.FollowDenied {
			t.Errorf("case %d not denied", i)
		}
	}
}

// F091-AC-02: grade bands of GLOSSARY; an instance refuses a seat of another school or band, and a full instance
// refuses a new seat but gives a held seat back (F091-AC-04, AC-09).
func TestPartitionAndCapacity(t *testing.T) {
	for grade, band := range map[int]string{1: "g1_3", 3: "g1_3", 4: "g4_5", 5: "g4_5", 6: "g6_7", 7: "g6_7", 8: "g8_9", 9: "g8_9"} {
		if b, ok := hub.BandOf(grade); !ok || b != band {
			t.Errorf("grade %d → %s", grade, b)
		}
	}
	for _, g := range []int{0, 10, -1} {
		if _, ok := hub.BandOf(g); ok {
			t.Errorf("grade %d accepted", g)
		}
	}
	if hub.Capacity(40, 40) != 40 || hub.Capacity(10, 40) != 10 || hub.Capacity(50, 40) != 40 || hub.Capacity(0, 40) != 40 {
		t.Fatal("capacity = min(area, hub_instance_capacity)")
	}
	now := time.Now()
	p := hub.Partition{TenantID: uuid.New(), SchoolID: uuid.New(), GradeBand: "g1_3"}
	in := hub.NewInstance("r1", testArea(), p, 2, now)
	seat := func(band string, school uuid.UUID) hub.Seat {
		return hub.Seat{Player: hub.PublicPlayer{UserID: uuid.New(), Nickname: "N"}, TenantID: p.TenantID, SchoolID: school, GradeBand: band}
	}
	if _, _, err := in.Join(seat("g8_9", p.SchoolID), hub.Vec3{}, now); !errors.Is(err, hub.ErrWrongPartition) {
		t.Fatalf("g8_9 in g1_3: %v", err)
	}
	if _, _, err := in.Join(seat("g1_3", uuid.New()), hub.Vec3{}, now); !errors.Is(err, hub.ErrWrongPartition) {
		t.Fatalf("other school: %v", err)
	}
	other := seat("g1_3", p.SchoolID)
	other.TenantID = uuid.New()
	if _, _, err := in.Join(other, hub.Vec3{}, now); !errors.Is(err, hub.ErrWrongPartition) {
		t.Fatalf("other tenant: %v", err)
	}
	a, b := seat("g1_3", p.SchoolID), seat("g1_3", p.SchoolID)
	ma, _, err := in.Join(a, hub.Vec3{}, now)
	if err != nil || ma.EID != 1 {
		t.Fatalf("a: %v", err)
	}
	if _, _, err := in.Join(b, hub.Vec3{}, now); err != nil {
		t.Fatal(err)
	}
	if _, _, err := in.Join(seat("g1_3", p.SchoolID), hub.Vec3{}, now); !errors.Is(err, hub.ErrFull) {
		t.Fatalf("third seat: %v", err)
	}
	in.Hold(a.Player.UserID, now, 30*time.Second)
	if m, rejoined, err := in.Join(a, hub.Vec3{}, now.Add(10*time.Second)); err != nil || !rejoined || m.EID != 1 || m.Held() {
		t.Fatalf("held seat back: %v %v", rejoined, err)
	}
	// grace over → removed (hub.player.left by the room host)
	in.Hold(b.Player.UserID, now, 30*time.Second)
	if gone := in.Expired(now.Add(29*time.Second), 30*time.Second, 0); len(gone) != 0 {
		t.Fatal("removed before the end of the grace")
	}
	if gone := in.Expired(now.Add(30*time.Second), 30*time.Second, 0); len(gone) != 1 || gone[0].Player.UserID != b.Player.UserID {
		t.Fatalf("expired %v", gone)
	}
	// silence fallback: no input for silence, then grace
	if gone := in.Expired(now.Add(10*time.Minute), 30*time.Second, 10*time.Second); len(gone) != 1 {
		t.Fatalf("silent member kept: %v", gone)
	}
	if in.IdleFor(now.Add(16*time.Minute)) < 5*time.Minute {
		t.Fatal("empty instance not idle")
	}
}

func testArea() hub.Area {
	return hub.Area{ID: "main_hub", Capacity: 40, MaxSpeedMPS: 6, Min: hub.Vec3{-100, -10, -100}, Max: hub.Vec3{100, 50, 100},
		Bands: []string{"g1_3"}, Active: true}
}
