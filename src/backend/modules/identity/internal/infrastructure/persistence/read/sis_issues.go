package read

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent"
	entissue "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/reconciliationissue"
	entschool "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/school"
	entimport "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/sisimport"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// SisIssueModel implements read.SisIssueReadModel (tenant + soft-delete filters from the ent interceptors).
type SisIssueModel struct{ Session *entx.ReadSession }

var _ read.SisIssueReadModel = SisIssueModel{}

// Issues returns issues of the scope, newest first.
func (m SisIssueModel) Issues(ctx context.Context, f read.SisIssueFilter) (out []dto.SisIssue, err error) {
	err = m.Session.Read(ctx, func(ctx context.Context) error {
		c := cl(ctx)
		q := c.ReconciliationIssue.Query()
		if f.SchoolIDs != nil {
			q = q.Where(entissue.SchoolIDIn(f.SchoolIDs...))
		}
		if f.SchoolID != nil {
			q = q.Where(entissue.SchoolID(*f.SchoolID))
		}
		if f.ImportID != nil { // the issues of the run of one import of the school (F052-1)
			iq := c.SisImport.Query().Where(entimport.ID(*f.ImportID))
			if f.SchoolID != nil {
				iq = iq.Where(entimport.SchoolID(*f.SchoolID))
			}
			imp, err := iq.Only(ctx)
			if ent.IsNotFound(err) {
				return problem.New(problem.NotFound, "")
			}
			if err != nil {
				return err
			}
			if imp.SyncRunID == nil { // not processed yet: no issue
				return nil
			}
			q = q.Where(entissue.SyncRunID(*imp.SyncRunID))
		}
		if f.ConnectionID != nil {
			q = q.Where(entissue.SisConnectionID(*f.ConnectionID))
		}
		if f.Type != "" {
			q = q.Where(entissue.IssueType(f.Type))
		}
		if f.Status != "" {
			q = q.Where(entissue.Status(f.Status))
		}
		if f.After != nil {
			q = q.Where(entissue.IDLT(f.After.ID))
		}
		rows, err := q.Order(ent.Desc(entissue.FieldID)).Limit(f.Limit + 1).All(ctx)
		if err != nil {
			return err
		}
		for _, r := range rows {
			out = append(out, dto.SisIssue{IssueID: r.ID, TenantID: r.TenantID, SchoolID: r.SchoolID, ConnectionID: r.SisConnectionID,
				IssueType: r.IssueType, EntityRefs: dto.SisIssueRefs(r.EntityRefs), SuggestedAction: r.SuggestedAction, Status: r.Status,
				Resolution: r.Resolution, ResolutionNote: r.ResolutionNote, ResolvedBy: r.ResolvedBy, ResolvedAt: r.ResolvedAt, CreatedAt: r.CreatedAt})
		}
		return nil
	})
	return out, err
}

// SchoolVisible reports whether a live school of the scope has this id.
func (m SisIssueModel) SchoolVisible(ctx context.Context, schoolID uuid.UUID) (ok bool, err error) {
	err = m.Session.Read(ctx, func(ctx context.Context) error {
		ok, err = cl(ctx).School.Query().Where(entschool.ID(schoolID)).Exist(ctx)
		return err
	})
	return ok, err
}
