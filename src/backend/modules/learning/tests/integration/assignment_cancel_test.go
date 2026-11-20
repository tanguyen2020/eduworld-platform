//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// deliverSchedule hands the outbox event typ of an assignment to the consumer learning.lms_schedule, times times.
func (c *curEnv) deliverSchedule(ctx context.Context, t *testing.T, typ, assignment string, times int) {
	t.Helper()
	var raw []byte
	if err := c.relay.QueryRowContext(ctx, `SELECT envelope FROM infra.outbox_events WHERE event_type = $1
		AND envelope->'payload'->>'assignment_id' = $2`, typ, assignment).Scan(&raw); err != nil {
		t.Fatalf("%s: %v", typ, err)
	}
	e := decodeEnvelope(t, raw)
	h := consumer.LMSSchedule{Schedule: c.lms}
	for i := 0; i < times; i++ {
		if err := c.uow.Do(tenancy.WithScope(ctx, scopeOf(t, *e.TenantID)), func(ctx context.Context) error { return h.Handle(ctx, e) }); err != nil {
			t.Fatalf("%s: %v", typ, err)
		}
	}
}

// F041-1 (Câu hỏi mở, owner decision 2026-11-20): cancelAssignment emits assignment.cancelled; the consumer
// learning.lms_schedule (F040) soft-deletes the assignment_due item with schedule_item.deleted, once on redelivery.
func TestAssignmentCancelRemovesSchedule(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.asgSetup(ctx, t)
	id := c.openAsg(t, k, nil)
	c.deliverSchedule(ctx, t, "assignment.published", id, 1)
	var item string
	if err := c.platform.QueryRowContext(ctx, `SELECT schedule_item_id FROM lms.schedule_items WHERE ref_type = 'assignment'
		AND ref_id = $1 AND kind = 'assignment_due' AND deleted_at IS NULL`, id).Scan(&item); err != nil {
		t.Fatalf("schedule item of the assignment: %v", err)
	}

	must(t, c.do(t, http.MethodPost, assignmentsPath+"/"+id+":cancel", adA, idem(), map[string]any{"reason": "Giao nhầm lớp"}), http.StatusOK, "cancel")
	ev := c.outboxPayloads(ctx, t, "assignment.cancelled", "assignment_id", id)
	if len(ev) != 1 || ev[0]["reason"] != "Giao nhầm lớp" || ev[0]["cancelled_by"] != c.users[adA].UserID.String() ||
		ev[0]["course_id"] != k.course {
		t.Fatalf("assignment.cancelled: %v", ev)
	}

	c.deliverSchedule(ctx, t, "assignment.cancelled", id, 2)
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM lms.schedule_items WHERE ref_id = $1 AND deleted_at IS NULL`, id); n != 0 {
		t.Fatalf("live items after cancel: %d", n)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM lms.schedule_items WHERE schedule_item_id = $1 AND deleted_at IS NOT NULL
		AND deleted_by = $2 AND delete_reason = 'cancelled'`, item, c.users[adA].UserID); n != 1 {
		t.Fatal("soft-delete of the schedule item")
	}
	if del := c.outboxPayloads(ctx, t, "schedule_item.deleted", "schedule_item_id", item); len(del) != 1 || del[0]["reason"] != "cancelled" {
		t.Fatalf("schedule_item.deleted: %v", del)
	}
}
