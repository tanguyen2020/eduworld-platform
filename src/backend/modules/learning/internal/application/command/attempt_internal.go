package command

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/attemptview"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/attempt"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// CreateInternalInput is AttemptCreateRequest of the facade operation createAttempt (x-allowed-callers: game).
type CreateInternalInput struct {
	StudentID        uuid.UUID
	Mode             string
	ContentType      string
	ContentID        uuid.UUID
	ContentVersionID *uuid.UUID
	ContextType      string
	ContextID        uuid.UUID
	AcceptLanguage   string
}

// internalModes are the modes module game opens in F030 (challenge: F076/F103).
var internalModes = []attempt.Mode{attempt.ModeQuest, attempt.ModeMinigame}

var internalContexts = map[attempt.Mode]string{attempt.ModeQuest: "quest_step", attempt.ModeMinigame: "minigame_session"}

// CreateInternal opens an attempt of a quest step or mini-game for a student, on behalf of module game (F030-AC-25):
// the activity is pinned at its current release (or the version given when it is published); the client then works
// it through the public attempt API and the result goes back to module game through activity.* events. The
// principal of the call is the student on whose behalf module game acts (pkg/modcall) [Proposed].
func (h Attempts) CreateInternal(ctx context.Context, in CreateInternalInput) (out InternalCreated, err error) {
	ctx, done := observability.UseCase(ctx, "learning.facade.CreateAttempt")
	defer func() { done(err) }()
	p, err := student(ctx)
	if err != nil {
		return out, err
	}
	if p.UserID != in.StudentID {
		return out, problem.New(problem.PermissionDenied, "the attempt is opened on behalf of another student")
	}
	mode := attempt.Mode(in.Mode)
	if !contains(internalModes, mode) {
		return out, problem.Invalid("/mode", problem.SubInvalid, "module game opens quest or minigame attempts only")
	}
	if internalContexts[mode] != in.ContextType {
		return out, problem.Invalid("/context/context_type", problem.SubInvalid, "does not match the mode")
	}
	switch in.ContentType {
	case "activity":
	case contentMinigameConfig:
		return h.createMinigameInternal(ctx, p, in, mode)
	default:
		return out, problem.Invalid("/activity/content_type", problem.SubInvalid, "only learning activities and mini-game configurations are opened")
	}
	key := idemKey(ctx)
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if prev, err := h.Store.AttemptByKey(ctx, p.UserID, key); err != nil || prev != nil {
			if err == nil {
				out = internalCreated(*prev)
			}
			return err
		}
		school, _, err := h.studentSchool(ctx, p)
		if err != nil {
			return err
		}
		src, err := LoadActivityVersion(ctx, h.Docs, h.Registry, in.ContentID, nil)
		if err != nil {
			return err
		}
		if in.ContentVersionID != nil && *in.ContentVersionID != src.Version.ID {
			// Another published version may be pinned only while it is still published.
			if src, err = LoadActivityVersion(ctx, h.Docs, h.Registry, in.ContentID, in.ContentVersionID); err != nil {
				return err
			}
			if src.Version.Status != "published" {
				return problem.New(problem.ContentNotPublished, "the activity version is not published")
			}
		}
		a, _, err := h.newActivityAttempt(ctx, p, school, src, mode, in.ContextType, in.ContextID, nil, in.AcceptLanguage, key, h.now())
		if err != nil {
			return err
		}
		out = internalCreated(a)
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// InternalCreated is AttemptCreated (the stored facade answer).
type InternalCreated struct {
	AttemptID        uuid.UUID  `json:"attempt_id"`
	Status           string     `json:"status"`
	QuestionCount    int        `json:"question_count"`
	StartedAt        string     `json:"started_at"`
	DeadlineAt       *string    `json:"deadline_at,omitempty"`
	ContentVersionID *uuid.UUID `json:"content_version_id,omitempty"`
}

func internalCreated(a attempt.Attempt) InternalCreated {
	out := InternalCreated{AttemptID: a.ID, Status: string(a.Status), QuestionCount: len(a.Items), StartedAt: a.StartedAt.Format(time.RFC3339Nano)}
	if a.DeadlineAt != nil {
		d := a.DeadlineAt.Format(time.RFC3339Nano)
		out.DeadlineAt = &d
	}
	v := a.SourceVersionID
	out.ContentVersionID = &v
	return out
}

// contentMinigameConfig is ActivityRef.content_type of a mini-game configuration of module game (F073).
const contentMinigameConfig = "minigame_config"

// createMinigameInternal opens a mini-game play for module game (startMiniGame free_play, quest step minigame —
// F073-AC-06, AC-16): content_id is the configuration, content_version_id the version module game pinned (required).
// The version must be in the projection ref.minigame_config_versions, else 409 CONFLICT (module game answers 503 with
// Retry-After); the items come from its question source (QUESTION_POOL_INSUFFICIENT without attempt when it is short).
func (h Attempts) createMinigameInternal(ctx context.Context, p authz.Principal, in CreateInternalInput, mode attempt.Mode) (out InternalCreated, err error) {
	if in.ContentVersionID == nil {
		return out, problem.Invalid("/activity/content_version_id", problem.SubRequired, "is required for a mini-game configuration")
	}
	key := idemKey(ctx)
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if prev, err := h.Store.AttemptByKey(ctx, p.UserID, key); err != nil || prev != nil {
			if err == nil {
				out = internalCreated(*prev)
			}
			return err
		}
		school, grade, err := h.studentSchool(ctx, p)
		if err != nil {
			return err
		}
		row, err := h.projected(ctx, *in.ContentVersionID)
		if err != nil {
			return err
		}
		if row.ConfigID != in.ContentID {
			return problem.Invalid("/activity/content_version_id", problem.SubInvalid, "is not a version of the mini-game configuration")
		}
		plugin, err := ActivityPlugin(h.Registry, string(activity.Minigame))
		if err != nil {
			return err
		}
		src, err := attemptview.MinigameSource(row, plugin, grade)
		if err != nil {
			return problem.New(problem.ActivityConfigInvalid, "the mini-game configuration cannot be played")
		}
		a, _, err := h.newMinigameAttempt(ctx, p, school, minigameStart{src: src, mode: mode, ctxType: in.ContextType, ctxID: in.ContextID,
			acceptLanguage: in.AcceptLanguage, key: key, now: h.now()})
		if err != nil {
			return err
		}
		out = internalCreated(a)
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}
