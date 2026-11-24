//go:build integration

package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

func glassBypass() float64 {
	return testutil.ToFloat64(tenancy.BypassCounter().WithLabelValues("chat", string(tenancy.ReasonBreakGlass)))
}

// glassRequest posts createBreakGlassRequest of m into tenant (System Admin writes always name the target tenant, R20).
func (s *safetyEnv) glassRequest(m *moderator, tenant uuid.UUID, scopeType string, scope uuid.UUID, from, to time.Time, minutes int, reason string) resp {
	s.t.Helper()
	return m.post(tenant, "/admin/break-glass/grants", map[string]any{"scope_type": scopeType, "scope_id": scope,
		"window_from": from.UTC().Format(time.RFC3339), "window_to": to.UTC().Format(time.RFC3339), "duration_minutes": minutes,
		"reason": reason}, 0)
}

func (s *safetyEnv) glassMessages(m *moderator, grant string, q string) resp {
	s.t.Helper()
	return m.get("/admin/break-glass/grants/" + grant + "/messages" + q)
}

func messageIDs(r resp) []string {
	var out []string
	for _, d := range r.data() {
		out = append(out, d["message_id"].(string))
	}
	return out
}

const glassReason = "Yeu cau phap ly so 42 ve hoc sinh nay"

// T038, T039, T042 / F124-AC-17, AC-18, AC-21 (TC-TEN-004, TC-TEN-024): a System Admin requests break-glass for a target
// tenant (pending, audit, break_glass.requested platform event without the reason); safety_moderator, tenant roles and
// requests outside the limits are refused; the requester cannot decide its own request; a second System Admin approves
// (If-Match) or rejects; a decided request is 409 BREAK_GLASS_NOT_PENDING.
func TestBreakGlassRequestAndDecision(t *testing.T) {
	authztest.Cover(t, "createBreakGlassRequest", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "approveBreakGlassRequest", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "rejectBreakGlassRequest", authztest.CrossTenant, authztest.RoleDenied)
	s := setupSafetyWith(t, true)
	sa, sa2, sm := s.moderator("system_admin", false), s.moderator("system_admin", false), s.moderator("safety_moderator", true)
	hs := s.student(s.tenantA, s.schoolA)
	now := s.clock.Now()
	from, to := now.Add(-7*24*time.Hour), now
	// AC-21: safety_moderator (even team lead) and tenant roles → 403; limits → 422.
	if r := s.glassRequest(sm, s.tenantA, "user", hs.id, from, to, 60, glassReason); r.status != http.StatusForbidden || r.code() != "PERMISSION_DENIED" {
		t.Fatalf("safety_moderator: %d %s", r.status, r.raw)
	}
	for _, tok := range []string{s.staff(s.tenantA, "tenant_admin"), s.staff(s.tenantA, "school_admin")} {
		r := s.do(http.MethodPost, "/admin/break-glass/grants", tok, map[string]string{"X-Tenant-Id": s.tenantA.String(), "Idempotency-Key": newKey()},
			map[string]any{"scope_type": "user", "scope_id": hs.id, "window_from": from.Format(time.RFC3339), "window_to": to.Format(time.RFC3339),
				"duration_minutes": 60, "reason": glassReason})
		if r.status != http.StatusForbidden {
			t.Fatalf("tenant role: %d %s", r.status, r.raw)
		}
	}
	for name, r := range map[string]resp{
		"reason 1001":    s.glassRequest(sa, s.tenantA, "user", hs.id, from, to, 60, strings.Repeat("r", 1001)),
		"reason 19":      s.glassRequest(sa, s.tenantA, "user", hs.id, from, to, 60, strings.Repeat("r", 19)),
		"duration 241":   s.glassRequest(sa, s.tenantA, "user", hs.id, from, to, 241, glassReason),
		"window 31 d":    s.glassRequest(sa, s.tenantA, "user", hs.id, now.Add(-31*24*time.Hour), now, 60, glassReason),
		"window to≤from": s.glassRequest(sa, s.tenantA, "user", hs.id, now, now, 60, glassReason),
	} {
		if r.status != http.StatusUnprocessableEntity || r.code() != "VALIDATION_FAILED" {
			t.Errorf("%s: %d %s", name, r.status, r.raw)
		}
	}
	// AC-17: pending, audit, event to module identity (ids only, never the reason).
	r := s.glassRequest(sa, s.tenantA, "user", hs.id, from, to, 60, glassReason)
	if r.status != http.StatusCreated || r.body["status"] != "pending" || r.body["requested_by"] != sa.id.String() ||
		r.body["tenant_id"] != s.tenantA.String() {
		t.Fatalf("request: %d %s", r.status, r.raw)
	}
	grant := r.body["grant_id"].(string)
	evs := s.envelopes("break_glass.requested")
	if len(evs) != 1 || evs[0].TenantID != nil || strings.Contains(string(evs[0].Payload), "phap ly") {
		t.Fatalf("break_glass.requested: %+v", evs)
	}
	if n := s.checkEvents("break_glass.requested"); n["break_glass.requested"] != 1 {
		t.Fatalf("schema: %v", n)
	}
	if len(s.auditMeta("break_glass.requested")) != 1 {
		t.Fatal("audit break_glass.requested")
	}
	// AC-18: self approval → 403 (audited denial); stale If-Match → 412; another System Admin approves; then 409.
	if r := sa.post(s.tenantA, "/admin/break-glass/grants/"+grant+":approve", map[string]any{}, 1); r.status != http.StatusForbidden ||
		r.code() != "PERMISSION_DENIED" {
		t.Fatalf("self approval: %d %s", r.status, r.raw)
	}
	if r := sm.post(s.tenantA, "/admin/break-glass/grants/"+grant+":approve", map[string]any{}, 1); r.status != http.StatusForbidden {
		t.Fatalf("safety_moderator approves: %d %s", r.status, r.raw)
	}
	if r := sa2.post(s.tenantB, "/admin/break-glass/grants/"+grant+":approve", map[string]any{}, 1); r.status != http.StatusNotFound {
		t.Fatalf("X-Tenant-Id of another tenant: %d %s", r.status, r.raw)
	}
	if r := sa2.post(s.tenantA, "/admin/break-glass/grants/"+grant+":approve", map[string]any{}, 7); r.status != http.StatusPreconditionFailed {
		t.Fatalf("stale If-Match: %d %s", r.status, r.raw)
	}
	a := sa2.post(s.tenantA, "/admin/break-glass/grants/"+grant+":approve", map[string]any{"decision_note": "Da kiem tra van ban"}, 1)
	if a.status != http.StatusOK || a.body["status"] != "approved" || a.body["decided_by"] != sa2.id.String() || a.body["expires_at"] == nil {
		t.Fatalf("approve: %d %s", a.status, a.raw)
	}
	if exp, _ := time.Parse(time.RFC3339Nano, a.body["expires_at"].(string)); !exp.Equal(now.Add(time.Hour)) {
		t.Fatalf("expires_at: %v", a.body["expires_at"])
	}
	if r := sa2.post(s.tenantA, "/admin/break-glass/grants/"+grant+":reject", map[string]any{}, 2); r.status != http.StatusConflict ||
		r.code() != "BREAK_GLASS_NOT_PENDING" {
		t.Fatalf("decided twice: %d %s", r.status, r.raw)
	}
	other := s.glassRequest(sa, s.tenantB, "channel", uuid.New(), from, to, 30, glassReason).body["grant_id"].(string)
	if r := sa2.post(s.tenantB, "/admin/break-glass/grants/"+other+":reject", map[string]any{"decision_note": "Khong du can cu"}, 1); r.status != http.StatusOK ||
		r.body["status"] != "rejected" || r.body["expires_at"] != nil {
		t.Fatalf("reject: %d %s", r.status, r.raw)
	}
	if len(s.auditMeta("break_glass.approved")) != 1 || len(s.auditMeta("break_glass.rejected")) != 1 {
		t.Fatal("audit approved/rejected")
	}
	if s.count(`SELECT count(*) FROM chat.break_glass_grants WHERE decided_by = requested_by`) != 0 {
		t.Fatal("two-person rule")
	}
}

// T040, T041, T043 / F124-AC-19, AC-20, AC-22 (TC-TEN-023, TC-CHAT-022): only the requester of an approved grant reads
// the messages of its scope (user: messages the student sent; channel: every message of the channel) in the target tenant
// and window, through tenancy.Bypass(break_glass) with its audit and metric; every page is audited and counted; another
// System Admin, an expired or revoked grant → 403 BREAK_GLASS_NOT_ACTIVE; the expirer ends grants; the list shows the
// grants with the messages viewed; no content in audit or events; purged gameplay messages are not restored.
func TestBreakGlassReadScopeExpiryRevoke(t *testing.T) {
	authztest.Cover(t, "listBreakGlassMessages", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listBreakGlassGrants", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "revokeBreakGlassGrant", authztest.CrossTenant, authztest.RoleDenied)
	s := setupSafetyWith(t, true)
	sa, sa2, sm := s.moderator("system_admin", false), s.moderator("system_admin", false), s.moderator("safety_moderator", true)
	hs, ban := s.student(s.tenantA, s.schoolA), s.student(s.tenantA, s.schoolA)
	ch := s.open(channel.Direct, hs, ban)
	start := s.clock.Now()
	mine1 := s.msg(ch, false, hs, 1, "zqbg mot")
	s.msg(ch, false, ban, 2, "zqbg hai") // the other student: in the channel scope only
	mine3 := s.msg(ch, false, hs, 3, "zqbg ba")
	party := s.open(channel.Party, hs, ban)
	gone := s.msg(party, true, hs, 1, "zqbg choi")
	// Purged by gameplay-chat-purge (ADR-0033): the row no longer exists and break-glass never restores it (AC-22).
	s.exec(`DELETE FROM chat.gameplay_messages WHERE message_id = $1`, gone)
	hsB := s.student(s.tenantB, s.schoolB)
	chB := s.open(channel.Direct, hsB, s.student(s.tenantB, s.schoolB))
	s.msg(chB, false, hsB, 1, "zqbg khac tenant")
	s.clock.Add(time.Hour)
	late := s.msg(ch, false, hs, 4, "zqbg sau cua so") // after window_to
	_ = late
	windowTo := start.Add(30 * time.Minute)
	approve := func(tenant uuid.UUID, scopeType string, scope uuid.UUID, minutes int) string {
		r := s.glassRequest(sa, tenant, scopeType, scope, start.Add(-time.Hour), windowTo, minutes, glassReason)
		if r.status != http.StatusCreated {
			t.Fatalf("request: %d %s", r.status, r.raw)
		}
		id := r.body["grant_id"].(string)
		if a := sa2.post(tenant, "/admin/break-glass/grants/"+id+":approve", map[string]any{}, 1); a.status != http.StatusOK {
			t.Fatalf("approve: %d %s", a.status, a.raw)
		}
		return id
	}
	userGrant := approve(s.tenantA, "user", hs.id, 60)
	pending := s.glassRequest(sa, s.tenantA, "user", hs.id, start.Add(-time.Hour), windowTo, 60, glassReason).body["grant_id"].(string)
	if r := s.glassMessages(sa, pending, ""); r.status != http.StatusForbidden || r.code() != "BREAK_GLASS_NOT_ACTIVE" {
		t.Fatalf("pending grant: %d %s", r.status, r.raw)
	}
	before := glassBypass()
	r := s.glassMessages(sa, userGrant, "")
	if r.status != http.StatusOK || strings.Join(messageIDs(r), ",") != mine1.String()+","+mine3.String() {
		t.Fatalf("user scope: %d %s", r.status, r.raw)
	}
	if glassBypass()-before < 1 {
		t.Fatal("eduworld_tenant_filter_bypass_total{reason=break_glass}")
	}
	found := false
	for _, m := range s.auditMeta("cross_tenant.accessed") {
		found = found || m["reason"] == "break_glass"
	}
	if !found {
		t.Fatal("audit cross_tenant.accessed break_glass")
	}
	if v := s.auditMeta("break_glass.messages_viewed"); len(v) != 1 || v[0]["messages"] != "2" {
		t.Fatalf("audit messages_viewed: %v", v)
	}
	// Keyset pages in SQL.
	p1 := s.glassMessages(sa, userGrant, "?limit=1")
	pg, _ := p1.body["page"].(map[string]any)
	cursor, _ := pg["next_cursor"].(string)
	if len(p1.data()) != 1 || cursor == "" || messageIDs(p1)[0] != mine1.String() {
		t.Fatalf("page 1: %s", p1.raw)
	}
	if p2 := s.glassMessages(sa, userGrant, "?limit=1&cursor="+cursor); len(p2.data()) != 1 || messageIDs(p2)[0] != mine3.String() {
		t.Fatalf("page 2: %s", p2.raw)
	}
	// Another System Admin (the approver) and a safety_moderator cannot read.
	if r := s.glassMessages(sa2, userGrant, ""); r.status != http.StatusForbidden || r.code() != "BREAK_GLASS_NOT_ACTIVE" {
		t.Fatalf("approver reads: %d %s", r.status, r.raw)
	}
	if r := s.glassMessages(sm, userGrant, ""); r.status != http.StatusForbidden {
		t.Fatalf("safety_moderator reads: %d %s", r.status, r.raw)
	}
	if r := s.do(http.MethodGet, "/admin/break-glass/grants/"+userGrant+"/messages", s.staff(s.tenantA, "tenant_admin"), nil, nil); r.status != http.StatusForbidden {
		t.Fatalf("tenant role reads: %d", r.status)
	}
	// Channel scope: every message of the channel in the window, nothing of another tenant.
	chGrant := approve(s.tenantA, "channel", ch, 60)
	if r := s.glassMessages(sa, chGrant, ""); len(r.data()) != 3 {
		t.Fatalf("channel scope: %s", r.raw)
	}
	partyGrant := approve(s.tenantA, "channel", party, 60)
	if r := s.glassMessages(sa, partyGrant, ""); r.status != http.StatusOK || len(r.data()) != 0 {
		t.Fatalf("purged gameplay restored: %s", r.raw)
	}
	crossGrant := approve(s.tenantA, "channel", chB, 60) // a channel of tenant B named in a grant of tenant A
	if r := s.glassMessages(sa, crossGrant, ""); len(r.data()) != 0 {
		t.Fatalf("other tenant leaked: %s", r.raw)
	}
	// AC-20: revoke ends at once; a second revoke is 409; listBreakGlassGrants shows the audit after the fact.
	if r := sa2.post(s.tenantB, "/admin/break-glass/grants/"+chGrant+":revoke", map[string]any{"reason": "Het can thiet"}, 0); r.status != http.StatusNotFound {
		t.Fatalf("revoke from another tenant: %d %s", r.status, r.raw)
	}
	if r := sa2.post(s.tenantA, "/admin/break-glass/grants/"+chGrant+":revoke", map[string]any{"reason": "Het can thiet"}, 0); r.status != http.StatusOK ||
		r.body["status"] != "revoked" {
		t.Fatalf("revoke: %d %s", r.status, r.raw)
	}
	if r := s.glassMessages(sa, chGrant, ""); r.status != http.StatusForbidden || r.code() != "BREAK_GLASS_NOT_ACTIVE" {
		t.Fatalf("revoked grant read: %d %s", r.status, r.raw)
	}
	if r := sa.post(s.tenantA, "/admin/break-glass/grants/"+chGrant+":revoke", map[string]any{"reason": "Lan hai"}, 0); r.status != http.StatusConflict ||
		r.code() != "BREAK_GLASS_NOT_PENDING" {
		t.Fatalf("revoke twice: %d %s", r.status, r.raw)
	}
	// Expiry: past expires_at reads stop at once; the expirer marks the grant expired.
	s.clock.Add(time.Hour)
	if r := s.glassMessages(sa, userGrant, ""); r.status != http.StatusForbidden || r.code() != "BREAK_GLASS_NOT_ACTIVE" {
		t.Fatalf("expired grant read: %d %s", r.status, r.raw)
	}
	if n, err := s.cmd.ExpireBreakGlass(context.Background()); err != nil || n < 1 {
		t.Fatalf("expirer: %d %v", n, err)
	}
	if n, _ := s.cmd.ExpireBreakGlass(context.Background()); n != 0 {
		t.Fatal("expired twice")
	}
	list := sa2.get("/admin/break-glass/grants?status=expired&tenant_ids=" + s.tenantA.String())
	var u map[string]any
	for _, g := range list.data() {
		if g["grant_id"] == userGrant {
			u = g
		}
	}
	if list.status != http.StatusOK || u == nil || u["messages_viewed_count"] != float64(4) || u["reason"] != glassReason {
		t.Fatalf("list grants: %d %s", list.status, list.raw)
	}
	if r := sm.get("/admin/break-glass/grants"); r.status != http.StatusForbidden {
		t.Fatalf("safety_moderator lists grants: %d", r.status)
	}
	pg1 := sa2.get("/admin/break-glass/grants?limit=1")
	pi, _ := pg1.body["page"].(map[string]any)
	if c, _ := pi["next_cursor"].(string); len(pg1.data()) != 1 || c == "" {
		t.Fatalf("grants page: %s", pg1.raw)
	} else if pg2 := sa2.get("/admin/break-glass/grants?limit=1&cursor=" + c); len(pg2.data()) != 1 || pg2.data()[0]["grant_id"] == pg1.data()[0]["grant_id"] {
		t.Fatalf("grants page 2: %s", pg2.raw)
	}
	if len(s.auditMeta("break_glass.revoked")) != 1 || len(s.auditMeta("break_glass.expired")) < 1 {
		t.Fatal("audit revoked/expired")
	}
	// AC-22: no message content in any audit or event.
	s.noContent("zqbg")
}
