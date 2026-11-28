//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	competitionhttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }

func jsonMarshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// mmPath is the concrete path of a matchmaking operation with id in every path parameter.
func mmPath(op, id string) string {
	p := competitionhttp.MatchmakingOperations[op].Pattern
	for _, param := range []string{"{ticket_id}", "{match_id}", "{duel_id}"} {
		p = strings.ReplaceAll(p, param, id)
	}
	return p
}

func mmBody(op string, subject uuid.UUID) any {
	switch op {
	case "createMatchmakingTicket":
		return map[string]any{"mode": "battle_1v1", "subject_id": subject}
	case "createDuel":
		return map[string]any{"opponent_id": uuid.New(), "subject_id": subject}
	}
	return nil
}

// T037 — F104-AC-23 (TC-AUTH-014, R23): every matchmaking/duel route × staff of the tenant and platform staff → 403
// GAME_ACCESS_STUDENT_ONLY; nothing is written (no ticket, no duel, no match).
func TestMMRoutesStaffDenied(t *testing.T) {
	m := setupMM(t)
	tokens := map[string]string{
		"school_admin":     m.staffToken(m.tenantA, "school_admin"),
		"academic_manager": m.staffToken(m.tenantA, "academic_manager"),
		"tenant_admin":     m.staffToken(m.tenantA, "tenant_admin"),
		"system_admin":     m.platformToken("system_admin"),
	}
	for _, id := range competitionhttp.MatchmakingPlayerOperations {
		o := competitionhttp.MatchmakingOperations[id]
		for role, tok := range tokens {
			r := m.do(o.Method, mmPath(id, uuid.NewString()), tok, map[string]string{"X-Tenant-Id": m.tenantA.String(), "Idempotency-Key": key()},
				mmBody(id, m.subject), "")
			if r.status != http.StatusForbidden || (r.code() != "GAME_ACCESS_STUDENT_ONLY" && (role != "system_admin" || r.code() != "TENANT_MISMATCH")) {
				t.Fatalf("%s by %s: %d %s", id, role, r.status, r.raw)
			}
		}
	}
	keys, err := m.vk.Do(t.Context(), m.vk.B().Keys().Pattern("ew:competition:mm:*").Build()).AsStrSlice()
	if err != nil || len(keys) != 0 || m.matches() != 0 || m.count(`SELECT count(*) FROM competition.duels`) != 0 {
		t.Fatalf("staff wrote something: %v %v", keys, err)
	}
	coverMMRoleDenied(t)
}

// T038 — F104-AC-24 (TC-TEN-001, TC-TEN-011): tickets, proposals and duels of another student or tenant → 404;
// X-Tenant-Id other than the session tenant → 403 TENANT_MISMATCH.
func TestMMTenantIsolation(t *testing.T) {
	m := setupMM(t)
	an := m.kid(m.tenantA, m.schoolA, 3)
	binh := m.kid(m.tenantA, m.schoolA, 3)
	chi := m.kid(m.tenantA, m.schoolA, 3)
	dung := m.kid(m.tenantB, m.schoolB, 3)
	tb := m.ticket(binh, http.StatusCreated, nil).str("ticket_id")
	m.ticket(chi, http.StatusCreated, nil)
	m.tick()
	pid := m.mustProposal(binh)
	tdung := m.ticket(dung, http.StatusCreated, nil).str("ticket_id")
	m.friends(binh, chi)
	m.clock.Add(time.Second)
	duel := m.call(binh, http.MethodPost, "/competition/duels", key(), map[string]any{"opponent_id": an.id, "subject_id": m.subject})
	if duel.status != http.StatusForbidden { // an is not a friend of binh
		t.Fatalf("duel to a non-friend %d", duel.status)
	}
	m.friends(binh, an)
	m.friends(dung, binh)
	d := m.call(binh, http.MethodPost, "/competition/duels", key(), map[string]any{"opponent_id": an.id, "subject_id": m.subject}).str("duel_id")
	for name, r := range map[string]presp{
		"get ticket of another":       m.getTicket(an, tb),
		"get ticket of tenant B":      m.getTicket(an, tdung),
		"cancel ticket of another":    m.call(an, http.MethodDelete, "/competition/matchmaking/tickets/"+tb, "", nil),
		"bot match of another":        m.call(an, http.MethodPost, "/competition/matchmaking/tickets/"+tb+":bot-match", key(), nil),
		"accept proposal of others":   m.acceptProposal(an, pid, 0),
		"decline proposal of others":  m.declineProposal(an, pid, 0),
		"accept duel of another":      m.call(chi, http.MethodPost, "/competition/duels/"+d+":accept", key(), nil),
		"decline duel from tenant B":  m.call(dung, http.MethodPost, "/competition/duels/"+d+":decline", "", nil),
		"get ticket of A from tenant": m.getTicket(dung, tb),
	} {
		if r.status != http.StatusNotFound {
			t.Fatalf("%s: %d %s", name, r.status, r.raw)
		}
	}
	// a duel to a student of another tenant is refused without saying why
	if r := m.call(binh, http.MethodPost, "/competition/duels", key(), map[string]any{"opponent_id": dung.id, "subject_id": m.subject}); r.code() != "SOCIAL_SCOPE_VIOLATION" {
		t.Fatalf("duel across tenants %s", r.raw)
	}
	for _, id := range competitionhttp.MatchmakingPlayerOperations {
		o := competitionhttp.MatchmakingOperations[id]
		r := m.do(o.Method, mmPath(id, tb), an.token, map[string]string{"X-Tenant-Id": m.tenantB.String(), "Idempotency-Key": key()},
			mmBody(id, m.subject), "")
		if r.status != http.StatusForbidden || r.code() != "TENANT_MISMATCH" {
			t.Fatalf("%s with X-Tenant-Id of B: %d %s", id, r.status, r.raw)
		}
	}
	coverMMCrossTenant(t)
	if g := m.getTicket(binh, tb); g.str("status") != "proposed" {
		t.Fatalf("ticket of binh changed by others: %s", g.raw)
	}
}

// T039 — F104-AC-25: Idempotency-Key required on createMatchmakingTicket, createDuel, acceptDuel, startBotMatch (428
// IDEMPOTENCY_KEY_REQUIRED, the code of pkg/httpx — Clarifications 2026-11-21); same key → same result
// (Idempotent-Replayed); same key, other body → 409 IDEMPOTENCY_KEY_REUSED (catalog code); match and duel audited.
func TestMMIdempotency(t *testing.T) {
	m := setupMM(t)
	a := m.kid(m.tenantA, m.schoolA, 3)
	for _, id := range []string{"createMatchmakingTicket", "createDuel", "acceptDuel", "startBotMatch"} {
		o := competitionhttp.MatchmakingOperations[id]
		r := m.do(o.Method, mmPath(id, uuid.NewString()), a.token, map[string]string{"X-Tenant-Id": a.tenant.String()}, mmBody(id, m.subject), "")
		if r.status != http.StatusPreconditionRequired || r.code() != "IDEMPOTENCY_KEY_REQUIRED" {
			t.Fatalf("%s without key: %d %s", id, r.status, r.raw)
		}
	}
	k := key()
	body := map[string]any{"mode": "battle_1v1", "subject_id": m.subject, "matchmaking_type": "skill"}
	first := m.call(a, http.MethodPost, "/competition/matchmaking/tickets", k, body)
	again := m.call(a, http.MethodPost, "/competition/matchmaking/tickets", k, body)
	if first.status != http.StatusCreated || again.status != http.StatusCreated || again.str("ticket_id") != first.str("ticket_id") ||
		again.header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replay %d %s / %d %s", first.status, first.raw, again.status, again.raw)
	}
	body["matchmaking_type"] = "random"
	if r := m.call(a, http.MethodPost, "/competition/matchmaking/tickets", k, body); r.status != http.StatusConflict ||
		r.code() != "IDEMPOTENCY_KEY_REUSED" {
		t.Fatalf("reused key %d %s", r.status, r.raw)
	}
}

// T040 — F104-AC-27 (NFR-PERF-007, TC-PERF-004 local part): 400 students of one grade enter a queue over 100 s (4/s,
// ratings spread over 1000–2000); the matchmaker runs every second and everyone accepts: wait p90 ≤ 45 s, every pass
// < 1 s with real PostgreSQL and Valkey. Metrics carry no school or student label.
func TestMMQueueLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("load")
	}
	m := setupMM(t)
	var kids []kid
	for i := range 400 {
		k := m.kid(m.tenantA, m.schoolA, 3)
		m.ratings.set(k, 1000+float64((i*37)%1000))
		kids = append(kids, k)
	}
	enq := map[uuid.UUID]time.Time{}
	waits := []float64{}
	accepted := map[string]bool{}
	var slowest time.Duration
	for sec := 0; sec < 160; sec++ {
		if sec < 100 {
			for _, k := range kids[sec*4 : sec*4+4] {
				m.ticket(k, http.StatusCreated, nil)
				enq[k.id] = m.clock.Now()
			}
		}
		start := time.Now()
		m.tick()
		if d := time.Since(start); d > slowest {
			slowest = d
		}
		for _, k := range kids {
			p := m.proposalOf(k)
			if p == nil {
				continue
			}
			pid := p["proposal_id"].(string)
			if accepted[pid+k.id.String()] {
				continue
			}
			accepted[pid+k.id.String()] = true
			m.acceptProposal(k, pid, http.StatusNoContent)
			waits = append(waits, m.clock.Now().Sub(enq[k.id]).Seconds())
		}
		m.clock.Add(time.Second)
	}
	if len(waits) < 390 {
		t.Fatalf("only %d of 400 matched", len(waits))
	}
	sort.Float64s(waits)
	p90 := waits[len(waits)*9/10]
	t.Logf("matched %d, wait p50 %.0f s, p90 %.0f s, slowest pass %v", len(waits), waits[len(waits)/2], p90, slowest)
	if p90 > 45 {
		t.Fatalf("wait p90 %.0f s > 45 s (NFR-PERF-007)", p90)
	}
	if slowest >= time.Second {
		t.Fatalf("matchmaker pass %v ≥ 1 s", slowest)
	}
	src, err := os.ReadFile(filepath.Join(testkit.RepoRoot(t), "src", "backend", "modules", "competition", "internal", "infrastructure",
		"observability", "matchmaking.go"))
	if err != nil {
		t.Fatal(err)
	}
	if regexp.MustCompile(`"(school_id|student_id|user_id|tenant_id|school|student)"`).Match(src) {
		t.Fatal("a matchmaking metric is labelled by school or student")
	}
}

// coverMMRoleDenied declares the role_denied case of the 10 F104 operations, verified by TestMMRoutesStaffDenied.
func coverMMRoleDenied(t *testing.T) {
	authztest.Cover(t, "createMatchmakingTicket", authztest.RoleDenied)
	authztest.Cover(t, "getMatchmakingTicket", authztest.RoleDenied)
	authztest.Cover(t, "cancelMatchmakingTicket", authztest.RoleDenied)
	authztest.Cover(t, "startBotMatch", authztest.RoleDenied)
	authztest.Cover(t, "acceptMatchProposal", authztest.RoleDenied)
	authztest.Cover(t, "declineMatchProposal", authztest.RoleDenied)
	authztest.Cover(t, "listCompetitionModes", authztest.RoleDenied)
	authztest.Cover(t, "createDuel", authztest.RoleDenied)
	authztest.Cover(t, "acceptDuel", authztest.RoleDenied)
	authztest.Cover(t, "declineDuel", authztest.RoleDenied)
}

// coverMMCrossTenant declares the cross_tenant case of the 10 F104 operations, verified by TestMMTenantIsolation (ids of
// another student or tenant → 404, X-Tenant-Id of another tenant → 403).
func coverMMCrossTenant(t *testing.T) {
	authztest.Cover(t, "createMatchmakingTicket", authztest.CrossTenant)
	authztest.Cover(t, "getMatchmakingTicket", authztest.CrossTenant)
	authztest.Cover(t, "cancelMatchmakingTicket", authztest.CrossTenant)
	authztest.Cover(t, "startBotMatch", authztest.CrossTenant)
	authztest.Cover(t, "acceptMatchProposal", authztest.CrossTenant)
	authztest.Cover(t, "declineMatchProposal", authztest.CrossTenant)
	authztest.Cover(t, "listCompetitionModes", authztest.CrossTenant)
	authztest.Cover(t, "createDuel", authztest.CrossTenant)
	authztest.Cover(t, "acceptDuel", authztest.CrossTenant)
	authztest.Cover(t, "declineDuel", authztest.CrossTenant)
}
