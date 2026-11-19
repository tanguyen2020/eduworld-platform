// Package read implements the read models of module game inside entx.ReadSession (READ ONLY, replica allowed):
// explicit SQL with tenant conditions (AGENTS R27); never device_id, ip_hash or IP in a result (F014-AC-11).
package read

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

func tx(ctx context.Context) (*sql.Tx, error) {
	t, ok := entx.Tx(ctx)
	if !ok {
		return nil, entx.ErrNoTransaction
	}
	return t, nil
}

// Manifests implements read.ManifestReadModel.
type Manifests struct{}

var _ read.ManifestReadModel = Manifests{}

// Latest implements read.ManifestReadModel.
func (Manifests) Latest(ctx context.Context, tenant uuid.UUID) (*read.Manifest, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	m := read.Manifest{TenantID: tenant}
	var entries []byte
	// tenancyguard:ok game.content_manifests has no deleted_at (versions are immutable); TenantScope RLS + explicit tenant_id
	err = t.QueryRowContext(ctx, `SELECT manifest_version, etag, entries, generated_at FROM game.content_manifests
		WHERE tenant_id = $1 ORDER BY manifest_version DESC LIMIT 1`, tenant).Scan(&m.Version, &m.ETag, &entries, &m.GeneratedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("content manifest: %w", err)
	}
	m.Entries, m.GeneratedAt = entries, m.GeneratedAt.UTC()
	return &m, nil
}

// LoginEvents implements read.LoginEventReadModel.
type LoginEvents struct{}

var _ read.LoginEventReadModel = LoginEvents{}

// List implements read.LoginEventReadModel.
func (LoginEvents) List(ctx context.Context, f read.LoginFilter) ([]read.LoginEvent, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	if len(f.TenantIDs) == 0 {
		return nil, nil
	}
	tenants := make([]string, len(f.TenantIDs))
	for i, id := range f.TenantIDs {
		tenants[i] = id.String()
	}
	// Explicit tenant_id list = the TenantScope tenant or the CrossTenantScope ids of System Admin; device_id and ip_hash
	// are never selected.
	// tenancyguard:ok append-only TA table (no deleted_at), tenant_id = ANY(tenants of the scope)
	rows, err := t.QueryContext(ctx, `SELECT login_event_id, tenant_id, student_id, game_session_id, platform, device_model,
			client_version, login_at, logout_at, end_reason
		FROM game.login_events
		WHERE tenant_id = ANY($1::uuid[]) AND ($2::uuid IS NULL OR student_id = $2)
			AND ($3::timestamptz IS NULL OR login_at >= $3) AND ($4::timestamptz IS NULL OR login_at < $4)
			AND ($5::timestamptz IS NULL OR (login_at, login_event_id) < ($5, $6))
		ORDER BY login_at DESC, login_event_id DESC LIMIT $7`,
		tenants, f.StudentID, f.From, f.To, f.AfterAt, f.AfterID, f.Limit+1)
	if err != nil {
		return nil, fmt.Errorf("login events: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.LoginEvent
	for rows.Next() {
		var e read.LoginEvent
		var model, version, reason sql.NullString
		var logout sql.NullTime
		if err := rows.Scan(&e.LoginEventID, &e.TenantID, &e.StudentID, &e.GameSessionID, &e.Platform, &model, &version,
			&e.LoginAt, &logout, &reason); err != nil {
			return nil, err
		}
		e.LoginAt = e.LoginAt.UTC()
		if model.Valid {
			e.DeviceModel = &model.String
		}
		if version.Valid {
			e.ClientVersion = &version.String
		}
		if logout.Valid {
			at := logout.Time.UTC()
			e.LogoutAt = &at
		}
		if reason.Valid {
			e.EndReason = &reason.String
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Students implements read.StudentReadModel.
type Students struct{}

var _ read.StudentReadModel = Students{}

// Student implements read.StudentReadModel.
func (Students) Student(ctx context.Context, tenant, student uuid.UUID) (*read.StudentRef, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	s := read.StudentRef{StudentID: student}
	// tenancyguard:ok projection ref.students (RLS + explicit tenant_id); a cleaned-up student keeps its login history
	err = t.QueryRowContext(ctx, `SELECT tenant_id, school_id FROM ref.students WHERE student_id = $1 AND tenant_id = $2`,
		student, tenant).Scan(&s.TenantID, &s.SchoolID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ref students: %w", err)
	}
	return &s, nil
}
