package liveops

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrParamNotAllowed is a school event parameter outside the template (422 EVENT_TEMPLATE_PARAM_NOT_ALLOWED).
var ErrParamNotAllowed = errors.New("liveops: parameter not allowed by the template")

// ParamError names the parameter.
type ParamError struct{ Param, Reason string }

func (e *ParamError) Error() string { return e.Param + ": " + e.Reason }
func (e *ParamError) Unwrap() error { return ErrParamNotAllowed }

func notAllowed(param, format string, args ...any) error {
	return &ParamError{Param: param, Reason: fmt.Sprintf(format, args...)}
}

// Template parameters a template may allow (event-template.schema.json allowed_params). name, starts_at, ends_at and the
// audience are always filled; what the audience may hold is the caller's scope (school or tenant).
const (
	ParamDescription   = "description"
	ParamSchools       = "audience.school_ids"
	ParamGrades        = "audience.grade_levels"
	ParamGoalTarget    = "community_goal_target"
	ParamQuests        = "quest_ids"
	ParamChallenges    = "challenge_ids"
	ParamRewardOption  = "reward_option_id"
	TemplateMaxDaysCap = 28
)

// TemplateParams is the catalog of allowed_params.
var TemplateParams = []string{ParamDescription, ParamSchools, ParamGrades, ParamGoalTarget, ParamQuests, ParamChallenges, ParamRewardOption}

// TemplateKinds are the event kinds of a school template (event.schema.json tenant events).
var TemplateKinds = []string{KindSchool, KindCommunityGoal, KindLearningCampaign}

// RewardOption is one reward set a school may choose (granted at the last milestone of the goal).
type RewardOption struct {
	RewardOptionID string    `json:"reward_option_id"`
	Label          Localized `json:"label"`
	Rewards        []Reward  `json:"rewards"`
}

// TemplateGoal is the community goal of a template (the school sets the target only).
type TemplateGoal struct {
	GoalCode               string          `json:"goal_code"`
	Metric                 string          `json:"metric"`
	Scope                  string          `json:"scope"`
	DefaultTarget          int64           `json:"default_target"`
	PerStudentDailyCap     int             `json:"per_student_daily_cap,omitempty"`
	ShowGradeContributions *bool           `json:"show_grade_contributions,omitempty"`
	Milestones             []GoalMilestone `json:"milestones"`
}

// Template is one event template version (event-template.schema.json without the server fields).
type Template struct {
	TemplateID       uuid.UUID      `json:"template_id"`
	Code             string         `json:"code"`
	Kind             string         `json:"event_kind"`
	Name             Localized      `json:"name"`
	Description      Localized      `json:"description,omitempty"`
	AllowedParams    []string       `json:"allowed_params"`
	MaxDurationDays  int            `json:"max_duration_days"`
	QuestOptions     []uuid.UUID    `json:"quest_options,omitempty"`
	ChallengeOptions []uuid.UUID    `json:"challenge_options,omitempty"`
	RewardOptions    []RewardOption `json:"reward_options,omitempty"`
	EventPoints      *Points        `json:"event_points,omitempty"`
	MilestoneRewards []Milestone    `json:"milestone_rewards,omitempty"`
	CommunityGoal    TemplateGoal   `json:"community_goal"`
}

// ParseTemplate reads a template version payload.
func ParseTemplate(raw []byte) (Template, error) {
	var t Template
	if err := json.Unmarshal(raw, &t); err != nil {
		return t, invalid("payload", "%v", err)
	}
	return t, nil
}

// Validate checks a template (platform content: bilingual, known parameters, a school or tenant goal, rewards of the
// event reward types).
func (t Template) Validate() error {
	if !slices.Contains(TemplateKinds, t.Kind) {
		return invalid("event_kind", "a template is for school, community_goal or learning_campaign events")
	}
	if strings.TrimSpace(t.Name["en"]) == "" {
		return &TranslationError{Field: "name"}
	}
	if t.Description != nil && strings.TrimSpace(t.Description["en"]) == "" {
		return &TranslationError{Field: "description"}
	}
	for i, p := range t.AllowedParams {
		if !slices.Contains(TemplateParams, p) {
			return invalid(fmt.Sprintf("allowed_params/%d", i), "unknown parameter")
		}
	}
	if t.MaxDurationDays < 1 || t.MaxDurationDays > TemplateMaxDaysCap {
		return invalid("max_duration_days", "1..%d", TemplateMaxDaysCap)
	}
	seen := map[string]bool{}
	for i, o := range t.RewardOptions {
		f := fmt.Sprintf("reward_options/%d", i)
		if o.RewardOptionID == "" || seen[o.RewardOptionID] {
			return invalid(f+"/reward_option_id", "unique id required")
		}
		seen[o.RewardOptionID] = true
		if strings.TrimSpace(o.Label["en"]) == "" {
			return &TranslationError{Field: f + "/label"}
		}
		if err := ValidRewards(f+"/rewards", o.Rewards, EventRewardTypes); err != nil {
			return err
		}
	}
	g := Goal{GoalCode: t.CommunityGoal.GoalCode, Metric: t.CommunityGoal.Metric, Scope: t.CommunityGoal.Scope, Target: t.CommunityGoal.DefaultTarget,
		Milestones: t.CommunityGoal.Milestones}
	if err := g.validate(OwnerTenant); err != nil {
		return err
	}
	if t.EventPoints != nil && t.EventPoints.Enabled {
		if err := validSources("event_points.sources", t.EventPoints.Sources); err != nil {
			return err
		}
	}
	for i, m := range t.MilestoneRewards {
		if err := ValidRewards(fmt.Sprintf("milestone_rewards/%d/rewards", i), m.Rewards, EventRewardTypes); err != nil {
			return err
		}
	}
	return nil
}

// Allows reports a parameter of allowed_params.
func (t Template) Allows(p string) bool { return slices.Contains(t.AllowedParams, p) }

// SchoolParams are the parameters a school fills (SchoolEventCreate/Update); Set names the optional parameters present
// in the request.
type SchoolParams struct {
	Name         Localized
	Description  Localized
	StartsAt     time.Time
	EndsAt       time.Time
	SchoolIDs    []uuid.UUID
	GradeLevels  []int
	GoalTarget   *int64
	QuestIDs     []uuid.UUID
	ChallengeIDs []uuid.UUID
	RewardOption *string
	Set          map[string]bool
}

// Check checks the parameters against the template (F077-AC-22): every optional parameter present is allowed, the
// quests, challenges and reward option are among the template's options, the duration is within the template and the
// platform limit (school_event_max_duration_days).
func (t Template) Check(p SchoolParams, maxDays int) error {
	for _, k := range TemplateParams {
		if p.Set[k] && !t.Allows(k) {
			return notAllowed(k, "not allowed by the template")
		}
	}
	for _, q := range p.QuestIDs {
		if !slices.Contains(t.QuestOptions, q) {
			return notAllowed(ParamQuests, "a quest outside the template")
		}
	}
	for _, c := range p.ChallengeIDs {
		if !slices.Contains(t.ChallengeOptions, c) {
			return notAllowed(ParamChallenges, "a challenge outside the template")
		}
	}
	if p.RewardOption != nil && t.RewardOption(*p.RewardOption) == nil {
		return notAllowed(ParamRewardOption, "a reward outside the template")
	}
	if !p.EndsAt.After(p.StartsAt) {
		return invalid("ends_at", "must be after starts_at")
	}
	limit := min(t.MaxDurationDays, maxDays)
	if p.EndsAt.Sub(p.StartsAt) > time.Duration(limit)*24*time.Hour {
		return notAllowed("ends_at", "longer than %d days", limit)
	}
	if strings.TrimSpace(p.Name["vi"]) == "" {
		return invalid("name.vi", "required")
	}
	for _, g := range p.GradeLevels {
		if g < 1 || g > 9 {
			return invalid("audience.grade_levels", "grades 1..9")
		}
	}
	return nil
}

// RewardOption returns the option of id (nil when none).
func (t Template) RewardOption(id string) *RewardOption {
	for i := range t.RewardOptions {
		if t.RewardOptions[i].RewardOptionID == id {
			return &t.RewardOptions[i]
		}
	}
	return nil
}

// Build returns the event version of a school event from the template and the parameters (owner tenant, template_ref,
// the template's kind, goal and points; the school's text, schedule, audience, target, components and reward option).
func (t Template) Build(eventID, tenant uuid.UUID, code string, templateVersion int, p SchoolParams, grades []int) Definition {
	scope := ScopeTenant
	if len(p.SchoolIDs) > 0 {
		scope = ScopeSchool
	}
	if len(p.GradeLevels) > 0 {
		grades = p.GradeLevels
	}
	target := t.CommunityGoal.DefaultTarget
	if p.GoalTarget != nil {
		target = *p.GoalTarget
	}
	ms := make([]GoalMilestone, 0, len(t.CommunityGoal.Milestones))
	for _, m := range t.CommunityGoal.Milestones {
		ms = append(ms, GoalMilestone{Percent: m.Percent, Rewards: append([]Reward{}, m.Rewards...), WorldChangeCode: m.WorldChangeCode})
	}
	if p.RewardOption != nil {
		if o := t.RewardOption(*p.RewardOption); o != nil && len(ms) > 0 {
			last := 0
			for i := range ms {
				if ms[i].Percent > ms[last].Percent {
					last = i
				}
			}
			ms[last].Rewards = append(ms[last].Rewards, o.Rewards...)
		}
	}
	show := t.CommunityGoal.ShowGradeContributions
	tn := tenant
	d := Definition{EventID: eventID, Code: code, OwnerScope: OwnerTenant, TenantID: &tn,
		TemplateRef: &TemplateRef{TemplateID: t.TemplateID, TemplateVersion: templateVersion}, Kind: t.Kind, Name: p.Name,
		Description: p.Description, Schedule: Schedule{StartAt: p.StartsAt.UTC(), EndAt: p.EndsAt.UTC(), Timezone: Timezone},
		Audience: Audience{Scope: scope, SchoolIDs: p.SchoolIDs, GradeLevels: grades},
		Components: Components{QuestIDs: p.QuestIDs, ChallengeIDs: p.ChallengeIDs, CommunityGoal: &Goal{GoalCode: t.CommunityGoal.GoalCode,
			Metric: t.CommunityGoal.Metric, Scope: t.CommunityGoal.Scope, Target: target, PerStudentDailyCap: t.CommunityGoal.PerStudentDailyCap,
			ShowGradeContributions: show, Milestones: ms}},
		EventPoints: t.EventPoints, MilestoneRewards: t.MilestoneRewards}
	if d.Description == nil {
		d.Description = Localized{"vi": p.Name["vi"]}
	}
	if d.Components.CommunityGoal.Scope == ScopeSchool && scope == ScopeTenant {
		d.Components.CommunityGoal.Scope = ScopeSchool // a tenant-wide school event still counts per school
	}
	return d
}
