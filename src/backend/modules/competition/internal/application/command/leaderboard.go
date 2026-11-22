package command

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/write"
	ldb "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/leaderboard"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/ranking"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// Jobs of F106 (scope names of SystemPlatformScope).
const (
	JobPeriodClose        = "leaderboard-period-close"
	JobLeaderboardRebuild = "leaderboard-rebuild"
	JobSeasonScheduler    = "season-scheduler"
	JobRDInflation        = "rd-inflation"
	// BoardRetention keeps a board in Valkey 7 days past the end of its period (§9.3).
	BoardRetention = 7 * 24 * time.Hour
	// snapshotTop is the top of every snapshot (§9.3: top 100 + the rank of every member of a school/grade scope).
	snapshotTop = 100
	// closeMargin: a period is closed 5 minutes after its end (§9.5).
	closeMargin = 5 * time.Minute
)

// Increment is a change of one student on the boards of one metric: an amount added (points, effort, team) or a value
// set (rating; Remove takes a provisional rating out).
type Increment struct {
	Metric  string
	Tenant  uuid.UUID
	Member  ldb.Member
	Student uuid.UUID
	Subject *uuid.UUID
	Topic   *uuid.UUID
	Mode    *string
	Amount  float64
	At      time.Time
	Set     bool
	Remove  bool
}

type planned struct {
	key     port.BoardKey
	student uuid.UUID
	value   float64
	tick    float64
	set     bool
	remove  bool
	ttl     time.Duration
}

// Boards keeps the Valkey boards of the open periods in step with the ledgers (F106-AC-13): Plan resolves, inside the
// transaction that wrote the ledger (after the processed_events check), the boards an increment reaches; Apply runs
// after the commit. A failed or dropped increment is repaired by the rebuild/reconciliation jobs (AC-14).
type Boards struct {
	Store   port.LeaderboardStore // nil: no cache
	Repo    write.RankingRepository
	Metrics port.RankingMetrics
	Now     func() time.Time
}

func (b Boards) now() time.Time {
	if b.Now != nil {
		return b.Now().UTC()
	}
	return time.Now().UTC()
}

// cached reports whether the scope of a definition has a Valkey board (team and tournament wait for their entities,
// global for a cross-tenant projection — Clarifications 2026-11-22).
func cached(d ldb.Definition) bool {
	switch d.Scope {
	case ldb.ScopeSchool, ldb.ScopeGrade, ldb.ScopeTenant, ldb.ScopeFriends:
		return d.Active()
	}
	return false
}

func matches(d ldb.Definition, inc Increment) bool {
	if d.Metric != inc.Metric || !d.ForGrade(inc.Member.Grade) {
		return false
	}
	f := d.Filters
	return (f.SubjectID == nil || (inc.Subject != nil && *inc.Subject == *f.SubjectID)) &&
		(f.TopicID == nil || (inc.Topic != nil && *inc.Topic == *f.TopicID)) &&
		(f.Mode == nil || (inc.Mode != nil && *inc.Mode == *f.Mode))
}

// periodOf returns the key and bounds of the period of d containing at.
func periodOf(d ldb.Definition, at time.Time, season *write.SeasonRow) (string, time.Time, time.Time, bool) {
	var s *ldb.Season
	if season != nil {
		s = season.Bounds()
	}
	key := ldb.Key(d.Period, at, s)
	if key == "" {
		return "", time.Time{}, time.Time{}, false
	}
	start, end, ok := ldb.Bounds(d.Period, key, s)
	return key, start, end, ok
}

// Plan resolves the boards of the increments (inside the transaction of the ledger write): active definitions of the
// metric and grade whose filters match; a board across schools only when the school allows it
// (allow_cross_school_listing); sanctioned students (restrict_competition) are left out of the period (F106-AC-18).
func (b Boards) Plan(ctx context.Context, season *write.SeasonRow, incs []Increment) ([]planned, error) {
	if b.Store == nil || len(incs) == 0 {
		return nil, nil
	}
	defs, err := b.Repo.Definitions(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(incs))
	for _, i := range incs {
		ids = append(ids, i.Student)
	}
	restricted, err := b.Repo.Restricted(ctx, incs[0].Tenant, ids, b.now())
	if err != nil {
		return nil, err
	}
	policies := map[uuid.UUID]write.SchoolPolicy{}
	var out []planned
	for _, inc := range incs {
		for _, d := range defs {
			if !cached(d) || !matches(d, inc) {
				continue
			}
			if d.CrossSchool() {
				p, ok := policies[inc.Member.School]
				if !ok {
					if p, err = b.Repo.SchoolPolicy(ctx, inc.Tenant, inc.Member.School); err != nil {
						return nil, err
					}
					policies[inc.Member.School] = p
				}
				if !p.CrossSchoolAllow {
					continue
				}
			}
			key, start, end, ok := periodOf(d, inc.At, season)
			if !ok {
				continue
			}
			p := planned{key: port.BoardKey{Tenant: inc.Tenant, Board: d.ID, Scope: ldb.ScopeKey(d, inc.Member), Period: key},
				student: inc.Student, value: inc.Amount, tick: ldb.Tick(start, inc.At), set: inc.Set, remove: inc.Remove || restricted[inc.Student],
				ttl: end.Add(BoardRetention).Sub(b.now())}
			if p.ttl <= 0 {
				continue
			}
			out = append(out, p)
		}
	}
	return out, nil
}

// Apply writes the planned increments to Valkey (after the commit). A board not built yet is queued for rebuild: the
// rebuild reads the committed ledger, so nothing is counted twice.
func (b Boards) Apply(ctx context.Context, ps []planned) {
	for _, p := range ps {
		var built bool
		var err error
		switch {
		case p.remove:
			err, built = b.Store.Remove(ctx, p.key, p.student), true
		case p.set:
			built, err = b.Store.Set(ctx, p.key, p.student, p.value, p.tick, p.ttl)
		case p.value != 0:
			built, err = b.Store.Incr(ctx, p.key, p.student, p.value, p.tick, p.ttl)
		default:
			continue
		}
		if err == nil && !built {
			err = b.Store.QueueRebuild(ctx, p.key)
		}
		if err != nil {
			slog.WarnContext(ctx, "competition: leaderboard not updated (rebuilt by the reconciliation)", "error", err)
		}
	}
}

// Leaderboards are the leaderboard jobs (worker jobs): rebuild of the boards lost or queued, hourly reconciliation,
// period close with snapshots, and the materialization of the published definitions.
type Leaderboards struct {
	UoW     *entx.UnitOfWork
	Repo    write.LeaderboardRepository
	Ranking write.RankingRepository
	Seasons write.SeasonRepository
	Store   port.LeaderboardStore
	Events  write.EventPublisher
	Audit   write.Auditor
	Metrics port.RankingMetrics
	Now     func() time.Time
}

func (h Leaderboards) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

// ParseScope returns the school and grade of a scope key of d (school "<school>", grade "<school>:g<N>", tenant
// "<tenant>:g<N>", friends "all").
func ParseScope(d ldb.Definition, scope string) (school *uuid.UUID, grade *int, ok bool) {
	head, g, hasGrade := strings.Cut(scope, ":g")
	switch d.Scope {
	case ldb.ScopeFriends:
		return nil, nil, scope == "all"
	case ldb.ScopeSchool:
		id, err := uuid.Parse(scope)
		return &id, nil, err == nil
	case ldb.ScopeGrade, ldb.ScopeTenant:
		n, err := strconv.Atoi(g)
		id, err2 := uuid.Parse(head)
		if !hasGrade || err != nil || err2 != nil || n < 1 || n > 9 {
			return nil, nil, false
		}
		if d.Scope == ldb.ScopeTenant {
			return nil, &n, true
		}
		return &id, &n, true
	}
	return nil, nil, false
}

// boardQuery is the ledger query of a board key; ok false for an unknown board or period.
func (h Leaderboards) boardQuery(ctx context.Context, k port.BoardKey) (write.BoardQuery, bool, error) {
	defs, err := h.Repo.Definitions(ctx)
	if err != nil {
		return write.BoardQuery{}, false, err
	}
	var def *ldb.Definition
	for i := range defs {
		if defs[i].ID == k.Board {
			def = &defs[i]
		}
	}
	if def == nil || !cached(*def) {
		return write.BoardQuery{}, false, nil
	}
	school, grade, ok := ParseScope(*def, k.Scope)
	if !ok {
		return write.BoardQuery{}, false, nil
	}
	var season *ldb.Season
	q := write.BoardQuery{Def: *def, Tenant: k.Tenant, School: school, Grade: grade, At: h.now()}
	if def.Period == ldb.PeriodSeason {
		id, isSeason := seasonKey(k.Period)
		if !isSeason {
			return write.BoardQuery{}, false, nil // not a season key: no such board
		}
		s, err := h.seasonByID(ctx, id)
		if err != nil || s == nil {
			return write.BoardQuery{}, false, err
		}
		season = s.Bounds()
		q.Season = &s.ID
	}
	start, end, ok := ldb.Bounds(def.Period, k.Period, season)
	if !ok {
		return write.BoardQuery{}, false, nil
	}
	q.Start, q.End = start, end
	if def.Metric == ldb.MetricRating {
		rules, _, err := h.Ranking.Rules(ctx, q.At)
		if err != nil {
			return write.BoardQuery{}, false, err
		}
		q.Provisional = rules.Provisional
	}
	return q, true, nil
}

// seasonKey returns the season of a season period key (s-<season_id>).
func seasonKey(key string) (uuid.UUID, bool) {
	id, err := uuid.Parse(strings.TrimPrefix(key, "s-"))
	return id, err == nil && strings.HasPrefix(key, "s-")
}

func (h Leaderboards) seasonByID(ctx context.Context, id uuid.UUID) (*write.SeasonRow, error) {
	all, err := h.Seasons.Seasons(ctx)
	if err != nil {
		return nil, err
	}
	for i := range all {
		if all[i].ID == id {
			return &all[i], nil
		}
	}
	return nil, nil
}

func (h Leaderboards) endOf(q write.BoardQuery) time.Duration {
	return q.End.Add(BoardRetention).Sub(h.now())
}

// Rebuild rebuilds one board from the ledger of its tenant (F106-AC-14, SC-005).
func (h Leaderboards) Rebuild(ctx context.Context, k port.BoardKey, reason string) (err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.RebuildLeaderboard")
	defer func() { done(err) }()
	sc, err := authz.EventScope(k.Tenant)
	if err != nil {
		return err
	}
	var members []port.BoardMember
	var q write.BoardQuery
	var ok bool
	err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		var err error
		if q, ok, err = h.boardQuery(ctx, k); err != nil || !ok {
			return err
		}
		members, err = h.Repo.Ranking(ctx, q)
		return err
	})
	if err != nil || !ok || h.endOf(q) <= 0 {
		return err
	}
	if err := h.Store.Replace(ctx, k, members, h.endOf(q)); err != nil {
		return err
	}
	if h.Metrics != nil {
		h.Metrics.Rebuild(reason)
	}
	return nil
}

// RebuildQueued rebuilds the boards queued by the read side (lost keys) and by dropped increments.
func (h Leaderboards) RebuildQueued(ctx context.Context) (int, error) {
	ks, err := h.Store.PopRebuilds(ctx, 50)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, k := range ks {
		if err := h.Rebuild(ctx, k, "missing"); err != nil {
			_ = h.Store.QueueRebuild(ctx, k)
			return n, fmt.Errorf("rebuild %s: %w", k.Board, err)
		}
		n++
	}
	return n, nil
}

// Reconcile compares every built board with its ledger and rebuilds the boards that drifted (F106-AC-14: increments
// lost after a commit, sanctions, students cleaned up).
func (h Leaderboards) Reconcile(ctx context.Context) (int, error) {
	ks, err := h.Store.BuiltBoards(ctx, 2000)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, k := range ks {
		sc, err := authz.EventScope(k.Tenant)
		if err != nil {
			continue
		}
		var want []port.BoardMember
		var ok bool
		err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
			q, found, err := h.boardQuery(ctx, k)
			if err != nil || !found {
				return err
			}
			ok = true
			want, err = h.Repo.Ranking(ctx, q)
			return err
		})
		if err != nil {
			return n, err
		}
		if !ok {
			continue
		}
		got, err := h.Store.Members(ctx, k)
		if err != nil {
			return n, err
		}
		if !sameValues(want, got) {
			if h.Metrics != nil {
				h.Metrics.Drift()
			}
			if err := h.Rebuild(ctx, k, "drift"); err != nil {
				return n, err
			}
			n++
		}
	}
	return n, nil
}

func sameValues(a, b []port.BoardMember) bool {
	if len(a) != len(b) {
		return false
	}
	m := make(map[uuid.UUID]float64, len(a))
	for _, x := range a {
		m[x.Student] = x.Value
	}
	for _, y := range b {
		if v, ok := m[y.Student]; !ok || v != y.Value {
			return false
		}
	}
	return true
}

// Materialize copies the definitions of the published leaderboard-definitions configuration into
// competition.leaderboard_definitions (F108 publishes; an invalid definition — class, XP in a school — is never
// materialized, F106-AC-12).
func (h Leaderboards) Materialize(ctx context.Context) (int, error) {
	ps, err := authz.SystemPlatformScope(JobPeriodClose)
	if err != nil {
		return 0, err
	}
	n := 0
	err = h.UoW.InTenantTx(ctx, ps, func(ctx context.Context) error {
		defs, version, err := h.Repo.PublishedDefinitions(ctx, h.now())
		if err != nil {
			return err
		}
		for _, d := range defs {
			if err := d.Validate(); err != nil {
				slog.WarnContext(ctx, "competition: leaderboard definition refused", "code", d.Code, "error", err)
				continue
			}
			ok, err := h.Repo.UpsertDefinition(ctx, d, version)
			if err != nil {
				return err
			}
			if ok {
				n++
			}
		}
		return nil
	})
	return n, err
}

// closedPeriod is the last closed period of d at now (5 minutes after its end): daily/weekly/monthly the previous
// period, season the last ended season.
func (h Leaderboards) closedPeriod(d ldb.Definition, now time.Time, seasons []write.SeasonRow) (string, time.Time, time.Time, *uuid.UUID, bool) {
	if d.Period == ldb.PeriodSeason {
		var last *write.SeasonRow
		for i := range seasons {
			s := seasons[i]
			if s.Status == "ended" && s.EndedAt != nil && !s.EndedAt.Add(closeMargin).After(now) && (last == nil || s.EndedAt.After(*last.EndedAt)) {
				last = &seasons[i]
			}
		}
		if last == nil {
			return "", time.Time{}, time.Time{}, nil, false
		}
		return ldb.SeasonKey(last.ID), last.StartsAt, *last.EndedAt, &last.ID, true
	}
	key := ldb.Previous(d.Period, now)
	start, end, ok := ldb.Bounds(d.Period, key, nil)
	if !ok || end.Add(closeMargin).After(now) {
		return "", time.Time{}, time.Time{}, nil, false
	}
	return key, start, end, nil, true
}

// ClosePeriods implements F106-AC-15: for every closed period of a board of a school, grade or tenant scope, a snapshot
// (top 100 + the rank of every member of a school/grade scope; a grade_goal board only its collective total and whether
// the goal is reached) and leaderboard.period_closed, once per (leaderboard_id, scope_id, period_key). The job runs
// under the singleton lock of the worker.
func (h Leaderboards) ClosePeriods(ctx context.Context) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.ClosePeriod")
	defer func() { done(err) }()
	started := time.Now()
	if _, err := h.Materialize(ctx); err != nil {
		return 0, err
	}
	now := h.now()
	ps, err := authz.SystemPlatformScope(JobPeriodClose)
	if err != nil {
		return 0, err
	}
	type work struct {
		def           ldb.Definition
		key           string
		start, end    time.Time
		season        *uuid.UUID
		scopesByTenet map[uuid.UUID][]write.ScopeRef
	}
	var todo []work
	err = h.UoW.InTenantTx(ctx, ps, func(ctx context.Context) error {
		defs, err := h.Repo.Definitions(ctx)
		if err != nil {
			return err
		}
		seasons, err := h.Seasons.Seasons(ctx)
		if err != nil {
			return err
		}
		for _, d := range defs {
			if !d.Active() || d.Period == ldb.PeriodAllTime || (!d.SchoolScoped() && d.Scope != ldb.ScopeTenant) {
				continue
			}
			key, start, end, season, ok := h.closedPeriod(d, now, seasons)
			if !ok {
				continue
			}
			scopes, err := h.Repo.Scopes(ctx, d, start, end)
			if err != nil {
				return err
			}
			w := work{def: d, key: key, start: start, end: end, season: season, scopesByTenet: map[uuid.UUID][]write.ScopeRef{}}
			for _, s := range scopes {
				w.scopesByTenet[s.Tenant] = append(w.scopesByTenet[s.Tenant], s)
			}
			todo = append(todo, w)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	for _, w := range todo {
		for tenant, scopes := range w.scopesByTenet {
			k, err := h.closeTenant(ctx, w.def, tenant, scopes, w.key, w.start, w.end, w.season, now)
			n += k
			if err != nil {
				return n, err
			}
		}
	}
	if h.Metrics != nil {
		h.Metrics.PeriodClose(time.Since(started))
	}
	return n, nil
}

func (h Leaderboards) closeTenant(ctx context.Context, d ldb.Definition, tenant uuid.UUID, scopes []write.ScopeRef, key string, start,
	end time.Time, season *uuid.UUID, now time.Time) (int, error) {
	sc, err := authz.EventScope(tenant)
	if err != nil {
		return 0, err
	}
	n := 0
	err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		var prov write.BoardQuery
		if d.Metric == ldb.MetricRating {
			rules, _, err := h.Ranking.Rules(ctx, now)
			if err != nil {
				return err
			}
			prov.Provisional = rules.Provisional
		}
		seen := map[string]bool{}
		for _, s := range scopes {
			m := ldb.Member{Tenant: tenant, School: s.School, Grade: s.Grade}
			if !d.ForGrade(s.Grade) {
				continue
			}
			scope := ldb.ScopeKey(d, m)
			if seen[scope] {
				continue
			}
			seen[scope] = true
			taken, err := h.Repo.SnapshotTaken(ctx, d.ID, scope, key)
			if err != nil {
				return err
			}
			if taken {
				continue
			}
			school, grade, _ := ParseScope(d, scope)
			rows, payload, err := h.snapshot(ctx, d, tenant, scope, school, grade, key, start, end, season, prov.Provisional, now)
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				continue
			}
			if err := h.Repo.InsertSnapshot(ctx, rows); err != nil {
				return err
			}
			t := tenant
			if err := h.Events.Publish(ctx, write.Event{Type: EventPeriodClosed, SubjectType: "leaderboard", SubjectID: d.ID.String(),
				PartitionKey: d.ID.String() + ":" + scope, TenantID: &t, OccurredAt: now, Payload: payload}); err != nil {
				return err
			}
			if err := h.Audit.Record(ctx, audit.Event{Action: AuditPeriodClosed, TargetType: "leaderboard", TargetID: d.ID.String(),
				Result: audit.ResultSuccess, SchoolID: idString(school), Metadata: map[string]any{"code": d.Code, "scope_id": scope,
					"period_key": key, "rows": len(rows)}}); err != nil {
				return err
			}
			n++
		}
		return nil
	})
	return n, err
}

func idString(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

// snapshot builds the rows and the leaderboard.period_closed payload of one scope.
func (h Leaderboards) snapshot(ctx context.Context, d ldb.Definition, tenant uuid.UUID, scope string, school *uuid.UUID, grade *int, key string,
	start, end time.Time, season *uuid.UUID, prov ranking.Provisional, now time.Time) ([]write.SnapshotRow, map[string]any, error) {
	payload := map[string]any{"leaderboard_id": d.ID, "code": d.Code, "metric": d.Metric, "scope_type": d.Scope, "scope_id": scope,
		"school_id": school, "grade_level": grade, "period_key": key, "top": []map[string]any{}}
	if d.Metric == ldb.MetricGradeGoal {
		if school == nil || grade == nil {
			return nil, nil, nil
		}
		cur, members, err := h.Repo.GradeGoal(ctx, tenant, *school, *grade, start, end)
		if err != nil || members == 0 {
			return nil, nil, err
		}
		target := d.GradeGoalTarget
		reached := cur >= target
		payload["members"] = members
		payload["grade_goal"] = map[string]any{"current": cur, "target": target, "reached": reached}
		return []write.SnapshotRow{{Tenant: tenant, Board: d.ID, Scope: scope, School: school, Grade: grade, Period: key, Value: cur,
			Target: &target, Reached: &reached, Members: &members, Captured: now}}, payload, nil
	}
	ranked, err := h.Repo.Ranking(ctx, write.BoardQuery{Def: d, Tenant: tenant, School: school, Grade: grade, Start: start, End: end,
		Season: season, Provisional: prov, At: now})
	if err != nil || len(ranked) == 0 {
		return nil, nil, err
	}
	total := len(ranked)
	var rows []write.SnapshotRow
	var top []map[string]any
	for i, m := range ranked {
		rank := i + 1
		if rank > snapshotTop && !d.SchoolScoped() {
			break
		}
		student := m.Student
		rows = append(rows, write.SnapshotRow{Tenant: tenant, Board: d.ID, Scope: scope, School: school, Grade: grade, Period: key,
			Student: &student, Rank: &rank, Value: m.Value, Members: &total, Captured: now})
		if rank <= snapshotTop {
			top = append(top, map[string]any{"student_id": m.Student, "rank": rank, "value": m.Value})
		}
	}
	payload["top"] = top
	payload["members"] = total
	return rows, payload, nil
}
