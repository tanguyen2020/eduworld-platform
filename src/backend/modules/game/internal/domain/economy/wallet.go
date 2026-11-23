// Package economy is the domain of the in-game economy of module game (F075, INVENTORY_ECONOMY §3–§6, ADR-0049): the
// Xu wallet that never goes negative, the daily earning cap, the shop purchase rules (price shown first, deterministic
// items, purchase limit, sale window), the duplicate conversion and the four-eyes economy adjustment. Pure: no I/O, the
// clock is passed in. Never a real-money price, a random reward or an item that gives an advantage (R19).
package economy

import (
	"errors"
	"fmt"
)

// Xu is the only currency of Release 1.0 (ADR-0049, OQ-14).
const Xu = "xu"

// CapBucket is the daily counter of the Xu earned by a student (shared table of the F074 caps).
const CapBucket = "currency:xu"

// Reason codes of gamification.currency_ledger and gamification.item_ledger (ADR-0049).
const (
	ReasonReward              = "reward"
	ReasonShopPurchase        = "shop_purchase"
	ReasonAdjustment          = "adjustment"
	ReasonRefund              = "refund"
	ReasonDuplicateConversion = "duplicate_conversion"
	ReasonCollection          = "collection"
)

// ErrInsufficientBalance is a debit larger than the balance (409 INSUFFICIENT_BALANCE).
var ErrInsufficientBalance = errors.New("economy: insufficient balance")

// ErrInvalidAmount is a zero or negative amount where a positive one is required.
var ErrInvalidAmount = errors.New("economy: invalid amount")

// Wallet is the balance of one currency of a student (a cache of Σ ledger, ADR-0049).
type Wallet struct {
	Currency string
	Balance  int64
}

// Credit adds amount (> 0) and returns the balance after.
func (w *Wallet) Credit(amount int64) (int64, error) {
	if amount <= 0 {
		return w.Balance, ErrInvalidAmount
	}
	w.Balance += amount
	return w.Balance, nil
}

// Debit removes amount (> 0); a balance that would go negative is refused and left unchanged (F075-AC-03, AC-09).
func (w *Wallet) Debit(amount int64) (int64, error) {
	if amount <= 0 {
		return w.Balance, ErrInvalidAmount
	}
	if amount > w.Balance {
		return w.Balance, ErrInsufficientBalance
	}
	w.Balance -= amount
	return w.Balance, nil
}

// Apply adds delta (≠ 0, either sign) as a compensating posting does: never below zero.
func (w *Wallet) Apply(delta int64) (int64, error) {
	switch {
	case delta > 0:
		return w.Credit(delta)
	case delta < 0:
		return w.Debit(-delta)
	}
	return w.Balance, ErrInvalidAmount
}

// Earnable is the part of amount a student may still earn today under the daily cap (F075-AC-02): the remaining
// allowance, never negative; capped reports that the cap cut the amount.
func Earnable(amount, earnedToday, dailyCap int) (credit int, capped bool) {
	if amount <= 0 {
		return 0, false
	}
	remaining := max(0, dailyCap-earnedToday)
	credit = min(amount, remaining)
	return credit, credit < amount
}

// Remaining is the Xu a student can still earn today.
func Remaining(earnedToday, dailyCap int) int { return max(0, dailyCap-earnedToday) }

// DuplicateTable is duplicate_item_xu: the Xu a non-stackable item already owned converts to, by rarity
// (INVENTORY_ECONOMY §6.3, CONFIG_DEFAULTS).
type DuplicateTable map[string]int

// Rarities are the item rarities (ItemRarity).
var Rarities = []string{"common", "uncommon", "rare", "epic", "legendary"}

// Validate requires a positive amount for every rarity.
func (d DuplicateTable) Validate() error {
	for _, r := range Rarities {
		if d[r] < 1 {
			return fmt.Errorf("duplicate_item_xu: %s needs a positive amount", r)
		}
	}
	return nil
}

// Conversion is the Xu of a duplicate of rarity (0 when the rarity is unknown).
func (d DuplicateTable) Conversion(rarity string) int { return d[rarity] }
