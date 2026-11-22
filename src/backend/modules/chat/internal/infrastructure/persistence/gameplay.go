package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
)

// Gameplay implements write.GameplayRepository (F117 US1, US3): ref.parties, the retention of ref.tenants /
// ref.platform_settings, the closing of a gameplay channel and the read cursor of a member. Explicit SQL with the
// tenant condition (RLS + tenant_id) and deleted_at on business tables (R27).
type Gameplay struct{}

var _ write.GameplayRepository = Gameplay{}

// Party implements write.GameplayRepository.
func (Gameplay) Party(ctx context.Context, tenant, party uuid.UUID) (*write.PartyState, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var raw []byte
	st := write.PartyState{PartyID: party, TenantID: tenant}
	var status string
	// tenancyguard:ok projection ref.parties of the session tenant (no deleted_at: rule 2 of the projections)
	err = t.QueryRowContext(ctx, `SELECT p.status, p.member_versions, p.source_version FROM ref.parties p
		WHERE p.tenant_id = $1 AND p.party_id = $2`, tenant, party).Scan(&status, &raw, &st.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("chat: party: %w", err)
	}
	st.Disbanded = status == "disbanded"
	st.Members = map[uuid.UUID]write.PartyMember{}
	if err := json.Unmarshal(raw, &st.Members); err != nil {
		return nil, fmt.Errorf("chat: party members: %w", err)
	}
	return &st, nil
}

// SaveParty implements write.GameplayRepository.
func (Gameplay) SaveParty(ctx context.Context, p write.PartyState) error {
	members, err := json.Marshal(p.Members)
	if err != nil {
		return err
	}
	status := "open"
	if p.Disbanded {
		status = "disbanded"
	}
	// tenancyguard:ok projection ref.parties of the session tenant (RLS WITH CHECK)
	if _, err := exec(ctx, `INSERT INTO ref.parties AS rp (party_id, tenant_id, status, member_versions, source_version)
		VALUES ($1, $2, $3, $4::jsonb, $5)
		ON CONFLICT (party_id) DO UPDATE SET status = EXCLUDED.status, member_versions = EXCLUDED.member_versions,
			source_version = GREATEST(rp.source_version, EXCLUDED.source_version), updated_at = now()
		WHERE rp.tenant_id = EXCLUDED.tenant_id`, p.PartyID, p.TenantID, status, string(members), p.Version); err != nil {
		return fmt.Errorf("chat: save party: %w", err)
	}
	return nil
}

// Retention implements write.GameplayRepository.
func (Gameplay) Retention(ctx context.Context, tenant uuid.UUID) (write.RetentionSettings, error) {
	t, err := tx(ctx)
	if err != nil {
		return write.RetentionSettings{}, err
	}
	var s write.RetentionSettings
	var days sql.NullInt64
	var platform []byte
	// tenancyguard:ok ref.tenants is the tenant registry (no deleted_at); ref.platform_settings is a G projection
	err = t.QueryRowContext(ctx, `SELECT (SELECT t.gameplay_chat_retention_days FROM ref.tenants t WHERE t.tenant_id = $1),
		(SELECT ps.value FROM ref.platform_settings ps WHERE ps.key = 'chat_retention')`, tenant).Scan(&days, &platform)
	if err != nil {
		return s, fmt.Errorf("chat: retention: %w", err)
	}
	if days.Valid {
		d := int(days.Int64)
		s.Tenant = &d
	}
	s.Platform = platform
	return s, nil
}

// TenantRetention implements write.GameplayRepository.
func (Gameplay) TenantRetention(ctx context.Context, tenant uuid.UUID, days *int, version int64) (bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return false, err
	}
	var exists bool
	// tenancyguard:ok ref.tenants is the tenant registry (no deleted_at)
	if err := t.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM ref.tenants t WHERE t.tenant_id = $1)`, tenant).Scan(&exists); err != nil {
		return false, fmt.Errorf("chat: tenant retention: %w", err)
	}
	if !exists {
		return false, nil
	}
	// tenancyguard:ok ref.tenants is the tenant registry (no deleted_at); only a newer version applies
	if _, err := t.ExecContext(ctx, `UPDATE ref.tenants SET gameplay_chat_retention_days = $2, retention_version = $3
		WHERE tenant_id = $1 AND retention_version < $3`, tenant, days, version); err != nil {
		return false, fmt.Errorf("chat: tenant retention: %w", err)
	}
	return true, nil
}

// CloseChannel implements write.GameplayRepository.
func (Gameplay) CloseChannel(ctx context.Context, c channel.Channel) (bool, error) {
	// tenancyguard:ok chat.channels of the session tenant, live open gameplay rows only
	n, err := exec(ctx, `UPDATE chat.channels SET status = 'closed', closed_at = $3, purge_after = $4, updated_at = now(),
		version = version + 1
		WHERE tenant_id = $1 AND channel_id = $2 AND is_gameplay AND status = 'open' AND deleted_at IS NULL`,
		c.TenantID, c.ID, c.ClosedAt, c.PurgeAfter)
	if err != nil {
		return false, fmt.Errorf("chat: close channel: %w", err)
	}
	return n == 1, nil
}

// ActiveMembers implements write.GameplayRepository.
func (Gameplay) ActiveMembers(ctx context.Context, tenant, id uuid.UUID) ([]uuid.UUID, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok chat.channel_members of the session tenant, live active rows
	rows, err := t.QueryContext(ctx, `SELECT m.user_id FROM chat.channel_members m WHERE m.tenant_id = $1 AND m.channel_id = $2
		AND m.deleted_at IS NULL AND m.left_at IS NULL ORDER BY m.joined_at, m.user_id`, tenant, id)
	if err != nil {
		return nil, fmt.Errorf("chat: active members: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []uuid.UUID
	for rows.Next() {
		var u uuid.UUID
		if err := rows.Scan(&u); err != nil {
			return nil, fmt.Errorf("chat: active members: %w", err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// MarkRead implements write.GameplayRepository: last_read_seq only grows and never passes last_seq of the channel.
func (Gameplay) MarkRead(ctx context.Context, tenant, id, user uuid.UUID, seq, known int64, at time.Time) (int64, bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, false, err
	}
	var unread int64
	// tenancyguard:ok chat.channel_members joined with chat.channels, both of the session tenant, live rows only
	err = t.QueryRowContext(ctx, `UPDATE chat.channel_members m SET last_read_seq = GREATEST(m.last_read_seq, LEAST($4::bigint, GREATEST(c.last_seq, $6::bigint))),
			updated_at = $5
		FROM chat.channels c
		WHERE m.tenant_id = $1 AND m.channel_id = $2 AND m.user_id = $3 AND m.deleted_at IS NULL AND m.left_at IS NULL
			AND c.tenant_id = m.tenant_id AND c.channel_id = m.channel_id AND c.deleted_at IS NULL
		RETURNING GREATEST(GREATEST(c.last_seq, $6::bigint) - m.last_read_seq, 0)`, tenant, id, user, seq, at, known).Scan(&unread)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("chat: mark read: %w", err)
	}
	return unread, true, nil
}
