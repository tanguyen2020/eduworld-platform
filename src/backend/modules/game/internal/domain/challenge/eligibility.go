package challenge

import (
	"math"
	"slices"
	"time"
)

// Rules are the platform parameters of the Challenge Engine (CONFIG_DEFAULTS: difficulty_levels,
// g1_3_challenge_rules, challenge_attempts_per_day, challenge_fail_cooldown, attempt_resume_window, deadline_grace,
// survival_life_regen_streak), read by infrastructure/config, never hard-coded here.
type Rules struct {
	DefaultLives   map[string]int // survival lives per level (difficulty_levels)
	DefaultSeconds map[string]int // seconds per item per level (difficulty_levels)
	AttemptsPerDay int            // challenge_attempts_per_day
	FailCooldown   time.Duration  // challenge_fail_cooldown
	CooldownFrom   string         // level from which the fail cooldown applies (hard)
	G13TimeFactor  float64        // g1_3_challenge_rules: time × 1.5
	G13MaxLevel    string         // g1_3_challenge_rules: easy–hard
	ResumeWindow   time.Duration  // attempt_resume_window of a challenge without a time limit
	DeadlineGrace  time.Duration  // deadline_grace of a graded result and of the timer
	RegenStreak    int            // survival: consecutive fully correct rounds that restore one life
}

func (r Rules) g13Max() string {
	if r.G13MaxLevel == "" {
		return Hard
	}
	return r.G13MaxLevel
}

// Allowed reports a profile the student of grade may choose: the grade is one of the profile (or of the challenge) and
// grades 1–3 play up to g1_3_challenge_rules.
func (r Rules) Allowed(v Version, p Profile, grade int) bool {
	grades := p.GradeLevels
	if len(grades) == 0 {
		grades = v.GradeLevels
	}
	if !slices.Contains(grades, grade) || !slices.Contains(v.GradeLevels, grade) {
		return false
	}
	return grade > 3 || Rank(p.Level) <= Rank(r.g13Max())
}

// Difficulties are the levels the student of grade may choose, easiest first (F076-AC-01).
func (r Rules) Difficulties(v Version, grade int) []string {
	out := []string{}
	for _, l := range Levels {
		if p, ok := v.Profile(l); ok && r.Allowed(v, p, grade) {
			out = append(out, l)
		}
	}
	return out
}

// LockedGrade is the lock reason of a level whose profile (or challenge) does not admit the grade of the student
// (ChallengeDifficultyOption.locked_reason_code). The contract also lists not_unlocked and school_disabled; F076 has no
// per-level unlock or school switch, so they are never returned [Confirmed — F076 Clarifications 2026-11-26].
const LockedGrade = "grade_not_allowed"

// MinItemSeconds is the floor of the time of one activity (survival: one item per activity, F076 Clarifications
// 2026-11-24 "tối thiểu 10 s").
const MinItemSeconds = 10

// Option is one level of a version for a student of grade (ChallengeDifficultyOption, delta F076).
type Option struct {
	Level            string
	Available        bool
	LockedReason     string // "" when available
	TimeLimitSeconds *int   // whole challenge × the grade 1–3 factor; nil = no time limit
	ItemSeconds      int    // seconds per activity × the grade 1–3 factor, at least MinItemSeconds
	Lives            *int   // survival only
	RewardMultiplier float64
}

// Options are the levels of a version for the student of grade, easiest first (getGameChallenge difficulties[]): the
// levels it may choose (Available, same as Difficulties) and the levels locked for its grade with their reason. Grades
// 1–3 never receive a level above g1_3_challenge_rules (expert, legendary), not even locked (AC-UX-0601).
func (r Rules) Options(v Version, grade int) []Option {
	out := []Option{}
	for _, l := range Levels {
		p, ok := v.Profile(l)
		if !ok || (grade >= 1 && grade <= 3 && Rank(l) > Rank(r.g13Max())) {
			continue
		}
		o := Option{Level: l, Available: r.Allowed(v, p, grade), ItemSeconds: max(MinItemSeconds, r.ItemSeconds(p, grade)), Lives: r.Lives(v, p),
			RewardMultiplier: p.RewardMultiplier}
		if !o.Available {
			o.LockedReason = LockedGrade
		}
		if v.TimeLimitSeconds > 0 {
			n := max(1, int(math.Floor(float64(v.TimeLimitSeconds)*r.g13Factor(grade))))
			o.TimeLimitSeconds = &n
		}
		out = append(out, o)
	}
	return out
}

func (r Rules) g13Factor(grade int) float64 {
	if grade >= 1 && grade <= 3 && r.G13TimeFactor > 0 {
		return r.G13TimeFactor
	}
	return 1
}

// Deadline is deadline_at on the server clock: time_limit_seconds × the grade 1–3 factor after start; nil = no time
// limit (the challenge continues within attempt_resume_window).
func (r Rules) Deadline(v Version, grade int, start time.Time) *time.Time {
	if v.TimeLimitSeconds <= 0 {
		return nil
	}
	d := start.Add(time.Duration(math.Floor(float64(v.TimeLimitSeconds)*r.g13Factor(grade))) * time.Second)
	return &d
}

// ItemSeconds is the time of one item of the level (seconds_per_item or difficulty_levels) × the grade 1–3 factor.
func (r Rules) ItemSeconds(p Profile, grade int) int {
	s := p.SecondsPerItem
	if s <= 0 {
		s = r.DefaultSeconds[p.Level]
	}
	return int(math.Floor(float64(s) * r.g13Factor(grade)))
}

// Lives are the survival lives of the level (profile or difficulty_levels); nil for the other types.
func (r Rules) Lives(v Version, p Profile) *int {
	if v.Type != TypeSurvival {
		return nil
	}
	n := p.Lives
	if n <= 0 {
		n = r.DefaultLives[p.Level]
	}
	if n <= 0 {
		n = 1
	}
	return &n
}

// MaxAttempts is the attempts per day (per period for weekly and seasonal) of a version.
func (r Rules) MaxAttempts(v Version) int {
	if v.Retry != nil && v.Retry.MaxAttemptsPerDay > 0 {
		return v.Retry.MaxAttemptsPerDay
	}
	return r.AttemptsPerDay
}

// Cooldown is the wait after a failed attempt at level: the version's cooldown_seconds_after_fail, else
// challenge_fail_cooldown from CooldownFrom (hard) up, else none.
func (r Rules) Cooldown(v Version, level string) time.Duration {
	if v.Retry != nil && v.Retry.CooldownSecondsAfterFail != nil {
		return time.Duration(*v.Retry.CooldownSecondsAfterFail) * time.Second
	}
	from := r.CooldownFrom
	if from == "" {
		from = Hard
	}
	if Rank(level) >= Rank(from) {
		return r.FailCooldown
	}
	return 0
}

// Student is what eligibility reads of a student: grade (projection) and the server facts of unlock_rule.
type Student struct {
	Grade           int
	Level           int
	CompletedQuests map[string]bool
	OpenAreas       map[string]bool
}

// Reasons a challenge is not available to a student (CHALLENGE_NOT_ELIGIBLE detail; 404 on the read routes).
const (
	ReasonGrade      = "grade"
	ReasonLocked     = "locked"
	ReasonWindow     = "availability"
	ReasonDifficulty = "difficulty"
	ReasonParty      = "party"
)

// Visible reports a version available to the student at now (F076-AC-01, AC-02): its grades hold the grade of the
// student (never a class, ADR-0095), the unlock rule holds on server facts and now is inside the window.
func (v Version) Visible(st Student, now time.Time) (bool, string) {
	if !slices.Contains(v.GradeLevels, st.Grade) {
		return false, ReasonGrade
	}
	if from, to := v.Window(); (from != nil && now.Before(*from)) || (to != nil && !now.Before(*to)) {
		return false, ReasonWindow
	}
	if u := v.Unlock; u != nil {
		if st.Level < u.MinLevel {
			return false, ReasonLocked
		}
		for _, q := range u.CompletedQuestIDs {
			if !st.CompletedQuests[q] {
				return false, ReasonLocked
			}
		}
		for _, a := range u.UnlockedAreaIDs {
			if !st.OpenAreas[a] {
				return false, ReasonLocked
			}
		}
	}
	return true, ""
}
