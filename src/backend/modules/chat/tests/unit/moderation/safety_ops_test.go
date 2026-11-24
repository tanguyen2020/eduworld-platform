package moderation

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation/breakglass"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation/oncall"
)

// F124 T031/T034: the on-call ladder of p1 cases (MODERATION §6.3) — primary at creation, secondary after
// p1_unclaimed_alert_minutes, lead after twice that, system_admin past the first response of report_sla; each level once
// (never back to a lower level), an empty level pages the next one that has people (F124-AC-14, AC-15).
func TestOncallLadder(t *testing.T) {
	l := oncall.Ladder{Unclaimed: 15 * time.Minute, FirstResponse: time.Hour}
	created := time.Date(2026, 11, 21, 23, 0, 0, 0, time.UTC)
	all := func(string) bool { return true }
	cases := []struct {
		last  string
		at    time.Duration
		has   func(string) bool
		want  string
		paged bool
	}{
		{"", 0, all, oncall.LevelPrimary, true},
		{oncall.LevelPrimary, 14 * time.Minute, all, "", false},
		{oncall.LevelPrimary, 15 * time.Minute, all, oncall.LevelSecondary, true},
		{oncall.LevelSecondary, 29 * time.Minute, all, "", false},
		{oncall.LevelSecondary, 30 * time.Minute, all, oncall.LevelLead, true},
		{oncall.LevelLead, 59 * time.Minute, all, "", false},
		{oncall.LevelLead, time.Hour, all, oncall.LevelSystemAdmin, true},
		{oncall.LevelSystemAdmin, 5 * time.Hour, all, "", false}, // every level once
		// No primary shift: the next level with people at once (F124-AC-15).
		{"", 0, func(l string) bool { return l != oncall.LevelPrimary }, oncall.LevelSecondary, true},
		{"", 0, func(l string) bool { return l != oncall.LevelPrimary && l != oncall.LevelSecondary }, oncall.LevelLead, true},
		// Lead paged early (no shifts): the secondary mark pages nothing lower.
		{oncall.LevelLead, 15 * time.Minute, func(l string) bool { return l == oncall.LevelLead || l == oncall.LevelSystemAdmin }, "", false},
		// A missed run pages the highest due level, not every level in turn.
		{oncall.LevelPrimary, 2 * time.Hour, all, oncall.LevelSystemAdmin, true},
	}
	for i, c := range cases {
		got, ok := l.Next(c.last, created, created.Add(c.at), c.has)
		if got != c.want || ok != c.paged {
			t.Errorf("case %d: %q %v, want %q %v", i, got, ok, c.want, c.paged)
		}
	}
}

func TestOncallShift(t *testing.T) {
	start := time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)
	ok := oncall.Shift{Role: oncall.RolePrimary, Start: start, End: start.Add(24 * time.Hour)}
	if err := ok.Validate(168 * time.Hour); err != nil {
		t.Fatal(err)
	}
	for _, s := range []oncall.Shift{
		{Role: "lead", Start: start, End: start.Add(time.Hour)},
		{Role: oncall.RolePrimary, Start: start, End: start},
		{Role: oncall.RolePrimary, Start: start, End: start.Add(169 * time.Hour)},
	} {
		if err := s.Validate(168 * time.Hour); !errors.Is(err, oncall.ErrInvalid) {
			t.Errorf("%+v: %v", s, err)
		}
	}
	other := oncall.Shift{Role: oncall.RolePrimary, Start: start.Add(23 * time.Hour), End: start.Add(30 * time.Hour)}
	if !ok.Overlaps(other) || other.Overlaps(oncall.Shift{Role: oncall.RoleSecondary, Start: start, End: start.Add(48 * time.Hour)}) {
		t.Fatal("overlap of one role only")
	}
	next := oncall.Shift{Role: oncall.RolePrimary, Start: start.Add(24 * time.Hour), End: start.Add(48 * time.Hour)}
	if ok.Overlaps(next) {
		t.Fatal("adjacent shifts do not overlap")
	}
	if !ok.Covers(start.Add(time.Hour)) || ok.Covers(start.Add(24*time.Hour)) {
		t.Fatal("covers [start, end)")
	}
}

// F124 T010/T044: the rules of a break-glass grant (MODERATION §6.1, CHILD_SAFETY §7) — request limits (F124-AC-21),
// two-person decision (AC-18), active window and scope (AC-19), revocation and expiry (AC-20).
func TestBreakGlassRequestLimits(t *testing.T) {
	lim := breakglass.Limits{MaxDuration: 4 * time.Hour, MaxWindow: 30 * 24 * time.Hour}
	from := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	reason := strings.Repeat("r", 20)
	good := breakglass.Request{ScopeType: breakglass.ScopeUser, WindowFrom: from, WindowTo: from.Add(7 * 24 * time.Hour), DurationMinutes: 60, Reason: reason}
	if err := good.Validate(lim); err != nil {
		t.Fatal(err)
	}
	for field, r := range map[string]breakglass.Request{
		"scope_type":       {ScopeType: "school", WindowFrom: from, WindowTo: from.Add(time.Hour), DurationMinutes: 60, Reason: reason},
		"window_to":        {ScopeType: breakglass.ScopeChannel, WindowFrom: from, WindowTo: from, DurationMinutes: 60, Reason: reason},
		"window":           {ScopeType: breakglass.ScopeChannel, WindowFrom: from, WindowTo: from.Add(31 * 24 * time.Hour), DurationMinutes: 60, Reason: reason},
		"duration_minutes": {ScopeType: breakglass.ScopeChannel, WindowFrom: from, WindowTo: from.Add(time.Hour), DurationMinutes: 241, Reason: reason},
		"reason":           {ScopeType: breakglass.ScopeChannel, WindowFrom: from, WindowTo: from.Add(time.Hour), DurationMinutes: 60, Reason: strings.Repeat("r", 19)},
	} {
		var fe breakglass.FieldError
		if err := r.Validate(lim); !errors.As(err, &fe) || fe.Field != field {
			t.Errorf("%s: %v", field, err)
		}
	}
	long := good
	long.Reason = strings.Repeat("r", 1001)
	if err := long.Validate(lim); err == nil {
		t.Fatal("reason over 1000")
	}
	zero := good
	zero.DurationMinutes = 0
	if err := zero.Validate(lim); err == nil {
		t.Fatal("duration 0")
	}
}

func TestBreakGlassDecision(t *testing.T) {
	now := time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC)
	requester, approver := uuid.New(), uuid.New()
	g := breakglass.Grant{Status: breakglass.StatusPending, RequestedBy: requester, DurationMinutes: 60}
	if _, err := g.Decide(requester, true, now); !errors.Is(err, breakglass.ErrSelfDecision) {
		t.Fatalf("self approval: %v", err)
	}
	ok, err := g.Decide(approver, true, now)
	if err != nil || ok.Status != breakglass.StatusApproved || ok.ExpiresAt == nil || !ok.ExpiresAt.Equal(now.Add(time.Hour)) ||
		ok.DecidedBy == nil || *ok.DecidedBy != approver {
		t.Fatalf("approve: %+v %v", ok, err)
	}
	if _, err := ok.Decide(uuid.New(), false, now); !errors.Is(err, breakglass.ErrNotPending) {
		t.Fatalf("decided twice: %v", err)
	}
	rej, err := g.Decide(approver, false, now)
	if err != nil || rej.Status != breakglass.StatusRejected || rej.ExpiresAt != nil {
		t.Fatalf("reject: %+v %v", rej, err)
	}
	// Active only for the requester, approved, before expires_at.
	if !ok.ReadableBy(requester, now.Add(59*time.Minute)) || ok.ReadableBy(approver, now) || ok.ReadableBy(requester, now.Add(time.Hour)) ||
		g.ReadableBy(requester, now) {
		t.Fatal("readable")
	}
	if !ok.ExpiredAt(now.Add(time.Hour)) || ok.ExpiredAt(now.Add(time.Minute)) {
		t.Fatal("expired")
	}
	rv, err := ok.Revoke(approver, now)
	if err != nil || rv.Status != breakglass.StatusRevoked || rv.ReadableBy(requester, now) {
		t.Fatalf("revoke: %+v %v", rv, err)
	}
	if _, err := rv.Revoke(approver, now); !errors.Is(err, breakglass.ErrNotPending) {
		t.Fatalf("revoke twice: %v", err)
	}
	if _, err := rej.Revoke(approver, now); !errors.Is(err, breakglass.ErrNotPending) {
		t.Fatalf("revoke rejected: %v", err)
	}
}
