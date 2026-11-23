package query

import (
	"context"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/achievement"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/economy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Economy answers the economy routes of a student (F075): getWallet, listWalletLedger, listShopItems, listInventory,
// listInventoryHistory, listCollections, getCollection, listAchievements — the student is always the principal, no route
// takes another student id (F075-AC-04, AC-12) — and listEconomyAdjustments of System Admin. Read only (READ ONLY
// session); every list is a keyset page in SQL.
type Economy struct {
	Read    read.Session
	Data    read.EconomyReadModel
	Cursors *page.Codec
	Now     func() time.Time
}

func (h Economy) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

// WalletView is Wallet of the contract.
type WalletView struct {
	CurrencyCode       string `json:"currency_code"`
	Balance            int64  `json:"balance"`
	DailyEarnRemaining int    `json:"daily_earn_remaining"`
}

// LedgerView is CurrencyLedgerEntry of the contract: never source_id, entry_id or the key (F075-AC-04).
type LedgerView struct {
	CurrencyCode string    `json:"currency_code"`
	Delta        int64     `json:"delta"`
	BalanceAfter int64     `json:"balance_after"`
	ReasonCode   string    `json:"reason_code"`
	SourceLabel  string    `json:"source_label,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// ItemHistoryView is ItemLedgerEntry of the contract.
type ItemHistoryView struct {
	ItemID      string    `json:"item_id"`
	ItemName    string    `json:"item_name,omitempty"`
	Kind        string    `json:"kind,omitempty"`
	Delta       int       `json:"delta"`
	ReasonCode  string    `json:"reason_code"`
	SourceLabel string    `json:"source_label,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// InventoryView is InventoryItem of the contract.
type InventoryView struct {
	ItemID     string            `json:"item_id"`
	Kind       string            `json:"kind"`
	Rarity     string            `json:"rarity,omitempty"`
	EquipSlot  string            `json:"equip_slot,omitempty"`
	Quantity   int               `json:"quantity"`
	AcquiredAt time.Time         `json:"acquired_at"`
	Name       map[string]string `json:"name,omitempty"`
}

// ShopItemView is ShopItem of the contract (price in Xu only — never a real-money price).
type ShopItemView struct {
	ShopItemID     uuid.UUID         `json:"shop_item_id"`
	ItemID         string            `json:"item_id"`
	Name           map[string]string `json:"name"`
	Description    map[string]string `json:"description,omitempty"`
	Kind           string            `json:"kind"`
	Rarity         string            `json:"rarity"`
	Price          int               `json:"price"`
	CurrencyCode   string            `json:"currency_code"`
	PurchaseLimit  int               `json:"purchase_limit"`
	PurchasedCount int               `json:"purchased_count"`
	Owned          bool              `json:"owned"`
	AvailableTo    *time.Time        `json:"available_to,omitempty"`
}

// CollectionView is CollectionProgress of the contract.
type CollectionView struct {
	CollectionID uuid.UUID         `json:"collection_id"`
	Code         string            `json:"code"`
	Title        map[string]string `json:"title,omitempty"`
	Type         string            `json:"type"`
	OwnedCount   int               `json:"owned_count"`
	TotalCount   int               `json:"total_count"`
	CompletedAt  *time.Time        `json:"completed_at,omitempty"`
}

// CollectionItemDetail is one item of CollectionDetail.
type CollectionItemDetail struct {
	ItemID      string `json:"item_id"`
	Name        string `json:"name,omitempty"`
	Rarity      string `json:"rarity,omitempty"`
	Owned       bool   `json:"owned"`
	HowToObtain string `json:"how_to_obtain,omitempty"`
}

// CollectionDetailView is CollectionDetail of the contract.
type CollectionDetailView struct {
	CollectionView
	Items             []CollectionItemDetail `json:"items"`
	CompletionRewards []RewardSummary        `json:"completion_rewards"`
}

// AchievementView is AchievementProgress of the contract.
type AchievementView struct {
	AchievementID   uuid.UUID         `json:"achievement_id"`
	Code            string            `json:"code"`
	Title           map[string]string `json:"title,omitempty"`
	Description     map[string]string `json:"description,omitempty"`
	Category        string            `json:"category"`
	Unlocked        bool              `json:"unlocked"`
	UnlockedAt      *time.Time        `json:"unlocked_at,omitempty"`
	Tier            string            `json:"tier"`
	ProgressCurrent int               `json:"progress_current"`
	ProgressTarget  int               `json:"progress_target"`
	Rewards         []RewardSummary   `json:"rewards"`
}

// Page is a cursor page of the contract.
type Page[T any] struct {
	Data []T      `json:"data"`
	Page PageInfo `json:"page"`
}

// sourceLabels are the friendly vi/en labels of the ledger reasons (F075-AC-04: no technical id).
var sourceLabels = map[string][2]string{
	economy.ReasonReward:              {"Phần thưởng", "Reward"},
	economy.ReasonShopPurchase:        {"Mua ở Cửa hàng", "Shop purchase"},
	economy.ReasonAdjustment:          {"Điều chỉnh", "Adjustment"},
	economy.ReasonRefund:              {"Hoàn Xu", "Refund"},
	economy.ReasonDuplicateConversion: {"Quy đổi vật phẩm trùng", "Duplicate item converted"},
	economy.ReasonCollection:          {"Bộ sưu tập", "Collection"},
}

func label(reason, lang string) string {
	l, ok := sourceLabels[reason]
	if !ok {
		return ""
	}
	if lang == "en" {
		return l[1]
	}
	return l[0]
}

func pick(m map[string]string, lang string) string {
	if lang == "en" && m["en"] != "" {
		return m["en"]
	}
	return m["vi"]
}

func limitOf(limit int) (int, error) {
	if limit != 0 && (limit < 1 || limit > page.MaxLimit) {
		return 0, problem.Invalid("limit", problem.SubInvalid, "must be 1..100")
	}
	return page.Limit(limit), nil
}

func (h Economy) decode(cursor string) (*page.Cursor, error) {
	if cursor == "" || h.Cursors == nil {
		return nil, nil
	}
	c, err := h.Cursors.Decode(cursor)
	if err != nil || c == nil {
		return nil, problem.Invalid("cursor", problem.SubInvalid, "malformed cursor")
	}
	return c, nil
}

func (h Economy) timeCursor(cursor string) (*read.TimeCursor, error) {
	c, err := h.decode(cursor)
	if err != nil || c == nil {
		return nil, err
	}
	at, err := time.Parse(time.RFC3339Nano, c.Key)
	if err != nil {
		return nil, problem.Invalid("cursor", problem.SubInvalid, "malformed cursor")
	}
	return &read.TimeCursor{At: at, ID: c.ID}, nil
}

func (h Economy) keyCursor(cursor string) (string, error) {
	c, err := h.decode(cursor)
	if err != nil || c == nil {
		return "", err
	}
	return c.Key, nil
}

func (h Economy) next(key string, id uuid.UUID) *string {
	if h.Cursors == nil {
		return nil
	}
	if id == uuid.Nil {
		id = uuid.Max // key-only cursor (codes): the codec requires an id, never read back
	}
	s := h.Cursors.Encode(page.Cursor{Key: key, ID: id})
	return &s
}

// GetWallet implements getWallet (F075-AC-04): the Xu balance of the principal and the Xu it may still earn today.
func (h Economy) GetWallet(ctx context.Context) (out WalletView, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.GetWallet")
	defer func() { done(err) }()
	id, tenant, err := student(ctx)
	if err != nil {
		return out, err
	}
	out.CurrencyCode = economy.Xu
	err = h.Read.Read(ctx, func(ctx context.Context) error {
		var err error
		if out.Balance, err = h.Data.Balance(ctx, tenant, id, economy.Xu); err != nil {
			return err
		}
		cap, err := h.Data.DailyCap(ctx, economy.Xu)
		if err != nil {
			return err
		}
		earned, err := h.Data.EarnedToday(ctx, tenant, id, h.now().In(ict).Format(time.DateOnly), economy.CapBucket)
		out.DailyEarnRemaining = economy.Remaining(earned, cap)
		return err
	})
	return out, err
}

// ListWalletLedger implements listWalletLedger (F075-AC-04): the Xu postings of the principal newest first, friendly
// labels, no technical id.
func (h Economy) ListWalletLedger(ctx context.Context, limit int, cursor, lang string) (out Page[LedgerView], err error) {
	ctx, done := observability.UseCase(ctx, "game.query.ListWalletLedger")
	defer func() { done(err) }()
	id, tenant, err := student(ctx)
	if err != nil {
		return out, err
	}
	if limit, err = limitOf(limit); err != nil {
		return out, err
	}
	after, err := h.timeCursor(cursor)
	if err != nil {
		return out, err
	}
	var rows []read.CurrencyLedgerRow
	if err := h.Read.Read(ctx, func(ctx context.Context) error {
		var err error
		rows, err = h.Data.CurrencyLedger(ctx, tenant, id, after, limit+1)
		return err
	}); err != nil {
		return out, err
	}
	out = Page[LedgerView]{Data: []LedgerView{}, Page: PageInfo{Limit: limit}}
	if len(rows) > limit {
		rows, out.Page.HasMore = rows[:limit], true
		last := rows[len(rows)-1]
		out.Page.NextCursor = h.next(last.CreatedAt.Format(time.RFC3339Nano), last.ID)
	}
	for _, r := range rows {
		out.Data = append(out.Data, LedgerView{CurrencyCode: r.Currency, Delta: r.Delta, BalanceAfter: r.BalanceAfter, ReasonCode: r.ReasonCode,
			SourceLabel: label(r.ReasonCode, lang), CreatedAt: r.CreatedAt})
	}
	return out, nil
}

// ListInventoryHistory implements listInventoryHistory (F075-AC-15).
func (h Economy) ListInventoryHistory(ctx context.Context, limit int, cursor, lang string) (out Page[ItemHistoryView], err error) {
	ctx, done := observability.UseCase(ctx, "game.query.ListInventoryHistory")
	defer func() { done(err) }()
	id, tenant, err := student(ctx)
	if err != nil {
		return out, err
	}
	if limit, err = limitOf(limit); err != nil {
		return out, err
	}
	after, err := h.timeCursor(cursor)
	if err != nil {
		return out, err
	}
	var rows []read.ItemHistoryRow
	if err := h.Read.Read(ctx, func(ctx context.Context) error {
		var err error
		rows, err = h.Data.ItemHistory(ctx, tenant, id, after, limit+1)
		return err
	}); err != nil {
		return out, err
	}
	out = Page[ItemHistoryView]{Data: []ItemHistoryView{}, Page: PageInfo{Limit: limit}}
	if len(rows) > limit {
		rows, out.Page.HasMore = rows[:limit], true
		last := rows[len(rows)-1]
		out.Page.NextCursor = h.next(last.CreatedAt.Format(time.RFC3339Nano), last.ID)
	}
	for _, r := range rows {
		out.Data = append(out.Data, ItemHistoryView{ItemID: r.ItemID, ItemName: pick(r.Name, lang), Kind: r.Kind, Delta: r.Delta, ReasonCode: r.ReasonCode,
			SourceLabel: label(r.ReasonCode, lang), CreatedAt: r.CreatedAt})
	}
	return out, nil
}

// ListInventory implements listInventory (F075-AC-15): the items of the principal by code, optional kind.
func (h Economy) ListInventory(ctx context.Context, kind string, limit int, cursor string) (out Page[InventoryView], err error) {
	ctx, done := observability.UseCase(ctx, "game.query.ListInventory")
	defer func() { done(err) }()
	id, tenant, err := student(ctx)
	if err != nil {
		return out, err
	}
	if limit, err = limitOf(limit); err != nil {
		return out, err
	}
	after, err := h.keyCursor(cursor)
	if err != nil {
		return out, err
	}
	var rows []read.InventoryRow
	if err := h.Read.Read(ctx, func(ctx context.Context) error {
		var err error
		rows, err = h.Data.Inventory(ctx, tenant, id, kind, after, limit+1)
		return err
	}); err != nil {
		return out, err
	}
	out = Page[InventoryView]{Data: []InventoryView{}, Page: PageInfo{Limit: limit}}
	if len(rows) > limit {
		rows, out.Page.HasMore = rows[:limit], true
		out.Page.NextCursor = h.next(rows[len(rows)-1].ItemID, uuid.Nil)
	}
	for _, r := range rows {
		out.Data = append(out.Data, InventoryView{ItemID: r.ItemID, Kind: r.Kind, Rarity: r.Rarity, EquipSlot: r.EquipSlot, Quantity: r.Quantity,
			AcquiredAt: r.AcquiredAt, Name: r.Name})
	}
	return out, nil
}

// ListShopItems implements listShopItems (F075-AC-06): the listings on sale for the principal (tenant, grade, window),
// server price in Xu, limit, how many it bought, whether it owns the item.
func (h Economy) ListShopItems(ctx context.Context, kind string, limit int, cursor string) (out Page[ShopItemView], err error) {
	ctx, done := observability.UseCase(ctx, "game.query.ListShopItems")
	defer func() { done(err) }()
	id, tenant, err := student(ctx)
	if err != nil {
		return out, err
	}
	if limit, err = limitOf(limit); err != nil {
		return out, err
	}
	c, err := h.decode(cursor)
	if err != nil {
		return out, err
	}
	f := read.ShopFilter{Tenant: tenant, Student: id, Kind: kind, Now: h.now(), Limit: limit + 1}
	if c != nil {
		n, err := strconv.Atoi(c.Key)
		if err != nil {
			return out, problem.Invalid("cursor", problem.SubInvalid, "malformed cursor")
		}
		f.AfterOrder, f.AfterID = &n, c.ID
	}
	var rows []read.ShopRow
	if err := h.Read.Read(ctx, func(ctx context.Context) error {
		grade, err := h.Data.Grade(ctx, tenant, id)
		if err != nil {
			return err
		}
		f.Grade = grade
		rows, err = h.Data.Shop(ctx, f)
		return err
	}); err != nil {
		return out, err
	}
	out = Page[ShopItemView]{Data: []ShopItemView{}, Page: PageInfo{Limit: limit}}
	if len(rows) > limit {
		rows, out.Page.HasMore = rows[:limit], true
		last := rows[len(rows)-1]
		out.Page.NextCursor = h.next(strconv.Itoa(last.FeaturedOrder), last.ShopItemID)
	}
	for _, r := range rows {
		out.Data = append(out.Data, ShopItemView{ShopItemID: r.ShopItemID, ItemID: r.ItemID, Name: r.Name, Description: r.Description, Kind: r.Kind,
			Rarity: r.Rarity, Price: r.Price, CurrencyCode: r.Currency, PurchaseLimit: r.PurchaseLimit, PurchasedCount: r.Purchased, Owned: r.Owned,
			AvailableTo: r.AvailableTo})
	}
	return out, nil
}

func collectionView(r read.CollectionRow) CollectionView {
	return CollectionView{CollectionID: r.CollectionID, Code: r.Code, Title: r.Title, Type: r.Type, OwnedCount: len(r.Owned), TotalCount: r.Total,
		CompletedAt: r.CompletedAt}
}

// ListCollections implements listCollections (F075-AC-20): visible collections of the principal with its progress; a
// hidden_until_first collection appears with the first item.
func (h Economy) ListCollections(ctx context.Context, limit int, cursor string) (out Page[CollectionView], err error) {
	ctx, done := observability.UseCase(ctx, "game.query.ListCollections")
	defer func() { done(err) }()
	id, tenant, err := student(ctx)
	if err != nil {
		return out, err
	}
	if limit, err = limitOf(limit); err != nil {
		return out, err
	}
	after, err := h.keyCursor(cursor)
	if err != nil {
		return out, err
	}
	var rows []read.CollectionRow
	if err := h.Read.Read(ctx, func(ctx context.Context) error {
		var err error
		rows, err = h.Data.Collections(ctx, tenant, id, after, limit+1)
		return err
	}); err != nil {
		return out, err
	}
	out = Page[CollectionView]{Data: []CollectionView{}, Page: PageInfo{Limit: limit}}
	if len(rows) > limit {
		rows, out.Page.HasMore = rows[:limit], true
		out.Page.NextCursor = h.next(rows[len(rows)-1].Code, uuid.Nil)
	}
	for _, r := range rows {
		out.Data = append(out.Data, collectionView(r))
	}
	return out, nil
}

// GetCollection implements getCollection (F075-AC-20): the items in order with the public "how to obtain" (vi/en) and
// whether the principal owns each; a hidden collection without an item yet is 404 (no existence leak).
func (h Economy) GetCollection(ctx context.Context, collection uuid.UUID, lang string) (out CollectionDetailView, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.GetCollection")
	defer func() { done(err) }()
	id, tenant, err := student(ctx)
	if err != nil {
		return out, err
	}
	var row *read.CollectionRow
	var items []read.CollectionItemView
	if err := h.Read.Read(ctx, func(ctx context.Context) error {
		var err error
		row, items, err = h.Data.Collection(ctx, tenant, id, collection)
		return err
	}); err != nil {
		return out, err
	}
	if row == nil {
		return out, problem.New(problem.NotFound, "")
	}
	owned := map[string]bool{}
	for _, o := range row.Owned {
		owned[o] = true
	}
	out = CollectionDetailView{CollectionView: collectionView(*row), Items: []CollectionItemDetail{}, CompletionRewards: rewardSummaries(row.Rewards)}
	for _, it := range items {
		out.Items = append(out.Items, CollectionItemDetail{ItemID: it.ItemID, Name: pick(it.Name, lang), Rarity: it.Rarity, Owned: owned[it.ItemID],
			HowToObtain: pick(it.Hint, lang)})
	}
	return out, nil
}

func rewardSummaries(rs []map[string]any) []RewardSummary {
	out := make([]RewardSummary, 0, len(rs))
	for _, r := range rs {
		kind, _ := r["type"].(string)
		s := RewardSummary{Kind: kind}
		if n, ok := r["amount"].(float64); ok {
			v := int(n)
			s.Amount = &v
		}
		if c, ok := r["currency_code"].(string); ok {
			s.CurrencyCode = c
		}
		for _, f := range []string{"item_id", "badge_id", "collection_item_id"} {
			if c, ok := r[f].(string); ok && c != "" {
				s.ItemID = &c
			}
		}
		out = append(out, s)
	}
	return out
}

// ListAchievements implements listAchievements (F075-AC-22): published achievements with the progress of the principal;
// a hidden achievement appears only once unlocked (never its conditions, progress or name before).
func (h Economy) ListAchievements(ctx context.Context, category string, limit int, cursor string) (out Page[AchievementView], err error) {
	ctx, done := observability.UseCase(ctx, "game.query.ListAchievements")
	defer func() { done(err) }()
	id, tenant, err := student(ctx)
	if err != nil {
		return out, err
	}
	if limit, err = limitOf(limit); err != nil {
		return out, err
	}
	after, err := h.keyCursor(cursor)
	if err != nil {
		return out, err
	}
	var rows []read.AchievementView
	if err := h.Read.Read(ctx, func(ctx context.Context) error {
		var err error
		rows, err = h.Data.Achievements(ctx, tenant, id, category, after, limit+1)
		return err
	}); err != nil {
		return out, err
	}
	out = Page[AchievementView]{Data: []AchievementView{}, Page: PageInfo{Limit: limit}}
	if len(rows) > limit {
		rows, out.Page.HasMore = rows[:limit], true
		out.Page.NextCursor = h.next(rows[len(rows)-1].Code, uuid.Nil)
	}
	for _, r := range rows {
		d, err := achievement.Parse(r.Payload)
		if err != nil {
			return out, err
		}
		st := achievement.State{Progress: r.Progress}
		v := AchievementView{AchievementID: r.AchievementID, Code: d.Code, Title: map[string]string{"vi": d.Name.VI, "en": d.Name.EN}, Category: d.Category,
			Unlocked: r.Tier != nil, UnlockedAt: r.UnlockedAt, Tier: "none", ProgressCurrent: r.Progress, Rewards: []RewardSummary{}}
		if d.Description != nil {
			v.Description = map[string]string{"vi": d.Description.VI, "en": d.Description.EN}
		}
		if r.Tier != nil {
			v.Tier, st.Tier = *r.Tier, *r.Tier
		}
		if t := d.Target(st); t != nil {
			v.ProgressTarget = *t
			for _, tier := range d.Tiers {
				if tier.Threshold == *t {
					v.Rewards = rewardSummaries(tier.Rewards)
				}
			}
		} else if n := len(d.Tiers); n > 0 {
			v.ProgressTarget = d.Tiers[n-1].Threshold
		}
		out.Data = append(out.Data, v)
	}
	return out, nil
}

// AdjustmentsInput filters listEconomyAdjustments.
type AdjustmentsInput struct {
	Status    string
	StudentID *uuid.UUID
	Limit     int
	Cursor    string
}

// ListAdjustments implements listEconomyAdjustments: the adjustments a second person reviews (game.economy.adjust, System
// Admin), in the tenants of tenant_ids (CrossTenantScope) or every tenant (audited bypass of the closed list), newest first.
func (h Economy) ListAdjustments(ctx context.Context, in AdjustmentsInput) (out Page[read.AdjustmentRecord], err error) {
	ctx, done := observability.UseCase(ctx, "game.query.ListEconomyAdjustments")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	if err := authz.Require(ctx, p, authz.PermGameEconomyAdjust, authz.Resource{}); err != nil {
		return out, err
	}
	if !p.HasPlatformRole() {
		authz.RecordDenied(ctx, string(problem.PermissionDenied), map[string]any{"why": "platform_role_required"})
		return out, problem.New(problem.PermissionDenied, "")
	}
	limit, err := limitOf(in.Limit)
	if err != nil {
		return out, err
	}
	after, err := h.timeCursor(in.Cursor)
	if err != nil {
		return out, err
	}
	f := read.AdjustmentFilter{Status: in.Status, StudentID: in.StudentID, After: after, Limit: limit + 1}
	sc, ok := tenancy.ScopeFrom(ctx)
	if !ok {
		return out, problem.New(problem.PermissionDenied, "")
	}
	switch s := sc.(type) {
	case tenancy.CrossTenantScope:
		f.TenantIDs = s.TenantIDs()
	case tenancy.PlatformScope:
		if !p.Has(authz.PermPlatformCrossTenantRead) {
			return out, problem.New(problem.PermissionDenied, "")
		}
		if ctx, err = tenancy.Bypass(tenancy.WithModule(ctx, "game"), tenancy.ReasonSystemAdminCrossTenant, tenancy.WithMode(tenancy.ModeRead)); err != nil {
			return out, err
		}
		f.AllTenants = true
	default:
		return out, problem.New(problem.PermissionDenied, "")
	}
	var rows []read.AdjustmentRecord
	if err := h.Read.Read(ctx, func(ctx context.Context) error {
		var err error
		rows, err = h.Data.Adjustments(ctx, f)
		return err
	}); err != nil {
		return out, err
	}
	out = Page[read.AdjustmentRecord]{Data: []read.AdjustmentRecord{}, Page: PageInfo{Limit: limit}}
	if len(rows) > limit {
		rows, out.Page.HasMore = rows[:limit], true
		last := rows[len(rows)-1]
		out.Page.NextCursor = h.next(last.CreatedAt.Format(time.RFC3339Nano), last.ID)
	}
	out.Data = append(out.Data, rows...)
	return out, nil
}
