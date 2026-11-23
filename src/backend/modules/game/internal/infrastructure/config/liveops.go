package config

import (
	"errors"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/liveops"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// Defaults of CONFIG_DEFAULTS for live-ops (owner EVENT_SEASON_SYSTEM tham số mặc định). The season track parameters
// (season_track_tiers, season_track_points_per_tier, season_points_daily_cap, season_point_sources) live in the published
// season track version; season_unclaimed_rewards is auto_grant (schema const).
const (
	DefaultEventClaimGraceHours     = 72               // event_claim_grace_hours
	DefaultEventFestivalMinDays     = 7                // event_festival_min_duration_days
	DefaultEventExtraDailyQuestsMax = 2                // event_extra_daily_quests_max
	DefaultEventPointsDailyCap      = 200              // event_points_daily_cap
	DefaultEventUpcomingVisibleDays = 7                // event_upcoming_visible_days
	DefaultEventRuleChangeHour      = 4                // event_rule_change_effective (04:00 ICT the next day)
	DefaultGoalContributionDailyCap = 50               // community_goal_contribution_daily_cap
	DefaultGoalFlushSeconds         = 30               // community_goal_progress_flush_seconds
	DefaultSchoolEventMaxDays       = 28               // school_event_max_duration_days
	DefaultSchoolEventsConcurrent   = 3                // school_events_concurrent_per_tenant
	DefaultDecorationBundleMaxMB    = 20               // event_decoration_bundle_max_mb
	DefaultAnalyticsMinCellSize     = 5                // analytics_min_cell_size (owner LEARNING_ANALYTICS)
	EventSchedulerInterval          = 15 * time.Second // event-scheduler tick (F077 plan)
	GoalRecomputeInterval           = 5 * time.Minute  // goal-recompute [Proposed]
	LiveOpsJobBatch                 = 200
)

// LiveOps is the live-ops policy of the commands and the queries.
type LiveOps struct {
	Command command.LiveOpsPolicy
	Query   query.LiveOpsPolicy
}

// LiveOpsFrom reads the F077 policy (T002): EW_GAME_EVENT_*, EW_GAME_CGOAL_*, EW_GAME_SCHOOL_EVENT* and
// EW_GAME_ANALYTICS_MIN_CELL_SIZE with the CONFIG_DEFAULTS values [Proposed: the platform settings pipeline of module
// identity carries no live-ops key yet — same pattern as the economy config of F075].
func LiveOpsFrom(c *config.Config) (LiveOps, error) {
	r := liveops.Policy{FestivalMinDays: c.Int("EW_GAME_EVENT_FESTIVAL_MIN_DAYS", DefaultEventFestivalMinDays),
		ExtraDailyQuestsMax: c.Int("EW_GAME_EVENT_EXTRA_DAILY_QUESTS_MAX", DefaultEventExtraDailyQuestsMax),
		PointsDailyCap:      c.Int("EW_GAME_EVENT_POINTS_DAILY_CAP", DefaultEventPointsDailyCap),
		BundleMaxBytes:      int64(c.Int("EW_GAME_EVENT_DECORATION_BUNDLE_MAX_MB", DefaultDecorationBundleMaxMB)) * 1024 * 1024,
		GoalDailyCap:        c.Int("EW_GAME_CGOAL_CONTRIBUTION_DAILY_CAP", DefaultGoalContributionDailyCap)}
	upcoming := time.Duration(c.Int("EW_GAME_EVENT_UPCOMING_VISIBLE_DAYS", DefaultEventUpcomingVisibleDays)) * 24 * time.Hour
	minCell := c.Int("EW_GAME_ANALYTICS_MIN_CELL_SIZE", DefaultAnalyticsMinCellSize)
	p := command.LiveOpsPolicy{Rules: r, ClaimGrace: time.Duration(c.Int("EW_GAME_EVENT_CLAIM_GRACE_HOURS", DefaultEventClaimGraceHours)) * time.Hour,
		UpcomingVisible: upcoming, RuleChangeHour: c.Int("EW_GAME_EVENT_RULE_CHANGE_HOUR", DefaultEventRuleChangeHour),
		FlushEvery:         time.Duration(c.Int("EW_GAME_CGOAL_FLUSH_SECONDS", DefaultGoalFlushSeconds)) * time.Second,
		SchoolEventMaxDays: c.Int("EW_GAME_SCHOOL_EVENT_MAX_DAYS", DefaultSchoolEventMaxDays),
		SchoolEventsMax:    c.Int("EW_GAME_SCHOOL_EVENTS_CONCURRENT", DefaultSchoolEventsConcurrent), MinCell: minCell}
	out := LiveOps{Command: p, Query: query.LiveOpsPolicy{UpcomingVisible: upcoming, PointsDailyCap: r.PointsDailyCap, MinCell: minCell}}
	switch {
	case p.ClaimGrace < 0 || p.ClaimGrace > 336*time.Hour:
		return out, errors.New("game config: EW_GAME_EVENT_CLAIM_GRACE_HOURS must be 0..336")
	case r.FestivalMinDays < 1 || r.FestivalMinDays > 60:
		return out, errors.New("game config: EW_GAME_EVENT_FESTIVAL_MIN_DAYS must be 1..60")
	case r.ExtraDailyQuestsMax < 0 || r.ExtraDailyQuestsMax > 20:
		return out, errors.New("game config: EW_GAME_EVENT_EXTRA_DAILY_QUESTS_MAX must be 0..20")
	case r.PointsDailyCap < 1 || r.PointsDailyCap > 10000 || r.GoalDailyCap < 1 || r.GoalDailyCap > 10000:
		return out, errors.New("game config: EW_GAME_EVENT_POINTS_DAILY_CAP and EW_GAME_CGOAL_CONTRIBUTION_DAILY_CAP must be 1..10000")
	case upcoming < 0 || upcoming > 60*24*time.Hour:
		return out, errors.New("game config: EW_GAME_EVENT_UPCOMING_VISIBLE_DAYS must be 0..60")
	case p.RuleChangeHour < 0 || p.RuleChangeHour > 23:
		return out, errors.New("game config: EW_GAME_EVENT_RULE_CHANGE_HOUR must be 0..23")
	case p.FlushEvery < time.Second || p.FlushEvery > time.Hour:
		return out, errors.New("game config: EW_GAME_CGOAL_FLUSH_SECONDS must be 1..3600")
	case p.SchoolEventMaxDays < 1 || p.SchoolEventMaxDays > liveops.TemplateMaxDaysCap:
		return out, errors.New("game config: EW_GAME_SCHOOL_EVENT_MAX_DAYS must be 1..28")
	case p.SchoolEventsMax < 1 || p.SchoolEventsMax > 50:
		return out, errors.New("game config: EW_GAME_SCHOOL_EVENTS_CONCURRENT must be 1..50")
	case r.BundleMaxBytes < 1<<20 || r.BundleMaxBytes > 1<<30:
		return out, errors.New("game config: EW_GAME_EVENT_DECORATION_BUNDLE_MAX_MB must be 1..1024")
	case minCell < 1 || minCell > 100:
		return out, errors.New("game config: EW_GAME_ANALYTICS_MIN_CELL_SIZE must be 1..100")
	}
	return out, nil
}
