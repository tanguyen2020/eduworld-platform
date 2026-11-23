package read

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/selection"
)

// MinigameConfigReadModel reads the projection ref.minigame_config_versions (views of the mini-game attempts, F073);
// nil when the version is not projected.
type MinigameConfigReadModel interface {
	MinigameConfigVersion(ctx context.Context, versionID uuid.UUID) (*dto.MinigameConfigVersion, error)
}

// MinigameSourceReadModel reads what a question source of a mini-game configuration may return in the scope of ctx
// (validateMinigameQuestionSource under the PlatformScope of a publish, F073-AC-02): the current published releases
// of the pool and the types of published question versions — metadata only, never a stem, an answer or a scoring rule.
// The delivery view serves the students of a tenant session; this model also reads platform content without a tenant.
type MinigameSourceReadModel interface {
	Pool(ctx context.Context, f PoolFilter) ([]selection.Candidate, error)
	QuestionTypes(ctx context.Context, versionIDs []uuid.UUID) (map[uuid.UUID]string, error)
}
