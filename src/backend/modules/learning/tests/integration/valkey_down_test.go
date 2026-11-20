//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/cache"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// F031-AC-06 (TC-PERF-008, quickstart 6): with Valkey stopped, a practice from the question pool (the path that uses the
// pool cache) still starts, is graded and its evidence reaches the mastery: learning events, evaluation and mastery are
// written to PostgreSQL only (ADR-0004).
func TestLearningRecordWithoutValkey(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s := c.attemptSetup(ctx, t, nil)
	k := c.masteryKit(ctx, t)
	container, addr := testkit.StartValkey(ctx, t)
	vk, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{addr}, DisableCache: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(vk.Close)
	att := c.att
	d := att.Delivery.(query.Delivery)
	d.Cache = cache.Pool{C: vk}
	att.Delivery = d
	if err := container.Stop(ctx, nil); err != nil {
		t.Fatal(err)
	}
	sctx := c.studentOf(ctx, hsAn)
	var in command.StartAdHocInput
	_ = json.Unmarshal([]byte(`{"mode":"practice","source":{"kind":"selection","topic_id":"`+s.qc.topic+`","question_count":3}}`), &in)
	v, err := att.StartAdHoc(sctx, in)
	if err != nil {
		t.Fatalf("start with Valkey down: %v", err)
	}
	right, _ := correctOption()
	for pos := 1; pos <= 3; pos++ {
		c.clk.Advance(answerTime) // not a rapid guess (F032-AC-16)
		if _, err := att.Respond(sctx, command.RespondInput{AttemptID: v.AttemptID, Position: pos,
			Response: json.RawMessage(`{"option_id":"` + right + `"}`)}); err != nil {
			t.Fatalf("respond with Valkey down: %v", err)
		}
	}
	if _, err := att.Submit(sctx, v.AttemptID); err != nil {
		t.Fatalf("submit with Valkey down: %v", err)
	}
	k.deliver(ctx, t, k.graded(ctx, t, v.AttemptID.String()))
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.learning_events WHERE payload->>'attempt_id' = $1`, v.AttemptID.String()); n != 6 {
		t.Fatalf("learning events = %d", n)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.evaluations WHERE attempt_id = $1 AND effective`, v.AttemptID); n != 1 {
		t.Fatalf("evaluations = %d", n)
	}
	if m := c.masteryOf(ctx, t, c.users[hsAn].UserID, s.qc.lo); m.evidence != 3 {
		t.Fatalf("mastery: %+v", m)
	}
}
