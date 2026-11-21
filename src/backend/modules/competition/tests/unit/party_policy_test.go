package unit

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/party"
)

func subj(school uuid.UUID, band string, class *uuid.UUID) party.Subject {
	return party.Subject{ID: uuid.New(), SchoolID: school, Band: band, ClassID: class, Active: true}
}

func rule(err error) string {
	var pe party.PolicyError
	if errors.As(err, &pe) {
		return pe.Rule
	}
	if err == nil {
		return ""
	}
	return err.Error()
}

// T022 — F100-AC-08, AC-09: the PolicyEvaluator of an invitation, rule by rule.
func TestInviteCheck(t *testing.T) {
	now := time.Now()
	school, school2 := uuid.New(), uuid.New()
	c1, c2 := uuid.New(), uuid.New()
	leader := subj(school, "g6_7", &c1)
	p, err := party.New(uuid.New(), uuid.New(), school, leader.ID, party.TypeFriend, "g6_7", "Nhóm", party.Settings{}, 4, now)
	if err != nil {
		t.Fatal(err)
	}
	base := func() party.InviteCheck {
		return party.InviteCheck{Party: p, Inviter: leader, Invitee: subj(school, "g6_7", &c2), Friends: true,
			Rules: party.PlatformRules("g6_7", party.MaxMembers)}
	}
	cases := []struct {
		name string
		mod  func(c *party.InviteCheck)
		want string
	}{
		{"ok", func(*party.InviteCheck) {}, ""},
		{"self", func(c *party.InviteCheck) { c.Invitee.ID = c.Inviter.ID }, party.ErrSelf.Error()},
		{"unknown invitee", func(c *party.InviteCheck) { c.Invitee.Active = false }, party.RuleTenant},
		{"blocked", func(c *party.InviteCheck) { c.Blocked = true }, party.ErrInviteNotAllowed.Error()},
		{"sanction", func(c *party.InviteCheck) { c.Invitee.Restricted = true }, party.RuleSanction},
		{"other school", func(c *party.InviteCheck) { c.Invitee.SchoolID = school2 }, party.RuleSchool},
		{"other school, unit cross-school", func(c *party.InviteCheck) { c.Invitee.SchoolID = school2; c.Scope.CrossSchool = true }, ""},
		{"grade band", func(c *party.InviteCheck) { c.Invitee.Band = "g8_9" }, party.RuleGradeBand},
		{"friend only", func(c *party.InviteCheck) { c.Friends = false }, party.RuleFriendOnly},
		{"kick block", func(c *party.InviteCheck) { c.Kicked = true }, party.RuleKickBlock},
		{"class off", func(c *party.InviteCheck) { c.Rules.CrossClass = false }, party.RuleClass},
		{"class off, same class", func(c *party.InviteCheck) { c.Rules.CrossClass = false; c.Invitee.ClassID = &c1 }, ""},
	}
	for _, c := range cases {
		chk := base()
		c.mod(&chk)
		if got := rule(chk.Evaluate()); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
	// invitation_policy: a member may invite only when the policy is members.
	member := subj(school, "g6_7", &c1)
	if err := p.Add(party.Member{StudentID: member.ID, SchoolID: school, JoinedAt: now}, now); err != nil {
		t.Fatal(err)
	}
	chk := base()
	chk.Inviter = member
	if !errors.Is(chk.Evaluate(), party.ErrInviteNotAllowed) {
		t.Fatal("member invited under leader_only")
	}
	p.InvitationPolicy = party.InviteMembers
	if err := chk.Evaluate(); err != nil {
		t.Fatalf("member under members: %v", err)
	}
}

// PARTY_SYSTEM §4, §8 R12 and the tightening of party_policies.
func TestPlatformRulesAndTighten(t *testing.T) {
	g13 := party.PlatformRules("g1_3", 8)
	if g13.CrossClass || !g13.Allows(party.TypeFriend) || !g13.Allows(party.TypeQuest) || g13.Allows(party.TypeStudyGroup) ||
		g13.Allows(party.TypeCustom) || g13.Allows(party.TypeCompetitionTeam) {
		t.Fatalf("g1_3: %+v", g13)
	}
	if g45 := party.PlatformRules("g4_5", 8); !g45.Allows(party.TypeCompetitionTeam) || g45.Allows(party.TypeStudyGroup) {
		t.Fatalf("g4_5: %+v", g45)
	}
	g67 := party.PlatformRules("g6_7", 8)
	if !g67.Allows(party.TypeStudyGroup) || !g67.Allows(party.TypeCustom) || !g67.CrossClass || g67.CrossGradeBand {
		t.Fatalf("g6_7: %+v", g67)
	}
	yes, no, three, nine := true, false, 3, 9
	r := g67.Tighten(party.Override{AllowedTypes: []string{party.TypeQuest, "unknown"}, CrossClass: &no, CrossGradeBand: &yes, MaxMembersCap: &three})
	if len(r.AllowedTypes) != 1 || r.CrossClass || r.CrossGradeBand || r.MaxMembersCap != 3 {
		t.Fatalf("tighten: %+v", r) // a lower level never loosens cross_grade_band
	}
	if r := r.Tighten(party.Override{MaxMembersCap: &nine, CrossClass: &yes}); r.MaxMembersCap != 3 || r.CrossClass {
		t.Fatalf("loosened: %+v", r)
	}
	if err := party.CheckCreate(g13, party.Subject{}, party.TypeCustom, 4); rule(err) == "" {
		t.Fatal("custom allowed at grade 3")
	}
	if err := party.CheckCreate(g67, party.Subject{Restricted: true}, party.TypeFriend, 4); rule(err) != party.RuleSanction {
		t.Fatalf("restricted creator: %v", err)
	}
}

// F100-AC-16: the join rules by visibility.
func TestJoinCheck(t *testing.T) {
	now := time.Now()
	school := uuid.New()
	c1, c2 := uuid.New(), uuid.New()
	leader := subj(school, "g6_7", &c1)
	p, _ := party.New(uuid.New(), uuid.New(), school, leader.ID, party.TypeQuest, "g6_7", "Nhóm", party.Settings{}, 4, now)
	check := func(vis string, joiner party.Subject, friend, invited bool) string {
		p.Visibility = vis
		return rule(party.JoinCheck{Party: p, Leader: leader, Joiner: joiner, FriendOfLeader: friend, Invited: invited,
			Rules: party.PlatformRules("g6_7", 8)}.Evaluate())
	}
	same, other := subj(school, "g6_7", &c1), subj(school, "g6_7", &c2)
	if got := check(party.VisibilityClass, same, false, false); got != "" {
		t.Fatalf("class same: %s", got)
	}
	if got := check(party.VisibilityClass, other, true, false); got != party.RuleVisibility {
		t.Fatalf("class other: %s", got)
	}
	if got := check(party.VisibilityFriends, other, true, false); got != "" {
		t.Fatalf("friends: %s", got)
	}
	if got := check(party.VisibilityFriends, other, false, false); got != party.RuleVisibility {
		t.Fatalf("not a friend: %s", got)
	}
	if got := check(party.VisibilityPrivate, same, true, false); got != party.RulePrivate {
		t.Fatalf("private: %s", got)
	}
	if got := check(party.VisibilityPrivate, same, true, true); got != "" {
		t.Fatalf("private invited: %s", got)
	}
}

// F100-AC-17, AC-19, AC-20, AC-21: membership changes of the aggregate.
func TestPartyAggregate(t *testing.T) {
	t0 := time.Date(2026, 11, 21, 3, 0, 0, 0, time.UTC)
	school := uuid.New()
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	p, err := party.New(uuid.New(), uuid.New(), school, a, party.TypeFriend, "g6_7", "Nhóm", party.Settings{}, 3, t0)
	if err != nil {
		t.Fatal(err)
	}
	_ = p.Add(party.Member{StudentID: b, JoinedAt: t0.Add(time.Second)}, t0)
	_ = p.Add(party.Member{StudentID: c, JoinedAt: t0.Add(2 * time.Second)}, t0)
	if !errors.Is(p.Add(party.Member{StudentID: uuid.New()}, t0), party.ErrFull) || p.StateVersion != 2 {
		t.Fatalf("full / version: %d", p.StateVersion)
	}
	if _, err := p.Update(party.Settings{MaxMembers: ptr(2)}, 8, t0); rule(err) == "" {
		t.Fatal("max below members accepted")
	}
	if _, err := p.Update(party.Settings{MaxMembers: ptr(5)}, 4, t0); rule(err) == "" {
		t.Fatal("max above the cap accepted")
	}
	if changed, err := p.Update(party.Settings{Visibility: ptr(party.VisibilityClass)}, 8, t0); err != nil || len(changed) != 1 || p.StateVersion != 3 {
		t.Fatalf("update: %v %v %d", changed, err, p.StateVersion)
	}
	if changed, _ := p.Update(party.Settings{Visibility: ptr(party.VisibilityClass)}, 8, t0); changed != nil || p.StateVersion != 3 {
		t.Fatal("no-op update changed the version")
	}
	// Leader leaves: c online → c leads although b joined first.
	r, err := p.Remove(a, map[uuid.UUID]bool{c: true}, t0)
	if err != nil || r.NewLeader != c || p.LeaderID != c {
		t.Fatalf("next leader: %+v %v", r, err)
	}
	if err := p.Transfer(b, t0); err != nil || p.LeaderID != b {
		t.Fatalf("transfer: %v", err)
	}
	if !errors.Is(p.Transfer(a, t0), party.ErrTargetNotMember) {
		t.Fatal("transfer to a former member")
	}
	// Nobody online: the member who joined first (c joined after b; b leads, b leaves → c).
	if r, _ := p.Remove(b, nil, t0); r.NewLeader != c || r.Disbanded {
		t.Fatalf("fallback: %+v", r)
	}
	if r, _ := p.Remove(c, nil, t0); !r.Disbanded || p.Status != party.StatusDisbanded || p.DisbandedAt == nil {
		t.Fatalf("all left: %+v", r)
	}
	if _, ok := p.Disband(t0); ok {
		t.Fatal("disband twice")
	}
	if n := party.DefaultName(""); n != "Nhóm mới" {
		t.Fatal(n)
	}
	if _, err := party.CleanName("  "); err == nil {
		t.Fatal("empty name")
	}
}

// F100-AC-12, AC-13: invitation transitions.
func TestInvitation(t *testing.T) {
	now := time.Now()
	inviter, invitee, leader := uuid.New(), uuid.New(), uuid.New()
	i := party.Invitation{InviterID: inviter, InviteeID: invitee, Status: party.InvitationPending, ExpiresAt: now.Add(time.Minute)}
	if !errors.Is(i.Respond(inviter, now), party.ErrNotInvitee) || !errors.Is(i.Respond(invitee, now.Add(time.Minute)), party.ErrExpired) {
		t.Fatal("respond")
	}
	if !errors.Is(i.CanCancel(invitee, leader), party.ErrCannotCancel) || i.CanCancel(leader, leader) != nil || i.CanCancel(inviter, leader) != nil {
		t.Fatal("cancel rights")
	}
	i.Reject(now)
	if i.Status != party.InvitationRejected || i.RespondedAt == nil || !errors.Is(i.Respond(invitee, now), party.ErrNotPending) {
		t.Fatal("reject")
	}
}

func ptr[T any](v T) *T { return &v }
