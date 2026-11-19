package port

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/selection"
)

// QuestionDelivery is the question bank seen by the attempt engine (F022 query.Delivery, QUESTION_BANK §1): selection
// of published questions, render DTOs without answers, server-side grading and the answer-side feedback given after
// grading. Every call reads in the tenant scope of the student.
type QuestionDelivery interface {
	Select(ctx context.Context, req selection.Request) ([]selection.Candidate, error)
	Render(ctx context.Context, versionIDs []uuid.UUID, seed uint64) ([]question.Rendered, []question.Presentation, error)
	Grade(ctx context.Context, versionID uuid.UUID, pres question.Presentation, response json.RawMessage) (question.GradeResult, error)
	Explanation(ctx context.Context, versionID uuid.UUID) (json.RawMessage, error)
	OptionFeedback(ctx context.Context, versionID uuid.UUID) (map[string]question.OptionFeedback, error)
}

// StudentDirectory tells where a student studies (projection ref.students of F020): school and grade, nil when the
// projection has no row yet.
type StudentDirectory interface {
	StudentSchool(ctx context.Context, studentID uuid.UUID) (*uuid.UUID, *int, error)
}
