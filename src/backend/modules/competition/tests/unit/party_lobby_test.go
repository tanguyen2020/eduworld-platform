package unit

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/party"
)

// lobbyParty is a party of n members joined one minute apart (the first is the leader).
func lobbyParty(t *testing.T, n int, typ string) (*party.Party, []uuid.UUID) {
	t.Helper()
	now := time.Date(2026, 11, 21, 3, 0, 0, 0, time.UTC)
	ids := make([]uuid.UUID, n)
	for i := range ids {
		ids[i] = uuid.New()
	}
	p, err := party.New(uuid.New(), uuid.New(), uuid.New(), ids[0], typ, "g6_7", "Nhóm", party.Settings{}, 8, now)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < n; i++ {
		if err := p.Add(party.Member{StudentID: ids[i], SchoolID: p.SchoolID, JoinedAt: now.Add(time.Duration(i) * time.Minute)}, now); err != nil {
			t.Fatal(err)
		}
	}
	return p, ids
}

// T016 — F101-AC-03, AC-04, AC-07, AC-09: selection and start checks of the lobby.
func TestLobbySelectAndCanStart(t *testing.T) {
	p, ids := lobbyParty(t, 3, party.TypeFriend)
	now := time.Now()
	sel := party.Selection{Type: party.ActivityExplore, Ref: "forest"}
	if err := p.Select(ids[1], sel, p.StateVersion, now); !errors.Is(err, party.ErrNotLeader) {
		t.Fatalf("member selects: %v", err)
	}
	if err := p.Select(ids[0], sel, p.StateVersion-1, now); !errors.Is(err, party.ErrVersion) {
		t.Fatalf("stale version: %v", err)
	}
	if err := p.Select(ids[0], party.Selection{Type: party.ActivityTeamBattle, Ref: "x"}, p.StateVersion, now); !errors.Is(err, party.ErrActivityDisabled) {
		t.Fatalf("team_battle in M4a: %v", err)
	}
	v := p.StateVersion
	if err := p.Select(ids[0], sel, v, now); err != nil || p.StateVersion != v+1 || p.Selected == nil || p.Selected.Ref != "forest" {
		t.Fatalf("select: %v %+v", err, p)
	}
	online := map[uuid.UUID]bool{ids[0]: true, ids[1]: true}
	ready := map[uuid.UUID]bool{ids[0]: true}
	if _, err := p.CanStart(ids[0], p.StateVersion, online, ready, 1, 8); !errors.Is(err, party.ErrNotReady) {
		t.Fatalf("B online not ready: %v", err)
	}
	ready[ids[1]] = true
	parts, err := p.CanStart(ids[0], p.StateVersion, online, ready, 1, 8)
	if err != nil || len(parts.Members) != 2 || len(parts.Offline) != 1 || parts.Offline[0] != ids[2] {
		t.Fatalf("offline C stays out: %v %+v", err, parts)
	}
	if _, err := p.CanStart(ids[0], p.StateVersion, online, ready, 3, 8); !errors.Is(err, party.ErrNotReady) {
		t.Fatalf("below min_members: %v", err)
	}
	if _, err := p.CanStart(ids[1], p.StateVersion, online, ready, 1, 8); !errors.Is(err, party.ErrNotLeader) {
		t.Fatalf("member starts: %v", err)
	}
	p.StartActivity(now)
	if _, err := p.CanStart(ids[0], p.StateVersion, online, ready, 1, 8); !errors.Is(err, party.ErrNotOpen) {
		t.Fatalf("in_activity: %v", err)
	}
	p.BackToLobby(now)
	if p.Status != party.StatusOpen {
		t.Fatal("back to lobby")
	}
	if !party.AllReady(p, online, ready) || party.AllReady(p, nil, ready) {
		t.Fatal("all_ready counts the online members only")
	}
}

// T013 — F101-AC-05, AC-08: explore eligibility by reason code, never the grade of a member (R16).
func TestExploreEligibility(t *testing.T) {
	a, b, c, d := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	subjects := []party.Subject{{ID: a, Band: "g6_7", Active: true}, {ID: b, Band: "g8_9", Active: true},
		{ID: c, Band: "g6_7", Active: true, Restricted: true}, {ID: d, Band: "g6_7", Active: true}}
	area := party.AreaRule{ID: "forest", Bands: []string{"g6_7"}, RequiresUnlock: true, Active: true}
	e := party.ExploreEligibility(area, subjects, map[uuid.UUID]bool{a: true, b: true, c: true}, 8)
	want := map[uuid.UUID]string{a: "", b: party.ReasonGradeBandMismatch, c: party.ReasonSanctioned, d: party.ReasonAreaLocked}
	for _, ch := range e.Members {
		if ch.Reason != want[ch.StudentID] || ch.Eligible != (want[ch.StudentID] == "") {
			t.Errorf("%v: %+v", ch.StudentID, ch)
		}
		if ch.Reason == party.ReasonGradeBandMismatch && ch.Requirement != "grade_bands:g6_7" {
			t.Errorf("requirement is the area's bands, not the student's grade: %q", ch.Requirement)
		}
	}
	if len(e.Ineligible()) != 3 || e.MinMembers != 1 || e.MaxMembers != 8 || !e.AllowContinue {
		t.Fatalf("%+v", e)
	}
}

// T024 — F101-AC-12, AC-13: an activity ends once; the summary keeps shared progress only.
func TestActivityEndAndSummary(t *testing.T) {
	a := party.Activity{Status: party.ActivityRunning}
	now := time.Now()
	err := a.End(party.EndCompleted, map[string]any{"result": "defeated", "score": 90, "mastery": 0.8, "attempt_id": "x",
		"phases_cleared": 2, "contributions": []any{1}}, now)
	if err != nil || a.Status != party.ActivityEnded || a.EndReason != party.EndCompleted {
		t.Fatalf("%v %+v", err, a)
	}
	if len(a.Summary) != 2 || a.Summary["result"] != "defeated" || a.Summary["phases_cleared"] != 2 {
		t.Fatalf("summary: %v", a.Summary)
	}
	if err := a.End(party.EndTimeout, nil, now); !errors.Is(err, party.ErrActivityEnded) {
		t.Fatalf("second end: %v", err)
	}
	for reason, outcome := range map[string]string{party.EndCompleted: "completed", party.EndTimeout: "abandoned", party.EndAllLeft: "abandoned",
		party.EndDisbanded: "aborted", party.EndServerFailure: "failed"} {
		if party.Outcome(reason) != outcome {
			t.Errorf("%s → %s", reason, party.Outcome(reason))
		}
	}
}

var sweepCfg = party.SweepConfig{LeaderGrace: time.Minute, DormantTimeout: 30 * time.Minute, MemberGrace: 2 * time.Minute,
	MinMembersTimeout: 2 * time.Minute}

// T033 — F101-AC-14, AC-15, AC-16: statuses, Ready reset, leader grace, automatic transfer, dormancy.
func TestSweepLeaderGraceAndDormant(t *testing.T) {
	p, ids := lobbyParty(t, 3, party.TypeFriend)
	t0 := time.Date(2026, 11, 21, 4, 0, 0, 0, time.UTC)
	on := func(xs ...uuid.UUID) map[uuid.UUID]string {
		m := map[uuid.UUID]string{}
		for _, x := range xs {
			m[x] = party.MemberOnline
		}
		return m
	}
	// First sweep: everybody online, statuses recorded without pushes.
	out := party.Sweep(party.SweepInput{Party: p, Presence: on(ids...), Now: t0}, sweepCfg)
	if len(out.Changes) != 0 || !out.Dirty {
		t.Fatalf("first sweep: %+v", out)
	}
	l := out.Lobby
	m := l.Members[ids[1]]
	m.Ready = true
	l.Members[ids[1]] = m
	// B disconnects in the lobby: status pushed, Ready reset, still a member.
	out = party.Sweep(party.SweepInput{Party: p, Lobby: l, Presence: on(ids[0], ids[2]), Now: t0.Add(5 * time.Second)}, sweepCfg)
	if len(out.Changes) != 1 || out.Changes[0].Status != party.MemberDisconnected || len(out.ReadyReset) != 1 {
		t.Fatalf("member disconnect: %+v", out)
	}
	// Leader A disconnects: grace starts; back within grace keeps the role.
	l = out.Lobby
	out = party.Sweep(party.SweepInput{Party: p, Lobby: l, Presence: on(ids[2]), Now: t0.Add(10 * time.Second)}, sweepCfg)
	if out.Lobby.LeaderGoneAt == nil || out.TransferTo != uuid.Nil {
		t.Fatalf("grace start: %+v", out)
	}
	back := party.Sweep(party.SweepInput{Party: p, Lobby: out.Lobby, Presence: on(ids[0], ids[2]), Now: t0.Add(50 * time.Second)}, sweepCfg)
	if back.Lobby.LeaderGoneAt != nil || back.TransferTo != uuid.Nil {
		t.Fatalf("leader back in grace: %+v", back)
	}
	// Past the grace: the online member who joined first (C, B is offline) becomes leader.
	late := party.Sweep(party.SweepInput{Party: p, Lobby: out.Lobby, Presence: on(ids[2]), Now: t0.Add(71 * time.Second)}, sweepCfg)
	if late.TransferTo != ids[2] {
		t.Fatalf("transfer to C: %+v", late)
	}
	// Nobody online: dormant (no transfer); the first member back becomes leader; 30 min without anybody disbands.
	dormant := party.Sweep(party.SweepInput{Party: p, Lobby: out.Lobby, Presence: on(), Now: t0.Add(80 * time.Second)}, sweepCfg)
	if dormant.TransferTo != uuid.Nil || dormant.Disband {
		t.Fatalf("dormant: %+v", dormant)
	}
	first := party.Sweep(party.SweepInput{Party: p, Lobby: dormant.Lobby, Presence: on(ids[1]), Now: t0.Add(90 * time.Second)}, sweepCfg)
	if first.TransferTo != ids[1] {
		t.Fatalf("first back is leader: %+v", first)
	}
	gone := party.Sweep(party.SweepInput{Party: p, Lobby: dormant.Lobby, Presence: on(), Now: t0.Add(80*time.Second + 30*time.Minute)},
		sweepCfg)
	if !gone.Disband {
		t.Fatalf("dormant timeout: %+v", gone)
	}
	q, qids := lobbyParty(t, 2, party.TypeQuest)
	if party.Sweep(party.SweepInput{Party: q, Lobby: party.Lobby{LastOnline: t0}, Presence: on(), Now: t0.Add(time.Hour)}, sweepCfg).Disband {
		t.Fatalf("a quest party stays dormant (PARTY_SYSTEM §9): %v", qids)
	}
}

// T033 — F101-AC-17, AC-18: member grace in a session, all left, below min_members past the threshold.
func TestSweepSession(t *testing.T) {
	p, ids := lobbyParty(t, 3, party.TypeFriend)
	p.StartActivity(time.Now())
	t0 := time.Date(2026, 11, 21, 4, 0, 0, 0, time.UTC)
	all := map[uuid.UUID]bool{ids[0]: true, ids[1]: true, ids[2]: true}
	act := &party.Activity{Status: party.ActivityRunning, MinMembers: 2, AllowContinue: true}
	pres := map[uuid.UUID]string{ids[0]: party.MemberInActivity, ids[1]: party.MemberInActivity}
	out := party.Sweep(party.SweepInput{Party: p, Presence: pres, Activity: act, Open: all, Now: t0}, sweepCfg)
	if out.EndActivity != "" || len(out.CloseMember) != 0 || len(out.ReadyReset) != 0 {
		t.Fatalf("C just left: %+v", out)
	}
	in := party.SweepInput{Party: p, Lobby: out.Lobby, Presence: pres, Activity: act, Open: all, Now: t0.Add(2 * time.Minute)}
	out = party.Sweep(in, sweepCfg)
	if len(out.CloseMember) != 1 || out.CloseMember[0] != ids[2] || out.EndActivity != "" {
		t.Fatalf("C past the grace, the others continue: %+v", out)
	}
	strict := *act
	strict.AllowContinue = false
	if o := party.Sweep(party.SweepInput{Party: p, Lobby: in.Lobby, Presence: pres, Activity: &strict, Open: all, Now: in.Now}, sweepCfg); o.EndActivity != party.EndTimeout {
		t.Fatalf("no continue without C: %+v", o)
	}
	two := map[uuid.UUID]bool{ids[0]: true, ids[1]: true}
	one := map[uuid.UUID]string{ids[0]: party.MemberOnline}
	b1 := party.Sweep(party.SweepInput{Party: p, Lobby: out.Lobby, Presence: one, Activity: act, Open: two, Now: t0.Add(3 * time.Minute)}, sweepCfg)
	if b1.Lobby.BelowMinSince == nil || b1.EndActivity != "" {
		t.Fatalf("below min starts: %+v", b1)
	}
	b2 := party.Sweep(party.SweepInput{Party: p, Lobby: b1.Lobby, Presence: one, Activity: act, Open: two, Now: t0.Add(5 * time.Minute)}, sweepCfg)
	if b2.EndActivity != party.EndTimeout {
		t.Fatalf("below min past 120 s: %+v", b2)
	}
	none := party.Sweep(party.SweepInput{Party: p, Lobby: b1.Lobby, Presence: pres, Activity: act, Open: map[uuid.UUID]bool{}, Now: t0}, sweepCfg)
	if none.EndActivity != party.EndAllLeft {
		t.Fatalf("all left: %+v", none)
	}
	if party.MemberStatus("in_match") != party.MemberInActivity || party.MemberStatus("offline") != party.MemberDisconnected ||
		party.MemberStatus("") != party.MemberDisconnected || !party.Connected(party.MemberAway) {
		t.Fatal("presence mapping")
	}
}
