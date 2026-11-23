package query

import (
	"context"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/view"
	ldb "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/leaderboard"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/ranking"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// LeaderboardStaffPermission lets school staff read the school/grade boards and the collective goals of the grades of
// their schools (lms.student.read, ADR-0095 §6).
const LeaderboardStaffPermission = authz.PermLmsStudentRead

// Rankings are the read use cases of F106: getMyRatings, listSeasons/getSeason, listLeaderboards, getLeaderboard,
// getMyLeaderboardPosition and getLeaderboardHistory. They read the read model (PostgreSQL, Valkey boards) only; a lost
// board is computed from the ledgers (stale = true) and queued for the rebuild job (F106-AC-14).
type Rankings struct {
	Read   read.ReadSession
	Reader read.RankingReader
	Store  port.LeaderboardStore // nil: always computed from the ledgers
	Now    func() time.Time
}

func (q Rankings) now() time.Time {
	if q.Now != nil {
		return q.Now().UTC()
	}
	return time.Now().UTC()
}

// rankViewer is the caller: a student of the game client or staff of the portal of a tenant (platform principals
// without a tenant see nothing here: 404).
func rankViewer(ctx context.Context) (viewer, error) { return matchViewer(ctx) }

// GetMyRatings implements getMyRatings (F106-AC-05, AC-07): per mode × subject of the season, the tier code and name,
// provisional and its progress, games played — never the rating, RD or volatility.
func (q Rankings) GetMyRatings(ctx context.Context, season *uuid.UUID, locale string) (out []map[string]any, err error) {
	ctx, done := observability.UseCase(ctx, "competition.query.GetMyRatings")
	defer func() { done(err) }()
	v, err := rankViewer(ctx)
	if err != nil {
		return nil, err
	}
	if !v.student {
		return nil, problem.New(problem.GameAccessStudentOnly, "")
	}
	out = []map[string]any{}
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		rules, err := q.Reader.Rules(ctx, q.now())
		if err != nil {
			return err
		}
		var s *port.SeasonRow
		if season != nil {
			s, err = q.Reader.Season(ctx, *season)
		} else {
			s, err = q.Reader.ActiveSeason(ctx)
		}
		if err != nil {
			return err
		}
		if s == nil {
			if season != nil {
				return problem.New(problem.NotFound, "")
			}
			return nil
		}
		rs, err := q.Reader.MyRatings(ctx, v.tenant, v.user, s.ID)
		if err != nil {
			return err
		}
		for _, r := range rs {
			out = append(out, ratingBody(rules, r, locale))
		}
		return nil
	})
	return out, err
}

// ratingBody is the Rating of the contract: no rating number (ADR-0018).
func ratingBody(rules ranking.Rules, r port.RatingRow, locale string) map[string]any {
	prov := rules.IsProvisional(r.Games, r.Rating.RD)
	b := map[string]any{"mode": r.Mode, "subject_id": r.Subject, "season_id": r.Season, "provisional": prov,
		"provisional_games_required": rules.Provisional.MinGames, "games_played": r.Games, "tier_code": nil, "tier_name": nil,
		"peak_tier_code": r.Peak}
	if !prov && r.Tier != nil {
		b["tier_code"] = *r.Tier
		b["tier_name"] = rules.Tiers.Name(*r.Tier, locale)
	}
	return b
}

// SeasonPage is a page of seasons.
type SeasonPage struct {
	Data []map[string]any
	Next string
}

func offsetOf(cursor string) (int, error) {
	if cursor == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(strings.TrimPrefix(cursor, "o"))
	if err != nil || n < 0 || !strings.HasPrefix(cursor, "o") {
		return 0, problem.Invalid("cursor", problem.SubInvalid, "is not a cursor of this list")
	}
	return n, nil
}

// periodCursor is the opaque keyset cursor of getLeaderboardHistory after p: "p<captured_at in µs>.<period_key>".
func periodCursor(p read.SnapshotPeriod) string {
	return "p" + strconv.FormatInt(p.Captured.UnixMicro(), 10) + "." + p.Period
}

// periodKeysetOf decodes a cursor of periodCursor ("" = first page); an offset cursor of the former OFFSET paging is
// refused like any other foreign cursor.
func periodKeysetOf(cursor string) (*read.PeriodKeyset, error) {
	if cursor == "" {
		return nil, nil
	}
	at, period, ok := strings.Cut(strings.TrimPrefix(cursor, "p"), ".")
	us, err := strconv.ParseInt(at, 10, 64)
	if !strings.HasPrefix(cursor, "p") || !ok || err != nil || period == "" {
		return nil, problem.Invalid("cursor", problem.SubInvalid, "is not a cursor of this list")
	}
	return &read.PeriodKeyset{Captured: time.UnixMicro(us).UTC(), Period: period}, nil
}

// ListSeasons implements listSeasons (F106-AC-24): newest first; a student also gets the peak tier of the season.
func (q Rankings) ListSeasons(ctx context.Context, limit int, cursor string) (page SeasonPage, err error) {
	ctx, done := observability.UseCase(ctx, "competition.query.ListSeasons")
	defer func() { done(err) }()
	v, err := rankViewer(ctx)
	if err != nil {
		return page, err
	}
	off, err := offsetOf(cursor)
	if err != nil {
		return page, err
	}
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		rules, err := q.Reader.Rules(ctx, q.now())
		if err != nil {
			return err
		}
		ss, err := q.Reader.Seasons(ctx, limit+1, off)
		if err != nil {
			return err
		}
		if len(ss) > limit {
			ss = ss[:limit]
			page.Next = "o" + strconv.Itoa(off+limit)
		}
		peaks, err := q.peaks(ctx, v, ss)
		if err != nil {
			return err
		}
		page.Data = make([]map[string]any, 0, len(ss))
		for _, s := range ss {
			page.Data = append(page.Data, seasonBody(s, v.student, rules.Tiers, peaks[s.ID]))
		}
		return nil
	})
	return page, err
}

// GetSeason implements getSeason.
func (q Rankings) GetSeason(ctx context.Context, id uuid.UUID) (out map[string]any, err error) {
	ctx, done := observability.UseCase(ctx, "competition.query.GetSeason")
	defer func() { done(err) }()
	v, err := rankViewer(ctx)
	if err != nil {
		return nil, err
	}
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		s, err := q.Reader.Season(ctx, id)
		if err != nil || s == nil {
			return err
		}
		rules, err := q.Reader.Rules(ctx, q.now())
		if err != nil {
			return err
		}
		peaks, err := q.peaks(ctx, v, []port.SeasonRow{*s})
		if err != nil {
			return err
		}
		out = seasonBody(*s, v.student, rules.Tiers, peaks[s.ID])
		return nil
	})
	if err == nil && out == nil {
		return nil, problem.New(problem.NotFound, "")
	}
	return out, err
}

func (q Rankings) peaks(ctx context.Context, v viewer, ss []port.SeasonRow) (map[uuid.UUID][]string, error) {
	if !v.student || len(ss) == 0 {
		return nil, nil
	}
	ids := make([]uuid.UUID, len(ss))
	for i, s := range ss {
		ids[i] = s.ID
	}
	return q.Reader.PeakTiers(ctx, v.tenant, v.user, ids)
}

func seasonBody(s port.SeasonRow, student bool, t ranking.Tiers, peaks []string) map[string]any {
	b := map[string]any{"season_id": s.ID, "code": s.Code, "name": s.Name, "theme_code": s.Theme, "starts_at": s.StartsAt,
		"ends_at": s.EndsAt, "status": s.Status}
	if student {
		peak := ""
		for _, p := range peaks {
			peak = t.Peak(peak, p)
		}
		b["my_peak_tier_code"] = nil
		if peak != "" {
			b["my_peak_tier_code"] = peak
		}
	}
	return b
}

// served reports whether F106 serves a board of this scope (team and tournament wait for their entities, global for a
// cross-tenant projection — spec Clarifications 2026-11-22).
func served(d ldb.Definition) bool {
	switch d.Scope {
	case ldb.ScopeSchool, ldb.ScopeGrade, ldb.ScopeTenant, ldb.ScopeFriends:
		return d.Active()
	}
	return false
}

// staffSchool reports whether the staff grants holding LeaderboardStaffPermission cover school (nil: a tenant-wide grant).
func staffSchool(p authz.Principal, school *uuid.UUID) bool {
	for _, g := range p.Grants {
		held := false
		for _, perm := range g.Permissions {
			if perm == LeaderboardStaffPermission {
				held = true
			}
		}
		if held && (g.SchoolID == nil || (school != nil && *g.SchoolID == *school)) {
			return true
		}
	}
	return false
}

func (q Rankings) requireStaff(ctx context.Context, v viewer) error {
	if v.p.Has(LeaderboardStaffPermission) {
		return nil
	}
	return authz.Require(ctx, v.p, LeaderboardStaffPermission, authz.Resource{TenantID: &v.tenant})
}

// ListLeaderboards implements listLeaderboards (F106-AC-21): a student gets the boards of its grade (grades 1–3: only
// the goal of their grade), allowed by its school and the platform, with the scope holding it; staff with
// lms.student.read gets the school/grade boards.
func (q Rankings) ListLeaderboards(ctx context.Context) (out []map[string]any, err error) {
	ctx, done := observability.UseCase(ctx, "competition.query.ListLeaderboards")
	defer func() { done(err) }()
	v, err := rankViewer(ctx)
	if err != nil {
		return nil, err
	}
	if !v.student {
		if err := q.requireStaff(ctx, v); err != nil {
			return nil, err
		}
	}
	out = []map[string]any{}
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		defs, err := q.Reader.Definitions(ctx)
		if err != nil {
			return err
		}
		if !v.student {
			for _, d := range defs {
				if served(d) && d.SchoolScoped() {
					out = append(out, summaryBody(d, nil, nil))
				}
			}
			return nil
		}
		rules, err := q.Reader.Rules(ctx, q.now())
		if err != nil {
			return err
		}
		st, err := q.Reader.Student(ctx, v.tenant, v.user)
		if err != nil || st == nil || !st.Active {
			return err
		}
		policy, err := q.Reader.SchoolPolicy(ctx, v.tenant, st.School)
		if err != nil {
			return err
		}
		for _, d := range defs {
			if !served(d) || !ldb.Sees(rules.Visibility, d, st.Grade) || !policy.Allows(d) {
				continue
			}
			var scope *uuid.UUID
			var grade *int
			switch d.Scope {
			case ldb.ScopeSchool, ldb.ScopeGrade:
				s := st.School
				scope = &s
			case ldb.ScopeTenant:
				t := v.tenant
				scope = &t
			}
			if d.Scope == ldb.ScopeGrade || d.Scope == ldb.ScopeTenant {
				g := st.Grade
				grade = &g
			}
			out = append(out, summaryBody(d, scope, grade))
		}
		return nil
	})
	return out, err
}

func filtersBody(f ldb.Filters) map[string]any {
	return map[string]any{"subject_id": f.SubjectID, "topic_id": f.TopicID, "mode": f.Mode}
}

func summaryBody(d ldb.Definition, scope *uuid.UUID, grade *int) map[string]any {
	return map[string]any{"leaderboard_id": d.ID, "code": d.Code, "title": d.Title, "metric": d.Metric, "scope_type": d.Scope,
		"period": d.Period, "scope_id": scope, "grade_level": grade, "filters": filtersBody(d.Filters)}
}

// BoardParams are the query parameters of a board.
type BoardParams struct {
	ScopeID *uuid.UUID
	Grade   *int
	Period  string
	Limit   int
	Cursor  string
}

// board is a resolved board: the definition, the viewer's scope and period.
type board struct {
	v      viewer
	rules  ranking.Rules
	def    ldb.Definition
	member ldb.Member
	grade  int // grade of the viewing student (0 for staff)
	season *port.SeasonRow
	key    string
	start  time.Time
	end    time.Time
	scope  *uuid.UUID
}

func notFound() error { return problem.New(problem.NotFound, "") }

// resolve checks who may read the board in which scope (F106-AC-17, AC-19, AC-20): a student only the scope holding
// it, of a board of its grade allowed by its school; staff with lms.student.read the school/grade boards of the
// schools of its grants. Anything else is 404 (no existence leak).
func (q Rankings) resolve(ctx context.Context, v viewer, id uuid.UUID, p BoardParams, period bool) (board, error) {
	b := board{v: v}
	rules, err := q.Reader.Rules(ctx, q.now())
	if err != nil {
		return b, err
	}
	b.rules = rules
	defs, err := q.Reader.Definitions(ctx)
	if err != nil {
		return b, err
	}
	found := false
	for _, d := range defs {
		if d.ID == id {
			b.def, found = d, true
		}
	}
	if !found || !served(b.def) {
		return b, notFound()
	}
	d := b.def
	if v.student {
		st, err := q.Reader.Student(ctx, v.tenant, v.user)
		if err != nil {
			return b, err
		}
		if st == nil || !st.Active || !ldb.Sees(rules.Visibility, d, st.Grade) {
			return b, notFound()
		}
		policy, err := q.Reader.SchoolPolicy(ctx, v.tenant, st.School)
		if err != nil {
			return b, err
		}
		if !policy.Allows(d) || (p.Grade != nil && *p.Grade != st.Grade) {
			return b, notFound()
		}
		if p.ScopeID != nil {
			switch d.Scope {
			case ldb.ScopeSchool, ldb.ScopeGrade:
				if *p.ScopeID != st.School {
					return b, notFound()
				}
			case ldb.ScopeTenant:
				if *p.ScopeID != v.tenant {
					return b, notFound()
				}
			default:
				return b, notFound()
			}
		}
		b.member = ldb.Member{Tenant: v.tenant, School: st.School, Grade: st.Grade}
		b.grade = st.Grade
	} else {
		if !d.SchoolScoped() {
			return b, notFound()
		}
		if p.ScopeID == nil {
			return b, problem.Invalid("scope_id", problem.SubRequired, "names the school of the board")
		}
		if d.Scope == ldb.ScopeGrade && p.Grade == nil {
			return b, problem.Invalid("grade_level", problem.SubRequired, "names the grade of the board")
		}
		if !staffSchool(v.p, p.ScopeID) {
			return b, notFound()
		}
		known, err := q.Reader.SchoolKnown(ctx, v.tenant, *p.ScopeID)
		if err != nil {
			return b, err
		}
		if !known {
			return b, notFound()
		}
		g := 0
		if p.Grade != nil {
			g = *p.Grade
		}
		if d.Scope == ldb.ScopeGrade && !d.ForGrade(g) {
			return b, notFound()
		}
		b.member = ldb.Member{Tenant: v.tenant, School: *p.ScopeID, Grade: g}
	}
	switch d.Scope {
	case ldb.ScopeSchool, ldb.ScopeGrade:
		s := b.member.School
		b.scope = &s
	case ldb.ScopeTenant:
		t := v.tenant
		b.scope = &t
	}
	if !period {
		return b, nil
	}
	return b, q.period(ctx, &b, p.Period)
}

// period resolves the period key (empty: the open period) and its bounds.
func (q Rankings) period(ctx context.Context, b *board, key string) error {
	d := b.def
	if d.Period == ldb.PeriodSeason {
		var s *port.SeasonRow
		var err error
		if key == "" {
			s, err = q.Reader.ActiveSeason(ctx)
		} else if id, perr := uuid.Parse(strings.TrimPrefix(key, "s-")); perr == nil && strings.HasPrefix(key, "s-") {
			s, err = q.Reader.Season(ctx, id)
		}
		if err != nil {
			return err
		}
		if s == nil {
			if key == "" {
				return notFound()
			}
			return problem.Invalid("period_key", problem.SubInvalid, "is not a season of this board")
		}
		b.season = s
	}
	var sb *ldb.Season
	if b.season != nil {
		sb = b.season.Bounds()
	}
	if key == "" {
		key = ldb.Key(d.Period, q.now(), sb)
	}
	start, end, ok := ldb.Bounds(d.Period, key, sb)
	if !ok {
		return problem.Invalid("period_key", problem.SubInvalid, "is not a period of this board")
	}
	if b.season != nil && b.season.EndedAt != nil && b.season.EndedAt.Before(end) {
		end = *b.season.EndedAt
	}
	b.key, b.start, b.end = key, start, end
	return nil
}

// PageBody is a LeaderboardPage.
type PageBody = map[string]any

// GetLeaderboard implements getLeaderboard (F106-AC-12…AC-21).
func (q Rankings) GetLeaderboard(ctx context.Context, id uuid.UUID, p BoardParams) (out PageBody, err error) {
	ctx, done := observability.UseCase(ctx, "competition.query.GetLeaderboard")
	defer func() { done(err) }()
	return q.page(ctx, id, p, true)
}

// GetMyLeaderboardPosition implements getMyLeaderboardPosition: the viewer's own place (and ±N of grades 6–9) only.
func (q Rankings) GetMyLeaderboardPosition(ctx context.Context, id uuid.UUID, p BoardParams) (out PageBody, err error) {
	ctx, done := observability.UseCase(ctx, "competition.query.GetMyLeaderboardPosition")
	defer func() { done(err) }()
	v, err := rankViewer(ctx)
	if err != nil {
		return nil, err
	}
	if !v.student {
		return nil, problem.New(problem.GameAccessStudentOnly, "")
	}
	return q.page(ctx, id, p, false)
}

func (q Rankings) page(ctx context.Context, id uuid.UUID, p BoardParams, withEntries bool) (PageBody, error) {
	v, err := rankViewer(ctx)
	if err != nil {
		return nil, err
	}
	if !v.student {
		if err := q.requireStaff(ctx, v); err != nil {
			return nil, err
		}
	}
	var out PageBody
	var rebuild *port.BoardKey
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		b, err := q.resolve(ctx, v, id, p, true)
		if err != nil {
			return err
		}
		out = pageHead(b)
		if b.def.Metric == ldb.MetricGradeGoal {
			gg, err := q.gradeGoal(ctx, b, b.start, b.end)
			if err != nil {
				return err
			}
			out["visibility"] = ldb.VisibilityGradeGoal
			out["grade_goal"] = gg
			out["page"] = map[string]any{"has_more": false, "limit": max(p.Limit, 1), "next_cursor": nil}
			return nil
		}
		r, key, err := q.ranking(ctx, b, p)
		if err != nil {
			return err
		}
		rebuild = key
		return q.fill(ctx, out, b, r, p, withEntries)
	})
	if err != nil {
		return nil, err
	}
	if rebuild != nil && q.Store != nil {
		if err := q.Store.QueueRebuild(ctx, *rebuild); err != nil {
			slog.WarnContext(ctx, "competition: rebuild not queued", "error", err)
		}
	}
	return out, nil
}

func pageHead(b board) PageBody {
	return PageBody{"leaderboard_id": b.def.ID, "scope_type": b.def.Scope, "scope_id": b.scope, "metric": b.def.Metric, "period": b.def.Period,
		"period_key": b.key, "filters": filtersBody(b.def.Filters), "stale": false, "entries": []map[string]any{}, "me": nil,
		"around_me": []map[string]any{}}
}

// gradeGoal is the collective goal of the grade (F106-AC-16): current/target, the viewer's own share (students), the
// contributors cell hidden below analytics_min_cell_size; no rank, no nickname of anybody.
func (q Rankings) gradeGoal(ctx context.Context, b board, start, end time.Time) (map[string]any, error) {
	cur, n, err := q.Reader.GradeGoal(ctx, b.v.tenant, b.member.School, b.member.Grade, start, end)
	if err != nil {
		return nil, err
	}
	var mine *float64
	if b.v.student {
		m, err := q.Reader.MyEffort(ctx, b.v.tenant, b.v.user, b.member.School, b.member.Grade, start, end)
		if err != nil {
			return nil, err
		}
		mine = &m
	}
	g := ldb.NewGradeGoal(b.member.Grade, cur, b.def.GradeGoalTarget, mine, n, b.rules.MinCellSize)
	return map[string]any{"grade_level": g.Grade, "current": g.Current, "target": g.Target, "reached": g.Reached,
		"my_contribution": g.Mine, "contributors_count": g.Contributors}, nil
}

// rankingView is what a board shows before the visibility rules: the head, the viewer and around it.
type rankingView struct {
	total  int
	top    []ldb.Ranked
	me     *ldb.Ranked
	around []ldb.Ranked
	stale  bool
}

func ranksOf(ms []port.BoardMember) []ldb.Ranked {
	out := make([]ldb.Ranked, len(ms))
	for i, m := range ms {
		out[i] = ldb.Ranked{Student: m.Student, Rank: i + 1, Value: m.Value}
	}
	return out
}

func window(all []ldb.Ranked, from, to int) []ldb.Ranked {
	from, to = max(from, 1), min(to, len(all))
	if from > to {
		return nil
	}
	return append([]ldb.Ranked(nil), all[from-1:to]...)
}

// headSize is how many members a viewer may see from the top: the band of a student, the page of staff.
func headSize(b board, p BoardParams) (int, int, error) {
	if !b.v.student {
		off, err := offsetOf(p.Cursor)
		return off, max(p.Limit, 1), err
	}
	band, _ := ldb.Band(b.rules.Visibility, b.grade)
	return 0, band.Top, nil
}

// ranking reads the board: the closed period from its snapshot, the open period from Valkey, else from the ledgers
// (stale; the key to rebuild is returned).
func (q Rankings) ranking(ctx context.Context, b board, p BoardParams) (rankingView, *port.BoardKey, error) {
	off, size, err := headSize(b, p)
	if err != nil {
		return rankingView{}, nil, err
	}
	around := 0
	if b.v.student {
		band, _ := ldb.Band(b.rules.Visibility, b.grade)
		around = band.Around
	}
	if b.def.Scope == ldb.ScopeFriends {
		return q.friends(ctx, b, off, size, around)
	}
	scope := ldb.ScopeKey(b.def, b.member)
	if !b.end.After(q.now()) {
		r, ok, err := q.fromSnapshot(ctx, b, scope, off, size, around)
		if err != nil {
			return rankingView{}, nil, err
		}
		if ok {
			return r, nil, nil
		}
	}
	k := port.BoardKey{Tenant: b.v.tenant, Board: b.def.ID, Scope: scope, Period: b.key}
	if q.Store != nil && b.end.After(q.now()) {
		built, err := q.Store.Built(ctx, k)
		if err != nil {
			return rankingView{}, nil, err
		}
		if built {
			return q.fromStore(ctx, b, k, off, size, around)
		}
	}
	r, err := q.window(ctx, q.query(b, nil), b.v, off, size, around)
	if err != nil {
		return rankingView{}, nil, err
	}
	var rebuild *port.BoardKey
	if b.end.After(q.now()) {
		r.stale = q.Store != nil
		rebuild = &k
	}
	return r, rebuild, nil
}

func (q Rankings) query(b board, students []uuid.UUID) port.BoardQuery {
	qq := port.BoardQuery{Def: b.def, Tenant: b.v.tenant, Start: b.start, End: b.end, Students: students, Provisional: b.rules.Provisional,
		At: q.now()}
	if b.season != nil {
		qq.Season = &b.season.ID
	}
	school, grade, _ := commandScope(b.def, ldb.ScopeKey(b.def, b.member))
	qq.School, qq.Grade = school, grade
	return qq
}

// commandScope mirrors command.ParseScope (school and grade of a scope key).
func commandScope(d ldb.Definition, scope string) (*uuid.UUID, *int, bool) {
	head, g, hasGrade := strings.Cut(scope, ":g")
	switch d.Scope {
	case ldb.ScopeSchool:
		id, err := uuid.Parse(scope)
		return &id, nil, err == nil
	case ldb.ScopeGrade, ldb.ScopeTenant:
		n, err := strconv.Atoi(g)
		id, err2 := uuid.Parse(head)
		if !hasGrade || err != nil || err2 != nil {
			return nil, nil, false
		}
		if d.Scope == ldb.ScopeTenant {
			return nil, &n, true
		}
		return &id, &n, true
	}
	return nil, nil, true
}

// window reads the shown part of a board from the ledgers (the stale path): ranks off+1..off+size and, for a student,
// its own row and the ranks around it — ranked and cut in SQL (API_DESIGN §5), never the whole board.
func (q Rankings) window(ctx context.Context, bq port.BoardQuery, v viewer, off, size, around int) (rankingView, error) {
	w := read.RankWindowQuery{From: off + 1, To: off + size}
	if v.student {
		u := v.user
		w.Viewer, w.Around = &u, around
	}
	res, err := q.Reader.RankingWindow(ctx, bq, w)
	if err != nil {
		return rankingView{}, err
	}
	var me *ldb.Ranked
	for _, x := range res.Rows {
		if v.student && x.Student == v.user {
			m := x
			me = &m
		}
	}
	return split(res.Total, res.Rows, me, w), nil
}

// split sorts the rows read in SQL for a window w into the page [From, To] and the ranks within Around of me.
func split(total int, rows []ldb.Ranked, me *ldb.Ranked, w read.RankWindowQuery) rankingView {
	r := rankingView{total: total, me: me}
	for _, x := range rows {
		if x.Rank >= w.From && x.Rank <= w.To {
			r.top = append(r.top, x)
		}
		if me != nil && x.Rank >= me.Rank-w.Around && x.Rank <= me.Rank+w.Around {
			r.around = append(r.around, x)
		}
	}
	return r
}

func fromList(all []ldb.Ranked, v viewer, off, size, around int) rankingView {
	r := rankingView{total: len(all), top: window(all, off+1, off+size)}
	if v.student {
		for i := range all {
			if all[i].Student == v.user {
				me := all[i]
				r.me = &me
				r.around = window(all, me.Rank-around, me.Rank+around)
			}
		}
	}
	return r
}

func (q Rankings) fromStore(ctx context.Context, b board, k port.BoardKey, off, size, around int) (rankingView, *port.BoardKey, error) {
	var r rankingView
	var err error
	if r.total, err = q.Store.Count(ctx, k); err != nil {
		return r, nil, err
	}
	if r.top, err = q.Store.Range(ctx, k, off+1, off+size); err != nil {
		return r, nil, err
	}
	if b.v.student {
		if r.me, err = q.Store.RankOf(ctx, k, b.v.user); err != nil {
			return r, nil, err
		}
		if r.me != nil && around > 0 {
			if r.around, err = q.Store.Range(ctx, k, r.me.Rank-around, r.me.Rank+around); err != nil {
				return r, nil, err
			}
		}
	}
	return r, nil, nil
}

// fromSnapshot reads the shown part of a closed period from its snapshot: the page off+1..off+size of the shown top
// and, for a student, its own rank and the ranks around it — cut in SQL (API_DESIGN §5); ok is false when the period
// was not snapshotted.
func (q Rankings) fromSnapshot(ctx context.Context, b board, scope string, off, size, around int) (rankingView, bool, error) {
	w := read.RankWindowQuery{From: off + 1, To: off + size}
	if b.v.student {
		u := b.v.user
		w.Viewer, w.Around = &u, around
	}
	s, err := q.Reader.SnapshotWindow(ctx, b.v.tenant, b.def.ID, scope, b.key, w)
	if err != nil || s == nil {
		return rankingView{}, false, err
	}
	return split(s.Total, s.Rows, s.Me, w), true, nil
}

// friends reads a friends board (§9.3: ZMSCORE of ≤ friends_max friends on the tenant-wide set of the board).
func (q Rankings) friends(ctx context.Context, b board, off, size, around int) (rankingView, *port.BoardKey, error) {
	ids, err := q.Reader.Friends(ctx, b.v.tenant, b.v.user, b.rules.FriendsMax)
	if err != nil {
		return rankingView{}, nil, err
	}
	ids = append(ids, b.v.user)
	k := port.BoardKey{Tenant: b.v.tenant, Board: b.def.ID, Scope: "all", Period: b.key}
	var members []port.BoardMember
	stale := false
	built := false
	if q.Store != nil && b.end.After(q.now()) {
		if built, err = q.Store.Built(ctx, k); err != nil {
			return rankingView{}, nil, err
		}
	}
	if built {
		sc, err := q.Store.Scores(ctx, k, ids)
		if err != nil {
			return rankingView{}, nil, err
		}
		for _, m := range sc {
			members = append(members, m)
		}
		sort.Slice(members, func(i, j int) bool {
			if members[i].Value != members[j].Value {
				return members[i].Value > members[j].Value
			}
			if members[i].Tick != members[j].Tick {
				return members[i].Tick > members[j].Tick
			}
			return members[i].Student.String() < members[j].Student.String()
		})
	}
	var r rankingView
	if built {
		r = fromList(ranksOf(members), b.v, off, size, around)
	} else {
		// stale path: ranked and cut in SQL over the friends of the viewer
		if r, err = q.window(ctx, q.query(b, ids), b.v, off, size, around); err != nil {
			return rankingView{}, nil, err
		}
		stale = q.Store != nil && b.end.After(q.now())
	}
	r.stale = stale
	if stale {
		return r, &k, nil
	}
	return r, nil, nil
}

// fill applies the exclusions (sanctioned, inactive), the visibility of the viewer's grade (F106-AC-17) and the
// nickname-only cards (F106-AC-18) to the page.
func (q Rankings) fill(ctx context.Context, out PageBody, b board, r rankingView, p BoardParams, withEntries bool) error {
	ids := map[uuid.UUID]bool{}
	for _, x := range append(append([]ldb.Ranked(nil), r.top...), r.around...) {
		ids[x.Student] = true
	}
	if r.me != nil {
		ids[r.me.Student] = true
	}
	list := make([]uuid.UUID, 0, len(ids))
	for id := range ids {
		list = append(list, id)
	}
	excluded, err := q.Reader.Excluded(ctx, b.v.tenant, list, q.now())
	if err != nil {
		return err
	}
	keep := func(rs []ldb.Ranked) []ldb.Ranked {
		var o []ldb.Ranked
		for _, x := range rs {
			if !excluded[x.Student] {
				o = append(o, x)
			}
		}
		return o
	}
	r.top, r.around = keep(r.top), keep(r.around)
	if r.me != nil && excluded[r.me.Student] {
		r.me = nil
	}
	var entries, around []ldb.Ranked
	visibility := ldb.VisibilityRanked
	var me *ldb.Me
	if b.v.student {
		vw := ldb.Apply(b.rules.Visibility, b.grade, r.total, r.top, r.me, r.around)
		visibility, entries, me, around = vw.Visibility, vw.Entries, vw.Me, vw.Around
	} else {
		entries = r.top
	}
	if !withEntries {
		entries = nil
	}
	cards, err := q.Reader.Cards(ctx, b.v.tenant, list)
	if err != nil {
		return err
	}
	var tiers map[uuid.UUID]*string
	rating := b.def.Metric == ldb.MetricRating
	if rating && b.season != nil && b.def.Filters.Mode != nil && b.def.Filters.SubjectID != nil {
		if tiers, err = q.Reader.Tiers(ctx, b.v.tenant, list, *b.def.Filters.Mode, *b.def.Filters.SubjectID, b.season.ID); err != nil {
			return err
		}
	}
	entry := func(x ldb.Ranked) map[string]any {
		c, ok := cards[x.Student]
		if !ok {
			c = port.Card{UserID: x.Student, Level: 1}
		}
		e := map[string]any{"rank": x.Rank, "player": view.CardBody(c), "is_me": b.v.student && x.Student == b.v.user}
		if rating {
			e["tier_code"] = tiers[x.Student]
		} else {
			e["value"] = x.Value
		}
		return e
	}
	es := make([]map[string]any, 0, len(entries))
	for _, x := range entries {
		es = append(es, entry(x))
	}
	as := make([]map[string]any, 0, len(around))
	for _, x := range around {
		as = append(as, entry(x))
	}
	out["visibility"], out["entries"], out["around_me"], out["stale"] = visibility, es, as, r.stale
	if me != nil {
		m := map[string]any{"rank": me.Rank, "percentile_bucket": me.Percentile, "value": me.Value, "tier_code": nil}
		if rating {
			m["value"] = nil
			if r.me != nil {
				m["tier_code"] = tiers[r.me.Student]
			}
		}
		out["me"] = m
	}
	limit := max(p.Limit, 1)
	pg := map[string]any{"has_more": false, "limit": limit, "next_cursor": nil}
	if !b.v.student {
		off, _ := offsetOf(p.Cursor)
		if off+limit < r.total {
			pg["has_more"], pg["next_cursor"] = true, "o"+strconv.Itoa(off+limit)
		}
	}
	out["page"] = pg
	return nil
}

// HistoryPage is a page of closed periods.
type HistoryPage struct {
	Data []map[string]any
	Next string
}

// GetLeaderboardHistory implements getLeaderboardHistory (F106-AC-15, AC-21): the closed periods of the board in the
// viewer's scope from the snapshots, newest first, with the same visibility; a grade_goal board shows its collective
// total and whether it was reached.
func (q Rankings) GetLeaderboardHistory(ctx context.Context, id uuid.UUID, p BoardParams) (page HistoryPage, err error) {
	ctx, done := observability.UseCase(ctx, "competition.query.GetLeaderboardHistory")
	defer func() { done(err) }()
	v, err := rankViewer(ctx)
	if err != nil {
		return page, err
	}
	if !v.student {
		if err := q.requireStaff(ctx, v); err != nil {
			return page, err
		}
	}
	after, err := periodKeysetOf(p.Cursor)
	if err != nil {
		return page, err
	}
	limit := max(p.Limit, 1)
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		b, err := q.resolve(ctx, v, id, p, false)
		if err != nil {
			return err
		}
		if b.def.Scope == ldb.ScopeFriends {
			page.Data = []map[string]any{}
			return nil
		}
		scope := ldb.ScopeKey(b.def, b.member)
		periods, err := q.Reader.SnapshotPeriods(ctx, v.tenant, b.def.ID, scope, after, limit+1)
		if err != nil {
			return err
		}
		if len(periods) > limit {
			periods = periods[:limit]
			page.Next = periodCursor(periods[limit-1])
		}
		page.Data = make([]map[string]any, 0, len(periods))
		for _, sp := range periods {
			item := map[string]any{"period_key": sp.Period, "captured_at": sp.Captured, "entries": []map[string]any{}}
			if b.def.Metric == ldb.MetricGradeGoal {
				item["grade_goal"] = q.closedGoal(ctx, b, sp)
				page.Data = append(page.Data, item)
				continue
			}
			bb := b
			bb.key = sp.Period
			r, _, err := q.fromSnapshot(ctx, bb, scope, 0, snapshotHead(bb), 0)
			if err != nil {
				return err
			}
			body := PageBody{}
			if err := q.fill(ctx, body, bb, r, BoardParams{Limit: snapshotHead(bb)}, true); err != nil {
				return err
			}
			item["entries"] = body["entries"]
			page.Data = append(page.Data, item)
		}
		return nil
	})
	return page, err
}

func snapshotHead(b board) int {
	if !b.v.student {
		return 100
	}
	band, _ := ldb.Band(b.rules.Visibility, b.grade)
	return band.Top
}

// closedGoal is the collective row of a closed week (no rank, no name; the cell of contributors hidden below the
// minimum cell size).
func (q Rankings) closedGoal(ctx context.Context, b board, sp read.SnapshotPeriod) map[string]any {
	for _, row := range sp.Rows {
		if row.Student != nil {
			continue
		}
		target := b.def.GradeGoalTarget
		if row.Target != nil {
			target = *row.Target
		}
		n := 0
		if row.Members != nil {
			n = *row.Members
		}
		var mine *float64
		if b.v.student {
			if start, end, ok := ldb.Bounds(ldb.PeriodWeekly, sp.Period, nil); ok {
				if m, err := q.Reader.MyEffort(ctx, b.v.tenant, b.v.user, b.member.School, b.member.Grade, start, end); err == nil {
					mine = &m
				}
			}
		}
		g := ldb.NewGradeGoal(b.member.Grade, row.Value, target, mine, n, b.rules.MinCellSize)
		return map[string]any{"grade_level": g.Grade, "current": g.Current, "target": g.Target, "reached": g.Reached,
			"my_contribution": g.Mine, "contributors_count": g.Contributors}
	}
	return nil
}
