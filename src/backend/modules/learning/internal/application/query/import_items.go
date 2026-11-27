package query

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/docximport"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// ImportItemsInput is the request of listContentImportItems.
type ImportItemsInput struct {
	ImportID uuid.UUID
	Status   string
	Page     PageInput
}

// ListImportItems pages the questions of a lessons_docx job by item_no (F028-AC-08): learning_content.import; the
// job must be visible in the scope (a job of another tenant is 404, F028-AC-14).
func (q Curriculum) ListImportItems(ctx context.Context, in ImportItemsInput) (out Page[dto.ImportItem], err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListContentImportItems")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	if err := authz.Require(ctx, p, authz.PermLearningContentImport, authz.Resource{}); err != nil {
		return out, err
	}
	switch in.Status {
	case "", docximport.StatusReady, docximport.StatusWarning, docximport.StatusError, docximport.StatusCreated, docximport.StatusDraftFailed:
	default:
		return out, problem.Invalid("status", problem.SubInvalid, "is not an import item status")
	}
	seek, limit, err := seekOf(q.Cursors, in.Page, kInt)
	if err != nil {
		return out, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		if _, err := q.Model.ImportJob(ctx, in.ImportID); err != nil {
			return err
		}
		rows, err := q.Model.ImportItems(ctx, in.ImportID, in.Status, seek)
		if err != nil {
			return err
		}
		out = keysetPage(rows, limit, q.Cursors, func(it dto.ImportItem) (uuid.UUID, []any) { return it.ID, []any{it.No} })
		return nil
	})
	return out, err
}
