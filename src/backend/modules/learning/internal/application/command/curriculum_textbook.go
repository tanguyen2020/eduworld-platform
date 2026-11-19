package command

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/curriculum"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Textbook mapping (F021 US7, CURRICULUM_MODEL §6): series, volumes and sections are platform metadata (no text of
// the book is stored); a section is mapped to platform LOs. Written by platform staff only.

// SeriesInput is the request of createTextbookSeries.
type SeriesInput struct {
	Code      string
	Name      string
	Publisher *string
}

// CreateSeries adds a textbook series.
func (h Curriculum) CreateSeries(ctx context.Context, in SeriesInput) (out dto.TextbookSeries, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.CreateTextbookSeries")
	defer func() { done(err) }()
	p, err := authorize(ctx, authz.PermCurriculumWrite)
	if err != nil {
		return out, err
	}
	if err := platformOnly(ctx, p); err != nil {
		return out, err
	}
	if err := curriculum.CheckCurriculumCode("code", in.Code); err != nil {
		return out, invalid(err)
	}
	if err := curriculum.CheckName("name", in.Name); err != nil {
		return out, invalid(err)
	}
	if in.Publisher != nil {
		if err := curriculum.CheckName("publisher", *in.Publisher); err != nil {
			return out, invalid(err)
		}
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		s := dto.TextbookSeries{ID: ids.New(), Code: in.Code, Name: in.Name, Publisher: in.Publisher}
		if err := h.Catalog.InsertSeries(ctx, s); err != nil {
			return err
		}
		out = s
		return idempotency.SetResult(ctx, dto.SeriesJSON(s))
	})
	return out, err
}

// TextbookInput is the request of createTextbook.
type TextbookInput struct {
	SeriesID   uuid.UUID
	SubjectID  uuid.UUID
	GradeLevel int
	Volume     int
}

// CreateTextbook adds a volume of a series for a subject × grade.
func (h Curriculum) CreateTextbook(ctx context.Context, in TextbookInput) (out dto.Textbook, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.CreateTextbook")
	defer func() { done(err) }()
	p, err := authorize(ctx, authz.PermCurriculumWrite)
	if err != nil {
		return out, err
	}
	if err := platformOnly(ctx, p); err != nil {
		return out, err
	}
	if in.GradeLevel < 1 || in.GradeLevel > 9 || in.Volume < 1 || in.Volume > 9 {
		return out, problem.Invalid("grade_level", problem.SubInvalid, "grade and volume must be 1–9")
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if _, err := h.Catalog.Series(ctx, in.SeriesID); err != nil {
			return err
		}
		if _, err := h.Catalog.Subject(ctx, in.SubjectID); err != nil {
			return problem.Invalid("subject_id", problem.SubInvalid, "is not a subject of the catalog")
		}
		t := dto.Textbook{ID: ids.New(), SeriesID: in.SeriesID, SubjectID: in.SubjectID, GradeLevel: in.GradeLevel, Volume: in.Volume}
		if err := h.Catalog.InsertTextbook(ctx, t); err != nil {
			return err
		}
		out = t
		return idempotency.SetResult(ctx, dto.TextbookJSON(t))
	})
	return out, err
}

// SectionInput is the request of createTextbookSection.
type SectionInput struct {
	TextbookID uuid.UUID
	Chapter    *string
	LessonNo   int
	Title      string
	PageRange  *string
}

func (in SectionInput) validate() error {
	if in.LessonNo < 1 || in.LessonNo > 999 {
		return problem.Invalid("lesson_no", problem.SubInvalid, "must be 1–999")
	}
	if err := curriculum.CheckName("title", in.Title); err != nil {
		return invalid(err)
	}
	if in.Chapter != nil && len([]rune(*in.Chapter)) > 128 {
		return problem.Invalid("chapter", problem.SubMaxLength, "is longer than 128 characters")
	}
	if in.PageRange != nil {
		if err := curriculum.CheckPageRange(*in.PageRange); err != nil {
			return invalid(err)
		}
	}
	return nil
}

// CreateSection adds a lesson of a textbook (metadata only, F021-AC-28).
func (h Curriculum) CreateSection(ctx context.Context, in SectionInput) (out dto.TextbookSection, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.CreateTextbookSection")
	defer func() { done(err) }()
	p, err := authorize(ctx, authz.PermCurriculumWrite)
	if err != nil {
		return out, err
	}
	if err := platformOnly(ctx, p); err != nil {
		return out, err
	}
	if err := in.validate(); err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if _, err := h.Catalog.Textbook(ctx, in.TextbookID); err != nil {
			return err
		}
		s := dto.TextbookSection{ID: ids.New(), TextbookID: in.TextbookID, Chapter: in.Chapter, LessonNo: in.LessonNo, Title: in.Title, PageRange: in.PageRange}
		if err := h.Catalog.InsertSection(ctx, s); err != nil {
			return err
		}
		if out, err = h.Catalog.Section(ctx, s.ID); err != nil {
			return err
		}
		return idempotency.SetResult(ctx, dto.SectionJSON(out))
	})
	return out, err
}

// UpdateSectionInput is the merge-patch of updateTextbookSection.
type UpdateSectionInput struct {
	ID        uuid.UUID
	IfMatch   int64
	Chapter   Patch[string]
	LessonNo  *int
	Title     *string
	PageRange Patch[string]
}

// UpdateSection edits a textbook section (If-Match).
func (h Curriculum) UpdateSection(ctx context.Context, in UpdateSectionInput) (out dto.TextbookSection, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.UpdateTextbookSection")
	defer func() { done(err) }()
	p, err := authorize(ctx, authz.PermCurriculumWrite)
	if err != nil {
		return out, err
	}
	if err := platformOnly(ctx, p); err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		s, err := h.Catalog.Section(ctx, in.ID)
		if err != nil {
			return err
		}
		s.Chapter = in.Chapter.apply(s.Chapter)
		s.PageRange = in.PageRange.apply(s.PageRange)
		if in.LessonNo != nil {
			s.LessonNo = *in.LessonNo
		}
		if in.Title != nil {
			s.Title = *in.Title
		}
		if err := (SectionInput{TextbookID: s.TextbookID, Chapter: s.Chapter, LessonNo: s.LessonNo, Title: s.Title, PageRange: s.PageRange}).validate(); err != nil {
			return err
		}
		out, err = h.Catalog.UpdateSection(ctx, s, in.IfMatch)
		return err
	})
	return out, err
}

// SectionObjectivesInput is the request of putTextbookSectionObjectives.
type SectionObjectivesInput struct {
	SectionID    uuid.UUID
	ObjectiveIDs []uuid.UUID
}

// PutSectionObjectives replaces the LOs mapped to a section (n–n); every LO is a live platform LO.
func (h Curriculum) PutSectionObjectives(ctx context.Context, in SectionObjectivesInput) (out dto.TextbookSection, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.PutTextbookSectionObjectives")
	defer func() { done(err) }()
	p, err := authorize(ctx, authz.PermCurriculumWrite)
	if err != nil {
		return out, err
	}
	if err := platformOnly(ctx, p); err != nil {
		return out, err
	}
	if len(in.ObjectiveIDs) > 50 {
		return out, problem.Invalid("objective_ids", problem.SubInvalid, "at most 50 objectives")
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if _, err := h.Catalog.Section(ctx, in.SectionID); err != nil {
			return err
		}
		seen := map[uuid.UUID]bool{}
		for _, id := range in.ObjectiveIDs {
			if seen[id] {
				return problem.Invalid("objective_ids", problem.SubInvalid, "duplicate objective")
			}
			seen[id] = true
			if _, err := h.Store.Objective(ctx, id); err != nil {
				return problem.Invalid("objective_ids", problem.SubInvalid, "must be live platform objectives")
			}
		}
		if err := h.Catalog.ReplaceSectionObjectives(ctx, in.SectionID, in.ObjectiveIDs); err != nil {
			return err
		}
		if out, err = h.Catalog.Section(ctx, in.SectionID); err != nil {
			return err
		}
		return idempotency.SetResult(ctx, dto.SectionJSON(out))
	})
	return out, err
}
