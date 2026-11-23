package read

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// TimeCursor is the position after the last returned row of a newest-first list (created_at DESC, id DESC).
type TimeCursor struct {
	At time.Time
	ID uuid.UUID
}

// CurrencyLedgerRow is one Xu posting as the student sees it: no source id or key (F075-AC-04).
type CurrencyLedgerRow struct {
	ID           uuid.UUID
	Currency     string
	Delta        int64
	BalanceAfter int64
	ReasonCode   string
	SourceType   string
	CreatedAt    time.Time
}

// ItemHistoryRow is one item posting as the student sees it.
type ItemHistoryRow struct {
	ID         uuid.UUID
	ItemID     string
	Kind       string
	Name       map[string]string
	Delta      int
	ReasonCode string
	SourceType string
	CreatedAt  time.Time
}

// InventoryRow is one item a student owns.
type InventoryRow struct {
	ItemID     string
	Kind       string
	Rarity     string
	EquipSlot  string
	Stackable  bool
	Quantity   int
	AcquiredAt time.Time
	Name       map[string]string
}

// ShopFilter selects the listings a student sees (F075-AC-06), keyset (featured_order, shop_item_id).
type ShopFilter struct {
	Tenant, Student uuid.UUID
	Grade           int
	Kind            string
	Now             time.Time
	AfterOrder      *int
	AfterID         uuid.UUID
	Limit           int
}

// ShopRow is one listing on sale for a student.
type ShopRow struct {
	ShopItemID    uuid.UUID
	ItemID        string
	Kind          string
	Rarity        string
	Name          map[string]string
	Description   map[string]string
	Price         int
	Currency      string
	PurchaseLimit int
	Purchased     int
	Owned         bool
	AvailableTo   *time.Time
	FeaturedOrder int
}

// CollectionRow is one collection with the progress of a student.
type CollectionRow struct {
	CollectionID uuid.UUID
	Code         string
	Type         string
	Title        map[string]string
	Owned        []string
	Total        int
	CompletedAt  *time.Time
	Rewards      []map[string]any
}

// CollectionItemView is one item of a collection detail.
type CollectionItemView struct {
	ItemID string
	Name   map[string]string
	Rarity string
	Hint   map[string]string
}

// AchievementView is one published achievement with the progress of a student (hidden locked ones excluded by the query).
type AchievementView struct {
	AchievementID uuid.UUID
	Code          string
	Payload       []byte
	Progress      int
	Tier          *string
	UnlockedAt    *time.Time
}

// AdjustmentFilter selects economy adjustments (System Admin), keyset (created_at DESC, adjustment_id DESC).
type AdjustmentFilter struct {
	TenantIDs  []uuid.UUID
	AllTenants bool
	Status     string
	StudentID  *uuid.UUID
	After      *TimeCursor
	Limit      int
}

// EconomyReadModel reads the economy of a student and the platform content (READ ONLY session, explicit tenant, keyset
// pages in SQL — limit+1 rows to detect more).
type EconomyReadModel interface {
	Balance(ctx context.Context, tenant, student uuid.UUID, currency string) (int64, error)
	// Grade is the grade of the projected student (ref.students; 0 when unknown).
	Grade(ctx context.Context, tenant, student uuid.UUID) (int, error)
	DailyCap(ctx context.Context, currency string) (int, error)
	EarnedToday(ctx context.Context, tenant, student uuid.UUID, day, bucket string) (int, error)
	CurrencyLedger(ctx context.Context, tenant, student uuid.UUID, after *TimeCursor, limit int) ([]CurrencyLedgerRow, error)
	ItemHistory(ctx context.Context, tenant, student uuid.UUID, after *TimeCursor, limit int) ([]ItemHistoryRow, error)
	Inventory(ctx context.Context, tenant, student uuid.UUID, kind, after string, limit int) ([]InventoryRow, error)
	Shop(ctx context.Context, f ShopFilter) ([]ShopRow, error)
	Collections(ctx context.Context, tenant, student uuid.UUID, after string, limit int) ([]CollectionRow, error)
	Collection(ctx context.Context, tenant, student, collection uuid.UUID) (*CollectionRow, []CollectionItemView, error)
	Achievements(ctx context.Context, tenant, student uuid.UUID, category, after string, limit int) ([]AchievementView, error)
	Adjustments(ctx context.Context, f AdjustmentFilter) ([]AdjustmentRecord, error)
}

// AdjustmentRecord is one economy adjustment (columns of gamification.economy_adjustments).
type AdjustmentRecord struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	StudentID    uuid.UUID
	Kind         string
	Currency     *string
	ItemID       *string
	Delta        int64
	Reason       string
	TicketRef    *string
	Status       string
	CreatedBy    uuid.UUID
	ApprovedBy   *uuid.UUID
	RejectedBy   *uuid.UUID
	RejectReason *string
	CreatedAt    time.Time
	AppliedAt    *time.Time
	RejectedAt   *time.Time
}
