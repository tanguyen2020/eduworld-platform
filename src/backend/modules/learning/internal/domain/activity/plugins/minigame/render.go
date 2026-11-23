package minigame

import (
	"github.com/google/uuid"
)

// Display options of an attempt that override the configuration (feedback of a minigame learning activity).
type Display struct {
	// Timing is the FeedbackTiming of the attempt ("" = feedback_policy.timing).
	Timing string
	// FeedbackDisplay is interaction.feedback_display of the activity ("" = the one of the configuration).
	FeedbackDisplay string
	// ShowExplanation and ShowCorrectAnswer override the configuration when set.
	ShowExplanation, ShowCorrectAnswer *bool
	// TimeLimitSeconds is the time limit of the attempt (nil = time_limit_seconds of the configuration).
	TimeLimitSeconds *int
	// Grade is the grade of the student (no speed bonus for grades 1–5; nil = unknown, treated as grades 1–5).
	Grade *int
}

const (
	maxNameLength       = 255
	maxDisplayTimeLimit = 3600
)

func short(s string) string {
	r := []rune(s)
	if len(r) > maxNameLength {
		return string(r[:maxNameLength])
	}
	return s
}

func localized(m map[string]string) map[string]any {
	if m["vi"] == "" || m["en"] == "" {
		return nil
	}
	return map[string]any{"vi": short(m["vi"]), "en": short(m["en"])}
}

func boolOr(b *bool, def bool) bool {
	if b == nil {
		return def
	}
	return *b
}

// TimeLimit is the time limit of an attempt: the one of the activity when set (and within the display bound of the
// MiniGameDisplayConfig), else time_limit_seconds of the configuration.
func (r Rules) TimeLimit(activityLimit *int) *int {
	if activityLimit != nil && *activityLimit > 0 && *activityLimit <= maxDisplayTimeLimit {
		n := *activityLimit
		return &n
	}
	if r.TimeLimitSeconds != nil && *r.TimeLimitSeconds > 0 {
		n := *r.TimeLimitSeconds
		return &n
	}
	return nil
}

// Resumable reports whether an interrupted play may be resumed (timed_challenge never: leaving = abandoned).
func (r Rules) Resumable() bool { return r.Type != TimedChallenge && boolOr(r.Limits.Resumable, true) }

// Prefetch reports whether the client receives every item at start (timed_challenge: no network wait between items;
// every answer is still graded by the server, F073-AC-26).
func (r Rules) Prefetch() bool { return r.Type == TimedChallenge }

// DisplayConfig is the MiniGameDisplayConfig of the public API (F073-AC-07): what the client needs to play — never the
// question source, an answer, a scoring rule, the bonus amounts or an option feedback. The closed shape is the contract;
// the speed bonus is announced only when the student earns it.
func (r Rules) DisplayConfig(d Display) map[string]any {
	timing := d.Timing
	if timing == "" {
		timing = r.FeedbackPolicy.Timing
	}
	out := map[string]any{
		"grade_levels":        append([]int(nil), r.GradeLevels...),
		"question_count":      r.QuestionCount,
		"feedback_timing":     timing,
		"show_explanation":    boolOr(d.ShowExplanation, boolOr(r.FeedbackPolicy.ShowExplanation, true)),
		"show_correct_answer": boolOr(d.ShowCorrectAnswer, boolOr(r.FeedbackPolicy.ShowCorrectAnswer, true)),
		"scoring": map[string]any{"points_per_correct": r.Scoring.PointsPerCorrect, "time_bonus_enabled": r.TimeBonusEnabled(d.Grade),
			"streak_bonus_enabled": r.StreakBonusEnabled()},
		"resumable":      r.Resumable(),
		"prefetch_items": r.Prefetch(),
	}
	if n := localized(r.Name); n != nil {
		out["name"] = n
	}
	if ins := localized(r.Instructions); ins != nil {
		out["instructions"] = ins
	}
	if tl := r.TimeLimit(d.TimeLimitSeconds); tl != nil {
		out["time_limit_seconds"] = *tl
	}
	if r.PerItemTimeSeconds != nil && *r.PerItemTimeSeconds > 0 {
		out["per_item_time_seconds"] = *r.PerItemTimeSeconds
	}
	if timing == "immediate" {
		fd := d.FeedbackDisplay
		if fd == "" {
			fd = r.FeedbackPolicy.FeedbackDisplay
		}
		if fd != "" {
			out["feedback_display"] = fd
		}
	}
	p := map[string]any{}
	for k, v := range map[string]string{"scene_variant": r.Presentation.SceneVariant, "theme": r.Presentation.Theme,
		"background_music_id": r.Presentation.BackgroundMusicID, "mascot_npc_id": r.Presentation.MascotNPCID} {
		if v != "" {
			p[k] = v
		}
	}
	if len(p) > 0 {
		out["presentation"] = p
	}
	return out
}

// RenderPayload is the render payload of a minigame attempt: {minigame_type, minigame_config_version_id, config}.
func (r Rules) RenderPayload(versionID uuid.UUID, d Display) map[string]any {
	return map[string]any{"minigame_type": r.Type, "minigame_config_version_id": versionID, "config": r.DisplayConfig(d)}
}
