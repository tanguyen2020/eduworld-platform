package query

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/gameadmin"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/i18ntext"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// TranslationJobs is getGameTranslationJob (F082-AC-12, AC-16): the requester reads its own job, a System Admin every
// job; any other job is 404 (no existence leak). A succeeded export carries a signed download link valid report_link_ttl.
type TranslationJobs struct {
	Read    read.Session
	Jobs    read.TranslationJobReader
	Storage port.ObjectStorage // nil: no download link
	LinkTTL time.Duration
	Now     func() time.Time
}

func isSystemAdmin(p authz.Principal) bool {
	for _, g := range p.Grants {
		if g.Role == "system_admin" && g.ScopeKind == authz.RoleScopePlatform {
			return true
		}
	}
	return false
}

// Get returns one job.
func (h TranslationJobs) Get(ctx context.Context, id uuid.UUID) (out gameadmin.TranslationJobView, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.GetGameTranslationJob")
	defer func() { done(err) }()
	p, sc, err := gameadmin.Principal(ctx)
	if err != nil {
		return out, err
	}
	if err := authz.Require(ctx, p, authz.PermGameContentTranslate, authz.Resource{}); err != nil {
		return out, err
	}
	var row *read.TranslationJobRow
	if err := h.Read.Read(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		var err error
		row, err = h.Jobs.TranslationJob(ctx, id)
		return err
	}); err != nil {
		return out, err
	}
	if row == nil || (row.RequestedBy != p.UserID && !isSystemAdmin(p)) {
		return out, problem.New(problem.NotFound, "")
	}
	out = gameadmin.TranslationJobView{JobID: row.ID, Kind: row.Kind, Status: row.Status, Format: row.Format, TargetLanguage: row.TargetLanguage,
		DryRun: row.DryRun, CreatedAt: row.CreatedAt.UTC(), FinishedAt: row.FinishedAt, ErrorMessage: row.ErrorMessage, Report: row.Report}
	if out.Report != nil && out.Report.Errors == nil {
		out.Report.Errors = []i18ntext.ReportError{}
	}
	if row.Status == "succeeded" && row.ResultKey != nil && h.Storage != nil {
		url, err := h.Storage.PresignGet(ctx, gameadmin.BucketExports, *row.ResultKey, h.LinkTTL)
		if err != nil {
			return out, problem.New(problem.Unavailable, "object storage unavailable").Retry().Wrap(err)
		}
		now := time.Now().UTC()
		if h.Now != nil {
			now = h.Now().UTC()
		}
		exp := now.Add(h.LinkTTL)
		out.DownloadURL, out.ExpiresAt = &url, &exp
	}
	return out, nil
}
