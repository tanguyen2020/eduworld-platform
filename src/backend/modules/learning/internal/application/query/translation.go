package query

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/scope"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// TranslationJobs reads translation jobs (F025 US5, learning_content.translate).
type TranslationJobs struct {
	Session read.Session
	Model   read.TranslationJobReadModel
	Files   port.ExportFiles
	Clock   func() time.Time
}

// Get returns a job of the caller's scope (another tenant's job is 404) with the presigned download URL of a finished
// export (F025-AC-22).
func (q TranslationJobs) Get(ctx context.Context, id uuid.UUID) (out dto.TranslationJobView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.GetLearningTranslationJob")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	if err := scope.RequireContent(ctx, p, authz.PermLearningContentTranslate, nil); err != nil {
		return out, err
	}
	if err := q.Session.Read(ctx, func(ctx context.Context) error {
		out, err = q.Model.Job(ctx, id)
		return err
	}); err != nil {
		return out, err
	}
	if out.Kind == "export" && out.Status == "succeeded" && out.ResultKey != nil && q.Files != nil {
		at := time.Now().UTC()
		if q.Clock != nil {
			at = q.Clock().UTC()
		}
		u, exp, err := q.Files.DownloadURL(ctx, *out.ResultKey, at)
		if err != nil {
			return out, err
		}
		out.DownloadURL, out.ExpiresAt = u, &exp
	}
	return out, nil
}
