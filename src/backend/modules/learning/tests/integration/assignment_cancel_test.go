//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
)

// F041-1 (Câu hỏi mở, owner decision 2026-11-20): cancelAssignment emits assignment.cancelled (schema v1, without the
// optional course_id since ADR-0095 — the LMS schedule of F040 that consumed it is removed).
func TestAssignmentCancelled(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.asgSetup(ctx, t)
	id := c.openAsg(t, k, nil)
	must(t, c.do(t, http.MethodPost, assignmentsPath+"/"+id+":cancel", adA, idem(), map[string]any{"reason": "Giao nhầm khối"}), http.StatusOK, "cancel")
	ev := c.validPayloads(ctx, t, "assignment.cancelled", id)
	if len(ev) != 1 || ev[0]["reason"] != "Giao nhầm khối" || ev[0]["cancelled_by"] != c.users[adA].UserID.String() || ev[0]["school_id"] != k.school.String() {
		t.Fatalf("assignment.cancelled: %v", ev)
	}
	if _, ok := ev[0]["course_id"]; ok {
		t.Fatalf("assignment.cancelled still carries course_id: %v", ev)
	}
}
