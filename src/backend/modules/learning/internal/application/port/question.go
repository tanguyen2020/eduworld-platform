package port

import (
	"encoding/json"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

// QuestionDoc is the authoring JSON of a question version checked against question.schema.json.
type QuestionDoc struct {
	Type          string
	Stem          json.RawMessage
	RenderPayload json.RawMessage
	AnswerKey     json.RawMessage
	ScoringRule   json.RawMessage
	Explanation   json.RawMessage
	Hints         []json.RawMessage
}

// QuestionSchema validates questions and responses against specs/schemas/question.schema.json (F022, T006): the
// structural layer before the semantic validators of the domain.
type QuestionSchema interface {
	ValidateAuthoring(d QuestionDoc) contentmodel.Violations
	ValidateResponse(questionType string, response json.RawMessage) error
}
