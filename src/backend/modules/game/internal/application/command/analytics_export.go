package command

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/analytics"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// ExportReports are the reports of AnalyticsExportRequest.
var ExportReports = map[string]bool{"metrics": true, "retention": true, "funnel": true, "content_stats": true, "performance": true}

// ExportRenderer renders one report of the stored parameters as CSV (query.GameAnalytics, without the authorization
// already checked when the job was created); ErrExportTooLarge beyond dashboard_export_max_rows.
type ExportRenderer interface {
	RenderCSV(ctx context.Context, report string, params json.RawMessage, maxRows int) (csv []byte, rows int, err error)
}

// ErrExportTooLarge: the report has more rows than dashboard_export_max_rows.
var ErrExportTooLarge = port.ErrExportTooLarge

// AnalyticsAuditor records report.exported (pkg/audit through the module recorder).
type AnalyticsAuditor interface {
	Record(ctx context.Context, e audit.Event) error
}

// AnalyticsExports are createGameAnalyticsExport and its worker (F080-AC-13, DASHBOARD_DATA §10): a job row (202), at most
// dashboard_export_max_concurrent_per_user queued/running jobs per user (429 EXPORT_LIMIT_REACHED), a worker writing the
// CSV to the export bucket and a signed link valid dashboard_export_link_ttl_minutes, audit report.exported.
type AnalyticsExports struct {
	UoW      *entx.UnitOfWork
	Store    port.ExportStore
	Files    port.ExportFiles
	Renderer ExportRenderer
	Audit    AnalyticsAuditor
	Config   analytics.Config
	Clock    func() time.Time
}

func (h AnalyticsExports) now() time.Time {
	if h.Clock != nil {
		return h.Clock().UTC()
	}
	return time.Now().UTC()
}

// ExportView is AnalyticsExportJob.
type ExportView struct {
	JobID       uuid.UUID  `json:"job_id"`
	Status      string     `json:"status"`
	Report      string     `json:"report"`
	RowCount    int        `json:"row_count"`
	DownloadURL string     `json:"download_url,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	ErrorCode   string     `json:"error_code,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// View maps a job (status expired is shown as succeeded without link, error EXPORT_EXPIRED).
func (h AnalyticsExports) View(ctx context.Context, j port.ExportJob) (ExportView, error) {
	v := ExportView{JobID: j.ID, Status: j.Status, Report: j.Report, RowCount: j.RowCount, ExpiresAt: j.ExpiresAt, CreatedAt: j.CreatedAt}
	if j.ErrorCode != nil {
		v.ErrorCode = *j.ErrorCode
	}
	expired := j.Status == "expired" || (j.ExpiresAt != nil && !h.now().Before(*j.ExpiresAt))
	if j.Status == "succeeded" || j.Status == "expired" {
		v.Status = "succeeded"
		if expired {
			v.ErrorCode = "EXPORT_EXPIRED"
		} else if j.ObjectKey != nil && h.Files != nil {
			url, err := h.Files.SignGet(ctx, *j.ObjectKey, h.Config.ExportLinkTTL)
			if err != nil {
				return v, problem.New(problem.Unavailable, "export storage unavailable").Retry().Wrap(err)
			}
			v.DownloadURL = url
		}
	}
	return v, nil
}

func requireAnalytics(ctx context.Context) (authz.Principal, error) {
	p, err := principal(ctx)
	if err != nil {
		return p, err
	}
	return p, authz.Require(ctx, p, authz.PermGameAnalyticsRead, authz.Resource{})
}

// Create is createGameAnalyticsExport (Idempotency-Key handled by the HTTP chain): the report parameters are validated by
// the renderer when the job runs; the range rule is checked now (422).
func (h AnalyticsExports) Create(ctx context.Context, report string, params json.RawMessage) (out ExportView, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.CreateGameAnalyticsExport")
	defer func() { done(err) }()
	p, err := requireAnalytics(ctx)
	if err != nil {
		return out, err
	}
	if !ExportReports[report] {
		return out, problem.Invalid("/report", problem.SubInvalid, "unknown report")
	}
	if len(params) == 0 {
		params = json.RawMessage(`{}`)
	}
	if len(params) > 8192 {
		return out, problem.Invalid("/params", problem.SubInvalid, "at most 8 KB")
	}
	if err := checkExportRange(params, h.Config.Range); err != nil {
		return out, err
	}
	j := port.ExportJob{ID: uuid.New(), RequestedBy: p.UserID, Report: report, Params: params, Status: "queued", CreatedAt: h.now()}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		n, err := h.Store.ActiveExports(ctx, p.UserID)
		if err != nil {
			return err
		}
		if h.Config.ExportMaxConcurrent > 0 && n >= h.Config.ExportMaxConcurrent {
			return RetryAfter{Err: problem.New(problem.ExportLimitReached, "at most 2 exports running per user"), After: 30 * time.Second}
		}
		return h.Store.InsertExport(ctx, j)
	})
	if err != nil {
		return out, err
	}
	return h.View(ctx, j)
}

func checkExportRange(params json.RawMessage, rules analytics.RangeRules) error {
	var p struct {
		From  string `json:"from"`
		To    string `json:"to"`
		Grain string `json:"grain"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return problem.Invalid("/params", problem.SubInvalid, "JSON object")
	}
	from, e1 := time.Parse(time.DateOnly, p.From)
	to, e2 := time.Parse(time.DateOnly, p.To)
	if e1 != nil || e2 != nil {
		return problem.Invalid("/params/from", problem.SubRequired, "from and to (YYYY-MM-DD) are required")
	}
	if _, err := analytics.NewRange(from, to, analytics.Grain(p.Grain), rules); err != nil {
		return problem.New(problem.DashboardRangeTooLarge, err.Error())
	}
	return nil
}

// Get is getGameAnalyticsExport: only the requester sees the job (404 otherwise, no existence leak).
func (h AnalyticsExports) Get(ctx context.Context, id uuid.UUID) (out ExportView, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.GetGameAnalyticsExport")
	defer func() { done(err) }()
	p, err := requireAnalytics(ctx)
	if err != nil {
		return out, err
	}
	var j *port.ExportJob
	if err := h.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		j, err = h.Store.Export(ctx, id)
		return err
	}); err != nil {
		return out, err
	}
	if j == nil || j.RequestedBy != p.UserID {
		return out, problem.New(problem.NotFound, "")
	}
	return h.View(ctx, *j)
}

// RunOnce claims one queued job (SKIP LOCKED), renders the CSV, writes it to the export bucket and records
// report.exported; failures are stored on the job (error_code), never retried in a loop.
func (h AnalyticsExports) RunOnce(ctx context.Context) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.RunGameAnalyticsExport")
	defer func() { done(err) }()
	if h.Files == nil || h.Renderer == nil {
		return 0, nil // no object storage configured: jobs stay queued
	}
	ps, err := authz.SystemPlatformScope(JobAnalyticsExport)
	if err != nil {
		return 0, err
	}
	var j *port.ExportJob
	if err := h.UoW.InTenantTx(ctx, ps, func(ctx context.Context) error {
		var err error
		j, err = h.Store.ClaimExport(ctx, h.now())
		return err
	}); err != nil || j == nil {
		return 0, err
	}
	csv, rows, rerr := h.Renderer.RenderCSV(ctx, j.Report, j.Params, h.Config.ExportMaxRows)
	now := h.now()
	j.FinishedAt = &now
	code := ""
	switch {
	case errors.Is(rerr, ErrExportTooLarge):
		code = "EXPORT_TOO_LARGE"
	case rerr != nil:
		code = "INTERNAL"
	default:
		key := "game-analytics/" + j.ID.String() + ".csv"
		if err := h.Files.Put(ctx, key, bytes.NewReader(csv), int64(len(csv))); err != nil {
			code = "INTERNAL"
		} else {
			exp := now.Add(h.Config.ExportRetention)
			j.ObjectKey, j.ExpiresAt, j.RowCount, j.Status = &key, &exp, rows, "succeeded"
		}
	}
	if code != "" {
		j.Status, j.ErrorCode = "failed", &code
	}
	err = h.UoW.InTenantTx(ctx, ps, func(ctx context.Context) error {
		if err := h.Store.FinishExport(ctx, *j); err != nil {
			return err
		}
		if j.Status == "succeeded" && h.Audit != nil {
			return h.Audit.Record(ctx, audit.Event{Action: "report.exported", Result: audit.ResultSuccess,
				Metadata: map[string]any{"report": j.Report, "export_id": j.ID.String(), "rows": rows, "requested_by": j.RequestedBy.String(),
					"module": "game", "kind": "game_analytics"}})
		}
		return nil
	})
	return 1, err
}
