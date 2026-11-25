package query

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/liveops"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// LiveOpsPolicy are the parameters the live-ops reads apply (CONFIG_DEFAULTS).
type LiveOpsPolicy struct {
	UpcomingVisible time.Duration // event_upcoming_visible_days
	PointsDailyCap  int           // event_points_daily_cap
	MinCell         int           // analytics_min_cell_size
}

// LiveOps answers the live-ops reads (F077): the events, community goals and season track of a student (the principal,
// never another student), the run stats of the operators, the templates and school events of a tenant. Read only; every
// list is a keyset page in SQL.
type LiveOps struct {
	Read     read.Session
	Data     read.LiveOpsReadModel
	Students port.StudentDirectory
	Counter  port.GoalCounter
	Cursors  *page.Codec
	Policy   LiveOpsPolicy
	Now      func() time.Time
}

func (h LiveOps) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

func (h LiveOps) cursor(cursor string) (*page.Cursor, error) {
	if cursor == "" || h.Cursors == nil {
		return nil, nil
	}
	c, err := h.Cursors.Decode(cursor)
	if err != nil || c == nil {
		return nil, problem.Invalid("cursor", problem.SubInvalid, "malformed cursor")
	}
	return c, nil
}

func (h LiveOps) timeCursor(cursor string) (*read.RunCursor, error) {
	c, err := h.cursor(cursor)
	if err != nil || c == nil {
		return nil, err
	}
	at, err := time.Parse(time.RFC3339Nano, c.Key)
	if err != nil {
		return nil, problem.Invalid("cursor", problem.SubInvalid, "malformed cursor")
	}
	return &read.RunCursor{StartsAt: at, ID: c.ID}, nil
}

func (h LiveOps) next(key string, id uuid.UUID) *string {
	if h.Cursors == nil {
		return nil
	}
	if id == uuid.Nil {
		id = uuid.Max
	}
	s := h.Cursors.Encode(page.Cursor{Key: key, ID: id})
	return &s
}

// tenantName keeps vi and en (TenantLocalizedName: vi required, en optional).
func tenantName(l liveops.Localized) map[string]string {
	out := map[string]string{"vi": l["vi"]}
	if l["en"] != "" {
		out["en"] = l["en"]
	}
	return out
}

// bilingual keeps vi and en (LocalizedName: both required — the platform content always has both).
func bilingual(l liveops.Localized) map[string]string {
	out := map[string]string{"vi": l["vi"], "en": l["en"]}
	if out["en"] == "" {
		out["en"] = out["vi"]
	}
	return out
}

// EventSummaryView is GameEventSummary of the contract.
type EventSummaryView struct {
	EventID      uuid.UUID         `json:"event_id"`
	EventRunID   uuid.UUID         `json:"event_run_id"`
	EventKind    string            `json:"event_kind"`
	OwnerScope   string            `json:"owner_scope"`
	Name         map[string]string `json:"name"`
	ShortTagline map[string]string `json:"short_tagline,omitempty"`
	ThemeCode    string            `json:"theme_code,omitempty"`
	StartsAt     time.Time         `json:"starts_at"`
	EndsAt       time.Time         `json:"ends_at"`
	Status       string            `json:"status"`
	Joined       bool              `json:"joined"`
	EventPoints  int               `json:"event_points"`
}

// MilestoneView is EventMilestone.
type MilestoneView struct {
	MilestoneID    string          `json:"milestone_id"`
	PointsRequired int             `json:"points_required"`
	Rewards        []RewardSummary `json:"rewards"`
	Reached        bool            `json:"reached"`
	Claimed        bool            `json:"claimed"`
}

// GoalView is CommunityGoalProgress: totals and grades only — no ranking, no student identity (R16).
type GoalView struct {
	GoalID       uuid.UUID   `json:"goal_id"`
	EventRunID   uuid.UUID   `json:"event_run_id"`
	Metric       string      `json:"metric"`
	Scope        string      `json:"scope"`
	ScopeID      uuid.UUID   `json:"scope_id"`
	Target       int64       `json:"target"`
	Progress     int64       `json:"progress"`
	Milestones   []int       `json:"milestones_reached"`
	MyContribute bool        `json:"my_contributed"`
	ByGrade      []GradeView `json:"by_grade,omitempty"`
	UpdatedAt    time.Time   `json:"updated_at"`
}

// GradeView is one grade of a school (by_grade).
type GradeView struct {
	GradeLevel int   `json:"grade_level"`
	Progress   int64 `json:"progress"`
}

// EventDetailView is GameEventDetail.
type EventDetailView struct {
	EventSummaryView
	Description          map[string]string `json:"description,omitempty"`
	QuestIDs             []uuid.UUID       `json:"quest_ids"`
	ChallengeIDs         []uuid.UUID       `json:"challenge_ids"`
	BossIDs              []uuid.UUID       `json:"boss_ids"`
	ShopItemIDs          []uuid.UUID       `json:"shop_item_ids"`
	CollectionID         *uuid.UUID        `json:"collection_id,omitempty"`
	Milestones           []MilestoneView   `json:"milestones"`
	CommunityGoal        *GoalView         `json:"community_goal,omitempty"`
	DailyPointsRemaining int               `json:"daily_points_remaining"`
	ClaimDeadlineAt      time.Time         `json:"claim_deadline_at"`
}

func (h LiveOps) player(ctx context.Context) (uuid.UUID, uuid.UUID, *port.WorldStudent, error) {
	id, tenant, err := student(ctx)
	if err != nil {
		return id, tenant, nil, err
	}
	st, err := h.Students.Student(ctx, tenant, id)
	if err != nil {
		return id, tenant, nil, err
	}
	if st == nil || st.Deleted || st.Status != "active" {
		return id, tenant, nil, problem.New(problem.EventNotAvailable, "")
	}
	return id, tenant, st, nil
}

func summary(r port.EventRunRow, p *port.ParticipationRow) EventSummaryView {
	v := EventSummaryView{EventID: r.EventID, EventRunID: r.RunID, EventKind: r.Definition.Kind, OwnerScope: r.Definition.OwnerScope,
		Name: tenantName(r.Definition.Name), ThemeCode: r.Definition.ThemeCode, StartsAt: r.StartsAt, EndsAt: r.EndsAt, Status: r.Status}
	if r.Definition.ShortTagline != nil {
		v.ShortTagline = tenantName(r.Definition.ShortTagline)
	}
	if p != nil {
		v.Joined, v.EventPoints = true, p.Points
	}
	return v
}

// ListGameEvents is listGameEvents (F077-AC-07): live, suspended, ended (before claim_until) and upcoming runs of the
// student's audience (tenant, school, grade — never a class), keyset page on (starts_at, event_run_id). status: live,
// upcoming or both.
func (h LiveOps) ListGameEvents(ctx context.Context, statuses []string, limit int, cursor string) (out Page[EventSummaryView], err error) {
	ctx, done := observability.UseCase(ctx, "game.query.ListGameEvents")
	defer func() { done(err) }()
	if limit, err = limitOf(limit); err != nil {
		return out, err
	}
	after, err := h.timeCursor(cursor)
	if err != nil {
		return out, err
	}
	f := read.StudentRunsFilter{Now: h.now(), Upcoming: h.upcoming(), Live: len(statuses) == 0 || slices.Contains(statuses, "live"),
		Soon: len(statuses) == 0 || slices.Contains(statuses, "upcoming"), After: after, Limit: limit + 1}
	out = Page[EventSummaryView]{Data: []EventSummaryView{}, Page: PageInfo{Limit: limit}}
	err = h.Read.Read(ctx, func(ctx context.Context) error {
		id, tenant, st, err := h.player(ctx)
		if err != nil {
			return err
		}
		f.Tenant, f.School, f.Grade = tenant, st.SchoolID, st.Grade
		rows, err := h.Data.StudentRuns(ctx, f)
		if err != nil {
			return err
		}
		if len(rows) > limit {
			rows, out.Page.HasMore = rows[:limit], true
			last := rows[len(rows)-1]
			out.Page.NextCursor = h.next(last.StartsAt.Format(time.RFC3339Nano), last.RunID)
		}
		for _, r := range rows {
			p, err := h.Data.Participation(ctx, tenant, r.RunID, id)
			if err != nil {
				return err
			}
			out.Data = append(out.Data, summary(r, p))
		}
		return nil
	})
	return out, err
}

func (h LiveOps) upcoming() time.Duration {
	if h.Policy.UpcomingVisible > 0 {
		return h.Policy.UpcomingVisible
	}
	return 7 * 24 * time.Hour
}

// GetGameEvent is getGameEvent (F077-AC-07, AC-04): the run of the event the student sees (404 EVENT_NOT_AVAILABLE
// otherwise) with its components (hidden while suspended), its milestones and the student's progress.
func (h LiveOps) GetGameEvent(ctx context.Context, event uuid.UUID) (out EventDetailView, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.GetGameEvent")
	defer func() { done(err) }()
	now := h.now()
	err = h.Read.Read(ctx, func(ctx context.Context) error {
		id, tenant, st, err := h.player(ctx)
		if err != nil {
			return err
		}
		r, err := h.Data.EventRun(ctx, tenant, event)
		if err != nil {
			return err
		}
		if r == nil || !r.Audience.Includes(r.TenantID, tenant, st.SchoolID, st.Grade) || !liveops.Visible(r.Run(), now, h.upcoming()) {
			return problem.New(problem.EventNotAvailable, "")
		}
		p, err := h.Data.Participation(ctx, tenant, r.RunID, id)
		if err != nil {
			return err
		}
		d := r.Definition
		out = EventDetailView{EventSummaryView: summary(*r, p), QuestIDs: []uuid.UUID{}, ChallengeIDs: []uuid.UUID{}, BossIDs: []uuid.UUID{},
			ShopItemIDs: []uuid.UUID{}, Milestones: []MilestoneView{}, ClaimDeadlineAt: r.ClaimUntil}
		if d.Description != nil {
			out.Description = tenantName(d.Description)
		}
		if r.Status != liveops.StatusSuspended {
			c := d.Components
			out.QuestIDs, out.ChallengeIDs, out.BossIDs, out.ShopItemIDs = nonNilUUIDs(c.QuestIDs), nonNilUUIDs(c.ChallengeIDs), nonNilUUIDs(c.BossIDs),
				nonNilUUIDs(c.ShopItemIDs)
			out.CollectionID = c.CollectionID
		}
		var reached, claimed []string
		points := 0
		if p != nil {
			reached, points = liveops.Reached(d.MilestoneRewards, p.Points, p.Reached), p.Points
			claimed = append(append([]string{}, p.Claimed...), p.Granted...)
		}
		for _, m := range d.MilestoneRewards {
			out.Milestones = append(out.Milestones, MilestoneView{MilestoneID: m.MilestoneID, PointsRequired: m.EventPoints,
				Rewards: rewardSummaries(m.Rewards), Reached: slices.Contains(reached, m.MilestoneID) || points >= m.EventPoints,
				Claimed: slices.Contains(claimed, m.MilestoneID)})
		}
		if d.PointsEnabled() {
			dailyCap := d.EventPoints.DailyCap
			if dailyCap <= 0 {
				dailyCap = h.Policy.PointsDailyCap
			}
			today, err := h.Data.EventPointsOn(ctx, tenant, r.RunID, id, liveops.Day(now))
			if err != nil {
				return err
			}
			out.DailyPointsRemaining = max(0, dailyCap-today)
		}
		if r.Status != liveops.StatusSuspended {
			g, err := h.Data.RunGoal(ctx, r.RunID)
			if err != nil {
				return err
			}
			if g != nil {
				v, err := h.goalView(ctx, *g, tenant, id, *st, now)
				if err != nil {
					return err
				}
				out.CommunityGoal = &v
			}
		}
		return nil
	})
	return out, err
}

func nonNilUUIDs(s []uuid.UUID) []uuid.UUID {
	if s == nil {
		return []uuid.UUID{}
	}
	return s
}

func scopeIDOf(scope string, tenant, school uuid.UUID) uuid.UUID {
	if scope == liveops.ScopeSchool {
		return school
	}
	return tenant
}

// goalView reads the progress of the student's scope: the Valkey counter first, rebuilt from PostgreSQL when missing
// (F077-AC-14); a platform goal shows the total over every tenant (counts only).
func (h LiveOps) goalView(ctx context.Context, g port.GoalRow, tenant, studentID uuid.UUID, st port.WorldStudent, now time.Time) (GoalView, error) {
	scope := scopeIDOf(g.Scope, tenant, st.SchoolID)
	v := GoalView{GoalID: g.GoalID, EventRunID: g.RunID, Metric: g.Metric, Scope: g.Scope, ScopeID: scope, Target: g.Target, Milestones: []int{},
		UpdatedAt: now}
	prog, err := h.Data.Progress(ctx, tenant, g.GoalID, scope)
	if err != nil {
		return v, err
	}
	v.Milestones = append(v.Milestones, prog.Reached...)
	total := prog.Total
	if g.Scope == liveops.ScopePlatform {
		if total, err = h.Data.PlatformTotal(ctx, g.GoalID); err != nil {
			return v, err
		}
	} else if h.Counter != nil {
		if cached, ok, err := h.Counter.Get(ctx, g.GoalID, scope); err == nil && ok {
			total = max(cached, total) // the cache is never behind the committed total for long; never shown lower than PostgreSQL
		} else if err == nil {
			_ = h.Counter.Set(ctx, g.GoalID, scope, total) // rebuilt from PostgreSQL (the truth)
		}
	}
	v.Progress = total
	if v.MyContribute, err = h.Data.Contributed(ctx, tenant, g.GoalID, studentID); err != nil {
		return v, err
	}
	if g.ShowGrades && g.Scope != liveops.ScopePlatform {
		rows, err := h.Data.GradeContributions(ctx, tenant, g.GoalID, st.SchoolID)
		if err != nil {
			return v, err
		}
		minCell := h.Policy.MinCell
		if minCell <= 0 {
			minCell = 5
		}
		for _, r := range liveops.VisibleGrades(rows, minCell) {
			v.ByGrade = append(v.ByGrade, GradeView{GradeLevel: r.Grade, Progress: r.Progress})
		}
	}
	return v, nil
}

// GetCommunityGoal is getCommunityGoal (F077-AC-16): the progress of the student's scope, the milestones, the student's
// own "contributed" flag and the grades of its school (when the event shows them; a grade with fewer than
// analytics_min_cell_size contributing students is hidden) — never a ranking or a student identity.
func (h LiveOps) GetCommunityGoal(ctx context.Context, goal uuid.UUID) (out GoalView, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.GetCommunityGoal")
	defer func() { done(err) }()
	now := h.now()
	err = h.Read.Read(ctx, func(ctx context.Context) error {
		id, tenant, st, err := h.player(ctx)
		if err != nil {
			return err
		}
		g, err := h.Data.Goal(ctx, goal)
		if err != nil {
			return err
		}
		if g == nil {
			return problem.New(problem.EventNotAvailable, "")
		}
		r, err := h.Data.Run(ctx, g.RunID)
		if err != nil {
			return err
		}
		if r == nil || !r.Audience.Includes(r.TenantID, tenant, st.SchoolID, st.Grade) || !liveops.Visible(r.Run(), now, h.upcoming()) ||
			r.Status == liveops.StatusSuspended {
			return problem.New(problem.EventNotAvailable, "")
		}
		out, err = h.goalView(ctx, *g, tenant, id, *st, now)
		return err
	})
	return out, err
}

// SeasonTierView is SeasonTrackTier.
type SeasonTierView struct {
	Tier           int             `json:"tier"`
	PointsRequired int             `json:"points_required"`
	Rewards        []RewardSummary `json:"rewards"`
	Reached        bool            `json:"reached"`
	Claimed        bool            `json:"claimed"`
	Milestone      bool            `json:"milestone"`
}

// SeasonTrackView is SeasonTrack (free track only).
type SeasonTrackView struct {
	SeasonID             uuid.UUID         `json:"season_id"`
	SeasonName           map[string]string `json:"season_name"`
	StartsAt             time.Time         `json:"starts_at"`
	EndsAt               time.Time         `json:"ends_at"`
	Points               int               `json:"points"`
	Tier                 int               `json:"tier"`
	MaxTier              int               `json:"max_tier"`
	PointsPerTier        int               `json:"points_per_tier"`
	DailyPointsRemaining int               `json:"daily_points_remaining"`
	Tiers                []SeasonTierView  `json:"tiers"`
}

// GetSeasonTrack is getSeasonTrack (F077-AC-18): the free track of the current season (404 when no season is open or it
// has no track), the student's points, tier, daily points left and the tiers reached and claimed.
func (h LiveOps) GetSeasonTrack(ctx context.Context) (out SeasonTrackView, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.GetSeasonTrack")
	defer func() { done(err) }()
	id, tenant, err := student(ctx)
	if err != nil {
		return out, err
	}
	now := h.now()
	err = h.Read.Read(ctx, func(ctx context.Context) error {
		s, err := h.Data.CurrentSeason(ctx, now)
		if err != nil {
			return err
		}
		if s == nil {
			return problem.New(problem.NotFound, "")
		}
		tr, err := h.Data.Track(ctx, s.SeasonID)
		if err != nil {
			return err
		}
		if tr == nil {
			return problem.New(problem.NotFound, "")
		}
		p, err := h.Data.SeasonProgress(ctx, tenant, s.SeasonID, id)
		if err != nil {
			return err
		}
		today, err := h.Data.SeasonPointsOn(ctx, tenant, s.SeasonID, id, liveops.Day(now))
		if err != nil {
			return err
		}
		t := tr.Track
		out = SeasonTrackView{SeasonID: s.SeasonID, SeasonName: bilingual(t.Name), StartsAt: s.StartsAt, EndsAt: s.EndsAt, MaxTier: t.MaxTier(),
			PointsPerTier: t.Points.PerTier, DailyPointsRemaining: max(0, t.Points.DailyCap-today), Tiers: []SeasonTierView{}}
		var claimed []int
		if p != nil {
			out.Points, out.Tier = p.Points, p.Tier
			claimed = append(append([]int{}, p.Claimed...), p.Granted...)
		}
		for _, tier := range t.Tiers {
			out.Tiers = append(out.Tiers, SeasonTierView{Tier: tier.Tier, PointsRequired: tier.Tier * t.Points.PerTier, Rewards: rewardSummaries(tier.Rewards),
				Reached: out.Tier >= tier.Tier, Claimed: slices.Contains(claimed, tier.Tier), Milestone: tier.IsMilestone})
		}
		return nil
	})
	return out, err
}

// SeasonHistoryView is SeasonTrackHistoryItem.
type SeasonHistoryView struct {
	SeasonID         uuid.UUID         `json:"season_id"`
	SeasonName       map[string]string `json:"season_name"`
	FinalTier        int               `json:"final_tier"`
	Points           int               `json:"points"`
	AutoGrantedTiers []int             `json:"auto_granted_tiers"`
	EndedAt          time.Time         `json:"ended_at"`
}

// ListSeasonTrackHistory is listSeasonTrackHistory (F077-AC-20): the past seasons of the student (kept, never deleted).
func (h LiveOps) ListSeasonTrackHistory(ctx context.Context, limit int, cursor string) (out Page[SeasonHistoryView], err error) {
	ctx, done := observability.UseCase(ctx, "game.query.ListSeasonTrackHistory")
	defer func() { done(err) }()
	id, tenant, err := student(ctx)
	if err != nil {
		return out, err
	}
	if limit, err = limitOf(limit); err != nil {
		return out, err
	}
	c, err := h.timeCursor(cursor)
	if err != nil {
		return out, err
	}
	var after *read.SeasonCursor
	if c != nil {
		after = &read.SeasonCursor{EndedAt: c.StartsAt, ID: c.ID}
	}
	out = Page[SeasonHistoryView]{Data: []SeasonHistoryView{}, Page: PageInfo{Limit: limit}}
	err = h.Read.Read(ctx, func(ctx context.Context) error {
		rows, err := h.Data.SeasonHistory(ctx, tenant, id, after, limit+1)
		if err != nil {
			return err
		}
		if len(rows) > limit {
			rows, out.Page.HasMore = rows[:limit], true
			last := rows[len(rows)-1]
			out.Page.NextCursor = h.next(last.EndedAt.Format(time.RFC3339Nano), last.SeasonID)
		}
		for _, r := range rows {
			granted := r.Granted
			if granted == nil {
				granted = []int{}
			}
			out.Data = append(out.Data, SeasonHistoryView{SeasonID: r.SeasonID, SeasonName: map[string]string{"vi": r.SeasonName, "en": r.SeasonName},
				FinalTier: r.Tier, Points: r.Points, AutoGrantedTiers: granted, EndedAt: r.EndedAt})
		}
		return nil
	})
	return out, err
}

// GetEventRunStats is getEventRunStats (F077-AC-06): game.liveops.read of a platform role (internal gateway, portal
// client — route guards); counts over every tenant, never a student.
func (h LiveOps) GetEventRunStats(ctx context.Context, event, run uuid.UUID) (out map[string]any, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.GetEventRunStats")
	defer func() { done(err) }()
	p, ok := authz.PrincipalFrom(ctx)
	if !ok {
		return out, problem.New(problem.Unauthenticated, "")
	}
	if err := authz.Require(ctx, p, authz.PermGameLiveopsRead, authz.Resource{}); err != nil {
		return out, err
	}
	err = h.Read.Read(ctx, func(ctx context.Context) error {
		r, err := h.Data.Run(ctx, run)
		if err != nil {
			return err
		}
		if r == nil || r.EventID != event {
			return problem.New(problem.NotFound, "")
		}
		st, err := h.Data.RunStats(ctx, run)
		if err != nil {
			return err
		}
		out = port.RunStatsView(*r, st, h.now())
		return nil
	})
	return out, err
}

// TemplateView is EventTemplate.
type TemplateView struct {
	TemplateID       uuid.UUID         `json:"template_id"`
	Code             string            `json:"code"`
	Name             map[string]string `json:"name"`
	Description      map[string]string `json:"description,omitempty"`
	EventKind        string            `json:"event_kind"`
	AllowedParams    []string          `json:"allowed_params"`
	MaxDurationDays  int               `json:"max_duration_days"`
	QuestOptions     []uuid.UUID       `json:"quest_options"`
	ChallengeOptions []uuid.UUID       `json:"challenge_options"`
	QuestItems       []OptionItemView  `json:"quest_option_items"`
	ChallengeItems   []OptionItemView  `json:"challenge_option_items"`
	RewardOptions    []map[string]any  `json:"reward_options"`
}

// OptionItemView is EventTemplateOptionItem: a quest or challenge of the template with the title and grades of its
// current published version (F081-AC-02: the portal never looks names up in another module).
type OptionItemView struct {
	ID          uuid.UUID         `json:"id"`
	Title       map[string]string `json:"title"`
	GradeLevels []int             `json:"grade_levels"`
}

// optionItems keeps the order of the template; an option without a published version is left out.
func optionItems(ids []uuid.UUID, known map[uuid.UUID]read.OptionItem) []OptionItemView {
	out := []OptionItemView{}
	for _, id := range ids {
		it, ok := known[id]
		if !ok {
			continue
		}
		grades := it.GradeLevels
		if grades == nil {
			grades = []int{}
		}
		out = append(out, OptionItemView{ID: id, Title: bilingual(it.Title), GradeLevels: grades})
	}
	return out
}

func staffTenant(ctx context.Context) (authz.Principal, uuid.UUID, error) {
	p, ok := authz.PrincipalFrom(ctx)
	if !ok {
		return p, uuid.Nil, problem.New(problem.Unauthenticated, "")
	}
	if p.UserType != authz.UserTypeStaff {
		return p, uuid.Nil, problem.New(problem.PortalAccessStaffOnly, "")
	}
	if p.TenantID == nil {
		return p, uuid.Nil, problem.New(problem.PermissionDenied, "")
	}
	return p, *p.TenantID, nil
}

// schoolGrants returns the schools the school-event grants of p are limited to (nil, true = the whole tenant; false
// without any school = no grant).
func schoolGrants(p authz.Principal) ([]uuid.UUID, bool) {
	var schools []uuid.UUID
	for _, g := range p.Grants {
		if !slices.Contains(g.Permissions, authz.PermLmsSchoolEventWrite) {
			continue
		}
		if g.SchoolID == nil {
			return nil, true
		}
		schools = append(schools, *g.SchoolID)
	}
	return schools, false
}

// requireSchoolEvents checks lms.school_event.write held on the tenant or on one school at least (403 + authz.denied
// without it: academic_manager, F077-AC-25).
func requireSchoolEvents(ctx context.Context, p authz.Principal, tenant uuid.UUID) ([]uuid.UUID, error) {
	schools, all := schoolGrants(p)
	if all {
		return nil, nil
	}
	if len(schools) == 0 {
		return nil, authz.Require(ctx, p, authz.PermLmsSchoolEventWrite, authz.Resource{TenantID: &tenant})
	}
	return schools, nil
}

// ListEventTemplates is listEventTemplates (F077-AC-21): the published templates assigned to the session tenant, keyset
// page on code.
func (h LiveOps) ListEventTemplates(ctx context.Context, limit int, cursor string) (out Page[TemplateView], err error) {
	ctx, done := observability.UseCase(ctx, "game.query.ListEventTemplates")
	defer func() { done(err) }()
	p, tenant, err := staffTenant(ctx)
	if err != nil {
		return out, err
	}
	if _, err := requireSchoolEvents(ctx, p, tenant); err != nil {
		return out, err
	}
	if limit, err = limitOf(limit); err != nil {
		return out, err
	}
	c, err := h.cursor(cursor)
	if err != nil {
		return out, err
	}
	afterCode := ""
	if c != nil {
		afterCode = c.Key
	}
	out = Page[TemplateView]{Data: []TemplateView{}, Page: PageInfo{Limit: limit}}
	err = h.Read.Read(ctx, func(ctx context.Context) error {
		rows, err := h.Data.Templates(ctx, tenant, afterCode, limit+1)
		if err != nil {
			return err
		}
		if len(rows) > limit {
			rows, out.Page.HasMore = rows[:limit], true
			out.Page.NextCursor = h.next(rows[len(rows)-1].Template.Code, uuid.Nil)
		}
		var questIDs, challengeIDs []uuid.UUID
		for _, r := range rows {
			questIDs = append(questIDs, r.Template.QuestOptions...)
			challengeIDs = append(challengeIDs, r.Template.ChallengeOptions...)
		}
		quests, err := h.Data.QuestOptionItems(ctx, questIDs)
		if err != nil {
			return err
		}
		challenges, err := h.Data.ChallengeOptionItems(ctx, challengeIDs)
		if err != nil {
			return err
		}
		for _, r := range rows {
			t := r.Template
			v := TemplateView{TemplateID: r.TemplateID, Code: t.Code, Name: bilingual(t.Name), EventKind: t.Kind, AllowedParams: nonNilStrings(t.AllowedParams),
				MaxDurationDays: t.MaxDurationDays, QuestOptions: nonNilUUIDs(t.QuestOptions), ChallengeOptions: nonNilUUIDs(t.ChallengeOptions),
				QuestItems: optionItems(t.QuestOptions, quests), ChallengeItems: optionItems(t.ChallengeOptions, challenges), RewardOptions: []map[string]any{}}
			if t.Description != nil {
				v.Description = bilingual(t.Description)
			}
			for _, o := range t.RewardOptions {
				v.RewardOptions = append(v.RewardOptions, map[string]any{"reward_option_id": o.RewardOptionID, "label": bilingual(o.Label)})
			}
			out.Data = append(out.Data, v)
		}
		return nil
	})
	return out, err
}

func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// ListSchoolEvents is listSchoolEvents (F077-AC-25, F081-AC-06): the school events of the session tenant, newest first
// (keyset on created_at, event_id); school-limited staff see their schools only. System Admin without X-Tenant-Id reads
// the tenants of tenant_ids or every tenant (audited cross_tenant.accessed); each row carries its tenant_id.
func (h LiveOps) ListSchoolEvents(ctx context.Context, status string, limit int, cursor string) (out Page[map[string]any], err error) {
	ctx, done := observability.UseCase(ctx, "game.query.ListSchoolEvents")
	defer func() { done(err) }()
	var f read.SchoolEventsFilter
	if p, ok := authz.PrincipalFrom(ctx); ok && p.TenantID == nil && p.UserType == authz.UserTypeStaff {
		if ctx, f, err = crossTenantEvents(ctx, p); err != nil {
			return out, err
		}
	} else {
		p, tenant, err := staffTenant(ctx)
		if err != nil {
			return out, err
		}
		if f.Schools, err = requireSchoolEvents(ctx, p, tenant); err != nil {
			return out, err
		}
		f.Tenants = []uuid.UUID{tenant}
	}
	if limit, err = limitOf(limit); err != nil {
		return out, err
	}
	after, err := h.timeCursor(cursor)
	if err != nil {
		return out, err
	}
	out = Page[map[string]any]{Data: []map[string]any{}, Page: PageInfo{Limit: limit}}
	err = h.Read.Read(ctx, func(ctx context.Context) error {
		f.Status, f.After, f.Limit = status, after, limit+1
		rows, err := h.Data.SchoolEvents(ctx, f)
		if err != nil {
			return err
		}
		if len(rows) > limit {
			rows, out.Page.HasMore = rows[:limit], true
			last := rows[len(rows)-1]
			out.Page.NextCursor = h.next(last.CreatedAt.Format(time.RFC3339Nano), last.EventID)
		}
		for _, r := range rows {
			out.Data = append(out.Data, port.SchoolEventView(r))
		}
		return nil
	})
	return out, err
}

// GetSchoolEvent is getSchoolEvent (F077-AC-25): an event of another school or tenant is 404.
func (h LiveOps) GetSchoolEvent(ctx context.Context, event uuid.UUID) (out map[string]any, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.GetSchoolEvent")
	defer func() { done(err) }()
	p, tenant, err := staffTenant(ctx)
	if err != nil {
		return out, err
	}
	schools, err := requireSchoolEvents(ctx, p, tenant)
	if err != nil {
		return out, err
	}
	err = h.Read.Read(ctx, func(ctx context.Context) error {
		r, err := h.Data.SchoolEvent(ctx, tenant, event)
		if err != nil {
			return err
		}
		if r == nil || (schools != nil && (r.SchoolID == nil || !slices.Contains(schools, *r.SchoolID))) {
			return problem.New(problem.NotFound, "")
		}
		out = port.SchoolEventView(*r)
		return nil
	})
	return out, err
}
