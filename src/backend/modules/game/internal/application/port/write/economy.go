package write

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Game content item types of the release core added by F075 (item is the F070 type, now published with item.schema.json).
const (
	ItemGameItem    = "item"
	ItemShopItem    = "shop_item"
	ItemCollection  = "collection"
	ItemAchievement = "achievement"
)

// Reward grant trigger kinds added by F075 (gamification.reward_grants.trigger_kind).
const (
	GrantAchievement   = "achievement"
	GrantCollection    = "collection"
	GrantKnowledgeCard = "knowledge_card"
)

// Ledger source types of the Xu and item ledgers.
const (
	SourceRewardGrant  = "reward_grant"
	SourceShopPurchase = "shop_purchase"
	SourceAdjustment   = "adjustment"
)

// CurrencyEntry is one row of gamification.currency_ledger (append-only).
type CurrencyEntry struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	StudentID      uuid.UUID
	Currency       string
	Delta          int64
	BalanceAfter   int64
	ReasonCode     string
	SourceType     string
	SourceID       uuid.UUID
	ItemID         *string
	Capped         bool
	IdempotencyKey string
	At             time.Time
}

// ItemEntry is one row of gamification.item_ledger (append-only).
type ItemEntry struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	StudentID      uuid.UUID
	ItemID         string
	Delta          int
	ReasonCode     string
	SourceType     string
	SourceID       uuid.UUID
	IdempotencyKey string
	At             time.Time
}

// Purchase is one row of gamification.shop_purchases (append-only).
type Purchase struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	StudentID      uuid.UUID
	ShopItemID     uuid.UUID
	VersionID      uuid.UUID
	ItemID         string
	Price          int
	Quantity       int
	EntryID        uuid.UUID
	IdempotencyKey string
	At             time.Time
}

// CollectionState is the progress row of a student on a collection (nil Owned and Exists false when none).
type CollectionState struct {
	Exists    bool
	Owned     []string
	Completed bool
}

// EconomyStore writes the wallets, the Xu and item ledgers, the inventory, the purchases and the collection progress of
// a student inside the transaction of ctx (TenantScope; RLS + explicit tenant_id). The ledgers are only inserted (R21).
type EconomyStore interface {
	// LockStudent serializes the economy writes of one student (advisory lock): wallet, inventory, collections.
	LockStudent(ctx context.Context, student uuid.UUID) error
	// Wallet returns the balance of the wallet of currency, creating it at 0 when missing, the row locked FOR UPDATE.
	Wallet(ctx context.Context, tenant, student uuid.UUID, currency string) (int64, error)
	// SetBalance writes the cached balance of the wallet (CHECK balance ≥ 0).
	SetBalance(ctx context.Context, tenant, student uuid.UUID, currency string, balance int64, at time.Time) error
	// InsertCurrencyEntry appends a Xu posting; false when its idempotency key exists (replay).
	InsertCurrencyEntry(ctx context.Context, e CurrencyEntry) (bool, error)
	// InsertItemEntry appends an item posting; false when its idempotency key exists (replay).
	InsertItemEntry(ctx context.Context, e ItemEntry) (bool, error)
	// Owned is the quantity of item the student owns (0 when none), the live row locked FOR UPDATE.
	Owned(ctx context.Context, tenant, student uuid.UUID, item string) (int, error)
	// AddInventory adds qty (≠ 0, either sign) to the live inventory row of item (created at qty when missing) and
	// returns the quantity after (never below 0).
	AddInventory(ctx context.Context, tenant, student uuid.UUID, school *uuid.UUID, item string, qty int, grant *uuid.UUID, at time.Time) (int, error)
	// Purchased is the quantity of a shop item the student bought before (every version).
	Purchased(ctx context.Context, tenant, student, shopItem uuid.UUID) (int, error)
	// InsertPurchase appends a purchase.
	InsertPurchase(ctx context.Context, p Purchase) error
	// Collection returns the progress of a collection (row locked FOR UPDATE).
	Collection(ctx context.Context, tenant, student, collection uuid.UUID) (CollectionState, error)
	// SaveCollection creates or updates the progress of a collection.
	SaveCollection(ctx context.Context, tenant, student, collection uuid.UUID, owned []string, completedAt *time.Time, at time.Time) error
	// CurrencyTotal is Σ delta of the Xu ledger of a student.
	CurrencyTotal(ctx context.Context, tenant, student uuid.UUID, currency string) (int64, error)
}

// WalletRef is a wallet found by the reconciliation job.
type WalletRef struct {
	TenantID  uuid.UUID
	StudentID uuid.UUID
	Currency  string
}

// WalletMismatch is a wallet whose balance differs from Σ ledger.
type WalletMismatch struct {
	WalletRef
	Balance int64
	Ledger  int64
}

// EconomyScan lists wallets for the platform job wallet-reconcile (PlatformScope; read only).
type EconomyScan interface {
	// WalletMismatches lists live wallets whose balance differs from Σ currency_ledger.delta after the cursor.
	WalletMismatches(ctx context.Context, after *WalletRef, limit int) ([]WalletMismatch, error)
}

// AchievementState is the progress row of a student on one achievement.
type AchievementState struct {
	Exists     bool
	Progress   int
	State      []byte // progress_state jsonb
	Tier       *string
	UnlockedAt *time.Time
	Version    int
}

// AchievementStore writes the achievement progress of a student and the counted events (TenantScope).
type AchievementStore interface {
	// MarkEvent records that eventID was counted for the achievement; false when it was already (redelivery).
	MarkEvent(ctx context.Context, tenant, student, achievement, eventID uuid.UUID, eventType string, at time.Time) (bool, error)
	// State returns the progress row (locked FOR UPDATE).
	State(ctx context.Context, tenant, student, achievement uuid.UUID) (AchievementState, error)
	// SaveState creates or updates the progress row.
	SaveState(ctx context.Context, tenant, student, achievement uuid.UUID, s AchievementState, at time.Time) error
}

// AdjustmentRow is one row of gamification.economy_adjustments.
type AdjustmentRow struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	StudentID      uuid.UUID
	Kind           string
	Currency       *string
	ItemID         *string
	Delta          int64
	Reason         string
	TicketRef      *string
	Status         string
	CreatedBy      uuid.UUID
	ApprovedBy     *uuid.UUID
	RejectedBy     *uuid.UUID
	RejectReason   *string
	LedgerEntryID  *uuid.UUID
	IdempotencyKey string
	CreatedAt      time.Time
	AppliedAt      *time.Time
	RejectedAt     *time.Time
}

// AdjustmentStore persists the economy adjustments (TenantScope of the target tenant).
type AdjustmentStore interface {
	// Insert appends a pending adjustment; false when its idempotency key exists.
	Insert(ctx context.Context, a AdjustmentRow) (bool, error)
	// ByKey returns the adjustment of an idempotency key (nil when none).
	ByKey(ctx context.Context, tenant uuid.UUID, key string) (*AdjustmentRow, error)
	// Get returns an adjustment of tenant locked FOR UPDATE (nil when unknown).
	Get(ctx context.Context, tenant, id uuid.UUID) (*AdjustmentRow, error)
	// Decide writes the decision (status, approver/rejecter, ledger entry, times).
	Decide(ctx context.Context, a AdjustmentRow) error
}

// CollectionItemRow is one item of a published collection version (gamification.collection_items).
type CollectionItemRow struct {
	ItemID      string
	Position    int
	Hint        map[string]string
	ObjectiveID *uuid.UUID
}

// CollectionItemStore writes the items of a published collection version (PlatformScope, game_platform; GA rows).
type CollectionItemStore interface {
	InsertCollectionItems(ctx context.Context, collection, versionID uuid.UUID, items []CollectionItemRow) error
}
