package command

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/efficacy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// SnapshotDeleteReason is delete_reason of the mastery snapshots of a withdrawn participant (F035-AC-19).
const SnapshotDeleteReason = "efficacy_withdrawn"

// WithdrawInput is EfficacyWithdrawalCreate.
type WithdrawInput struct {
	Scope            string
	SchoolID         *uuid.UUID
	ClassID          *uuid.UUID
	StudentIDs       []uuid.UUID
	Reason           string
	SchoolRequestRef *string
	IdempotencyKey   string
}

func (in WithdrawInput) validate() error {
	if !efficacy.ValidReason(in.Reason) {
		return problem.Invalid("/reason", problem.SubInvalid, "must have 10 to 1000 characters")
	}
	if in.SchoolRequestRef != nil && len([]rune(*in.SchoolRequestRef)) > efficacy.MaxRefLen {
		return problem.Invalid("/school_request_ref", problem.SubInvalid, "must have at most 128 characters")
	}
	switch in.Scope {
	case efficacy.ScopeTenant:
	case efficacy.ScopeSchool:
		if in.SchoolID == nil {
			return problem.Invalid("/school_id", problem.SubRequired, "is required for the scope school")
		}
	case efficacy.ScopeClass:
		if in.ClassID == nil {
			return problem.Invalid("/class_id", problem.SubRequired, "is required for the scope class")
		}
	case efficacy.ScopeStudents:
		if n := len(in.StudentIDs); n < 1 || n > efficacy.MaxWithdrawStudents {
			return problem.Invalid("/student_ids", problem.SubRequired, "lists 1 to 500 students for the scope students")
		}
	default:
		return problem.Invalid("/scope", problem.SubInvalid, "must be tenant, school, class or students")
	}
	return nil
}

// WithdrawalView is the view of a withdrawal row.
func WithdrawalView(w write.WithdrawalRow) dto.EfficacyWithdrawalView {
	return dto.EfficacyWithdrawalView{WithdrawalID: w.ID, TenantID: w.TenantID, StudyID: w.StudyID, Scope: w.Scope, AffectedCount: w.Affected,
		CancelledExamSessionCount: w.CancelledSessions, CreatedAt: w.CreatedAt}
}

// WithdrawEfficacyParticipants withdraws participants of a study at the request of the school (F035-AC-18): the active
// participants of the scope (tenant of X-Tenant-Id) become withdrawn, leave the audience of the efficacy sessions not
// opened yet, the sessions not opened of a class withdrawn as a whole are cancelled, their mastery snapshots are
// soft-deleted (F035-AC-19); idempotent by Idempotency-Key (the same key answers the first withdrawal); audit
// efficacy.participants_withdrawn.
func (h Efficacy) WithdrawEfficacyParticipants(ctx context.Context, studyID uuid.UUID, in WithdrawInput) (out dto.EfficacyWithdrawalView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.WithdrawEfficacyParticipants")
	defer func() { done(err) }()
	p, err := efficacyAdmin(ctx, authz.PermPlatformEfficacyStudyManage)
	if err != nil {
		return out, err
	}
	tenant, err := targetTenant(p)
	if err != nil {
		return out, err
	}
	if err := in.validate(); err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if in.IdempotencyKey != "" {
			if prev, err := h.Store.WithdrawalByKey(ctx, p.UserID, in.IdempotencyKey); err != nil || prev != nil {
				if err == nil {
					out = WithdrawalView(*prev)
				}
				return err
			}
		}
		s, err := h.Store.Study(ctx, studyID, true)
		if err != nil {
			return err
		}
		if s == nil {
			return problem.New(problem.NotFound, "")
		}
		if s.Status != efficacy.StudyDraft && s.Status != efficacy.StudyActive {
			return problem.New(problem.InvalidStateTransition, "participants are withdrawn from a running study only")
		}
		scope := write.WithdrawScope{TenantID: tenant, StudyID: &studyID, SchoolID: in.SchoolID, ClassID: in.ClassID}
		if in.Scope == efficacy.ScopeStudents {
			scope.StudentIDs = in.StudentIDs
		}
		now := h.now()
		w := write.WithdrawalRow{ID: ids.New(), TenantID: tenant, StudyID: &studyID, Scope: in.Scope, SchoolID: in.SchoolID, ClassID: in.ClassID,
			StudentIDs: scope.StudentIDs, Reason: in.Reason, SchoolRequestRef: in.SchoolRequestRef, RequestedBy: p.UserID,
			IdempotencyKey: truncateRunes(in.IdempotencyKey, 64), CreatedAt: now}
		if w.IdempotencyKey == "" {
			w.IdempotencyKey = w.ID.String()
		}
		affected, cancelled, err := h.withdrawWith(ctx, p, scope, in.Scope, w.ID, now)
		if err != nil {
			return err
		}
		w.Affected, w.CancelledSessions = affected, cancelled
		if err := h.Store.InsertWithdrawal(ctx, w); err != nil {
			return err
		}
		if err := h.auditTenant(ctx, tenant, ActionParticipantsWithdraw, "efficacy_study", studyID, in.Reason, map[string]any{
			"scope": in.Scope, "affected_count": affected, "cancelled_exam_session_count": cancelled, "withdrawal_id": w.ID,
			"school_request_ref": in.SchoolRequestRef}); err != nil {
			return err
		}
		out = WithdrawalView(w)
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// withdraw is withdrawWith for a whole tenant (agreement revoked): every running study of the tenant.
func (h Efficacy) withdraw(ctx context.Context, p authz.Principal, scope write.WithdrawScope, at time.Time) (int, int, error) {
	return h.withdrawWith(ctx, p, scope, efficacy.ScopeTenant, uuid.New(), at)
}

// withdrawWith applies a withdrawal in the transaction of ctx (TenantScope of the tenant of the scope): participants,
// classes withdrawn as a whole (scopes tenant, school, class), the audience and the sessions not opened yet of the
// efficacy waves, the mastery snapshots.
func (h Efficacy) withdrawWith(ctx context.Context, p authz.Principal, scope write.WithdrawScope, kind string, withdrawalID uuid.UUID,
	at time.Time) (int, int, error) {
	parts, err := h.Store.Withdraw(ctx, scope, at, withdrawalID)
	if err != nil {
		return 0, 0, err
	}
	var classes []write.StudyClassRow
	if kind != efficacy.ScopeStudents {
		if classes, err = h.Store.MarkClassesWithdrawn(ctx, scope, at, withdrawalID); err != nil {
			return 0, 0, err
		}
	}
	students := map[uuid.UUID][]uuid.UUID{} // study → students
	for _, x := range parts {
		students[x.StudyID] = append(students[x.StudyID], x.StudentID)
	}
	if kind == efficacy.ScopeStudents {
		// Before the pre wave the students are no participants yet: they leave the audience all the same.
		for _, st := range scope.StudentIDs {
			if scope.StudyID != nil && !containsID(students[*scope.StudyID], st) {
				students[*scope.StudyID] = append(students[*scope.StudyID], st)
			}
		}
	}
	whole := map[[2]uuid.UUID]bool{}
	studies := map[uuid.UUID]bool{}
	for _, c := range classes {
		whole[[2]uuid.UUID{c.StudyID, c.ClassID}] = true
		studies[c.StudyID] = true
	}
	for st := range students {
		studies[st] = true
	}
	cancelled := 0
	for study := range studies {
		ws, err := h.Store.Waves(ctx, study)
		if err != nil {
			return 0, 0, err
		}
		for _, w := range ws {
			if w.Status != efficacy.WaveScheduled {
				continue
			}
			sessions, err := h.Store.WaveSessions(ctx, w.ID)
			if err != nil {
				return 0, 0, err
			}
			for _, s := range sessions {
				if s.TenantID != scope.TenantID || s.Status != "active" {
					continue
				}
				if whole[[2]uuid.UUID{study, s.ClassID}] {
					if h.Exams != nil {
						if err := h.Exams.CancelExamSession(ctx, s.ExamSessionID, "efficacy_withdrawn"); err != nil {
							return 0, 0, err
						}
					}
					if err := h.Store.CancelWaveSession(ctx, s); err != nil {
						return 0, 0, err
					}
					cancelled++
					continue
				}
				if len(students[study]) > 0 && h.Exclusions != nil {
					if _, err := h.Exclusions.ExcludeParticipations(ctx, s.ExamSessionID, students[study], p.UserID); err != nil {
						return 0, 0, err
					}
				}
			}
		}
		if ids := students[study]; len(ids) > 0 {
			st := study
			if _, err := h.Store.SoftDeleteSnapshots(ctx, scope.TenantID, &st, ids, SnapshotDeleteReason, &p.UserID); err != nil {
				return 0, 0, err
			}
		}
	}
	if h.Metrics != nil {
		h.Metrics.Participants(efficacy.ParticipantWithdrawn, len(parts))
	}
	return len(parts), cancelled, nil
}

// ExcludeAnonymized excludes an anonymized student from every study of the tenant (F035-AC-19, step of
// anonymizeUserData, F016) and soft-deletes the derived study data (mastery snapshots) in the transaction of ctx; the
// learning record is kept.
func (h Efficacy) ExcludeAnonymized(ctx context.Context, tenant, studentID uuid.UUID, by *uuid.UUID) (int, error) {
	n, err := h.Store.ExcludeStudent(ctx, tenant, studentID, efficacy.ReasonAnonymized, h.now())
	if err != nil {
		return 0, err
	}
	if _, err := h.Store.SoftDeleteSnapshots(ctx, tenant, nil, []uuid.UUID{studentID}, efficacy.ReasonAnonymized, by); err != nil {
		return 0, err
	}
	return n, nil
}

// AnonymizeStudent is the efficacy step of anonymizeUserData (F035-AC-19, F016): in the TenantScope of the saga, the
// student leaves every study of the tenant (excluded, reason anonymized) and the derived study data (mastery snapshots)
// is soft-deleted; the learning record is kept, nothing is deleted physically. Returns the participants excluded.
func (h Efficacy) AnonymizeStudent(ctx context.Context, studentID uuid.UUID) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.AnonymizeEfficacyStudent")
	defer func() { done(err) }()
	sc, ok := tenancy.ScopeFrom(ctx)
	if !ok {
		return 0, problem.New(problem.Internal, "anonymizeUserData without tenant scope")
	}
	tenant, single := sc.SessionTenantID()
	if !single {
		return 0, problem.New(problem.Internal, "anonymizeUserData without a single tenant")
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		n, err = h.ExcludeAnonymized(ctx, tenant, studentID, nil)
		return err
	})
	if err == nil && n > 0 && h.Metrics != nil {
		h.Metrics.Participants(efficacy.ParticipantExcluded, n)
	}
	return n, err
}
