package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/achievement"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/economy"
)

// ItemDef is the current published version of an item definition (game.item_definition_versions).
type ItemDef struct {
	ItemID    string
	Kind      string
	Rarity    string
	EquipSlot string
	Stackable bool
	MaxStack  int
	Name      map[string]string
}

// ListingRow is the current published version of a shop item as one tenant sees it.
type ListingRow struct {
	economy.Listing
	FeaturedOrder int
}

// CollectionRef is the current published version of a collection that holds an item.
type CollectionRef struct {
	CollectionID uuid.UUID
	VersionID    uuid.UUID
	Code         string
	Type         string
	Total        int
	Rewards      []map[string]any
}

// CardRef is the knowledge card of a learning objective (the item of a published knowledge_card collection).
type CardRef struct {
	ItemID       string
	CollectionID uuid.UUID
}

// AchievementRow is the current published version of an achievement.
type AchievementRow struct {
	VersionID  uuid.UUID
	Definition achievement.Definition
}

// CurrencyRow is a currency (gamification.currencies).
type CurrencyRow struct {
	Code     string
	DailyCap int
	Active   bool
}

// EconomyContent reads the published economy content (G tables; the student never sees an unpublished version).
type EconomyContent interface {
	// Items returns the current published definitions of the codes (unknown codes are absent).
	Items(ctx context.Context, codes []string) (map[string]ItemDef, error)
	// Listing returns the current published version of a shop item with its tenant assignment for tenant (nil when the
	// shop item has no published version).
	Listing(ctx context.Context, tenant, shopItem uuid.UUID) (*ListingRow, error)
	// CollectionsOf lists the current published collections holding item.
	CollectionsOf(ctx context.Context, item string) ([]CollectionRef, error)
	// CardFor returns the knowledge card of a learning objective (nil when no published collection names it).
	CardFor(ctx context.Context, objective uuid.UUID) (*CardRef, error)
	// AchievementsFor lists the current published achievements triggered by eventType.
	AchievementsFor(ctx context.Context, eventType string) ([]AchievementRow, error)
	// Currency returns a currency (nil when unknown).
	Currency(ctx context.Context, code string) (*CurrencyRow, error)
}

// LiveOps says whether the event or season of a limited shop listing runs at at (EVENT_SEASON_SYSTEM). Until F077
// delivers the event runs, no event or season runs (fail closed: limited listings are not sold).
type LiveOps interface {
	Running(ctx context.Context, tenant uuid.UUID, event, season *uuid.UUID, at time.Time) (bool, error)
}
