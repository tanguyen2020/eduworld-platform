package economy

import (
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
)

// Purchase refusals (INVENTORY_ECONOMY §11; mapped to 409 by the application).
var (
	ErrUnavailable     = errors.New("economy: shop item unavailable")
	ErrLimitReached    = errors.New("economy: purchase limit reached")
	ErrAlreadyOwned    = errors.New("economy: item already owned")
	ErrPriceChanged    = errors.New("economy: price changed")
	ErrQuantityInvalid = errors.New("economy: quantity invalid")
)

// Listing is a published shop item version as the shop sells it (the price is the server's, never the client's, R5).
type Listing struct {
	ShopItemID    uuid.UUID
	VersionID     uuid.UUID
	ItemID        string
	ItemKind      string
	Price         int
	Currency      string
	PurchaseLimit int
	From, To      *time.Time
	Grades        []int
	EventID       *uuid.UUID
	SeasonID      *uuid.UUID
	Assigned      bool // assigned to the tenant of the student (or to every tenant)
	Stackable     bool
	MaxStack      int
}

// Buyer is what the policy knows of the student.
type Buyer struct {
	Grade     int
	Owned     int // quantity of the item owned now
	Purchased int // quantity of this shop item bought before, every version
	LiveOps   bool
}

// Available says whether a listing is on sale for the buyer at now (F075-AC-06): assigned to the tenant, inside its sale
// window, of the grade of the student when grades are set, and — for an event or season listing — while that event or
// season runs (liveRunning).
func (l Listing) Available(grade int, now time.Time, liveRunning bool) bool {
	if !l.Assigned || l.Currency != Xu || l.Price < 1 {
		return false
	}
	if l.From != nil && now.Before(*l.From) {
		return false
	}
	if l.To != nil && !now.Before(*l.To) {
		return false
	}
	if len(l.Grades) > 0 && !slices.Contains(l.Grades, grade) {
		return false
	}
	if (l.EventID != nil || l.SeasonID != nil) && !liveRunning {
		return false
	}
	return true
}

// Limit is the purchase limit of the listing (shop_purchase_limit_default when unset).
func (l Listing) Limit(def int) int {
	if l.PurchaseLimit > 0 {
		return l.PurchaseLimit
	}
	return max(1, def)
}

// Check applies the purchase rules in order (F075-AC-10): on sale, the expected price is the current price, a
// non-stackable item is bought once and never when owned, the limit counts every version, a stack never exceeds
// max_stack. Returns the total price in Xu.
func (l Listing) Check(b Buyer, quantity, expectedPrice, defaultLimit int, now time.Time) (int, error) {
	if !l.Available(b.Grade, now, b.LiveOps) {
		return 0, ErrUnavailable
	}
	if quantity < 1 || (!l.Stackable && quantity != 1) {
		return 0, ErrQuantityInvalid
	}
	if expectedPrice != l.Price {
		return 0, ErrPriceChanged
	}
	if !l.Stackable && b.Owned > 0 {
		return 0, ErrAlreadyOwned
	}
	if b.Purchased+quantity > l.Limit(defaultLimit) {
		return 0, ErrLimitReached
	}
	if l.Stackable && l.MaxStack > 0 && b.Owned+quantity > l.MaxStack {
		return 0, ErrLimitReached
	}
	return l.Price * quantity, nil
}
