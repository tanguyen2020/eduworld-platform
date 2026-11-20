//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// answerTime is the reading time the tests spend on a question before answering: above the default rapid_guess
// threshold of the single-choice fixtures (3 s in grades 1–3), so the answers of the F030/F031 tests stay mastery evidence.
const answerTime = 5 * time.Second

// selfKit runs the consumer learning.mastery_on_graded wired like the binary of F032 (diagnostic priors and the
// recommendations recomputed after the mastery, in the same transaction) on a real NATS JetStream.
func (c *curEnv) selfKit(ctx context.Context, t *testing.T) *masteryKit {
	t.Helper()
	js := jetStream(ctx, t)
	repo, self := write.MasteryRepo{}, write.SelfStudyRepo{}
	k := &masteryKit{c: c, js: js, mastery: command.Mastery{Store: repo, Evidence: repo, Learning: write.LearningEventWriter{},
		Events: write.Publisher{W: c.writer}, Clock: c.clk.Now, Metrics: observability.MasteryCounter{},
		Diagnostic: &command.DiagnosticPriors{Reader: self, Prior: command.DefaultPracticeConfig().Diagnostic}, Recommend: c.recs}}
	o := consumer.MasteryOptions(consumer.MasteryOnGraded{Apply: k.mastery}, c.uow, c.app, js)
	o.BackOff = []time.Duration{300 * time.Millisecond}
	cons, err := outbox.NewConsumer(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { defer close(done); _ = cons.Run(runCtx) }()
	t.Cleanup(func() { cancel(); <-done })
	return k
}

// graded delivers activity.graded of an attempt to learning.mastery_on_graded and waits for it.
func (k *masteryKit) gradedOnly(ctx context.Context, t *testing.T, attemptID string) {
	t.Helper()
	k.deliverTo(ctx, t, k.graded(ctx, t, attemptID), "learning.mastery_on_graded")
}

// publishCurriculum marks the curriculum version of the test published (TEST helper: the publish flow is F021).
func (c *curEnv) publishCurriculum(ctx context.Context, t *testing.T, versionID string) {
	t.Helper()
	// tenancyguard:ok TEST helper under learning_platform
	if _, err := c.platform.ExecContext(ctx, `UPDATE curriculum.curriculum_versions SET status = 'published', published_at = now()
		WHERE curriculum_version_id = $1`, versionID); err != nil {
		t.Fatal(err)
	}
}

// adhoc starts an ad-hoc attempt as sub (a new Idempotency-Key unless h has one).
func (c *curEnv) adhoc(t *testing.T, sub string, h hdr, body map[string]any) resp {
	t.Helper()
	if h == nil {
		h = hdr{}
	}
	if _, ok := h["Idempotency-Key"]; !ok {
		h = merge(idem(), h)
	}
	return c.do(t, http.MethodPost, "/learning/activity-attempts", sub, h, body)
}

// answerAfter advances the test clock by d (the response time measured by the server) and answers.
func (c *curEnv) answerAfter(t *testing.T, d time.Duration, sub, attemptID string, position int, option string) resp {
	t.Helper()
	c.clk.Advance(d)
	return c.answer(t, sub, attemptID, position, map[string]any{"option_id": option}, nil)
}

// submit submits an attempt as sub.
func (c *curEnv) submit(t *testing.T, sub, attemptID string) resp {
	t.Helper()
	return c.do(t, http.MethodPost, "/learning/activity-attempts/"+attemptID+":submit", sub, idem(), nil)
}
