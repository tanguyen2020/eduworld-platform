package minigame

import (
	"math"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
)

// YoungGrade is the highest grade without speed bonus nor wrong penalty (grades 1–5, F073-AC-09, NFR-A11Y-002).
const YoungGrade = 5

// young reports a student of grades 1–5; an unknown grade is treated the same way (the safer rule for children).
func young(grade *int) bool { return grade == nil || *grade <= YoungGrade }

// bonuses reports whether the game score adds the speed and streak bonuses (result_calculation.game_score).
func (r Rules) bonuses() bool { return r.ResultCalculation.GameScore != GamePointsOnly }

// TimeBonusEnabled reports whether a student of grade earns the speed bonus.
func (r Rules) TimeBonusEnabled(grade *int) bool {
	return r.Scoring.TimeBonus != nil && r.Scoring.TimeBonus.Enabled && r.bonuses() && !young(grade)
}

// StreakBonusEnabled reports whether the streak bonus applies.
func (r Rules) StreakBonusEnabled() bool {
	s := r.Scoring.StreakBonus
	return s != nil && s.Enabled && s.EveryNCorrect >= 2 && r.bonuses()
}

// zeroBonusAtMS is the server interval at which the speed bonus reaches 0: per_item_time_seconds when set, else the time
// of one item in time_limit_seconds (time_limit_seconds / question_count), else twice full_bonus_within_ms.
func (r Rules) zeroBonusAtMS() int64 {
	switch {
	case r.PerItemTimeSeconds != nil && *r.PerItemTimeSeconds > 0:
		return int64(*r.PerItemTimeSeconds) * 1000
	case r.TimeLimitSeconds != nil && *r.TimeLimitSeconds > 0 && r.QuestionCount > 0:
		return int64(*r.TimeLimitSeconds) * 1000 / int64(r.QuestionCount)
	}
	return 2 * int64(r.Scoring.TimeBonus.FullBonusWithinMS)
}

// TimeBonus is the speed bonus of a correct answer given serverMS, the server interval from the moment the item was
// served (the start or the previous answer) to the answer — never client_elapsed_ms (MINIGAME_ENGINE §6): max_bonus up
// to full_bonus_within_ms, then linear down to 0 at zeroBonusAtMS (rounded to the nearest point), 0 afterwards; when
// zeroBonusAtMS is not after full_bonus_within_ms the bonus is max_bonus up to full_bonus_within_ms and 0 afterwards.
func (r Rules) TimeBonus(serverMS int64) int {
	tb := r.Scoring.TimeBonus
	if tb == nil || tb.MaxBonus <= 0 {
		return 0
	}
	full, zero := int64(max(tb.FullBonusWithinMS, 0)), r.zeroBonusAtMS()
	serverMS = max(serverMS, 0)
	switch {
	case serverMS <= full:
		return tb.MaxBonus
	case zero <= full || serverMS >= zero:
		return 0
	}
	return int(math.Round(float64(tb.MaxBonus) * float64(zero-serverMS) / float64(zero-full)))
}

// Points applies one graded answer to the game score (MinigameScorer, MINIGAME_ENGINE §6, F073-AC-08, AC-09): an answer
// that is not scored (an unscored retry) changes nothing; a correct answer adds points_per_correct, the speed bonus (not
// for grades 1–5) and the streak bonus each every_n_correct consecutive correct answers (bonuses only with
// points_plus_bonus); a wrong answer resets the streak and subtracts wrong_penalty (0 for grades 1–5). The game score
// never goes below 0; Points is what the answer really added.
func (r Rules) Points(in activity.GameAnswer, grade *int) activity.GamePoints {
	score, streak := max(in.Score, 0), max(in.Streak, 0)
	if !in.Scored {
		return activity.GamePoints{Score: score, Streak: streak}
	}
	delta := 0
	if in.Correct {
		streak++
		delta = r.Scoring.PointsPerCorrect
		if r.TimeBonusEnabled(grade) {
			delta += r.TimeBonus(in.ServerMS)
		}
		if r.StreakBonusEnabled() && streak%r.Scoring.StreakBonus.EveryNCorrect == 0 {
			delta += max(r.Scoring.StreakBonus.Bonus, 0)
		}
	} else {
		streak = 0
		if !young(grade) {
			delta = -max(r.Scoring.WrongPenalty, 0)
		}
	}
	next := max(score+delta, 0)
	return activity.GamePoints{Points: next - score, Score: next, Streak: streak}
}

// LearningScore is the learning score of result_calculation from the final item results: sum_question_scores = Σ item
// score over Σ item max score; correct_ratio = correct items over items. It never uses the game score.
func (r Rules) LearningScore(items []activity.ItemResult) (score, maxScore float64, correct int) {
	for _, it := range items {
		maxScore += it.MaxScore
		if it.Answered {
			score += it.Score
			if it.IsCorrect {
				correct++
			}
		}
	}
	if r.ResultCalculation.LearningScore == LearningCorrectRatio {
		score, maxScore = float64(correct), float64(len(items))
	}
	return activity.Round2(score), activity.Round2(maxScore), correct
}

// CompletionMet applies completion_rule: complete_all = every item answered; min_correct_ratio = correct items / items ≥
// threshold; min_learning_score = learning score ≥ threshold.
func (r Rules) CompletionMet(items []activity.ItemResult) bool {
	if len(items) == 0 {
		return false
	}
	answered, correct := 0, 0
	for _, it := range items {
		if it.Answered {
			answered++
			if it.IsCorrect {
				correct++
			}
		}
	}
	th := 0.0
	if r.CompletionRule.Threshold != nil {
		th = *r.CompletionRule.Threshold
	}
	switch r.CompletionRule.Type {
	case MinCorrectRatio:
		return float64(correct)/float64(len(items)) >= th
	case MinLearningScore:
		s, _, _ := r.LearningScore(items)
		return s >= th
	}
	return answered == len(items)
}
