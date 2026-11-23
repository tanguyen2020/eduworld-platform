package command

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/achievement"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/economy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// PurchaseItem is the InventoryItem of a PurchaseResult.
type PurchaseItem struct {
	ItemID     string            `json:"item_id"`
	Kind       string            `json:"kind"`
	Rarity     string            `json:"rarity,omitempty"`
	EquipSlot  string            `json:"equip_slot,omitempty"`
	Quantity   int               `json:"quantity"`
	AcquiredAt time.Time         `json:"acquired_at"`
	Name       map[string]string `json:"name,omitempty"`
}

// PurchaseResult is PurchaseResult of the contract.
type PurchaseResult struct {
	ShopItemID   uuid.UUID    `json:"shop_item_id"`
	BalanceAfter int64        `json:"balance_after"`
	Item         PurchaseItem `json:"item"`
}

func purchaseProblem(err error) error {
	switch {
	case errors.Is(err, economy.ErrUnavailable):
		return problem.New(problem.ShopItemUnavailable, "")
	case errors.Is(err, economy.ErrLimitReached):
		return problem.New(problem.PurchaseLimitReached, "")
	case errors.Is(err, economy.ErrAlreadyOwned):
		return problem.New(problem.ItemAlreadyOwned, "")
	case errors.Is(err, economy.ErrPriceChanged):
		return problem.New(problem.Conflict, "the price changed; show the new price before buying")
	case errors.Is(err, economy.ErrQuantityInvalid):
		return problem.Invalid("quantity", problem.SubInvalid, "1 for a non-stackable item")
	case errors.Is(err, economy.ErrInsufficientBalance):
		return problem.New(problem.InsufficientBalance, "")
	}
	return err
}

func playerOf(ctx context.Context) (tenant, student uuid.UUID, err error) {
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || !p.Valid() {
		return uuid.Nil, uuid.Nil, problem.New(problem.Unauthenticated, "")
	}
	if p.UserType != authz.UserTypeStudent || p.TenantID == nil {
		return uuid.Nil, uuid.Nil, problem.New(problem.GameAccessStudentOnly, "")
	}
	return *p.TenantID, p.UserID, nil
}

// Purchase is purchaseShopItem (F075-AC-07…AC-10): in one transaction the wallet of the student is locked, the listing is
// checked at the server price (sale window, tenant, grade, event/season, limit over every version, ownership), the Xu
// posting (shop_purchase), the item posting, the inventory, the purchase row, the outbox events shop.item_purchased,
// item.granted, currency.debited and the audit record are written, and the response is stored for the Idempotency-Key.
// A listing not assigned to the tenant is SHOP_ITEM_UNAVAILABLE (no existence leak, F075-AC-12).
func (h *Economy) Purchase(ctx context.Context, shopItem uuid.UUID, quantity, expectedPrice int) (out PurchaseResult, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.PurchaseShopItem")
	defer func() {
		result := "ok"
		var pe *problem.Error
		if errors.As(err, &pe) {
			result = string(pe.Code)
		} else if err != nil {
			result = "error"
		}
		h.metric(func(m EconomyMetrics) { m.ShopPurchase(result) })
		done(err)
	}()
	tenant, student, err := playerOf(ctx)
	if err != nil {
		return out, err
	}
	if quantity == 0 {
		quantity = 1
	}
	now := h.now()
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		st, err := h.Students.Student(ctx, tenant, student)
		if err != nil {
			return err
		}
		if st == nil || st.Deleted || st.Status != "active" {
			return problem.New(problem.GameAccessStudentOnly, "")
		}
		l, err := h.Content.Listing(ctx, tenant, shopItem)
		if err != nil {
			return err
		}
		if l == nil {
			return problem.New(problem.ShopItemUnavailable, "")
		}
		defs, err := h.Content.Items(ctx, []string{l.ItemID})
		if err != nil {
			return err
		}
		def, ok := defs[l.ItemID]
		if !ok {
			return problem.New(problem.ShopItemUnavailable, "")
		}
		l.Stackable, l.MaxStack = def.Stackable, def.MaxStack
		if err := h.Store.LockStudent(ctx, student); err != nil {
			return err
		}
		owned, err := h.Store.Owned(ctx, tenant, student, l.ItemID)
		if err != nil {
			return err
		}
		bought, err := h.Store.Purchased(ctx, tenant, student, shopItem)
		if err != nil {
			return err
		}
		live := false
		if (l.EventID != nil || l.SeasonID != nil) && h.LiveOps != nil {
			if live, err = h.LiveOps.Running(ctx, tenant, l.EventID, l.SeasonID, now); err != nil {
				return err
			}
		}
		total, err := l.Check(economy.Buyer{Grade: st.Grade, Owned: owned, Purchased: bought, LiveOps: live}, quantity, expectedPrice,
			h.Policy.PurchaseLimitDefault, now)
		if err != nil {
			return purchaseProblem(err)
		}
		bal, err := h.Store.Wallet(ctx, tenant, student, economy.Xu)
		if err != nil {
			return err
		}
		w := economy.Wallet{Currency: economy.Xu, Balance: bal}
		after, err := w.Debit(int64(total))
		if err != nil {
			return purchaseProblem(err)
		}
		purchase := ids.New()
		item := l.ItemID
		entry := write.CurrencyEntry{ID: ids.New(), TenantID: tenant, StudentID: student, Currency: economy.Xu, Delta: -int64(total), BalanceAfter: after,
			ReasonCode: economy.ReasonShopPurchase, SourceType: write.SourceShopPurchase, SourceID: purchase, ItemID: &item,
			IdempotencyKey: Key("shop_purchase", purchase.String()), At: now}
		if _, err := h.post(ctx, entry); err != nil {
			return err
		}
		school := st.SchoolID
		if _, err := h.addItem(ctx, itemAdd{tenant: tenant, student: student, school: &school, def: def, qty: quantity, reason: economy.ReasonShopPurchase,
			sourceType: write.SourceShopPurchase, sourceID: purchase, key: Key("shop_purchase_item", purchase.String()), at: now}); err != nil {
			return err
		}
		if err := h.Store.InsertPurchase(ctx, write.Purchase{ID: purchase, TenantID: tenant, StudentID: student, ShopItemID: shopItem, VersionID: l.VersionID,
			ItemID: l.ItemID, Price: total, Quantity: quantity, EntryID: entry.ID, IdempotencyKey: Key("purchase", purchase.String()), At: now}); err != nil {
			return err
		}
		if err := h.Events.Publish(ctx, write.Event{Type: EventShopItemPurchased, SubjectType: "student", SubjectID: student.String(),
			PartitionKey: student.String(), TenantID: &tenant, OccurredAt: now, Payload: map[string]any{"purchase_id": purchase, "student_id": student,
				"shop_item_id": shopItem, "shop_item_version_id": l.VersionID, "item_id": l.ItemID, "item_kind": def.Kind, "price": total,
				"quantity": quantity, "currency_code": economy.Xu, "balance_after": after, "currency_entry_id": entry.ID, "purchased_at": now}}); err != nil {
			return err
		}
		if err := h.Audit.Record(ctx, audit.Event{Action: auditShopPurchased, TargetType: "shop_item", TargetID: shopItem.String(),
			Result: audit.ResultSuccess, Metadata: map[string]any{"purchase_id": purchase, "item_id": l.ItemID, "price": total, "quantity": quantity}}); err != nil {
			return err
		}
		held, err := h.Store.Owned(ctx, tenant, student, l.ItemID)
		if err != nil {
			return err
		}
		out = PurchaseResult{ShopItemID: shopItem, BalanceAfter: after, Item: PurchaseItem{ItemID: def.ItemID, Kind: def.Kind, Rarity: def.Rarity,
			EquipSlot: def.EquipSlot, Quantity: held, AcquiredAt: now, Name: def.Name}}
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// AdjustmentInput is EconomyAdjustmentRequest of the contract.
type AdjustmentInput struct {
	TenantID  uuid.UUID
	StudentID uuid.UUID
	Kind      string
	Currency  *string
	ItemID    *string
	Delta     int64
	Reason    string
	TicketRef *string
}

// AdjustmentView is EconomyAdjustment of the contract.
type AdjustmentView struct {
	AdjustmentID uuid.UUID  `json:"adjustment_id"`
	TenantID     uuid.UUID  `json:"tenant_id"`
	StudentID    uuid.UUID  `json:"student_id"`
	Kind         string     `json:"kind"`
	CurrencyCode *string    `json:"currency_code,omitempty"`
	ItemID       *string    `json:"item_id,omitempty"`
	Delta        int64      `json:"delta"`
	Reason       string     `json:"reason"`
	TicketRef    *string    `json:"ticket_ref,omitempty"`
	Status       string     `json:"status"`
	CreatedBy    uuid.UUID  `json:"created_by"`
	ApprovedBy   *uuid.UUID `json:"approved_by,omitempty"`
	RejectedBy   *uuid.UUID `json:"rejected_by,omitempty"`
	RejectReason *string    `json:"reject_reason,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	AppliedAt    *time.Time `json:"applied_at,omitempty"`
	RejectedAt   *time.Time `json:"rejected_at,omitempty"`
}

// ViewOf maps an adjustment row to the contract.
func ViewOf(a write.AdjustmentRow) AdjustmentView {
	return AdjustmentView{AdjustmentID: a.ID, TenantID: a.TenantID, StudentID: a.StudentID, Kind: a.Kind, CurrencyCode: a.Currency, ItemID: a.ItemID,
		Delta: a.Delta, Reason: a.Reason, TicketRef: a.TicketRef, Status: a.Status, CreatedBy: a.CreatedBy, ApprovedBy: a.ApprovedBy,
		RejectedBy: a.RejectedBy, RejectReason: a.RejectReason, CreatedAt: a.CreatedAt.UTC(), AppliedAt: a.AppliedAt, RejectedAt: a.RejectedAt}
}

// adjuster authorizes an economy adjustment use case (F075-AC-27): a principal with game.economy.adjust whose session
// tenant (X-Tenant-Id of System Admin) is the target tenant; the route guards (portal client, platform role, internal
// gateway) ran before. Missing permission → 403 PERMISSION_DENIED audited as authz.denied.
func adjuster(ctx context.Context) (authz.Principal, uuid.UUID, error) {
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || !p.Valid() {
		return p, uuid.Nil, problem.New(problem.Unauthenticated, "")
	}
	if p.TenantID == nil {
		authz.RecordDenied(ctx, string(problem.PermissionDenied), map[string]any{"why": "no_target_tenant"})
		return p, uuid.Nil, problem.New(problem.PermissionDenied, "")
	}
	tenant := *p.TenantID
	if err := authz.Require(ctx, p, authz.PermGameEconomyAdjust, authz.Resource{TenantID: &tenant}); err != nil {
		return p, uuid.Nil, err
	}
	if !p.HasPlatformRole() {
		authz.RecordDenied(ctx, string(problem.PermissionDenied), map[string]any{"why": "platform_role_required"})
		return p, uuid.Nil, problem.New(problem.PermissionDenied, "")
	}
	return p, tenant, nil
}

func adjustmentProblem(err error) error {
	var fe *economy.AdjustmentFieldError
	switch {
	case errors.As(err, &fe):
		return problem.Invalid(fe.Field, problem.SubInvalid, fe.Reason)
	case errors.Is(err, economy.ErrNotPending):
		return problem.New(problem.InvalidStateTransition, "the adjustment is already decided")
	case errors.Is(err, economy.ErrInsufficientBalance):
		return problem.New(problem.InsufficientBalance, "")
	}
	return err
}

// CreateAdjustment is createEconomyAdjustment (F075-AC-25): a pending compensating posting of the target tenant
// (X-Tenant-Id = tenant_id of the body), validated (kind, target, delta ≠ 0, reason 10–1000), nothing posted yet, audited.
func (h *Economy) CreateAdjustment(ctx context.Context, in AdjustmentInput) (out AdjustmentView, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.CreateEconomyAdjustment")
	defer func() { done(err) }()
	p, tenant, err := adjuster(ctx)
	if err != nil {
		return out, err
	}
	if in.TenantID != tenant {
		return out, problem.Invalid("tenant_id", problem.SubInvalid, "must equal X-Tenant-Id")
	}
	a := economy.Adjustment{StudentID: in.StudentID, Kind: in.Kind, Currency: in.Currency, ItemID: in.ItemID, Delta: in.Delta, Reason: in.Reason,
		TicketRef: in.TicketRef, Status: economy.StatusPending, CreatedBy: p.UserID}
	if err := a.Validate(); err != nil {
		return out, adjustmentProblem(err)
	}
	now := h.now()
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		st, err := h.Students.Student(ctx, tenant, in.StudentID)
		if err != nil {
			return err
		}
		if st == nil {
			return problem.Invalid("student_id", problem.SubInvalid, "no student of the tenant")
		}
		if in.ItemID != nil {
			defs, err := h.Content.Items(ctx, []string{*in.ItemID})
			if err != nil {
				return err
			}
			if _, ok := defs[*in.ItemID]; !ok {
				return problem.Invalid("item_id", problem.SubInvalid, "unknown item")
			}
		}
		id := ids.New()
		row := write.AdjustmentRow{ID: id, TenantID: tenant, StudentID: in.StudentID, Kind: in.Kind, Currency: in.Currency, ItemID: in.ItemID, Delta: in.Delta,
			Reason: in.Reason, TicketRef: in.TicketRef, Status: economy.StatusPending, CreatedBy: p.UserID, IdempotencyKey: Key("adjustment", id.String()),
			CreatedAt: now}
		if _, err := h.Adjustments.Insert(ctx, row); err != nil {
			return err
		}
		if err := h.Audit.Record(ctx, audit.Event{Action: auditAdjustmentCreated, TargetType: "economy_adjustment", TargetID: id.String(),
			Result: audit.ResultSuccess, Metadata: map[string]any{"student_id": in.StudentID, "kind": in.Kind, "delta": in.Delta, "item_id": in.ItemID}}); err != nil {
			return err
		}
		out = ViewOf(row)
		return idempotency.SetResult(ctx, out)
	})
	if err == nil {
		h.after(ctx, func() { h.metric(func(m EconomyMetrics) { m.Adjustment(economy.StatusPending) }) })
	}
	return out, err
}

// ApproveAdjustment is approveEconomyAdjustment (F075-AC-26): a second person with game.economy.adjust posts the
// compensating entry (reason adjustment) to the Xu, item or XP ledger in one transaction, writes economy.adjusted and the
// audit. The creator approving is 403 PERMISSION_DENIED (four-eyes); a balance going negative 409 INSUFFICIENT_BALANCE;
// a decided adjustment 422 INVALID_STATE_TRANSITION.
func (h *Economy) ApproveAdjustment(ctx context.Context, id uuid.UUID) (out AdjustmentView, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.ApproveEconomyAdjustment")
	defer func() { done(err) }()
	p, tenant, err := adjuster(ctx)
	if err != nil {
		return out, err
	}
	now := h.now()
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		row, a, err := h.loadAdjustment(ctx, tenant, id)
		if err != nil {
			return err
		}
		if err := h.decide(ctx, a.Approve, p.UserID); err != nil {
			return err
		}
		entry, err := h.applyAdjustment(ctx, *row, now)
		if err != nil {
			return err
		}
		row.Status, row.ApprovedBy, row.LedgerEntryID, row.AppliedAt = economy.StatusApplied, a.ApprovedBy, &entry, &now
		if err := h.Adjustments.Decide(ctx, *row); err != nil {
			return err
		}
		if err := h.Events.Publish(ctx, write.Event{Type: EventEconomyAdjusted, SubjectType: "student", SubjectID: row.StudentID.String(),
			PartitionKey: row.StudentID.String(), TenantID: &tenant, OccurredAt: now, Payload: map[string]any{"adjustment_id": row.ID,
				"student_id": row.StudentID, "kind": row.Kind, "currency_code": row.Currency, "item_id": row.ItemID, "delta": row.Delta,
				"entry_ids": []uuid.UUID{entry}, "reason": row.Reason, "ticket_ref": row.TicketRef, "created_by": row.CreatedBy, "approved_by": p.UserID,
				"applied_at": now}}); err != nil {
			return err
		}
		if err := h.Audit.Record(ctx, audit.Event{Action: auditAdjustmentApproved, TargetType: "economy_adjustment", TargetID: id.String(),
			Result: audit.ResultSuccess, Metadata: map[string]any{"student_id": row.StudentID, "kind": row.Kind, "delta": row.Delta, "entry_id": entry}}); err != nil {
			return err
		}
		out = ViewOf(*row)
		return idempotency.SetResult(ctx, out)
	})
	if err == nil {
		h.after(ctx, func() { h.metric(func(m EconomyMetrics) { m.Adjustment(economy.StatusApplied) }) })
	}
	return out, deniedOutside(ctx, err)
}

// RejectAdjustment is rejectEconomyAdjustment: a second person refuses a pending adjustment with a reason; nothing is
// posted, the decision is audited.
func (h *Economy) RejectAdjustment(ctx context.Context, id uuid.UUID, reason string) (out AdjustmentView, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.RejectEconomyAdjustment")
	defer func() { done(err) }()
	p, tenant, err := adjuster(ctx)
	if err != nil {
		return out, err
	}
	now := h.now()
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		row, a, err := h.loadAdjustment(ctx, tenant, id)
		if err != nil {
			return err
		}
		if err := h.decide(ctx, a.Reject, p.UserID); err != nil {
			return err
		}
		row.Status, row.RejectedBy, row.RejectReason, row.RejectedAt = economy.StatusRejected, a.RejectedBy, &reason, &now
		if err := h.Adjustments.Decide(ctx, *row); err != nil {
			return err
		}
		if err := h.Audit.Record(ctx, audit.Event{Action: auditAdjustmentRejected, TargetType: "economy_adjustment", TargetID: id.String(),
			Result: audit.ResultSuccess, Reason: reason, Metadata: map[string]any{"student_id": row.StudentID}}); err != nil {
			return err
		}
		out = ViewOf(*row)
		return idempotency.SetResult(ctx, out)
	})
	if err == nil {
		h.after(ctx, func() { h.metric(func(m EconomyMetrics) { m.Adjustment(economy.StatusRejected) }) })
	}
	return out, deniedOutside(ctx, err)
}

func (h *Economy) loadAdjustment(ctx context.Context, tenant, id uuid.UUID) (*write.AdjustmentRow, *economy.Adjustment, error) {
	row, err := h.Adjustments.Get(ctx, tenant, id)
	if err != nil {
		return nil, nil, err
	}
	if row == nil {
		return nil, nil, problem.New(problem.NotFound, "")
	}
	return row, &economy.Adjustment{ID: row.ID, StudentID: row.StudentID, Kind: row.Kind, Currency: row.Currency, ItemID: row.ItemID, Delta: row.Delta,
		Reason: row.Reason, Status: row.Status, CreatedBy: row.CreatedBy}, nil
}

// fourEyesDenied is the creator deciding its own adjustment: 403 PERMISSION_DENIED, audited as authz.denied after the
// transaction rolled back (deniedOutside).
type fourEyesDenied struct{ error }

func (e fourEyesDenied) Unwrap() error { return e.error }

// decide applies the four-eyes decision: the creator deciding is 403 PERMISSION_DENIED.
func (h *Economy) decide(_ context.Context, fn func(uuid.UUID) error, by uuid.UUID) error {
	err := fn(by)
	if errors.Is(err, economy.ErrSelfApproval) {
		return fourEyesDenied{problem.New(problem.PermissionDenied, "the creator of an adjustment cannot decide it")}
	}
	return adjustmentProblem(err)
}

// deniedOutside records authz.denied of a four-eyes refusal outside the rolled back transaction (F075-AC-26).
func deniedOutside(ctx context.Context, err error) error {
	var fe fourEyesDenied
	if errors.As(err, &fe) {
		authz.RecordDenied(ctx, string(problem.PermissionDenied), map[string]any{"why": "four_eyes"})
	}
	return err
}

// applyAdjustment posts the compensating entry of an approved adjustment and returns its ledger entry id.
func (h *Economy) applyAdjustment(ctx context.Context, a write.AdjustmentRow, at time.Time) (uuid.UUID, error) {
	key := Key("adjustment_entry", a.ID.String())
	switch a.Kind {
	case economy.AdjustXP:
		if h.XP == nil {
			return uuid.Nil, problem.New(problem.Internal, "xp adjustments are not configured")
		}
		id, err := h.XP.AdjustXP(ctx, a.TenantID, a.StudentID, a.ID, a.Delta)
		return id, adjustmentProblem(err)
	case economy.AdjustCurrency:
		if err := h.Store.LockStudent(ctx, a.StudentID); err != nil {
			return uuid.Nil, err
		}
		e := write.CurrencyEntry{ID: ids.New(), TenantID: a.TenantID, StudentID: a.StudentID, Currency: economy.Xu, Delta: a.Delta,
			ReasonCode: economy.ReasonAdjustment, SourceType: write.SourceAdjustment, SourceID: a.ID, IdempotencyKey: key, At: at}
		if _, err := h.credit(ctx, e); err != nil {
			return uuid.Nil, adjustmentProblem(err)
		}
		return e.ID, nil
	case economy.AdjustItem:
		defs, err := h.Content.Items(ctx, []string{*a.ItemID})
		if err != nil {
			return uuid.Nil, err
		}
		def, ok := defs[*a.ItemID]
		if !ok {
			return uuid.Nil, problem.Invalid("item_id", problem.SubInvalid, "unknown item")
		}
		if err := h.Store.LockStudent(ctx, a.StudentID); err != nil {
			return uuid.Nil, err
		}
		owned, err := h.Store.Owned(ctx, a.TenantID, a.StudentID, def.ItemID)
		if err != nil {
			return uuid.Nil, err
		}
		after := int64(owned) + a.Delta
		if after < 0 {
			return uuid.Nil, problem.New(problem.InsufficientBalance, "the student does not own that many")
		}
		if after > int64(def.MaxStack) {
			return uuid.Nil, problem.Invalid("delta", problem.SubInvalid, "more than max_stack of the item")
		}
		var school *uuid.UUID
		if st, err := h.Students.Student(ctx, a.TenantID, a.StudentID); err == nil && st != nil {
			s := st.SchoolID
			school = &s
		}
		return h.addItem(ctx, itemAdd{tenant: a.TenantID, student: a.StudentID, school: school, def: def, qty: int(a.Delta), reason: economy.ReasonAdjustment,
			sourceType: write.SourceAdjustment, sourceID: a.ID, key: key, at: at})
	}
	return uuid.Nil, problem.Invalid("kind", problem.SubInvalid, "unknown kind")
}

// AchievementEvent is a trigger event of the achievement evaluator.
type AchievementEvent struct {
	TenantID uuid.UUID
	Students []uuid.UUID
	Event    achievement.Event
}

// EvaluateAchievements is the achievement-evaluator (F075-AC-21, AC-24): for every published achievement triggered by the
// event type whose filters the payload passes, each student's progress advances once per event (a redelivered event is
// recorded and counts nothing again), achievement.progressed is written, and every tier newly reached unlocks once per
// (student, achievement, tier) — the reward grant (rewards through the appliers) and achievement.unlocked. Runs in the
// consumer transaction (TenantScope of the envelope).
func (h *Economy) EvaluateAchievements(ctx context.Context, in AchievementEvent) error {
	defs, err := h.Content.AchievementsFor(ctx, in.Event.Type)
	if err != nil || len(defs) == 0 {
		return err
	}
	at := h.now()
	for _, student := range in.Students {
		st, err := h.Students.Student(ctx, in.TenantID, student)
		if err != nil {
			return err
		}
		if st == nil {
			return ErrStudentNotProjected
		}
		if st.Deleted || st.Status != "active" {
			continue
		}
		if err := h.Store.LockStudent(ctx, student); err != nil {
			return err
		}
		school := st.SchoolID
		for _, d := range defs {
			if err := h.evaluate(ctx, in.TenantID, student, &school, d.Definition, in.Event, at); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *Economy) evaluate(ctx context.Context, tenant, student uuid.UUID, school *uuid.UUID, d achievement.Definition, e achievement.Event,
	at time.Time) error {
	if !d.Matches(e) {
		return nil
	}
	fresh, err := h.Achievements.MarkEvent(ctx, tenant, student, d.AchievementID, e.ID, e.Type, at)
	if err != nil || !fresh {
		return err
	}
	row, err := h.Achievements.State(ctx, tenant, student, d.AchievementID)
	if err != nil {
		return err
	}
	st := achievement.State{Progress: row.Progress}
	if len(row.State) > 0 {
		_ = json.Unmarshal(row.State, &st)
		st.Progress = row.Progress
	}
	if row.Tier != nil {
		st.Tier = *row.Tier
	}
	changed := d.Advance(&st, e, capZone)
	unlocked := d.NewTiers(st)
	if !changed && len(unlocked) == 0 {
		return nil
	}
	for _, t := range unlocked {
		g := write.RewardGrant{ID: ids.New(), TenantID: tenant, StudentID: student, SchoolID: school,
			TriggerEventID: uuid.NewSHA1(grantNS, []byte("achievement:"+d.AchievementID.String()+":"+t.Tier+":"+student.String())),
			TriggerKind:    write.GrantAchievement, Grants: copyGrants(t.Rewards),
			IdempotencyKey: Key("achievement", d.AchievementID.String(), t.Tier, student.String()), At: at}
		if g.Grants == nil {
			g.Grants = []map[string]any{}
		}
		ok, err := applyGrant(ctx, h.Grants, h.Registry, h.Events, g, nil)
		if err != nil {
			return err
		}
		st.Tier = t.Tier
		if !ok {
			continue // this tier was unlocked before (redelivery, another version): never twice
		}
		if row.UnlockedAt == nil {
			row.UnlockedAt = &at
		}
		if err := h.Events.Publish(ctx, write.Event{Type: EventAchievementUnlocked, SubjectType: "student", SubjectID: student.String(),
			PartitionKey: student.String(), TenantID: &tenant, OccurredAt: at, Payload: map[string]any{"achievement_id": d.AchievementID, "code": d.Code,
				"version": max(1, d.Version), "student_id": student, "tier": t.Tier, "category": d.Category, "hidden": d.Hidden(), "grant_id": g.ID,
				"unlocked_at": at}}); err != nil {
			return err
		}
		h.after(ctx, func() { h.metric(func(m EconomyMetrics) { m.AchievementUnlocked() }) })
	}
	state, err := json.Marshal(st)
	if err != nil {
		return err
	}
	var tier *string
	if st.Tier != "" {
		t := st.Tier
		tier = &t
	}
	if tier != nil && row.UnlockedAt == nil {
		row.UnlockedAt = &at
	}
	if err := h.Achievements.SaveState(ctx, tenant, student, d.AchievementID, write.AchievementState{Exists: row.Exists, Progress: st.Progress,
		State: state, Tier: tier, UnlockedAt: row.UnlockedAt, Version: max(1, d.Version)}, at); err != nil {
		return err
	}
	if !changed {
		return nil
	}
	return h.Events.Publish(ctx, write.Event{Type: EventAchievementProgress, SubjectType: "student", SubjectID: student.String(),
		PartitionKey: student.String(), TenantID: &tenant, OccurredAt: at, Payload: map[string]any{"achievement_id": d.AchievementID, "code": d.Code,
			"version": max(1, d.Version), "student_id": student, "progress": st.Progress, "target": d.Target(st), "tier": tier, "hidden": d.Hidden(),
			"progressed_at": at}})
}

// AdjustXP posts the compensating XP entry of an approved economy adjustment (F075-AC-26; reason adjustment, source
// adjustment, idempotent per adjustment): xp_total never below 0 (409 INSUFFICIENT_BALANCE), the levels a positive delta
// reaches (floor protection keeps the level on a negative one), xp.awarded. Runs in the transaction of the caller.
func (h Progression) AdjustXP(ctx context.Context, tenant, student, adjustment uuid.UUID, delta int64) (uuid.UUID, error) {
	if err := h.Store.LockStudent(ctx, student); err != nil {
		return uuid.Nil, err
	}
	state, err := h.Store.Progress(ctx, tenant, student)
	if err != nil {
		return uuid.Nil, err
	}
	if state == nil {
		if delta < 0 {
			return uuid.Nil, economy.ErrInsufficientBalance
		}
		st, err := h.Students.Student(ctx, tenant, student)
		if err != nil {
			return uuid.Nil, err
		}
		if st == nil {
			return uuid.Nil, ErrStudentNotProjected
		}
		if state, err = h.ensureProgress(ctx, tenant, student, st.SchoolID); err != nil {
			return uuid.Nil, err
		}
	}
	if state.XPTotal+delta < 0 {
		return uuid.Nil, economy.ErrInsufficientBalance
	}
	now := h.now()
	school := state.SchoolID
	e := write.XpEntry{ID: ids.New(), TenantID: tenant, StudentID: student, SchoolID: &school, Amount: int(delta), ReasonCode: write.ReasonAdjustment,
		SourceType: "adjustment", SourceCode: "economy.adjustment", SourceID: adjustment, IdempotencyKey: Key("adjustment_xp", adjustment.String()), At: now}
	ok, err := h.Store.InsertEntry(ctx, e)
	if err != nil {
		return uuid.Nil, err
	}
	if !ok {
		return uuid.Nil, problem.New(problem.InvalidStateTransition, "the adjustment is already posted")
	}
	state.XPTotal += delta
	var levels []map[string]any
	if delta > 0 {
		curve, err := h.Curves.Current(ctx)
		if err != nil {
			return uuid.Nil, err
		}
		entry := e.ID
		if levels, err = h.levelUp(ctx, tenant, student, &school, state, curve, &entry, now); err != nil {
			return uuid.Nil, err
		}
	}
	if err := h.Store.SaveProgress(ctx, tenant, student, *state); err != nil {
		return uuid.Nil, err
	}
	if err := h.Events.Publish(ctx, write.Event{Type: EventXPAwarded, SubjectType: "student", SubjectID: student.String(), PartitionKey: student.String(),
		TenantID: &tenant, OccurredAt: now, Payload: xpAwardedPayload(e, state.XPTotal, state.Level)}); err != nil {
		return uuid.Nil, err
	}
	h.push(ctx, student, pushes{levels: levels}, now)
	return e.ID, nil
}
