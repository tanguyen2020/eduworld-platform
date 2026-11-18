package read

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent"
	entrecerr "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/syncrecorderror"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// SisImportModel implements read.SisImportReadModel.
type SisImportModel struct{ Session *entx.ReadSession }

var _ read.SisImportReadModel = SisImportModel{}

// Import returns one import of the scope.
func (m SisImportModel) Import(ctx context.Context, id uuid.UUID) (out dto.SisImport, err error) {
	err = m.Session.Read(ctx, func(ctx context.Context) error {
		r, err := cl(ctx).SisImport.Get(ctx, id)
		if ent.IsNotFound(err) {
			return problem.New(problem.NotFound, "")
		}
		if err != nil {
			return err
		}
		out = dto.NewSisImport(r.ID, r.SchoolID, r.Status, r.DryRun, r.EntityKinds, r.SyncRunID, r.Stats, r.CreatedAt)
		return nil
	})
	return out, err
}

// RunErrors returns the record errors of a run (sync_record_errors is append-only and run-scoped; the run
// belongs to an import already checked in the scope).
func (m SisImportModel) RunErrors(ctx context.Context, runID uuid.UUID, limit int) (out []dto.SisRecordError, err error) {
	err = m.Session.Read(ctx, func(ctx context.Context) error {
		rows, err := cl(ctx).SyncRecordError.Query().Where(entrecerr.SyncRunID(runID)).Order(ent.Asc(entrecerr.FieldRowNo)).Limit(limit).All(ctx)
		if err != nil {
			return err
		}
		for _, r := range rows {
			e := dto.SisRecordError{EntityKind: r.EntityKind, ExternalID: r.ExternalID, RowNo: r.RowNo, Column: r.ColumnName, Code: r.ErrorCode,
				Message: r.Message, OccurredAt: r.CreatedAt}
			out = append(out, e)
		}
		return nil
	})
	return out, err
}
