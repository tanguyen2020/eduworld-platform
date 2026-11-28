//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// T027 — F136-AC-18 (TC-GAM-014): a report of a nickname (target_type user, reason inappropriate_content,
// context.reported_field nickname) snapshots the CURRENT nickname of the reported student from ref.player_profiles into
// the evidence of the case — never a value of the client (R5); a second reporter joins the same case within 24 hours;
// getModerationCase shows reported_nickname to the Safety Team; reported_field on a message report is 422; the reported
// student learns nothing of the reporter.
func TestReportNicknameSnapshot(t *testing.T) {
	s := setupSafety(t)
	an, binh, chi := s.student(s.tenantA, s.schoolA), s.student(s.tenantA, s.schoolA), s.student(s.tenantA, s.schoolA2)
	body := map[string]any{"target_type": "user", "target_id": an.id, "reason": "inappropriate_content",
		"note": "ten that la Fake Client Nick", "context": map[string]any{"reported_field": "nickname"}}
	r := s.report(binh, newKey(), "vi", body)
	if r.status != http.StatusCreated {
		t.Fatalf("report nickname: %d %s", r.status, r.raw)
	}
	if r2 := s.report(chi, newKey(), "vi", body); r2.status != http.StatusCreated {
		t.Fatalf("second reporter: %d %s", r2.status, r2.raw)
	}
	caseID := s.caseOf(an.id)
	if c := s.caseRow(caseID); c["report_count"] != 2 || !strings.Contains(c["subjects"].(string), an.id.String()) {
		t.Fatalf("merged case: %v", c)
	}
	rows, err := s.su.QueryContext(context.Background(), `SELECT snapshot FROM chat.moderation_evidence WHERE case_id = $1`, caseID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	n := 0
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			t.Fatal(err)
		}
		var snap map[string]any
		if err := json.Unmarshal(raw, &snap); err != nil {
			t.Fatal(err)
		}
		if snap["reported_nickname"] != an.nickname || strings.Contains(string(raw), "Fake Client Nick") {
			t.Fatalf("snapshot %s, want the server nickname %q", raw, an.nickname)
		}
		n++
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("evidence rows %d, want one per report", n)
	}
	lead := s.moderator("safety_moderator", true)
	d := lead.get("/admin/moderation/cases/" + caseID.String())
	if d.status != http.StatusOK {
		t.Fatalf("getModerationCase: %d %s", d.status, d.raw)
	}
	ev, _ := d.body["evidence"].([]any)
	if len(ev) == 0 || ev[0].(map[string]any)["reported_nickname"] != an.nickname {
		t.Fatalf("evidence of the case: %s", d.raw)
	}
	// reported_field belongs to a user report.
	msgBody := map[string]any{"target_type": "message", "target_id": an.id, "reason": "inappropriate_content",
		"context": map[string]any{"reported_field": "nickname"}}
	if r := s.report(binh, newKey(), "vi", msgBody); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("reported_field on a message: %d %s", r.status, r.raw)
	}
	if r := s.report(binh, newKey(), "vi", map[string]any{"target_type": "user", "target_id": an.id, "reason": "inappropriate_content",
		"context": map[string]any{"reported_field": "avatar"}}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("unknown reported_field: %d %s", r.status, r.raw)
	}
	// The reported student sees no report about itself (no reporter id anywhere in its notifications).
	if h := s.call(an, http.MethodGet, "/notifications", "", nil); strings.Contains(h.raw, binh.id.String()) {
		t.Fatalf("reporter leaked to the reported student: %s", h.raw)
	}
}
