package analytics

import "math"

// MetricVersion is the current definition of the learning rollup metrics (F043-AC-05): accuracy and rapid_guess are
// numerator/denominator pairs over the scored first answers, accuracy excludes the rapid guesses. A new formula bumps it;
// dashboards read the current version only.
const MetricVersion = 1

// Ratio returns num/den rounded to 3 decimals, nil when den is 0 (no data, never 0 %).
func Ratio(num, den int64) *float64 {
	if den <= 0 {
		return nil
	}
	v := math.Round(float64(num)/float64(den)*1000) / 1000
	return &v
}

// Accuracy is the accuracy pair of scored first answers without the rapid guesses (LEARNING_ANALYTICS §4, F043-AC-05):
// numerator correct − correct rapid guesses, denominator answered − rapid guesses.
func Accuracy(answered, correct, rapid, correctRapid int64) (num, den int64) {
	den = max(answered-rapid, 0)
	num = min(max(correct-correctRapid, 0), den)
	return num, den
}

// Levels are the mastery levels in the order of every level distribution (level_dist[0..4]).
var Levels = [5]string{"not_started", "beginning", "developing", "proficient", "mastered"}

// LevelIndex is the position of level in Levels (-1 when unknown).
func LevelIndex(level string) int {
	for i, l := range Levels {
		if l == level {
			return i
		}
	}
	return -1
}

// BelowProficient is the share of the students with evidence (level above not_started) that are below proficient
// (beginning + developing), nil without evidence ("LO cần củng cố", SCHOOL_LMS §8.1).
func BelowProficient(dist []int) *float64 {
	if len(dist) != len(Levels) {
		return nil
	}
	with := int64(dist[1] + dist[2] + dist[3] + dist[4])
	return Ratio(int64(dist[1]+dist[2]), with)
}
