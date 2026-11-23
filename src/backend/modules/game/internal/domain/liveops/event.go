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

// ErrInvalid is an event, template or track version that cannot be published or created (422 VALIDATION_FAILED naming
// the field); ErrTranslationIncomplete is a platform event missing its English text (422 TRANSLATION_INCOMPLETE).
var (
	ErrInvalid               = errors.New("liveops: invalid")
	ErrTranslationIncomplete = errors.New("liveops: translation incomplete")
)

// FieldError names the invalid field (never a value).
type FieldError struct {
	Field, Reason string
}

func (e *FieldError) Error() string { return e.Field + ": " + e.Reason }
func (e *FieldError) Unwrap() error { return ErrInvalid }

func invalid(field, format string, args ...any) error {
	return &FieldError{Field: field, Reason: fmt.Sprintf(format, args...)}
}

// TranslationError names the text that has no English version.
type TranslationError struct{ Field string }

func (e *TranslationError) Error() string { return e.Field + ": English text required" }
func (e *TranslationError) Unwrap() error { return ErrTranslationIncomplete }

// Event kinds (EVENT_SEASON_SYSTEM §3).
const (
	KindWorldFestival    = "world_festival"
	KindWeekend          = "weekend"
	KindSchool           = "school"
	KindCommunityGoal    = "community_goal"
	KindLearningCampaign = "learning_campaign"
	KindSpecialBoss      = "special_boss"
	KindLimitedQuest     = "limited_quest"
)

// Owner scopes and audience scopes.
const (
	OwnerPlatform = "platform"
	OwnerTenant   = "tenant"
	ScopePlatform = "platform"
	ScopeTenant   = "tenant"
	ScopeSchool   = "school"
)

// Timezone is the only schedule timezone (event.schema.json schedule.timezone).
const Timezone = "Asia/Ho_Chi_Minh"

// ICT is Asia/Ho_Chi_Minh (fixed +07:00: no daylight saving) — the day of the daily caps and of the rule change.
var ICT = time.FixedZone("ICT", 7*3600)

// Day is the ICT calendar day of t (daily caps of event points, contributions and season points).
func Day(t time.Time) time.Time {
	y, m, d := t.In(ICT).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// Localized is a text by language (vi required; en required for platform content).
type Localized map[string]string

// Pick returns the text in lang ("en" falls back to vi when missing).
func (l Localized) Pick(lang string) string {
	if lang == "en" && l["en"] != "" {
		return l["en"]
	}
	return l["vi"]
}

// Reward is one reward of an event, a milestone, a goal milestone or a season tier (event.schema.json $defs/reward).
type Reward = map[string]any

// Schedule is the schedule of a version.
type Schedule struct {
	StartAt         time.Time `json:"start_at"`
	EndAt           time.Time `json:"end_at"`
	Timezone        string    `json:"timezone"`
	WeekdayWindows  []string  `json:"weekday_windows,omitempty"`
	ClaimGraceHours *int      `json:"claim_grace_hours,omitempty"`
}

// Eligibility is the participation condition (never a lock on learning content).
type Eligibility struct {
	CompletedChapterCodes []string `json:"completed_chapter_codes,omitempty"`
	MinLevel              *int     `json:"min_level,omitempty"`
}

// Decoration is the world decoration of an event (ADR-0057: one bundle with a reduced Low preset variant).
type Decoration struct {
	EnvironmentPresetID string   `json:"environment_preset_id,omitempty"`
	BundleID            string   `json:"bundle_id,omitempty"`
	BundleSizeBytes     int64    `json:"bundle_size_bytes,omitempty"`
	LowPresetBundleID   string   `json:"low_preset_bundle_id,omitempty"`
	Areas               []string `json:"areas"`
}

// MetricFilter narrows the contributions of a goal to subjects or objectives.
type MetricFilter struct {
	SubjectIDs   []uuid.UUID `json:"subject_ids,omitempty"`
	ObjectiveIDs []uuid.UUID `json:"objective_ids,omitempty"`
}

// GoalMilestone is a milestone of a community goal (percent of the target).
type GoalMilestone struct {
	Percent         int      `json:"percent"`
	Rewards         []Reward `json:"rewards"`
	WorldChangeCode string   `json:"world_change_code,omitempty"`
}

// Goal is the community goal of an event.
type Goal struct {
	GoalCode               string          `json:"goal_code"`
	Name                   Localized       `json:"name,omitempty"`
	Metric                 string          `json:"metric"`
	MetricFilter           *MetricFilter   `json:"metric_filter,omitempty"`
	Scope                  string          `json:"scope"`
	Target                 int64           `json:"target"`
	PerStudentDailyCap     int             `json:"per_student_daily_cap,omitempty"`
	ShowGradeContributions *bool           `json:"show_grade_contributions,omitempty"`
	Milestones             []GoalMilestone `json:"milestones"`
}

// ShowGrades reports show_grade_contributions (default true).
func (g Goal) ShowGrades() bool { return g.ShowGradeContributions == nil || *g.ShowGradeContributions }

// Percents returns the milestone percents in ascending order.
func (g Goal) Percents() []int {
	out := make([]int, 0, len(g.Milestones))
	for _, m := range g.Milestones {
		out = append(out, m.Percent)
	}
	slices.Sort(out)
	return out
}

// Milestone returns the milestone of percent (nil when none).
func (g Goal) Milestone(percent int) *GoalMilestone {
	for i := range g.Milestones {
		if g.Milestones[i].Percent == percent {
			return &g.Milestones[i]
		}
	}
	return nil
}

// Components are the content an event adds to the world (references to published content).
type Components struct {
	QuestIDs      []uuid.UUID `json:"quest_ids,omitempty"`
	ChallengeIDs  []uuid.UUID `json:"challenge_ids,omitempty"`
	BossIDs       []uuid.UUID `json:"boss_ids,omitempty"`
	ShopItemIDs   []uuid.UUID `json:"shop_item_ids,omitempty"`
	CollectionID  *uuid.UUID  `json:"collection_id,omitempty"`
	CommunityGoal *Goal       `json:"community_goal,omitempty"`
	Decoration    *Decoration `json:"decoration,omitempty"`
}

// Source is the points of one source code.
type Source struct {
	SourceCode string `json:"source_code"`
	Points     int    `json:"points"`
}

// Points are the event points of an event (or the season points of a track).
type Points struct {
	Enabled  bool     `json:"enabled"`
	DailyCap int      `json:"daily_cap,omitempty"`
	Sources  []Source `json:"sources,omitempty"`
}

// Milestone is a reward milestone of event points.
type Milestone struct {
	MilestoneID string   `json:"milestone_id"`
	EventPoints int      `json:"event_points"`
	Rewards     []Reward `json:"rewards"`
}

// TemplateRef is the template a tenant event was made from.
type TemplateRef struct {
	TemplateID      uuid.UUID `json:"template_id"`
	TemplateVersion int       `json:"template_version"`
}

// Definition is one event version (event.schema.json without the server fields).
type Definition struct {
	EventID              uuid.UUID    `json:"event_id"`
	Code                 string       `json:"code"`
	OwnerScope           string       `json:"owner_scope"`
	TenantID             *uuid.UUID   `json:"tenant_id,omitempty"`
	TemplateRef          *TemplateRef `json:"template_ref,omitempty"`
	Kind                 string       `json:"event_kind"`
	Name                 Localized    `json:"name"`
	Description          Localized    `json:"description"`
	ShortTagline         Localized    `json:"short_tagline,omitempty"`
	ThemeCode            string       `json:"theme_code,omitempty"`
	BannerAssetID        *uuid.UUID   `json:"banner_asset_id,omitempty"`
	Schedule             Schedule     `json:"schedule"`
	Audience             Audience     `json:"audience"`
	Eligibility          *Eligibility `json:"eligibility,omitempty"`
	Components           Components   `json:"components"`
	EventPoints          *Points      `json:"event_points,omitempty"`
	ParticipationRewards []Reward     `json:"participation_rewards,omitempty"`
	MilestoneRewards     []Milestone  `json:"milestone_rewards,omitempty"`
}

// Parse reads an event version payload.
func Parse(raw []byte) (Definition, error) {
	var d Definition
	if err := json.Unmarshal(raw, &d); err != nil {
		return d, invalid("payload", "%v", err)
	}
	return d, nil
}

// PointsEnabled reports event points on.
func (d Definition) PointsEnabled() bool { return d.EventPoints != nil && d.EventPoints.Enabled }

// Milestone returns the milestone of id (nil when none).
func (d Definition) Milestone(id string) *Milestone {
	for i := range d.MilestoneRewards {
		if d.MilestoneRewards[i].MilestoneID == id {
			return &d.MilestoneRewards[i]
		}
	}
	return nil
}

// ClaimUntil is end_at + claim_grace_hours (default event_claim_grace_hours).
func (d Definition) ClaimUntil(defaultGrace time.Duration) time.Time {
	if d.Schedule.ClaimGraceHours != nil {
		return d.Schedule.EndAt.Add(time.Duration(*d.Schedule.ClaimGraceHours) * time.Hour)
	}
	return d.Schedule.EndAt.Add(defaultGrace)
}

// Policy holds the platform parameters of the checks (CONFIG_DEFAULTS, EVENT_SEASON_SYSTEM tham số).
type Policy struct {
	FestivalMinDays     int   // event_festival_min_duration_days
	ExtraDailyQuestsMax int   // event_extra_daily_quests_max
	PointsDailyCap      int   // event_points_daily_cap (event_points.daily_cap when absent)
	BundleMaxBytes      int64 // event_decoration_bundle_max_mb
	GoalDailyCap        int   // community_goal_contribution_daily_cap (per_student_daily_cap when absent)
}

// EventRewardTypes are the reward types of an event (EVENT_SEASON_SYSTEM §4: cosmetic, Xu, badge, title, collection
// item — never XP, which only reward rules give).
var EventRewardTypes = []string{"currency", "item", "badge", "title", "collection_item"}

// Metrics of a community goal.
var Metrics = []string{"correct_answers", "activities_completed", "quests_completed", "discoveries_found", "boss_phases_cleared",
	"coop_sessions"}

// CheckTranslations is the bilingual rule of a platform event (ADR-0056, F077-AC-26): name, description, short tagline
// and the community goal name need an English text. Tenant events need Vietnamese only.
func (d Definition) CheckTranslations() error {
	if d.OwnerScope != OwnerPlatform {
		return nil
	}
	check := func(field string, l Localized, required bool) error {
		if l == nil && !required {
			return nil
		}
		if strings.TrimSpace(l["en"]) == "" {
			return &TranslationError{Field: field}
		}
		return nil
	}
	if err := check("name", d.Name, true); err != nil {
		return err
	}
	if err := check("description", d.Description, true); err != nil {
		return err
	}
	if err := check("short_tagline", d.ShortTagline, false); err != nil {
		return err
	}
	if g := d.Components.CommunityGoal; g != nil {
		return check("components.community_goal.name", g.Name, false)
	}
	return nil
}

// Validate checks a version against the healthy rules (EVENT_SEASON_SYSTEM §9, F077-AC-27), the decoration budget
// (§12, F077-AC-28) and the reward and goal rules. extraDaily is the number of daily-recurring quests of the version
// (resolved by the caller from the published quests).
func (d Definition) Validate(p Policy, extraDaily int) error {
	if strings.TrimSpace(d.Name["vi"]) == "" {
		return invalid("name.vi", "required")
	}
	if !d.Schedule.EndAt.After(d.Schedule.StartAt) {
		return invalid("schedule.end_at", "must be after start_at")
	}
	if d.Schedule.Timezone != "" && d.Schedule.Timezone != Timezone {
		return invalid("schedule.timezone", "must be %s", Timezone)
	}
	if d.Kind == KindWorldFestival && d.Schedule.EndAt.Sub(d.Schedule.StartAt) < time.Duration(p.FestivalMinDays)*24*time.Hour {
		return invalid("schedule.end_at", "a world festival lasts at least %d days (never forces playing online)", p.FestivalMinDays)
	}
	if d.Kind == KindLearningCampaign && d.Eligibility != nil && d.Eligibility.MinLevel != nil {
		return invalid("eligibility.min_level", "a learning event never requires a level")
	}
	if p.ExtraDailyQuestsMax >= 0 && extraDaily > p.ExtraDailyQuestsMax {
		return invalid("components.quest_ids", "at most %d extra quests per day", p.ExtraDailyQuestsMax)
	}
	if err := d.Audience.Validate(d.OwnerScope); err != nil {
		return err
	}
	if d.EventPoints != nil && d.EventPoints.Enabled {
		if err := validSources("event_points.sources", d.EventPoints.Sources); err != nil {
			return err
		}
	}
	if err := ValidRewards("participation_rewards", d.ParticipationRewards, EventRewardTypes); err != nil {
		return err
	}
	seen := map[string]bool{}
	for i, m := range d.MilestoneRewards {
		f := fmt.Sprintf("milestone_rewards/%d", i)
		if m.MilestoneID == "" || seen[m.MilestoneID] {
			return invalid(f+"/milestone_id", "unique milestone id required")
		}
		seen[m.MilestoneID] = true
		if !d.PointsEnabled() {
			return invalid(f, "milestones need event points")
		}
		if err := ValidRewards(f+"/rewards", m.Rewards, EventRewardTypes); err != nil {
			return err
		}
	}
	if g := d.Components.CommunityGoal; g != nil {
		if err := g.validate(d.OwnerScope); err != nil {
			return err
		}
	}
	if dec := d.Components.Decoration; dec != nil {
		if err := dec.validate(p.BundleMaxBytes); err != nil {
			return err
		}
	}
	return nil
}

func (g Goal) validate(owner string) error {
	if !slices.Contains(Metrics, g.Metric) {
		return invalid("components.community_goal.metric", "unknown metric")
	}
	switch g.Scope {
	case ScopeSchool, ScopeTenant:
	case ScopePlatform:
		if owner != OwnerPlatform {
			return invalid("components.community_goal.scope", "a tenant event has a school or tenant goal")
		}
	default:
		return invalid("components.community_goal.scope", "unknown scope")
	}
	if g.Target < 1 {
		return invalid("components.community_goal.target", "at least 1")
	}
	if len(g.Milestones) == 0 {
		return invalid("components.community_goal.milestones", "at least one milestone")
	}
	seen := map[int]bool{}
	for i, m := range g.Milestones {
		f := fmt.Sprintf("components.community_goal.milestones/%d", i)
		if m.Percent < 1 || m.Percent > 100 || seen[m.Percent] {
			return invalid(f+"/percent", "unique percent 1..100")
		}
		seen[m.Percent] = true
		if err := ValidRewards(f+"/rewards", m.Rewards, EventRewardTypes); err != nil {
			return err
		}
	}
	return nil
}

func (dec Decoration) validate(maxBytes int64) error {
	if dec.BundleID == "" {
		return nil
	}
	if dec.LowPresetBundleID == "" {
		return invalid("components.decoration.low_preset_bundle_id", "a decoration bundle needs its Low preset variant (ADR-0057)")
	}
	if dec.BundleSizeBytes < 1 {
		return invalid("components.decoration.bundle_size_bytes", "required with bundle_id")
	}
	if maxBytes > 0 && dec.BundleSizeBytes > maxBytes {
		return invalid("components.decoration.bundle_size_bytes", "over the decoration budget (%d MB)", maxBytes/(1024*1024))
	}
	return nil
}

// ValidRewards checks the type and the fields of rewards (types: the allowed types of the place; currency is Xu only).
func ValidRewards(field string, rs []Reward, types []string) error {
	for i, r := range rs {
		f := fmt.Sprintf("%s/%d", field, i)
		t, _ := r["type"].(string)
		if !slices.Contains(types, t) {
			return invalid(f+"/type", "reward type %q is not allowed here", t)
		}
		if t == "currency" {
			if c, _ := r["currency_code"].(string); c != "xu" {
				return invalid(f+"/currency_code", "only xu")
			}
			if Amount(r) < 1 {
				return invalid(f+"/amount", "at least 1")
			}
		}
	}
	return nil
}

// RewardItemCodes returns the item codes rewards name (item, badge, collection_item).
func RewardItemCodes(rs []Reward) []string {
	var out []string
	for _, r := range rs {
		for _, k := range []string{"item_id", "badge_id", "collection_item_id"} {
			if c, _ := r[k].(string); c != "" {
				out = append(out, c)
			}
		}
	}
	return out
}

// AllRewards returns every reward of a version (participation, milestones, goal milestones).
func (d Definition) AllRewards() []Reward {
	out := append([]Reward{}, d.ParticipationRewards...)
	for _, m := range d.MilestoneRewards {
		out = append(out, m.Rewards...)
	}
	if g := d.Components.CommunityGoal; g != nil {
		for _, m := range g.Milestones {
			out = append(out, m.Rewards...)
		}
	}
	return out
}

func validSources(field string, ss []Source) error {
	if len(ss) == 0 {
		return invalid(field, "at least one source")
	}
	seen := map[string]bool{}
	for i, s := range ss {
		if !slices.Contains(SourceCodes, s.SourceCode) || seen[s.SourceCode] {
			// login, check-in, streak or online-time sources are not in the catalog (no reward for showing up — R19).
			return invalid(fmt.Sprintf("%s/%d/source_code", field, i), "unknown or repeated source %q", s.SourceCode)
		}
		seen[s.SourceCode] = true
		if s.Points < 1 {
			return invalid(fmt.Sprintf("%s/%d/points", field, i), "at least 1")
		}
	}
	return nil
}

// RuleChangeEffective is when a new version of a running event applies (event_rule_change_effective: 04:00 ICT of the next
// day, F077-AC-11); a version of a run that is not live applies at once.
func RuleChangeEffective(publishedAt time.Time, live bool, hourICT int) time.Time {
	if !live {
		return publishedAt.UTC()
	}
	t := publishedAt.In(ICT)
	next := time.Date(t.Year(), t.Month(), t.Day()+1, hourICT, 0, 0, 0, ICT)
	return next.UTC()
}

// Amount returns the integer amount of a reward (JSON number or Go int).
func Amount(r Reward) int {
	switch n := r["amount"].(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	case json.Number:
		v, _ := n.Int64()
		return int(v)
	}
	return 0
}
