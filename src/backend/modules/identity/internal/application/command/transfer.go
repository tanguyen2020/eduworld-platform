package command

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/enrollment"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// TransferStudentInput moves a student to a class (same school, other school, other tenant).
type TransferStudentInput struct {
	StudentID     uuid.UUID
	TargetClassID uuid.UUID
	On            time.Time
	Source        string     // enrollment.SourceSIS / SourceManual / SourcePlatformCommon
	SyncRunID     *uuid.UUID // SIS run that caused the transfer (F011), if any
}

// TransferStudentHandler is the transfer port used by F011 (SIS apply) and platform tools. The caller
// authorizes and provides a scope covering the source and target tenants (TenantScope for moves inside
// one tenant; CrossTenantScope or a platform-job bypass across tenants, ADR-0089 §3).
type TransferStudentHandler interface {
	Handle(ctx context.Context, in TransferStudentInput) (enrollment.Transfer, error)
}

// TransferDeps are the ports of the transfer.
type TransferDeps struct {
	UoW         write.UnitOfWork
	Enrollments write.EnrollmentRepository
	Students    write.StudentRepository
	Events      write.EventPublisher
	Audit       write.Auditor
}

// TransferStudent implements TransferStudentHandler (F010-AC-11, AC-13, AC-14, AC-15).
type TransferStudent struct{ TransferDeps }

// EnrollmentPayload is the payload of enrollment.changed.
type EnrollmentPayload struct {
	StudentID     string     `json:"student_id"`
	ClassID       string     `json:"class_id"`
	Op            string     `json:"op"`
	EnrollmentID  string     `json:"enrollment_id"`
	SchoolID      string     `json:"school_id"`
	Status        string     `json:"status"`
	StartDate     string     `json:"start_date"`
	EndDate       *string    `json:"end_date"`
	Source        string     `json:"source"`
	SyncRunID     *uuid.UUID `json:"sync_run_id"`
	SourceVersion int64      `json:"source_version"`
}

// MembershipPayload is the payload of tenant_membership.changed.
type MembershipPayload struct {
	UserID   string `json:"user_id"`
	TenantID string `json:"tenant_id"`
	Op       string `json:"op"`
	Source   string `json:"source"`
	UserType string `json:"user_type"`
}

func enrollmentPayload(e *enrollment.Enrollment, op string, syncRun *uuid.UUID) EnrollmentPayload {
	p := EnrollmentPayload{StudentID: e.StudentID.String(), ClassID: e.ClassID.String(), Op: op, EnrollmentID: e.ID.String(),
		SchoolID: e.SchoolID.String(), Status: string(e.Status), StartDate: e.StartDate.Format(time.DateOnly), Source: e.Source,
		SyncRunID: syncRun, SourceVersion: e.Version}
	if e.EndDate != nil {
		d := e.EndDate.Format(time.DateOnly)
		p.EndDate = &d
	}
	return p
}

// Handle runs the transfer in one transaction.
func (h TransferStudent) Handle(ctx context.Context, in TransferStudentInput) (out enrollment.Transfer, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.TransferStudent")
	defer func() { done(err) }()
	if _, ok := tenancy.ScopeFrom(ctx); !ok {
		return out, problem.New(problem.Internal, "transfer needs a scope from the caller")
	}
	if in.StudentID == uuid.Nil || in.TargetClassID == uuid.Nil {
		return out, problem.Invalid("class_id", problem.SubRequired, "student and target class are required")
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		cur, err := h.Enrollments.LockActive(ctx, in.StudentID)
		if err != nil {
			return err
		}
		target, err := h.Enrollments.Placement(ctx, in.TargetClassID)
		var pe *problem.Error
		if errors.As(err, &pe) && pe.Code == problem.NotFound {
			return problem.Invalid("class_id", problem.SubInvalid, "unknown target class")
		}
		if err != nil {
			return err
		}
		tr, err := enrollment.TransferTo(cur, ids.New(), target, in.On, in.Source)
		switch {
		case errors.Is(err, enrollment.ErrSameClass):
			return problem.Invalid("class_id", problem.SubInvalid, "the student is already in this class")
		case errors.Is(err, enrollment.ErrBeforeStart):
			return problem.Invalid("on", problem.SubInvalid, "transfer date before the current enrollment")
		case err != nil:
			return err
		}
		if err := h.Enrollments.Close(ctx, tr.Closed); err != nil {
			return err
		}
		if tr.Kind != enrollment.KindClass {
			if err := h.Students.Move(ctx, in.StudentID, target); err != nil {
				return err
			}
		}
		if err := h.Enrollments.Open(ctx, tr.Opened); err != nil {
			return err
		}
		out = tr
		return h.publish(ctx, tr, in)
	})
	return out, err
}

func (h TransferStudent) publish(ctx context.Context, tr enrollment.Transfer, in TransferStudentInput) error {
	student := in.StudentID.String()
	from, to := tr.Closed.TenantID, tr.Opened.TenantID
	if err := h.Events.Publish(ctx, write.Event{Type: "enrollment.changed", SubjectType: "enrollment", SubjectID: tr.Closed.ID.String(),
		TenantID: &from, Payload: enrollmentPayload(tr.Closed, "transferred", in.SyncRunID)}); err != nil {
		return err
	}
	if err := h.Events.Publish(ctx, write.Event{Type: "enrollment.changed", SubjectType: "enrollment", SubjectID: tr.Opened.ID.String(),
		TenantID: &to, Payload: enrollmentPayload(tr.Opened, "added", in.SyncRunID)}); err != nil {
		return err
	}
	if tr.Kind != enrollment.KindClass {
		if err := h.Events.Publish(ctx, write.Event{Type: "student.transferred", SubjectType: "student", SubjectID: student, TenantID: &to,
			Payload: map[string]string{"student_id": student, "from_tenant_id": from.String(), "to_tenant_id": to.String(),
				"from_school_id": tr.Closed.SchoolID.String(), "to_school_id": tr.Opened.SchoolID.String()}}); err != nil {
			return err
		}
	}
	if tr.Kind == enrollment.KindTenant { // removed then added (F010-AC-15)
		for _, m := range []struct {
			tenant uuid.UUID
			op     string
		}{{from, "removed"}, {to, "added"}} {
			t := m.tenant
			if err := h.Events.Publish(ctx, write.Event{Type: "tenant_membership.changed", SubjectType: "tenant_membership", SubjectID: student,
				TenantID: &t, Payload: MembershipPayload{UserID: student, TenantID: t.String(), Op: m.op, Source: "transfer", UserType: "student"}}); err != nil {
				return err
			}
		}
	}
	return h.Audit.Record(ctx, audit.Event{Action: "enrollment.changed", TargetType: "student", TargetID: student, Result: audit.ResultSuccess,
		Metadata: map[string]any{"kind": string(tr.Kind), "from_class_id": tr.Closed.ClassID.String(), "to_class_id": tr.Opened.ClassID.String(),
			"source": in.Source}})
}
