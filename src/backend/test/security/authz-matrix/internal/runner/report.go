package runner

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// Summary counts the verdicts of a run.
type Summary struct {
	Total        int     `json:"total"`
	Pass         int     `json:"pass"`
	Fail         int     `json:"fail"`
	Observed     int     `json:"observed"`
	Inconclusive int     `json:"inconclusive"`
	Skipped      int     `json:"skipped"`
	PassRate     float64 `json:"pass_rate"` // pass / (pass + fail + inconclusive), percent
}

// Report is the JSON output of a run (data-model F132 §1: report). Synthetic data only, no token, no payload.
type Report struct {
	RunID      string             `json:"run_id"`
	Env        string             `json:"env"`
	Mode       string             `json:"mode"`
	Target     string             `json:"target"` // host of the base URL (no credentials)
	Commit     string             `json:"commit,omitempty"`
	StartedAt  time.Time          `json:"started_at"`
	FinishedAt time.Time          `json:"finished_at"`
	Summary    Summary            `json:"summary"`
	ByRule     map[string]Summary `json:"by_rule"`
	Results    []Result           `json:"results"`
	NotCovered int                `json:"not_covered,omitempty"` // maxLength strings under composite schemas
}

// Summarise fills Summary and ByRule from Results.
func (rep *Report) Summarise() {
	rep.Summary, rep.ByRule = summary(rep.Results), map[string]Summary{}
	groups := map[string][]Result{}
	for _, r := range rep.Results {
		groups[r.Rule] = append(groups[r.Rule], r)
	}
	for k, g := range groups {
		rep.ByRule[k] = summary(g)
	}
}

func summary(rs []Result) Summary {
	var s Summary
	for _, r := range rs {
		s.Total++
		switch r.Verdict {
		case Pass:
			s.Pass++
		case Fail:
			s.Fail++
		case Observed:
			s.Observed++
		case Inconclusive:
			s.Inconclusive++
		case Skipped:
			s.Skipped++
		}
	}
	if d := s.Pass + s.Fail + s.Inconclusive; d > 0 {
		s.PassRate = float64(s.Pass) * 100 / float64(d)
	}
	return s
}

// Gate returns an error when the run must fail the job: any fail; with strict, also any inconclusive or skipped
// assertion (the staging gate of F132-AC-15/AC-17 requires 100 % of the asserted cases).
func (rep *Report) Gate(strict bool) error {
	if rep.Summary.Fail > 0 {
		return fmt.Errorf("authz-matrix: %d failed case(s)", rep.Summary.Fail)
	}
	if strict && (rep.Summary.Inconclusive > 0 || rep.Summary.Skipped > 0) {
		return fmt.Errorf("authz-matrix: %d inconclusive and %d skipped case(s) in strict mode", rep.Summary.Inconclusive, rep.Summary.Skipped)
	}
	if rep.Summary.Total == 0 {
		return fmt.Errorf("authz-matrix: no case ran")
	}
	return nil
}

// WriteJSON writes the report as indented JSON.
func (rep *Report) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rep)
}

// WriteMarkdown writes the human report: totals, per rule, then every fail and inconclusive case.
func (rep *Report) WriteMarkdown(w io.Writer) error {
	b := &strings.Builder{}
	fmt.Fprintf(b, "# authz-matrix — %s (%s)\n\n", rep.Mode, rep.Env)
	fmt.Fprintf(b, "Run `%s` · target `%s` · commit `%s` · %s → %s\n\n", rep.RunID, rep.Target, orDash(rep.Commit),
		rep.StartedAt.UTC().Format(time.RFC3339), rep.FinishedAt.UTC().Format(time.RFC3339))
	s := rep.Summary
	verdict := "ĐẠT"
	if s.Fail > 0 || s.Inconclusive > 0 || s.Skipped > 0 {
		verdict = "KHÔNG ĐẠT"
	}
	fmt.Fprintf(b, "**Kết luận: %s** — %d ca, %d pass, %d fail, %d inconclusive, %d skipped, %d observed; tỷ lệ đạt %.2f%%.\n\n",
		verdict, s.Total, s.Pass, s.Fail, s.Inconclusive, s.Skipped, s.Observed, s.PassRate)
	if rep.NotCovered > 0 {
		fmt.Fprintf(b, "Chuỗi trong oneOf/anyOf/allOf không sinh ca maxLength: %d (kiểm bằng test contract của module).\n\n", rep.NotCovered)
	}
	b.WriteString("| Quy tắc | Ca | Pass | Fail | Inconclusive | Skipped | Observed |\n|---|---|---|---|---|---|---|\n")
	rules := make([]string, 0, len(rep.ByRule))
	for k := range rep.ByRule {
		rules = append(rules, k)
	}
	sort.Strings(rules)
	for _, k := range rules {
		r := rep.ByRule[k]
		fmt.Fprintf(b, "| `%s` | %d | %d | %d | %d | %d | %d |\n", k, r.Total, r.Pass, r.Fail, r.Inconclusive, r.Skipped, r.Observed)
	}
	var bad []Result
	for _, r := range rep.Results {
		if r.Verdict == Fail || r.Verdict == Inconclusive {
			bad = append(bad, r)
		}
	}
	if len(bad) > 0 {
		b.WriteString("\n## Ca fail / inconclusive\n\n| Verdict | Operation / đích | Role | Quan hệ | Trường | Kỳ vọng | Nhận | Ghi chú |\n|---|---|---|---|---|---|---|---|\n")
		for _, r := range bad {
			what := r.OpID
			if what == "" {
				what = r.Target
			}
			fmt.Fprintf(b, "| %s | `%s` | %s | %s | %s | %s | %d %s | %s |\n", r.Verdict, what, orDash(string(r.Role)), orDash(string(r.Relation)),
				orDash(r.Field), r.Expected, r.Status, r.Code, r.Note)
		}
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
