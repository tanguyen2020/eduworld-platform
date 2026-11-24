package port

import (
	"context"

	"github.com/google/uuid"
)

// LearningRef is one learning content reference of a game config version (activity, question_set, lesson).
type LearningRef struct {
	ContentType string
	ContentID   uuid.UUID
	Version     *int
}

// LearningContentCheck asks module learning which referenced platform learning content is released
// (checkPublishedLearningContent, F078-AC-07): one bool per reference, in order. ErrLearningUnavailable when module
// learning cannot answer (503).
type LearningContentCheck interface {
	PublishedContent(ctx context.Context, refs []LearningRef) ([]bool, error)
}
