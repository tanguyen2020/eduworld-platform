package read

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent"
	entconn "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/sisconnection"
	entconnschool "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/sisconnectionschool"
	entrecerr "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/syncrecorderror"
	entrun "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/syncrun"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// SisAdminModel implements read.SisAdminReadModel (tenant + soft-delete filters from the ent interceptors).
type SisAdminModel struct{ Session *entx.ReadSession }

var _ read.SisAdminReadModel = SisAdminModel{}

func runDTO(r *ent.SyncRun) dto.SyncRun {
	stats := r.Stats
	if stats == nil {
		stats = map[string]int{}
	}
	return dto.SyncRun{SyncRunID: r.ID, ConnectionID: r.SisConnectionID, TenantID: r.TenantID, Kind: r.Mode, Status: r.Status, DryRun: r.DryRun,
		Trigger: r.Trigger, FailureCode: r.FailureCode, StartedAt: r.StartedAt, FinishedAt: r.FinishedAt, Stats: stats}
}

// Connections lists the connections of the scope, newest first, with their active school count and last run.
func (m SisAdminModel) Connections(ctx context.Context, kind, status string, after *page.Cursor, limit int) (out []dto.SisConnection, err error) {
	err = m.Session.Read(ctx, func(ctx context.Context) error {
		c := cl(ctx)
		q := c.SisConnection.Query()
		if kind != "" {
			q = q.Where(entconn.ProviderKind(kind))
		}
		if status != "" {
			q = q.Where(entconn.Status(status))
		}
		if after != nil {
			q = q.Where(entconn.IDLT(after.ID))
		}
		rows, err := q.Order(ent.Desc(entconn.FieldID)).Limit(limit + 1).All(ctx)
		if err != nil {
			return err
		}
		for _, r := range rows {
			n, err := c.SisConnectionSchool.Query().Where(entconnschool.SisConnectionID(r.ID), entconnschool.Active(true)).Count(ctx)
			if err != nil {
				return err
			}
			updated := r.UpdatedAt
			v := dto.SisConnection{ConnectionID: r.ID, TenantID: r.TenantID, ProviderKind: r.ProviderKind, ContractVersion: r.ContractVersion,
				Status: r.Status, SchoolCount: n, CreatedAt: r.CreatedAt, UpdatedAt: &updated}
			last, err := c.SyncRun.Query().Where(entrun.SisConnectionID(r.ID)).Order(ent.Desc(entrun.FieldID)).First(ctx)
			switch {
			case err == nil:
				d := runDTO(last)
				v.LastSyncRun = &d
			case !ent.IsNotFound(err):
				return err
			}
			out = append(out, v)
		}
		return nil
	})
	return out, err
}

// ConnectionTenant returns the tenant of a connection.
func (m SisAdminModel) ConnectionTenant(ctx context.Context, id uuid.UUID) (t uuid.UUID, err error) {
	err = m.Session.Read(ctx, func(ctx context.Context) error {
		r, err := cl(ctx).SisConnection.Get(ctx, id)
		if ent.IsNotFound(err) {
			return problem.New(problem.NotFound, "")
		}
		if err == nil {
			t = r.TenantID
		}
		return err
	})
	return t, err
}

// RunTenant returns the tenant of a run.
func (m SisAdminModel) RunTenant(ctx context.Context, id uuid.UUID) (t uuid.UUID, err error) {
	err = m.Session.Read(ctx, func(ctx context.Context) error {
		r, err := cl(ctx).SyncRun.Get(ctx, id)
		if ent.IsNotFound(err) {
			return problem.New(problem.NotFound, "")
		}
		if err == nil {
			t = r.TenantID
		}
		return err
	})
	return t, err
}

// Runs lists the runs of a connection, newest first.
func (m SisAdminModel) Runs(ctx context.Context, conn uuid.UUID, status string, after *page.Cursor, limit int) (out []dto.SyncRun, err error) {
	err = m.Session.Read(ctx, func(ctx context.Context) error {
		q := cl(ctx).SyncRun.Query().Where(entrun.SisConnectionID(conn))
		if status != "" {
			q = q.Where(entrun.Status(status))
		}
		if after != nil {
			q = q.Where(entrun.IDLT(after.ID))
		}
		rows, err := q.Order(ent.Desc(entrun.FieldID)).Limit(limit + 1).All(ctx)
		if err != nil {
			return err
		}
		for _, r := range rows {
			out = append(out, runDTO(r))
		}
		return nil
	})
	return out, err
}

// Run returns one run of the scope.
func (m SisAdminModel) Run(ctx context.Context, id uuid.UUID) (out dto.SyncRun, err error) {
	err = m.Session.Read(ctx, func(ctx context.Context) error {
		r, err := cl(ctx).SyncRun.Get(ctx, id)
		if ent.IsNotFound(err) {
			return problem.New(problem.NotFound, "")
		}
		if err == nil {
			out = runDTO(r)
		}
		return err
	})
	return out, err
}

// RunErrorPage pages the record errors of a run by id (the run was checked in the scope by the caller).
func (m SisAdminModel) RunErrorPage(ctx context.Context, run uuid.UUID, after *page.Cursor, limit int) (out []dto.SisRecordError, err error) {
	err = m.Session.Read(ctx, func(ctx context.Context) error {
		q := cl(ctx).SyncRecordError.Query().Where(entrecerr.SyncRunID(run))
		if after != nil {
			q = q.Where(entrecerr.IDGT(after.ID))
		}
		rows, err := q.Order(ent.Asc(entrecerr.FieldID)).Limit(limit + 1).All(ctx)
		if err != nil {
			return err
		}
		for _, r := range rows {
			out = append(out, dto.SisRecordError{ID: r.ID, EntityKind: r.EntityKind, ExternalID: r.ExternalID, RowNo: r.RowNo, Column: r.ColumnName,
				Code: r.ErrorCode, Message: r.Message, OccurredAt: r.CreatedAt})
		}
		return nil
	})
	return out, err
}
