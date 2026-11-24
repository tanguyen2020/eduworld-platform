// Package challenge holds the rules of the grade challenges and school tournaments created by a school from a platform
// template (F107 US1, COMPETITION_SYSTEM §10.2, ADR-0027, ADR-0095): the template payload, the parameters a tenant may
// fill (allowed_params only), the audience in one school (grade ± students − excluded, exclude wins), the schedule
// window of the template and the reward rules. Pure Go, no I/O.
package challenge

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/tournament"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/tournament/bracket"
)

// Kinds of a challenge (ADR-0095: class_challenge and class_tournament are gone).
const (
	KindGradeChallenge   = "grade_challenge"
	KindSchoolTournament = "school_tournament"
)

// Formats of a challenge.
const (
	FormatTimedQuiz  = "timed_quiz"
	FormatTournament = "tournament"
)

// Config types of the templates (competition.game_configs.config_type) and their resource types of the assignments.
const (
	ConfigChallengeTemplates   = "challenge-templates"
	ConfigTournamentTemplates  = "tournament-templates"
	ResourceChallengeTemplate  = "challenge_template"
	ResourceTournamentTemplate = "tournament_template"
)

// ResourceType returns the assignment resource type of a template config type ("" when it is not a template).
func ResourceType(configType string) string {
	switch configType {
	case ConfigChallengeTemplates:
		return ResourceChallengeTemplate
	case ConfigTournamentTemplates:
		return ResourceTournamentTemplate
	}
	return ""
}

// ConfigType returns the config type of an assignment resource type ("" when unknown).
func ConfigType(resourceType string) string {
	switch resourceType {
	case ResourceChallengeTemplate:
		return ConfigChallengeTemplates
	case ResourceTournamentTemplate:
		return ConfigTournamentTemplates
	}
	return ""
}

// ValidationError is a rejected request field (422 VALIDATION_FAILED naming the field).
type ValidationError struct {
	Field  string
	Reason string
}

func (e ValidationError) Error() string { return e.Field + ": " + e.Reason }

func invalid(field, format string, args ...any) error {
	return ValidationError{Field: field, Reason: fmt.Sprintf(format, args...)}
}

// ErrTemplate: the published payload of a template is not a usable template (platform data error).
var ErrTemplate = errors.New("challenge: invalid template payload")

// Known parameters a template may allow (allowed_params ⊆ Params). Rules, scoring, rewards and chat are never parameters
// (R15, R19, ADR-0020, ADR-0027).
const (
	ParamSubjectID           = "subject_id"
	ParamTopicIDs            = "topic_ids"
	ParamMaxEntries          = "max_entries"
	ParamSeedingPolicy       = "seeding_policy"
	ParamRegistrationOpenAt  = "registration_open_at"
	ParamRegistrationCloseAt = "registration_close_at"
	ParamAutoConfirm         = "auto_confirm"
)

// Params are the parameters a template may let a tenant fill.
var Params = []string{ParamSubjectID, ParamTopicIDs, ParamMaxEntries, ParamSeedingPolicy, ParamRegistrationOpenAt,
	ParamRegistrationCloseAt, ParamAutoConfirm}

// Forbidden reports a key that can never be a tenant parameter: anything about chat (always on, F107-AC-28), rewards,
// points or scoring (template only, F107-AC-02).
func Forbidden(key string) bool {
	k := strings.ToLower(key)
	for _, w := range []string{"chat", "reward", "point", "score", "rule", "prize"} {
		if strings.Contains(k, w) {
			return true
		}
	}
	return false
}

// TournamentSpec is the tournament part of a template.
type TournamentSpec struct {
	Format          string `json:"format"`
	ParticipantType string `json:"participant_type"`
	TeamSize        *int   `json:"team_size"`
	MaxEntries      int    `json:"max_entries"`
	MinEntries      int    `json:"min_entries"`
	SeedingPolicy   string `json:"seeding_policy"`
	AutoConfirm     bool   `json:"auto_confirm"`
	RoundGapMinutes int    `json:"round_gap_minutes"`
	BracketReset    bool   `json:"bracket_reset"`
}

// TimedQuizSpec is the timed-quiz part of a template.
type TimedQuizSpec struct {
	JoinBeforeMinutes int `json:"join_before_minutes"`
	DurationMinutes   int `json:"duration_minutes"`
}

// Schedule is the window of the template: the earliest start after creation, the latest start and the longest duration.
type Schedule struct {
	MinLeadMinutes     int `json:"min_lead_minutes"`
	MaxLeadDays        int `json:"max_lead_days"`
	MaxDurationMinutes int `json:"max_duration_minutes"`
}

// TimersSpec is tournament_timers of the template (minutes); a missing value takes the platform default.
type TimersSpec struct {
	CheckInMinutes    int  `json:"check_in_minutes"`
	NoShowMinutes     int  `json:"no_show_minutes"`
	ComplaintMinutes  int  `json:"complaint_minutes"`
	RescheduleMinutes int  `json:"reschedule_minutes"`
	RescheduleMax     *int `json:"reschedule_max"`
}

// RewardRule is one reward rule of the template (COMPETITION_SYSTEM §10.8): a rank range or a minimum of completed
// matches, Competition Points (source_type tournament) and a cosmetic/badge/title reference for module game.
type RewardRule struct {
	RuleID     string            `json:"rule_id"`
	RankMin    *int              `json:"rank_min,omitempty"`
	RankMax    *int              `json:"rank_max,omitempty"`
	MinMatches *int              `json:"min_matches,omitempty"`
	Points     int               `json:"points"`
	RewardRef  map[string]string `json:"reward_ref"`
}

// Applies reports whether the rule rewards an entry of rank having completed played matches.
func (r RewardRule) Applies(rank, played int) bool {
	if r.RankMin != nil && rank < *r.RankMin {
		return false
	}
	if r.RankMax != nil && rank > *r.RankMax {
		return false
	}
	if r.MinMatches != nil && played < *r.MinMatches {
		return false
	}
	return r.RankMin != nil || r.RankMax != nil || r.MinMatches != nil
}

// Template is a published template version.
type Template struct {
	SchemaVersion  int             `json:"schema_version"`
	Kind           string          `json:"kind"`
	Format         string          `json:"format"`
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	SubjectID      *uuid.UUID      `json:"subject_id,omitempty"`
	GradeLevels    []int           `json:"grade_levels"`
	Mode           string          `json:"mode"`
	Tournament     *TournamentSpec `json:"tournament,omitempty"`
	TimedQuiz      *TimedQuizSpec  `json:"timed_quiz,omitempty"`
	Schedule       Schedule        `json:"schedule"`
	Timers         *TimersSpec     `json:"timers,omitempty"`
	AllowedParams  []string        `json:"allowed_params"`
	RewardRuleRefs []RewardRule    `json:"reward_rule_refs"`
	RewardSummary  string          `json:"reward_summary"`
}

// ParseTemplate decodes and checks a published template payload: a known kind and format (class kinds refused,
// ADR-0095), a tournament part with a known format, participant type, seeding policy and entry bounds, a timed-quiz
// part for a timed quiz, grade levels in 1–9, allowed params known and never chat/rewards/scoring (F107-AC-28).
func ParseTemplate(payload []byte) (Template, error) {
	var t Template
	if err := json.Unmarshal(payload, &t); err != nil {
		return t, fmt.Errorf("%w: %w", ErrTemplate, err)
	}
	bad := func(why string) (Template, error) { return t, fmt.Errorf("%w: %s", ErrTemplate, why) }
	if err := CheckKind(t.Kind); err != nil {
		return bad("kind")
	}
	if len(t.GradeLevels) == 0 || len(t.GradeLevels) > 9 {
		return bad("grade_levels")
	}
	for _, g := range t.GradeLevels {
		if g < 1 || g > 9 {
			return bad("grade_levels")
		}
	}
	for _, p := range t.AllowedParams {
		if Forbidden(p) || !slices.Contains(Params, p) {
			return bad("allowed_params " + p)
		}
	}
	if t.Schedule.MaxDurationMinutes <= 0 || t.Schedule.MaxLeadDays <= 0 || t.Schedule.MinLeadMinutes < 0 {
		return bad("schedule")
	}
	switch t.Format {
	case FormatTimedQuiz:
		if t.Kind != KindGradeChallenge || t.TimedQuiz == nil || t.TimedQuiz.DurationMinutes <= 0 || t.TimedQuiz.JoinBeforeMinutes < 0 ||
			t.Mode != "timed_quiz" {
			return bad("timed_quiz")
		}
	case FormatTournament:
		s := t.Tournament
		if s == nil || !slices.Contains(tournament.Formats, s.Format) || !bracket.ValidPolicy(s.SeedingPolicy) ||
			s.MaxEntries < 2 || s.MaxEntries > 256 || s.RoundGapMinutes < 1 {
			return bad("tournament")
		}
		if s.MinEntries < tournament.MinEntries(s.Format) {
			s.MinEntries = tournament.MinEntries(s.Format)
		}
		if s.MinEntries > s.MaxEntries {
			return bad("tournament entries")
		}
		switch s.ParticipantType {
		case tournament.ParticipantIndividual:
			if s.TeamSize != nil || t.Mode != "battle_1v1" {
				return bad("participant")
			}
		case tournament.ParticipantTeam:
			if s.TeamSize == nil || *s.TeamSize < 2 || *s.TeamSize > 8 || t.Mode != "team_battle" {
				return bad("participant")
			}
		default:
			return bad("participant")
		}
	default:
		return bad("format")
	}
	seen := map[string]bool{}
	for _, r := range t.RewardRuleRefs {
		if r.RuleID == "" || len(r.RuleID) > 64 || seen[r.RuleID] || r.Points < 0 || r.Points > 1000 || len(r.RewardRef) > 4 {
			return bad("reward_rule_refs")
		}
		seen[r.RuleID] = true
	}
	return t, nil
}

// CheckKind refuses the class kinds and anything unknown (F107-AC-01: 422 VALIDATION_FAILED).
func CheckKind(kind string) error {
	if kind != KindGradeChallenge && kind != KindSchoolTournament {
		return invalid("kind", "only grade_challenge and school_tournament exist (no class challenge, ADR-0095)")
	}
	return nil
}

// TimersOr returns the tournament timers of the template, completed with the platform defaults.
func (t Template) TimersOr(def tournament.Timers) tournament.Timers {
	out := def
	if s := t.Timers; s != nil {
		if s.CheckInMinutes > 0 {
			out.CheckIn = time.Duration(s.CheckInMinutes) * time.Minute
		}
		if s.NoShowMinutes > 0 {
			out.NoShow = time.Duration(s.NoShowMinutes) * time.Minute
		}
		if s.ComplaintMinutes > 0 {
			out.Complaint = time.Duration(s.ComplaintMinutes) * time.Minute
		}
		if s.RescheduleMinutes > 0 {
			out.Reschedule = time.Duration(s.RescheduleMinutes) * time.Minute
		}
		if s.RescheduleMax != nil && *s.RescheduleMax >= 0 && *s.RescheduleMax <= 2 {
			out.RescheduleMax = *s.RescheduleMax
		}
	}
	return out
}
