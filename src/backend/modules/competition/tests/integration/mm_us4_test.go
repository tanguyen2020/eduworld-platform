//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/persistence"
)

// T032/T039 — F104-AC-18, AC-26: friend and tournament never queue, topic needs topic_id, unknown mode/type and
// subjects not available for the grade → 422 VALIDATION_FAILED naming the field.
func TestMMTypeValidation(t *testing.T) {
	m := setupMM(t)
	a := m.kid(m.tenantA, m.schoolA, m.classA, 3)
	cases := []struct {
		body map[string]any
		want string
	}{
		{map[string]any{"matchmaking_type": "friend"}, "matchmaking_type"},
		{map[string]any{"matchmaking_type": "tournament"}, "matchmaking_type"},
		{map[string]any{"matchmaking_type": "topic"}, "topic_id"},
		{map[string]any{"matchmaking_type": "abc"}, ""},
		{map[string]any{"mode": "abc"}, ""},
		{map[string]any{"mode": "coop_challenge"}, "mode"},
		{map[string]any{"subject_id": "not-a-uuid"}, ""},
	}
	for _, c := range cases {
		r := m.ticket(a, 0, c.body)
		if r.status != http.StatusUnprocessableEntity || r.code() != "VALIDATION_FAILED" {
			t.Fatalf("%v: %d %s", c.body, r.status, r.raw)
		}
		if c.want != "" && !containsField(r, c.want) {
			t.Fatalf("%v: field %s not named: %s", c.body, c.want, r.raw)
		}
	}
	// topic matchmaking with its topic is accepted (unranked)
	if r := m.ticket(a, http.StatusCreated, map[string]any{"matchmaking_type": "topic", "topic_id": uuid.New()}); r.str("matchmaking_type") != "topic" {
		t.Fatalf("topic ticket %s", r.raw)
	}
	// a configuration listing subjects per grade refuses another subject
	m.publishConfig(map[string]any{"subjects": []map[string]any{{"subject_id": m.subject, "grade_levels": []int{3}}}})
	b := m.kid(m.tenantA, m.schoolA, m.classA, 3)
	if r := m.ticket(b, 0, map[string]any{"subject_id": uuid.New()}); r.status != http.StatusUnprocessableEntity || !containsField(r, "subject_id") {
		t.Fatalf("subject not available: %d %s", r.status, r.raw)
	}
	m.ticket(b, http.StatusCreated, nil)
}

func containsField(r presp, field string) bool {
	errs, _ := r.body["errors"].([]any)
	for _, e := range errs {
		if m, ok := e.(map[string]any); ok && (m["field"] == field || m["pointer"] == "/"+field) {
			return true
		}
	}
	return false
}

// publishConfig publishes a new matchmaking configuration version (seed + overrides) in effect now (what F108 does;
// superuser here).
func (m *menv) publishConfig(over map[string]any) {
	m.t.Helper()
	var cfg map[string]any
	if err := jsonUnmarshal(readSeed(m.t), &cfg); err != nil {
		m.t.Fatal(err)
	}
	for k, v := range over {
		cfg[k] = v
	}
	b := jsonMarshal(cfg)
	n := m.count(`SELECT coalesce(max(version_no), 0) FROM competition.game_config_versions`) + 1
	m.exec(`INSERT INTO competition.game_config_versions (config_version_id, created_at, updated_at, config_id, version_no, status, title,
		payload, effective_from, published_at) VALUES ($1, now(), now(), '01936a00-0000-7000-8000-000000000104', $2, 'published', 'test', $3, $4, $4)`,
		uuid.New(), n, string(b), m.clock.Now().Add(-time.Second))
}

// T033 — F104-AC-19, AC-20 (TC-CMP-010): a duel to a friend of the same grade band → pending 60 s, duel.sent; not a
// friend, another band or blocked → 403 SOCIAL_SCOPE_VIOLATION; accept before 60 s → unranked friend match and
// duel.accepted; after 60 s → 410 DUEL_EXPIRED and the job marks it expired; decline → duel.declined.
func TestMMDuel(t *testing.T) {
	m := setupMM(t)
	a := m.kid(m.tenantA, m.schoolA, m.classA, 3)
	b := m.kid(m.tenantA, m.schoolA, m.classA, 2)
	stranger := m.kid(m.tenantA, m.schoolA, m.classA, 3)
	older := m.kid(m.tenantA, m.schoolA, m.classA, 5)
	foe := m.kid(m.tenantA, m.schoolA, m.classA, 3)
	m.friends(a, b)
	m.friends(a, older)
	m.friends(a, foe)
	m.block(foe, a)
	duel := func(from, to kid, want int) presp {
		r := m.call(from, http.MethodPost, "/competition/duels", key(), map[string]any{"opponent_id": to.id, "subject_id": m.subject})
		if r.status != want {
			t.Fatalf("duel %s → %s: %d %s", from.nickname, to.nickname, r.status, r.raw)
		}
		return r
	}
	for _, to := range []kid{stranger, older, foe} {
		if r := duel(a, to, http.StatusForbidden); r.code() != "SOCIAL_SCOPE_VIOLATION" {
			t.Fatalf("duel to %s: %s", to.nickname, r.raw)
		}
	}
	d := duel(a, b, http.StatusCreated)
	if d.str("status") != "pending" {
		t.Fatalf("duel %s", d.raw)
	}
	exp, _ := time.Parse(time.RFC3339Nano, d.str("expires_at"))
	if !exp.Equal(m.clock.Now().Add(60 * time.Second)) {
		t.Fatalf("expires_at %v", exp)
	}
	duel(a, b, http.StatusConflict) // one pending duel per pair
	if len(m.events("duel.sent")) != 1 || len(m.audits("duel.sent")) != 1 {
		t.Fatal("duel.sent event and audit")
	}
	if r := m.call(a, http.MethodPost, "/competition/duels/"+d.str("duel_id")+":accept", key(), nil); r.status != http.StatusNotFound {
		t.Fatalf("challenger accepts %d", r.status)
	}
	m.clock.Add(59 * time.Second)
	acc := m.call(b, http.MethodPost, "/competition/duels/"+d.str("duel_id")+":accept", key(), nil)
	if acc.status != http.StatusOK || acc.str("status") != "accepted" || acc.str("match_id") == "" {
		t.Fatalf("accept %d %s", acc.status, acc.raw)
	}
	if m.count(`SELECT count(*) FROM competition.matches WHERE match_id = $1 AND NOT ranked AND matchmaking_type = 'friend'
		AND duel_id = $2 AND grade_level = 2`, acc.str("match_id"), d.str("duel_id")) != 1 {
		t.Fatal("friend match row")
	}
	if ev := m.events("duel.accepted"); len(ev) != 1 || ev[0]["match_id"] != acc.str("match_id") {
		t.Fatalf("duel.accepted %v", ev)
	}
	if r := m.call(b, http.MethodPost, "/competition/duels/"+d.str("duel_id")+":accept", key(), nil); r.code() != "DUEL_EXPIRED" {
		t.Fatalf("accept twice %s", r.raw)
	}
	late := duel(a, b, http.StatusCreated)
	m.clock.Add(61 * time.Second)
	if r := m.call(b, http.MethodPost, "/competition/duels/"+late.str("duel_id")+":accept", key(), nil); r.status != http.StatusGone ||
		r.code() != "DUEL_EXPIRED" {
		t.Fatalf("accept after 61 s %d %s", r.status, r.raw)
	}
	n, err := m.mm.ExpireDuels(jobCtx(), persistence.Matchmaking{}, 100)
	if err != nil || n != 1 || len(m.events("duel.expired")) != 1 {
		t.Fatalf("duel-expiry %d %v", n, err)
	}
	dec := duel(a, b, http.StatusCreated)
	if r := m.call(b, http.MethodPost, "/competition/duels/"+dec.str("duel_id")+":decline", "", nil); r.status != http.StatusNoContent {
		t.Fatalf("decline %d %s", r.status, r.raw)
	}
	if len(m.events("duel.declined")) != 1 || len(m.audits("duel.declined")) != 1 {
		t.Fatal("duel.declined")
	}
	if seen := m.checkSchemasOf([]string{"duel.sent", "duel.accepted", "duel.declined", "duel.expired", "match.created"}); seen["duel.sent"] != 3 || seen["match.created"] != 1 {
		t.Fatalf("event schemas %v", seen)
	}
}

// T034 — F104-AC-21: a direct proposal of a tournament match (no queue) uses the same ready check and creates the match
// idempotently by proposal_id with its tournament_node_id; F104-AC-22: listCompetitionModes follows the grade and the
// configuration in effect.
func TestMMDirectProposalAndModes(t *testing.T) {
	m := setupMM(t)
	a := m.kid(m.tenantA, m.schoolA, m.classA, 4)
	b := m.kid(m.tenantA, m.schoolA2, uuid.Nil, 4)
	node := uuid.New()
	in := command.DirectInput{TournamentNodeID: node, Mode: "battle_1v1", SubjectID: m.subject, Grade: 4, ScopeKind: "tenant",
		Entries: [2]command.DirectEntry{{Students: []uuid.UUID{a.id}, Tenants: []uuid.UUID{a.tenant}, Schools: []uuid.UUID{a.school}},
			{Students: []uuid.UUID{b.id}, Tenants: []uuid.UUID{b.tenant}, Schools: []uuid.UUID{b.school}}}}
	id, err := m.mm.CreateDirectProposal(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if again, err := m.mm.CreateDirectProposal(context.Background(), in); err != nil || again != id {
		t.Fatalf("second request %v %v", again, err)
	}
	if m.mustProposal(a) != id.String() || m.mustProposal(b) != id.String() {
		t.Fatal("proposal not pushed")
	}
	m.acceptProposal(a, id.String(), http.StatusNoContent)
	m.acceptProposal(b, id.String(), http.StatusNoContent)
	if m.count(`SELECT count(*) FROM competition.matches WHERE match_id = $1 AND tournament_node_id = $2 AND matchmaking_type = 'tournament'
		AND NOT ranked`, id, node) != 1 {
		t.Fatal("tournament match")
	}
	// modes
	g3 := m.kid(m.tenantA, m.schoolA, m.classA, 3)
	modes := func(k kid) []map[string]any {
		r := m.call(k, http.MethodGet, "/competition/modes", "", nil)
		if r.status != http.StatusOK {
			t.Fatalf("modes %d %s", r.status, r.raw)
		}
		return r.data()
	}
	if ms := modes(g3); len(ms) != 1 || ms[0]["mode"] != "battle_1v1" {
		t.Fatalf("grade 3 modes %v", ms)
	}
	if ms := modes(a); len(ms) != 2 {
		t.Fatalf("grade 4 modes %v", ms)
	}
	m.publishConfig(map[string]any{"subjects": []map[string]any{{"subject_id": m.subject, "grade_levels": []int{3}}}})
	ms := modes(g3)
	subjects, _ := ms[0]["subjects"].([]any)
	if len(subjects) != 1 || subjects[0] != m.subject.String() {
		t.Fatalf("subjects of grade 3 %v", ms)
	}
}
