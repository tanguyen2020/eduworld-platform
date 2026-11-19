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

// ContentSchema validates the type-specific documents of the composed content types (F024) against the published
// JSON Schemas: question set definitions, blueprint definitions, lesson/material blocks.
type ContentSchema interface {
	ValidateSetDefinition(def json.RawMessage) contentmodel.Violations
	ValidateBlueprintDefinition(def json.RawMessage) contentmodel.Violations
	ValidateBlocks(blocks json.RawMessage) contentmodel.Violations
}

// ActivitySchema validates the config of an activity version against the JSON Schema of its activity_type
// (specs/schemas/activities, ADR-0048): content, interaction, evaluation, feedback, completion_rule.
type ActivitySchema interface {
	ValidateConfig(activityType string, config json.RawMessage) contentmodel.Violations
}
