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
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// LoginEventRepo implements write.LoginEventRepository on game.login_events (append-only; the end is written once,
// enforced by the trigger game.login_events_close_once).
type LoginEventRepo struct{}

var _ write.LoginEventRepository = LoginEventRepo{}

// Create implements write.LoginEventRepository.
func (LoginEventRepo) Create(ctx context.Context, e write.LoginEvent) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	var actor any
	if p, ok := authz.PrincipalFrom(ctx); ok {
		actor = p.UserID
	}
	var version any
	if e.ClientVersion != "" {
		version = e.ClientVersion
	}
	if _, err := t.ExecContext(ctx, `INSERT INTO game.login_events (login_event_id, tenant_id, created_at, created_by, student_id,
			game_session_id, device_id, platform, device_model, client_version, ip_hash, login_at, sis_connection_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $3, $12)`,
		e.ID, e.TenantID, e.LoginAt, actor, e.StudentID, e.GameSessionID, e.DeviceID, e.Platform, e.DeviceModel, version,
		e.IPHash, e.SISConnectionID); err != nil {
		return fmt.Errorf("login events: create: %w", err)
	}
	return nil
}

// Close implements write.LoginEventRepository.
func (LoginEventRepo) Close(ctx context.Context, tenant, session uuid.UUID, at time.Time, reason string) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok append-only TA table (no deleted_at): the open login event of one session of the TenantScope
	if _, err := t.ExecContext(ctx, `UPDATE game.login_events SET logout_at = $3, end_reason = $4
		WHERE tenant_id = $1 AND game_session_id = $2 AND logout_at IS NULL`, tenant, session, at.UTC(), reason); err != nil {
		return fmt.Errorf("login events: close: %w", err)
	}
	return nil
}

// Directory implements write.Directory on the identity projections.
type Directory struct{}

var _ write.Directory = Directory{}

// Student implements write.Directory.
func (Directory) Student(ctx context.Context, tenant, student uuid.UUID) (*write.Student, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	s := write.Student{StudentID: student}
	var deleted sql.NullTime
	// tenancyguard:ok projection ref.students (RLS + explicit tenant_id); deleted_at is read as the projected source state
	err = t.QueryRowContext(ctx, `SELECT tenant_id, school_id, status, deleted_at FROM ref.students
		WHERE student_id = $1 AND tenant_id = $2`, student, tenant).Scan(&s.TenantID, &s.SchoolID, &s.Status, &deleted)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ref students: %w", err)
	}
	s.Deleted = deleted.Valid
	return &s, nil
}

// SchoolCurfew implements write.Directory (settings_subset.curfew_window of school.updated, ADR-0078).
func (Directory) SchoolCurfew(ctx context.Context, tenant, school uuid.UUID) (*write.CurfewWindow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var raw []byte
	// tenancyguard:ok projection ref.schools (RLS + explicit tenant_id); projections have no deleted_at
	err = t.QueryRowContext(ctx, `SELECT settings_subset->'curfew_window' FROM ref.schools
		WHERE school_id = $1 AND tenant_id = $2`, school, tenant).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) || len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ref schools: %w", err)
	}
	var w write.CurfewWindow
	if err := json.Unmarshal(raw, &w); err != nil || w.Start == "" || w.End == "" {
		return nil, nil //nolint:nilerr // a malformed projected window is ignored: the platform curfew still applies
	}
	return &w, nil
}

// Publisher implements write.EventPublisher with the outbox writer.
type Publisher struct{ W *outbox.Writer }

var _ write.EventPublisher = Publisher{}

// Publish writes e into the outbox of the transaction.
func (p Publisher) Publish(ctx context.Context, e write.Event) error {
	_, err := p.W.Publish(ctx, outbox.Message{Type: e.Type, Subject: outbox.Subject{Type: e.SubjectType, ID: e.SubjectID},
		PartitionKey: e.PartitionKey, TenantID: e.TenantID, OccurredAt: e.OccurredAt, Payload: e.Payload})
	return err
}

// Auditor implements write.Auditor with the outbox audit recorder.
type Auditor struct{ R audit.Recorder }

var _ write.Auditor = Auditor{}

// Record records e (in the transaction of ctx, or in its own when there is none).
func (a Auditor) Record(ctx context.Context, e audit.Event) error { return a.R.Record(ctx, e) }
