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
	"regexp"
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
	case "school.synced", "school.updated":
		ok, err := c.school(ctx, e)
		return classify("schools", e, ok, err)
	case "platform_settings.changed":
		ok, err := c.platformSettings(ctx, e)
		return classify("platform_settings", e, ok, err)
	}
	return nil
}

func (c IdentityProjection) school(ctx context.Context, e outbox.Envelope) (bool, error) {
	var p struct {
		SchoolID       uuid.UUID      `json:"school_id"`
		TenantID       uuid.UUID      `json:"tenant_id"`
		Status         *string        `json:"status"`
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
	tenant, err := envTenant(e)
	switch {
	case err != nil:
		return false, err
	case p.SchoolID == uuid.Nil || (p.TenantID != uuid.Nil && p.TenantID != tenant):
		return false, invalid(e.EventType, "school_id/tenant_id")
	case !maxLen(p.Status, 64) || len(settings) > 16384:
		return false, invalid(e.EventType, "length")
	}
	return c.P.School(ctx, tenant, p.SchoolID, p.Status, settings, version(p.SourceVersion, e))
}

// platformSettings keeps platform_curfew (key platform_curfew, scope platform), chat_retention (scope platform) and the
// tenant override of gameplay_chat_retention_days (key tenant_settings, scope tenant, F117); other keys are not used.
func (c IdentityProjection) platformSettings(ctx context.Context, e outbox.Envelope) (bool, error) {
	var p struct {
		Key          string          `json:"key"`
		Scope        string          `json:"scope"`
		TenantID     *uuid.UUID      `json:"tenant_id"`
		After        json.RawMessage `json:"after"`
		CurfewWindow *struct {
			Start string `json:"start"`
			End   string `json:"end"`
		} `json:"curfew_window"`
	}
	if err := decode(e, &p); err != nil {
		return false, err
	}
	switch {
	case p.Key == "chat_retention" && p.Scope == "platform":
		return c.chatRetention(ctx, e, p.After)
	case p.Key == "tenant_settings" && p.Scope == "tenant":
		return c.tenantRetention(ctx, e, p.TenantID, p.After)
	}
	if p.Key != "platform_curfew" || p.Scope != "platform" {
		return false, nil
	}
	w := p.CurfewWindow
	switch {
	case e.TenantID != nil:
		return false, invalid(e.EventType, "platform_curfew is a platform setting")
	case w == nil || !curfewBound(w.Start) || !curfewBound(w.End) || w.Start == w.End:
		return false, invalid(e.EventType, "curfew_window {start, end} HH:MM is required")
	}
	value, _ := json.Marshal(map[string]string{"start": w.Start, "end": w.End})
	return c.P.PlatformCurfew(ctx, value, e.OccurredAt.UTC().UnixMicro())
}

// chatRetention keeps chat_retention of the platform (ref.platform_settings): purge_after of the gameplay channels and the
// ephemeral retention of the purge (F117-AC-15, AC-21).
func (c IdentityProjection) chatRetention(ctx context.Context, e outbox.Envelope, after json.RawMessage) (bool, error) {
	var v struct {
		Days  *int `json:"gameplay_chat_retention_days"`
		Hours *int `json:"ephemeral_retention_hours"`
	}
	switch {
	case e.TenantID != nil:
		return false, invalid(e.EventType, "chat_retention is a platform setting")
	case len(after) == 0 || len(after) > 16384 || json.Unmarshal(after, &v) != nil || v.Days == nil || *v.Days < 1 || *v.Days > 365 ||
		(v.Hours != nil && (*v.Hours < 1 || *v.Hours > 168)):
		return false, invalid(e.EventType, "chat_retention {gameplay_chat_retention_days 1..365, ephemeral_retention_hours 1..168}")
	}
	return c.P.PlatformChatRetention(ctx, after, e.OccurredAt.UTC().UnixMicro())
}

// tenantRetention keeps the tenant override of gameplay_chat_retention_days (ref.tenants; absent or null = inherit).
func (c IdentityProjection) tenantRetention(ctx context.Context, e outbox.Envelope, payloadTenant *uuid.UUID, after json.RawMessage) (bool, error) {
	tenant, err := envTenant(e)
	if err != nil {
		return false, err
	}
	if payloadTenant != nil && *payloadTenant != tenant {
		return false, invalid(e.EventType, "tenant_id")
	}
	var v struct {
		Days *int `json:"gameplay_chat_retention_days"`
	}
	if len(after) > 0 && string(after) != "null" {
		if err := json.Unmarshal(after, &v); err != nil {
			return false, invalid(e.EventType, "after")
		}
	}
	if v.Days != nil && (*v.Days < 1 || *v.Days > 365) {
		return false, invalid(e.EventType, "gameplay_chat_retention_days 1..365")
	}
	return c.P.TenantChatRetention(ctx, tenant, v.Days, e.OccurredAt.UTC().UnixMicro())
}

var curfewRe = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

func curfewBound(s string) bool { return curfewRe.MatchString(s) }

func (c IdentityProjection) tenant(ctx context.Context, e outbox.Envelope) (bool, error) {
	var p struct {
		TenantID       uuid.UUID      `json:"tenant_id"`
		Code           *string        `json:"code"`
		Name           *string        `json:"name"`
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
	case !maxLen(p.Code, 64) || !maxLen(p.Name, 255) || len(settings) > 16384:
		return false, invalid(e.EventType, "length")
	}
	return c.P.Tenant(ctx, p.TenantID, p.Code, p.Name, p.TenantType, p.Status, p.IsQA, settings, version(p.SourceVersion, e))
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
// level of the PlayerCard, F095-AC-11) and level.reached (level, title of the PlayerCard — delta F095); public data
// only, R16.
type GameProjection struct{ P command.Projection }

// GameProjectionOptions returns its options.
func GameProjectionOptions(h GameProjection, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "chat", Name: messaging.ProjectionGameName, Stream: messaging.GameStream,
		Subjects: messaging.Subjects("game", messaging.ProjectionGameEvents), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// Handle implements outbox.Handler.
func (c GameProjection) Handle(ctx context.Context, e outbox.Envelope) error {
	if e.EventType == "level.reached" {
		return c.levelReached(ctx, e)
	}
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

// levelReached applies level.reached (owner F074): the level and the level title (LocalizedName {vi, en}) of the public
// card (delta F095). A title that is not a bounded {vi, en} pair is a payload error (DLQ).
func (c GameProjection) levelReached(ctx context.Context, e outbox.Envelope) error {
	var p struct {
		StudentID uuid.UUID `json:"student_id"`
		ToLevel   int       `json:"to_level"`
		Title     *struct {
			Vi string `json:"vi"`
			En string `json:"en"`
		} `json:"title"`
	}
	if err := decode(e, &p); err != nil {
		return outbox.Permanent(err)
	}
	tenant, err := envTenant(e)
	switch {
	case err != nil:
		return outbox.Permanent(err)
	case p.StudentID == uuid.Nil || p.ToLevel < 2 || p.ToLevel > 200:
		return outbox.Permanent(invalid(e.EventType, "student_id/to_level"))
	}
	var title []byte
	if p.Title != nil {
		if p.Title.Vi == "" || p.Title.En == "" || !maxLen(&p.Title.Vi, 255) || !maxLen(&p.Title.En, 255) {
			return outbox.Permanent(invalid(e.EventType, "title"))
		}
		title, _ = json.Marshal(map[string]string{"vi": p.Title.Vi, "en": p.Title.En})
	}
	ok, err := c.P.PlayerLevel(ctx, tenant, p.StudentID, p.ToLevel, title)
	return classify("player_profiles", e, ok, err)
}
