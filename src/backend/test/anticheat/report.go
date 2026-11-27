package anticheat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Status of a scenario run.
type Status string

// Statuses: a skipped scenario never counts as passed (SC-005 needs 10/10 passed).
const (
	Passed  Status = "passed"
	Failed  Status = "failed"
	Skipped Status = "skipped"
)

// Result of one scenario.
type Result struct {
	ID       string        `json:"id"`
	AC       string        `json:"ac"`
	TC       string        `json:"tc,omitempty"`
	Title    string        `json:"title"`
	Status   Status        `json:"status"`
	Reason   string        `json:"reason,omitempty"` // why it failed/skipped (no token, body or PII)
	Duration time.Duration `json:"duration_ns"`
}

// RunOne runs one scenario with a timeout.
func RunOne(ctx context.Context, e *Env, s Scenario, timeout time.Duration) Result {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	t0 := time.Now()
	err := s.Run(ctx, e)
	r := Result{ID: s.ID, AC: s.AC, TC: s.TC, Title: s.Title, Status: Passed, Duration: time.Since(t0)}
	switch {
	case errors.Is(err, ErrSkip):
		r.Status, r.Reason = Skipped, err.Error()
	case err != nil:
		r.Status, r.Reason = Failed, err.Error()
	}
	return r
}

// Summary is the verdict of a run: passed only when every scenario of ANTI_CHEAT §11 passed (SC-005).
func Summary(rs []Result) (passed, total int, ok bool) {
	for _, r := range rs {
		if r.Status == Passed {
			passed++
		}
	}
	return passed, len(rs), len(rs) == len(Scenarios) && passed == len(rs)
}

// Markdown renders the report (docs/12-project format: Vietnamese, no PII, no response body).
func Markdown(env string, at time.Time, rs []Result) string {
	passed, total, ok := Summary(rs)
	var b strings.Builder
	fmt.Fprintf(&b, "# Kiểm tra chống gian lận — %s (%s)\n\n", at.Format("2006-01-02 15:04 MST"), env)
	verdict := "KHÔNG ĐẠT"
	if ok {
		verdict = "ĐẠT"
	}
	fmt.Fprintf(&b, "Kết luận: **%s** — %d/%d kịch bản ANTI_CHEAT §11 đạt (SC-005 cần %d/%d; kịch bản bỏ qua không tính là đạt).\n\n",
		verdict, passed, total, len(Scenarios), len(Scenarios))
	b.WriteString("| Kịch bản | Tiêu chí | TC | Mô tả | Kết quả | Ghi chú |\n|---|---|---|---|---|---|\n")
	for _, r := range rs {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s |\n", r.ID, r.AC, dash(r.TC), r.Title, r.Status, dash(strings.ReplaceAll(r.Reason, "|", "/")))
	}
	return b.String()
}

// JSON renders the machine-readable report.
func JSON(env string, at time.Time, rs []Result) ([]byte, error) {
	passed, total, ok := Summary(rs)
	return json.MarshalIndent(map[string]any{"env": env, "at": at.UTC().Format(time.RFC3339), "passed": passed, "total": total,
		"ok": ok, "results": rs}, "", "  ")
}

func dash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
