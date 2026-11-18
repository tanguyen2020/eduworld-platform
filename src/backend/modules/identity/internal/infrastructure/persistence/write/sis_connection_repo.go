package write

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent"
	entschool "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/school"
	entcs "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/sisconnectionschool"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// SisConnectionRepo implements write.SisConnectionRepository.
type SisConnectionRepo struct{}

var _ write.SisConnectionRepository = SisConnectionRepo{}

func connToPort(r *ent.SisConnection) *write.SisConnection {
	return &write.SisConnection{ID: r.ID, TenantID: r.TenantID, ProviderKind: r.ProviderKind, Status: r.Status, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}

// Create inserts the connection (tenant explicit).
func (SisConnectionRepo) Create(ctx context.Context, c *write.SisConnection) error {
	row, err := client(ctx).SisConnection.Create().SetID(c.ID).SetTenantID(c.TenantID).SetProviderKind(c.ProviderKind).SetStatus(c.Status).Save(ctx)
	if err != nil {
		return err
	}
	*c = *connToPort(row)
	return nil
}

// TenantOf reads the tenant of a connection (runs under tenancy.Bypass: the filter is lifted).
func (SisConnectionRepo) TenantOf(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	row, err := client(ctx).SisConnection.Get(ctx, id)
	if ent.IsNotFound(err) {
		return uuid.Nil, problem.New(problem.NotFound, "")
	}
	if err != nil {
		return uuid.Nil, err
	}
	return row.TenantID, nil
}

// Get loads a connection of the scope.
func (SisConnectionRepo) Get(ctx context.Context, id uuid.UUID) (*write.SisConnection, error) {
	row, err := client(ctx).SisConnection.Get(ctx, id)
	if ent.IsNotFound(err) {
		return nil, problem.New(problem.NotFound, "")
	}
	if err != nil {
		return nil, err
	}
	return connToPort(row), nil
}

// SetSchools makes schoolIDs the active schools of c.
func (r SisConnectionRepo) SetSchools(ctx context.Context, c *write.SisConnection, schoolIDs []uuid.UUID, by uuid.UUID) ([]write.SisConnectionSchool, error) {
	cl := client(ctx)
	if len(schoolIDs) > 0 {
		n, err := cl.School.Query().Where(entschool.IDIn(schoolIDs...), entschool.TenantID(c.TenantID)).Count(ctx)
		if err != nil {
			return nil, err
		}
		if n != len(schoolIDs) {
			return nil, problem.Invalid("school_ids", problem.SubInvalid, "every school must belong to the tenant of the connection")
		}
	}
	want := map[uuid.UUID]bool{}
	for _, s := range schoolIDs {
		want[s] = true
	}
	rows, err := cl.SisConnectionSchool.Query().Where(entcs.SisConnectionID(c.ID)).All(ctx)
	if err != nil {
		return nil, err
	}
	have := map[uuid.UUID]*ent.SisConnectionSchool{}
	for _, row := range rows {
		have[row.SchoolID] = row
		if row.Active && !want[row.SchoolID] {
			if _, err := cl.SisConnectionSchool.Update().Where(entcs.ID(row.ID)).SetActive(false).Save(ctx); err != nil {
				return nil, err
			}
		}
	}
	for _, s := range schoolIDs {
		var err error
		if row, ok := have[s]; ok {
			if !row.Active {
				_, err = cl.SisConnectionSchool.Update().Where(entcs.ID(row.ID)).SetActive(true).SetAssignedBy(by).Save(ctx)
			}
		} else {
			_, err = cl.SisConnectionSchool.Create().SetTenantID(c.TenantID).SetSisConnectionID(c.ID).SetSchoolID(s).
				SetActive(true).SetAssignedBy(by).Save(ctx)
		}
		if ent.IsConstraintError(err) && strings.Contains(err.Error(), "sis_connection_schools_one_active") {
			return nil, problem.New(problem.SisSchoolAlreadyConnected, "").With("school_id", s.String()).Wrap(err)
		}
		if err != nil {
			return nil, err
		}
	}
	return r.Schools(ctx, c.ID)
}

// Schools lists the schools of a connection.
func (SisConnectionRepo) Schools(ctx context.Context, connectionID uuid.UUID) ([]write.SisConnectionSchool, error) {
	rows, err := client(ctx).SisConnectionSchool.Query().Where(entcs.SisConnectionID(connectionID)).
		Order(ent.Asc(entcs.FieldAssignedAt)).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]write.SisConnectionSchool, 0, len(rows))
	for _, r := range rows {
		out = append(out, write.SisConnectionSchool{ConnectionID: r.SisConnectionID, SchoolID: r.SchoolID, TenantID: r.TenantID, Active: r.Active, AssignedAt: r.AssignedAt})
	}
	return out, nil
}
