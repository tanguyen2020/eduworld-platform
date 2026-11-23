package command

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/collection"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/economy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// Economy events (specs/events/payloads/game/*.v1, owner F075).
const (
	EventCurrencyCredited     = "currency.credited"
	EventCurrencyDebited      = "currency.debited"
	EventItemGranted          = "item.granted"
	EventShopItemPurchased    = "shop.item_purchased"
	EventCollectionItemAdded  = "collection.item_added"
	EventCollectionCompleted  = "collection.completed"
	EventAchievementProgress  = "achievement.progressed"
	EventAchievementUnlocked  = "achievement.unlocked"
	EventEconomyAdjusted      = "economy.adjusted"
	auditShopPurchased        = "shop.item_purchased"
	auditAdjustmentCreated    = "economy_adjustment.created"
	auditAdjustmentApproved   = "economy_adjustment.approved"
	auditAdjustmentRejected   = "economy_adjustment.rejected"
	auditCurrencyMismatchName = "game.currency_ledger.mismatch"
)

// grantNS is the UUIDv5 namespace of the trigger_event_id of the F075 reward grants (achievement tier, collection
// completion, knowledge card): one grant per (student, source) whatever event triggers it.
var grantNS = uuid.MustParse("0193a5f4-6c1e-7b2a-9d1f-00000000f075")

// EconomyPolicy holds the platform parameters of the economy (CONFIG_DEFAULTS: duplicate_item_xu,
// shop_purchase_limit_default). xu_daily_cap is gamification.currencies.daily_cap.
type EconomyPolicy struct {
	Duplicate            economy.DuplicateTable
	PurchaseLimitDefault int
}

// EconomyMetrics observes the economy; nil = not observed.
type EconomyMetrics interface {
	CurrencyDelta(reason string, delta int64)
	ShopPurchase(result string)
	LedgerMismatch()
	AchievementUnlocked()
	Adjustment(status string)
}

// Economy is the write side of the in-game economy (F075): the grant appliers of Xu, items, badges and collection items
// called by the Reward Engine in its transaction, the shop purchase, the collections and knowledge cards, the achievement
// evaluator, the four-eyes economy adjustments and the wallet reconciliation. Every change of Xu or of an item goes
// through an append-only ledger in the transaction that moves the cached balance or inventory (ADR-0049); prices,
// balances and rewards are the server's (R5).
type Economy struct {
	UoW          write.UnitOfWork
	AfterCommit  write.AfterCommit
	Store        write.EconomyStore
	Grants       write.ProgressionStore // reward grants and the daily cap counters (shared with F074)
	Achievements write.AchievementStore
	Adjustments  write.AdjustmentStore
	Scan         write.EconomyScan
	Content      port.EconomyContent
	Students     port.StudentDirectory
	LiveOps      port.LiveOps
	Events       write.EventPublisher
	Audit        write.Auditor
	// Registry applies the grants of the rewards the economy gives itself (completion rewards, tiers, cards); it is the
	// registry these appliers are registered in (late bound by the composition root).
	Registry write.GrantApplierRegistry
	// XP posts the XP of an approved xp adjustment (command.Progression).
	XP      XPAdjuster
	Metrics EconomyMetrics
	Policy  EconomyPolicy
	Now     func() time.Time
}

// XPAdjuster posts a compensating XP entry (command.Progression.AdjustXP).
type XPAdjuster interface {
	AdjustXP(ctx context.Context, tenant, student, adjustment uuid.UUID, delta int64) (uuid.UUID, error)
}

func (h *Economy) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

func (h *Economy) metric(fn func(m EconomyMetrics)) {
	if h.Metrics != nil {
		fn(h.Metrics)
	}
}

func intOf(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}

// credit appends a positive Xu posting and moves the wallet (locked), then writes currency.credited. A replayed key
// adds nothing (false).
func (h *Economy) credit(ctx context.Context, e write.CurrencyEntry) (bool, error) {
	bal, err := h.Store.Wallet(ctx, e.TenantID, e.StudentID, e.Currency)
	if err != nil {
		return false, err
	}
	w := economy.Wallet{Currency: e.Currency, Balance: bal}
	if e.BalanceAfter, err = w.Apply(e.Delta); err != nil {
		return false, err
	}
	return h.post(ctx, e)
}

// post appends a Xu posting whose balance_after is set, writes the balance and the event.
func (h *Economy) post(ctx context.Context, e write.CurrencyEntry) (bool, error) {
	ok, err := h.Store.InsertCurrencyEntry(ctx, e)
	if err != nil || !ok {
		return false, err
	}
	if err := h.Store.SetBalance(ctx, e.TenantID, e.StudentID, e.Currency, e.BalanceAfter, e.At); err != nil {
		return false, err
	}
	typ := EventCurrencyCredited
	if e.Delta < 0 {
		typ = EventCurrencyDebited
	}
	payload := map[string]any{"entry_id": e.ID, "student_id": e.StudentID, "currency_code": e.Currency, "delta": e.Delta,
		"balance_after": e.BalanceAfter, "reason_code": e.ReasonCode, "source_type": e.SourceType, "source_id": e.SourceID, "item_id": e.ItemID,
		"created_at": e.At}
	if e.Delta > 0 {
		payload["capped"] = e.Capped
	}
	if err := h.Events.Publish(ctx, write.Event{Type: typ, SubjectType: "student", SubjectID: e.StudentID.String(), PartitionKey: e.StudentID.String(),
		TenantID: &e.TenantID, OccurredAt: e.At, Payload: payload}); err != nil {
		return false, err
	}
	reason, delta := e.ReasonCode, e.Delta
	h.after(ctx, func() { h.metric(func(m EconomyMetrics) { m.CurrencyDelta(reason, delta) }) })
	return true, nil
}

// after runs fn after the commit of the transaction of ctx (metrics; nothing when no AfterCommit is wired).
func (h *Economy) after(ctx context.Context, fn func()) {
	if h.AfterCommit != nil {
		_ = h.AfterCommit(ctx, func(context.Context) { fn() })
	}
}

// GrantCurrency is the applier of currency grants (F075-AC-01, AC-02): the Xu of the grant (already diminished like the
// XP by the Reward Engine) up to what the student may still earn today (currencies.daily_cap, bucket currency:xu, ICT
// day), one reward posting idempotent per (grant, position), the cap counter, currency.credited — all in the transaction
// of the RewardGrant.
func (h *Economy) GrantCurrency(ctx context.Context, gc write.GrantContext, g map[string]any) error {
	code, _ := g["currency_code"].(string)
	amount := intOf(g["amount"])
	if code != economy.Xu || amount < 1 {
		return fmt.Errorf("economy: currency grant needs currency_code xu and an amount")
	}
	cur, err := h.Content.Currency(ctx, code)
	if err != nil {
		return err
	}
	if cur == nil || !cur.Active {
		return fmt.Errorf("economy: currency %s is not active", code)
	}
	if err := h.Store.LockStudent(ctx, gc.StudentID); err != nil {
		return err
	}
	day := CapDay(gc.At)
	counters, err := h.Grants.LockCaps(ctx, gc.TenantID, gc.StudentID, day, []string{economy.CapBucket})
	if err != nil {
		return err
	}
	credit, capped := economy.Earnable(amount, counters[economy.CapBucket], cur.DailyCap)
	if credit == 0 {
		return nil // the cap took it all: no zero posting (delta ≠ 0)
	}
	ok, err := h.credit(ctx, write.CurrencyEntry{ID: ids.New(), TenantID: gc.TenantID, StudentID: gc.StudentID, Currency: code,
		Delta: int64(credit), ReasonCode: economy.ReasonReward, SourceType: write.SourceRewardGrant, SourceID: gc.GrantID, Capped: capped,
		IdempotencyKey: Key("currency", gc.GrantID.String(), itoa(gc.Index)), At: gc.At})
	if err != nil || !ok {
		return err
	}
	return h.Grants.AddCap(ctx, gc.TenantID, gc.StudentID, day, economy.CapBucket, credit)
}

// GrantItem is the applier of item, badge and collection_item grants (F075-AC-13, AC-14): the item (of kind when set)
// enters the inventory with an item posting (stackable: quantity added up to max_stack) and item.granted; a non-stackable
// item already owned converts to duplicate_item_xu Xu of its rarity (reason duplicate_conversion), noted on the grant.
func (h *Economy) GrantItem(ctx context.Context, gc write.GrantContext, g map[string]any, field, kind string) error {
	code, _ := g[field].(string)
	if code == "" {
		return fmt.Errorf("economy: %s grant without %s", g["type"], field)
	}
	defs, err := h.Content.Items(ctx, []string{code})
	if err != nil {
		return err
	}
	def, ok := defs[code]
	if !ok {
		return fmt.Errorf("economy: item %s has no published version", code)
	}
	if kind != "" && def.Kind != kind {
		return fmt.Errorf("economy: item %s is not a %s", code, kind)
	}
	if err := h.Store.LockStudent(ctx, gc.StudentID); err != nil {
		return err
	}
	owned, err := h.Store.Owned(ctx, gc.TenantID, gc.StudentID, code)
	if err != nil {
		return err
	}
	qty := max(1, intOf(g["quantity"]))
	if def.Stackable {
		qty = min(qty, max(0, def.MaxStack-owned))
	} else {
		qty = 1
	}
	if (!def.Stackable && owned > 0) || qty == 0 {
		xu := h.Policy.Duplicate.Conversion(def.Rarity)
		if xu < 1 {
			return fmt.Errorf("economy: duplicate_item_xu has no amount for rarity %s", def.Rarity)
		}
		item := code
		ok, err := h.credit(ctx, write.CurrencyEntry{ID: ids.New(), TenantID: gc.TenantID, StudentID: gc.StudentID, Currency: economy.Xu,
			Delta: int64(xu), ReasonCode: economy.ReasonDuplicateConversion, SourceType: write.SourceRewardGrant, SourceID: gc.GrantID, ItemID: &item,
			IdempotencyKey: Key("duplicate", gc.GrantID.String(), itoa(gc.Index)), At: gc.At})
		if err == nil && ok {
			g["duplicate_xu"] = xu // shown in the reward (reward.granted), F075-AC-14
		}
		return err
	}
	reason := economy.ReasonReward
	if g["type"] == "collection_item" {
		reason = economy.ReasonCollection
	}
	grant := gc.GrantID
	_, err = h.addItem(ctx, itemAdd{tenant: gc.TenantID, student: gc.StudentID, school: gc.SchoolID, def: def, qty: qty, reason: reason,
		sourceType: write.SourceRewardGrant, sourceID: gc.GrantID, grant: &grant, ruleVersion: gc.RuleVersionID,
		key: Key("item", gc.GrantID.String(), itoa(gc.Index)), at: gc.At})
	return err
}

type itemAdd struct {
	tenant, student uuid.UUID
	school          *uuid.UUID
	def             port.ItemDef
	qty             int
	reason          string
	sourceType      string
	sourceID        uuid.UUID
	grant           *uuid.UUID
	ruleVersion     *uuid.UUID
	key             string
	at              time.Time
}

// addItem appends the item posting, moves the inventory, writes item.granted and the collection progress. Returns the
// item ledger entry id (uuid.Nil on a replayed key).
func (h *Economy) addItem(ctx context.Context, in itemAdd) (uuid.UUID, error) {
	e := write.ItemEntry{ID: ids.New(), TenantID: in.tenant, StudentID: in.student, ItemID: in.def.ItemID, Delta: in.qty, ReasonCode: in.reason,
		SourceType: in.sourceType, SourceID: in.sourceID, IdempotencyKey: in.key, At: in.at}
	ok, err := h.Store.InsertItemEntry(ctx, e)
	if err != nil || !ok {
		return uuid.Nil, err
	}
	total, err := h.Store.AddInventory(ctx, in.tenant, in.student, in.school, in.def.ItemID, in.qty, in.grant, in.at)
	if err != nil {
		return uuid.Nil, err
	}
	if in.qty < 0 {
		return e.ID, nil
	}
	if err := h.Events.Publish(ctx, write.Event{Type: EventItemGranted, SubjectType: "student", SubjectID: in.student.String(),
		PartitionKey: in.student.String(), TenantID: &in.tenant, OccurredAt: in.at, Payload: map[string]any{"entry_id": e.ID, "student_id": in.student,
			"item_id": in.def.ItemID, "item_kind": in.def.Kind, "quantity": in.qty, "quantity_total": total, "reason_code": in.reason,
			"source_type": in.sourceType, "source_id": in.sourceID, "grant_id": in.grant, "rule_version_id": in.ruleVersion, "granted_at": in.at}}); err != nil {
		return uuid.Nil, err
	}
	return e.ID, h.collect(ctx, in.tenant, in.student, in.school, in.def.ItemID, in.at)
}

// collect records item in the progress of every published collection that holds it (F075-AC-18): collection.item_added
// once per item, and on completion collection.completed with the completion rewards granted once.
func (h *Economy) collect(ctx context.Context, tenant, student uuid.UUID, school *uuid.UUID, item string, at time.Time) error {
	refs, err := h.Content.CollectionsOf(ctx, item)
	if err != nil {
		return err
	}
	for _, ref := range refs {
		st, err := h.Store.Collection(ctx, tenant, student, ref.CollectionID)
		if err != nil {
			return err
		}
		p := collection.Progress{Owned: st.Owned, Completed: st.Completed}
		added, completed := p.Add(item, ref.Total)
		if !added {
			continue
		}
		var done *time.Time
		if p.Completed {
			done = &at
		}
		if err := h.Store.SaveCollection(ctx, tenant, student, ref.CollectionID, p.Owned, done, at); err != nil {
			return err
		}
		if err := h.Events.Publish(ctx, write.Event{Type: EventCollectionItemAdded, SubjectType: "student", SubjectID: student.String(),
			PartitionKey: student.String(), TenantID: &tenant, OccurredAt: at, Payload: map[string]any{"collection_id": ref.CollectionID, "code": ref.Code,
				"student_id": student, "item_id": item, "owned_count": len(p.Owned), "total_count": ref.Total, "added_at": at}}); err != nil {
			return err
		}
		if !completed {
			continue
		}
		var grantID *uuid.UUID
		if len(ref.Rewards) > 0 {
			g := write.RewardGrant{ID: ids.New(), TenantID: tenant, StudentID: student, SchoolID: school,
				TriggerEventID: uuid.NewSHA1(grantNS, []byte("collection:"+ref.CollectionID.String()+":"+student.String())), TriggerKind: write.GrantCollection,
				Grants: copyGrants(ref.Rewards), IdempotencyKey: Key("collection", ref.CollectionID.String(), student.String()), At: at}
			ok, err := applyGrant(ctx, h.Grants, h.Registry, h.Events, g, nil)
			if err != nil {
				return err
			}
			if ok {
				id := g.ID
				grantID = &id
			}
		}
		if err := h.Events.Publish(ctx, write.Event{Type: EventCollectionCompleted, SubjectType: "student", SubjectID: student.String(),
			PartitionKey: student.String(), TenantID: &tenant, OccurredAt: at, Payload: map[string]any{"collection_id": ref.CollectionID, "code": ref.Code,
				"collection_type": ref.Type, "student_id": student, "total_count": ref.Total, "grant_id": grantID, "completed_at": at}}); err != nil {
			return err
		}
	}
	return nil
}

func copyGrants(gs []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(gs))
	for _, g := range gs {
		m := make(map[string]any, len(g))
		for k, v := range g {
			m[k] = v
		}
		out = append(out, m)
	}
	return out
}

// GrantKnowledgeCard is the consumer step of learning_objective.progressed (F075-AC-19): a level of proficient or above
// grants the knowledge card of the objective once (reward grant knowledge_card, unique per (student, objective)); a lower
// level later takes nothing back. Runs in the consumer transaction (TenantScope of the envelope).
func (h *Economy) GrantKnowledgeCard(ctx context.Context, tenant, student, objective uuid.UUID, level string) error {
	if !collection.EarnsCard(level) {
		return nil
	}
	card, err := h.Content.CardFor(ctx, objective)
	if err != nil || card == nil {
		return err
	}
	st, err := h.Students.Student(ctx, tenant, student)
	if err != nil {
		return err
	}
	if st == nil {
		return ErrStudentNotProjected
	}
	if st.Deleted || st.Status != "active" {
		return nil
	}
	school := st.SchoolID
	at := h.now()
	g := write.RewardGrant{ID: ids.New(), TenantID: tenant, StudentID: student, SchoolID: &school,
		TriggerEventID: uuid.NewSHA1(grantNS, []byte("card:"+objective.String()+":"+student.String())), TriggerKind: write.GrantKnowledgeCard,
		Grants: []map[string]any{{"type": "collection_item", "collection_item_id": card.ItemID}}, IdempotencyKey: Key("knowledge_card",
			objective.String(), student.String()), At: at}
	_, err = applyGrant(ctx, h.Grants, h.Registry, h.Events, g, nil)
	return err
}

// ReconcileWallets is the nightly wallet-reconcile (F075-AC-03, INVENTORY_ECONOMY §4.2): every wallet whose balance
// differs from Σ currency_ledger.delta raises the mismatch metric and an audit record in its tenant; nothing is corrected.
func (h *Economy) ReconcileWallets(ctx context.Context, batch int) (n int, err error) {
	ps, err := authz.SystemPlatformScope("wallet-reconcile")
	if err != nil {
		return 0, err
	}
	var after *write.WalletRef
	var errs []error
	for {
		var page []write.WalletMismatch
		if err := h.UoW.InTenantTx(ctx, ps, func(ctx context.Context) error {
			var err error
			page, err = h.Scan.WalletMismatches(ctx, after, batch)
			return err
		}); err != nil {
			return n, err
		}
		for _, m := range page {
			n++
			h.metric(func(m EconomyMetrics) { m.LedgerMismatch() })
			sc, err := authz.EventScope(m.TenantID)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			mm := m
			if err := h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
				return h.Audit.Record(ctx, audit.Event{Action: auditCurrencyMismatchName, TargetType: "wallet", TargetID: mm.StudentID.String(),
					Result: audit.ResultFailure, Metadata: map[string]any{"ledger": "currency", "currency_code": mm.Currency, "cached": mm.Balance,
						"ledger_total": mm.Ledger}})
			}); err != nil {
				errs = append(errs, err)
			}
		}
		if len(page) < batch || len(page) == 0 {
			break
		}
		last := page[len(page)-1].WalletRef
		after = &last
	}
	return n, errors.Join(errs...)
}
