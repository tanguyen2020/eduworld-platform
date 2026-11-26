package analytics

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// F080-AC-10, AC-ANL-109: wellbeing metrics exist and nothing measures "total play time" as a goal.
func TestCatalogWellbeingAndNoTotalPlaytime(t *testing.T) {
	for _, c := range []string{"sessions_over_reminder", "sessions_over_second_reminder", "late_night_play_rate", "daily_playtime_p90"} {
		if _, ok := Lookup(c); !ok {
			t.Fatalf("missing wellbeing metric %s", c)
		}
	}
	for _, m := range Catalog {
		lc := strings.ToLower(m.Code)
		if strings.Contains(lc, "total") || strings.Contains(lc, "playtime_sum") || strings.Contains(lc, "time_played") {
			t.Fatalf("metric %s looks like a total play time metric (A-2, R19)", m.Code)
		}
		if m.Unit == "seconds" && m.Kind != KindPercentile {
			t.Fatalf("duration metric %s must be a percentile", m.Code)
		}
	}
	if p, _ := Lookup("daily_playtime_p90"); p.Quantile != 0.9 {
		t.Fatal("daily_playtime_p90 is the 90th percentile")
	}
}

func TestPercentileAndMerge(t *testing.T) {
	b := []float64{60, 120, 300}
	h := Hist(b, []float64{30, 90, 100, 200, 400})
	if got := h; len(got) != 4 || got[0] != 1 || got[1] != 2 || got[2] != 1 || got[3] != 1 {
		t.Fatalf("hist %v", got)
	}
	p, ok := Percentile(b, h, 0.5)
	if !ok || p < 60 || p > 120 {
		t.Fatalf("p50 = %v", p)
	}
	if _, ok := Percentile(b, nil, 0.5); ok {
		t.Fatal("empty histogram has no percentile")
	}
	if m := MergeHist([]int64{1, 2}, []int64{3, 4, 5}); m[0] != 4 || m[1] != 6 || m[2] != 5 {
		t.Fatalf("merge %v", m)
	}
}

func TestCellValueKinds(t *testing.T) {
	dau, _ := Lookup("dau")
	c := dau.Merge(Cell{Num: 10, Den: 1}, Cell{Num: 5, Den: 1}) // two tenants, same day
	if v := dau.Value(c); v == nil || *v != 15 {
		t.Fatalf("dau across tenants = %v", v)
	}
	if v := dau.Value(Cell{Num: 70, Den: 7}); *v != 10 {
		t.Fatalf("weekly dau average = %v", *v)
	}
	r, _ := Lookup("quest_completion_rate")
	if v := r.Value(Cell{Num: 1, Den: 0}); v != nil {
		t.Fatal("empty denominator is undefined")
	}
	if v := r.Value(r.Merge(Cell{Num: 1, Den: 4}, Cell{Num: 1, Den: 4})); *v != 0.25 {
		t.Fatalf("ratio = %v", *v)
	}
	mau, _ := Lookup("mau")
	if v := mau.Value(Cell{Num: 42, Den: 0}); *v != 42 {
		t.Fatalf("mau = %v", *v)
	}
}

// F080-AC-16: a small cell is hidden and, alone, it takes the smallest visible cell with it.
func TestSuppressComplementary(t *testing.T) {
	h := Suppress([]Population{{"a", 3}, {"b", 20}, {"c", 7}}, 5)
	if !h["a"] || !h["c"] || h["b"] {
		t.Fatalf("hidden = %v", h)
	}
	h = Suppress([]Population{{"a", 3}, {"b", 2}, {"c", 7}}, 5)
	if !h["a"] || !h["b"] || h["c"] {
		t.Fatalf("two small cells need no complement: %v", h)
	}
	if h := Suppress([]Population{{"a", 3}}, 5); !h["a"] {
		t.Fatal("single small cell")
	}
}

func TestFilter(t *testing.T) {
	f, err := NewFilter(map[string][]string{DimPlatform: {"web"}})
	if err != nil || f.Key != DimPlatform || !f.Matches(Dims{DimPlatform: "web"}) || f.Matches(Dims{DimPlatform: "ios"}) {
		t.Fatalf("filter %v %v", f, err)
	}
	if _, err := NewFilter(map[string][]string{DimPlatform: {"web"}, DimLocale: {"vi-VN"}}); !errors.Is(err, ErrTwoDimensions) {
		t.Fatal("two dimensions must fail")
	}
	f, err = NewFilter(map[string][]string{DimSchool: {"s1"}, DimGradeBand: {"g1_3"}})
	if err != nil || !f.Matches(Dims{DimSchool: "s1", DimGradeBand: "g1_3"}) {
		t.Fatalf("school+grade %v %v", f, err)
	}
	if (Dims{}).Hash() == (Dims{DimSchool: Unknown}).Hash() {
		t.Fatal("whole tenant and unknown school must differ")
	}
	if (Dims{"b": "1", "a": "2"}).Key() != "a=2&b=1" {
		t.Fatal("canonical key")
	}
}

// F080-AC-01, AC-34: closed attributes per type, reminder ranges.
func TestValidateTelemetry(t *testing.T) {
	if err := ValidateTelemetry(0, "break_reminder_shown", map[string]any{"type": "break_reminder_shown", "occurred_at": "x",
		"reminder_no": 2.0, "minutes_played": 90.0}); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []map[string]any{
		{"reminder_no": 0.0, "minutes_played": 1.0},
		{"reminder_no": 21.0, "minutes_played": 1.0},
		{"reminder_no": 1.0, "minutes_played": 1441.0},
		{"reminder_no": 1.0, "minutes_played": 1.0, "fps_p50": 3.0},
	} {
		if err := ValidateTelemetry(3, "break_reminder_shown", bad); err == nil {
			t.Fatalf("%v must fail", bad)
		}
	}
	if err := ValidateTelemetry(1, "screen_view", map[string]any{"screen_code": "x", "student_name": "y"}); err == nil {
		t.Fatal("unknown attribute must fail")
	}
	if p := TelemetryProps("crash", map[string]any{"error_code": "E", "crash_signature": "sig"}); p["crash_signature"] != nil {
		t.Fatal("crash_signature never goes to analytics (F080-AC-04)")
	}
}

// F080-AC-03, AC-34, edge cases.
func TestDropTelemetry(t *testing.T) {
	c := DefaultConfig()
	now := time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)
	if DropTelemetry("screen_view", now, now, false, false, c) != DropProductOff {
		t.Fatal("screen_view dropped when product telemetry is off")
	}
	for _, k := range []string{"perf_sample", "crash", "break_reminder_shown", "load_time", "client_error"} {
		if DropTelemetry(k, now, now, false, false, c) != Keep {
			t.Fatalf("%s kept when product telemetry is off", k)
		}
	}
	if DropTelemetry("perf_sample", now.Add(6*time.Minute), now, true, false, c) != DropFuture {
		t.Fatal("future")
	}
	if DropTelemetry("perf_sample", now.Add(-49*time.Hour), now, true, false, c) != DropLate {
		t.Fatal("late")
	}
	if DropTelemetry("perf_sample", now, now, true, true, c) != DropQATenant {
		t.Fatal("qa")
	}
	if DropTelemetry("network_sample", now, now, true, false, c) != DropNotInCatalog {
		t.Fatal("network_sample is not in the catalog")
	}
}

// F080-AC-05: mapping keeps catalog keys only, one row per student of a multi-student event.
func TestMap(t *testing.T) {
	st, gs := uuid.New(), uuid.New()
	end := time.Date(2026, 11, 25, 16, 0, 0, 0, time.UTC) // 23:00 ICT
	out := Map("game_session.ended", map[string]any{"student_id": st.String(), "game_session_id": gs.String(), "platform": "web",
		"duration_ms": 1_800_000.0, "end_reason": "logout", "ended_at": end.Format(time.RFC3339)}, DefaultConfig().LateNight)
	if len(out) != 1 || out[0].Props["active_seconds"] != 1800.0 || out[0].Props["late_night"] != true || *out[0].SessionID != gs {
		t.Fatalf("session_end %+v", out)
	}
	if _, ok := out[0].Props["student_id"]; ok {
		t.Fatal("student_id never in props")
	}
	a, b := uuid.New(), uuid.New()
	out = Map("match.completed", map[string]any{"mode": "team_battle", "matchmaking_type": "team", "status": "completed",
		"results": []any{map[string]any{"student_id": a.String(), "outcome": "win", "score": 9.0},
			map[string]any{"student_id": b.String(), "outcome": "lose"}}}, Window{})
	if len(out) != 2 || out[0].Props["score"] != nil || out[0].Props["social"] != true {
		t.Fatalf("match %+v", out)
	}
	if Map("activity.submitted", map[string]any{"student_id": st.String(), "context_type": "assignment"}, Window{}) != nil {
		t.Fatal("assignments are not game activity")
	}
	if Map("unknown.event", map[string]any{"student_id": st.String()}, Window{}) != nil {
		t.Fatal("unknown events map to nothing")
	}
}

func TestKeysAndIDs(t *testing.T) {
	if _, err := NewKeyRing(Key{Version: 1, Secret: []byte("short")}); err == nil {
		t.Fatal("short secret")
	}
	r, err := NewKeyRing(Key{Version: 1, Secret: make([]byte, 32)}, Key{Version: 2, Secret: []byte(strings.Repeat("k", 32))})
	if err != nil || r.Current().Version != 2 || len(r.All()) != 2 {
		t.Fatalf("ring %v %v", r, err)
	}
	id := uuid.New()
	pk := Pseudonym(r.Current(), id)
	if len(pk) != 64 || strings.Contains(pk, id.String()) || pk == Pseudonym(r.All()[1], id) {
		t.Fatal("pseudonym")
	}
	ce := uuid.New()
	first, again, other := ClientEventID("p1", ce), ClientEventID("p1", ce), ClientEventID("p2", ce)
	if first != again || first == other {
		t.Fatal("client event ids dedup per player")
	}
}

// F080-AC-09 late rule and the late night window.
func TestLateAndWindow(t *testing.T) {
	now := time.Date(2026, 11, 26, 3, 0, 0, 0, time.UTC)
	w := 48 * time.Hour
	if Late(now.Add(-30*time.Hour), now, w, true) != ApplyReclose || Late(now.Add(-50*time.Hour), now, w, true) != Drop ||
		Late(now.Add(-time.Hour), now, w, false) != Apply {
		t.Fatal("late")
	}
	win := DefaultConfig().LateNight
	day := time.Date(2026, 11, 25, 3, 0, 0, 0, time.UTC) // 10:00 ICT
	if win.Intersects(day, day.Add(time.Hour)) {
		t.Fatal("morning is not late night")
	}
	if !win.Intersects(day.Add(13*time.Hour), day.Add(13*time.Hour+30*time.Minute)) { // 23:00 ICT
		t.Fatal("23:00 is late night")
	}
}

func TestRangeGrain(t *testing.T) {
	r := DefaultConfig().Range
	d := func(s string) time.Time { x, _ := time.Parse(time.DateOnly, s); return x }
	for _, c := range []struct {
		from, to string
		g        Grain
	}{{"2026-01-01", "2026-01-07", Day}, {"2026-01-01", "2026-03-31", Week}, {"2025-01-01", "2026-12-30", Month}} {
		rg, err := NewRange(d(c.from), d(c.to), "", r)
		if err != nil || rg.Grain != c.g || len(rg.Buckets()) > MaxPoints {
			t.Fatalf("%v %v %v", c, rg, err)
		}
	}
	if _, err := NewRange(d("2024-01-01"), d("2026-01-01"), "", r); !errors.Is(err, ErrRangeTooLarge) {
		t.Fatal("731 days")
	}
	if _, err := NewRange(d("2026-01-01"), d("2026-03-31"), Day, r); !errors.Is(err, ErrGrainTooFine) {
		t.Fatal("finer grain")
	}
	if GradeBand(3) != "g1_3" || GradeBand(9) != "g8_9" || GradeBand(0) != Unknown {
		t.Fatal("bands")
	}
}
