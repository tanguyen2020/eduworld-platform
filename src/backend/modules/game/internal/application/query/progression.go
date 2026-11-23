package query

import (
	"context"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/progression"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/reward"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// ict is the calendar of the daily XP cap (Asia/Ho_Chi_Minh).
var ict = time.FixedZone("ICT", 7*3600)

// Progression answers the progression routes of a student (F074 US3): getProgression, listXpHistory,
// listProgressionLevels and getXpStatus. Read only (READ ONLY session); the student is always the principal — no route
// takes another student id (F074-AC-19).
type Progression struct {
	Read     read.Session
	Curves   port.Curves
	Data     read.ProgressionReadModel
	Cursors  *page.Codec
	DailyCap int // xp_daily_cap
	Now      func() time.Time
}

func (h Progression) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

// RewardSummary is RewardGrantSummary of the contract.
type RewardSummary struct {
	Kind         string            `json:"kind"`
	Amount       *int              `json:"amount,omitempty"`
	CurrencyCode string            `json:"currency_code,omitempty"`
	ItemID       *string           `json:"item_id,omitempty"`
	Label        map[string]string `json:"label,omitempty"`
}

// Title is LevelTitle of the contract.
type Title struct {
	Code  string            `json:"code"`
	Label map[string]string `json:"label"`
}

// ProgressionStatus is ProgressionStatus of the contract (F074-AC-17).
type ProgressionStatus struct {
	Level            int             `json:"level"`
	Title            Title           `json:"title"`
	XPTotal          int64           `json:"xp_total"`
	XPInLevel        int64           `json:"xp_in_level"`
	XPToNext         *int64          `json:"xp_to_next"`
	DailyXPRemaining int             `json:"daily_xp_remaining"`
	CurveVersion     int             `json:"curve_version"`
	MaxLevel         int             `json:"max_level"`
	NextLevelRewards []RewardSummary `json:"next_level_rewards"`
}

// XPEntry is XpLedgerEntry of the contract: never source_id, rule_id or idempotency_key (F074-AC-18).
type XPEntry struct {
	Amount      int       `json:"amount"`
	SourceType  string    `json:"source_type"`
	SourceCode  string    `json:"source_code"`
	SourceLabel string    `json:"source_label,omitempty"`
	Capped      bool      `json:"capped"`
	CreatedAt   time.Time `json:"created_at"`
}

// XPHistoryPage is the page of listXpHistory.
type XPHistoryPage struct {
	Data []XPEntry `json:"data"`
	Page PageInfo  `json:"page"`
}

// LevelView is ProgressionLevel of the contract.
type LevelView struct {
	Level        int             `json:"level"`
	XPRequired   int64           `json:"xp_required"`
	XPCumulative int64           `json:"xp_cumulative"`
	TitleCode    string          `json:"title_code,omitempty"`
	Rewards      []RewardSummary `json:"rewards"`
}

// LevelsView is the body of listProgressionLevels.
type LevelsView struct {
	CurveVersion int         `json:"curve_version"`
	MaxLevel     int         `json:"max_level"`
	Levels       []LevelView `json:"levels"`
}

// DailyCap is one bucket of XpStatus.daily_caps.
type DailyCap struct {
	Bucket  string `json:"bucket"`
	Granted int    `json:"granted"`
	Cap     int    `json:"cap"`
}

// XPStatus is XpStatus of the contract (short form of getProgression).
type XPStatus struct {
	Level         int        `json:"level"`
	XPTotal       int64      `json:"xp_total"`
	XPToNextLevel int64      `json:"xp_to_next_level"`
	DailyCaps     []DailyCap `json:"daily_caps"`
}

// snapshotOf reads the profile, the current curve and today's XP of the principal.
type progressionSnapshot struct {
	level   int
	xp      int64
	curve   *port.CurveRow
	granted int
}

func (h Progression) load(ctx context.Context) (progressionSnapshot, error) {
	var s progressionSnapshot
	id, tenant, err := student(ctx)
	if err != nil {
		return s, err
	}
	err = h.Read.Read(ctx, func(ctx context.Context) error {
		p, err := h.Data.Profile(ctx, tenant, id)
		if err != nil {
			return err
		}
		s.level = 1
		if p != nil {
			s.level, s.xp = max(1, p.Level), max(0, p.XPTotal)
		}
		if s.curve, err = h.Curves.Current(ctx); err != nil {
			return err
		}
		s.granted, err = h.Data.DailyGranted(ctx, tenant, id, h.now().In(ict).Format(time.DateOnly), reward.TotalBucket)
		return err
	})
	if err == nil && s.curve == nil {
		err = problem.New(problem.Unavailable, "no progression curve is published")
	}
	return s, err
}

func summaries(rs []progression.Reward) []RewardSummary {
	out := make([]RewardSummary, 0, len(rs))
	for _, r := range rs {
		s := RewardSummary{Kind: r.Type()}
		if n, ok := r["amount"].(float64); ok {
			v := int(n)
			s.Amount = &v
		}
		if c, ok := r["currency_code"].(string); ok {
			s.CurrencyCode = c
		}
		if i, ok := r["item_id"].(string); ok {
			s.ItemID = &i
		}
		out = append(out, s)
	}
	return out
}

func mapRewards(ms []map[string]any) []progression.Reward {
	out := make([]progression.Reward, 0, len(ms))
	for _, m := range ms {
		out = append(out, progression.Reward(m))
	}
	return out
}

// GetProgression implements getProgression (F074-AC-17): level, title (vi/en), XP in the level, XP to the next level
// (null at max_level, F074-AC-16), XP still earnable today, curve version, max level and the rewards of the next level.
func (h Progression) GetProgression(ctx context.Context) (out ProgressionStatus, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.GetProgression")
	defer func() { done(err) }()
	s, err := h.load(ctx)
	if err != nil {
		return out, err
	}
	tb := s.curve.Table()
	level := min(s.level, tb.MaxLevel)
	in, next := tb.Status(level, s.xp)
	title := tb.TitleAt(level)
	out = ProgressionStatus{Level: level, Title: Title{Code: title.Code, Label: title.Name}, XPTotal: s.xp, XPInLevel: in, XPToNext: next,
		DailyXPRemaining: max(0, h.DailyCap-s.granted), CurveVersion: s.curve.Version, MaxLevel: tb.MaxLevel, NextLevelRewards: []RewardSummary{}}
	if level < tb.MaxLevel {
		out.NextLevelRewards = summaries(tb.Level(level + 1).Rewards)
	}
	return out, nil
}

// GetXPStatus implements getXpStatus: level, XP and today's cap (0 XP to the next level at max_level).
func (h Progression) GetXPStatus(ctx context.Context) (out XPStatus, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.GetXpStatus")
	defer func() { done(err) }()
	s, err := h.load(ctx)
	if err != nil {
		return out, err
	}
	tb := s.curve.Table()
	level := min(s.level, tb.MaxLevel)
	_, next := tb.Status(level, s.xp)
	out = XPStatus{Level: level, XPTotal: s.xp, DailyCaps: []DailyCap{{Bucket: reward.TotalBucket, Granted: s.granted, Cap: h.DailyCap}}}
	if next != nil {
		out.XPToNextLevel = *next
	}
	return out, nil
}

// ListXPHistory implements listXpHistory (F074-AC-18, AC-20): the postings of the principal newest first, cursor page,
// friendly source label in lang; a malformed cursor is 422 VALIDATION_FAILED on cursor.
func (h Progression) ListXPHistory(ctx context.Context, limit int, cursor, lang string) (out XPHistoryPage, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.ListXpHistory")
	defer func() { done(err) }()
	id, tenant, err := student(ctx)
	if err != nil {
		return out, err
	}
	if limit != 0 && (limit < 1 || limit > page.MaxLimit) {
		return out, problem.Invalid("limit", problem.SubInvalid, "must be 1..100")
	}
	limit = page.Limit(limit)
	var after *read.XPHistoryCursor
	if h.Cursors != nil && cursor != "" {
		c, err := h.Cursors.Decode(cursor)
		if err != nil {
			return out, problem.Invalid("cursor", problem.SubInvalid, "malformed cursor")
		}
		at, err := time.Parse(time.RFC3339Nano, c.Key)
		if err != nil {
			return out, problem.Invalid("cursor", problem.SubInvalid, "malformed cursor")
		}
		after = &read.XPHistoryCursor{At: at, ID: c.ID}
	}
	var rows []read.XPHistoryRow
	if err := h.Read.Read(ctx, func(ctx context.Context) error {
		var err error
		rows, err = h.Data.XPHistory(ctx, tenant, id, after, limit+1)
		return err
	}); err != nil {
		return out, err
	}
	out = XPHistoryPage{Data: []XPEntry{}, Page: PageInfo{Limit: limit}}
	if len(rows) > limit {
		rows, out.Page.HasMore = rows[:limit], true
		if h.Cursors != nil {
			last := rows[len(rows)-1]
			next := h.Cursors.Encode(page.Cursor{Key: last.CreatedAt.Format(time.RFC3339Nano), ID: last.ID})
			out.Page.NextCursor = &next
		}
	}
	for _, r := range rows {
		out.Data = append(out.Data, XPEntry{Amount: r.Amount, SourceType: r.SourceType, SourceCode: r.SourceCode,
			SourceLabel: reward.SourceLabel(r.SourceCode, lang), Capped: r.Capped, CreatedAt: r.CreatedAt})
	}
	return out, nil
}

// ListLevels implements listProgressionLevels: the level table of the current curve version (game.progression_levels).
func (h Progression) ListLevels(ctx context.Context) (out LevelsView, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.ListProgressionLevels")
	defer func() { done(err) }()
	if _, _, err := student(ctx); err != nil {
		return out, err
	}
	var curve *port.CurveRow
	var rows []read.ProgressionLevel
	if err := h.Read.Read(ctx, func(ctx context.Context) error {
		var err error
		if curve, err = h.Curves.Current(ctx); err != nil || curve == nil {
			return err
		}
		rows, err = h.Data.Levels(ctx, curve.VersionID)
		return err
	}); err != nil {
		return out, err
	}
	if curve == nil {
		return out, problem.New(problem.Unavailable, "no progression curve is published")
	}
	out = LevelsView{CurveVersion: curve.Version, MaxLevel: curve.Curve.MaxLevel, Levels: make([]LevelView, 0, len(rows))}
	for _, r := range rows {
		v := LevelView{Level: r.Level, XPCumulative: r.Cumulative, TitleCode: r.TitleCode, Rewards: summaries(mapRewards(r.Rewards))}
		if r.XPRequired != nil {
			v.XPRequired = *r.XPRequired
		}
		out.Levels = append(out.Levels, v)
	}
	return out, nil
}
