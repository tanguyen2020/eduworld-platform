//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

// F073-AC-17 (T027, TC-ACT-005, ADR-0043): a play keeps the questions and the rules of the configuration version it
// pinned when a new version is published meanwhile; new plays use the version module game pins; a minigame learning
// activity following the current version pins the version current at its start.
func TestMinigameVersionPinning(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s := c.attemptSetup(ctx, t, nil)
	c.user(hsGiang, authztest.Student(c.tenantA).PrincipalData, false)
	c.enroll(ctx, t, hsGiang, 7)
	configID := uuid.New()
	v1 := mgSnapshot(configID, 1, "multiple_choice", selectionSource(s.qc.subject, "single_choice"), 2, func(m map[string]any) {
		m["scoring"] = map[string]any{"points_per_correct": 100}
	})
	c.project(ctx, t, v1)
	a1, err := c.mgStart(ctx, hsGiang, v1, learningapi.AttemptModeMinigame)
	if err != nil || a1.QuestionCount != 2 {
		t.Fatalf("play on v1: %+v %v", a1, err)
	}
	// The wrapper activity follows the current version: its first play pins v1.
	act := must(t, c.do(t, http.MethodPost, "/cms/learning/activities", qlcmA, idem(), activityBody(s.qc, "minigame", minigameActivity(configID, nil, nil))),
		http.StatusCreated, "activity").str("activity_id")
	c.publishActivity(ctx, t, act, 1)
	w1 := must(t, c.start(t, hsGiang, act, nil, map[string]any{"mode": "minigame"}), http.StatusCreated, "activity play on v1")
	// v2 is published while the plays run: 3 questions, 50 points, time limit 60 s.
	c.clk.Advance(time.Second)
	v2 := mgSnapshot(configID, 2, "multiple_choice", selectionSource(s.qc.subject, "single_choice"), 3, func(m map[string]any) {
		m["scoring"] = map[string]any{"points_per_correct": 50}
		m["time_limit_seconds"] = 60
	})
	c.project(ctx, t, v2)
	right, _ := correctOption()
	g := c.getAttempt(t, hsGiang, a1.AttemptId)
	if len(g.items("items")) != 2 || mgNum(g, "render", "render_payload", "config", "question_count") != 2 ||
		g.str("render", "render_payload", "minigame_config_version_id") != v1.versionID.String() ||
		mgNum(g, "render", "render_payload", "config", "scoring", "points_per_correct") != 100 {
		t.Fatalf("v1 play after v2: %s", g.raw)
	}
	if r := must(t, c.mgAnswer(t, hsGiang, a1.AttemptId, 1, right, nil), http.StatusOK, "v1 answer"); mgNum(r, "game_points") != 100 {
		t.Fatalf("v1 rules: %s", r.raw)
	}
	w1id := uuid.MustParse(w1.str("attempt_id"))
	if r := must(t, c.mgAnswer(t, hsGiang, w1id, 1, right, nil), http.StatusOK, "activity v1 answer"); mgNum(r, "game_points") != 100 ||
		c.getAttempt(t, hsGiang, w1id).str("render", "render_payload", "minigame_config_version_id") != v1.versionID.String() {
		t.Fatalf("activity play keeps v1: %s", r.raw)
	}
	// New plays: module game pins v2; the activity now pins the current version v2.
	a2, err := c.mgStart(ctx, hsGiang, v2, learningapi.AttemptModeMinigame)
	if err != nil || a2.QuestionCount != 3 || a2.DeadlineAt == nil || a2.DeadlineAt.Sub(a2.StartedAt) != 60*time.Second {
		t.Fatalf("play on v2: %+v %v", a2, err)
	}
	if r := must(t, c.mgAnswer(t, hsGiang, a2.AttemptId, 1, right, nil), http.StatusOK, "v2 answer"); mgNum(r, "game_points") != 50 {
		t.Fatalf("v2 rules: %s", r.raw)
	}
	w2 := must(t, c.start(t, hsBinh, act, nil, map[string]any{"mode": "minigame"}), http.StatusCreated, "activity play on v2")
	if w2.str("render", "render_payload", "minigame_config_version_id") != v2.versionID.String() || len(w2.items("items")) != 3 {
		t.Fatalf("activity play on the current version: %s", w2.raw)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.attempts WHERE attempt_id IN ($1, $2) AND minigame_config_version_id = $3`,
		a1.AttemptId, w1id, v1.versionID); n != 2 {
		t.Fatal("pinned v1")
	}
}
