// Package consumer holds the event consumers of module learning (presentation of the worker profile events):
// they decode the payload against its schema and call one application handler; scope, idempotency
// (processed_events) and DLQ are handled by pkg/outbox.Consumer.
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

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// IdentityProjection is the consumer learning.projection_identity: it keeps the ref.* projections of module
// identity (DATABASE_DESIGN §2.17, F020 US1). Envelope tenant gives the scope (pkg/outbox); a payload that
// breaks its schema or references another tenant's row is refused at once (DLQ_LEARNING, F020-AC-04); a
// missing parent (tenant not projected yet) is retried with delay before DLQ.
type IdentityProjection struct {
	Upsert command.UpsertProjectionHandler
	Now    func() time.Time // lag metric clock; nil = time.Now
}

// IdentityProjectionOptions returns the outbox consumer options of learning.projection_identity (platform
// events such as province.updated are accepted, so Platform is true).
func IdentityProjectionOptions(h IdentityProjection, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "learning", Name: messaging.ProjectionIdentityName, Stream: messaging.ProjectionIdentityStream,
		Subjects: messaging.ProjectionIdentitySubjects(), Platform: true, UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
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

// version is source_version when present, otherwise the envelope time (payload schemas of identity).
func version(sv *int64, e outbox.Envelope) int64 {
	if sv != nil {
		return *sv
	}
	return e.OccurredAt.UTC().UnixMicro()
}

func decode(e outbox.Envelope, v any) error {
	d := json.NewDecoder(bytes.NewReader(e.Payload))
	if err := d.Decode(v); err != nil {
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

func (c IdentityProjection) dispatch(ctx context.Context, e outbox.Envelope) (string, error) {
	var applied bool
	var err error
	switch e.EventType {
	case "tenant.created", "tenant.updated":
		applied, err = c.tenant(ctx, e)
		return when(applied, "tenants"), err
	case "province.updated":
		applied, err = c.province(ctx, e)
		return when(applied, "provinces"), err
	case "school.synced", "school.updated":
		applied, err = c.school(ctx, e)
		return when(applied, "schools"), err
	case "class.synced":
		applied, err = c.class(ctx, e)
		return when(applied, "classes"), err
	case "student.synced":
		applied, err = c.student(ctx, e)
		return when(applied, "students"), err
	case "student.placement_changed":
		applied, err = c.placement(ctx, e)
		return when(applied, "students"), err
	case "student.cleaned_up", "student.restored":
		applied, err = c.studentDeleted(ctx, e)
		return when(applied, "students"), err
	case "enrollment.changed":
		applied, err = c.enrollment(ctx, e)
		return when(applied, "enrollments"), err
	}
	return "", nil // another identity event: nothing to project
}

func when(applied bool, table string) string {
	if applied {
		return table
	}
	return ""
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

type provincePayload struct {
	Code        string            `json:"code"`
	CountryCode string            `json:"country_code"`
	Name        map[string]string `json:"name"`
	Status      string            `json:"status"`
}

func (c IdentityProjection) province(ctx context.Context, e outbox.Envelope) (bool, error) {
	var p provincePayload
	if err := decode(e, &p); err != nil {
		return false, err
	}
	if e.TenantID != nil {
		return false, fmt.Errorf("%w: province.updated must be a platform event", command.ErrProjectionTenantMismatch)
	}
	name, _ := json.Marshal(p.Name)
	switch {
	case p.Code == "" || utf8.RuneCountInString(p.Code) > 64:
		return false, invalid(e.EventType, "code")
	case len(p.CountryCode) != 2:
		return false, invalid(e.EventType, "country_code")
	case p.Status != "active" && p.Status != "inactive":
		return false, invalid(e.EventType, "status")
	case p.Name["vi"] == "" || len(name) > 4096:
		return false, invalid(e.EventType, "name")
	}
	return c.Upsert.Province(ctx, write.RefProvince{Code: p.Code, CountryCode: p.CountryCode, NameI18n: name, Status: p.Status,
		SourceVersion: version(nil, e)})
}

type schoolPayload struct {
	SchoolID       uuid.UUID         `json:"school_id"`
	TenantID       uuid.UUID         `json:"tenant_id"` // school.updated only
	Name           *string           `json:"name"`
	NameI18n       map[string]string `json:"name_i18n"`
	ProvinceCode   *string           `json:"province_code"`
	SchoolLevel    *string           `json:"school_level"`
	IsVirtual      *bool             `json:"is_virtual"`
	Status         *string           `json:"status"`
	SettingsSubset map[string]any    `json:"settings_subset"`
	SourceVersion  *int64            `json:"source_version"`
}

func (c IdentityProjection) school(ctx context.Context, e outbox.Envelope) (bool, error) {
	var p schoolPayload
	if err := decode(e, &p); err != nil {
		return false, err
	}
	settings, ok := jsonObject(p.SettingsSubset, 16384)
	var nameI18n []byte
	if p.NameI18n != nil {
		nameI18n, _ = json.Marshal(p.NameI18n)
	}
	switch {
	case p.SchoolID == uuid.Nil:
		return false, invalid(e.EventType, "school_id is required")
	case !maxLen(p.Name, 255) || !maxLen(p.ProvinceCode, 64) || !maxLen(p.SchoolLevel, 64) || !maxLen(p.Status, 64) ||
		!ok || len(nameI18n) > 4096:
		return false, invalid(e.EventType, "length")
	}
	return c.Upsert.School(ctx, e.TenantID, write.RefSchool{SchoolID: p.SchoolID, TenantID: p.TenantID, Name: p.Name,
		NameI18n: nameI18n, ProvinceCode: p.ProvinceCode, SchoolLevel: p.SchoolLevel, IsVirtual: p.IsVirtual, Status: p.Status,
		SettingsSubset: settings, SourceVersion: version(p.SourceVersion, e)})
}

// classPayload is class.synced; source_version, academic_year_id and is_virtual are read when module identity sends
// them (OQ F020-1, F011 T040), the envelope time orders the events otherwise (F020 T044).
type classPayload struct {
	ClassID        uuid.UUID  `json:"class_id"`
	SchoolID       uuid.UUID  `json:"school_id"`
	GradeLevel     int        `json:"grade_level"`
	Name           string     `json:"name"`
	AcademicYearID *uuid.UUID `json:"academic_year_id"`
	IsVirtual      *bool      `json:"is_virtual"`
	SourceVersion  *int64     `json:"source_version"`
}

func (c IdentityProjection) class(ctx context.Context, e outbox.Envelope) (bool, error) {
	var p classPayload
	if err := decode(e, &p); err != nil {
		return false, err
	}
	switch {
	case p.ClassID == uuid.Nil || p.SchoolID == uuid.Nil:
		return false, invalid(e.EventType, "class_id and school_id are required")
	case p.GradeLevel < 1 || p.GradeLevel > 9:
		return false, invalid(e.EventType, "grade_level")
	case p.Name == "" || utf8.RuneCountInString(p.Name) > 128:
		return false, invalid(e.EventType, "name")
	}
	return c.Upsert.Class(ctx, e.TenantID, write.RefClass{ClassID: p.ClassID, SchoolID: p.SchoolID, GradeLevel: p.GradeLevel,
		Name: p.Name, AcademicYearID: p.AcademicYearID, IsVirtual: p.IsVirtual, SourceVersion: version(p.SourceVersion, e)})
}

type studentPayload struct {
	StudentID     uuid.UUID `json:"student_id"`
	SchoolID      uuid.UUID `json:"school_id"`
	Action        string    `json:"action"`
	GradeLevel    *int      `json:"grade_level"`
	Status        *string   `json:"status"`
	SourceVersion *int64    `json:"source_version"`
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
		Status: status, SourceVersion: version(p.SourceVersion, e)})
}

// placementPayload is student.placement_changed.v1 (ADR-0095 mục 2): the school, grade and effective status of a student.
type placementPayload struct {
	StudentID     uuid.UUID `json:"student_id"`
	TenantID      uuid.UUID `json:"tenant_id"`
	SchoolID      uuid.UUID `json:"school_id"`
	GradeLevel    int       `json:"grade_level"`
	Status        string    `json:"status"`
	SourceVersion *int64    `json:"source_version"`
}

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
	case e.TenantID == nil || *e.TenantID != p.TenantID:
		return false, fmt.Errorf("%w: the payload tenant is not the envelope tenant", command.ErrProjectionTenantMismatch)
	}
	grade, status := p.GradeLevel, p.Status
	return c.Upsert.Student(ctx, e.TenantID, write.RefStudent{StudentID: p.StudentID, SchoolID: p.SchoolID, GradeLevel: &grade,
		Status: &status, SourceVersion: version(p.SourceVersion, e)})
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

type enrollmentPayload struct {
	StudentID     uuid.UUID `json:"student_id"`
	ClassID       uuid.UUID `json:"class_id"`
	Op            string    `json:"op"`
	Status        *string   `json:"status"`
	StartDate     *string   `json:"start_date"`
	EndDate       *string   `json:"end_date"`
	SourceVersion *int64    `json:"source_version"`
}

var enrollmentStatus = map[string]string{"added": "active", "ended": "ended", "transferred": "transferred"}

func (c IdentityProjection) enrollment(ctx context.Context, e outbox.Envelope) (bool, error) {
	var p enrollmentPayload
	if err := decode(e, &p); err != nil {
		return false, err
	}
	if p.ClassID == uuid.Nil {
		// ADR-0095: an enrollment without a class (WP-I1 drops the class) carries nothing for ref.enrollments; the school and
		// grade come with student.placement_changed.
		return false, nil
	}
	status, ok := enrollmentStatus[p.Op]
	if p.Status != nil {
		status = *p.Status
	}
	if p.StudentID == uuid.Nil || p.ClassID == uuid.Nil || !ok || !slices.Contains([]string{"active", "transferred", "ended"}, status) {
		return false, invalid(e.EventType, "student_id, class_id, op and status are required")
	}
	if p.StartDate == nil {
		return false, invalid(e.EventType, "start_date is required by the projection key")
	}
	start, err := time.Parse(time.DateOnly, *p.StartDate)
	if err != nil {
		return false, invalid(e.EventType, "start_date")
	}
	r := write.RefEnrollment{StudentID: p.StudentID, ClassID: p.ClassID, StartDate: start, Status: status,
		SourceVersion: version(p.SourceVersion, e)}
	if p.EndDate != nil {
		end, err := time.Parse(time.DateOnly, *p.EndDate)
		if err != nil {
			return false, invalid(e.EventType, "end_date")
		}
		r.EndDate = &end
	}
	return c.Upsert.Enrollment(ctx, e.TenantID, r)
}
