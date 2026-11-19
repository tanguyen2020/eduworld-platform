package write

import (
	"context"
	"slices"
	"strings"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent"
	entcur "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/curriculum"
	entlo "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/learningobjective"
	enttopic "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/topic"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/trash"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// CurriculumTrash implements write.CurriculumTrashStore with ent: entries of curricula, topics and objectives
// (resource_type curriculum). Under tenancy.Bypass(trash) the entx filter shows deleted rows of every tenant.
type CurriculumTrash struct{}

var _ write.CurriculumTrashStore = CurriculumTrash{}

// SoftDeleteCurriculum implements write.CurriculumTrashStore.
func (CurriculumTrash) SoftDeleteCurriculum(ctx context.Context, id uuid.UUID, reason string) error {
	n, err := client(ctx).Curriculum.Delete().Where(entcur.ID(id)).Exec(entx.WithDeleteReason(ctx, reason))
	if err != nil {
		return MapCurriculumErr(err)
	}
	if n == 0 {
		return problem.New(problem.NotFound, "")
	}
	return nil
}

func entry(id uuid.UUID, tenant *uuid.UUID, title string, at *time.Time, by *uuid.UUID, reason *string, conflict bool) trash.Entry {
	e := trash.Entry{ResourceType: contentmodel.ResourceCurriculum, ResourceID: id, TenantID: tenant, Title: title, DeletedBy: by, Conflict: conflict}
	if at != nil {
		e.DeletedAt = *at
	}
	if reason != nil {
		e.DeleteReason = *reason
	}
	return e
}

func sameTenant[P ~func(*entsql.Selector)](t *uuid.UUID, isNil func() P, eq func(uuid.UUID) P) P {
	if t == nil {
		return isNil()
	}
	return eq(*t)
}

func (CurriculumTrash) curriculumEntry(ctx context.Context, r *ent.Curriculum) (trash.Entry, error) {
	conflict, err := client(ctx).Curriculum.Query().Where(entcur.Code(r.Code), entcur.DeletedAtIsNil(), entcur.IDNEQ(r.ID),
		sameTenant(r.TenantID, entcur.TenantIDIsNil, entcur.TenantID)).Exist(ctx)
	return entry(r.ID, r.TenantID, "curriculum "+r.Code, r.DeletedAt, r.DeletedBy, r.DeleteReason, conflict), err
}

func (CurriculumTrash) topicEntry(ctx context.Context, r *ent.Topic) (trash.Entry, error) {
	conflict, err := client(ctx).Topic.Query().Where(enttopic.Code(r.Code), enttopic.CurriculumVersionID(r.CurriculumVersionID),
		enttopic.DeletedAtIsNil(), enttopic.IDNEQ(r.ID)).Exist(ctx)
	return entry(r.ID, r.TenantID, "topic "+r.Code, r.DeletedAt, r.DeletedBy, r.DeleteReason, conflict), err
}

func (CurriculumTrash) objectiveEntry(ctx context.Context, r *ent.LearningObjective) (trash.Entry, error) {
	conflict, err := client(ctx).LearningObjective.Query().Where(entlo.Code(r.Code), entlo.CurriculumVersionID(r.CurriculumVersionID),
		entlo.DeletedAtIsNil(), entlo.IDNEQ(r.ID)).Exist(ctx)
	return entry(r.ID, r.TenantID, "learning objective "+r.Code, r.DeletedAt, r.DeletedBy, r.DeleteReason, conflict), err
}

// DeletedEntries implements write.CurriculumTrashStore.
func (s CurriculumTrash) DeletedEntries(ctx context.Context, f port.TrashFilter) ([]trash.Entry, error) {
	c := client(ctx)
	limit := f.Limit + 1
	var out []trash.Entry

	cq := c.Curriculum.Query().Where(entcur.DeletedAtNotNil())
	tq := c.Topic.Query().Where(enttopic.DeletedAtNotNil())
	lq := c.LearningObjective.Query().Where(entlo.DeletedAtNotNil())
	if len(f.TenantIDs) > 0 {
		if f.PlatformToo {
			cq = cq.Where(entcur.Or(entcur.TenantIDIn(f.TenantIDs...), entcur.TenantIDIsNil()))
			tq = tq.Where(enttopic.Or(enttopic.TenantIDIn(f.TenantIDs...), enttopic.TenantIDIsNil()))
			lq = lq.Where(entlo.Or(entlo.TenantIDIn(f.TenantIDs...), entlo.TenantIDIsNil()))
		} else {
			cq = cq.Where(entcur.TenantIDIn(f.TenantIDs...))
			tq = tq.Where(enttopic.TenantIDIn(f.TenantIDs...))
			lq = lq.Where(entlo.TenantIDIn(f.TenantIDs...))
		}
	}
	if f.AfterAt != nil {
		cq = cq.Where(entcur.Or(entcur.DeletedAtLT(*f.AfterAt), entcur.And(entcur.DeletedAt(*f.AfterAt), entcur.IDLT(f.AfterID))))
		tq = tq.Where(enttopic.Or(enttopic.DeletedAtLT(*f.AfterAt), enttopic.And(enttopic.DeletedAt(*f.AfterAt), enttopic.IDLT(f.AfterID))))
		lq = lq.Where(entlo.Or(entlo.DeletedAtLT(*f.AfterAt), entlo.And(entlo.DeletedAt(*f.AfterAt), entlo.IDLT(f.AfterID))))
	}
	cs, err := cq.Order(entcur.ByDeletedAt(entsql.OrderDesc()), entcur.ByID(entsql.OrderDesc())).Limit(limit).All(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range cs {
		e, err := s.curriculumEntry(ctx, r)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	ts, err := tq.Order(enttopic.ByDeletedAt(entsql.OrderDesc()), enttopic.ByID(entsql.OrderDesc())).Limit(limit).All(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range ts {
		e, err := s.topicEntry(ctx, r)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	ls, err := lq.Order(entlo.ByDeletedAt(entsql.OrderDesc()), entlo.ByID(entsql.OrderDesc())).Limit(limit).All(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range ls {
		e, err := s.objectiveEntry(ctx, r)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	slices.SortFunc(out, func(a, b trash.Entry) int {
		if c := b.DeletedAt.Compare(a.DeletedAt); c != 0 {
			return c
		}
		return strings.Compare(b.ResourceID.String(), a.ResourceID.String())
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// RestoreEntry implements write.CurriculumTrashStore.
func (s CurriculumTrash) RestoreEntry(ctx context.Context, id uuid.UUID) (trash.Entry, error) {
	c := client(ctx)
	if r, err := c.Curriculum.Query().Where(entcur.ID(id)).Only(ctx); err == nil {
		e, err := s.curriculumEntry(ctx, r)
		if err != nil {
			return e, err
		}
		return e, restore(ctx, r.DeletedAt, e, func() error {
			return c.Curriculum.Update().Where(entcur.ID(id)).ClearDeletedAt().ClearDeletedBy().ClearDeleteReason().Exec(ctx)
		})
	} else if !ent.IsNotFound(err) {
		return trash.Entry{}, err
	}
	if r, err := c.Topic.Query().Where(enttopic.ID(id)).Only(ctx); err == nil {
		e, err := s.topicEntry(ctx, r)
		if err != nil {
			return e, err
		}
		return e, restore(ctx, r.DeletedAt, e, func() error {
			return c.Topic.Update().Where(enttopic.ID(id)).ClearDeletedAt().ClearDeletedBy().ClearDeleteReason().Exec(ctx)
		})
	} else if !ent.IsNotFound(err) {
		return trash.Entry{}, err
	}
	r, err := c.LearningObjective.Query().Where(entlo.ID(id)).Only(ctx)
	if err != nil {
		return trash.Entry{}, MapCurriculumErr(err)
	}
	e, err := s.objectiveEntry(ctx, r)
	if err != nil {
		return e, err
	}
	return e, restore(ctx, r.DeletedAt, e, func() error {
		return c.LearningObjective.Update().Where(entlo.ID(id)).ClearDeletedAt().ClearDeletedBy().ClearDeleteReason().Exec(ctx)
	})
}

// restore clears the deletion when the row is deleted and its unique key is free (Update().Where, not
// UpdateOneID: entx.Driver binds the transaction of ctx, F020-4).
func restore(_ context.Context, deletedAt *time.Time, e trash.Entry, clear func() error) error {
	if deletedAt == nil {
		return trash.ErrNotDeleted
	}
	if err := trash.CheckRestore(e); err != nil {
		return err
	}
	if err := clear(); err != nil {
		if ent.IsConstraintError(err) {
			return trash.ErrConflict
		}
		return MapCurriculumErr(err)
	}
	return nil
}
