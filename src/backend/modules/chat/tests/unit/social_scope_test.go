package unit

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/social"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/security"
)

// T009 (F095-AC-02, AC-15, AC-23): same school always; another school only in a unit tenant with
// cross_school_social_enabled; never with a block in either direction; never in a common tenant that did not re-enable
// friends. Different tenants never reach the policy (404 before, see the integration tests).
func TestScopePolicy(t *testing.T) {
	school, other := uuid.New(), uuid.New()
	a := social.Student{ID: uuid.New(), SchoolID: school, Status: "active"}
	same := social.Student{ID: uuid.New(), SchoolID: school, Status: "active"}
	far := social.Student{ID: uuid.New(), SchoolID: other, Status: "active"}
	cases := []struct {
		name    string
		tenant  social.Tenant
		to      social.Student
		blocked bool
		want    error
	}{
		{"same school", social.Tenant{Type: social.TenantSchool}, same, false, nil},
		{"same school blocked", social.Tenant{Type: social.TenantSchool}, same, true, social.ErrBlocked},
		{"other school, school tenant", social.Tenant{Type: social.TenantSchool, CrossSchool: true}, far, false, social.ErrOtherSchool},
		{"other school, unit tenant off", social.Tenant{Type: social.TenantUnit}, far, false, social.ErrOtherSchool},
		{"other school, unit tenant on", social.Tenant{Type: social.TenantUnit, CrossSchool: true}, far, false, nil},
		{"other school, unit on, blocked", social.Tenant{Type: social.TenantUnit, CrossSchool: true}, far, true, social.ErrBlocked},
		{"common tenant default (friends on, owner 2026-11-28)", social.Tenant{Type: social.TenantCommon, CommonFriendsEnabled: true}, same, false, nil},
		{"common tenant switched off by System Admin", social.Tenant{Type: social.TenantCommon}, same, false, social.ErrFriendsDisabled},
		{"common tenant on, blocked", social.Tenant{Type: social.TenantCommon, CommonFriendsEnabled: true}, same, true, social.ErrBlocked},
		{"school tenant ignores the common switch", social.Tenant{Type: social.TenantSchool}, same, false, nil},
	}
	for _, c := range cases {
		if got := (social.ScopePolicy{}).Check(c.tenant, a, c.to, c.blocked); !errors.Is(got, c.want) || (c.want == nil && got != nil) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

// T009 (F095-AC-03): ≤ 10 pending, ≤ 100 friends → FRIEND_LIMIT_REACHED; ≤ 20 per day → RATE_LIMITED with the delay
// until the next platform day.
func TestFriendLimits(t *testing.T) {
	ict := time.FixedZone("ICT", 7*3600)
	l := social.Limits{MaxPending: 10, MaxPerDay: 20, MaxFriends: 100, DayLocation: ict}
	cases := []struct {
		c    social.Counters
		want error
	}{
		{social.Counters{Pending: 9, Today: 19, Friends: 99}, nil},
		{social.Counters{Pending: 10}, social.ErrLimitReached},
		{social.Counters{Friends: 100}, social.ErrLimitReached},
		{social.Counters{Today: 20}, social.ErrDailyLimit},
		{social.Counters{Pending: 10, Today: 20}, social.ErrLimitReached},
	}
	for _, c := range cases {
		if got := l.CheckSend(c.c); !errors.Is(got, c.want) || (c.want == nil && got != nil) {
			t.Errorf("%+v: %v, want %v", c.c, got, c.want)
		}
	}
	if err := l.CheckFriends(100); !errors.Is(err, social.ErrLimitReached) {
		t.Fatalf("friends limit: %v", err)
	}
	now := time.Date(2026, 11, 21, 16, 30, 0, 0, time.UTC) // 23:30 ICT
	if got := l.DayStart(now); !got.Equal(time.Date(2026, 11, 21, 0, 0, 0, 0, ict)) {
		t.Fatalf("day start %v", got)
	}
	if got := l.RetryAfter(now); got != 30*time.Minute {
		t.Fatalf("retry after %v", got)
	}
}

// F095-AC-05, AC-06: a live request is accepted or rejected once; the requester sees a rejected request pending until
// it expires, then expired (SC-003).
func TestFriendRequestLifecycle(t *testing.T) {
	now := time.Date(2026, 11, 21, 3, 0, 0, 0, time.UTC)
	r := social.FriendRequest{Status: social.StatusPending, ExpiresAt: now.Add(time.Hour)}
	if err := r.Reject(now); err != nil {
		t.Fatal(err)
	}
	if r.Status != social.StatusRejected || r.RespondedAt == nil {
		t.Fatalf("rejected %+v", r)
	}
	if got := r.OutgoingStatus(now); got != social.StatusPending {
		t.Fatalf("requester sees %s before expiry", got)
	}
	if got := r.OutgoingStatus(now.Add(2 * time.Hour)); got != social.StatusExpired {
		t.Fatalf("requester sees %s after expiry", got)
	}
	if err := r.Accept(now); !errors.Is(err, social.ErrNotPending) {
		t.Fatalf("accept after reject: %v", err)
	}
	late := social.FriendRequest{Status: social.StatusPending, ExpiresAt: now}
	if err := late.Accept(now); !errors.Is(err, social.ErrNotPending) {
		t.Fatalf("accept an expired request: %v", err)
	}
}

// The pair is stored once in the order PostgreSQL compares uuid (CHECK user_low_id < user_high_id).
func TestPairOrder(t *testing.T) {
	a := uuid.MustParse("ffffffff-0000-7000-8000-000000000000")
	b := uuid.MustParse("00000000-0000-7000-8000-000000000001")
	p := social.NewPair(a, b)
	if p.Low != b || p.High != a || social.NewPair(b, a) != p || p.Other(a) != b || p.Other(b) != a {
		t.Fatalf("pair %+v", p)
	}
}

// F095-AC-21: hidden → offline; exam → generic activity without area; unchanged view → no fan-out.
func TestPresenceVisible(t *testing.T) {
	if v := (social.Presence{Status: "online", Activity: "quest", AreaID: "main_hub", Hidden: true}).Visible(); v.Status != "offline" ||
		v.Activity != "" || v.AreaID != "" {
		t.Fatalf("hidden %+v", v)
	}
	if v := (social.Presence{Status: "in_activity", Activity: "exam", AreaID: "exam_hall"}).Visible(); v.Activity != "learning" || v.AreaID != "" {
		t.Fatalf("exam %+v", v)
	}
	if v := (social.Presence{}).Visible(); v.Status != "offline" {
		t.Fatalf("none %+v", v)
	}
	a := social.Presence{Status: "online", Activity: "idle", Ts: 1}.Visible()
	b := social.Presence{Status: "online", Activity: "idle", Ts: 2}.Visible()
	if !a.SameView(b) {
		t.Fatal("a refresh must not be a visible change")
	}
}

// F095-AC-08: generated codes match ^[A-Z0-9]{6,12}$ (8 characters); malformed codes are refused.
func TestFriendCodes(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		c, err := security.Codes{}.NewCode()
		if err != nil {
			t.Fatal(err)
		}
		if len(c) != social.CodeLength || !social.ValidCode(c) {
			t.Fatalf("code %q", c)
		}
		seen[c] = true
	}
	if len(seen) < 199 {
		t.Fatalf("codes repeat: %d distinct of 200", len(seen))
	}
	for _, bad := range []string{"", "ABC12", "abcdefgh", "ABCDEFGHIJKLM", "ABCD-123"} {
		if social.ValidCode(bad) {
			t.Errorf("%q accepted", bad)
		}
	}
}
