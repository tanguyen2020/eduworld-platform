//go:build integration

package integration

import (
	"context"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

const examSessionsPath = "/exams/sessions"

// examKit is the LMS kit of F041 with a published blueprint of the subject (F042).
type examKit struct {
	asgKit
	blueprint string // content id
	version   string // published version id
}

// examSetup publishes extra questions of the LO (the kit has 3) and a published blueprint of one LO rule (2 questions ×
// 2 points) with variants.
func (c *curEnv) examSetup(ctx context.Context, t *testing.T, extra int, variants map[string]any) examKit {
	t.Helper()
	k := examKit{asgKit: c.asgSetup(ctx, t)}
	for range extra {
		c.published(ctx, t, qlcmA, k.s.qc, nil)
	}
	return c.examSetupOn(ctx, t, k, 4, []any{k.loRule(2, 2)}, variants)
}

// examSetupOn publishes another blueprint on the kit of k.
func (c *curEnv) examSetupOn(ctx context.Context, t *testing.T, k examKit, total float64, rules []any, variants map[string]any) examKit {
	t.Helper()
	body := blueprintBody(k.s.qc, total, rules, variants)
	k.blueprint = must(t, c.do(t, http.MethodPost, blueprintsPath, qlcmA, idem(), body), http.StatusCreated, "blueprint").str("content_id")
	k.version = c.publishDoc(ctx, t, "assessment_blueprints", "assessment_blueprint_versions", "assessment_blueprint_id", k.blueprint, 1)
	return k
}

// num reads a number of the body.
func (r resp) num(key string) float64 {
	v, _ := r.body[key].(float64)
	return v
}

// loRule is one rule of count questions of the LO of the kit.
func (k examKit) loRule(count int, points float64) map[string]any {
	return map[string]any{"objective_ids": []string{k.s.qc.lo}, "count": count, "points_each": points}
}

// sessionBody is an ExamSessionCreate for class 3A, open from a minute ago for an hour, with extra fields.
func (c *curEnv) sessionBody(k examKit, extra map[string]any) map[string]any {
	now := c.clk.Now()
	b := map[string]any{"title": "Kiểm tra chương 2", "assessment_blueprint_version_id": k.version, "course_id": k.course,
		"targets":      []any{map[string]any{"target_type": "class", "target_id": k.class.String()}},
		"window_start": now.Add(-time.Minute).Format(time.RFC3339), "window_end": now.Add(time.Hour).Format(time.RFC3339),
		"duration_seconds": 900}
	for key, v := range extra {
		b[key] = v
	}
	return b
}

// createSession creates a session as qlcmA and returns its id.
func (c *curEnv) createSession(t *testing.T, k examKit, extra map[string]any) string {
	t.Helper()
	return must(t, c.do(t, http.MethodPost, examSessionsPath, qlcmA, idem(), c.sessionBody(k, extra)), http.StatusCreated, "exam session").
		str("exam_session_id")
}

// startExam starts (or resumes) the attempt of sub on a session with a device token.
func (c *curEnv) startExam(t *testing.T, sub, session, device string, body any) resp {
	t.Helper()
	h := idem()
	if device != "" {
		h["X-Exam-Device-Token"] = device
	}
	return c.do(t, http.MethodPost, examSessionsPath+"/"+session+"/attempts", sub, h, body)
}

// saveExam saves the answer of a position.
func (c *curEnv) saveExam(t *testing.T, sub, attempt string, pos int, device string, body map[string]any) resp {
	t.Helper()
	h := hdr{}
	if device != "" {
		h["X-Exam-Device-Token"] = device
	}
	return c.do(t, http.MethodPut, "/exams/attempts/"+attempt+"/answers/"+strconv.Itoa(pos), sub, h, body)
}

// answerExam answers every item of an attempt, right or wrong, then submits it with key.
func (c *curEnv) answerExam(t *testing.T, sub string, a resp, device string, right bool, key hdr) resp {
	t.Helper()
	ok, wrong := correctOption()
	opt := ok
	if !right {
		opt = wrong
	}
	for pos := range a.items("items") {
		c.clk.Advance(answerTime)
		must(t, c.saveExam(t, sub, a.str("attempt_id"), pos+1, device, map[string]any{"response": map[string]any{"option_id": opt}}),
			http.StatusNoContent, "save answer")
	}
	if device != "" {
		key = merge(key, hdr{"X-Exam-Device-Token": device})
	}
	return c.do(t, http.MethodPost, "/exams/attempts/"+a.str("attempt_id")+":submit", sub, key, nil)
}

// gradeExams hands the exam.submitted events of an attempt to the consumer learning.exam_grade (times each).
func (c *curEnv) gradeExams(ctx context.Context, t *testing.T, attempt string, times int) {
	t.Helper()
	rows, err := c.relay.QueryContext(ctx, `SELECT envelope FROM infra.outbox_events WHERE event_type = 'exam.submitted'
		AND envelope->'payload'->>'attempt_id' = $1`, attempt)
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
	h := consumer.ExamGrade{Grader: c.exams}
	for _, e := range envs {
		for range times {
			if err := c.uow.Do(tenancy.WithScope(ctx, scopeOf(t, *e.TenantID)), func(ctx context.Context) error { return h.Handle(ctx, e) }); err != nil {
				t.Fatalf("grade: %v", err)
			}
		}
	}
}

// runExamWindows runs the job exam-window once with the test clock.
func (c *curEnv) runExamWindows(ctx context.Context, t *testing.T) {
	t.Helper()
	if _, err := c.exams.RunWindows(ctx, 100); err != nil {
		t.Fatalf("exam-window: %v", err)
	}
}
