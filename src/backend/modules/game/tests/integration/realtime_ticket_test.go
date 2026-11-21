//go:build integration

package integration

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/realtime"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

func valkeyClient(ctx context.Context, t *testing.T) (valkey.Client, func()) {
	t.Helper()
	ctr, addr := testkit.StartValkey(ctx, t)
	c, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{addr}, DisableCache: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Close)
	return c, func() { _ = ctr.Stop(ctx, nil) }
}

func (e *env) ticket(t *testing.T, s student, d device, session string) resp {
	t.Helper()
	return e.do(t, http.MethodPost, "/game/sessions/"+session+"/realtime-tickets", d.token,
		map[string]string{"X-Tenant-Id": s.tenant.String()}, nil)
}

// T009 (F090-AC-01, AC-04): the ticket has 256 random bits, only its SHA-256 is stored in Valkey with the ids of the
// session and realtime_ticket_ttl; url comes from the configuration; ws_ticket_rate → 429 with Retry-After; Valkey
// down → 503 UNAVAILABLE while the other routes keep working.
func TestRealtimeTicket(t *testing.T) {
	ctx := context.Background()
	vk, stop := valkeyClient(ctx, t)
	e := setup(ctx, t, envOpts{tickets: realtime.TicketStore{C: vk, Now: func() time.Time { return time.Now() }},
		realtimeURL: "wss://play.test.example/realtime/v1"})
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	d := e.device(an)
	r := e.start(t, an, d, newKey(), nil)
	if r.status != http.StatusCreated {
		t.Fatalf("start: %d %s", r.status, r.raw)
	}
	session := r.body["game_session_id"].(string)
	r = e.ticket(t, an, d, session)
	if r.status != http.StatusCreated || r.body["url"] != "wss://play.test.example/realtime/v1" || r.body["protocol_version"] != float64(1) {
		t.Fatalf("ticket: %d %s", r.status, r.raw)
	}
	ticket := r.body["ticket"].(string)
	if raw, err := base64.RawURLEncoding.DecodeString(ticket); err != nil || len(raw) < 32 {
		t.Fatalf("ticket entropy: %d bytes, %v", len(raw), err)
	}
	if exp, _ := time.Parse(time.RFC3339Nano, r.body["expires_at"].(string)); exp.Sub(e.clock.Now()) != time.Minute {
		t.Fatalf("expires_at %v", r.body["expires_at"])
	}
	key := rt.TicketKey(rt.HashTicket(ticket))
	raw, err := vk.Do(ctx, vk.B().Get().Key(key).Build()).ToString()
	if err != nil {
		t.Fatalf("stored ticket: %v", err)
	}
	if strings.Contains(raw, ticket) {
		t.Fatal("plain ticket stored")
	}
	var c rt.TicketClaims
	if err := json.Unmarshal([]byte(raw), &c); err != nil || c.UserID != an.id || c.TenantID != an.tenant || c.SchoolID != an.school ||
		c.GameSessionID.String() != session || c.DeviceID != d.id {
		t.Fatalf("claims %+v", c)
	}
	if ttl, _ := vk.Do(ctx, vk.B().Ttl().Key(key).Build()).AsInt64(); ttl <= 0 || ttl > 60 {
		t.Fatalf("ttl %d", ttl)
	}
	// ws_ticket_rate = 10 per minute: 9 more pass, the 11th is 429 with Retry-After.
	for i := 0; i < 9; i++ {
		if r := e.ticket(t, an, d, session); r.status != http.StatusCreated {
			t.Fatalf("ticket %d: %d %s", i+2, r.status, r.raw)
		}
	}
	r = e.ticket(t, an, d, session)
	if r.status != http.StatusTooManyRequests || r.code() != string(problem.RateLimited) || r.header.Get("Retry-After") == "" {
		t.Fatalf("rate limit: %d %s %v", r.status, r.raw, r.header)
	}
	// Valkey down: 503 UNAVAILABLE, the REST heartbeat still answers.
	b := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	db := e.device(b)
	rb := e.start(t, b, db, newKey(), nil)
	stop()
	r = e.ticket(t, b, db, rb.body["game_session_id"].(string))
	if r.status != http.StatusServiceUnavailable || r.code() != string(problem.Unavailable) || r.header.Get("Retry-After") == "" {
		t.Fatalf("valkey down: %d %s", r.status, r.raw)
	}
	if r := e.heartbeat(t, b, db, rb.body["game_session_id"].(string)); r.status != http.StatusNoContent {
		t.Fatalf("REST heartbeat while Valkey is down: %d %s", r.status, r.raw)
	}
}

// T010 (F090-AC-02, AC-03; TC-AUTH-013/014, TC-TEN-001/011): staff tokens → 403 GAME_ACCESS_STUDENT_ONLY with the
// authz.denied audit and no ticket; another student's or another tenant's session → 404; an ended or replaced session
// → 409; X-Tenant-Id of another tenant → 403 TENANT_MISMATCH.
func TestRealtimeTicketAuthz(t *testing.T) {
	ctx := context.Background()
	mem := &memTickets{saved: map[string]rt.TicketClaims{}}
	e := setup(ctx, t, envOpts{tickets: mem})
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	binh := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	dung := e.newStudent(ctx, t, e.tenantB, e.schoolB, "active")
	d := e.device(an)
	session := e.start(t, an, d, newKey(), nil).body["game_session_id"].(string)

	for _, role := range []string{"school_admin", "academic_manager", "tenant_admin"} {
		tok := e.idp.Token(e.staff(e.tenantA, role), "eduworld-portal", nowWall())
		r := e.do(t, http.MethodPost, "/game/sessions/"+session+"/realtime-tickets", tok, map[string]string{"X-Tenant-Id": e.tenantA.String()}, nil)
		if r.status != http.StatusForbidden || r.code() != string(problem.GameAccessStudentOnly) {
			t.Fatalf("%s: %d %s", role, r.status, r.raw)
		}
	}
	for _, role := range []string{"safety_moderator", "system_admin"} {
		tok := e.idp.Token(e.platformStaff(role), "eduworld-portal", nowWall())
		r := e.do(t, http.MethodPost, "/game/sessions/"+session+"/realtime-tickets", tok, map[string]string{"X-Tenant-Id": e.tenantA.String()}, nil)
		if r.status != http.StatusForbidden {
			t.Fatalf("%s: %d %s", role, r.status, r.raw)
		}
	}
	if n := e.audits(ctx, t, "authz.denied"); n < 3 {
		t.Fatalf("authz.denied audits = %d", n)
	}
	// Another student of the same tenant, and a student of tenant B: 404 (no existence leak).
	if r := e.ticket(t, binh, e.device(binh), session); r.status != http.StatusNotFound {
		t.Fatalf("other student: %d %s", r.status, r.raw)
	}
	if r := e.ticket(t, dung, e.device(dung), session); r.status != http.StatusNotFound {
		t.Fatalf("other tenant: %d %s", r.status, r.raw)
	}
	// X-Tenant-Id of tenant B for a student of tenant A.
	r := e.do(t, http.MethodPost, "/game/sessions/"+session+"/realtime-tickets", d.token, map[string]string{"X-Tenant-Id": e.tenantB.String()}, nil)
	if r.status != http.StatusForbidden || r.code() != string(problem.TenantMismatch) {
		t.Fatalf("tenant mismatch: %d %s", r.status, r.raw)
	}
	if len(mem.saved) != 0 {
		t.Fatalf("tickets stored on refusals: %d", len(mem.saved))
	}
	// Replaced (takeover from device B) → 409 for the old session; ended (logout) → 409.
	nd := e.device(an)
	newSession := e.takeover(t, an, nd).body["game_session_id"].(string)
	if r := e.ticket(t, an, d, session); r.status != http.StatusConflict {
		t.Fatalf("replaced session: %d %s", r.status, r.raw)
	}
	if r := e.ticket(t, an, nd, newSession); r.status != http.StatusCreated {
		t.Fatalf("new session: %d %s", r.status, r.raw)
	}
	if r := e.end(t, an, nd, newSession); r.status != http.StatusNoContent {
		t.Fatalf("end: %d", r.status)
	}
	if r := e.ticket(t, an, nd, newSession); r.status != http.StatusConflict {
		t.Fatalf("ended session: %d %s", r.status, r.raw)
	}
	if r := e.ticket(t, an, nd, uuid.NewString()); r.status != http.StatusNotFound {
		t.Fatalf("unknown session: %d %s", r.status, r.raw)
	}
	authztest.Cover(t, "createRealtimeTicket", authztest.CrossTenant, authztest.RoleDenied)
}

// T040 (F090-AC-31, TC-AUTH-092, ADR-0078): inside the effective curfew createRealtimeTicket → 403 CURFEW_ACTIVE with
// reopen_at and no ticket key in Valkey.
func TestRealtimeTicketCurfew(t *testing.T) {
	ctx := context.Background()
	vk, _ := valkeyClient(ctx, t)
	e := setup(ctx, t, envOpts{curfew: "21:30-06:30", tickets: realtime.TicketStore{C: vk}})
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.clock.Set(time.Date(2026, 11, 19, 21, 0, 0, 0, ict))
	d := e.device(an)
	session := e.start(t, an, d, newKey(), nil).body["game_session_id"].(string)
	e.clock.Set(time.Date(2026, 11, 19, 21, 35, 0, 0, ict))
	r := e.ticket(t, an, d, session)
	if r.status != http.StatusForbidden || r.code() != string(problem.CurfewActive) {
		t.Fatalf("curfew: %d %s", r.status, r.raw)
	}
	if at, _ := time.Parse(time.RFC3339, r.body["reopen_at"].(string)); !at.Equal(time.Date(2026, 11, 20, 6, 30, 0, 0, ict)) {
		t.Fatalf("reopen_at %v", r.body["reopen_at"])
	}
	keys, err := vk.Do(ctx, vk.B().Keys().Pattern("ew:game:rt:ticket:*").Build()).AsStrSlice()
	if err != nil || len(keys) != 0 {
		t.Fatalf("ticket keys in curfew: %v %v", keys, err)
	}
}
