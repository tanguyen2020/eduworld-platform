package command

import (
	"context"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// ActionStudentMergeLinked is the audit action of a merge step of module learning.
const ActionStudentMergeLinked = "student.merge_linked"

// Merge statuses (UserLifecycleResult.status).
const (
	MergeDone        = "done"
	MergeBlocked     = "blocked"
	MergeNothingToDo = "nothing_to_do"
)

// MergeUserDataInput is the learning step of the account merge saga of module identity.
type MergeUserDataInput struct {
	MergeID uuid.UUID
	From    uuid.UUID
	Into    uuid.UUID
	Reason  string
}

// MergeUserDataResult is UserLifecycleResult (also the replay body of the facade).
type MergeUserDataResult struct {
	Status        string `json:"status"`
	BlockedReason string `json:"blocked_reason,omitempty"`
}

// MergeUserData links the original student_id to the target user (OQ F016-2, F045-3, owner 2026-11-20; IDENTITY_MAPPING
// §7): attempts, learning records and mastery stay under the original student_id, a link original → target is written
// once per merge_id (idempotent), nothing_to_do when the student has no learning data in the tenant.
type MergeUserData struct {
	UoW   write.UnitOfWork
	Store write.MergeLinkStore
	Audit write.Auditor
	Clock func() time.Time
}

// Handle runs the step in the tenant scope of ctx (x-context-required: tenant_scope, idempotency_key).
func (h MergeUserData) Handle(ctx context.Context, in MergeUserDataInput) (out MergeUserDataResult, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.MergeUserData")
	defer func() { done(err) }()
	p, err := authorize(ctx, authz.PermPlatformUserMerge)
	if err != nil {
		return out, err
	}
	switch n := utf8.RuneCountInString(in.Reason); {
	case n < 3 || n > 1000:
		return out, problem.Invalid("reason", problem.SubInvalid, "must have 3 to 1000 characters")
	case in.MergeID == uuid.Nil || in.From == uuid.Nil || in.Into == uuid.Nil:
		return out, problem.Invalid("merge_id", problem.SubRequired, "merge_id, user_id and target_user_id are required")
	case in.From == in.Into:
		return MergeUserDataResult{Status: MergeBlocked, BlockedReason: "the source and target users are the same"}, nil
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		out, err = h.link(ctx, p, in)
		if err != nil {
			return err
		}
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

func (h MergeUserData) link(ctx context.Context, p authz.Principal, in MergeUserDataInput) (MergeUserDataResult, error) {
	if l, err := h.Store.MergeLinkByID(ctx, in.MergeID); err != nil || l != nil {
		if err != nil {
			return MergeUserDataResult{}, err
		}
		if l.SourceID != in.From || l.TargetID != in.Into {
			return MergeUserDataResult{Status: MergeBlocked, BlockedReason: "merge_id already links other users"}, nil
		}
		return MergeUserDataResult{Status: MergeDone}, nil
	}
	if l, err := h.Store.MergeLinkOfSource(ctx, in.From); err != nil || l != nil {
		if err != nil {
			return MergeUserDataResult{}, err
		}
		if l.TargetID != in.Into {
			return MergeUserDataResult{Status: MergeBlocked, BlockedReason: "the source user is already merged into another user"}, nil
		}
		return MergeUserDataResult{Status: MergeDone}, nil
	}
	has, err := h.Store.HasStudentData(ctx, in.From)
	if err != nil {
		return MergeUserDataResult{}, err
	}
	if !has {
		return MergeUserDataResult{Status: MergeNothingToDo}, nil
	}
	at := now(h.Clock).UTC()
	if err := h.Store.InsertMergeLink(ctx, write.MergeLink{MergeID: in.MergeID, SourceID: in.From, TargetID: in.Into, Reason: in.Reason,
		CreatedBy: p.UserID, CreatedAt: at}); err != nil {
		return MergeUserDataResult{}, err
	}
	if err := h.Audit.Record(ctx, audit.Event{Action: ActionStudentMergeLinked, TargetType: "student", TargetID: in.From.String(),
		Result: audit.ResultSuccess, Reason: in.Reason, Metadata: map[string]any{"merge_id": in.MergeID.String(),
			"target_user_id": in.Into.String()}}); err != nil {
		return MergeUserDataResult{}, err
	}
	return MergeUserDataResult{Status: MergeDone}, nil
}
