package command

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/placement"
)

// PlacementPayload is the payload of student.placement_changed.v1 (ADR-0095 mục 2, 7; owner F010).
type PlacementPayload struct {
	StudentID          string     `json:"student_id"`
	TenantID           string     `json:"tenant_id"`
	SchoolID           string     `json:"school_id"`
	GradeLevel         int        `json:"grade_level"`
	Status             string     `json:"status"`
	PreviousTenantID   *string    `json:"previous_tenant_id"`
	PreviousSchoolID   *string    `json:"previous_school_id"`
	PreviousGradeLevel *int       `json:"previous_grade_level"`
	PreviousStatus     *string    `json:"previous_status"`
	ChangedFields      []string   `json:"changed_fields"`
	AcademicYearID     *string    `json:"academic_year_id"`
	EffectiveFrom      *string    `json:"effective_from"`
	Source             string     `json:"source"`
	SyncRunID          *uuid.UUID `json:"sync_run_id"`
	SourceVersion      int64      `json:"source_version"`
}

// PlacementChange describes the origin of a placement change.
type PlacementChange struct {
	Source        string     // placement.Source…
	SyncRunID     *uuid.UUID // SIS run (source sis)
	EffectiveFrom *time.Time // date of effect given by the source; nil = occurred_at of the envelope
}

// placementTracker remembers the placement of a student before a change, inside the transaction of the change: done
// reads it again and publishes student.placement_changed when the tenant, school, grade or effective status differs.
type placementTracker struct {
	students  write.StudentRepository
	events    write.EventPublisher
	studentID uuid.UUID
	before    *placement.Record
}

// trackPlacement locks the student row and keeps its current placement (nil when the student has no row yet).
func trackPlacement(ctx context.Context, students write.StudentRepository, events write.EventPublisher, studentID uuid.UUID) (*placementTracker, error) {
	before, err := students.Placement(ctx, studentID)
	if err != nil {
		return nil, err
	}
	return &placementTracker{students: students, events: events, studentID: studentID, before: before}, nil
}

// Before is the placement read when the tracker was created (nil for a new student).
func (t *placementTracker) Before() *placement.Record { return t.before }

// done publishes the change, if any, in the current transaction; the student row must still be readable in the scope.
func (t *placementTracker) done(ctx context.Context, c PlacementChange) error {
	after, err := t.students.Placement(ctx, t.studentID)
	if err != nil || after == nil {
		return err
	}
	return publishPlacement(ctx, t.events, t.studentID, t.before, *after, c)
}

// publishPlacement publishes student.placement_changed from before to after (nothing when nothing changed).
func publishPlacement(ctx context.Context, events write.EventPublisher, studentID uuid.UUID, before *placement.Record, after placement.Record,
	c PlacementChange) error {
	var prev *placement.Placement
	if before != nil {
		prev = &before.Placement
	}
	changed := placement.Changed(prev, after.Placement)
	if len(changed) == 0 {
		return nil
	}
	p := PlacementPayload{StudentID: studentID.String(), TenantID: after.TenantID.String(), SchoolID: after.SchoolID.String(),
		GradeLevel: after.Grade, Status: after.Status(), ChangedFields: changed, Source: c.Source, SyncRunID: c.SyncRunID,
		SourceVersion: after.Version}
	if before != nil {
		t, s, g, st := before.TenantID.String(), before.SchoolID.String(), before.Grade, before.Status()
		p.PreviousTenantID, p.PreviousSchoolID, p.PreviousGradeLevel, p.PreviousStatus = &t, &s, &g, &st
	}
	if after.AcademicYearID != nil {
		y := after.AcademicYearID.String()
		p.AcademicYearID = &y
	}
	if c.EffectiveFrom != nil {
		d := c.EffectiveFrom.UTC().Format(time.DateOnly)
		p.EffectiveFrom = &d
	}
	tenant := after.TenantID
	return events.Publish(ctx, write.Event{Type: "student.placement_changed", SubjectType: "student", SubjectID: studentID.String(),
		TenantID: &tenant, Payload: p})
}
