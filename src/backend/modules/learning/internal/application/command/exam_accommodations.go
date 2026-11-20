package command

import (
	"context"
	"encoding/json"
	"strconv"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/exam"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// AccommodationsInput is the body of putExamAccommodations (ExamAccommodationsUpdate, F055 delta D2).
type AccommodationsInput struct {
	Accommodations []exam.Accommodation `json:"accommodations"`
	Reason         string               `json:"reason"`
}

// PutExamAccommodations replaces the personal extra times of a session (putExamAccommodations, FR-EXM-009, F055-AC-18):
// lms.assignment.write in the session's school and subject, If-Match on the session version, only before the session
// closes (409 CONFLICT), students of the session's audience only (422), reason 3–500 characters; each participation
// gets its new effective duration (attempts already started keep their deadline_at), audit
// exam_session.accommodations_updated; the Idempotency-Key replay returns the stored result.
func (h Exams) PutExamAccommodations(ctx context.Context, id uuid.UUID, ifMatch int64, in AccommodationsInput) (out dto.ExamSessionView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.PutExamAccommodations")
	defer func() { done(err) }()
	p, err := staffOf(ctx)
	if err != nil {
		return out, err
	}
	if n := utf8.RuneCountInString(in.Reason); n < 3 || n > 500 {
		return out, problem.Invalid("/reason", problem.SubInvalid, "must have 3 to 500 characters")
	}
	if in.Accommodations == nil {
		return out, problem.Invalid("/accommodations", problem.SubRequired, "is required")
	}
	if err := examViolations(exam.ValidateAccommodations(in.Accommodations)); err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		s, err := h.Store.Session(ctx, id, true)
		if err != nil {
			return err
		}
		if s == nil {
			return problem.New(problem.NotFound, "")
		}
		if err := AuthorizeLMS(ctx, p, authz.PermLmsAssignmentWrite, s.SchoolID, &s.SubjectID); err != nil {
			return err
		}
		if s.Version != ifMatch {
			return problem.New(problem.VersionMismatch, "")
		}
		if !exam.CanChangeAccommodations(exam.Status(s.Status)) {
			return problem.New(problem.Conflict, "extra time changes only before the exam session closes")
		}
		var before []exam.Accommodation
		_ = json.Unmarshal(s.Accommodations, &before)
		kept := map[uuid.UUID]bool{}
		for i, a := range in.Accommodations {
			ok, err := h.Store.SetParticipationTime(ctx, s.ID, a.StudentID, a.ExtraTimePercent, exam.EffectiveDuration(s.DurationSec, a.ExtraTimePercent))
			if err != nil {
				return err
			}
			if !ok {
				return problem.Invalid("/accommodations/"+strconv.Itoa(i)+"/student_id", problem.SubInvalid, "is not a student of the session")
			}
			kept[a.StudentID] = true
		}
		for _, a := range before {
			if kept[a.StudentID] {
				continue
			}
			if _, err := h.Store.SetParticipationTime(ctx, s.ID, a.StudentID, 0, s.DurationSec); err != nil {
				return err
			}
		}
		acc, _ := json.Marshal(nonNilAccommodations(in.Accommodations))
		ok, err := h.Store.SetAccommodations(ctx, s.ID, acc, s.Version)
		if err != nil {
			return err
		}
		if !ok {
			return problem.New(problem.VersionMismatch, "")
		}
		s.Accommodations, s.Version = acc, s.Version+1
		if err := h.audit(ctx, "exam_session.accommodations_updated", *s, in.Reason, map[string]any{"student_count": len(in.Accommodations),
			"previous_count": len(before)}); err != nil {
			return err
		}
		out = ExamSessionViewOf(*s, h.now())
		return idempotency.SetResult(ctx, out)
	})
	return out, deniedOutside(ctx, err)
}
