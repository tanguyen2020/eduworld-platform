package read

import (
	"context"

	"github.com/google/uuid"
)

// PublishedRef is one learning content reference of a game config (checkPublishedLearningContent, F078-AC-07).
type PublishedRef struct {
	ContentType string // activity, question_set, lesson
	ContentID   uuid.UUID
	Version     *int
}

// PublishedContentReadModel reports which platform learning content is released (current pointer, live row).
type PublishedContentReadModel interface {
	// Published returns, for each reference, whether a platform item has a release pointer and, when Version is set,
	// whether that version is published.
	Published(ctx context.Context, refs []PublishedRef) ([]bool, error)
}
