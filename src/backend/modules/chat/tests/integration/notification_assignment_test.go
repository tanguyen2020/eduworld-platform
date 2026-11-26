//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// eventScope is the TenantScope the outbox consumer opens for a tenant event.
func eventScope(t *testing.T, tenant uuid.UUID) tenancy.Scope {
	t.Helper()
	sc, err := authz.EventScope(tenant)
	if err != nil {
		t.Fatal(err)
	}
	return sc
}

// assignmentPayload is assignment.created.v2 (also assignment.published) with status: grade targets (ADR-0095).
func assignmentPayload(id, school uuid.UUID, status string, at time.Time) map[string]any {
	return map[string]any{"assignment_id": id, "school_id": school, "subject_id": uuid.New(), "created_by": uuid.New(), "content_type": "activity",
		"content_id": uuid.New(), "mode": "practice", "status": status, "targets": []any{map[string]any{"target_type": "grade", "grade_level": 3, "student_count": 3}},
		"open_at": at.Format(time.RFC3339), "due_at": at.Add(72 * time.Hour).Format(time.RFC3339), "feedback_timing": "immediate",
		"result_release": "immediate"}
}

// T013 / F123-AC-02, F123-AC-06, F123-AC-07 (TC-SVC-004): assignment.published (open) asks module learning for the
// recipients (facade listAssignmentRecipients through pkg/modcall, every page) — the excused student is not among them —
// and stores one assignment_new per student with the title as only param; a scheduled assignment notifies nobody;
// assignment.results_released notifies the same way; students of another tenant returned for the event are dropped.
// While learning does not answer, the consumer does not ack, retries and stores nothing; after max deliveries the event
// goes to the dead letters; once learning is back a new event notifies everyone (no partial batch).
func TestNotificationAssignment(t *testing.T) {
	n := setupNotify(t, notifyOpts{maxDeliver: 3, backoff: 100 * time.Millisecond})
	an := n.student(n.tenantA, n.schoolA)
	binh := n.student(n.tenantA, n.schoolA) // excused by the school: learning does not return Bình
	chi := n.student(n.tenantA, n.schoolA)
	dung := n.student(n.tenantB, n.schoolB)
	now := n.clock.Now()

	asg := uuid.New()
	n.learning.set(asg, an.id, chi.id, dung.id) // dung: another tenant (dropped by the policy)
	ev := uuid.New()
	n.publish("learning", "assignment.published", n.tenantA, ev, assignmentPayload(asg, n.schoolA, "open", now))
	n.waitNotifications(chi.id, 1)
	n.waitProcessed("chat.notify_assignment", ev)
	if n.notifications(an.id) != 1 || n.notifications(binh.id) != 0 || n.notifications(dung.id) != 0 {
		t.Fatalf("recipients: an %d binh %d dung %d", n.notifications(an.id), n.notifications(binh.id), n.notifications(dung.id))
	}
	if got := n.count(`SELECT count(*) FROM social.notifications WHERE kind = 'assignment_new' AND message_code = 'assignment.new'
		AND ref_type = 'assignment' AND ref_id = $1 AND params = jsonb_build_object('title', 'Ôn tập bảng nhân')
		AND action = jsonb_build_object('kind', 'open_assignment', 'id', $1::text)`, asg); got != 2 {
		t.Fatalf("assignment_new rows: %d", got)
	}
	// A scheduled assignment notifies nobody (only status open, F123-AC-02).
	sched := uuid.New()
	n.learning.set(sched, an.id)
	sev := uuid.New()
	n.publish("learning", "assignment.published", n.tenantA, sev, assignmentPayload(sched, n.schoolA, "scheduled", now))
	n.waitProcessed("chat.notify_assignment", sev)
	if n.notifications(an.id) != 1 {
		t.Fatal("scheduled assignment notified")
	}
	// Results released.
	n.publish("learning", "assignment.results_released", n.tenantA, uuid.New(), map[string]any{"assignment_id": asg,
		"school_id": n.schoolA, "released_at": now.Format(time.RFC3339), "trigger": "manual"})
	n.waitNotifications(an.id, 2)
	n.waitNotifications(chi.id, 2)
	if got := n.count(`SELECT count(*) FROM social.notifications WHERE kind = 'assignment_results_released' AND ref_id = $1`, asg); got != 2 {
		t.Fatalf("results released rows: %d", got)
	}

	// F123-AC-07: learning down → no ack, retries, nothing stored, dead letter after maxDeliver (3).
	n.learning.setDown(true)
	down := uuid.New()
	asg2 := uuid.New()
	n.learning.set(asg2, an.id, chi.id)
	n.publish("learning", "assignment.published", n.tenantA, down, assignmentPayload(asg2, n.schoolA, "open", now))
	n.waitCount(1, 20*time.Second, `SELECT count(*) FROM infra.dead_letters WHERE consumer = 'chat.notify_assignment' AND event_id = $1`, down)
	if calls := n.learning.callCount(); calls < 3 {
		t.Fatalf("learning called %d times before the dead letter", calls)
	}
	if got := n.count(`SELECT count(*) FROM social.notifications WHERE ref_id = $1`, asg2); got != 0 {
		t.Fatalf("partial batch stored while learning was down: %d", got)
	}
	if n.processed("chat.notify_assignment", down) {
		t.Fatal("event acked as processed while learning was down")
	}
	// Learning back: the next event of that assignment notifies every recipient.
	n.learning.setDown(false)
	n.publish("learning", "assignment.results_released", n.tenantA, uuid.New(), map[string]any{"assignment_id": asg2,
		"school_id": n.schoolA, "released_at": now.Format(time.RFC3339), "trigger": "after_close"})
	n.waitCount(2, 10*time.Second, `SELECT count(*) FROM social.notifications WHERE ref_id = $1`, asg2)
}
