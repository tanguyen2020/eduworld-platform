package command

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/scope"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/curriculum"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Patch is one field of a JSON merge-patch: Set false = absent (unchanged); Set with Value nil = null (cleared).
type Patch[T any] struct {
	Set   bool
	Value *T
}

// apply returns the patched value of cur.
func (p Patch[T]) apply(cur *T) *T {
	if !p.Set {
		return cur
	}
	return ptrClone(p.Value)
}

// VersionInput is the editable data of a version (CurriculumVersionInput).
type VersionInput struct {
	Label           string
	EffectiveFrom   *time.Time
	EffectiveTo     *time.Time
	ChangeNote      *string
	ContentLanguage string
}

func (in VersionInput) validate() error {
	if err := curriculum.CheckVersionLabel(in.Label); err != nil {
		return invalid(err)
	}
	if err := curriculum.CheckEffective(in.EffectiveFrom, in.EffectiveTo); err != nil {
		return invalid(err)
	}
	if in.ChangeNote != nil && utf8.RuneCountInString(*in.ChangeNote) > curriculum.MaxChangeNoteLen {
		return problem.Invalid("change_note", problem.SubMaxLength, "is longer than 1000 characters")
	}
	if in.ContentLanguage != string(contentmodel.LangVI) && in.ContentLanguage != string(contentmodel.LangEN) {
		return problem.Invalid("content_language", problem.SubInvalid, "must be vi or en")
	}
	return nil
}

// CreateCurriculumInput is the request of createCurriculum.
type CreateCurriculumInput struct {
	Code      string
	Name      map[string]string
	Framework *string
	TenantID  *uuid.UUID // System Admin creating a tenant curriculum; empty = platform for platform staff
	Version   VersionInput
}

// CreateCurriculum creates a curriculum and its version 1 in draft (F021-AC-01): a tenant user creates for its
// session tenant, platform staff a platform curriculum, System Admin a tenant one with tenant_id.
func (h Curriculum) CreateCurriculum(ctx context.Context, in CreateCurriculumInput) (out dto.Curriculum, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.CreateCurriculum")
	defer func() { done(err) }()
	p, err := authorize(ctx, authz.PermCurriculumWrite)
	if err != nil {
		return out, err
	}
	ownerScope := contentmodel.OwnerPlatform
	o := contentmodel.Owner{Scope: contentmodel.OwnerPlatform}
	switch {
	case p.TenantID != nil:
		ownerScope, o = contentmodel.OwnerTenant, contentmodel.Owner{Scope: contentmodel.OwnerTenant, TenantID: ptrClone(p.TenantID)}
	case in.TenantID != nil:
		ownerScope, o = contentmodel.OwnerTenant, contentmodel.Owner{Scope: contentmodel.OwnerTenant, TenantID: ptrClone(in.TenantID)}
	}
	sc, err := scope.ForWrite(p, ownerScope, in.TenantID)
	if err != nil {
		return out, err
	}
	ctx = tenancy.WithScope(ctx, sc)
	if err := curriculum.CheckCurriculumCode("code", in.Code); err != nil {
		return out, invalid(err)
	}
	if err := curriculum.CheckLearningName("name", in.Name); err != nil {
		return out, invalid(err)
	}
	if in.Version.Label == "" {
		in.Version.Label = "1.0"
	}
	if in.Version.ContentLanguage == "" {
		in.Version.ContentLanguage = string(contentmodel.LangVI)
	}
	if err := in.Version.validate(); err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		c, v, err := h.createCurriculum(ctx, p.UserID, o, in)
		if err != nil {
			return err
		}
		c.Latest = refOf(v)
		out = c
		return idempotency.SetResult(ctx, dto.CurriculumJSON(c))
	})
	return out, err
}

// createCurriculum inserts the curriculum and version 1 (shared with the import).
func (h Curriculum) createCurriculum(ctx context.Context, actor uuid.UUID, o contentmodel.Owner, in CreateCurriculumInput) (dto.Curriculum, dto.CurriculumVersion, error) {
	if dup, err := h.Store.CurriculumByCode(ctx, o, in.Code); err != nil {
		return dto.Curriculum{}, dto.CurriculumVersion{}, err
	} else if dup != nil {
		return dto.Curriculum{}, dto.CurriculumVersion{}, problem.New(problem.Conflict, "a curriculum with this code already exists")
	}
	c := dto.Curriculum{ID: ids.New(), Code: in.Code, Name: in.Name, Framework: in.Framework, Owner: o, LatestVersionNo: 1}
	if err := h.Store.InsertCurriculum(ctx, c); err != nil {
		return c, dto.CurriculumVersion{}, err
	}
	v := dto.CurriculumVersion{ID: ids.New(), CurriculumID: c.ID, VersionNo: 1, Label: in.Version.Label, Status: contentmodel.StatusDraft,
		EffectiveFrom: in.Version.EffectiveFrom, EffectiveTo: in.Version.EffectiveTo, ChangeNote: in.Version.ChangeNote,
		ContentLanguage: in.Version.ContentLanguage, AuthorID: actor, Owner: o}
	if err := h.Store.InsertVersion(ctx, v); err != nil {
		return c, v, err
	}
	if err := h.recordCurriculum(ctx, o, audit.Event{Action: ActionContentVersionCreated, TargetID: c.ID.String(),
		Metadata: map[string]any{"curriculum_version_id": v.ID.String(), "version": 1}}); err != nil {
		return c, v, err
	}
	loaded, err := h.Store.Curriculum(ctx, c.ID)
	if err != nil {
		return c, v, err
	}
	return loaded, v, nil
}

// UpdateCurriculumInput is the merge-patch of updateCurriculum.
type UpdateCurriculumInput struct {
	ID        uuid.UUID
	IfMatch   int64
	Name      map[string]string // nil = unchanged
	Framework Patch[string]
}

// UpdateCurriculum changes the name or framework of a curriculum (If-Match, F021-AC-09).
func (h Curriculum) UpdateCurriculum(ctx context.Context, in UpdateCurriculumInput) (out dto.Curriculum, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.UpdateCurriculum")
	defer func() { done(err) }()
	p, err := authorize(ctx, authz.PermCurriculumWrite)
	if err != nil {
		return out, err
	}
	if in.Name != nil {
		if err := curriculum.CheckLearningName("name", in.Name); err != nil {
			return out, invalid(err)
		}
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		c, err := h.Store.Curriculum(ctx, in.ID)
		if err != nil {
			return err
		}
		if err := writable(ctx, p, authz.PermCurriculumWrite, c.Owner); err != nil {
			return err
		}
		if in.Name != nil {
			c.Name = in.Name
		}
		c.Framework = in.Framework.apply(c.Framework)
		if c, err = h.Store.UpdateCurriculum(ctx, c, in.IfMatch); err != nil {
			return err
		}
		out, err = h.withRefs(ctx, c)
		return err
	})
	return out, err
}

// DeleteCurriculumInput is the request of deleteCurriculum.
type DeleteCurriculumInput struct {
	ID      uuid.UUID
	IfMatch int64
	Reason  string
}

// DeleteCurriculum soft-deletes a curriculum that never had a published version (F021-AC-10): Thùng rác
// (resource_type curriculum), audit content.deleted and curriculum.deleted; otherwise 409 (archive instead).
func (h Curriculum) DeleteCurriculum(ctx context.Context, in DeleteCurriculumInput) (err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.DeleteCurriculum")
	defer func() { done(err) }()
	p, err := authorize(ctx, authz.PermCurriculumWrite)
	if err != nil {
		return err
	}
	if utf8.RuneCountInString(in.Reason) > 500 {
		return problem.Invalid("reason", problem.SubMaxLength, "is longer than 500 characters")
	}
	handler, ok := h.Resources.Get(contentmodel.ResourceCurriculum)
	if !ok {
		return problem.New(problem.Internal, "no curriculum resource handler")
	}
	return h.UoW.Do(ctx, func(ctx context.Context) error {
		c, err := h.Store.Curriculum(ctx, in.ID)
		if err != nil {
			return err
		}
		if err := writable(ctx, p, authz.PermCurriculumWrite, c.Owner); err != nil {
			return err
		}
		if err := checkRowVersion(c.RowVersion, in.IfMatch); err != nil {
			return err
		}
		vs, err := h.Store.Versions(ctx, c.ID)
		if err != nil {
			return err
		}
		for _, v := range vs {
			if v.Status.Immutable() {
				return problem.New(problem.Conflict, "a curriculum with a published version can only be archived")
			}
		}
		if _, err := handler.SoftDelete(ctx, c.ID, in.Reason); err != nil {
			if errors.Is(err, contentmodel.ErrItemPublished) {
				return problem.New(problem.Conflict, "a curriculum with a current release can only be archived")
			}
			return err
		}
		if err := h.recordCurriculum(ctx, c.Owner, audit.Event{Action: ActionContentDeleted, TargetID: c.ID.String(), Reason: in.Reason,
			Metadata: map[string]any{"resource_type": string(contentmodel.ResourceCurriculum)}}); err != nil {
			return err
		}
		by := p.UserID
		return h.trash().publishResource(ctx, contentmodel.ResourceCurriculum, "deleted", ResourceEventPayload{
			ResourceType: string(contentmodel.ResourceCurriculum), ResourceID: c.ID, TenantID: c.Owner.TenantID, DeletedBy: &by, Reason: in.Reason})
	})
}

// UpdateVersionDraftInput is the merge-patch of updateCurriculumVersionDraft.
type UpdateVersionDraftInput struct {
	CurriculumID  uuid.UUID
	VersionNo     int
	IfMatch       int64
	Label         *string
	EffectiveFrom Patch[time.Time]
	EffectiveTo   Patch[time.Time]
	ChangeNote    *string
	SubjectGrades *[]dto.SubjectGrade // nil = unchanged; replaces the list
}

// UpdateVersionDraft edits a draft version (F021-AC-07: other statuses → 409 CONTENT_NOT_EDITABLE).
func (h Curriculum) UpdateVersionDraft(ctx context.Context, in UpdateVersionDraftInput) (out dto.CurriculumVersion, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.UpdateCurriculumVersionDraft")
	defer func() { done(err) }()
	p, err := authorize(ctx, authz.PermCurriculumWrite)
	if err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		v, err := h.Store.Version(ctx, in.CurriculumID, in.VersionNo)
		if err != nil {
			return err
		}
		if v, err = h.draftVersion(ctx, p, v.ID); err != nil {
			return err
		}
		if err := checkRowVersion(v.RowVersion, in.IfMatch); err != nil {
			return err
		}
		if in.Label != nil {
			v.Label = *in.Label
		}
		v.EffectiveFrom = in.EffectiveFrom.apply(v.EffectiveFrom)
		v.EffectiveTo = in.EffectiveTo.apply(v.EffectiveTo)
		if in.ChangeNote != nil {
			v.ChangeNote = in.ChangeNote
		}
		if err := (VersionInput{Label: v.Label, EffectiveFrom: v.EffectiveFrom, EffectiveTo: v.EffectiveTo, ChangeNote: v.ChangeNote,
			ContentLanguage: v.ContentLanguage}).validate(); err != nil {
			return err
		}
		if in.SubjectGrades != nil {
			if err := h.replaceSubjectGrades(ctx, v, *in.SubjectGrades); err != nil {
				return err
			}
		}
		out, err = h.Store.UpdateVersion(ctx, v, in.IfMatch)
		return err
	})
	return out, err
}

// replaceSubjectGrades makes the subject × grade pairs of a draft version equal to want; a pair that still has
// topics cannot be removed (422).
func (h Curriculum) replaceSubjectGrades(ctx context.Context, v dto.CurriculumVersion, want []dto.SubjectGrade) error {
	have, err := h.Store.SubjectGrades(ctx, v.ID)
	if err != nil {
		return err
	}
	key := func(s uuid.UUID, g int) string { return s.String() + "/" + strconv.Itoa(g) }
	wanted := map[string]bool{}
	for i, sg := range want {
		if sg.GradeLevel < 1 || sg.GradeLevel > 9 {
			return problem.Invalid("subject_grades["+strconv.Itoa(i)+"].grade_level", problem.SubInvalid, "must be 1–9")
		}
		if _, err := h.Catalog.Subject(ctx, sg.SubjectID); err != nil {
			return problem.Invalid("subject_grades["+strconv.Itoa(i)+"].subject_id", problem.SubInvalid, "is not a subject of the catalog")
		}
		wanted[key(sg.SubjectID, sg.GradeLevel)] = true
	}
	present := map[string]bool{}
	topics, err := h.Store.Topics(ctx, v.ID)
	if err != nil {
		return err
	}
	for _, sg := range have {
		k := key(sg.SubjectID, sg.GradeLevel)
		present[k] = true
		if wanted[k] {
			continue
		}
		for _, t := range topics {
			if t.SubjectID == sg.SubjectID && t.GradeLevel == sg.GradeLevel {
				return problem.Invalid("subject_grades", problem.SubInvalid, "a subject × grade with topics cannot be removed")
			}
		}
		if err := h.Store.DeleteSubjectGrade(ctx, sg.ID); err != nil {
			return err
		}
	}
	for _, sg := range want {
		k := key(sg.SubjectID, sg.GradeLevel)
		if present[k] {
			continue
		}
		present[k] = true
		sg.ID = ids.New()
		if err := h.Store.InsertSubjectGrade(ctx, v, sg); err != nil {
			return err
		}
	}
	return nil
}

// ensureSubjectGrade adds the pair of a new topic when the version does not have it yet (F021 clarification).
func (h Curriculum) ensureSubjectGrade(ctx context.Context, v dto.CurriculumVersion, subjectID uuid.UUID, grade int) error {
	have, err := h.Store.SubjectGrades(ctx, v.ID)
	if err != nil {
		return err
	}
	for _, sg := range have {
		if sg.SubjectID == subjectID && sg.GradeLevel == grade {
			return nil
		}
	}
	return h.Store.InsertSubjectGrade(ctx, v, dto.SubjectGrade{ID: ids.New(), SubjectID: subjectID, GradeLevel: grade})
}

// ---------- subjects (platform catalog)

// SubjectInput is the request of createSubject.
type SubjectInput struct {
	Code        string
	Name        map[string]string
	GradeLevels []int
	IconKey     *string
}

func checkGrades(gs []int) error {
	if len(gs) == 0 || len(gs) > 9 {
		return problem.Invalid("grade_levels", problem.SubInvalid, "must hold 1–9 grades")
	}
	for _, g := range gs {
		if g < 1 || g > 9 {
			return problem.Invalid("grade_levels", problem.SubInvalid, "grades must be 1–9")
		}
	}
	return nil
}

// CreateSubject adds a subject to the platform catalog (platform staff only, F021-AC-06).
func (h Curriculum) CreateSubject(ctx context.Context, in SubjectInput) (out dto.Subject, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.CreateSubject")
	defer func() { done(err) }()
	p, err := authorize(ctx, authz.PermCurriculumWrite)
	if err != nil {
		return out, err
	}
	if err := platformOnly(ctx, p); err != nil {
		return out, err
	}
	if err := curriculum.CheckSubjectCode(in.Code); err != nil {
		return out, invalid(err)
	}
	if err := curriculum.CheckLearningName("name", in.Name); err != nil {
		return out, invalid(err)
	}
	if err := checkGrades(in.GradeLevels); err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		s := dto.Subject{ID: ids.New(), Code: in.Code, Name: in.Name, GradeLevels: slices.Clone(in.GradeLevels), IconKey: in.IconKey}
		if err := h.Catalog.InsertSubject(ctx, s); err != nil {
			return err
		}
		if out, err = h.Catalog.Subject(ctx, s.ID); err != nil {
			return err
		}
		return idempotency.SetResult(ctx, dto.SubjectJSON(out))
	})
	return out, err
}

// UpdateSubjectInput is the merge-patch of updateSubject.
type UpdateSubjectInput struct {
	ID          uuid.UUID
	IfMatch     int64
	Name        map[string]string
	GradeLevels []int
	IconKey     Patch[string]
}

// UpdateSubject changes a subject of the platform catalog (platform staff only).
func (h Curriculum) UpdateSubject(ctx context.Context, in UpdateSubjectInput) (out dto.Subject, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.UpdateSubject")
	defer func() { done(err) }()
	p, err := authorize(ctx, authz.PermCurriculumWrite)
	if err != nil {
		return out, err
	}
	if err := platformOnly(ctx, p); err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		s, err := h.Catalog.Subject(ctx, in.ID)
		if err != nil {
			return err
		}
		if in.Name != nil {
			if err := curriculum.CheckLearningName("name", in.Name); err != nil {
				return invalid(err)
			}
			s.Name = in.Name
		}
		if in.GradeLevels != nil {
			if err := checkGrades(in.GradeLevels); err != nil {
				return err
			}
			s.GradeLevels = in.GradeLevels
		}
		s.IconKey = in.IconKey.apply(s.IconKey)
		out, err = h.Catalog.UpdateSubject(ctx, s, in.IfMatch)
		return err
	})
	return out, err
}
