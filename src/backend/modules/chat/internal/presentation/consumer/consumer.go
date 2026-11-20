// Package consumer holds the event consumers of module chat (worker profile events): they decode the payload
// against its schema and call one application handler; scope, idempotency (processed_events) and DLQ are handled by
// pkg/outbox.Consumer.
package consumer

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

var errPayload = errors.New("invalid payload")

func invalid(eventType, why string) error {
	return fmt.Errorf("%w: %s: %s", errPayload, eventType, why)
}

func decode(e outbox.Envelope, v any) error {
	if err := json.NewDecoder(bytes.NewReader(e.Payload)).Decode(v); err != nil {
		return invalid(e.EventType, "not a JSON object of the schema")
	}
	return nil
}

func maxLen(s *string, n int) bool { return s == nil || utf8.RuneCountInString(*s) <= n }

func version(sv *int64, e outbox.Envelope) int64 {
	if sv != nil {
		return *sv
	}
	return e.OccurredAt.UTC().UnixMicro()
}

// classify turns handler errors into outbox decisions: schema/tenant errors go to the DLQ at once, others retry.
func classify(table string, e outbox.Envelope, applied bool, err error) error {
	switch {
	case err == nil:
		if applied && table != "" {
			observability.ProjectionLag(table, time.Since(e.OccurredAt))
		}
		return nil
	case errors.Is(err, errPayload), errors.Is(err, command.ErrProjectionTenantMismatch):
		return outbox.Permanent(err)
	}
	return err
}

func envTenant(e outbox.Envelope) (uuid.UUID, error) {
	if e.TenantID == nil || *e.TenantID == uuid.Nil {
		return uuid.Nil, invalid(e.EventType, "tenant event without tenant")
	}
	return *e.TenantID, nil
}

// IdentityProjection is the consumer chat.projection_identity (ref.tenants, ref.students).
type IdentityProjection struct{ P command.Projection }

// IdentityProjectionOptions returns its options (user.status_changed may be a platform event: Platform true).
func IdentityProjectionOptions(h IdentityProjection, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "chat", Name: messaging.ProjectionIdentityName, Stream: messaging.IdentityStream,
		Subjects: messaging.Subjects("identity", messaging.ProjectionIdentityEvents), Platform: true, UoW: uow, DLQDB: dlq, JS: js,
		Handler: h.Handle}
}

// Handle implements outbox.Handler.
func (c IdentityProjection) Handle(ctx context.Context, e outbox.Envelope) error {
	switch e.EventType {
	case "tenant.created", "tenant.updated":
		ok, err := c.tenant(ctx, e)
		return classify("tenants", e, ok, err)
	case "student.synced":
		ok, err := c.student(ctx, e)
		return classify("students", e, ok, err)
	case "user.status_changed":
		var p struct {
			UserID uuid.UUID `json:"user_id"`
			To     string    `json:"to"`
		}
		if err := decode(e, &p); err != nil {
			return outbox.Permanent(err)
		}
		if p.UserID == uuid.Nil || !slices.Contains([]string{"pending", "active", "locked", "deactivated", "anonymized"}, p.To) {
			return outbox.Permanent(invalid(e.EventType, "user_id and to are required"))
		}
		ok, err := c.P.StudentStatus(ctx, e.TenantID, p.UserID, p.To)
		return classify("students", e, ok, err)
	case "student.cleaned_up", "student.restored":
		var p struct {
			StudentID uuid.UUID `json:"student_id"`
			TenantID  uuid.UUID `json:"tenant_id"`
		}
		if err := decode(e, &p); err != nil {
			return outbox.Permanent(err)
		}
		tenant, err := envTenant(e)
		if err != nil || p.StudentID == uuid.Nil || p.TenantID != tenant {
			return outbox.Permanent(invalid(e.EventType, "student_id and tenant_id"))
		}
		ok, err := c.P.StudentDeleted(ctx, tenant, p.StudentID, e.EventType == "student.cleaned_up", e.OccurredAt)
		return classify("students", e, ok, err)
	}
	return nil
}

func (c IdentityProjection) tenant(ctx context.Context, e outbox.Envelope) (bool, error) {
	var p struct {
		TenantID       uuid.UUID      `json:"tenant_id"`
		Code           *string        `json:"code"`
		TenantType     string         `json:"tenant_type"`
		Status         string         `json:"status"`
		IsQA           *bool          `json:"is_qa"`
		SettingsSubset map[string]any `json:"settings_subset"`
		SourceVersion  *int64         `json:"source_version"`
	}
	if err := decode(e, &p); err != nil {
		return false, err
	}
	var settings []byte
	if p.SettingsSubset != nil {
		settings, _ = json.Marshal(p.SettingsSubset)
	}
	switch {
	case p.TenantID == uuid.Nil || e.TenantID == nil || *e.TenantID != p.TenantID:
		return false, invalid(e.EventType, "tenant_id")
	case !slices.Contains([]string{"school", "unit", "common"}, p.TenantType), p.Status != "active" && p.Status != "disabled":
		return false, invalid(e.EventType, "tenant_type/status")
	case !maxLen(p.Code, 64) || len(settings) > 16384:
		return false, invalid(e.EventType, "length")
	}
	return c.P.Tenant(ctx, p.TenantID, p.Code, p.TenantType, p.Status, p.IsQA, settings, version(p.SourceVersion, e))
}

func (c IdentityProjection) student(ctx context.Context, e outbox.Envelope) (bool, error) {
	var p struct {
		StudentID     uuid.UUID `json:"student_id"`
		SchoolID      uuid.UUID `json:"school_id"`
		Action        string    `json:"action"`
		GradeLevel    *int      `json:"grade_level"`
		Status        *string   `json:"status"`
		SourceVersion *int64    `json:"source_version"`
	}
	if err := decode(e, &p); err != nil {
		return false, err
	}
	tenant, err := envTenant(e)
	switch {
	case err != nil:
		return false, err
	case p.StudentID == uuid.Nil || p.SchoolID == uuid.Nil:
		return false, invalid(e.EventType, "student_id and school_id are required")
	case !slices.Contains([]string{"create", "update", "deactivate"}, p.Action):
		return false, invalid(e.EventType, "action")
	case p.GradeLevel != nil && (*p.GradeLevel < 1 || *p.GradeLevel > 9), !maxLen(p.Status, 64):
		return false, invalid(e.EventType, "grade_level/status")
	}
	status := p.Status
	if status == nil && p.Action == "deactivate" {
		s := "deactivated"
		status = &s
	}
	return c.P.Student(ctx, tenant, p.StudentID, p.SchoolID, p.GradeLevel, status, version(p.SourceVersion, e))
}

// GameProjection is the consumer chat.projection_game: ref.player_profiles from game_profile.updated (nickname, avatar,
// level of the PlayerCard, F095-AC-11; public data only, R16).
type GameProjection struct{ P command.Projection }

// GameProjectionOptions returns its options.
func GameProjectionOptions(h GameProjection, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "chat", Name: messaging.ProjectionGameName, Stream: messaging.GameStream,
		Subjects: messaging.Subjects("game", messaging.ProjectionGameEvents), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// Handle implements outbox.Handler.
func (c GameProjection) Handle(ctx context.Context, e outbox.Envelope) error {
	if e.EventType != "game_profile.updated" {
		return nil
	}
	var p struct {
		StudentID     uuid.UUID      `json:"student_id"`
		TenantID      uuid.UUID      `json:"tenant_id"`
		Nickname      *string        `json:"nickname"`
		AvatarConfig  map[string]any `json:"avatar_config"`
		Level         int            `json:"level"`
		SourceVersion *int64         `json:"source_version"`
	}
	if err := decode(e, &p); err != nil {
		return outbox.Permanent(err)
	}
	var avatar []byte
	if p.AvatarConfig != nil {
		avatar, _ = json.Marshal(p.AvatarConfig)
	}
	tenant, err := envTenant(e)
	switch {
	case err != nil:
		return outbox.Permanent(err)
	case p.StudentID == uuid.Nil || p.TenantID != tenant || p.Level < 1 || p.SourceVersion == nil:
		return outbox.Permanent(invalid(e.EventType, "student_id/tenant_id/level/source_version"))
	case !maxLen(p.Nickname, 32) || len(avatar) > 4096 || len(p.AvatarConfig) > 12:
		return outbox.Permanent(invalid(e.EventType, "length"))
	}
	ok, err := c.P.PlayerProfile(ctx, tenant, p.StudentID, p.Nickname, p.Level, avatar, *p.SourceVersion)
	return classify("player_profiles", e, ok, err)
}
