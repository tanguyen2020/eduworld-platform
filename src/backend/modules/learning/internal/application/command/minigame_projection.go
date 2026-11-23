package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/plugins/minigame"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// MinigameSnapshotMaxBytes bounds the snapshot of a configuration version (CHECK of ref.minigame_config_versions).
const MinigameSnapshotMaxBytes = 65536

// ErrMinigameSnapshot is returned for a content.published minigame_config the projection refuses (DLQ).
var ErrMinigameSnapshot = errors.New("minigame config snapshot refused")

// MiniGameConfigSnapshot is the minigame_config block of a game content.published (item_type minigame_config).
type MiniGameConfigSnapshot struct {
	ConfigID    uuid.UUID
	VersionID   uuid.UUID
	Version     int
	Type        string
	GradeLevels []int
	Snapshot    json.RawMessage
	PublishedAt time.Time
	// SourceVersion orders the events of the producer (the release time in microseconds).
	SourceVersion int64
}

// UpsertMiniGameConfigSnapshotHandler is the typed interface of the command (ADR-0087).
type UpsertMiniGameConfigSnapshotHandler interface {
	Upsert(ctx context.Context, in MiniGameConfigSnapshot) (bool, error)
}

// UpsertMiniGameConfigSnapshot keeps the projection ref.minigame_config_versions (F073-AC-04, ADR-0043): each published
// version of a mini-game configuration of module game is stored once and never changed; a replayed event, or another
// event about a version already projected, applies nothing (TC-SVC-003). A snapshot the minigame plugin cannot play is
// refused (ErrMinigameSnapshot, the consumer dead-letters it). Platform data: called under the PlatformScope of the
// consumer learning.minigame_config_projection.
type UpsertMiniGameConfigSnapshot struct {
	Store write.MinigameConfigProjector
}

var _ UpsertMiniGameConfigSnapshotHandler = UpsertMiniGameConfigSnapshot{}

// Upsert implements UpsertMiniGameConfigSnapshotHandler.
func (h UpsertMiniGameConfigSnapshot) Upsert(ctx context.Context, in MiniGameConfigSnapshot) (applied bool, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.UpsertMiniGameConfigSnapshot")
	defer func() { done(err) }()
	refuse := func(why string) error { return fmt.Errorf("%w: %s", ErrMinigameSnapshot, why) }
	if in.ConfigID == uuid.Nil || in.VersionID == uuid.Nil || in.PublishedAt.IsZero() {
		return false, refuse("minigame_config_id, version_id and released_at are required")
	}
	if len(in.Snapshot) > MinigameSnapshotMaxBytes {
		return false, refuse("snapshot larger than 64 KiB")
	}
	rules, err := minigame.Parse(in.Snapshot)
	if err != nil {
		return false, refuse(err.Error())
	}
	sorted := slices.Sorted(slices.Values(in.GradeLevels))
	switch {
	case rules.Type != in.Type || !minigame.Valid(in.Type):
		return false, refuse("minigame_type differs from the snapshot")
	case rules.Version != in.Version:
		return false, refuse("version differs from the snapshot")
	case rules.MinigameConfigID != in.ConfigID || rules.MinigameConfigVersionID != in.VersionID:
		return false, refuse("ids differ from the snapshot")
	case len(sorted) == 0 || len(sorted) > 9 || len(slices.Compact(slices.Clone(sorted))) != len(sorted) || sorted[0] < 1 || sorted[len(sorted)-1] > 9:
		return false, refuse("grade_levels")
	}
	return h.Store.InsertMinigameConfigVersion(ctx, dto.MinigameConfigVersion{ID: in.VersionID, ConfigID: in.ConfigID, Version: in.Version,
		Type: in.Type, GradeLevels: sorted, Snapshot: in.Snapshot, PublishedAt: in.PublishedAt.UTC(), SourceVersion: in.SourceVersion})
}
