package command

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/attemptview"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/attempt"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/selection"
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
	// Challenge fields (mode challenge, F076): the difficulty band of the level, the grade of the student, the
	// objectives of the challenge, the question count and the server time limit of the attempt.
	DifficultyMin    *int
	DifficultyMax    *int
	GradeLevel       *int
	ObjectiveIDs     []uuid.UUID
	QuestionCount    *int
	TimeLimitSeconds *int
}

// internalModes are the modes module game opens (quest, minigame: F030/F073; challenge: F076).
var internalModes = []attempt.Mode{attempt.ModeQuest, attempt.ModeMinigame, attempt.ModeChallenge}

var internalContexts = map[attempt.Mode]string{attempt.ModeQuest: "quest_step", attempt.ModeMinigame: "minigame_session",
	attempt.ModeChallenge: "challenge_attempt"}

// CreateInternal opens an attempt of a quest step, a mini-game or a challenge activity for a student, on behalf of
// module game (F030-AC-25, F076-AC-08): the activity is pinned at its current release (or the version given when it is
// published); a challenge attempt draws its questions for the grade of the student inside the difficulty band of the
// level (challengeDraw); the client then works
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
		return out, problem.Invalid("/mode", problem.SubInvalid, "module game opens quest, minigame or challenge attempts only")
	}
	if internalContexts[mode] != in.ContextType {
		return out, problem.Invalid("/context/context_type", problem.SubInvalid, "does not match the mode")
	}
	draw, err := challengeBand(mode, in)
	if err != nil {
		return out, err
	}
	switch in.ContentType {
	case "activity":
	case contentMinigameConfig:
		if draw != nil {
			return out, problem.Invalid("/activity/content_type", problem.SubInvalid, "a challenge opens learning activities only")
		}
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
		school, grade, err := h.studentSchool(ctx, p)
		if err != nil {
			return err
		}
		if draw != nil && (grade == nil || *grade != draw.grade) {
			// The questions are drawn for the grade of the student only (ADR-0095): another grade is never served.
			return problem.Invalid("/activity/grade_level", problem.SubInvalid, "is not the grade of the student")
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
		a, _, err := h.newActivityAttempt(ctx, p, school, src, mode, in.ContextType, in.ContextID, nil, in.AcceptLanguage, key, h.now(), draw)
		if err != nil {
			return err
		}
		out = internalCreated(a)
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// challengeDraw is the question draw of a challenge attempt (F076-AC-09): published questions of the tenant pool for
// the subject of the activity, the grade of the student, the difficulty band of the level and the objectives of the
// challenge, instead of the activity items (CHALLENGE_BOSS_SYSTEM §2.2: no content of another grade or band).
type challengeDraw struct {
	min, max, grade int
	objectives      []uuid.UUID
	count           int // 0 = the natural item count of the activity
	timeLimit       *int
}

// challengeBand checks the challenge fields of createAttempt: required with mode challenge (band 1..5, min ≤ max, grade
// 1..9), ignored by the other modes.
func challengeBand(mode attempt.Mode, in CreateInternalInput) (*challengeDraw, error) {
	if mode != attempt.ModeChallenge {
		return nil, nil
	}
	switch {
	case in.DifficultyMin == nil || in.DifficultyMax == nil:
		return nil, problem.Invalid("/activity/difficulty_min", problem.SubRequired, "the difficulty band is required with mode challenge")
	case *in.DifficultyMin < 1 || *in.DifficultyMax > 5 || *in.DifficultyMin > *in.DifficultyMax:
		return nil, problem.Invalid("/activity/difficulty_min", problem.SubInvalid, "1 ≤ difficulty_min ≤ difficulty_max ≤ 5")
	case in.GradeLevel == nil || *in.GradeLevel < 1 || *in.GradeLevel > 9:
		return nil, problem.Invalid("/activity/grade_level", problem.SubRequired, "the grade of the student is required with mode challenge")
	case len(in.ObjectiveIDs) > 30:
		return nil, problem.Invalid("/activity/objective_ids", problem.SubInvalid, "at most 30")
	case in.QuestionCount != nil && (*in.QuestionCount < 1 || *in.QuestionCount > 50):
		return nil, problem.Invalid("/activity/question_count", problem.SubInvalid, "1..50")
	case in.TimeLimitSeconds != nil && (*in.TimeLimitSeconds < 10 || *in.TimeLimitSeconds > 7200):
		return nil, problem.Invalid("/activity/time_limit_seconds", problem.SubInvalid, "10..7200")
	}
	d := &challengeDraw{min: *in.DifficultyMin, max: *in.DifficultyMax, grade: *in.GradeLevel, objectives: in.ObjectiveIDs,
		timeLimit: in.TimeLimitSeconds}
	if in.QuestionCount != nil {
		d.count = *in.QuestionCount
	}
	return d, nil
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

// challengeQuestions draws the questions of a challenge attempt (F076-AC-09, SC-002): the published questions of the
// tenant pool for the subject of the activity, exactly the grade of the student and the difficulty band of the level,
// of the objectives of the challenge (none = every objective) and of the question types the activity renders. count is
// the draw size, else the natural item count of the activity (at least one). A band the pool cannot fill is 422
// QUESTION_POOL_INSUFFICIENT (strict selection: never widened), never a question of another grade or outside the band.
func (h Attempts) challengeQuestions(ctx context.Context, d challengeDraw, src ActivitySource, allowed []string, natural int,
	seed int64) ([]uuid.UUID, error) {
	n := d.count
	if n == 0 {
		n = max(natural, 1)
	}
	f := selection.Filters{SubjectID: src.Version.SubjectID, GradeLevels: []int{d.grade}, ObjectiveIDs: d.objectives, Types: allowed,
		Difficulty: &selection.Range{Min: d.min, Max: d.max}}
	cs, err := h.Delivery.Select(ctx, selection.Request{Rule: selection.Rule{Filters: f, Count: n}, Seed: uint64(seed), Strict: true})
	if err != nil {
		return nil, err
	}
	out := make([]uuid.UUID, len(cs))
	for i, c := range cs {
		out[i] = c.VersionID
	}
	return out, nil
}
