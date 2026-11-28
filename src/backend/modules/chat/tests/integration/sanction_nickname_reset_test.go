//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"
)

// T031 — F136-AC-19 (TC-GAM-014): the case action nickname_reset creates an immediate sanction nickname_reset
// (ends_at = starts_at, scope all) for a student of the case, with its audit and sanction.applied (sanction_type
// nickname_reset, schema v1) in the outbox; it restricts nothing (the student still sends messages), the sanction-expirer
// never lifts it and it cannot be revoked; a tenant role is refused on the admin route and a student outside the case is
// 422.
func TestSanctionNicknameReset(t *testing.T) {
	s := setupSafety(t)
	an, binh, chi := s.student(s.tenantA, s.schoolA), s.student(s.tenantA, s.schoolA), s.student(s.tenantA, s.schoolA)
	if r := s.report(binh, newKey(), "vi", map[string]any{"target_type": "user", "target_id": an.id, "reason": "inappropriate_content",
		"context": map[string]any{"reported_field": "nickname"}}); r.status != http.StatusCreated {
		t.Fatalf("report: %d %s", r.status, r.raw)
	}
	caseID := s.caseOf(an.id)
	sm := s.moderator("safety_moderator", false)
	path := "/admin/moderation/cases/" + caseID.String()
	claim := sm.post(s.tenantA, path+":assign", map[string]any{"assignee_moderator_id": sm.id}, 0)
	v := version(claim)
	if claim.status != http.StatusOK {
		t.Fatalf("claim: %d %s", claim.status, claim.raw)
	}
	if r := sm.post(s.tenantA, path+":act", map[string]any{"action": "nickname_reset", "target_user_id": chi.id, "note": "ten khong phu hop"}, v); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("student outside the case: %d %s", r.status, r.raw)
	}
	r := sm.post(s.tenantA, path+":act", map[string]any{"action": "nickname_reset", "note": "ten khong phu hop"}, v)
	if r.status != http.StatusOK || r.body["status"] != "in_review" {
		t.Fatalf("nickname_reset: %d %s", r.status, r.raw)
	}
	var starts, ends time.Time
	var scope string
	if err := s.su.QueryRowContext(context.Background(), `SELECT starts_at, ends_at, scope FROM chat.sanctions WHERE user_id = $1
		AND type = 'nickname_reset' AND case_id = $2 AND issued_by = $3`, an.id, caseID, sm.id).Scan(&starts, &ends, &scope); err != nil {
		t.Fatalf("sanction row: %v", err)
	}
	if !ends.Equal(starts) || scope != "all" {
		t.Fatalf("immediate sanction: %v %v %s", starts, ends, scope)
	}
	applied := s.events("sanction.applied")
	last := applied[len(applied)-1]
	if last["sanction_type"] != "nickname_reset" || last["user_id"] != an.id.String() || last["issued_by"] != sm.id.String() {
		t.Fatalf("sanction.applied %v", last)
	}
	if n := s.checkEvents("sanction.applied")["sanction.applied"]; n < 1 {
		t.Fatalf("sanction.applied schema: %d", n)
	}
	if !hasAudit(s.audits(), "moderation.sanction.applied") || !hasAudit(s.audits(), "moderation.case.acted") {
		t.Fatal("audits of the reset")
	}
	// Not a restriction: no message refusal, never lifted by the expirer, never revoked.
	s.clock.Add(time.Hour)
	if _, err := s.cmd.ExpireSanctions(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := len(s.events("sanction.lifted")); n != 0 {
		t.Fatalf("sanction.lifted of an immediate sanction: %d", n)
	}
	var id string
	if err := s.su.QueryRowContext(context.Background(), `SELECT sanction_id::text FROM chat.sanctions WHERE user_id = $1 AND type = 'nickname_reset'`,
		an.id).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if rv := sm.post(s.tenantA, "/admin/moderation/sanctions/"+id+":revoke", map[string]any{"reason": "nham lan"}, 0); rv.status != http.StatusConflict {
		t.Fatalf("revoke nickname_reset: %d %s", rv.status, rv.raw)
	}
	// A tenant role cannot act on cases (platform permission moderation.case.act — R20).
	adm := s.staff(s.tenantA, "school_admin")
	h := map[string]string{"X-Tenant-Id": s.tenantA.String(), "If-Match": `W/"` + jsonNum(version(r)) + `"`, "Idempotency-Key": newKey()}
	if rr := s.do(http.MethodPost, path+":act", adm, h, map[string]any{"action": "nickname_reset", "note": "ten khong phu hop"}); rr.status != http.StatusForbidden {
		t.Fatalf("school_admin: %d %s", rr.status, rr.raw)
	}
}
