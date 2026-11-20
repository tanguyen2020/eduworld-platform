package command

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/attempt"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/i18ntext"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// admitAssignment runs the gate of a school assignment for a start (F041-AC-13) and checks the assignment gives this
// content; an attempt in progress is resumed (its view, done = true).
func (h Attempts) admitAssignment(ctx context.Context, p authz.Principal, contextType string, contextID *uuid.UUID, contentType string,
	contentID uuid.UUID) (AssignmentAdmission, *dto.AttemptView, error) {
	if h.Gate == nil {
		return AssignmentAdmission{}, nil, problem.Invalid("/mode", problem.SubInvalid, "school assignments are not available in this binary")
	}
	if contextType != "assignment" || contextID == nil {
		return AssignmentAdmission{}, nil, problem.Invalid("/context_id", problem.SubRequired, "an assignment attempt needs context_type assignment and its context_id")
	}
	adm, err := h.Gate.Admit(ctx, p.UserID, *contextID)
	if err != nil {
		return adm, nil, err
	}
	if adm.ContentType != contentType || adm.ContentID != contentID {
		return adm, nil, problem.Invalid("/context_id", problem.SubInvalid, "the assignment does not give this content")
	}
	if adm.OpenAttempt != nil {
		a, err := h.Store.Attempt(ctx, *adm.OpenAttempt)
		if err != nil {
			return adm, nil, err
		}
		v, err := h.resumeView(ctx, a)
		return adm, &v, err
	}
	return adm, nil, nil
}

// startAssignment starts (or resumes) the attempt of a school assignment on an activity (F041-AC-13, AC-14): the version
// pinned by the assignment (the current release with follow_latest), its feedback timing, context assignment.
func (h Attempts) startAssignment(ctx context.Context, p authz.Principal, in StartActivityInput) (out dto.AttemptView, err error) {
	ctxType := in.ContextType
	if ctxType == "" {
		ctxType = "assignment"
	}
	key := idemKey(ctx)
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if prev, err := h.Store.AttemptByKey(ctx, p.UserID, key); err != nil || prev != nil {
			if err == nil {
				out, err = h.resumeView(ctx, *prev)
			}
			return err
		}
		adm, resumed, err := h.admitAssignment(ctx, p, ctxType, in.ContextID, "activity", in.ActivityID)
		if err != nil {
			return err
		}
		if resumed != nil {
			out = *resumed
			return idempotency.SetResult(ctx, out)
		}
		school, _, err := h.studentSchool(ctx, p)
		if err != nil {
			return err
		}
		src, err := LoadActivityVersion(ctx, h.Docs, h.Registry, in.ActivityID, adm.VersionID)
		if err != nil {
			return err
		}
		src.Config.Feedback.Timing = adm.FeedbackTiming
		if err := h.checkStartRate(ctx, p.UserID); err != nil {
			return err
		}
		a, rendered, err := h.newActivityAttempt(ctx, p, school, src, attempt.ModeAssignment, "assignment", adm.AssignmentID, nil,
			in.AcceptLanguage, key, h.now())
		if err != nil {
			return err
		}
		if out, err = h.view(ctx, a, rendered, &src, nil, nil); err != nil {
			return err
		}
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// startAssignmentSet starts (or resumes) the attempt of a school assignment on a question set (OQ F062-2, F041-AC-13):
// startAdHocAttempt with mode assignment, source question_set, context assignment; the version pinned by the assignment.
func (h Attempts) startAssignmentSet(ctx context.Context, p authz.Principal, in StartAdHocInput) (out dto.AttemptView, err error) {
	key := idemKey(ctx)
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if prev, err := h.Store.AttemptByKey(ctx, p.UserID, key); err != nil || prev != nil {
			if err == nil {
				out, err = h.view(ctx, *prev, nil, nil, nil, nil)
			}
			return err
		}
		adm, resumed, err := h.admitAssignment(ctx, p, in.ContextType, in.ContextID, "question_set", *in.Source.QuestionSetID)
		if err != nil {
			return err
		}
		if resumed != nil {
			out = *resumed
			return idempotency.SetResult(ctx, out)
		}
		school, _, err := h.studentSchool(ctx, p)
		if err != nil {
			return err
		}
		seed, id, now := randomSeed(), ids.New(), h.now()
		version, qids, err := h.questionSetItems(ctx, adm.ContentID, adm.VersionID, seed)
		if err != nil {
			return err
		}
		if err := h.checkStartRate(ctx, p.UserID); err != nil {
			return err
		}
		items, rendered, err := h.pinItems(ctx, qids, seed, true, nil)
		if err != nil {
			return err
		}
		a := attempt.New(attempt.Start{ID: id, TenantID: *p.TenantID, SchoolID: school, StudentID: p.UserID, Mode: attempt.ModeAssignment,
			ContextType: "assignment", ContextID: adm.AssignmentID, ActivityType: adHocType(items), SourceKind: "question_set",
			SourceVersionID: version, Seed: seed, Now: now, ActivityFeedback: adm.FeedbackTiming, DisplayLanguage: i18ntext.Locale(in.AcceptLanguage),
			IdempotencyKey: key, Items: items})
		if err := h.Store.InsertAttempt(ctx, a); err != nil {
			return err
		}
		if err := h.started(ctx, a); err != nil {
			return err
		}
		if out, err = h.view(ctx, a, rendered, nil, nil, nil); err != nil {
			return err
		}
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}
