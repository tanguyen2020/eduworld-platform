//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/natscmd"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// wsCall serves one realtime command of the gateway for k (Ew-* headers of its ticket).
func (m *menv) wsCall(k kid, h rt.CommandHandler, payload map[string]any) rt.Reply {
	m.t.Helper()
	b, _ := json.Marshal(payload)
	return h(context.Background(), rt.Caller{TenantID: k.tenant, UserID: k.id, SchoolID: k.school, GameSessionID: uuid.New(),
		DeviceID: uuid.New()}, b)
}

// T023 — F104-AC-10 (TC-MM-001): cancel by REST and by WS matchmaking.ticket.cancel → cancelled (user); a ticket
// waiting longer than mm_ticket_ttl (5 min) expires.
func TestMMCancelAndExpire(t *testing.T) {
	m := setupMM(t)
	a := m.kid(m.tenantA, m.schoolA, m.classA, 3)
	b := m.kid(m.tenantA, m.schoolA2, uuid.Nil, 4)
	c := m.kid(m.tenantB, m.schoolB, uuid.Nil, 5)
	m.setOnline(a, b, c)
	ta := m.ticket(a, http.StatusCreated, nil).str("ticket_id")
	if r := m.call(a, http.MethodDelete, "/competition/matchmaking/tickets/"+ta, "", nil); r.status != http.StatusNoContent {
		t.Fatalf("cancel %d %s", r.status, r.raw)
	}
	if g := m.getTicket(a, ta); g.str("status") != "cancelled" || g.str("cancel_reason") != "user" {
		t.Fatalf("after cancel %s", g.raw)
	}
	if u := m.lastUpdate(a); u["status"] != "cancelled" || u["cancel_reason"] != "user" {
		t.Fatalf("push %v", u)
	}
	if r := m.call(a, http.MethodDelete, "/competition/matchmaking/tickets/"+ta, "", nil); r.status != http.StatusNoContent {
		t.Fatalf("cancel twice %d", r.status)
	}
	tb := m.ticket(b, http.StatusCreated, nil).str("ticket_id")
	if rep := m.wsCall(b, natscmd.TicketCancel(m.mm), map[string]any{"ticket_id": tb}); !rep.OK {
		t.Fatalf("ws cancel %+v", rep)
	}
	if g := m.getTicket(b, tb); g.str("status") != "cancelled" || g.str("cancel_reason") != "user" {
		t.Fatalf("after ws cancel %s", g.raw)
	}
	if rep := m.wsCall(a, natscmd.TicketCancel(m.mm), map[string]any{"ticket_id": tb}); rep.OK || rep.Problem == nil || rep.Problem.Code != "NOT_FOUND" {
		t.Fatalf("ws cancel of someone else %+v", rep)
	}
	tc := m.ticket(c, http.StatusCreated, nil).str("ticket_id")
	m.clock.Add(4 * time.Minute)
	m.sweep()
	if g := m.getTicket(c, tc); g.str("status") != "queued" {
		t.Fatalf("expired too early %s", g.raw)
	}
	m.clock.Add(61 * time.Second)
	m.sweep()
	if g := m.getTicket(c, tc); g.str("status") != "expired" {
		t.Fatalf("not expired after 5 min %s", g.raw)
	}
	if u := m.lastUpdate(c); u["status"] != "expired" {
		t.Fatalf("expiry push %v", u)
	}
	m.ticket(c, http.StatusCreated, nil) // free again
	// a student disconnected longer than 30 s loses its ticket (edge case, cancel_reason system)
	d := m.kid(m.tenantA, m.schoolA, m.classA, 3)
	td := m.ticket(d, http.StatusCreated, nil).str("ticket_id")
	m.sweep() // offline since now
	m.clock.Add(31 * time.Second)
	m.sweep()
	if g := m.getTicket(d, td); g.str("status") != "cancelled" || g.str("cancel_reason") != "system" {
		t.Fatalf("disconnected student kept its ticket %s", g.raw)
	}
}

// T024 — F104-AC-11 (SC-005): Valkey loses every ticket (FLUSHALL): the waiting students are told cancelled (system),
// a proposal of lost tickets creates no match in PostgreSQL.
func TestMMValkeyReset(t *testing.T) {
	m := setupMM(t)
	a := m.kid(m.tenantA, m.schoolA, m.classA, 3)
	b := m.kid(m.tenantA, m.schoolA, m.classA, 3)
	c := m.kid(m.tenantA, m.schoolA, m.classA, 6)
	m.setOnline(a, b, c)
	for _, k := range []kid{a, b, c} {
		m.ticket(k, http.StatusCreated, nil)
	}
	m.tick()
	pid := m.mustProposal(a)
	m.acceptProposal(a, pid, http.StatusNoContent)
	m.sweep() // the worker knows the waiting tickets
	if err := m.vk.Do(context.Background(), m.vk.B().Flushall().Build()).Error(); err != nil {
		t.Fatal(err)
	}
	if r := m.acceptProposal(b, pid, 0); r.status != http.StatusNotFound {
		t.Fatalf("accept of a lost proposal %d %s", r.status, r.raw)
	}
	m.sweep()
	for _, k := range []kid{a, b, c} {
		u := m.lastUpdate(k)
		if u == nil || u["status"] != "cancelled" || u["cancel_reason"] != "system" {
			t.Fatalf("%s not told: %v", k.nickname, u)
		}
	}
	m.tick()
	if m.matches() != 0 {
		t.Fatal("ghost match after the Valkey reset")
	}
	m.ticket(a, http.StatusCreated, nil) // the client creates a new ticket
}

// T025 — F104-AC-12, AC-13: past mm_bot_after (90 s) the solo ticket offers the practice bot; startBotMatch cancels the
// ticket and creates an unranked bot match of the same subject and grade (idempotent); too early → 409; more than 30
// tickets in an hour → 429 + Retry-After.
func TestMMBotAndTicketRate(t *testing.T) {
	m := setupMM(t)
	a := m.kid(m.tenantA, m.schoolA, m.classA, 3)
	m.setOnline(a)
	ta := m.ticket(a, http.StatusCreated, nil).str("ticket_id")
	if r := m.call(a, http.MethodPost, "/competition/matchmaking/tickets/"+ta+":bot-match", key(), nil); r.status != http.StatusConflict {
		t.Fatalf("bot before 90 s: %d %s", r.status, r.raw)
	}
	m.clock.Add(89 * time.Second)
	m.sweep()
	if m.getTicket(a, ta).body["bot_offer_available"] != false {
		t.Fatal("bot offered before 90 s")
	}
	m.clock.Add(2 * time.Second)
	m.sweep()
	if u := m.lastUpdate(a); u["bot_offer_available"] != true {
		t.Fatalf("bot offer push %v", u)
	}
	if m.getTicket(a, ta).body["bot_offer_available"] != true {
		t.Fatal("bot offer in getMatchmakingTicket")
	}
	k := key()
	r := m.call(a, http.MethodPost, "/competition/matchmaking/tickets/"+ta+":bot-match", k, nil)
	if r.status != http.StatusCreated || r.body["is_bot"] != true || r.body["ranked"] != false || r.str("matchmaking_type") != "bot" {
		t.Fatalf("bot match %d %s", r.status, r.raw)
	}
	again := m.call(a, http.MethodPost, "/competition/matchmaking/tickets/"+ta+":bot-match", k, nil)
	if again.str("match_id") != r.str("match_id") || again.header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replay %s", again.raw)
	}
	if m.count(`SELECT count(*) FROM competition.matches WHERE match_id = $1 AND is_bot AND NOT ranked AND grade_level = 3
		AND subject_id = $2`, r.str("match_id"), m.subject) != 1 {
		t.Fatal("bot match row")
	}
	if g := m.getTicket(a, ta); g.str("status") != "cancelled" || g.str("cancel_reason") != "user" {
		t.Fatalf("ticket after bot %s", g.raw)
	}
	// mm_ticket_rate: 30 tickets per hour
	b := m.kid(m.tenantA, m.schoolA, m.classA, 3)
	for i := range 30 {
		id := m.ticket(b, http.StatusCreated, nil).str("ticket_id")
		if c := m.call(b, http.MethodDelete, "/competition/matchmaking/tickets/"+id, "", nil); c.status != http.StatusNoContent {
			t.Fatalf("cancel %d: %d", i, c.status)
		}
		m.clock.Add(time.Minute)
	}
	over := m.ticket(b, http.StatusTooManyRequests, nil)
	if over.code() != "RATE_LIMITED" || over.header.Get("Retry-After") == "" {
		t.Fatalf("31st ticket %s", over.raw)
	}
	m.clock.Add(31 * time.Minute)
	m.ticket(b, http.StatusCreated, nil) // the oldest left the hour window
}
