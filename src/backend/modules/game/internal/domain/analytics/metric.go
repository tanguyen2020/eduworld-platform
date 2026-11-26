package analytics

import "sort"

// Kind is how a metric aggregates across days, dimension values and tenants (DASHBOARD_DATA §3.2).
type Kind string

// Kinds of the catalog.
const (
	// KindCount: value = numerator (distinct players of the bucket, e.g. dau); week/month value = average of the days.
	KindCount Kind = "count"
	// KindLast: rolling distinct count (wau, mau) or ratio of a window (stickiness); week/month value = last day.
	KindLast Kind = "last"
	// KindRatio: value = Σ numerator / Σ denominator.
	KindRatio Kind = "ratio"
	// KindSum: value = Σ numerator (Xu flows).
	KindSum Kind = "sum"
	// KindPercentile: value interpolated from the merged histogram (hist) at Quantile.
	KindPercentile Kind = "percentile"
	// KindWeekly: ratio computed per ISO week and month from player_days (no daily row).
	KindWeekly Kind = "weekly"
)

// Metric is one entry of the catalog of GAME_ANALYTICS §4 (A-7: defined once; changing the formula bumps Version).
type Metric struct {
	Code     string
	Version  int
	Kind     Kind
	Unit     string    // "players", "ratio", "seconds", "xu", "level", "sessions"
	Quantile float64   // KindPercentile only
	Bounds   []float64 // histogram upper bounds (KindPercentile)
	// Extra is the extra dimension key the metric is also cut by (currency reason, event), besides the standard sets.
	Extra string
}

// Histogram bounds (seconds and levels) — fixed buckets, DASHBOARD_DATA §3.2.
var (
	secondsBounds = []float64{60, 120, 300, 600, 900, 1200, 1800, 2700, 3600, 5400, 7200, 10800, 14400, 21600}
	levelBounds   = []float64{1, 2, 3, 5, 8, 10, 15, 20, 30, 40, 50}
)

// Catalog is the metric catalog implemented by the rollup jobs (F080-AC-08, AC-10, AC-35). There is deliberately no
// "total play time" metric: duration metrics are a median and a 90th percentile used to detect over-play (A-2, R19).
var Catalog = []Metric{
	{Code: "dau", Version: 1, Kind: KindCount, Unit: "players"},
	{Code: "wau", Version: 1, Kind: KindLast, Unit: "players"},
	{Code: "mau", Version: 1, Kind: KindLast, Unit: "players"},
	{Code: "stickiness", Version: 1, Kind: KindLast, Unit: "ratio"},
	{Code: "sessions_per_player_day", Version: 1, Kind: KindRatio, Unit: "sessions"},
	{Code: "session_duration_p50", Version: 1, Kind: KindPercentile, Unit: "seconds", Quantile: 0.5, Bounds: secondsBounds},
	{Code: "active_days_per_week", Version: 1, Kind: KindWeekly, Unit: "ratio"},
	{Code: "chapter0_completion", Version: 1, Kind: KindRatio, Unit: "ratio"},
	{Code: "quest_completion_rate", Version: 1, Kind: KindRatio, Unit: "ratio"},
	{Code: "challenge_success_rate", Version: 1, Kind: KindRatio, Unit: "ratio"},
	{Code: "boss_defeat_rate", Version: 1, Kind: KindRatio, Unit: "ratio"},
	{Code: "abandon_rate", Version: 1, Kind: KindRatio, Unit: "ratio"},
	{Code: "chapter_completion_rate", Version: 1, Kind: KindRatio, Unit: "ratio"},
	{Code: "level_distribution", Version: 1, Kind: KindPercentile, Unit: "level", Quantile: 0.5, Bounds: levelBounds},
	{Code: "xp_cap_hit_rate", Version: 1, Kind: KindRatio, Unit: "ratio"},
	{Code: "social_play_rate", Version: 1, Kind: KindWeekly, Unit: "ratio"},
	{Code: "party_size_avg", Version: 1, Kind: KindRatio, Unit: "players"},
	{Code: "match_participation_rate", Version: 1, Kind: KindWeekly, Unit: "ratio"},
	{Code: "match_completion_rate", Version: 1, Kind: KindRatio, Unit: "ratio"},
	{Code: "event_participation_rate", Version: 1, Kind: KindRatio, Unit: "ratio", Extra: "event_id"},
	{Code: "chat_activity", Version: 1, Kind: KindRatio, Unit: "ratio"},
	{Code: "currency_sources", Version: 1, Kind: KindSum, Unit: "xu", Extra: "reason_code"},
	{Code: "currency_sinks", Version: 1, Kind: KindSum, Unit: "xu", Extra: "reason_code"},
	{Code: "currency_net_flow", Version: 1, Kind: KindSum, Unit: "xu"},
	{Code: "currency_cap_hit_rate", Version: 1, Kind: KindRatio, Unit: "ratio"},
	{Code: "shop_conversion", Version: 1, Kind: KindWeekly, Unit: "ratio"},
	{Code: "sessions_over_reminder", Version: 1, Kind: KindRatio, Unit: "ratio"},
	{Code: "sessions_over_second_reminder", Version: 1, Kind: KindRatio, Unit: "ratio"},
	{Code: "late_night_play_rate", Version: 1, Kind: KindRatio, Unit: "ratio"},
	{Code: "daily_playtime_p90", Version: 1, Kind: KindPercentile, Unit: "seconds", Quantile: 0.9, Bounds: secondsBounds},
}

var byCode = func() map[string]Metric {
	m := make(map[string]Metric, len(Catalog))
	for _, x := range Catalog {
		m[x.Code] = x
	}
	return m
}()

// Lookup returns the metric of code.
func Lookup(code string) (Metric, bool) {
	m, ok := byCode[code]
	return m, ok
}

// Codes lists every metric code (sorted).
func Codes() []string {
	out := make([]string, 0, len(Catalog))
	for _, m := range Catalog {
		out = append(out, m.Code)
	}
	sort.Strings(out)
	return out
}

// Cell is one aggregated cell of a metric: numerator, denominator, distinct players of the population (suppression) and
// histogram (percentile metrics). Daily KindCount rows carry Den = 1; weekly/monthly ones the number of days merged.
type Cell struct {
	Num, Den int64
	Players  int64
	Hist     []int64
}

// Merge adds b to a across dimension values or tenants of the same bucket (KindCount keeps the number of days).
func (m Metric) Merge(a, b Cell) Cell {
	out := Cell{Num: a.Num + b.Num, Den: a.Den + b.Den, Players: a.Players + b.Players, Hist: MergeHist(a.Hist, b.Hist)}
	if m.Kind == KindCount {
		out.Den = max(a.Den, b.Den)
	}
	return out
}

// Value computes the value of a cell; nil when undefined (empty denominator, empty histogram).
func (m Metric) Value(c Cell) *float64 {
	var v float64
	switch {
	case m.Kind == KindCount:
		v = float64(c.Num)
		if c.Den > 1 {
			v /= float64(c.Den)
		}
	case m.Kind == KindSum, m.Kind == KindLast && m.Unit == "players":
		v = float64(c.Num)
	case m.Kind == KindRatio, m.Kind == KindWeekly, m.Kind == KindLast:
		if c.Den <= 0 {
			return nil
		}
		v = float64(c.Num) / float64(c.Den)
	case m.Kind == KindPercentile:
		p, ok := Percentile(m.Bounds, c.Hist, m.Quantile)
		if !ok {
			return nil
		}
		v = p
	default:
		return nil
	}
	return &v
}
