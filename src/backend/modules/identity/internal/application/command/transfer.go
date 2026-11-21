package command

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/placement"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// TransferStudentInput moves a student to a school and grade (same school other grade, other school, other tenant —
// ADR-0095 mục 2: there is no class any more).
type TransferStudentInput struct {
	StudentID      uuid.UUID
	TargetSchoolID uuid.UUID
	TargetGrade    int
	On             time.Time  // date of effect (student.placement_changed.effective_from)
	Source         string     // placement.SourceSIS / SourceManual / SourcePlatformCommon
	SyncRunID      *uuid.UUID // SIS run that caused the transfer (F011), if any
}

// TransferStudentResult is the kind of the move and the placements before and after.
type TransferStudentResult struct {
	Kind   placement.Kind
	Before placement.Placement
	After  placement.Placement
}

// TransferStudentHandler is the transfer port used by F011 (SIS apply) and platform tools. The caller
// authorizes and provides a scope covering the source and target tenants (TenantScope for moves inside
// one tenant; CrossTenantScope or a platform-job bypass across tenants, ADR-0089 §3).
type TransferStudentHandler interface {
	Handle(ctx context.Context, in TransferStudentInput) (TransferStudentResult, error)
}

// TransferDeps are the ports of the transfer.
type TransferDeps struct {
	UoW      write.UnitOfWork
	Schools  write.SchoolRepository
	Students write.StudentRepository
	Events   write.EventPublisher
	Audit    write.Auditor
}

// TransferStudent implements TransferStudentHandler (F010-AC-11, AC-13, AC-14, AC-15).
type TransferStudent struct{ TransferDeps }

// MembershipPayload is the payload of tenant_membership.changed.
type MembershipPayload struct {
	UserID   string `json:"user_id"`
	TenantID string `json:"tenant_id"`
	Op       string `json:"op"`
	Source   string `json:"source"`
	UserType string `json:"user_type"`
}

// Handle runs the transfer in one transaction: the student row is locked, so concurrent transfers of one student
// serialize (F010-AC-11).
func (h TransferStudent) Handle(ctx context.Context, in TransferStudentInput) (out TransferStudentResult, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.TransferStudent")
	defer func() { done(err) }()
	if _, ok := tenancy.ScopeFrom(ctx); !ok {
		return out, problem.New(problem.Internal, "transfer needs a scope from the caller")
	}
	if in.StudentID == uuid.Nil || in.TargetSchoolID == uuid.Nil {
		return out, problem.Invalid("school_id", problem.SubRequired, "student and target school are required")
	}
	if !placement.ValidGrade(in.TargetGrade) {
		return out, problem.Invalid("grade_level", problem.SubInvalid, "grade_level must be 1…9")
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		tr, err := trackPlacement(ctx, h.Students, h.Events, in.StudentID)
		if err != nil {
			return err
		}
		cur := tr.Before()
		if cur == nil {
			return problem.New(problem.NotFound, "student not found in scope")
		}
		s, err := h.Schools.Get(ctx, in.TargetSchoolID)
		var pe *problem.Error
		if errors.As(err, &pe) && pe.Code == problem.NotFound {
			return problem.Invalid("school_id", problem.SubInvalid, "unknown target school")
		}
		if err != nil {
			return err
		}
		target := placement.Placement{TenantID: s.TenantID, SchoolID: s.ID, Grade: in.TargetGrade, Active: true}
		kind, err := placement.Move(cur.Placement, target)
		switch {
		case errors.Is(err, placement.ErrUnchanged):
			return problem.Invalid("school_id", problem.SubInvalid, "the student is already in this school and grade")
		case errors.Is(err, placement.ErrInvalid):
			return problem.Invalid("student_id", problem.SubInvalid, "the student has no effective school/grade to move from")
		case err != nil:
			return err
		}
		if err := h.Students.Move(ctx, in.StudentID, target); err != nil {
			return err
		}
		on := in.On
		if err := tr.done(ctx, PlacementChange{Source: in.Source, SyncRunID: in.SyncRunID, EffectiveFrom: &on}); err != nil {
			return err
		}
		out = TransferStudentResult{Kind: kind, Before: cur.Placement, After: target}
		return h.publish(ctx, out, in)
	})
	return out, err
}

func (h TransferStudent) publish(ctx context.Context, tr TransferStudentResult, in TransferStudentInput) error {
	student := in.StudentID.String()
	from, to := tr.Before.TenantID, tr.After.TenantID
	if tr.Kind != placement.KindGrade {
		if err := h.Events.Publish(ctx, write.Event{Type: "student.transferred", SubjectType: "student", SubjectID: student, TenantID: &to,
			Payload: map[string]string{"student_id": student, "from_tenant_id": from.String(), "to_tenant_id": to.String(),
				"from_school_id": tr.Before.SchoolID.String(), "to_school_id": tr.After.SchoolID.String()}}); err != nil {
			return err
		}
	}
	if tr.Kind == placement.KindTenant { // removed then added (F010-AC-15)
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
	return h.Audit.Record(ctx, audit.Event{Action: "student.placement_changed", TargetType: "student", TargetID: student, Result: audit.ResultSuccess,
		Metadata: map[string]any{"kind": string(tr.Kind), "from_school_id": tr.Before.SchoolID.String(), "to_school_id": tr.After.SchoolID.String(),
			"from_grade_level": tr.Before.Grade, "to_grade_level": tr.After.Grade, "source": in.Source}})
}
