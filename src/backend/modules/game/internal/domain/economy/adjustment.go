package economy

import (
	"errors"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Adjustment kinds and states (gamification.economy_adjustments).
const (
	AdjustCurrency = "currency"
	AdjustItem     = "item"
	AdjustXP       = "xp"

	StatusPending  = "pending_approval"
	StatusApplied  = "applied"
	StatusRejected = "rejected"
)

// Adjustment errors.
var (
	ErrSelfApproval  = errors.New("economy: the approver created the adjustment") // 403 PERMISSION_DENIED (four-eyes)
	ErrNotPending    = errors.New("economy: adjustment already decided")          // 422 INVALID_STATE_TRANSITION
	ErrAdjustInvalid = errors.New("economy: invalid adjustment")                  // 422 VALIDATION_FAILED
)

// AdjustmentFieldError names the invalid field of an adjustment request.
type AdjustmentFieldError struct {
	Field, Reason string
}

func (e *AdjustmentFieldError) Error() string { return e.Field + ": " + e.Reason }
func (e *AdjustmentFieldError) Unwrap() error { return ErrAdjustInvalid }

// Adjustment is a compensating posting waiting for, or decided by, a second person (ADR-0049, ANTI_CHEAT §7).
type Adjustment struct {
	ID         uuid.UUID
	StudentID  uuid.UUID
	Kind       string
	Currency   *string
	ItemID     *string
	Delta      int64
	Reason     string
	TicketRef  *string
	Status     string
	CreatedBy  uuid.UUID
	ApprovedBy *uuid.UUID
	RejectedBy *uuid.UUID
}

// Validate checks a new request: a known kind with its target (currency xu, an item code), a delta ≠ 0, a reason of
// 10–1000 characters (F075-AC-25, AC-27).
func (a Adjustment) Validate() error {
	if !slices.Contains([]string{AdjustCurrency, AdjustItem, AdjustXP}, a.Kind) {
		return &AdjustmentFieldError{"kind", "must be currency, item or xp"}
	}
	if a.Delta == 0 {
		return &AdjustmentFieldError{"delta", "must not be 0"}
	}
	n := utf8.RuneCountInString(strings.TrimSpace(a.Reason))
	if n < 10 || utf8.RuneCountInString(a.Reason) > 1000 {
		return &AdjustmentFieldError{"reason", "10 to 1000 characters"}
	}
	switch a.Kind {
	case AdjustCurrency:
		if a.Currency == nil || *a.Currency != Xu {
			return &AdjustmentFieldError{"currency_code", "must be xu"}
		}
		if a.ItemID != nil {
			return &AdjustmentFieldError{"item_id", "only for an item adjustment"}
		}
	case AdjustItem:
		if a.ItemID == nil || *a.ItemID == "" {
			return &AdjustmentFieldError{"item_id", "required for an item adjustment"}
		}
		if a.Currency != nil {
			return &AdjustmentFieldError{"currency_code", "only for a currency adjustment"}
		}
	case AdjustXP:
		if a.ItemID != nil || a.Currency != nil {
			return &AdjustmentFieldError{"kind", "an xp adjustment names no currency or item"}
		}
	}
	return nil
}

// Approve records the decision of approver: a second person only (four-eyes), a pending adjustment only.
func (a *Adjustment) Approve(approver uuid.UUID) error {
	if a.Status != StatusPending {
		return ErrNotPending
	}
	if approver == a.CreatedBy {
		return ErrSelfApproval
	}
	a.Status, a.ApprovedBy = StatusApplied, &approver
	return nil
}

// Reject records a refusal by a second person.
func (a *Adjustment) Reject(by uuid.UUID) error {
	if a.Status != StatusPending {
		return ErrNotPending
	}
	if by == a.CreatedBy {
		return ErrSelfApproval
	}
	a.Status, a.RejectedBy = StatusRejected, &by
	return nil
}
