package read

import (
	"context"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent"
	entissue "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/reconciliationissue"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// SisIssueModel implements read.SisIssueReadModel (tenant + soft-delete filters from the ent interceptors).
type SisIssueModel struct{ Session *entx.ReadSession }

var _ read.SisIssueReadModel = SisIssueModel{}

// Issues returns issues of the scope, newest first.
func (m SisIssueModel) Issues(ctx context.Context, f read.SisIssueFilter) (out []dto.SisIssue, err error) {
	err = m.Session.Read(ctx, func(ctx context.Context) error {
		q := cl(ctx).ReconciliationIssue.Query()
		if f.SchoolIDs != nil {
			q = q.Where(entissue.SchoolIDIn(f.SchoolIDs...))
		}
		if f.SchoolID != nil {
			q = q.Where(entissue.SchoolID(*f.SchoolID))
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
