// Package consumer holds the event consumers of module game (presentation of the worker profile events): they
// decode the payload against its schema and call one application handler; scope, idempotency (processed_events)
// and DLQ are handled by pkg/outbox.Consumer.
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

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// IdentityProjection is the consumer game.projection_identity: it keeps ref.tenants, ref.schools and ref.students of
// eduworld_game (DATABASE_DESIGN §2.17, F014 T008). A payload that breaks its schema or references another tenant's
// row is refused at once (DLQ_GAME); a missing parent (tenant not projected yet) is retried with delay before DLQ.
type IdentityProjection struct {
	Upsert command.UpsertProjection
	Now    func() time.Time // lag metric clock; nil = time.Now
}

// IdentityProjectionOptions returns the outbox consumer options of game.projection_identity (user.status_changed may
// be a platform event, so Platform is true).
func IdentityProjectionOptions(h IdentityProjection, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "game", Name: messaging.ProjectionIdentityName, Stream: messaging.IdentityStream,
		Subjects: messaging.Subjects(messaging.ProjectionIdentityEvents), Platform: true, UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

var errPayload = errors.New("invalid payload")

func invalid(eventType, why string) error {
	return fmt.Errorf("%w: %s: %s", errPayload, eventType, why)
}

// Handle implements outbox.Handler.
func (c IdentityProjection) Handle(ctx context.Context, e outbox.Envelope) error {
	table, err := c.dispatch(ctx, e)
	switch {
	case err == nil:
		if table != "" {
			now := time.Now
			if c.Now != nil {
				now = c.Now
			}
			observability.ProjectionLag().WithLabelValues(table).Observe(now().Sub(e.OccurredAt).Seconds())
		}
		return nil
	case errors.Is(err, errPayload):
		observability.ConsumerRejected().WithLabelValues(messaging.ProjectionIdentityName, "invalid_payload").Inc()
		return outbox.Permanent(err)
	case errors.Is(err, command.ErrProjectionTenantMismatch), errors.Is(err, command.ErrProjectionNoTenant):
		observability.ConsumerRejected().WithLabelValues(messaging.ProjectionIdentityName, "tenant_mismatch").Inc()
		return outbox.Permanent(err)
	default:
		return err // transient (e.g. tenant not projected yet: FK) → NAK with delay, DLQ after MaxDeliver
	}
}

// version is source_version when present, otherwise the envelope time.
func version(sv *int64, e outbox.Envelope) int64 {
	if sv != nil {
		return *sv
	}
	return e.OccurredAt.UTC().UnixMicro()
}

func decode(e outbox.Envelope, v any) error {
	if err := json.NewDecoder(bytes.NewReader(e.Payload)).Decode(v); err != nil {
		return invalid(e.EventType, "not a JSON object of the schema")
	}
	return nil
}

func maxLen(s *string, n int) bool { return s == nil || utf8.RuneCountInString(*s) <= n }

func jsonObject(m map[string]any, maxBytes int) ([]byte, bool) {
	if m == nil {
		return nil, true
	}
	b, err := json.Marshal(m)
	return b, err == nil && len(b) <= maxBytes
}

func when(applied bool, table string) string {
	if applied {
		return table
	}
	return ""
}

func (c IdentityProjection) dispatch(ctx context.Context, e outbox.Envelope) (string, error) {
	var applied bool
	var err error
	switch e.EventType {
	case "tenant.created", "tenant.updated":
		applied, err = c.tenant(ctx, e)
		return when(applied, "tenants"), err
	case "school.synced", "school.updated":
		applied, err = c.school(ctx, e)
		return when(applied, "schools"), err
	case "student.synced":
		applied, err = c.student(ctx, e)
		return when(applied, "students"), err
	case "student.placement_changed":
		applied, err = c.placement(ctx, e)
		return when(applied, "students"), err
	case "user.status_changed":
		applied, err = c.status(ctx, e)
		return when(applied, "students"), err
	case "student.cleaned_up", "student.restored":
		applied, err = c.studentDeleted(ctx, e)
		return when(applied, "students"), err
	case "student.sis_link_created", "student.sis_link_superseded":
		applied, err = c.studentSIS(ctx, e)
		return when(applied, "students"), err
	}
	return "", nil // another identity event: nothing to project
}

type tenantPayload struct {
	TenantID       uuid.UUID      `json:"tenant_id"`
	Code           *string        `json:"code"`
	TenantType     string         `json:"tenant_type"`
	Status         string         `json:"status"`
	IsQA           *bool          `json:"is_qa"`
	SettingsSubset map[string]any `json:"settings_subset"`
	SourceVersion  *int64         `json:"source_version"`
}

func (c IdentityProjection) tenant(ctx context.Context, e outbox.Envelope) (bool, error) {
	var p tenantPayload
	if err := decode(e, &p); err != nil {
		return false, err
	}
	settings, ok := jsonObject(p.SettingsSubset, 16384)
	switch {
	case p.TenantID == uuid.Nil:
		return false, invalid(e.EventType, "tenant_id is required")
	case !slices.Contains([]string{"school", "unit", "common"}, p.TenantType):
		return false, invalid(e.EventType, "tenant_type")
	case p.Status != "active" && p.Status != "disabled":
		return false, invalid(e.EventType, "status")
	case !maxLen(p.Code, 64) || !ok:
		return false, invalid(e.EventType, "length")
	}
	return c.Upsert.Tenant(ctx, e.TenantID, write.RefTenant{TenantID: p.TenantID, Code: p.Code, TenantType: p.TenantType,
		Status: p.Status, IsQA: p.IsQA, SettingsSubset: settings, SourceVersion: version(p.SourceVersion, e)})
}

type schoolPayload struct {
	SchoolID       uuid.UUID      `json:"school_id"`
	TenantID       uuid.UUID      `json:"tenant_id"` // school.updated only
	Status         *string        `json:"status"`
	SettingsSubset map[string]any `json:"settings_subset"`
	SourceVersion  *int64         `json:"source_version"`
}

func (c IdentityProjection) school(ctx context.Context, e outbox.Envelope) (bool, error) {
	var p schoolPayload
	if err := decode(e, &p); err != nil {
		return false, err
	}
	settings, ok := jsonObject(p.SettingsSubset, 16384)
	switch {
	case p.SchoolID == uuid.Nil:
		return false, invalid(e.EventType, "school_id is required")
	case !maxLen(p.Status, 64) || !ok:
		return false, invalid(e.EventType, "length")
	}
	return c.Upsert.School(ctx, e.TenantID, write.RefSchool{SchoolID: p.SchoolID, TenantID: p.TenantID, Status: p.Status,
		SettingsSubset: settings, SourceVersion: version(p.SourceVersion, e)})
}

type studentPayload struct {
	StudentID     uuid.UUID  `json:"student_id"`
	SchoolID      uuid.UUID  `json:"school_id"`
	Action        string     `json:"action"`
	GradeLevel    *int       `json:"grade_level"`
	Status        *string    `json:"status"`
	SISConnection *uuid.UUID `json:"sis_connection_id"`
	SourceVersion *int64     `json:"source_version"`
}

func (c IdentityProjection) student(ctx context.Context, e outbox.Envelope) (bool, error) {
	var p studentPayload
	if err := decode(e, &p); err != nil {
		return false, err
	}
	switch {
	case p.StudentID == uuid.Nil || p.SchoolID == uuid.Nil:
		return false, invalid(e.EventType, "student_id and school_id are required")
	case !slices.Contains([]string{"create", "update", "deactivate"}, p.Action):
		return false, invalid(e.EventType, "action")
	case p.GradeLevel != nil && (*p.GradeLevel < 1 || *p.GradeLevel > 9):
		return false, invalid(e.EventType, "grade_level")
	case !maxLen(p.Status, 64):
		return false, invalid(e.EventType, "status")
	}
	status := p.Status
	if status == nil && p.Action == "deactivate" {
		s := "deactivated"
		status = &s
	}
	return c.Upsert.Student(ctx, e.TenantID, write.RefStudent{StudentID: p.StudentID, SchoolID: p.SchoolID, GradeLevel: p.GradeLevel,
		Status: status, SISConnectionID: p.SISConnection, SourceVersion: version(p.SourceVersion, e)})
}

// placementPayload is student.placement_changed.v1 (ADR-0095 mục 2): the fields the projection uses (tolerant of the others).
type placementPayload struct {
	StudentID     uuid.UUID `json:"student_id"`
	TenantID      uuid.UUID `json:"tenant_id"`
	SchoolID      uuid.UUID `json:"school_id"`
	GradeLevel    int       `json:"grade_level"`
	Status        string    `json:"status"`
	SourceVersion *int64    `json:"source_version"`
}

// placement applies student.placement_changed to ref.students (school, grade level, active/inactive), so a student
// created or moved outside the SIS sync reaches the game projection too (the grade transition is game.grade_transition).
func (c IdentityProjection) placement(ctx context.Context, e outbox.Envelope) (bool, error) {
	var p placementPayload
	if err := decode(e, &p); err != nil {
		return false, err
	}
	switch {
	case p.StudentID == uuid.Nil || p.SchoolID == uuid.Nil || p.TenantID == uuid.Nil:
		return false, invalid(e.EventType, "student_id, tenant_id and school_id are required")
	case p.GradeLevel < 1 || p.GradeLevel > 9:
		return false, invalid(e.EventType, "grade_level")
	case p.Status != "active" && p.Status != "inactive":
		return false, invalid(e.EventType, "status")
	}
	return c.Upsert.Placement(ctx, e.TenantID, write.RefPlacement{StudentID: p.StudentID, TenantID: p.TenantID, SchoolID: p.SchoolID,
		GradeLevel: p.GradeLevel, Active: p.Status == "active", SourceVersion: version(p.SourceVersion, e)})
}

type statusPayload struct {
	UserID uuid.UUID `json:"user_id"`
	To     string    `json:"to"`
}

var userStatuses = []string{"pending", "active", "locked", "deactivated", "anonymized"}

// status applies user.status_changed to the projected student with that id (staff users have no row: no effect).
func (c IdentityProjection) status(ctx context.Context, e outbox.Envelope) (bool, error) {
	var p statusPayload
	if err := decode(e, &p); err != nil {
		return false, err
	}
	if p.UserID == uuid.Nil || !slices.Contains(userStatuses, p.To) {
		return false, invalid(e.EventType, "user_id and to are required")
	}
	return c.Upsert.StudentStatus(ctx, e.TenantID, p.UserID, p.To)
}

type studentLifecyclePayload struct {
	StudentID uuid.UUID `json:"student_id"`
	TenantID  uuid.UUID `json:"tenant_id"`
}

func (c IdentityProjection) studentDeleted(ctx context.Context, e outbox.Envelope) (bool, error) {
	var p studentLifecyclePayload
	if err := decode(e, &p); err != nil {
		return false, err
	}
	if p.StudentID == uuid.Nil || p.TenantID == uuid.Nil {
		return false, invalid(e.EventType, "student_id and tenant_id are required")
	}
	return c.Upsert.StudentDeleted(ctx, e.TenantID, p.StudentID, p.TenantID, e.EventType == "student.cleaned_up", e.OccurredAt)
}

type sisLinkPayload struct {
	UserID     uuid.UUID `json:"user_id"`
	TenantID   uuid.UUID `json:"tenant_id"`         // student.sis_link_created
	Connection uuid.UUID `json:"sis_connection_id"` // student.sis_link_created
	New        uuid.UUID `json:"new_sis_connection_id"`
}

// studentSIS applies the SIS link of a student (OQ F014-1): the new connection of student.sis_link_superseded or the
// connection of student.sis_link_created.
func (c IdentityProjection) studentSIS(ctx context.Context, e outbox.Envelope) (bool, error) {
	var p sisLinkPayload
	if err := decode(e, &p); err != nil {
		return false, err
	}
	conn := p.Connection
	if e.EventType == "student.sis_link_superseded" {
		conn = p.New
	}
	switch {
	case p.UserID == uuid.Nil || conn == uuid.Nil:
		return false, invalid(e.EventType, "user_id and the SIS connection are required")
	case e.EventType == "student.sis_link_created" && (e.TenantID == nil || p.TenantID != *e.TenantID):
		return false, fmt.Errorf("%w: payload tenant", command.ErrProjectionTenantMismatch)
	}
	return c.Upsert.StudentSIS(ctx, e.TenantID, p.UserID, conn)
}
