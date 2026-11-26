package contract

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/notification"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/contract"
)

// recorder is a TEST DOUBLE of the CreateNotifications use case (R17): it records what the notifier hands over.
type recorder struct {
	src        notification.Source
	recipients []uuid.UUID
	actor      *uuid.UUID
	assignment uuid.UUID
	calls      int
}

func (r *recorder) FromEvent(_ context.Context, src notification.Source, recipients []uuid.UUID, actor *uuid.UUID) (int, error) {
	r.src, r.recipients, r.actor, r.calls = src, recipients, actor, r.calls+1
	return len(recipients), nil
}

func (r *recorder) FromAssignment(_ context.Context, src notification.Source, assignment uuid.UUID) (int, error) {
	r.src, r.assignment, r.calls = src, assignment, r.calls+1
	return 1, nil
}

// Contract test of the source events of the notifier (F123 T014): a payload valid against the JSON Schema of its producer
// (specs/events/payloads, the shared assignment.created v2 for assignment.published) decodes to the right recipient,
// actor, reference and params; each event type is on the subjects of exactly one chat.notify_* consumer; an
// assignment.published that is not open notifies nobody.
func TestNotificationSourcesContract(t *testing.T) {
	tenant, school := uuid.New(), uuid.New()
	a, b, ref := uuid.New(), uuid.New(), uuid.New()
	now := time.Now().UTC().Format(time.RFC3339)
	assignmentCreated := func(status string) map[string]any {
		return map[string]any{"assignment_id": ref.String(), "school_id": school.String(), "subject_id": uuid.NewString(),
			"created_by": uuid.NewString(), "content_type": "activity", "content_id": uuid.NewString(), "mode": "practice",
			"status": status, "targets": []any{map[string]any{"target_type": "grade", "grade_level": 3, "student_count": 40}},
			"open_at": now, "due_at": now, "feedback_timing": "after_submit", "result_release": "after_close"}
	}
	cases := []struct {
		name, module, schemaEvent string
		event                     string
		version                   int
		payload                   map[string]any
		recipient, actor          *uuid.UUID
		assignment                bool
		params                    map[string]string
		none                      bool
	}{
		{name: "friend_request", module: "chat", event: "friend_request.sent", version: 1, recipient: &b, actor: &a,
			payload: map[string]any{"request_id": ref.String(), "tenant_id": tenant.String(), "requester_id": a.String(),
				"addressee_id": b.String(), "source": "direct", "expires_at": now}},
		{name: "party_invitation", module: "competition", event: "party.invitation_sent", version: 1, recipient: &b, actor: &a,
			payload: map[string]any{"invitation_id": ref.String(), "party_id": uuid.NewString(), "tenant_id": tenant.String(),
				"inviter_id": a.String(), "invitee_id": b.String(), "expires_at": now}},
		{name: "assignment_open", module: "learning", schemaEvent: "assignment.created", event: "assignment.published", version: 2,
			assignment: true, payload: assignmentCreated("open")},
		{name: "assignment_scheduled", module: "learning", schemaEvent: "assignment.created", event: "assignment.published", version: 2,
			none: true, payload: assignmentCreated("scheduled")},
		{name: "assignment_results", module: "learning", event: "assignment.results_released", version: 1, assignment: true,
			payload: map[string]any{"assignment_id": ref.String(), "school_id": school.String(), "released_at": now, "trigger": "manual"}},
		{name: "level", module: "game", event: "level.reached", version: 1, recipient: &b, params: map[string]string{"level": "5"},
			payload: map[string]any{"student_id": b.String(), "from_level": 4, "to_level": 5, "curve_version": 1, "reached_at": now}},
		{name: "achievement", module: "game", event: "achievement.unlocked", version: 1, recipient: &b, params: map[string]string{"tier": "gold"},
			payload: map[string]any{"achievement_id": ref.String(), "code": "first_quest", "student_id": b.String(), "tier": "gold",
				"category": "first_time", "unlocked_at": now}},
		{name: "report_resolved", module: "chat", event: "report.resolved", version: 1, recipient: &b,
			payload: map[string]any{"tenant_id": tenant.String(), "report_id": ref.String(), "case_id": uuid.NewString(),
				"reporter_id": b.String(), "kind": "moderation_notice", "message_code": "moderation.report_handled", "resolved_at": now}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			se := c.schemaEvent
			if se == "" {
				se = c.event
			}
			if err := contract.CheckPayload(c.module, se, c.version, c.payload); err != nil {
				t.Fatalf("payload breaks %s v%d: %v", se, c.version, err)
			}
			owners := 0
			for _, r := range messaging.Notifiers {
				for _, e := range r.Events {
					if e == c.event && r.Producer == c.module {
						owners++
					}
				}
			}
			if owners != 1 {
				t.Fatalf("%s is read by %d notifier consumers", c.event, owners)
			}
			raw, err := json.Marshal(c.payload)
			if err != nil {
				t.Fatal(err)
			}
			rec := &recorder{}
			e := outbox.Envelope{EventID: uuid.New(), EventType: c.event, SchemaVersion: c.version, OccurredAt: time.Now().UTC(),
				Producer: c.module, TenantID: &tenant, Payload: raw}
			if err := (consumer.Notifier{H: rec}).Handle(context.Background(), e); err != nil {
				t.Fatalf("handle: %v", err)
			}
			switch {
			case c.none:
				if rec.calls != 0 {
					t.Fatalf("not open must notify nobody: %+v", rec)
				}
			case c.assignment:
				if rec.calls != 1 || rec.assignment != ref || rec.src.RefID == nil || *rec.src.RefID != ref || rec.src.TenantID != tenant {
					t.Fatalf("assignment: %+v", rec)
				}
			default:
				if rec.calls != 1 || len(rec.recipients) != 1 || rec.recipients[0] != *c.recipient || rec.src.TenantID != tenant ||
					rec.src.EventID != e.EventID {
					t.Fatalf("recipient: %+v", rec)
				}
				if (c.actor == nil) != (rec.actor == nil) || (c.actor != nil && *rec.actor != *c.actor) {
					t.Fatalf("actor: %v", rec.actor)
				}
				for k, v := range c.params {
					if rec.src.Params[k] != v {
						t.Fatalf("params: %v", rec.src.Params)
					}
				}
			}
		})
	}
}
