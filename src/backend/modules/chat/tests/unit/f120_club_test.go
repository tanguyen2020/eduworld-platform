package unit

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/club"
)

// T041 — F120-AC-17, AC-18, AC-19 (domain): the four club types only (no study_group, ADR-0095); a student creates only
// competition_team / interest_group and only as the school policy allows (disabled → refused, requires_approval →
// pending_approval, auto → active); staff clubs are active at once; members are added to an active club below its cap.
func TestClubDomain(t *testing.T) {
	if club.Type("study_group").Valid() {
		t.Fatal("study_group is a club type")
	}
	for _, c := range []struct {
		staff  bool
		typ    club.Type
		policy club.Creation
		want   string
		err    error
	}{
		{false, club.InterestGroup, club.CreationDisabled, "", club.ErrDisabled},
		{false, club.InterestGroup, club.CreationRequiresApproval, club.StatusPending, nil},
		{false, club.CompetitionTeam, club.CreationAuto, club.StatusActive, nil},
		{false, club.SchoolClub, club.CreationAuto, "", club.ErrStudentType},
		{true, club.SchoolClub, club.CreationDisabled, club.StatusActive, nil},
		{false, club.Type("study_group"), club.CreationAuto, "", club.ErrType},
	} {
		got, err := club.InitialStatus(c.staff, c.typ, c.policy)
		if got != c.want || !errors.Is(err, c.err) {
			t.Errorf("%v %s %s: %s %v", c.staff, c.typ, c.policy, got, err)
		}
	}
	// The boolean school.updated carries today maps to requires_approval / disabled [Proposed].
	if club.ParseCreation(true) != club.CreationRequiresApproval || club.ParseCreation(false) != club.CreationDisabled ||
		club.ParseCreation("auto") != club.CreationAuto || club.ParseCreation("weird") != club.CreationDisabled {
		t.Fatal("ParseCreation")
	}
	c := club.Club{Status: club.StatusPending, MaxMembers: 2, MemberCount: 1}
	if !errors.Is(c.CanAdd(), club.ErrNotApproved) {
		t.Fatal("pending club accepts members")
	}
	c.Status = club.StatusActive
	if c.CanAdd() != nil {
		t.Fatal("active club refuses")
	}
	c.MemberCount = 2
	if !errors.Is(c.CanAdd(), club.ErrFull) {
		t.Fatal("full club accepts")
	}
	c.Status = club.StatusArchived
	if !errors.Is(c.CanAdd(), club.ErrNotActive) {
		t.Fatal("archived club accepts")
	}
	if _, err := club.CheckName(" a "); !errors.Is(err, club.ErrName) {
		t.Fatal("1-char name accepted")
	}
	monday := club.WeekStart(time.Date(2026, 11, 26, 23, 0, 0, 0, time.UTC), time.FixedZone("ICT", 7*3600)) // Fri 06:00 ICT
	if monday.Format("2006-01-02") != "2026-11-23" {
		t.Fatalf("week start %s", monday)
	}
}

// F120-AC-14, AC-12, AC-13, AC-18 (domain): deterministic channel ids — one direct channel per normalised pair (same id
// whoever opens it), one World channel per school (or per tenant), one Area channel per room, one team channel per club
// (is_gameplay false: never purged).
func TestPersistentChannelIDs(t *testing.T) {
	tenant, a, b := uuid.New(), uuid.New(), uuid.New()
	if channel.DirectID(tenant, a, b) != channel.DirectID(tenant, b, a) || channel.DirectID(tenant, a, b) == channel.DirectID(uuid.New(), a, b) {
		t.Fatal("direct id")
	}
	s1, s2 := uuid.New(), uuid.New()
	if channel.WorldID(tenant, &s1) == channel.WorldID(tenant, &s2) || channel.WorldID(tenant, nil) == channel.WorldID(tenant, &s1) {
		t.Fatal("world id")
	}
	if channel.AreaID(tenant, "hub:main_hub:01") == channel.AreaID(tenant, "hub:main_hub:02") {
		t.Fatal("area id")
	}
	clubID := uuid.New()
	ch, err := channel.New(channel.ClubID(tenant, clubID), tenant, &s1, channel.Team, channel.ClubScope(clubID), time.Now())
	if err != nil || ch.IsGameplay || !ch.Persistent() {
		t.Fatalf("club team channel: %+v %v", ch, err)
	}
	dm, _ := channel.New(channel.DirectID(tenant, a, b), tenant, nil, channel.Direct, channel.DirectScope(a, b), time.Now())
	if o, ok := dm.Other(a); !ok || o != b || !dm.DirectOf(b) || dm.DirectOf(uuid.New()) {
		t.Fatal("direct pair")
	}
	if _, err := channel.New(uuid.New(), tenant, nil, channel.World, nil, time.Now()); !errors.Is(err, channel.ErrType) {
		t.Fatal("world through New")
	}
	if w, err := channel.NewImplicit(channel.WorldID(tenant, &s1), tenant, &s1, channel.World, channel.WorldScope(tenant, &s1),
		time.Now()); err != nil || w.IsGameplay {
		t.Fatalf("world: %v", err)
	}
}
