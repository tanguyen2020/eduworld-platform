//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

const assignmentsPath = "/lms/assignments"

// asgKit is the school of the LMS kit with the subject of its curriculum (F041; ADR-0095: no course).
type asgKit struct {
	lmsKit
	subject string
}

func (c *curEnv) asgSetup(ctx context.Context, t *testing.T) asgKit {
	t.Helper()
	k := asgKit{lmsKit: c.lmsSetup(ctx, t)}
	k.subject = k.s.qc.subject
	return k
}

// asgBody is an AssignmentCreate of an activity for grade 3 of the school (ADR-0095), open from now for 3 days, with
// extra fields.
func (c *curEnv) asgBody(k asgKit, extra map[string]any) map[string]any {
	now := c.clk.Now()
	b := map[string]any{"school_id": k.school.String(), "subject_id": k.subject, "title": "Ôn tập bảng nhân", "mode": "practice",
		"content_ref": map[string]any{"content_type": "activity", "content_id": k.s.activityID},
		"targets":     []any{map[string]any{"target_type": "grade", "grade_level": 3}},
		"open_at":     now.Add(-time.Minute).Format(time.RFC3339), "due_at": now.Add(72 * time.Hour).Format(time.RFC3339), "max_attempts": 2}
	for key, v := range extra {
		b[key] = v
	}
	return b
}

// createAsg creates an assignment as sub.
func (c *curEnv) createAsg(t *testing.T, sub string, body map[string]any) resp {
	t.Helper()
	return c.do(t, http.MethodPost, assignmentsPath, sub, idem(), body)
}

// publishAsg publishes an assignment as sub with the ETag of r.
func (c *curEnv) publishAsg(t *testing.T, sub string, r resp) resp {
	t.Helper()
	return c.do(t, http.MethodPost, assignmentsPath+"/"+r.str("assignment_id")+":publish", sub, merge(idem(), ifm(r.header.Get("ETag"))), nil)
}

// openAsg creates and publishes an assignment as adA.
func (c *curEnv) openAsg(t *testing.T, k asgKit, extra map[string]any) string {
	t.Helper()
	r := must(t, c.createAsg(t, adA, c.asgBody(k, extra)), http.StatusCreated, "create assignment")
	p := must(t, c.publishAsg(t, adA, r), http.StatusOK, "publish assignment")
	return p.str("assignment_id")
}

// deliverAttempts hands the attempt events of the outbox of an attempt to the consumer learning.assignment_submissions
// (in the transaction and event scope the outbox consumer opens); each event once unless twice.
func (c *curEnv) deliverAttempts(ctx context.Context, t *testing.T, attemptID string, twice bool) {
	t.Helper()
	rows, err := c.relay.QueryContext(ctx, `SELECT envelope FROM infra.outbox_events WHERE event_type IN
		('activity.started','activity.submitted','activity.graded','attempt.re_evaluated') AND envelope->'payload'->>'attempt_id' = $1
		ORDER BY created_at, event_id`, attemptID)
	if err != nil {
		t.Fatal(err)
	}
	var envs []outbox.Envelope
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			t.Fatal(err)
		}
		envs = append(envs, decodeEnvelope(t, raw))
	}
	_ = rows.Close()
	h := consumer.AssignmentSubmissions{Tracker: c.asg}
	for _, e := range envs {
		for i := 0; i < 1+map[bool]int{true: 1}[twice]; i++ {
			if err := c.uow.Do(tenancy.WithScope(ctx, scopeOf(t, *e.TenantID)), func(ctx context.Context) error { return h.Handle(ctx, e) }); err != nil {
				t.Fatalf("%s: %v", e.EventType, err)
			}
		}
	}
}

// enrollmentEvent hands an identity event to the consumer learning.assignment_enrollment.
func (c *curEnv) enrollmentEvent(ctx context.Context, t *testing.T, typ string, payload []byte) {
	t.Helper()
	tenant := c.tenantA
	e := outbox.Envelope{EventID: ids.New(), EventType: typ, TenantID: &tenant, OccurredAt: c.clk.Now(), Payload: payload}
	h := consumer.AssignmentEnrollment{Tracker: c.asg}
	if err := c.uow.Do(tenancy.WithScope(ctx, scopeOf(t, tenant)), func(ctx context.Context) error { return h.Handle(ctx, e) }); err != nil {
		t.Fatalf("%s: %v", typ, err)
	}
}

// submission reads the submission of a student (status, attempts used, score).
func (c *curEnv) submission(ctx context.Context, t *testing.T, assignment string, student uuid.UUID) (string, int, *float64) {
	t.Helper()
	var status string
	var used int
	var score *float64
	if err := c.platform.QueryRowContext(ctx, `SELECT status, attempts_used, score::float8 FROM lms.assignment_submissions
		WHERE assignment_id = $1 AND student_id = $2 AND deleted_at IS NULL`, assignment, student).Scan(&status, &used, &score); err != nil {
		t.Fatalf("submission: %v", err)
	}
	return status, used, score
}

// outboxCount counts outbox events of a type about an assignment.
func (c *curEnv) outboxCount(ctx context.Context, t *testing.T, typ, assignment string) int {
	t.Helper()
	return count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = $1
		AND envelope->'payload'->>'assignment_id' = $2`, typ, assignment)
}

func decodeEnvelope(t *testing.T, raw []byte) outbox.Envelope {
	t.Helper()
	var e outbox.Envelope
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatal(err)
	}
	return e
}
