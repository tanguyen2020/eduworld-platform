// Package regulation maps the learning data of a student to the reference levels of the Vietnamese assessment circulars
// (F044 US3, ASSESSMENT_REGULATION_PROFILE §4): TT27/2020 for primary grades, TT22/2021 for lower secondary. The result
// is for reference only and never replaces the official assessment (ADR-0072); it is never shown to students. Pure Go.
package regulation

import "time"

// Profile codes.
const (
	ProfilePrimaryTT27        = "primary_tt27"
	ProfileLowerSecondaryTT22 = "lower_secondary_tt22"
)

// Level codes (learning.rm_student_subject_reference_levels.level_code; labels are i18n).
const (
	LevelExcellent        = "excellent"
	LevelCompleted        = "completed"
	LevelNotCompleted     = "not_completed"
	LevelPass             = "pass"
	LevelNotPass          = "not_pass"
	LevelInsufficientData = "insufficient_data"
)

// Periods of an academic year.
const (
	PeriodMidTerm1 = "mid_term_1"
	PeriodEndTerm1 = "end_term_1"
	PeriodMidTerm2 = "mid_term_2"
	PeriodEndTerm2 = "end_term_2"
)

// Rules are the thresholds of assessment_reference_thresholds (CONFIG_DEFAULTS [Confirmed — Technical Architect
// 2026-11-15], owner ASSESSMENT_REGULATION_PROFILE §4); stored in learning.assessment_profiles.rules, a change creates a new
// profile version (F044-AC-14).
type Rules struct {
	MinCoverage              float64 `json:"min_coverage"`
	MinEvidence              int     `json:"min_evidence"`
	ExcellentProficientRatio float64 `json:"excellent_proficient_ratio"`
	ExcellentMasteredRatio   float64 `json:"excellent_mastered_ratio"`
	CompletedDevelopingRatio float64 `json:"completed_developing_ratio"`
	PassDevelopingRatio      float64 `json:"pass_developing_ratio"`
}

// DefaultRules returns the default thresholds of ASSESSMENT_REGULATION_PROFILE §4.1, §4.2.
func DefaultRules() Rules {
	return Rules{MinCoverage: 0.7, MinEvidence: 3, ExcellentProficientRatio: 0.8, ExcellentMasteredRatio: 0.4,
		CompletedDevelopingRatio: 0.6, PassDevelopingRatio: 0.6}
}

// DefaultDisclaimer is the text attached to every reference answer and export (F044-AC-12) [Proposed wording, legal review
// before the pilot: ASSESSMENT_REGULATION_PROFILE §6].
func DefaultDisclaimer() map[string]string {
	return map[string]string{
		"vi": "Mức tham khảo do EduWorld ước lượng từ dữ liệu học tập, chỉ để nhà trường tham khảo, không thay thế đánh giá chính thức theo Thông tư 27/2020 và Thông tư 22/2021.",
		"en": "Reference level estimated by EduWorld from learning data, for the school's reference only; it does not replace the official assessment under Circulars 27/2020 and 22/2021.",
	}
}

// Counts of the objectives in scope of a student × subject × period: Covered = objectives with ≥ MinEvidence evidence;
// the level counts are over the covered objectives (an objective without evidence reaches no level).
type Counts struct {
	InScope           int
	Covered           int
	AtLeastDeveloping int
	AtLeastProficient int
	Mastered          int
}

// Result of the mapping.
type Result struct {
	Level           string
	Coverage        float64
	ProficientRatio float64
	MasteredRatio   float64
}

func ratio(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) / float64(d)
}

// Map applies the profile (F044-AC-11): coverage under MinCoverage → insufficient_data; TT27: ≥ 80 % proficient and ≥ 40 %
// mastered → excellent, ≥ 60 % developing → completed, else not_completed; TT22: ≥ 60 % developing → pass, else not_pass.
// Ratios use the objectives in scope as denominator.
func Map(profile string, r Rules, c Counts) Result {
	res := Result{Coverage: ratio(c.Covered, c.InScope), ProficientRatio: ratio(c.AtLeastProficient, c.InScope),
		MasteredRatio: ratio(c.Mastered, c.InScope)}
	developing := ratio(c.AtLeastDeveloping, c.InScope)
	switch {
	case c.InScope == 0 || res.Coverage < r.MinCoverage:
		res.Level = LevelInsufficientData
	case profile == ProfileLowerSecondaryTT22 && developing >= r.PassDevelopingRatio:
		res.Level = LevelPass
	case profile == ProfileLowerSecondaryTT22:
		res.Level = LevelNotPass
	case res.ProficientRatio >= r.ExcellentProficientRatio && res.MasteredRatio >= r.ExcellentMasteredRatio:
		res.Level = LevelExcellent
	case developing >= r.CompletedDevelopingRatio:
		res.Level = LevelCompleted
	default:
		res.Level = LevelNotCompleted
	}
	return res
}

// ProfileFor returns the profile of a grade level (1–5 TT27, 6–9 TT22; "" otherwise).
func ProfileFor(grade int) string {
	switch {
	case grade >= 1 && grade <= 5:
		return ProfilePrimaryTT27
	case grade >= 6 && grade <= 9:
		return ProfileLowerSecondaryTT22
	}
	return ""
}

// ValidPeriod reports a period code.
func ValidPeriod(p string) bool {
	switch p {
	case PeriodMidTerm1, PeriodEndTerm1, PeriodMidTerm2, PeriodEndTerm2:
		return true
	}
	return false
}

// CurrentPeriod returns the period containing day and the start year of its academic year under the default calendar
// [Proposed until the school calendar of the LMS is available]: the year starts on 1 September; mid_term_1 until
// 15 November, end_term_1 until 15 January, mid_term_2 until 31 March, end_term_2 until 31 August.
func CurrentPeriod(day time.Time) (string, int) {
	y := day.Year()
	if day.Month() < time.September {
		y--
	}
	switch {
	case day.Before(time.Date(y, 11, 16, 0, 0, 0, 0, day.Location())):
		return PeriodMidTerm1, y
	case day.Before(time.Date(y+1, 1, 16, 0, 0, 0, 0, day.Location())):
		return PeriodEndTerm1, y
	case day.Before(time.Date(y+1, 4, 1, 0, 0, 0, 0, day.Location())):
		return PeriodMidTerm2, y
	}
	return PeriodEndTerm2, y
}

// PeriodRange returns the first and last day (UTC dates) whose official exams count for the period: the term so far.
func PeriodRange(year int, period string) (time.Time, time.Time) {
	d := func(y int, m time.Month, day int) time.Time { return time.Date(y, m, day, 0, 0, 0, 0, time.UTC) }
	switch period {
	case PeriodMidTerm1:
		return d(year, 9, 1), d(year, 11, 15)
	case PeriodEndTerm1:
		return d(year, 9, 1), d(year+1, 1, 15)
	case PeriodMidTerm2:
		return d(year+1, 1, 16), d(year+1, 3, 31)
	}
	return d(year+1, 1, 16), d(year+1, 8, 31)
}

// ScopeShare is the share of the objectives of the grade × subject in curriculum order that are in scope at the end of
// the period, when the school has no LMS schedule (ASSESSMENT_REGULATION_PROFILE §4) [Proposed].
func ScopeShare(period string) float64 {
	switch period {
	case PeriodMidTerm1:
		return 0.25
	case PeriodEndTerm1:
		return 0.5
	case PeriodMidTerm2:
		return 0.75
	}
	return 1
}
