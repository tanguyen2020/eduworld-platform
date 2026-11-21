package unit

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/party"
)

func subj(school uuid.UUID, grade int) party.Subject {
	return party.Subject{ID: uuid.New(), SchoolID: school, Grade: grade, Band: party.BandOf(grade), Active: true}
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
	leader := subj(school, 6)
	p, err := party.New(uuid.New(), uuid.New(), school, leader.ID, party.TypeFriend, "g6_7", "Nhóm", party.Settings{}, 4, now)
	if err != nil {
		t.Fatal(err)
	}
	base := func() party.InviteCheck {
		return party.InviteCheck{Party: p, Inviter: leader, Invitee: subj(school, 7), Friends: true,
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
		{"grade band", func(c *party.InviteCheck) { c.Invitee.Grade, c.Invitee.Band = 8, "g8_9" }, party.RuleGradeBand},
		{"friend only", func(c *party.InviteCheck) { c.Friends = false }, party.RuleFriendOnly},
		{"kick block", func(c *party.InviteCheck) { c.Kicked = true }, party.RuleKickBlock},
		{"same grade only, other grade", func(c *party.InviteCheck) { c.Rules.SameGradeOnly = true }, party.RuleGrade},
		{"same grade only, same grade", func(c *party.InviteCheck) { c.Rules.SameGradeOnly = true; c.Invitee.Grade = 6 }, ""},
	}
	for _, c := range cases {
		chk := base()
		c.mod(&chk)
		if got := rule(chk.Evaluate()); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
	// invitation_policy: a member may invite only when the policy is members.
	member := subj(school, 6)
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
	if !g13.SameGradeOnly || !g13.Allows(party.TypeFriend) || !g13.Allows(party.TypeQuest) || g13.Allows(party.TypeStudyParty) ||
		g13.Allows(party.TypeCustom) || g13.Allows(party.TypeCompetitionTeam) {
		t.Fatalf("g1_3: %+v", g13)
	}
	if g45 := party.PlatformRules("g4_5", 8); !g45.Allows(party.TypeCompetitionTeam) || g45.Allows(party.TypeStudyParty) || g45.SameGradeOnly {
		t.Fatalf("g4_5: %+v", g45)
	}
	g67 := party.PlatformRules("g6_7", 8)
	if !g67.Allows(party.TypeStudyParty) || !g67.Allows(party.TypeCustom) || g67.SameGradeOnly || g67.CrossGradeBand {
		t.Fatalf("g6_7: %+v", g67)
	}
	yes, no, three, nine := true, false, 3, 9
	r := g67.Tighten(party.Override{AllowedTypes: []string{party.TypeQuest, "unknown"}, SameGradeOnly: &yes, CrossGradeBand: &yes, MaxMembersCap: &three})
	if len(r.AllowedTypes) != 1 || !r.SameGradeOnly || r.CrossGradeBand || r.MaxMembersCap != 3 {
		t.Fatalf("tighten: %+v", r) // a lower level never loosens cross_grade_band
	}
	if r := r.Tighten(party.Override{MaxMembersCap: &nine, SameGradeOnly: &no}); r.MaxMembersCap != 3 || !r.SameGradeOnly {
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
	leader := subj(school, 6)
	p, _ := party.New(uuid.New(), uuid.New(), school, leader.ID, party.TypeQuest, "g6_7", "Nhóm", party.Settings{}, 4, now)
	check := func(vis string, joiner party.Subject, friend, invited bool) string {
		p.Visibility = vis
		return rule(party.JoinCheck{Party: p, Leader: leader, Joiner: joiner, FriendOfLeader: friend, Invited: invited,
			Rules: party.PlatformRules("g6_7", 8)}.Evaluate())
	}
	same, other := subj(school, 6), subj(school, 7)
	// ADR-0095: no class circle — a legacy class visibility joins nobody without an invitation.
	if got := check("class", same, true, false); got != party.RuleVisibility {
		t.Fatalf("class visibility: %s", got)
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
	if _, err := p.Update(party.Settings{Visibility: ptr("class")}, 8, t0); rule(err) == "" {
		t.Fatal("visibility class accepted (ADR-0095)")
	}
	if changed, err := p.Update(party.Settings{Visibility: ptr(party.VisibilityFriends)}, 8, t0); err != nil || len(changed) != 1 || p.StateVersion != 3 {
		t.Fatalf("update: %v %v %d", changed, err, p.StateVersion)
	}
	if changed, _ := p.Update(party.Settings{Visibility: ptr(party.VisibilityFriends)}, 8, t0); changed != nil || p.StateVersion != 3 {
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

// ADR-0095 D2 (F100-AC-09, PARTY_SYSTEM §8 R12): a grade 1–3 party takes members of the exact grade of the inviter or
// leader; another school only inside the tenant when cross-school social is on (ADR-0054); another tenant is never
// projected (RuleTenant); a friend party still takes friends only.
func TestGrades13SameGrade(t *testing.T) {
	now := time.Now()
	school, school2 := uuid.New(), uuid.New()
	leader := subj(school, 2)
	p, err := party.New(uuid.New(), uuid.New(), school, leader.ID, party.TypeFriend, "g1_3", "Nhóm", party.Settings{}, 4, now)
	if err != nil {
		t.Fatal(err)
	}
	invite := func(invitee party.Subject, friends, crossSchool bool) string {
		return rule(party.InviteCheck{Party: p, Inviter: leader, Invitee: invitee, Friends: friends,
			Rules: party.PlatformRules("g1_3", party.MaxMembers), Scope: party.Scope{CrossSchool: crossSchool}}.Evaluate())
	}
	otherTenant := subj(school, 2)
	otherTenant.Active = false // unknown in the session tenant: the Subject lookup is tenant-scoped
	cases := []struct {
		name        string
		invitee     party.Subject
		friends     bool
		crossSchool bool
		want        string
	}{
		{"same grade, same school", subj(school, 2), true, false, ""},
		{"other grade of the band", subj(school, 3), true, false, party.RuleGrade},
		{"other grade, cross-school on", subj(school2, 1), true, true, party.RuleGrade},
		{"same grade, other school, cross-school off", subj(school2, 2), true, false, party.RuleSchool},
		{"same grade, other school, cross-school on", subj(school2, 2), true, true, ""},
		{"other tenant", otherTenant, true, true, party.RuleTenant},
		{"same grade, not a friend", subj(school, 2), false, false, party.RuleFriendOnly},
		{"grade unknown", subj(school, 0), true, false, party.RuleGradeBand},
	}
	for _, c := range cases {
		if got := invite(c.invitee, c.friends, c.crossSchool); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
	// joinParty of a friends party checks the grade of the leader the same way.
	p.Visibility = party.VisibilityFriends
	join := func(joiner party.Subject) string {
		return rule(party.JoinCheck{Party: p, Leader: leader, Joiner: joiner, FriendOfLeader: true,
			Rules: party.PlatformRules("g1_3", party.MaxMembers), Scope: party.Scope{CrossSchool: true}}.Evaluate())
	}
	if got := join(subj(school2, 2)); got != "" {
		t.Fatalf("join same grade other school: %s", got)
	}
	if got := join(subj(school, 1)); got != party.RuleGrade {
		t.Fatalf("join other grade: %s", got)
	}
	// study_party is not allowed below grade 6 (D7 keeps the behaviour of study_group).
	if err := party.CheckCreate(party.PlatformRules("g1_3", 8), leader, party.TypeStudyParty, 4); rule(err) == "" {
		t.Fatal("study_party allowed at grade 2")
	}
	if party.ValidType("study_group") {
		t.Fatal("study_group still a valid party type")
	}
}

// ADR-0095: a row written before the backfill reads as study_party / private.
func TestPartyLegacyValues(t *testing.T) {
	p := party.Party{Type: "study_group", Visibility: "class"}
	p.Legacy()
	if p.Type != party.TypeStudyParty || p.Visibility != party.VisibilityPrivate {
		t.Fatalf("legacy: %+v", p)
	}
}
