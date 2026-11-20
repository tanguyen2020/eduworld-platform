package videolesson

import "slices"

// CueProcessed reports whether an answer to a cue closes it (VIDEO_LEARNING §5, F065-AC-09): a right answer, or a wrong
// one once the student has answered attempts_per_question times on a cue with allow_skip_after_wrong.
func CueProcessed(c Cue, isCorrect bool, answers, attemptsPerQuestion int) bool {
	return isCorrect || (c.SkipAfterWrong() && answers >= max(attemptsPerQuestion, 1))
}

// PendingCues are the cues not yet processed, in at_ms order (ActivityProgressState.pending_cue_ids); required only
// lists the required ones.
func PendingCues(c Config, processed []string, requiredOnly bool) []string {
	out := []string{}
	for _, q := range c.OrderedCues() {
		if slices.Contains(processed, q.CueID) || (requiredOnly && !q.IsRequired()) {
			continue
		}
		out = append(out, q.CueID)
	}
	return out
}

// Outcome is the completion of a video attempt at submit (F065-AC-10, AC-25).
type Outcome struct {
	Completed bool
	// MissingRequired are the required cues not processed when require_required_cues (422 CUE_ANSWER_REQUIRED).
	MissingRequired []string
}

// Evaluate applies the completion rule (VIDEO_LEARNING §7, CONFIG_DEFAULTS video_completion): completed when the
// plausible watched ratio reaches min_watched_ratio and every required cue is processed (require_required_cues); an
// interactive video is completed once an ending segment was reached (its main video stops at the branch point).
func Evaluate(c Config, ratio float64, processed []string, reachedEnding bool) Outcome {
	o := Outcome{MissingRequired: []string{}}
	if c.RequireRequiredCues() {
		o.MissingRequired = PendingCues(c, processed, true)
	}
	if c.Interactive() {
		// The main video stops at the branch point: the ending segment replaces the watched ratio (VIDEO_LEARNING §6).
		o.Completed = len(o.MissingRequired) == 0 && reachedEnding
		return o
	}
	o.Completed = len(o.MissingRequired) == 0 && ratio+1e-9 >= c.MinWatchedRatio()
	return o
}
