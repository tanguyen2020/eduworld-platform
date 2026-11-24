//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/cache"
	hubconfig "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/config"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/persistence"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/consumer"
	competitionhttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// Seeded templates of the F107 migration (seed/tournament_templates.json).
var (
	tplSingleElim = uuid.MustParse("01936a00-0000-7000-8000-000000000171")
	tplRoundRobin = uuid.MustParse("01936a00-0000-7000-8000-000000000172")
	tplQuickQuiz  = uuid.MustParse("01936a00-0000-7000-8000-000000000173")
	tplTeamGroups = uuid.MustParse("01936a00-0000-7000-8000-000000000174")
	tplDoubleElim = uuid.MustParse("01936a00-0000-7000-8000-000000000175")
)

// blockedTerm is the placeholder the fake screener flags (R15: no real sensitive word in tests).
const blockedTerm = "PLACEHOLDER_TERM"

// fakeScreener is a TEST DOUBLE of port.TextScreener (MOCK of the chat facade screenText — R17): flags texts containing
// blockedTerm; down = module chat not answering.
type fakeScreener struct {
	mu    sync.Mutex
	down  bool
	calls int
}

func (s *fakeScreener) Screen(_ context.Context, _ uuid.UUID, _ string, texts []string) ([]int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.down {
		return nil, port.ErrScreeningUnavailable
	}
	var bad []int
	for i, t := range texts {
		if strings.Contains(t, blockedTerm) {
			bad = append(bad, i)
		}
	}
	return bad, nil
}

// tenv is the F107 integration environment: the party environment (migrated eduworld_competition with the F104–F107
// seeds, pkg/httpx chain, fake OIDC, tenants A/B/U), a real Valkey for the direct proposals of matchmaking, the F104 and
// F107 use cases on the same server and the settable clock (2026-11-21 10:00 ICT).
type tenv struct {
	*penv
	vk      valkey.Client
	mm      command.Matchmaking
	trn     command.Tournaments
	q       query.Tournaments
	assign  command.TemplateAssignments
	alist   query.Assignments
	screen  *fakeScreener
	subject uuid.UUID
}

func setupTrn(t *testing.T) *tenv {
	t.Helper()
	e := &tenv{screen: &fakeScreener{}, subject: uuid.New()}
	_, e.vk = valkeyLobby(t)
	store := cache.MatchmakingStore{C: e.vk}
	e.penv = setupParty(t, popts{extra: func(p *penv, h *competitionhttp.Handlers) {
		e.mm = command.Matchmaking{UoW: p.uow, AfterCommit: entx.AfterCommit, Repo: persistence.Matchmaking{}, Directory: persistence.PartyDirectory{},
			Parties: persistence.Parties{}, Events: p.parties.Events, Audit: p.parties.Audit, Store: store, Realtime: p.push, Presence: p.online,
			Metrics: observability.Matchmaking{}, Settings: command.MatchmakingSettings{Owner: "worker-1", LockTTL: 5 * time.Second},
			Now: p.clock.Now, Tracker: command.NewTicketTracker(), Ratings: persistence.RatingReader{}}
		repo := persistence.RankingStore{}
		e.trn = command.Tournaments{UoW: p.uow, AfterCommit: entx.AfterCommit, Repo: persistence.TournamentStore{}, Events: p.parties.Events,
			Audit: p.parties.Audit, Screener: e.screen, Matches: e.mm, Ratings: persistence.RatingReader{}, Rules: repo, Points: repo, Push: p.push,
			Cards: persistence.PartyDirectory{}, Metrics: observability.Tournament{}, Timers: hubconfig.DefaultTournamentTimers, Now: p.clock.Now}
		e.q = query.Tournaments{Read: p.read, Reader: persistence.TournamentRead{}, Now: p.clock.Now}
		e.assign = command.TemplateAssignments{UoW: p.uow, Repo: persistence.TournamentStore{}, Audit: p.parties.Audit, Now: p.clock.Now}
		e.alist = query.Assignments{Read: p.read, Reader: persistence.TournamentStore{}}
		h.Matchmaking = e.mm
		h.MatchmakingQueries = query.Matchmaking{Read: p.read, Reader: persistence.Matchmaking{}, Store: store, Now: p.clock.Now}
		h.Tournaments, h.TournamentQueries = e.trn, e.q
	}})
	for _, s := range []struct{ tenant, school uuid.UUID }{{e.tenantA, e.schoolA}, {e.tenantA, e.schoolA2}, {e.tenantB, e.schoolB},
		{e.tenantU, e.schoolU1}, {e.tenantU, e.schoolU2}} {
		e.exec(`INSERT INTO ref.schools (school_id, tenant_id, status, settings_subset, source_version) VALUES ($1, $2, 'active', '{}', 1)
			ON CONFLICT (school_id) DO NOTHING`, s.school, s.tenant)
	}
	return e
}

// assignTpl assigns a seeded template to tenant (the saga of F045 through the facade is tested in TestF107Assignments).
func (e *tenv) assignTpl(tenant, tpl uuid.UUID) {
	e.t.Helper()
	typ := "challenge_template"
	if tpl == tplTeamGroups || tpl == tplDoubleElim {
		typ = "tournament_template"
	}
	e.exec(`INSERT INTO infra.tenant_resource_assignments (assignment_id, tenant_id, created_at, updated_at, resource_type, resource_id, assigned_by,
		assigned_at) VALUES ($1, $2, now(), now(), $3, $4, $5, now())`, uuid.New(), tenant, typ, tpl, uuid.New())
}

// adminOf is the portal token of the school_admin of school (lms.challenge.write limited to that school).
func (e *tenv) adminOf(tenant, school uuid.UUID) string {
	sub := "kc-sa-" + uuid.NewString()
	e.res.Set(sub, authztest.Staff(tenant, "school_admin", authztest.AtSchool(school)).PrincipalData)
	return e.idp.Token(sub, httpxtest.PortalClient, time.Now())
}

// sysAdmin is the portal token of a System Admin working in tenant (X-Tenant-Id resolved to the target tenant).
func (e *tenv) sysAdmin(tenant uuid.UUID) string {
	sub := "kc-sys-" + uuid.NewString()
	d := authztest.Platform("system_admin").PrincipalData
	t := tenant
	d.TenantID = &t
	e.res.Set(sub, d)
	return e.idp.Token(sub, httpxtest.PortalClient, time.Now())
}

// staffGame is a game-client token of school staff (GAME_ACCESS_STUDENT_ONLY on player routes).
func (e *tenv) staffGame(tenant uuid.UUID) string {
	sub := "kc-sg-" + uuid.NewString()
	e.res.Set(sub, authztest.Staff(tenant, "school_admin").PrincipalData)
	return e.idp.Token(sub, httpxtest.GameClient, time.Now())
}

// staff calls a route with a portal token in tenant.
func (e *tenv) staff(token string, tenant uuid.UUID, method, path, idem string, body any) presp {
	e.t.Helper()
	h := map[string]string{"X-Tenant-Id": tenant.String()}
	if idem != "" {
		h["Idempotency-Key"] = idem
	}
	return e.do(method, path, token, h, body, "")
}

// challengeBody is a ChallengeCreate of school starting in 2 hours (grade 3, the test subject).
func (e *tenv) challengeBody(tpl, school uuid.UUID, grade int, edit func(b map[string]any)) map[string]any {
	b := map[string]any{"template_id": tpl, "name": "Đấu trường Toán khối 3", "school_id": school,
		"targets":   []map[string]any{{"target_type": "grade", "grade_level": grade}},
		"starts_at": e.clock.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339), "params": map[string]any{"subject_id": e.subject.String()}}
	if edit != nil {
		edit(b)
	}
	return b
}

// create creates a challenge as token and fails the test unless the status is want.
func (e *tenv) createCh(token string, tenant uuid.UUID, body map[string]any, want int) presp {
	e.t.Helper()
	r := e.staff(token, tenant, http.MethodPost, "/lms/challenges", key(), body)
	if want != 0 && r.status != want {
		e.t.Fatalf("createChallenge: %d %s", r.status, r.raw)
	}
	return r
}

// tournamentOf returns the tournament id of a created challenge.
func tournamentOf(t *testing.T, r presp) string {
	t.Helper()
	id, _ := r.body["tournament_id"].(string)
	if id == "" {
		t.Fatalf("no tournament: %s", r.raw)
	}
	return id
}

// tick runs one pass of tournament-scheduler at the test clock.
func (e *tenv) tick() int {
	e.t.Helper()
	n, err := e.trn.Tick(jobCtx())
	if err != nil {
		e.t.Fatalf("tournament tick: %v", err)
	}
	return n
}

// mmTick runs the matchmaker (the ready checks past their deadline).
func (e *tenv) mmTick() {
	e.t.Helper()
	if _, err := e.mm.Tick(jobCtx()); err != nil {
		e.t.Fatalf("mm tick: %v", err)
	}
}

// open prepares a tournament of the single elimination template with kids registered and confirmed, the bracket
// generated, ready for the check-in (status scheduled).
func (e *tenv) prepared(tpl uuid.UUID, admin string, kids []kid, edit func(b map[string]any)) string {
	e.t.Helper()
	ch := e.createCh(admin, e.tenantA, e.challengeBody(tpl, e.schoolA, kids[0].grade, edit), http.StatusCreated)
	tid := tournamentOf(e.t, ch)
	if r := e.staff(admin, e.tenantA, http.MethodPost, "/competition/tournaments/"+tid+":publish", key(), nil); r.status != 200 {
		e.t.Fatalf("publish: %d %s", r.status, r.raw)
	}
	ids := make([]string, len(kids))
	for i, k := range kids {
		ids[i] = k.id.String()
	}
	if r := e.staff(admin, e.tenantA, http.MethodPost, "/competition/tournaments/"+tid+"/entries", key(), map[string]any{"student_ids": ids}); r.status != 201 {
		e.t.Fatalf("register on behalf: %d %s", r.status, r.raw)
	}
	e.clock.set(e.clock.Now().Add(106 * time.Minute)) // registration closed (starts_at − 15 min passed)
	e.tick()
	var status string
	_ = e.su.QueryRowContext(context.Background(), `SELECT status FROM competition.tournaments WHERE tournament_id = $1`, tid).Scan(&status)
	if status == "registration_closed" {
		if r := e.staff(admin, e.tenantA, http.MethodPost, "/competition/tournaments/"+tid+"/entries:confirm", key(), nil); r.status != 200 {
			e.t.Fatalf("confirm: %d %s", r.status, r.raw)
		}
	}
	if r := e.staff(admin, e.tenantA, http.MethodPost, "/competition/tournaments/"+tid+":generate-bracket", key(), nil); r.status != 200 {
		e.t.Fatalf("generate: %d %s", r.status, r.raw)
	}
	return tid
}

// checkInAll checks every entry of kids in.
func (e *tenv) checkIn(tid string, kids ...kid) {
	e.t.Helper()
	for _, k := range kids {
		entry := e.entryOf(tid, k)
		if r := e.call(k, http.MethodPost, "/competition/tournaments/"+tid+"/entries/"+entry+":check-in", "", nil); r.status != 200 {
			e.t.Fatalf("check-in %s: %d %s", k.nickname, r.status, r.raw)
		}
	}
}

func (e *tenv) entryOf(tid string, k kid) string {
	e.t.Helper()
	var id string
	err := e.su.QueryRowContext(context.Background(), `SELECT entry_id FROM competition.tournament_entries WHERE tournament_id = $1
		AND member_ids @> jsonb_build_array($2::text) ORDER BY (status = 'withdrawn') LIMIT 1`, tid, k.id.String()).Scan(&id)
	if err != nil {
		e.t.Fatalf("entry of %s: %v", k.nickname, err)
	}
	return id
}

// node is one bracket node row.
type nodeRow struct {
	id, status          string
	a, b, winner, match *string
	round, position     int
	bracket             string
}

func (e *tenv) nodes(tid string) []nodeRow {
	e.t.Helper()
	rows, err := e.su.QueryContext(context.Background(), `SELECT node_id, status, entry_a_id::text, entry_b_id::text, winner_entry_id::text,
		match_id::text, round, position, bracket FROM competition.bracket_nodes WHERE tournament_id = $1 ORDER BY wave, bracket, round, position`, tid)
	if err != nil {
		e.t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var out []nodeRow
	for rows.Next() {
		var n nodeRow
		if err := rows.Scan(&n.id, &n.status, &n.a, &n.b, &n.winner, &n.match, &n.round, &n.position, &n.bracket); err != nil {
			e.t.Fatal(err)
		}
		out = append(out, n)
	}
	return out
}

func (e *tenv) tstatus(tid string) string {
	e.t.Helper()
	var s string
	_ = e.su.QueryRowContext(context.Background(), `SELECT status FROM competition.tournaments WHERE tournament_id = $1`, tid).Scan(&s)
	return s
}

// accept makes every listed kid accept the direct proposal of a node (REST fallback of the ready check).
func (e *tenv) accept(match string, kids ...kid) {
	e.t.Helper()
	for _, k := range kids {
		if r := e.call(k, http.MethodPost, "/competition/matches/"+match+"/proposal:accept", "", nil); r.status != 204 {
			e.t.Fatalf("accept %s: %d %s", k.nickname, r.status, r.raw)
		}
	}
}

// completed delivers match.completed of a node's match (team 0 = entry A wins unless aWins is false).
func (e *tenv) completed(match string, a, b kid, aWins bool) {
	e.t.Helper()
	oa, ob := "win", "loss"
	if !aWins {
		oa, ob = "loss", "win"
	}
	p := map[string]any{"match_id": match, "room_id": match, "mode": "battle_1v1", "matchmaking_type": "tournament", "ranked": false, "rated": false,
		"is_bot": false, "subject_id": e.subject, "grade_level": a.grade, "status": "completed", "end_reason": "finished", "winner_team": 0,
		"rule_version_id": uuid.New(), "tournament_id": uuid.New(), "tournament_node_id": uuid.New(), "ended_at": e.clock.Now().UTC(),
		"results": []map[string]any{{"student_id": a.id, "tenant_id": a.tenant, "school_id": a.school, "team_no": 0, "outcome": oa, "score": 300,
			"correct_count": 3}, {"student_id": b.id, "tenant_id": b.tenant, "school_id": b.school, "team_no": 1, "outcome": ob, "score": 100,
			"correct_count": 1}}}
	e.deliver("match.completed", a.tenant, p)
}

// deliver runs the consumer competition.tournament_match on one event (processed_events first: a replayed event id is a
// duplicate).
func (e *tenv) deliver(typ string, tenant uuid.UUID, p map[string]any) uuid.UUID {
	e.t.Helper()
	b, _ := json.Marshal(p)
	id := uuid.New()
	e.redeliver(typ, tenant, id, b)
	return id
}

func (e *tenv) redeliver(typ string, tenant, id uuid.UUID, payload []byte) {
	e.t.Helper()
	r := &renv{penv: e.penv}
	t := tenant
	r.consume("competition.tournament_match", consumerHandle(e.trn), envelope(typ, id, &t, e.clock.Now(), payload))
}

// kidsOf projects n grade students of school A.
func (e *tenv) kidsOf(n, grade int) []kid {
	out := make([]kid, n)
	for i := range out {
		out[i] = e.kid(e.tenantA, e.schoolA, grade)
	}
	return out
}

func consumerHandle(h command.Tournaments) func(context.Context, outbox.Envelope) error {
	return consumer.TournamentMatch{H: h}.Handle
}

func envelope(typ string, id uuid.UUID, tenant *uuid.UUID, at time.Time, payload []byte) outbox.Envelope {
	return outbox.Envelope{EventID: id, EventType: typ, OccurredAt: at, TenantID: tenant, Payload: payload}
}

// rawJSON renders a decoded payload back to JSON (assertions on ids).
func (e *tenv) rawJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
