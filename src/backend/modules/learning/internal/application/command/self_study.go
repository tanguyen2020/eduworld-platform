package command

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/attemptview"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/attempt"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/mastery"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/selection"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/retrywrong"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// PracticeConfig holds the parameters of CONFIG_DEFAULTS used by the self-study use cases of F032; the values are the
// defaults of their owner documents until module learning reads them from platform settings (like attempt.Policy of
// F030) [Proposed].
type PracticeConfig struct {
	RapidGuess attempt.RapidGuess      // rapid_guess_min_response_ms (LEARNING_ANALYTICS §3.1)
	Mix        selection.Mix           // interleave_mix (LEARNING_EXPERIENCE §5.2)
	Schedule   retrywrong.Schedule     // retry_wrong_schedule (LEARNING_EXPERIENCE §6.1)
	Diagnostic mastery.DiagnosticPrior // diagnostic_prior (LEARNING_ANALYTICS §3.5)
	// AvoidRecent is avoid_recent.window_days of QUESTION_BANK §6 (7 days).
	AvoidRecent time.Duration
	// RecommendationTTL: an active recommendation expires after 7 days (recommendation_follow_rate window, §4) [Proposed].
	RecommendationTTL time.Duration
	// Skips: a target skipped this many times is not recommended again (LEARNING_ANALYTICS §6).
	Skips int
	// Shown is the number of recommendations shown at once (3); Stored bounds the active rows of a student.
	Shown, Stored int
	// DayLocation is the zone of the calendar days of the review list (Asia/Ho_Chi_Minh).
	DayLocation string
}

// DefaultPracticeConfig returns the defaults of CONFIG_DEFAULTS.
func DefaultPracticeConfig() PracticeConfig {
	return PracticeConfig{RapidGuess: attempt.DefaultRapidGuess(), Mix: selection.DefaultMix(), Schedule: retrywrong.DefaultSchedule(),
		Diagnostic: mastery.DefaultDiagnosticPrior(), AvoidRecent: 7 * 24 * time.Hour, RecommendationTTL: 7 * 24 * time.Hour, Skips: 3,
		Shown: 3, Stored: 20, DayLocation: "Asia/Ho_Chi_Minh"}
}

// day is the calendar day of t in the day location.
func (c PracticeConfig) day(t time.Time) string {
	loc, err := time.LoadLocation(c.DayLocation)
	if err != nil || c.DayLocation == "" {
		loc = time.UTC
	}
	return t.In(loc).Format(time.DateOnly)
}

// SelfStudy are the stores of the self-study use cases of F032 used by the attempt engine (nil in Attempts = F030 only:
// practice from a question set or a selection, no review list, no rapid_guess).
type SelfStudy struct {
	Wrong           write.WrongQuestionStore
	Recommendations write.RecommendationStore
	Reader          write.SelfStudyReader
	Masteries       write.MasteryStore
	Config          PracticeConfig
	Metrics         SelfStudyMetrics
	// Subjects resolves the subject label of the metrics (nil = "unknown").
	Subjects SubjectCodes
}

// SelfStudyMetrics counts eduworld_hint_requests_total{subject,grade_band}, eduworld_rapid_guess_responses_total{subject,grade_band}
// (MONITORING §3) and eduworld_learning_recommendations_generated_total{rule_code}; nil = not counted.
type SelfStudyMetrics interface {
	HintRevealed(subject, gradeBand string)
	RapidGuess(subject, gradeBand string)
	RecommendationGenerated(ruleCode string)
}

// reviewModes are the modes whose first-try wrong answers enter the review list (study modes; diagnostic, exam,
// mini-games and competition do not) [Proposed].
var reviewModes = []attempt.Mode{attempt.ModePractice, attempt.ModeRetryWrong, attempt.ModeAssignment, attempt.ModeQuest}

// recordAnswer updates the review list after a scored answer (F032-AC-21): a first-try wrong answer of a study mode
// enters the list (or restarts the schedule of an active entry); a correct answer of the original question, or of the
// sibling reviewing the entry in a retry_wrong attempt, moves the entry on and clears it on the second correct day.
// Rapid guesses and unscored retries change nothing.
func (s *SelfStudy) recordAnswer(ctx context.Context, a attempt.Attempt, it attempt.Item, qv dto.QuestionVersion, ans attempt.Answer,
	correct, rapid bool, now time.Time) error {
	if s == nil || s.Wrong == nil || !ans.IsScored || rapid || !slices.Contains(reviewModes, a.Mode) {
		return nil
	}
	var entry *write.WrongEntryRow
	var err error
	if a.Mode == attempt.ModeRetryWrong {
		if entry, err = s.Wrong.RetryEntry(ctx, a.ID, it.Position); err != nil {
			return err
		}
	}
	if entry == nil {
		if entry, err = s.Wrong.ActiveEntry(ctx, a.StudentID, qv.QuestionID); err != nil {
			return err
		}
	}
	cfg := s.Config
	if entry == nil {
		if correct || ans.AttemptNo != 1 {
			return nil
		}
		subject := qv.SubjectID
		var sp *uuid.UUID
		if subject != uuid.Nil {
			sp = &subject
		}
		objectives := it.ObjectiveIDs
		if len(objectives) > 20 {
			objectives = objectives[:20]
		}
		d := qv.Difficulty
		if d < 1 || d > 5 {
			d = 3
		}
		_, err := s.Wrong.InsertEntry(ctx, write.WrongEntryRow{ID: ids.New(), TenantID: a.TenantID, SchoolID: a.SchoolID, StudentID: a.StudentID,
			QuestionID: qv.QuestionID, QuestionVersionID: it.QuestionVersionID, SubjectID: sp, ObjectiveIDs: objectives, QuestionType: it.QuestionType,
			Difficulty: d, FirstWrongAttemptID: a.ID, Entry: retrywrong.New(now, cfg.Schedule)})
		return err
	}
	if correct {
		entry.Entry.Correct(now, cfg.day(now), cfg.Schedule)
	} else {
		entry.Entry.Wrong(now, cfg.Schedule)
	}
	ok, err := s.Wrong.UpdateEntry(ctx, *entry)
	if err == nil && !ok {
		return problem.New(problem.VersionMismatch, "the review entry changed meanwhile").Retry()
	}
	return err
}

// rapidGuess flags a scored answer answered too fast to read the stem (F032-AC-16, AC-19); nil SelfStudy flags nothing.
func (s *SelfStudy) rapidGuess(ctx context.Context, a attempt.Attempt, it attempt.Item, qv dto.QuestionVersion, ans attempt.Answer, grade *int) bool {
	if s == nil {
		return false
	}
	rapid := s.Config.RapidGuess.Flag(attempt.GuessInput{Mode: a.Mode, QuestionType: it.QuestionType, ItemMinMS: qv.MinResponseMS, Grade: grade,
		ResponseMS: ans.ResponseMS, Scored: ans.IsScored})
	if rapid && s.Metrics != nil {
		s.Metrics.RapidGuess(subjectLabel(ctx, s.Subjects, nonNilSubject(qv.SubjectID)), gradeBandLabel(grade))
	}
	return rapid
}

// ---------- hints

// RevealHintInput is the path of revealNextHint.
type RevealHintInput struct {
	AttemptID uuid.UUID
	Position  int
}

// RevealHint opens the next hint level of an item (F032-AC-10…AC-15, ACTIVITY_ENGINE §8.3): the hints of the pinned
// question version, in order; the level is recorded on the item (hints_revealed) in the transaction; the same
// Idempotency-Key replays the same level (pkg/httpx). Hints never cost score nor XP.
func (h Attempts) RevealHint(ctx context.Context, in RevealHintInput) (out dto.HintView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.RevealNextHint")
	defer func() { done(err) }()
	p, err := student(ctx)
	if err != nil {
		return out, err
	}
	if in.Position < 1 {
		return out, problem.Invalid("/position", problem.SubInvalid, "no item at this position")
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		a, err := h.loadOwn(ctx, p, in.AttemptID)
		if err != nil {
			return err
		}
		it, err := a.Item(in.Position)
		if err != nil {
			return attemptProblem(err)
		}
		qv, err := h.Questions.QuestionVersionByID(ctx, it.QuestionVersionID)
		if err != nil {
			return err
		}
		var hints []json.RawMessage
		if len(qv.Hints) > 0 {
			_ = json.Unmarshal(qv.Hints, &hints)
		}
		total := min(len(hints), attempt.MaxHintLevels)
		row := a.RowVersion
		level, err := a.RevealHint(in.Position, total, h.now(), h.policy())
		if err != nil {
			return attemptProblem(err)
		}
		it, _ = a.Item(in.Position)
		if err := h.Store.UpdateItem(ctx, *it); err != nil {
			return err
		}
		if err := h.Store.UpdateAttempt(ctx, a, row); err != nil {
			return err
		}
		if h.Self != nil && h.Self.Metrics != nil {
			var grade *int
			if h.Students != nil {
				if _, grade, err = h.Students.StudentSchool(ctx, a.StudentID); err != nil {
					return err
				}
			}
			h.Self.Metrics.HintRevealed(subjectLabel(ctx, h.Self.Subjects, nonNilSubject(qv.SubjectID)), gradeBandLabel(grade))
		}
		out = dto.HintView{Position: in.Position, Level: level, TotalLevels: total,
			Hint: dto.HintTextView{Text: attemptview.Truncate(attemptview.RichText(hints[level-1], a.DisplayLanguage), 2000)}}
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// ---------- recommendations followed by an attempt

// followedRecommendation returns the recommendation id of a start when it is an active recommendation of the student;
// an unknown id, or one of another student, is ignored without error (F032-AC-32).
func (h Attempts) followedRecommendation(ctx context.Context, p authz.Principal, id *uuid.UUID) (*write.RecommendationRow, error) {
	if id == nil || h.Self == nil || h.Self.Recommendations == nil {
		return nil, nil
	}
	r, err := h.Self.Recommendations.Recommendation(ctx, p.UserID, *id)
	if err != nil || r == nil || r.Status != "active" {
		return nil, err
	}
	return r, nil
}

// startRecommendation moves the followed recommendation to started with the attempt (F032-AC-32).
func (h Attempts) startRecommendation(ctx context.Context, r *write.RecommendationRow, attemptID uuid.UUID) error {
	if r == nil {
		return nil
	}
	now := h.now()
	r.Status, r.StartedAttemptID, r.ClosedAt = "started", &attemptID, &now
	ok, err := h.Self.Recommendations.UpdateRecommendation(ctx, *r)
	if err == nil && !ok {
		return problem.New(problem.VersionMismatch, "the recommendation changed meanwhile").Retry()
	}
	return err
}

func recommendationID(r *write.RecommendationRow) *uuid.UUID {
	if r == nil {
		return nil
	}
	id := r.ID
	return &id
}

// tooFastRatio is the share of the scored answers of a submitted attempt flagged rapid_guess (ANTI_CHEAT §3 "too_fast",
// metric eduworld_answers_too_fast_ratio of §10). nil when nothing is measured: no metrics, rapid_guess not detected
// (F030 behaviour without SelfStudy) or no scored answer.
func (h Attempts) tooFastRatio(ctx context.Context, a attempt.Attempt) (*float64, error) {
	if h.Metrics == nil || h.Self == nil {
		return nil, nil
	}
	scored, rapid, err := h.Store.ScoredAnswerCounts(ctx, a.ID)
	if err != nil || scored == 0 {
		return nil, err
	}
	r := float64(rapid) / float64(scored)
	return &r, nil
}
