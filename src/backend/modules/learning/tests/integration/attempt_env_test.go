//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

// testClock is a TEST DOUBLE of the server clock of the attempt engine (deadlines, resume window): never in a binary (R17).
type testClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *testClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// Students of the attempt tests (quickstart §1): hs.an.3a (curEnv) and hs.binh.3a of tenant A, hs.dung.7a of tenant B.
const (
	hsBinh = "hs.binh.3a"
	hsDung = "hs.dung.7a"
)

// attSetup is the content of the attempt tests: the curriculum of tenant A, three published single-choice questions
// of tenant A and a published quiz over them; the students are in the projection ref.students.
type attSetup struct {
	qc         qbCurriculum
	questions  []string
	activityID string
}

// enroll puts a student into ref.students (the projection of F020) with a school of its tenant.
func (c *curEnv) enroll(ctx context.Context, t *testing.T, sub string, grade int) {
	t.Helper()
	d := c.users[sub]
	school := uuid.New()
	if _, err := c.platform.ExecContext(ctx, `INSERT INTO ref.schools (school_id, tenant_id, status, source_version) VALUES ($1, $2, 'active', 1)`,
		school, *d.TenantID); err != nil {
		t.Fatal(err)
	}
	if _, err := c.platform.ExecContext(ctx, `INSERT INTO ref.students (student_id, tenant_id, school_id, grade_level, source_version)
		VALUES ($1, $2, $3, $4, 1)`, d.UserID, *d.TenantID, school, grade); err != nil {
		t.Fatal(err)
	}
}

func (c *curEnv) attemptSetup(ctx context.Context, t *testing.T, config func(qvs ...string) map[string]any) attSetup {
	t.Helper()
	c.user(hsBinh, authztest.Student(c.tenantA).PrincipalData, false)
	c.user(hsDung, authztest.Student(c.tenantB).PrincipalData, false)
	c.enroll(ctx, t, hsAn, 3)
	c.enroll(ctx, t, hsBinh, 3)
	c.enroll(ctx, t, hsDung, 7)
	s := attSetup{qc: c.qbSetup(ctx, t)}
	for i := 0; i < 3; i++ {
		s.questions = append(s.questions, c.published(ctx, t, qlcmA, s.qc, nil))
	}
	if config == nil {
		config = quizConfig
	}
	s.activityID = c.createActivity(t, qlcmA, activityBody(s.qc, "quiz", config(s.questions...)))
	c.publishActivity(ctx, t, s.activityID, 1)
	return s
}

// start starts an attempt on an activity as sub with the headers h (an Idempotency-Key unless h has one).
func (c *curEnv) start(t *testing.T, sub, activityID string, h hdr, body map[string]any) resp {
	t.Helper()
	if h == nil {
		h = hdr{}
	}
	if _, ok := h["Idempotency-Key"]; !ok {
		h = merge(idem(), h)
	}
	if body == nil {
		body = map[string]any{}
	}
	return c.do(t, http.MethodPost, "/learning/activities/"+activityID+"/attempts", sub, h, body)
}

// answer sends a response to position as sub.
func (c *curEnv) answer(t *testing.T, sub, attemptID string, position int, response any, extra map[string]any) resp {
	t.Helper()
	body := map[string]any{"position": position, "response": response}
	for k, v := range extra {
		body[k] = v
	}
	return c.do(t, http.MethodPost, "/learning/activity-attempts/"+attemptID+"/responses", sub, idem(), body)
}

// correctOption returns the correct option of the single-choice fixture (o_3b2e: 7 × 8 = 56) and a wrong one.
func correctOption() (string, string) { return "o_3b2e", "o_a91c" }

// answerKeysIn lists the server-only keys found anywhere in a JSON document (F030-AC-02).
func answerKeysIn(raw string) []string {
	var v any
	_ = json.Unmarshal([]byte(raw), &v)
	keys := map[string]bool{}
	var rec func(x any)
	rec = func(x any) {
		switch t := x.(type) {
		case map[string]any:
			for k, e := range t {
				keys[k] = true
				rec(e)
			}
		case []any:
			for _, e := range t {
				rec(e)
			}
		}
	}
	rec(v)
	var out []string
	for _, k := range []string{"answer_key", "scoring_rule", "accepted_answers", "correct_option_id", "correct_option_ids", "explanation",
		"option_feedback", "hints", "misconception_tag", "min_response_ms", "question_source", "pass_threshold"} {
		if keys[k] {
			out = append(out, k)
		}
	}
	// options[].feedback: a feedback key inside an option.
	if strings.Contains(raw, `"feedback":"`) {
		out = append(out, "options[].feedback")
	}
	return out
}

// studentOf returns the context of a student principal of the test (the HTTP chain builds the same).
func (c *curEnv) studentOf(ctx context.Context, sub string) context.Context {
	return authztest.Ctx(ctx, authz.NewPrincipal(c.users[sub]))
}

// authzPrincipal is the principal of a demo user of the test.
func authzPrincipal(c *curEnv, sub string) authz.Principal { return authz.NewPrincipal(c.users[sub]) }
