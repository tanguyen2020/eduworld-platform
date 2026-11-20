package consumer

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/natsx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// CourseProvisioner provisions the courses of a class (command.LMS).
type CourseProvisioner interface {
	EnsureCoursesForClass(ctx context.Context, c command.ClassSynced) error
}

// LMSCourses is the consumer learning.lms_courses (F040-AC-01, AC-02): class.synced of a tenant creates one course per
// subject of the grade of the class, once (processed_events and the unique course of class × subject × year); a closed
// class archives its courses.
type LMSCourses struct {
	Courses CourseProvisioner
}

// LMSCoursesOptions returns the outbox consumer options (tenant events of module identity).
func LMSCoursesOptions(h LMSCourses, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "learning", Name: messaging.LMSCoursesName, Stream: natsx.EventStream("identity"),
		Subjects: messaging.LMSCoursesSubjects(), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// Handle implements outbox.Handler.
func (c LMSCourses) Handle(ctx context.Context, e outbox.Envelope) error {
	var p struct {
		ClassID        uuid.UUID  `json:"class_id"`
		SchoolID       uuid.UUID  `json:"school_id"`
		GradeLevel     int        `json:"grade_level"`
		AcademicYear   string     `json:"academic_year"`
		AcademicYearID *uuid.UUID `json:"academic_year_id"`
		Status         string     `json:"status"`
	}
	if err := json.Unmarshal(e.Payload, &p); err != nil || p.ClassID == uuid.Nil || p.SchoolID == uuid.Nil || e.TenantID == nil ||
		p.GradeLevel < 1 || p.GradeLevel > 12 || (p.AcademicYear == "" && p.AcademicYearID == nil) {
		return invalid(e.EventType, "payload is not a class")
	}
	return c.Courses.EnsureCoursesForClass(ctx, command.ClassSynced{TenantID: *e.TenantID, ClassID: p.ClassID, SchoolID: p.SchoolID,
		Grade: p.GradeLevel, AcademicYear: p.AcademicYear, AcademicYearID: p.AcademicYearID, Closed: p.Status == "closed"})
}

// ScheduleWriter adds a schedule item of an event (command.LMS).
type ScheduleWriter interface {
	UpsertScheduleFromEvent(ctx context.Context, e command.ScheduleFromEvent) error
}

// LMSSchedule is the consumer learning.lms_schedule (F040-AC-15): assignment.published (payload assignment.created.v1,
// owner F041) adds an assignment_due item on its due date, exam.opened (owner F042) an exam item on its opening date,
// for the course of the event, once per event; events without course add nothing.
type LMSSchedule struct {
	Schedule ScheduleWriter
}

// LMSScheduleOptions returns the outbox consumer options (tenant events of module learning).
func LMSScheduleOptions(h LMSSchedule, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "learning", Name: messaging.LMSScheduleName, Stream: messaging.AttemptStream,
		Subjects: messaging.LMSScheduleSubjects(), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// Handle implements outbox.Handler.
func (c LMSSchedule) Handle(ctx context.Context, e outbox.Envelope) error {
	var p struct {
		AssignmentID  uuid.UUID  `json:"assignment_id"`
		ExamSessionID uuid.UUID  `json:"exam_session_id"`
		CourseID      *uuid.UUID `json:"course_id"`
		DueAt         *time.Time `json:"due_at"`
		OpenAt        *time.Time `json:"open_at"`
	}
	if err := json.Unmarshal(e.Payload, &p); err != nil || e.TenantID == nil {
		return invalid(e.EventType, "payload is not an assignment or an exam")
	}
	if p.CourseID == nil {
		return nil
	}
	ev := command.ScheduleFromEvent{EventID: e.EventID, TenantID: *e.TenantID, CourseID: *p.CourseID}
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	switch {
	case e.EventType == "assignment.published" && p.AssignmentID != uuid.Nil && p.DueAt != nil:
		ev.Kind, ev.RefType, ev.RefID, ev.Date = "assignment_due", "assignment", p.AssignmentID, p.DueAt.In(loc).Format(time.DateOnly)
	case e.EventType == "exam.opened" && p.ExamSessionID != uuid.Nil && p.OpenAt != nil:
		ev.Kind, ev.RefType, ev.RefID, ev.Date = "exam", "exam_session", p.ExamSessionID, p.OpenAt.In(loc).Format(time.DateOnly)
	default:
		return invalid(e.EventType, "payload has no reference or date")
	}
	return c.Schedule.UpsertScheduleFromEvent(ctx, ev)
}
