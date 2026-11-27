// Package attempt is the learning behaviour of a bot (TEST_STRATEGY §8.1: an answer about every 20 s while
// learning, ~3 submits per session; TC-PERF-005 exam burst; TC-PERF-013 inter-module): startActivityAttempt,
// submitActivityResponse per item with a raw response (never a score, ADR-0016), then submitActivityAttempt with an
// Idempotency-Key. Every attempt answered 2xx is counted, so the chaos reconciliation (F132-AC-12) can compare the bot
// count with the submitted rows of the perf-* tenants.
package attempt

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/gameclient/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/loadbot/internal/report"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/loadbot/internal/restcall"
)

// Options of one attempt.
type Options struct {
	ActivityID string
	Think      func() time.Duration // between answers (answer_rate: ≥ 1 s per question)
}

type started struct {
	AttemptID string `json:"attempt_id"`
	Items     []struct {
		Position          int    `json:"position"`
		QuestionVersionID string `json:"question_version_id"`
		RenderPayload     struct {
			Options []struct {
				OptionID string `json:"option_id"`
				ID       string `json:"id"`
			} `json:"options"`
		} `json:"render_payload"`
	} `json:"items"`
}

// Result of one attempt.
type Result struct {
	AttemptID string
	Answered  int
	Submitted bool // submit answered 2xx
}

// Run does one attempt.
func Run(ctx context.Context, rc *restcall.Client, m *report.Metrics, o Options) (Result, error) {
	if o.Think == nil {
		o.Think = func() time.Duration { return 20 * time.Second }
	}
	status, b, err := rc.Do(ctx, "startActivityAttempt", http.MethodPost, "/learning/activities/"+o.ActivityID+"/attempts",
		map[string]any{"mode": "practice"}, "")
	if err != nil {
		return Result{}, fmt.Errorf("attempt: start: %w", err)
	}
	if status != http.StatusCreated && status != http.StatusOK {
		return Result{}, fmt.Errorf("attempt: start: status %d", status)
	}
	var a started
	if err := json.Unmarshal(b, &a); err != nil || a.AttemptID == "" {
		return Result{}, fmt.Errorf("attempt: start body: %w", err)
	}
	res := Result{AttemptID: a.AttemptID}
	for _, it := range a.Items {
		if len(it.RenderPayload.Options) == 0 {
			m.Error("attempt_unsupported_item") // only choice items are answered by the bot
			continue
		}
		select {
		case <-ctx.Done():
			return res, ctx.Err()
		case <-time.After(o.Think()):
		}
		opt := it.RenderPayload.Options[0].OptionID
		if opt == "" {
			opt = it.RenderPayload.Options[0].ID
		}
		status, _, err := rc.Do(ctx, "submitActivityResponse", http.MethodPost, "/learning/activity-attempts/"+a.AttemptID+"/responses",
			map[string]any{"kind": "question", "position": it.Position, "question_version_id": it.QuestionVersionID,
				"response": map[string]any{"option_id": opt}}, "")
		if err == nil && status/100 == 2 {
			res.Answered++
		}
	}
	key := ids.V7(time.Now())
	status, _, err = rc.Do(ctx, "submitActivityAttempt", http.MethodPost, "/learning/activity-attempts/"+a.AttemptID+":submit", nil, key)
	if err != nil {
		return res, fmt.Errorf("attempt: submit: %w", err)
	}
	// 2xx submits are counted by rc (eduworld_loadbot_http_requests_total{op="submitActivityAttempt",status_class="2xx"}).
	res.Submitted = status/100 == 2
	return res, nil
}
