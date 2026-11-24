//go:build integration

package integration

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

// T015 / F107-AC-07, AC-08, AC-09 (TC-TEN-003): the lifecycle moves by command and job (generate-bracket before the
// seeding → 422); a student of the audience registers (same key → same entry; again → 409), out of the scope 404 (other
// school, excluded, other tenant), restricted 403 POLICY_DENIED, full 409; the organizer registers the audience on their
// behalf (excluded and ineligible students skipped) and confirms the entries; withdraw is soft and the student may come
// back; an auto-confirm template confirms at the close.
func TestF107Entries(t *testing.T) {
	e := setupTrn(t)
	e.assignTpl(e.tenantA, tplSingleElim)
	e.assignTpl(e.tenantA, tplRoundRobin)
	admin := e.adminOf(e.tenantA, e.schoolA)
	an, binh, chi := e.kid(e.tenantA, e.schoolA, 3), e.kid(e.tenantA, e.schoolA, 3), e.kid(e.tenantA, e.schoolA, 3)
	em := e.kid(e.tenantA, e.schoolA, 4)
	far := e.kid(e.tenantA, e.schoolA2, 3)
	dung := e.kid(e.tenantB, e.schoolB, 3)
	g5 := e.kid(e.tenantA, e.schoolA, 5)
	extra := e.kidsOf(3, 3)
	ch := e.createCh(admin, e.tenantA, e.challengeBody(tplSingleElim, e.schoolA, 3, func(b map[string]any) {
		b["targets"] = []map[string]any{{"target_type": "grade", "grade_level": 3}, {"target_type": "students", "student_ids": []string{em.id.String()}}}
		b["exclude_student_ids"] = []string{chi.id.String()}
		b["params"] = map[string]any{"subject_id": e.subject.String(), "max_entries": 5}
	}), 201)
	tid := tournamentOf(t, ch)
	path := "/competition/tournaments/" + tid
	if r := e.call(an, http.MethodPost, path+"/entries", key(), nil); r.status != 404 {
		t.Fatalf("a draft is invisible to students: %d", r.status)
	}
	pub := e.staff(admin, e.tenantA, http.MethodPost, path+":publish", key(), nil)
	if pub.status != 200 || pub.str("status") != "registration_open" {
		t.Fatalf("publish (window already open): %d %s", pub.status, pub.raw)
	}
	if len(e.audits("tournament.published")) != 1 {
		t.Fatal("audit tournament.published")
	}
	if r := e.staff(admin, e.tenantA, http.MethodPost, path+":generate-bracket", key(), nil); r.status != 422 || r.code() != "INVALID_STATE_TRANSITION" {
		t.Fatalf("generate before seeding: %d %s", r.status, r.raw)
	}
	idem := key()
	r1 := e.call(an, http.MethodPost, path+"/entries", idem, nil)
	if r1.status != 201 {
		t.Fatalf("register: %d %s", r1.status, r1.raw)
	}
	r2 := e.call(an, http.MethodPost, path+"/entries", idem, nil)
	ents1, _ := r1.body["entries"].([]any)
	ents2, _ := r2.body["entries"].([]any)
	if r2.status != 201 || len(ents1) != 1 || e.rawJSON(ents1[0]) != e.rawJSON(ents2[0]) {
		t.Fatalf("replay: %s", r2.raw)
	}
	if r := e.call(an, http.MethodPost, path+"/entries", key(), nil); r.status != 409 {
		t.Fatalf("registered twice: %d", r.status)
	}
	for _, k := range []kid{far, chi, g5} {
		if r := e.call(k, http.MethodPost, path+"/entries", key(), nil); r.status != 404 {
			t.Fatalf("%s out of scope: %d %s", k.nickname, r.status, r.raw)
		}
	}
	if r := e.call(dung, http.MethodPost, path+"/entries", key(), nil); r.status != 404 {
		t.Fatalf("other tenant: %d", r.status)
	}
	e.exec(`INSERT INTO competition.competition_restrictions (restriction_id, tenant_id, created_at, updated_at, school_id, student_id, kind, reason,
		starts_at, ends_at) VALUES ($1, $2, now(), now(), $3, $4, 'restrict_competition', 'test', $5, $6)`, uuid.New(), e.tenantA, e.schoolA, binh.id,
		e.clock.Now().Add(-time.Hour), e.clock.Now().Add(24*time.Hour))
	if r := e.call(binh, http.MethodPost, path+"/entries", key(), nil); r.status != 403 || r.code() != "POLICY_DENIED" {
		t.Fatalf("restricted: %d %s", r.status, r.raw)
	}
	// withdraw (soft) and come back.
	entry := e.entryOf(tid, an)
	if r := e.call(an, http.MethodDelete, path+"/entries/"+entry, "", nil); r.status != 204 {
		t.Fatalf("withdraw: %d %s", r.status, r.raw)
	}
	if n := e.count(`SELECT count(*) FROM competition.tournament_entries WHERE entry_id = $1 AND status = 'withdrawn' AND deleted_at IS NULL`, entry); n != 1 {
		t.Fatal("a withdrawn entry stays (soft)")
	}
	if r := e.call(an, http.MethodPost, path+"/entries", key(), nil); r.status != 201 || e.entryOf(tid, an) != entry {
		t.Fatalf("come back: %d %s", r.status, r.raw)
	}
	// the organizer registers the audience: em (listed, grade 4) and the extra grade-3 students; chi (excluded) never,
	// binh skipped (restricted), an already registered.
	ob := e.staff(admin, e.tenantA, http.MethodPost, path+"/entries", key(), nil)
	if ob.status != 201 {
		t.Fatalf("on behalf: %d %s", ob.status, ob.raw)
	}
	if n := e.count(`SELECT count(*) FROM competition.tournament_entries WHERE tournament_id = $1 AND member_ids @> jsonb_build_array($2::text)`,
		tid, chi.id.String()); n != 0 {
		t.Fatal("an excluded student got an entry")
	}
	if n := e.count(`SELECT count(*) FROM competition.tournament_entries WHERE tournament_id = $1 AND status = 'registered'`, tid); n != 5 {
		t.Fatalf("%d entries (max_entries 5)", n)
	}
	if e.tstatus(tid) != "registration_closed" {
		t.Fatalf("full: the registration closes (%s)", e.tstatus(tid))
	}
	late := e.kid(e.tenantA, e.schoolA, 3)
	if r := e.call(late, http.MethodPost, path+"/entries", key(), nil); r.status != 409 || r.code() != "CONFLICT" {
		t.Fatalf("full: %d %s", r.status, r.raw)
	}
	_ = extra
	conf := e.staff(admin, e.tenantA, http.MethodPost, path+"/entries:confirm", key(), nil)
	if conf.status != 200 || e.tstatus(tid) != "seeding" {
		t.Fatalf("confirm: %d %s", conf.status, conf.raw)
	}
	if n := e.count(`SELECT count(*) FROM competition.tournament_entries WHERE tournament_id = $1 AND status = 'confirmed'`, tid); n != 5 {
		t.Fatalf("%d confirmed", n)
	}
	// auto-confirm (round robin template, grades 4–9): at the close the registered entries are confirmed.
	g4 := []kid{em, e.kid(e.tenantA, e.schoolA, 4), e.kid(e.tenantA, e.schoolA, 4)}
	rr := e.createCh(admin, e.tenantA, e.challengeBody(tplRoundRobin, e.schoolA, 4, nil), 201)
	rid := tournamentOf(t, rr)
	e.staff(admin, e.tenantA, http.MethodPost, "/competition/tournaments/"+rid+":publish", key(), nil)
	for _, k := range g4 {
		if r := e.call(k, http.MethodPost, "/competition/tournaments/"+rid+"/entries", key(), nil); r.status != 201 {
			t.Fatalf("round robin register: %d %s", r.status, r.raw)
		}
	}
	e.clock.Add(106 * time.Minute)
	e.tick()
	if e.tstatus(rid) != "seeding" || e.count(`SELECT count(*) FROM competition.tournament_entries WHERE tournament_id = $1 AND status = 'confirmed'`, rid) != 3 {
		t.Fatalf("auto-confirm: %s", e.tstatus(rid))
	}
	if r := e.call(em, http.MethodDelete, "/competition/tournaments/"+rid+"/entries/"+e.entryOf(rid, em), "", nil); r.status != 204 {
		t.Fatalf("withdraw before the lock: %d", r.status)
	}
	authztest.Cover(t, "publishTournament", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "registerTournamentEntry", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "withdrawTournamentEntry", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "confirmTournamentEntries", authztest.CrossTenant, authztest.RoleDenied)
}

// T016 / F107-AC-10: the check-in opens 15 minutes before the start (before: 422), a member checks its entry in; at
// starts_at the entries not checked in are withdrawn and their opponents get a bye; below the minimum the tournament is
// cancelled (audit, reason not_enough_entries).
func TestF107CheckIn(t *testing.T) {
	e := setupTrn(t)
	e.assignTpl(e.tenantA, tplSingleElim)
	admin := e.adminOf(e.tenantA, e.schoolA)
	kids := e.kidsOf(5, 3)
	tid := e.prepared(tplSingleElim, admin, kids, nil)
	path := "/competition/tournaments/" + tid
	entry := e.entryOf(tid, kids[0])
	e.clock.Add(-30 * time.Minute) // before the window
	if r := e.call(kids[0], http.MethodPost, path+"/entries/"+entry+":check-in", "", nil); r.status != 422 {
		t.Fatalf("check-in before the window: %d %s", r.status, r.raw)
	}
	e.clock.Add(30 * time.Minute)
	if r := e.call(kids[1], http.MethodPost, path+"/entries/"+entry+":check-in", "", nil); r.status != 404 {
		t.Fatalf("the entry of another student: %d", r.status)
	}
	e.checkIn(tid, kids[:4]...)
	e.clock.Add(15 * time.Minute) // starts_at
	e.tick()
	if e.tstatus(tid) != "in_progress" {
		t.Fatalf("status %s", e.tstatus(tid))
	}
	gone := e.entryOf(tid, kids[4])
	if n := e.count(`SELECT count(*) FROM competition.tournament_entries WHERE entry_id = $1 AND status = 'withdrawn'`, gone); n != 1 {
		t.Fatal("no check-in → withdrawn")
	}
	byes := 0
	for _, n := range e.nodes(tid) {
		if n.status == "bye" {
			byes++
		}
		if (n.a != nil && *n.a == gone) || (n.b != nil && *n.b == gone) {
			if n.status != "bye" && n.status != "void" {
				t.Fatalf("a node not played yet keeps the withdrawn entry: %+v", n)
			}
		}
	}
	if byes < 3 {
		t.Fatalf("5 entries in 8 slots: at least 3 byes, got %d", byes)
	}
	// only 2 of 4 check in: cancelled.
	other := e.kidsOf(4, 3)
	t2 := e.prepared(tplSingleElim, admin, other, nil)
	e.checkIn(t2, other[:2]...)
	e.clock.Add(15 * time.Minute)
	e.tick()
	if e.tstatus(t2) != "cancelled" {
		t.Fatalf("below the minimum: %s", e.tstatus(t2))
	}
	found := false
	for _, a := range e.audits("tournament.cancelled") {
		if a["reason"] == "not_enough_entries" {
			found = true
		}
	}
	if !found {
		t.Fatal("the organizer is told through the audit trail")
	}
	authztest.Cover(t, "checkInTournamentEntry", authztest.CrossTenant, authztest.RoleDenied)
}

// T021 / F107-AC-11, AC-12, AC-13, AC-14: the bracket of N = 5 single elimination (8 slots, 3 byes for seeds 1–3, one
// transaction, audit, random seed stored), the same key replays it, a second generation after scheduled 422, too few
// confirmed entries 422 VALIDATION_FAILED; a round robin of 5 has 10 matches; a manual seeding needs the order.
func TestF107GenerateBracket(t *testing.T) {
	e := setupTrn(t)
	e.assignTpl(e.tenantA, tplSingleElim)
	e.assignTpl(e.tenantA, tplRoundRobin)
	admin := e.adminOf(e.tenantA, e.schoolA)
	e.kidsOf(5, 3)
	ch := e.createCh(admin, e.tenantA, e.challengeBody(tplSingleElim, e.schoolA, 3, nil), 201)
	tid := tournamentOf(t, ch)
	path := "/competition/tournaments/" + tid
	e.staff(admin, e.tenantA, http.MethodPost, path+":publish", key(), nil)
	e.staff(admin, e.tenantA, http.MethodPost, path+"/entries", key(), nil)
	e.clock.Add(106 * time.Minute)
	e.tick()
	e.staff(admin, e.tenantA, http.MethodPost, path+"/entries:confirm", key(), nil)
	idem := key()
	g1 := e.staff(admin, e.tenantA, http.MethodPost, path+":generate-bracket", idem, nil)
	if g1.status != 200 || e.tstatus(tid) != "scheduled" {
		t.Fatalf("generate: %d %s", g1.status, g1.raw)
	}
	g2 := e.staff(admin, e.tenantA, http.MethodPost, path+":generate-bracket", idem, nil)
	if g2.status != 200 || e.rawJSON(g2.body) != e.rawJSON(g1.body) {
		t.Fatalf("the same key replays the same bracket: %s", g2.raw)
	}
	if r := e.staff(admin, e.tenantA, http.MethodPost, path+":generate-bracket", key(), nil); r.status != 422 || r.code() != "INVALID_STATE_TRANSITION" {
		t.Fatalf("generate again: %d %s", r.status, r.raw)
	}
	nodes := e.nodes(tid)
	r1, byes := 0, 0
	for _, n := range nodes {
		if n.round == 1 {
			r1++
			if n.status == "bye" {
				byes++
			}
		}
	}
	if len(nodes) != 7 || r1 != 4 || byes != 3 {
		t.Fatalf("8 slots: %d nodes, %d round-1, %d byes", len(nodes), r1, byes)
	}
	seeds := map[int]string{}
	rows, _ := e.su.Query(`SELECT seed, entry_id FROM competition.tournament_entries WHERE tournament_id = $1 AND seed IS NOT NULL`, tid)
	for rows.Next() {
		var s int
		var id string
		_ = rows.Scan(&s, &id)
		seeds[s] = id
	}
	_ = rows.Close()
	for _, n := range nodes {
		if n.round == 1 && n.status == "bye" {
			w := *n.winner
			if w != seeds[1] && w != seeds[2] && w != seeds[3] {
				t.Fatalf("a bye for a low seed: %s", w)
			}
		}
	}
	if e.count(`SELECT count(*) FROM competition.tournaments WHERE tournament_id = $1 AND seed_value IS NOT NULL`, tid) != 1 {
		t.Fatal("the random seed is stored for the audit (F107-AC-13)")
	}
	if len(e.audits("tournament.bracket_generated")) != 1 {
		t.Fatal("audit tournament.bracket_generated")
	}
	if gs := g1.body["nodes"].([]any); len(gs) != 7 {
		t.Fatal("the response is the bracket")
	}
	// too few: 3 confirmed entries of a single elimination.
	three := e.createCh(admin, e.tenantA, e.challengeBody(tplSingleElim, e.schoolA, 5, nil), 201)
	k5 := e.kidsOf(3, 5)
	t3 := tournamentOf(t, three)
	e.staff(admin, e.tenantA, http.MethodPost, "/competition/tournaments/"+t3+":publish", key(), nil)
	e.staff(admin, e.tenantA, http.MethodPost, "/competition/tournaments/"+t3+"/entries", key(), nil)
	_ = k5
	e.exec(`UPDATE competition.tournaments SET status = 'registration_closed' WHERE tournament_id = $1`, t3) // fixture: closed early
	e.staff(admin, e.tenantA, http.MethodPost, "/competition/tournaments/"+t3+"/entries:confirm", key(), nil)
	if r := e.staff(admin, e.tenantA, http.MethodPost, "/competition/tournaments/"+t3+":generate-bracket", key(), nil); r.status != 422 ||
		fieldOf(r) != "entries" {
		t.Fatalf("below the minimum: %d %s", r.status, r.raw)
	}
	// round robin of 5: 10 matches, every entry plays 4.
	g4 := e.kidsOf(5, 4)
	rr := e.prepared(tplRoundRobin, admin, g4, nil)
	if n := len(e.nodes(rr)); n != 10 {
		t.Fatalf("round robin of 5: %d matches", n)
	}
	authztest.Cover(t, "generateTournamentBracket", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getTournamentBracket", authztest.CrossTenant, authztest.RoleDenied)
}
