package query

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// exportParams are the parameters of an AnalyticsExportRequest (the filter of the report).
type exportParams struct {
	From           string      `json:"from"`
	To             string      `json:"to"`
	Grain          string      `json:"grain"`
	MetricCodes    []string    `json:"metric_codes"`
	TenantIDs      []uuid.UUID `json:"tenant_ids"`
	SchoolIDs      []string    `json:"school_ids"`
	GradeBands     []string    `json:"grade_bands"`
	Platforms      []string    `json:"platforms"`
	Locales        []string    `json:"locales"`
	DeviceTiers    []string    `json:"device_tiers"`
	ClientVersions []string    `json:"client_versions"`
	FunnelCode     string      `json:"funnel_code"`
	Kind           string      `json:"kind"`
	Sort           string      `json:"sort"`
}

func fstr(v *float64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}

func istr(v *int64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatInt(*v, 10)
}

// RenderCSV implements the export renderer of command.AnalyticsExports: the report of the stored parameters through the same catalog queries
// (suppression included: an export never shows a small cell), as UTF-8 CSV with a header; > maxRows rows →
// port.ErrExportTooLarge. Never chat content, never a student (DASHBOARD_DATA §10).
func (q GameAnalytics) RenderCSV(ctx context.Context, report string, raw json.RawMessage, maxRows int) ([]byte, int, error) {
	var p exportParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, 0, problem.Invalid("/params", problem.SubInvalid, "JSON object")
	}
	from, _ := time.Parse(time.DateOnly, p.From)
	to, _ := time.Parse(time.DateOnly, p.To)
	f := AnalyticsFilter{From: from, To: to, Grain: p.Grain, TenantIDs: p.TenantIDs, Schools: p.SchoolIDs, GradeBands: p.GradeBands,
		Platforms: p.Platforms, Locales: p.Locales, DeviceTiers: p.DeviceTiers, ClientVersions: p.ClientVersions}
	ctx = context.WithValue(ctx, exportKey{}, true)
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	rows := 0
	write := func(rec ...string) error {
		if rows >= maxRows && maxRows > 0 {
			return port.ErrExportTooLarge
		}
		rows++
		return w.Write(rec)
	}
	var err error
	switch report {
	case "metrics":
		var v MetricsView
		if v, err = q.GetMetrics(ctx, p.MetricCodes, f); err != nil {
			return nil, 0, err
		}
		_ = w.Write([]string{"metric_code", "dims", "granularity", "period_start", "value", "suppressed"})
		for _, s := range v.Data {
			dims, _ := json.Marshal(s.Dims)
			for _, pt := range s.Points {
				if err := write(s.MetricCode, string(dims), s.Granularity, pt.PeriodStart, fstr(pt.Value), strconv.FormatBool(pt.Suppressed)); err != nil {
					return nil, rows, err
				}
			}
		}
	case "retention":
		var v RetentionView
		if v, err = q.GetRetention(ctx, f); err != nil {
			return nil, 0, err
		}
		_ = w.Write([]string{"cohort_date", "cohort_size", "day_offset", "rate"})
		for _, c := range v.Cohorts {
			for _, r := range c.Retention {
				if err := write(c.CohortDate, istr(c.CohortSize), strconv.Itoa(r.DayOffset), fstr(r.Rate)); err != nil {
					return nil, rows, err
				}
			}
		}
	case "funnel":
		code := p.FunnelCode
		if code == "" {
			code = "onboarding"
		}
		var v FunnelView
		if v, err = q.GetFunnel(ctx, code, f); err != nil {
			return nil, 0, err
		}
		_ = w.Write([]string{"funnel_code", "step_code", "count", "conversion_rate"})
		for _, s := range v.Steps {
			if err := write(v.FunnelCode, s.StepCode, istr(s.Count), fstr(s.ConversionRate)); err != nil {
				return nil, rows, err
			}
		}
	case "content_stats":
		_ = w.Write([]string{"kind", "content_id", "starts", "completions", "completion_rate", "abandon_rate", "success_rate", "median_duration_ms",
			"flagged_for_review"})
		cursor := ""
		for {
			v, err := q.GetContentStats(ctx, p.Kind, p.Sort, 100, cursor, f)
			if err != nil {
				return nil, 0, err
			}
			for _, it := range v.Data {
				if err := write(it.Kind, it.ContentID, istr(it.Starts), istr(it.Completions), fstr(it.CompletionRate), fstr(it.AbandonRate),
					fstr(it.SuccessRate), istr(it.MedianDurationMS), strconv.FormatBool(it.FlaggedForReview)); err != nil {
					return nil, rows, err
				}
			}
			if !v.Page.HasMore || v.Page.NextCursor == nil {
				break
			}
			cursor = *v.Page.NextCursor
		}
	case "performance":
		var v PerformanceView
		if v, err = q.GetPerformance(ctx, f); err != nil {
			return nil, 0, err
		}
		_ = w.Write([]string{"platform", "device_tier", "preset", "renderer", "client_version", "sessions", "fps_p50", "fps_p10",
			"fps_budget_pass_rate", "load_time_p50_ms", "crash_free_sessions"})
		for _, r := range v.Data {
			if err := write(r.Platform, r.DeviceTier, r.Preset, r.Renderer, r.ClientVersion, strconv.FormatInt(r.Sessions, 10), fstr(r.FPSP50),
				fstr(r.FPSP10), fstr(r.FPSBudgetPassRate), istr(r.LoadTimeP50MS), fstr(r.CrashFreeSessions)); err != nil {
				return nil, rows, err
			}
		}
	default:
		return nil, 0, errors.New("export: unknown report " + report)
	}
	w.Flush()
	return buf.Bytes(), rows, w.Error()
}
