package command

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// ActionProfileViewed is the audit action of a read of the learning profile of a student (LEARNING_ANALYTICS §9).
const ActionProfileViewed = "learning.profile_viewed"

// ProfileViews records who read the learning profile of a student (F031-AC-16). A read context never writes (ADR-0087):
// the HTTP handler of getStudentProgress calls this write use case after a successful read, in its own transaction;
// a failed audit fails the request (no unaudited profile read) [Proposed].
type ProfileViews struct {
	UoW   write.UnitOfWork
	Audit write.Auditor
}

// Record audits one read of the progress of studentID (school of the student, subject filter when given).
func (h ProfileViews) Record(ctx context.Context, studentID, schoolID uuid.UUID, subjectID *uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.RecordProfileView")
	defer func() { done(err) }()
	meta := map[string]any{"view": "progress"}
	if subjectID != nil {
		meta["subject_id"] = subjectID.String()
	}
	return h.UoW.Do(ctx, func(ctx context.Context) error {
		return h.Audit.Record(ctx, audit.Event{Action: ActionProfileViewed, TargetType: "student", TargetID: studentID.String(),
			Result: audit.ResultSuccess, SchoolID: schoolID.String(), Metadata: meta})
	})
}
