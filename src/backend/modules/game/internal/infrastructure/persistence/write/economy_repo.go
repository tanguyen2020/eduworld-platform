package write

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// EconomyRepo implements write.EconomyStore and write.EconomyScan on gamification.wallets, currency_ledger, item_ledger,
// shop_purchases, collection_progress and game.inventory_items (F075). The ledgers and the purchases are only ever
// inserted (R21, F075-AC-05).
type EconomyRepo struct{}

var (
	_ write.EconomyStore        = EconomyRepo{}
	_ write.EconomyScan         = EconomyRepo{}
	_ write.CollectionItemStore = GameContentRepo{}
)

// LockStudent implements write.EconomyStore.
func (EconomyRepo) LockStudent(ctx context.Context, student uuid.UUID) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok advisory lock only (no table read)
	if _, err := t.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('game.eco:' || $1::text, 0))`, student); err != nil {
		return fmt.Errorf("economy: lock student: %w", err)
	}
	return nil
}

// Wallet implements write.EconomyStore.
func (EconomyRepo) Wallet(ctx context.Context, tenant, student uuid.UUID, currency string) (int64, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, err
	}
	now := time.Now().UTC()
	// tenancyguard:ok insert under the TenantScope (RLS WITH CHECK); the conflict target is the live-row index
	if _, err := t.ExecContext(ctx, `INSERT INTO gamification.wallets (wallet_id, tenant_id, created_at, created_by, updated_at, student_id,
			currency_code, balance) VALUES ($1, $2, $3, $4, $3, $5, $6, 0)
		ON CONFLICT (tenant_id, student_id, currency_code) WHERE deleted_at IS NULL DO NOTHING`, ids.New(), tenant, now, actor(ctx), student,
		currency); err != nil {
		return 0, fmt.Errorf("economy: wallet: %w", err)
	}
	var bal int64
	// tenancyguard:ok tenant table gamification.wallets: explicit tenant_id + deleted_at IS NULL (RLS as well)
	if err := t.QueryRowContext(ctx, `SELECT balance FROM gamification.wallets WHERE tenant_id = $1 AND student_id = $2 AND currency_code = $3
		AND deleted_at IS NULL FOR UPDATE`, tenant, student, currency).Scan(&bal); err != nil {
		return 0, fmt.Errorf("economy: lock wallet: %w", err)
	}
	return bal, nil
}

// SetBalance implements write.EconomyStore.
func (EconomyRepo) SetBalance(ctx context.Context, tenant, student uuid.UUID, currency string, balance int64, at time.Time) error {
	// tenancyguard:ok tenant table gamification.wallets: explicit tenant_id + deleted_at IS NULL (RLS as well)
	ok, err := exec(ctx, `UPDATE gamification.wallets SET balance = $4, updated_at = $5, updated_by = $6, version = version + 1
		WHERE tenant_id = $1 AND student_id = $2 AND currency_code = $3 AND deleted_at IS NULL`, tenant, student, currency, balance, at.UTC(), actor(ctx))
	if err != nil {
		return fmt.Errorf("economy: set balance: %w", err)
	}
	if !ok {
		return errors.New("economy: set balance: no live wallet")
	}
	return nil
}

// InsertCurrencyEntry implements write.EconomyStore.
func (EconomyRepo) InsertCurrencyEntry(ctx context.Context, e write.CurrencyEntry) (bool, error) {
	// tenancyguard:ok append-only insert under the TenantScope (RLS WITH CHECK); the idempotency key makes a replay a no-op
	ok, err := exec(ctx, `INSERT INTO gamification.currency_ledger (entry_id, tenant_id, created_at, created_by, student_id, currency_code, delta,
			balance_after, reason_code, source_type, source_id, item_id, capped, idempotency_key)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14) ON CONFLICT (idempotency_key) DO NOTHING`,
		e.ID, e.TenantID, e.At.UTC(), actor(ctx), e.StudentID, e.Currency, e.Delta, e.BalanceAfter, e.ReasonCode, e.SourceType, e.SourceID, e.ItemID,
		e.Capped, e.IdempotencyKey)
	if err != nil {
		return false, fmt.Errorf("economy: currency ledger: %w", err)
	}
	return ok, nil
}

// InsertItemEntry implements write.EconomyStore.
func (EconomyRepo) InsertItemEntry(ctx context.Context, e write.ItemEntry) (bool, error) {
	// tenancyguard:ok append-only insert under the TenantScope (RLS WITH CHECK); the idempotency key makes a replay a no-op
	ok, err := exec(ctx, `INSERT INTO gamification.item_ledger (entry_id, tenant_id, created_at, created_by, student_id, item_id, delta,
			reason_code, source_type, source_id, idempotency_key) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (idempotency_key) DO NOTHING`,
		e.ID, e.TenantID, e.At.UTC(), actor(ctx), e.StudentID, e.ItemID, e.Delta, e.ReasonCode, e.SourceType, e.SourceID, e.IdempotencyKey)
	if err != nil {
		return false, fmt.Errorf("economy: item ledger: %w", err)
	}
	return ok, nil
}

// Owned implements write.EconomyStore.
func (EconomyRepo) Owned(ctx context.Context, tenant, student uuid.UUID, item string) (int, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, err
	}
	var n int
	// tenancyguard:ok tenant table game.inventory_items: explicit tenant_id + deleted_at IS NULL (RLS as well)
	err = t.QueryRowContext(ctx, `SELECT quantity FROM game.inventory_items WHERE tenant_id = $1 AND student_id = $2 AND item_id = $3
		AND deleted_at IS NULL FOR UPDATE`, tenant, student, item).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("economy: owned: %w", err)
	}
	return n, nil
}

// AddInventory implements write.EconomyStore.
func (EconomyRepo) AddInventory(ctx context.Context, tenant, student uuid.UUID, school *uuid.UUID, item string, qty int, grant *uuid.UUID,
	at time.Time) (int, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, err
	}
	var n int
	// tenancyguard:ok tenant table game.inventory_items: explicit tenant_id + deleted_at IS NULL (RLS as well)
	err = t.QueryRowContext(ctx, `UPDATE game.inventory_items SET quantity = GREATEST(0, quantity + $4), updated_at = $5, updated_by = $6,
			version = version + 1, acquired_at = CASE WHEN quantity = 0 AND $4 > 0 THEN $5 ELSE acquired_at END,
			reward_grant_id = CASE WHEN quantity = 0 AND $4 > 0 THEN $7 ELSE reward_grant_id END
		WHERE tenant_id = $1 AND student_id = $2 AND item_id = $3 AND deleted_at IS NULL RETURNING quantity`,
		tenant, student, item, qty, at.UTC(), actor(ctx), grant).Scan(&n)
	if err == nil {
		return n, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("economy: inventory: %w", err)
	}
	if qty <= 0 {
		return 0, errors.New("economy: inventory: nothing to remove")
	}
	// tenancyguard:ok insert under the TenantScope (RLS WITH CHECK); one live row per (student, item)
	if _, err := t.ExecContext(ctx, `INSERT INTO game.inventory_items (inventory_item_id, tenant_id, created_at, created_by, updated_at, student_id,
			item_id, school_id, quantity, acquired_at, reward_grant_id) VALUES ($1, $2, $3, $4, $3, $5, $6, $7, $8, $3, $9)`,
		ids.New(), tenant, at.UTC(), actor(ctx), student, item, school, qty, grant); err != nil {
		return 0, fmt.Errorf("economy: inventory: %w", err)
	}
	return qty, nil
}

// Purchased implements write.EconomyStore.
func (EconomyRepo) Purchased(ctx context.Context, tenant, student, shopItem uuid.UUID) (int, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, err
	}
	var n int
	// tenancyguard:ok append-only tenant table gamification.shop_purchases (no deleted_at): explicit tenant_id (RLS as well)
	if err := t.QueryRowContext(ctx, `SELECT coalesce(sum(quantity), 0) FROM gamification.shop_purchases
		WHERE tenant_id = $1 AND student_id = $2 AND shop_item_id = $3`, tenant, student, shopItem).Scan(&n); err != nil {
		return 0, fmt.Errorf("economy: purchased: %w", err)
	}
	return n, nil
}

// InsertPurchase implements write.EconomyStore.
func (EconomyRepo) InsertPurchase(ctx context.Context, p write.Purchase) error {
	// tenancyguard:ok append-only insert under the TenantScope (RLS WITH CHECK)
	_, err := exec(ctx, `INSERT INTO gamification.shop_purchases (purchase_id, tenant_id, created_at, created_by, student_id, shop_item_id,
			shop_item_version_id, item_id, price_amount, quantity, currency_ledger_entry_id, idempotency_key)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`, p.ID, p.TenantID, p.At.UTC(), actor(ctx), p.StudentID, p.ShopItemID, p.VersionID,
		p.ItemID, p.Price, p.Quantity, p.EntryID, p.IdempotencyKey)
	if err != nil {
		return fmt.Errorf("economy: purchase: %w", err)
	}
	return nil
}

// Collection implements write.EconomyStore.
func (EconomyRepo) Collection(ctx context.Context, tenant, student, coll uuid.UUID) (write.CollectionState, error) {
	t, err := tx(ctx)
	if err != nil {
		return write.CollectionState{}, err
	}
	var raw []byte
	var done sql.NullTime
	// tenancyguard:ok tenant table gamification.collection_progress: explicit tenant_id + deleted_at IS NULL (RLS as well)
	err = t.QueryRowContext(ctx, `SELECT owned_item_ids, completed_at FROM gamification.collection_progress
		WHERE tenant_id = $1 AND student_id = $2 AND collection_id = $3 AND deleted_at IS NULL FOR UPDATE`, tenant, student, coll).Scan(&raw, &done)
	if errors.Is(err, sql.ErrNoRows) {
		return write.CollectionState{}, nil
	}
	if err != nil {
		return write.CollectionState{}, fmt.Errorf("economy: collection progress: %w", err)
	}
	st := write.CollectionState{Exists: true, Completed: done.Valid}
	if err := json.Unmarshal(raw, &st.Owned); err != nil {
		return st, fmt.Errorf("economy: collection progress: %w", err)
	}
	return st, nil
}

// SaveCollection implements write.EconomyStore.
func (EconomyRepo) SaveCollection(ctx context.Context, tenant, student, coll uuid.UUID, owned []string, completedAt *time.Time, at time.Time) error {
	raw, err := json.Marshal(owned)
	if err != nil {
		return err
	}
	// tenancyguard:ok upsert under the TenantScope (RLS insert/update policies); the conflict target is the live-row index
	_, err = exec(ctx, `INSERT INTO gamification.collection_progress AS c (collection_progress_id, tenant_id, created_at, created_by, updated_at,
			student_id, collection_id, owned_item_ids, owned_count, completed_at) VALUES ($1, $2, $3, $4, $3, $5, $6, $7::jsonb, $8, $9)
		ON CONFLICT (tenant_id, student_id, collection_id) WHERE deleted_at IS NULL DO UPDATE SET owned_item_ids = EXCLUDED.owned_item_ids,
			owned_count = EXCLUDED.owned_count, completed_at = coalesce(c.completed_at, EXCLUDED.completed_at), updated_at = EXCLUDED.updated_at,
			updated_by = EXCLUDED.created_by, version = c.version + 1`,
		ids.New(), tenant, at.UTC(), actor(ctx), student, coll, string(raw), len(owned), completedAt)
	if err != nil {
		return fmt.Errorf("economy: save collection progress: %w", err)
	}
	return nil
}

// CurrencyTotal implements write.EconomyStore.
func (EconomyRepo) CurrencyTotal(ctx context.Context, tenant, student uuid.UUID, currency string) (int64, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, err
	}
	var n int64
	// tenancyguard:ok append-only tenant table gamification.currency_ledger (no deleted_at): explicit tenant_id (RLS as well)
	if err := t.QueryRowContext(ctx, `SELECT coalesce(sum(delta), 0) FROM gamification.currency_ledger
		WHERE tenant_id = $1 AND student_id = $2 AND currency_code = $3`, tenant, student, currency).Scan(&n); err != nil {
		return 0, fmt.Errorf("economy: currency total: %w", err)
	}
	return n, nil
}

// WalletMismatches implements write.EconomyScan.
func (EconomyRepo) WalletMismatches(ctx context.Context, after *write.WalletRef, limit int) ([]write.WalletMismatch, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	at, as, ac := uuid.Nil, uuid.Nil, ""
	if after != nil {
		at, as, ac = after.TenantID, after.StudentID, after.Currency
	}
	// tenancyguard:ok platform job scope: compares every live wallet with its ledger (read only); deleted_at filtered
	rows, err := t.QueryContext(ctx, `SELECT w.tenant_id, w.student_id, w.currency_code, w.balance, coalesce(l.total, 0)
		FROM gamification.wallets w
		LEFT JOIN LATERAL (SELECT sum(delta) AS total FROM gamification.currency_ledger x WHERE x.tenant_id = w.tenant_id
			AND x.student_id = w.student_id AND x.currency_code = w.currency_code) l ON true
		WHERE w.deleted_at IS NULL AND (w.tenant_id, w.student_id, w.currency_code) > ($1, $2, $3) AND w.balance <> coalesce(l.total, 0)
		ORDER BY w.tenant_id, w.student_id, w.currency_code LIMIT $4`, at, as, ac, limit)
	if err != nil {
		return nil, fmt.Errorf("economy: wallet mismatches: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.WalletMismatch
	for rows.Next() {
		var m write.WalletMismatch
		if err := rows.Scan(&m.TenantID, &m.StudentID, &m.Currency, &m.Balance, &m.Ledger); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// InsertCollectionItems implements write.CollectionItemStore.
func (GameContentRepo) InsertCollectionItems(ctx context.Context, coll, versionID uuid.UUID, items []write.CollectionItemRow) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, it := range items {
		hint, err := json.Marshal(it.Hint)
		if err != nil {
			return err
		}
		// tenancyguard:ok platform content (GA table gamification.collection_items, no tenant_id, append-only); PlatformScope only
		if _, err := t.ExecContext(ctx, `INSERT INTO gamification.collection_items (collection_item_id, created_at, created_by, collection_version_id,
				collection_id, item_id, position, source_hint, objective_id) VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9)`,
			ids.New(), now, actor(ctx), versionID, coll, it.ItemID, it.Position, string(hint), it.ObjectiveID); err != nil {
			return fmt.Errorf("collection items: %w", err)
		}
	}
	return nil
}

// AchievementRepo implements write.AchievementStore on gamification.student_achievements and
// gamification.achievement_progress_events.
type AchievementRepo struct{}

var _ write.AchievementStore = AchievementRepo{}

// MarkEvent implements write.AchievementStore.
func (AchievementRepo) MarkEvent(ctx context.Context, tenant, student, achievement, event uuid.UUID, eventType string, at time.Time) (bool, error) {
	// tenancyguard:ok append-only insert under the TenantScope (RLS WITH CHECK); the unique key makes a redelivery a no-op
	ok, err := exec(ctx, `INSERT INTO gamification.achievement_progress_events (progress_event_id, tenant_id, created_at, created_by, student_id,
			achievement_id, event_id, event_type) VALUES ($1, $2, $3, $4, $5, $6, $7, $8) ON CONFLICT DO NOTHING`,
		ids.New(), tenant, at.UTC(), actor(ctx), student, achievement, event, eventType)
	if err != nil {
		return false, fmt.Errorf("achievements: mark event: %w", err)
	}
	return ok, nil
}

// State implements write.AchievementStore.
func (AchievementRepo) State(ctx context.Context, tenant, student, achievement uuid.UUID) (write.AchievementState, error) {
	t, err := tx(ctx)
	if err != nil {
		return write.AchievementState{}, err
	}
	var s write.AchievementState
	var tier sql.NullString
	var unlocked sql.NullTime
	// tenancyguard:ok tenant table gamification.student_achievements: explicit tenant_id + deleted_at IS NULL (RLS as well)
	err = t.QueryRowContext(ctx, `SELECT progress, progress_state, tier, unlocked_at, achievement_version FROM gamification.student_achievements
		WHERE tenant_id = $1 AND student_id = $2 AND achievement_id = $3 AND deleted_at IS NULL FOR UPDATE`, tenant, student, achievement).
		Scan(&s.Progress, &s.State, &tier, &unlocked, &s.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return write.AchievementState{}, nil
	}
	if err != nil {
		return s, fmt.Errorf("achievements: state: %w", err)
	}
	s.Exists = true
	if tier.Valid {
		s.Tier = &tier.String
	}
	if unlocked.Valid {
		u := unlocked.Time.UTC()
		s.UnlockedAt = &u
	}
	return s, nil
}

// SaveState implements write.AchievementStore: progress is never lowered (GREATEST, F075-AC-24).
func (AchievementRepo) SaveState(ctx context.Context, tenant, student, achievement uuid.UUID, s write.AchievementState, at time.Time) error {
	state := s.State
	if len(state) == 0 {
		state = []byte("{}")
	}
	// tenancyguard:ok upsert under the TenantScope (RLS insert/update policies); the conflict target is the live-row index
	_, err := exec(ctx, `INSERT INTO gamification.student_achievements AS a (student_achievement_id, tenant_id, created_at, created_by, updated_at,
			student_id, achievement_id, tier, progress, progress_state, achievement_version, unlocked_at)
		VALUES ($1, $2, $3, $4, $3, $5, $6, $7, $8, $9::jsonb, $10, $11)
		ON CONFLICT (tenant_id, student_id, achievement_id) WHERE deleted_at IS NULL DO UPDATE SET tier = EXCLUDED.tier,
			progress = GREATEST(a.progress, EXCLUDED.progress), progress_state = EXCLUDED.progress_state,
			achievement_version = EXCLUDED.achievement_version, unlocked_at = coalesce(a.unlocked_at, EXCLUDED.unlocked_at),
			updated_at = EXCLUDED.updated_at, updated_by = EXCLUDED.created_by, version = a.version + 1`,
		ids.New(), tenant, at.UTC(), actor(ctx), student, achievement, s.Tier, s.Progress, string(state), max(1, s.Version), s.UnlockedAt)
	if err != nil {
		return fmt.Errorf("achievements: save state: %w", err)
	}
	return nil
}

// AdjustmentRepo implements write.AdjustmentStore on gamification.economy_adjustments.
type AdjustmentRepo struct{}

var _ write.AdjustmentStore = AdjustmentRepo{}

const adjustmentColumns = `adjustment_id, tenant_id, student_id, kind, currency_code, item_id, delta, reason, ticket_ref, status, created_by,
	approved_by, rejected_by, reject_reason, ledger_entry_id, idempotency_key, created_at, applied_at, rejected_at`

// ScanAdjustment reads one row of adjustmentColumns.
func ScanAdjustment(row interface{ Scan(...any) error }) (write.AdjustmentRow, error) {
	var a write.AdjustmentRow
	var cur, item, ticket, rejectReason sql.NullString
	var approved, rejected, entry uuid.NullUUID
	var applied, rejectedAt sql.NullTime
	err := row.Scan(&a.ID, &a.TenantID, &a.StudentID, &a.Kind, &cur, &item, &a.Delta, &a.Reason, &ticket, &a.Status, &a.CreatedBy, &approved, &rejected,
		&rejectReason, &entry, &a.IdempotencyKey, &a.CreatedAt, &applied, &rejectedAt)
	if err != nil {
		return a, err
	}
	str := func(s sql.NullString) *string {
		if !s.Valid {
			return nil
		}
		v := s.String
		return &v
	}
	id := func(u uuid.NullUUID) *uuid.UUID {
		if !u.Valid {
			return nil
		}
		v := u.UUID
		return &v
	}
	tm := func(t sql.NullTime) *time.Time {
		if !t.Valid {
			return nil
		}
		v := t.Time.UTC()
		return &v
	}
	a.Currency, a.ItemID, a.TicketRef, a.RejectReason = str(cur), str(item), str(ticket), str(rejectReason)
	a.ApprovedBy, a.RejectedBy, a.LedgerEntryID = id(approved), id(rejected), id(entry)
	a.AppliedAt, a.RejectedAt = tm(applied), tm(rejectedAt)
	a.CreatedAt = a.CreatedAt.UTC()
	return a, nil
}

// Insert implements write.AdjustmentStore.
func (AdjustmentRepo) Insert(ctx context.Context, a write.AdjustmentRow) (bool, error) {
	// tenancyguard:ok insert under the TenantScope (RLS WITH CHECK); the idempotency key is unique among live rows
	ok, err := exec(ctx, `INSERT INTO gamification.economy_adjustments (adjustment_id, tenant_id, created_at, created_by, updated_at, student_id,
			kind, currency_code, item_id, delta, reason, ticket_ref, status, idempotency_key)
		VALUES ($1, $2, $3, $4, $3, $5, $6, $7, $8, $9, $10, $11, $12, $13) ON CONFLICT (idempotency_key) WHERE deleted_at IS NULL DO NOTHING`,
		a.ID, a.TenantID, a.CreatedAt.UTC(), a.CreatedBy, a.StudentID, a.Kind, a.Currency, a.ItemID, a.Delta, a.Reason, a.TicketRef, a.Status,
		a.IdempotencyKey)
	if err != nil {
		return false, fmt.Errorf("economy adjustments: insert: %w", err)
	}
	return ok, nil
}

// ByKey implements write.AdjustmentStore.
func (AdjustmentRepo) ByKey(ctx context.Context, tenant uuid.UUID, key string) (*write.AdjustmentRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant table gamification.economy_adjustments: explicit tenant_id + deleted_at IS NULL (RLS as well)
	a, err := ScanAdjustment(t.QueryRowContext(ctx, `SELECT `+adjustmentColumns+` FROM gamification.economy_adjustments
		WHERE tenant_id = $1 AND idempotency_key = $2 AND deleted_at IS NULL`, tenant, key))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("economy adjustments: by key: %w", err)
	}
	return &a, nil
}

// Get implements write.AdjustmentStore.
func (AdjustmentRepo) Get(ctx context.Context, tenant, id uuid.UUID) (*write.AdjustmentRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant table gamification.economy_adjustments: explicit tenant_id + deleted_at IS NULL (RLS as well)
	a, err := ScanAdjustment(t.QueryRowContext(ctx, `SELECT `+adjustmentColumns+` FROM gamification.economy_adjustments
		WHERE tenant_id = $1 AND adjustment_id = $2 AND deleted_at IS NULL FOR UPDATE`, tenant, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("economy adjustments: get: %w", err)
	}
	return &a, nil
}

// Decide implements write.AdjustmentStore.
func (AdjustmentRepo) Decide(ctx context.Context, a write.AdjustmentRow) error {
	// tenancyguard:ok tenant table gamification.economy_adjustments: explicit tenant_id + deleted_at IS NULL (RLS as well)
	ok, err := exec(ctx, `UPDATE gamification.economy_adjustments SET status = $3, approved_by = $4, rejected_by = $5, reject_reason = $6,
			ledger_entry_id = $7, applied_at = $8, rejected_at = $9, updated_at = now(), updated_by = $10, version = version + 1
		WHERE tenant_id = $1 AND adjustment_id = $2 AND status = 'pending_approval' AND deleted_at IS NULL`,
		a.TenantID, a.ID, a.Status, a.ApprovedBy, a.RejectedBy, a.RejectReason, a.LedgerEntryID, a.AppliedAt, a.RejectedAt, actor(ctx))
	if err != nil {
		return fmt.Errorf("economy adjustments: decide: %w", err)
	}
	if !ok {
		return errors.New("economy adjustments: decide: not pending")
	}
	return nil
}
