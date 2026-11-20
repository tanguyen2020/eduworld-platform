package write

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/catalog"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/profile"
)

// ErrNicknameTaken: another live profile of the tenant holds the nickname (unique index game_profiles_nickname_uq).
var ErrNicknameTaken = errors.New("game profile: nickname already used in the tenant")

// ErrProfileConflict: the profile changed since it was loaded (optimistic version).
var ErrProfileConflict = errors.New("game profile: concurrent update")

// NicknameChange is one row of game.nickname_changes (append-only).
type NicknameChange struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	StudentID uuid.UUID
	Old       *string
	New       string
	Reason    string
	ChangedBy uuid.UUID
	ChangedAt time.Time
}

// ProfileRepository persists the GameProfile aggregate (game.game_profiles) and its nickname history under the
// TenantScope of the transaction, with explicit tenant and deleted_at conditions (AGENTS R27).
type ProfileRepository interface {
	// Ensure inserts p when the student has no profile row yet (EnsureGameProfile); with login set it records
	// last_game_login_at on an existing live profile. A soft-deleted profile is left as is (restore is identity's).
	Ensure(ctx context.Context, p *profile.Profile, login *time.Time) error
	// Get returns the live profile of student in tenant locked FOR UPDATE, nil when none.
	Get(ctx context.Context, tenant, student uuid.UUID) (*profile.Profile, error)
	// Save writes nickname, avatar, settings and language of p (version check, version+1 on p). A nickname used by
	// another live profile of the tenant is ErrNicknameTaken.
	Save(ctx context.Context, p *profile.Profile) error
	// AppendNicknameChange appends one history row.
	AppendNicknameChange(ctx context.Context, c NicknameChange) error
	// PlayerChangesSince counts the player_change rows of student since since.
	PlayerChangesSince(ctx context.Context, tenant, student uuid.UUID, since time.Time) (int, error)
	// SoftDelete soft-deletes the live profile and the saved outfits of student (student cleanup, ADR-0032).
	SoftDelete(ctx context.Context, tenant, student uuid.UUID, reason string, at time.Time) (bool, error)
	// Restore undoes a SoftDelete of reason; a nickname taken meanwhile by another live profile is cleared (the
	// student chooses again: setup_required). Returns whether a profile was restored.
	Restore(ctx context.Context, tenant, student uuid.UUID, reason string) (bool, error)
	// Anonymize replaces the nickname by anon and removes the custom avatar (anonymizeUserData); avatar_created_at
	// and level stay for statistics. Returns whether a profile changed.
	Anonymize(ctx context.Context, tenant, student uuid.UUID, anon string) (bool, error)
	// Reassign moves the live profile of from to into (mergeUserData when into has none); false when from has none.
	Reassign(ctx context.Context, tenant, from, into uuid.UUID) (bool, error)
}

// CatalogReader reads the platform content F070 checks against inside the write transaction.
type CatalogReader interface {
	// Current returns the published character catalog, nil when none is published yet.
	Current(ctx context.Context) (*catalog.Catalog, error)
	// Items returns the current definition (kind, equip_slot) of the known item codes among ids.
	Items(ctx context.Context, ids []string) (map[string]profile.Item, error)
}

// OwnershipReader reads the inventory of a student (game.inventory_items, written by F075).
type OwnershipReader interface {
	// Owned returns the codes among ids the student owns in tenant.
	Owned(ctx context.Context, tenant, student uuid.UUID, ids []string) (map[string]bool, error)
}

// PresetRepository persists the saved outfits (game.avatar_presets).
type PresetRepository interface {
	// Upsert writes the outfit of slot (overwrites the slot) and returns its updated_at.
	Upsert(ctx context.Context, tenant, student uuid.UUID, slot int, name *string, cfg []byte, at time.Time) error
	// MoveStudent moves the live outfits of from to into in tenant where into has none in the slot; the rest of
	// from is soft-deleted (mergeUserData).
	MoveStudent(ctx context.Context, tenant, from, into uuid.UUID, reason string, at time.Time) error
}

// SchoolPolicy reads the school settings projected from module identity (ref.schools.settings_subset).
type SchoolPolicy interface {
	// NicknameChangeLimit returns nickname_change_monthly_limit of a school, nil when it sets none.
	NicknameChangeLimit(ctx context.Context, tenant, school uuid.UUID) (*int, error)
}

// CatalogPublisher writes the platform content (PlatformScope only: seed, Game Operations Console F078).
type CatalogPublisher interface {
	// PublishCatalog stores payload as a new published version of the catalog code and points the catalog to it,
	// unless the current version has the same sha256 (returns false).
	PublishCatalog(ctx context.Context, code string, payload []byte, sha string, note string, at time.Time) (bool, error)
	// PublishItem stores def as a new published version of the item unless the current one is identical.
	PublishItem(ctx context.Context, def catalog.ItemDefinition, at time.Time) (bool, error)
}
