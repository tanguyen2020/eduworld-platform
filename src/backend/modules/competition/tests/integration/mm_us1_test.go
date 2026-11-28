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

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/cache"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/persistence"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/chatid"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// T014 — F104-AC-01: grade, school, tenant and rating come from the server (fields of the body ignored; no class, ADR-0095); the
// ticket is queued in Valkey only and matchmaking.ticket.updated is pushed. TestSeedMatchmakingConfig: the migration
// seed equals seed/matchmaking_config.json.
func TestMMCreateTicketServerSide(t *testing.T) {
	m := setupMM(t)
	an := m.kid(m.tenantA, m.schoolA, 3)
	r := m.ticket(an, http.StatusCreated, map[string]any{"grade_level": 9, "school_id": uuid.New(), "rating": 2900})
	id := r.str("ticket_id")
	if r.str("status") != "queued" || r.str("matchmaking_type") != "skill" || r.str("mode") != "battle_1v1" {
		t.Fatalf("ticket %s", r.raw)
	}
	noPII(t, r)
	tk, err := m.store.Ticket(context.Background(), uuid.MustParse(id))
	if err != nil || tk == nil {
		t.Fatalf("ticket not in Valkey: %v", err)
	}
	if tk.Grade != 3 || tk.Schools[0] != m.schoolA || tk.TenantID != m.tenantA || tk.Rating.Mu != 1500 ||
		tk.ScopeKind != "school" || tk.ScopeID != m.schoolA || tk.GradeKey != "g3" {
		t.Fatalf("server-side fields %+v", tk)
	}
	// ADR-0095: the ticket carries no class.
	fields, err := m.vk.Do(context.Background(), m.vk.B().Hgetall().Key(cache.MMTicketKey(uuid.MustParse(id))).Build()).AsStrMap()
	if err != nil || len(fields) == 0 {
		t.Fatalf("ticket hash: %v", err)
	}
	if _, ok := fields["classes"]; ok {
		t.Fatal("ticket still carries classes")
	}
	if !strings.Contains(tk.Queue, ":s:"+m.schoolA.String()+":g3:") {
		t.Fatalf("queue key %s", tk.Queue)
	}
	if u := m.lastUpdate(an); u == nil || u["status"] != "queued" || u["ticket_id"] != id {
		t.Fatalf("ticket.updated %v", u)
	}
	if m.matches() != 0 || m.count(`SELECT count(*) FROM pg_tables WHERE tablename LIKE '%ticket%'`) != 0 {
		t.Fatal("a ticket must never reach PostgreSQL")
	}
	if g := m.getTicket(an, id); g.status != http.StatusOK || g.str("ticket_id") != id {
		t.Fatalf("get %d %s", g.status, g.raw)
	}
}

// T014 — the seed of the matchmaking configuration in the migration equals the seed file (CONFIG_DEFAULTS values).
func TestSeedMatchmakingConfig(t *testing.T) {
	m := setupMM(t)
	seed := readSeed(t)
	var raw []byte
	// ADR-0095: version 2 (no same_class) is the current published version.
	if err := m.su.QueryRowContext(context.Background(), `SELECT v.payload FROM competition.game_config_versions v
		JOIN competition.game_configs c ON c.config_id = v.config_id AND c.current_version_id = v.config_version_id
		WHERE c.config_type = 'matchmaking-configs' AND v.status = 'published' AND v.version_no = 2`).
		Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var want, got any
	_ = json.Unmarshal(seed, &want)
	_ = json.Unmarshal(raw, &got)
	wb, _ := json.Marshal(want)
	gb, _ := json.Marshal(got)
	if string(wb) != string(gb) {
		t.Fatalf("seed differs:\n%s\n%s", wb, gb)
	}
	// a published version never changes (R11): the trigger refuses an update of its payload
	if _, err := m.su.ExecContext(context.Background(), `UPDATE competition.game_config_versions SET payload = '{}'::jsonb
		WHERE status = 'published'`); err == nil || !strings.Contains(err.Error(), "GAME_CONFIG_VERSION_PUBLISHED") {
		t.Fatalf("published version updated: %v", err)
	}
	// the tenant role cannot write the platform configuration (R20)
	if _, err := m.app.ExecContext(context.Background(), `UPDATE competition.game_configs SET latest_version_no = 9`); err == nil {
		t.Fatal("competition_app wrote competition.game_configs")
	}
}

// T014 — F104-AC-03 (TC-TEN-012): same school by default; a unit tenant with cross-school play uses the tenant scope;
// two tenants never share a queue.
func TestMMScope(t *testing.T) {
	m := setupMM(t)
	a := m.kid(m.tenantA, m.schoolA, 3)
	other := m.kid(m.tenantA, m.schoolA2, 3)
	b := m.kid(m.tenantB, m.schoolB, 3)
	u1 := m.kid(m.tenantU, m.schoolU1, 3)
	u2 := m.kid(m.tenantU, m.schoolU2, 3)
	for _, k := range []kid{a, other, b, u1, u2} {
		m.ticket(k, http.StatusCreated, nil)
	}
	m.clock.Add(5 * time.Minute / 2) // waits past every rating window (rating ignored, scope never)
	m.tick()
	for _, k := range []kid{a, other, b} {
		if m.proposalOf(k) != nil {
			t.Fatalf("%s proposed across schools or tenants", k.nickname)
		}
	}
	p1, p2 := m.proposalOf(u1), m.proposalOf(u2)
	if p1 == nil || p2 == nil || p1["proposal_id"] != p2["proposal_id"] {
		t.Fatalf("unit tenant with cross-school play: %v %v", p1, p2)
	}
}

// T015 — F104-AC-04, AC-05 (TC-CMP-001, SC-002): two grade-3 students of one school, ratings 50 apart, are proposed on
// the first pass; the proposal shows nickname/avatar/level only; both accept → one match with match_id = proposal_id,
// match.created + audit, matchmaking.match.assigned; a retry creates nothing more.
func TestMMProposalAccept(t *testing.T) {
	m := setupMM(t)
	an := m.kid(m.tenantA, m.schoolA, 3)
	binh := m.kid(m.tenantA, m.schoolA, 3)
	far := m.kid(m.tenantA, m.schoolA, 3)
	m.ratings.set(an, 1500)
	m.ratings.set(binh, 1550)
	m.ratings.set(far, 1900)
	ta := m.ticket(an, http.StatusCreated, nil).str("ticket_id")
	m.clock.Add(time.Second)
	m.ticket(binh, http.StatusCreated, nil)
	m.ticket(far, http.StatusCreated, nil)
	m.clock.Add(time.Second)
	if n := m.tick(); n != 1 {
		t.Fatalf("one proposal expected, %d", n)
	}
	if m.proposalOf(far) != nil {
		t.Fatal("rating gap 400 at 2 s proposed")
	}
	pid := m.mustProposal(an)
	if m.mustProposal(binh) != pid {
		t.Fatal("not the same proposal")
	}
	raw, _ := json.Marshal(m.proposalOf(an))
	noPII(t, presp{raw: string(raw)})
	if strings.Contains(string(raw), "1550") || strings.Contains(string(raw), "rating") {
		t.Fatalf("proposal leaks the rating: %s", raw)
	}
	if g := m.getTicket(an, ta); g.str("status") != "proposed" || g.str("proposal_id") != pid {
		t.Fatalf("ticket after proposal %s", g.raw)
	}
	m.acceptProposal(an, pid, http.StatusNoContent)
	if m.matches() != 0 {
		t.Fatal("match before every accept")
	}
	m.acceptProposal(binh, pid, http.StatusNoContent)
	if m.count(`SELECT count(*) FROM competition.matches WHERE match_id = $1 AND status = 'lobby' AND ranked AND NOT is_bot
		AND matchmaking_type = 'skill' AND grade_level = 3 AND scope_kind = 'school'`, pid) != 1 {
		t.Fatal("match row")
	}
	if m.count(`SELECT count(*) FROM competition.match_participants WHERE match_id = $1`, pid) != 2 {
		t.Fatal("participants")
	}
	if ev := m.events("match.created"); len(ev) != 1 || ev[0]["match_id"] != pid {
		t.Fatalf("match.created %v", ev)
	}
	if len(m.audits("match.created")) != 1 {
		t.Fatal("audit match.created")
	}
	if seen := m.checkSchemasOf([]string{"match.created"}); seen["match.created"] != 1 {
		t.Fatalf("match.created schema %v", seen)
	}
	for _, k := range []kid{an, binh} {
		as := m.push.of(k.id, "matchmaking.match.assigned")
		if len(as) != 1 || as[0]["match_id"] != pid {
			t.Fatalf("match.assigned to %s: %v", k.nickname, as)
		}
		// delta F109 2026-11-27: the match chat channel of module chat (F117 id), no team channel in a 1v1.
		if c, _ := as[0]["chat_channel_ids"].(map[string]any); c["match"] != chatid.Match(uuid.MustParse(pid)).String() || c["team"] != nil {
			t.Fatalf("match.assigned chat_channel_ids of %s: %v", k.nickname, as[0])
		}
	}
	if g := m.getTicket(an, ta); g.str("status") != "matched" || g.str("match_id") != pid {
		t.Fatalf("ticket after match %s", g.raw)
	}
	m.acceptProposal(an, pid, http.StatusConflict)
	m.tick()
	if m.matches() != 1 {
		t.Fatal("second match")
	}
	// the students are free again: a new ticket is accepted
	m.ticket(an, http.StatusCreated, nil)
}

// T015 — F104-AC-06: a decline cancels the decliner's ticket (declined_proposal) and requeues the other with its wait
// kept; a timeout requeues who accepted; 3 declines/timeouts in 10 minutes start the cooldown (429 + Retry-After).
func TestMMDeclineTimeoutCooldown(t *testing.T) {
	m := setupMM(t)
	a := m.kid(m.tenantA, m.schoolA, 3)
	b := m.kid(m.tenantA, m.schoolA, 3)
	start := m.clock.Now()
	ta := m.ticket(a, http.StatusCreated, nil).str("ticket_id")
	for round := 1; round <= 3; round++ {
		tb := m.ticket(b, http.StatusCreated, nil).str("ticket_id")
		m.clock.Add(2 * time.Second)
		m.tick()
		pid := m.mustProposal(b)
		if round < 3 {
			m.acceptProposal(a, pid, http.StatusNoContent)
			m.declineProposal(b, pid, http.StatusNoContent)
			if g := m.getTicket(b, tb); g.str("status") != "cancelled" || g.str("cancel_reason") != "declined_proposal" {
				t.Fatalf("decliner ticket %s", g.raw)
			}
		} else { // timeout: a accepts, b stays silent
			m.acceptProposal(a, pid, http.StatusNoContent)
			m.clock.Add(16 * time.Second)
			m.tick()
			if g := m.getTicket(b, tb); g.str("status") != "cancelled" || g.str("cancel_reason") != "timeout" {
				t.Fatalf("silent ticket %s", g.raw)
			}
		}
		g := m.getTicket(a, ta)
		if g.str("status") != "queued" || g.body["requeued"] != true {
			t.Fatalf("round %d: accepted side not requeued: %s", round, g.raw)
		}
		cs := m.push.of(a.id, "matchmaking.proposal.cancelled")
		if len(cs) != round || cs[round-1]["requeued"] != true {
			t.Fatalf("proposal.cancelled to a: %v", cs)
		}
	}
	tk, _ := m.store.Ticket(context.Background(), uuid.MustParse(ta))
	if tk == nil || tk.EnqueuedAt.UnixMilli() != start.UnixMilli() {
		t.Fatalf("requeue must keep enqueued_at %v: %+v", start, tk)
	}
	r := m.ticket(b, http.StatusTooManyRequests, nil)
	if r.code() != "RATE_LIMITED" || r.header.Get("Retry-After") == "" {
		t.Fatalf("cooldown: %d %s", r.status, r.raw)
	}
}

// T016 — F104-AC-07: a second ticket is 409 MM_TICKET_ALREADY_ACTIVE (with the ticket_id of the caller's own active
// ticket — delta F109 2026-11-27), also for concurrent requests with other keys.
func TestMMOneTicketPerStudent(t *testing.T) {
	m := setupMM(t)
	a := m.kid(m.tenantA, m.schoolA, 3)
	first := m.ticket(a, http.StatusCreated, nil).str("ticket_id")
	if r := m.ticket(a, http.StatusConflict, nil); r.code() != "MM_TICKET_ALREADY_ACTIVE" || first == "" || r.str("ticket_id") != first {
		t.Fatalf("second ticket %s (own ticket %s)", r.raw, first)
	}
	b := m.kid(m.tenantA, m.schoolA, 3)
	var wg sync.WaitGroup
	var mu sync.Mutex
	codes := map[int]int{}
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := m.ticket(b, 0, nil)
			mu.Lock()
			codes[r.status]++
			mu.Unlock()
		}()
	}
	wg.Wait()
	if codes[http.StatusCreated] != 1 || codes[http.StatusConflict] != 7 {
		t.Fatalf("concurrent tickets: %v", codes)
	}
}

// T016 — F104-AC-08: blocked students are never in one proposal (H6); a restricted student (abandon_penalty,
// restrict_competition) gets 403 POLICY_DENIED; a restriction arriving while queued removes the ticket at the next
// pass (H5 checked again before proposing).
func TestMMEligibilityAndBlocks(t *testing.T) {
	m := setupMM(t)
	a := m.kid(m.tenantA, m.schoolA, 3)
	b := m.kid(m.tenantA, m.schoolA, 3)
	c := m.kid(m.tenantA, m.schoolA, 3)
	m.block(a, b)
	m.ticket(a, http.StatusCreated, nil)
	m.clock.Add(time.Second)
	m.ticket(b, http.StatusCreated, nil)
	m.clock.Add(time.Minute) // rating ignored: only the block keeps a and b apart
	m.tick()
	if m.proposalOf(a) != nil || m.proposalOf(b) != nil {
		t.Fatal("blocked students proposed together")
	}
	m.ticket(c, http.StatusCreated, nil)
	m.tick()
	pa := m.proposalOf(a)
	if pa == nil || m.mustProposal(c) != pa["proposal_id"] {
		t.Fatal("a must meet c")
	}
	for _, kind := range []string{"abandon_penalty", "restrict_competition"} {
		k := m.kid(m.tenantA, m.schoolA, 3)
		m.restrict(k, kind)
		if r := m.ticket(k, http.StatusForbidden, nil); r.code() != "POLICY_DENIED" {
			t.Fatalf("%s: %s", kind, r.raw)
		}
	}
	late := m.kid(m.tenantA, m.schoolA, 3)
	tl := m.ticket(late, http.StatusCreated, nil).str("ticket_id")
	m.restrict(late, "restrict_competition")
	m.tick()
	if g := m.getTicket(late, tl); g.str("status") != "cancelled" || g.str("cancel_reason") != "system" {
		t.Fatalf("restricted while queued: %s", g.raw)
	}
	if m.proposalOf(late) != nil || m.proposalOf(b) != nil {
		t.Fatal("restricted or blocked student proposed")
	}
}

// T017 — F104-AC-09 (SC-003): two matchmakers run in parallel on 200 tickets: one matchmaker per queue, no ticket in
// two proposals, no student in two matches.
func TestMMParallelMatchmakers(t *testing.T) {
	m := setupMM(t)
	var kids []kid
	for range 200 {
		k := m.kid(m.tenantA, m.schoolA, 3)
		kids = append(kids, k)
		m.ticket(k, http.StatusCreated, nil)
	}
	m.clock.Add(time.Minute)
	second := command.Matchmaking{UoW: m.uow, AfterCommit: entx.AfterCommit, Repo: persistence.Matchmaking{}, Directory: persistence.PartyDirectory{},
		Parties: persistence.Parties{}, Events: m.parties.Events, Audit: m.parties.Audit, Store: m.store, Realtime: m.push, Presence: m.online,
		Metrics: observability.Matchmaking{}, Settings: command.MatchmakingSettings{Owner: "worker-2", LockTTL: 5 * time.Second}, Now: m.clock.Now}
	var wg sync.WaitGroup
	for _, w := range []command.Matchmaking{m.mm, second} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 5 {
				if _, err := w.Tick(jobCtx()); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()
	proposals := map[string]int{}
	for _, k := range kids {
		ps := m.push.of(k.id, "matchmaking.proposal.created")
		if len(ps) != 1 {
			t.Fatalf("%s got %d proposals", k.nickname, len(ps))
		}
		proposals[ps[0]["proposal_id"].(string)]++
	}
	if len(proposals) != 100 {
		t.Fatalf("%d proposals for 200 tickets", len(proposals))
	}
	var awg sync.WaitGroup
	for _, k := range kids {
		awg.Add(1)
		go func() {
			defer awg.Done()
			m.acceptProposal(k, m.mustProposal(k), http.StatusNoContent)
		}()
	}
	awg.Wait()
	if n := m.matches(); n != 100 {
		t.Fatalf("%d matches", n)
	}
	if m.count(`SELECT count(DISTINCT student_id) FROM competition.match_participants`) != 200 ||
		m.count(`SELECT count(*) FROM competition.match_participants`) != 200 {
		t.Fatal("a student in two matches")
	}
}
