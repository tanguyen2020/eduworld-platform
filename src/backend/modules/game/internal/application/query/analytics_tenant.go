package query

import (
	"context"
	"encoding/json"
	"slices"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/analytics"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// TenantSummaryInput is the filter of getTenantGameSummary.
type TenantSummaryInput struct {
	From, To   time.Time
	Grain      string
	SchoolIDs  []uuid.UUID
	GradeBands []string
}

// SeriesPoint is one point of active_students_series.
type SeriesPoint struct {
	T     string `json:"t"`
	Value *int64 `json:"value"`
}

// ShareView is one key of a distribution.
type ShareView struct {
	Key   string   `json:"key"`
	Share *float64 `json:"share"`
}

// SchoolRowView is one row of by_school.
type SchoolRowView struct {
	SchoolID            uuid.UUID `json:"school_id"`
	ActiveStudents      *int64    `json:"active_students"`
	QuestCompletionRate *float64  `json:"quest_completion_rate"`
}

// BandRowView is one row of by_grade_band.
type BandRowView struct {
	GradeBand           string   `json:"grade_band"`
	ActiveStudents      *int64   `json:"active_students"`
	QuestCompletionRate *float64 `json:"quest_completion_rate"`
}

// TenantSummaryView is TenantGameSummary (GAME_ANALYTICS §7.2): aggregates of the session tenant only — no economy
// detail, no student list, no total play time (R16, R19); every number may be null when its cell has fewer than
// analytics_min_cell_size students.
type TenantSummaryView struct {
	TenantID                       uuid.UUID       `json:"tenant_id"`
	From                           string          `json:"from"`
	To                             string          `json:"to"`
	ActiveStudents                 *int64          `json:"active_students"`
	DAUAvg                         *float64        `json:"dau_avg"`
	RetentionD7                    *float64        `json:"retention_d7"`
	QuestCompletionRate            *float64        `json:"quest_completion_rate"`
	EventParticipationRate         *float64        `json:"event_participation_rate"`
	SessionsOverReminderRate       *float64        `json:"sessions_over_reminder_rate"`
	ActiveStudentsSeries           []SeriesPoint   `json:"active_students_series"`
	ActiveDays3PlusRate            *float64        `json:"active_days_3plus_rate"`
	SessionDurationP50Seconds      *float64        `json:"session_duration_p50_seconds"`
	Chapter0CompletionRate         *float64        `json:"chapter0_completion_rate"`
	SocialPlayRate                 *float64        `json:"social_play_rate"`
	SessionsOverSecondReminderRate *float64        `json:"sessions_over_second_reminder_rate"`
	LateNightPlayRate              *float64        `json:"late_night_play_rate"`
	PlatformDistribution           []ShareView     `json:"platform_distribution"`
	DeviceTierDistribution         []ShareView     `json:"device_tier_distribution"`
	ByGradeBand                    []BandRowView   `json:"by_grade_band"`
	BySchool                       []SchoolRowView `json:"by_school"`
	SuppressedCells                int             `json:"suppressed_cells"`
	Meta
}

var gradeBands = []string{"g1_3", "g4_5", "g6_7", "g8_9"}
var platforms = []string{"windows", "android", "ios", "web", "macos", "linux"}

// summaryCodes are the metrics read by the tenant summary.
var summaryCodes = []string{"dau", "wau", "mau", "quest_completion_rate", "event_participation_rate", "sessions_over_reminder",
	"sessions_over_second_reminder", "late_night_play_rate", "session_duration_p50", "chapter0_completion"}

var weeklyCodes = []string{"active_days_per_week", "social_play_rate"}

// tenantScope resolves the schools a staff member reads (F080-AC-15, AC-32): analytics.tenant.read on the tenant → every
// school; analytics.school.read → the schools of the grants; a grant limited to a subject is not enough (game data has
// no subject dimension → 403 PERMISSION_DENIED); no grant → 403. nil = the whole tenant.
func tenantScope(ctx context.Context, p authz.Principal, tenant uuid.UUID) ([]uuid.UUID, error) {
	var schools []uuid.UUID
	subjectOnly := false
	for _, g := range p.Grants {
		tenantRead := slices.Contains(g.Permissions, authz.PermAnalyticsTenantRead)
		schoolRead := slices.Contains(g.Permissions, authz.PermAnalyticsSchoolRead)
		if !tenantRead && !schoolRead {
			continue
		}
		if g.SubjectID != nil {
			subjectOnly = true
			continue
		}
		if g.SchoolID == nil && (tenantRead || schoolRead) {
			return nil, nil
		}
		if g.SchoolID != nil {
			schools = append(schools, *g.SchoolID)
		}
	}
	if len(schools) > 0 {
		return schools, nil
	}
	if subjectOnly {
		authz.RecordDenied(ctx, string(problem.PermissionDenied), map[string]any{"why": "subject_scope", "permission": string(authz.PermAnalyticsSchoolRead)})
		return nil, problem.New(problem.PermissionDenied, "a subject-scoped role does not cover game reports")
	}
	return nil, authz.Require(ctx, p, authz.PermAnalyticsTenantRead, authz.Resource{TenantID: &tenant})
}

// GetTenantSummary is getTenantGameSummary (F080-AC-15, AC-16, AC-31, AC-32): the session tenant only, schools in the
// scope of the caller (a school of another tenant → 404, of the tenant but outside the grants → 403), every field from the
// rollup tables (never events_raw/player_days) with complementary small-cell suppression; cached per tenant data version.
func (q GameAnalytics) GetTenantSummary(ctx context.Context, in TenantSummaryInput) (out TenantSummaryView, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.GetTenantGameSummary")
	defer func() { done(err) }()
	p, tenant, err := staffTenant(ctx)
	if err != nil {
		return out, err
	}
	scope, err := tenantScope(ctx, p, tenant)
	if err != nil {
		return out, err
	}
	rg, err := q.dashRange(in.From, in.To, in.Grain)
	if err != nil {
		return out, err
	}
	for i, b := range in.GradeBands {
		if !slices.Contains(gradeBands, b) {
			return out, problem.Invalid("grade_bands/"+itoa(i), problem.SubInvalid, "g1_3, g4_5, g6_7 or g8_9")
		}
	}
	key := cacheKey(tenant, 0, "", nil)
	var cached bool
	err = q.run(ctx, "tenant_game_summary", p.UserID, tenant, func(ctx context.Context) error {
		if len(in.SchoolIDs) > 0 {
			found, err := q.Model.SchoolsInTenant(ctx, tenant, in.SchoolIDs)
			if err != nil {
				return err
			}
			for _, s := range in.SchoolIDs {
				if !slices.Contains(found, s) {
					return problem.New(problem.NotFound, "")
				}
				if scope != nil && !slices.Contains(scope, s) {
					_ = authz.Require(ctx, p, authz.PermAnalyticsSchoolRead, authz.Resource{TenantID: &tenant, SchoolID: &s})
					return problem.New(problem.PermissionDenied, "")
				}
			}
		}
		schools := in.SchoolIDs
		if len(schools) == 0 {
			schools = scope
		}
		if q.Cache != nil {
			if ver, err := q.Cache.Version(ctx, tenant); err == nil {
				key = cacheKey(tenant, ver, "tenant_game_summary", rg, schools, in.GradeBands)
				if b, ok, err := q.Cache.Get(ctx, key); err == nil && ok && json.Unmarshal(b, &out) == nil {
					cached = true
					return nil
				}
			}
		}
		out, err = q.tenantSummary(ctx, tenant, rg, schools, in.GradeBands)
		return err
	})
	if err == nil && !cached && q.Cache != nil {
		ttl := q.Config.CacheTTLClosed
		if !rg.To.Before(analytics.DayOf(q.now())) {
			ttl = q.Config.CacheTTLToday
		}
		if b, merr := json.Marshal(out); merr == nil {
			_ = q.Cache.Set(ctx, key, b, ttl)
		}
	}
	return out, err
}

// scopeReader merges rollup rows of one dimension set for the selected values with suppression over the whole set.
type scopeReader struct {
	q      GameAnalytics
	cells  map[cellKey]analytics.Cell
	dims   map[string]analytics.Dims
	hidden map[cellKey]bool
	flt    analytics.Filter
	macc   *metaAcc
}

func (q GameAnalytics) scope(ctx context.Context, tenant uuid.UUID, rg analytics.Range, g analytics.Grain, codes []string, flt analytics.Filter,
	macc *metaAcc) (scopeReader, error) {
	cells, dims, err := q.merged(ctx, []uuid.UUID{tenant}, rg, g, codes, flt.Key, macc)
	if err != nil {
		return scopeReader{}, err
	}
	return scopeReader{q: q, cells: cells, dims: dims, hidden: q.hiddenCells(cells), flt: flt, macc: macc}, nil
}

// total merges the selected values of code over the buckets; hidden when any merged cell is hidden or the total is small.
func (s scopeReader) total(code string, buckets []time.Time) (analytics.Cell, bool) {
	m, _ := analytics.Lookup(code)
	var out analytics.Cell
	any := false
	for _, b := range buckets {
		var bc analytics.Cell
		for dk, d := range s.dims {
			if !s.flt.Matches(d) && s.flt.Key != "" {
				continue
			}
			if s.flt.Key == "" && dk != "" {
				continue
			}
			k := cellKey{date: b, code: code, dims: dk}
			c, ok := s.cells[k]
			if !ok {
				continue
			}
			if s.hidden[k] {
				return out, true
			}
			bc = m.Merge(bc, c)
			any = true
		}
		if m.Kind == analytics.KindCount {
			out.Num += bc.Num
			out.Den += max(bc.Den, 0)
			out.Players = max(out.Players, bc.Players)
		} else {
			p := max(out.Players, bc.Players)
			out = m.Merge(out, bc)
			out.Players = p
		}
	}
	if !any {
		return out, false
	}
	return out, analytics.Small(out.Players, s.q.Config.MinCellSize) && m.Kind != analytics.KindSum
}

func (q GameAnalytics) tenantSummary(ctx context.Context, tenant uuid.UUID, rg analytics.Range, schools []uuid.UUID, bands []string) (TenantSummaryView, error) {
	out := TenantSummaryView{TenantID: tenant, From: rg.From.Format(time.DateOnly), To: rg.To.Format(time.DateOnly),
		ActiveStudentsSeries: []SeriesPoint{}, PlatformDistribution: []ShareView{}, DeviceTierDistribution: []ShareView{},
		ByGradeBand: []BandRowView{}, BySchool: []SchoolRowView{}}
	var macc metaAcc
	schoolVals := make([]string, len(schools))
	for i, s := range schools {
		schoolVals[i] = s.String()
	}
	flt, err := analytics.NewFilter(map[string][]string{analytics.DimSchool: schoolVals, analytics.DimGradeBand: bands})
	if err != nil {
		return out, problem.Invalid("dimension", problem.SubInvalid, err.Error())
	}
	buckets := rg.Buckets()
	sc, err := q.scope(ctx, tenant, rg, rg.Grain, summaryCodes, flt, &macc)
	if err != nil {
		return out, err
	}
	hide := func() { out.SuppressedCells++ }
	ratio := func(code string) *float64 {
		c, hidden := sc.total(code, buckets)
		if hidden {
			hide()
			return nil
		}
		m, _ := analytics.Lookup(code)
		return m.Value(c)
	}
	out.DAUAvg = ratio("dau")
	out.QuestCompletionRate = ratio("quest_completion_rate")
	out.EventParticipationRate = ratio("event_participation_rate")
	out.SessionsOverReminderRate = ratio("sessions_over_reminder")
	out.SessionsOverSecondReminderRate = ratio("sessions_over_second_reminder")
	out.LateNightPlayRate = ratio("late_night_play_rate")
	out.SessionDurationP50Seconds = ratio("session_duration_p50")
	out.Chapter0CompletionRate = ratio("chapter0_completion")
	// Weekly-only metrics from the weekly (or monthly) table.
	wg := rg.Grain
	if wg == analytics.Day {
		wg = analytics.Week
	}
	wr := analytics.Range{From: rg.From, To: rg.To, Grain: wg}
	wsc, err := q.scope(ctx, tenant, wr, wg, weeklyCodes, flt, &macc)
	if err != nil {
		return out, err
	}
	wbuckets := wr.Buckets()
	for _, c := range []struct {
		code string
		dst  **float64
	}{{"active_days_per_week", &out.ActiveDays3PlusRate}, {"social_play_rate", &out.SocialPlayRate}} {
		cell, hidden := wsc.total(c.code, wbuckets)
		if hidden {
			hide()
			continue
		}
		m, _ := analytics.Lookup(c.code)
		*c.dst = m.Value(cell)
	}
	// active_students_series: dau / wau / mau of each bucket; active_students: mau at the last day (≤ today).
	seriesCode := map[analytics.Grain]string{analytics.Day: "dau", analytics.Week: "wau", analytics.Month: "mau"}[rg.Grain]
	for _, b := range buckets {
		pt := SeriesPoint{T: b.Format(time.DateOnly)}
		if c, hidden := sc.total(seriesCode, []time.Time{b}); hidden {
			hide()
		} else if c.Num > 0 || c.Players > 0 {
			v := c.Num
			pt.Value = &v
		}
		out.ActiveStudentsSeries = append(out.ActiveStudentsSeries, pt)
	}
	last := rg.To
	if today := analytics.DayOf(q.now()); last.After(today) {
		last = today
	}
	lr := analytics.Range{From: last, To: last, Grain: analytics.Day}
	day, err := q.scope(ctx, tenant, lr, analytics.Day, []string{"mau", "dau"}, flt, &macc)
	if err != nil {
		return out, err
	}
	if c, hidden := day.total("mau", []time.Time{last}); hidden {
		hide()
	} else if c.Num > 0 {
		v := c.Num
		out.ActiveStudents = &v
	}
	// retention_d7 over the cohorts of the range.
	rrows, err := q.Model.Retention(ctx, port.DashboardQuery{Tenants: []uuid.UUID{tenant}, From: rg.From, To: rg.To, Grain: analytics.Day}, flt.Key, flt.Values)
	if err != nil {
		return out, err
	}
	var size, kept int64
	for _, r := range rrows {
		if r.DayN == 7 {
			size, kept = size+r.Size, kept+r.Retained
			macc.add(r.Computed, r.Final)
		}
	}
	if size > 0 {
		if analytics.Small(size, q.Config.MinCellSize) {
			hide()
		} else {
			v := float64(kept) / float64(size)
			out.RetentionD7 = &v
		}
	}
	// Breakdowns: schools, grade bands, platforms, device tiers (whole dimension suppression, never a student list).
	if err := q.breakdowns(ctx, tenant, rg, schools, bands, &out, &macc); err != nil {
		return out, err
	}
	af, err := q.Model.AvailableFrom(ctx, []uuid.UUID{tenant})
	if err != nil {
		return out, err
	}
	out.Meta = q.meta(rg, macc, af)
	return out, nil
}

// group reads one dimension set over the range and returns per dims value: mau at the last day (population, suppressed
// over the whole set) and the merged cells of the other codes.
func (q GameAnalytics) group(ctx context.Context, tenant uuid.UUID, rg analytics.Range, set string, codes []string, macc *metaAcc,
	keep func(analytics.Dims) bool) (map[string]analytics.Dims, map[string]map[string]analytics.Cell, map[string]bool, map[string]int64, error) {
	flt := analytics.Filter{Key: set}
	sc, err := q.scope(ctx, tenant, rg, rg.Grain, codes, flt, macc)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	last := rg.To
	if today := analytics.DayOf(q.now()); last.After(today) {
		last = today
	}
	day, err := q.scope(ctx, tenant, analytics.Range{From: last, To: last, Grain: analytics.Day}, analytics.Day, []string{"mau"}, flt, macc)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	pops := []analytics.Population{}
	mau := map[string]int64{}
	for k, c := range day.cells {
		if keep(day.dims[k.dims]) {
			pops = append(pops, analytics.Population{Key: k.dims, Players: c.Num})
			mau[k.dims] = c.Num
		}
	}
	hidden := analytics.Suppress(pops, q.Config.MinCellSize)
	cells := map[string]map[string]analytics.Cell{}
	dims := map[string]analytics.Dims{}
	for k, c := range sc.cells {
		d := sc.dims[k.dims]
		if !keep(d) {
			continue
		}
		m, _ := analytics.Lookup(k.code)
		if cells[k.dims] == nil {
			cells[k.dims] = map[string]analytics.Cell{}
		}
		cells[k.dims][k.code] = m.Merge(cells[k.dims][k.code], c)
		dims[k.dims] = d
	}
	for k, d := range day.dims {
		if keep(d) {
			dims[k] = d
		}
	}
	return dims, cells, hidden, mau, nil
}

func (q GameAnalytics) breakdowns(ctx context.Context, tenant uuid.UUID, rg analytics.Range, schools []uuid.UUID, bands []string,
	out *TenantSummaryView, macc *metaAcc) error {
	inSchools := func(d analytics.Dims) bool {
		if len(schools) == 0 {
			return true
		}
		return slices.ContainsFunc(schools, func(s uuid.UUID) bool { return s.String() == d[analytics.DimSchool] })
	}
	inBands := func(d analytics.Dims) bool {
		return len(bands) == 0 || slices.Contains(bands, d[analytics.DimGradeBand])
	}
	qcr, _ := analytics.Lookup("quest_completion_rate")
	// by_school (no grade filter: the school set; with one: the (school, grade_band) set merged per school).
	set := analytics.DimSchool
	if len(bands) > 0 {
		set = analytics.DimSchool + "," + analytics.DimGradeBand
	}
	dims, cells, hidden, mau, err := q.group(ctx, tenant, rg, set, []string{"quest_completion_rate"}, macc,
		func(d analytics.Dims) bool { return inSchools(d) && inBands(d) })
	if err != nil {
		return err
	}
	type schoolAcc struct {
		active int64
		qc     analytics.Cell
		hidden bool
	}
	bySchool := map[string]*schoolAcc{}
	for k, d := range dims {
		s := d[analytics.DimSchool]
		if _, err := uuid.Parse(s); err != nil {
			continue
		}
		a := bySchool[s]
		if a == nil {
			a = &schoolAcc{}
			bySchool[s] = a
		}
		a.hidden = a.hidden || hidden[k]
		a.active += mau[k]
		a.qc = qcr.Merge(a.qc, cells[k]["quest_completion_rate"])
	}
	keys := make([]string, 0, len(bySchool))
	for k := range bySchool {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		a := bySchool[k]
		row := SchoolRowView{SchoolID: uuid.MustParse(k)}
		if a.hidden || analytics.Small(a.active, q.Config.MinCellSize) {
			out.SuppressedCells += 2
		} else {
			v := a.active
			row.ActiveStudents = &v
			row.QuestCompletionRate = qcr.Value(a.qc)
		}
		out.BySchool = append(out.BySchool, row)
	}
	// by_grade_band (the grade set, or (school, grade_band) merged per band when schools are filtered).
	set = analytics.DimGradeBand
	if len(schools) > 0 {
		set = analytics.DimSchool + "," + analytics.DimGradeBand
	}
	dims, cells, hidden, mau, err = q.group(ctx, tenant, rg, set, []string{"quest_completion_rate"}, macc,
		func(d analytics.Dims) bool { return inSchools(d) && inBands(d) })
	if err != nil {
		return err
	}
	for _, b := range gradeBands {
		var active int64
		var qc analytics.Cell
		hid, seen := false, false
		for k, d := range dims {
			if d[analytics.DimGradeBand] != b {
				continue
			}
			seen = true
			hid = hid || hidden[k]
			active += mau[k]
			qc = qcr.Merge(qc, cells[k]["quest_completion_rate"])
		}
		if !seen {
			continue
		}
		row := BandRowView{GradeBand: b}
		if hid || analytics.Small(active, q.Config.MinCellSize) {
			out.SuppressedCells += 2
		} else {
			v := active
			row.ActiveStudents = &v
			row.QuestCompletionRate = qcr.Value(qc)
		}
		out.ByGradeBand = append(out.ByGradeBand, row)
	}
	// Distributions: shares of the player-days (dau) per platform / device tier; not available with a grade filter.
	if len(bands) > 0 {
		return nil
	}
	for _, dist := range []struct {
		key  string
		dst  *[]ShareView
		vals []string
	}{{analytics.DimPlatform, &out.PlatformDistribution, platforms}, {analytics.DimDeviceTier, &out.DeviceTierDistribution, []string{"minimum", "mid", "high"}}} {
		set := dist.key
		if len(schools) > 0 {
			set = analytics.DimSchool + "," + dist.key
		}
		dims, cells, hidden, _, err := q.group(ctx, tenant, rg, set, []string{"dau"}, macc, inSchools)
		if err != nil {
			return err
		}
		perKey := map[string]int64{}
		hid := map[string]bool{}
		var total int64
		for k, d := range dims {
			v := d[dist.key]
			perKey[v] += cells[k]["dau"].Num
			total += cells[k]["dau"].Num
			hid[v] = hid[v] || hidden[k]
		}
		if total == 0 {
			continue
		}
		for _, v := range dist.vals {
			n, ok := perKey[v]
			if !ok {
				continue
			}
			sv := ShareView{Key: v}
			if hid[v] {
				out.SuppressedCells++
			} else {
				s := float64(n) / float64(total)
				sv.Share = &s
			}
			*dist.dst = append(*dist.dst, sv)
		}
	}
	return nil
}

func itoa(i int) string {
	b, _ := json.Marshal(i)
	return string(b)
}
