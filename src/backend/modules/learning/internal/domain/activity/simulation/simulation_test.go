package simulation

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

type vectorFile struct {
	TemplateCode    string  `json:"template_code"`
	TemplateVersion int     `json:"template_version"`
	G               float64 `json:"g"`
	Vectors         []struct {
		Name       string          `json:"name"`
		Parameters map[string]any  `json:"parameters"`
		Seed       int64           `json:"seed"`
		ActionLog  json.RawMessage `json:"action_log"`
		Expect     struct {
			Valid      bool               `json:"valid"`
			RejectCode string             `json:"reject_code"`
			Outcome    map[string]float64 `json:"outcome"`
		} `json:"expect"`
	} `json:"vectors"`
}

func loadVectors(t testing.TB) vectorFile {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "..", "..", "..", "specs", "examples", "simulation", "physics.lever", "v1", "vectors.json"))
	if err != nil {
		t.Fatal(err)
	}
	var f vectorFile
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

// replay is the server path of a run: the generic log limits, then the checker.
func replay(d Descriptor, raw json.RawMessage, params map[string]any, seed int64) (Outcome, error) {
	var log []Action
	if err := json.Unmarshal(raw, &log); err != nil {
		return nil, err
	}
	if err := ValidateLog(d, len(raw), log); err != nil {
		return nil, err
	}
	ch, _ := CheckerFor(d.TemplateCode, d.TemplateVersion)
	return ch.Run(params, seed, log)
}

// F066-AC-17 (T021, TC-SIM-003): the Go checker gives exactly the result of every shared vector, and its p95 is far
// inside the SIMULATION_ENGINE §6 target (checked here against 50 ms for one replay).
func TestLeverVectors(t *testing.T) {
	f := loadVectors(t)
	d, ok := Lookup(f.TemplateCode, f.TemplateVersion)
	if !ok || f.G != LeverG || len(f.Vectors) < 20 {
		t.Fatalf("vectors: %s v%d g=%v n=%d", f.TemplateCode, f.TemplateVersion, f.G, len(f.Vectors))
	}
	var durations []time.Duration
	for _, v := range f.Vectors {
		start := time.Now()
		o, err := replay(d, v.ActionLog, v.Parameters, v.Seed)
		durations = append(durations, time.Since(start))
		if !v.Expect.Valid {
			le, ok := AsLogError(err)
			if !ok || le.Code != v.Expect.RejectCode {
				t.Fatalf("%s: want reject %s, got %v", v.Name, v.Expect.RejectCode, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: %v", v.Name, err)
		}
		if len(o) != len(v.Expect.Outcome) {
			t.Fatalf("%s: outcome %v want %v", v.Name, o, v.Expect.Outcome)
		}
		for k, want := range v.Expect.Outcome {
			if got, ok := o[k]; !ok || math.Abs(got-want) > 1e-9 {
				t.Fatalf("%s: %s = %v want %v", v.Name, k, got, want)
			}
		}
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	if p95 := durations[len(durations)*95/100]; p95 > 50*time.Millisecond {
		t.Fatalf("checker p95 %v", p95)
	}
}

// BenchmarkLeverChecker measures one replay of the longest valid shared vector.
func BenchmarkLeverChecker(b *testing.B) {
	f := loadVectors(b)
	d, _ := Lookup(f.TemplateCode, f.TemplateVersion)
	v := f.Vectors[11]
	for i := 0; i < b.N; i++ {
		if _, err := replay(d, v.ActionLog, v.Parameters, v.Seed); err != nil {
			b.Fatal(err)
		}
	}
}

// F066-AC-13 (T022): 501 actions, more than 64 KB, more than 20 arguments are refused before the checker.
func TestValidateLogLimits(t *testing.T) {
	d, _ := Lookup("physics.lever", 1)
	many := make([]Action, 501)
	for i := range many {
		many[i] = Action{TMS: int64(i), Action: "release"}
	}
	if le, ok := AsLogError(ValidateLog(d, 100, many)); !ok || le.Code != RejectTooManyActions {
		t.Fatal("501 actions")
	}
	if le, ok := AsLogError(ValidateLog(d, MaxLogBytes+1, many[:1])); !ok || le.Code != RejectTooLarge {
		t.Fatal("64 KB")
	}
	args := map[string]any{}
	for i := 0; i < 21; i++ {
		args[strings.Repeat("a", i+1)] = i
	}
	if le, ok := AsLogError(ValidateLog(d, 100, []Action{{Action: "release", Args: args}})); !ok || le.Code != RejectTooManyArgs {
		t.Fatal("21 args")
	}
	if le, ok := AsLogError(ValidateLog(d, 100, []Action{{TMS: -1, Action: "release"}})); !ok || le.Code != RejectTimeOrder {
		t.Fatal("negative t_ms")
	}
}

// F066-AC-09, AC-11 (T022, T023): config validation against the template, goals evaluated on the outcome, the check
// and the asset ids never in the render payload, client mismatch, fixed seed.
func TestConfigGoalsRender(t *testing.T) {
	raw := `{"content":{"simulation_kind":"physics","template_code":"physics.lever","min_template_version":1,
	 "parameters":{"arm_length_m":1,"slots_per_side":4,"mass_1_kg":2,"mass_2_kg":1},
	 "goals":[{"goal_id":"balance","description":{"vi":"Cân bằng đòn bẩy"},"check":{"metric":"balanced","operator":"eq","value":1}},
	   {"goal_id":"near","description":{"vi":"Gần cân bằng"},"check":{"metric":"torque_diff","operator":"abs_diff_lte","value":0,"tolerance":0.5}}],
	 "steps":[{"step_id":"place","instruction":{"vi":"Đặt vật"}}],"seed_policy":"fixed"},"interaction":{"max_runs":5}}`
	c, err := Parse(json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	if vs := c.Validate(); len(vs) != 0 {
		t.Fatalf("valid config: %v", vs)
	}
	bad := c
	bad.Content.Parameters = map[string]any{"arm_length_m": 9, "slots_per_side": 4, "mass_1_kg": 2, "mass_2_kg": 1, "script": "x"}
	bad.Content.Goals = append([]Goal{}, c.Content.Goals...)
	bad.Content.Goals[0].Check.Metric = "temperature"
	got := bad.Validate().Error()
	for _, want := range []string{"parameters", "goals/0/check/metric"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %s in %s", want, got)
		}
	}
	unknown := c
	unknown.Content.TemplateCode = "chemistry.boiling"
	if vs := unknown.Validate(); len(vs) != 1 || vs[0].Field != "/config/content/template_code" {
		t.Fatalf("unknown template: %v", vs)
	}
	newer := c
	newer.Content.MinTemplateVersion = 2
	if vs := newer.Validate(); len(vs) != 1 {
		t.Fatalf("min_template_version above the catalog: %v", vs)
	}

	d, _ := c.Template()
	balanced := Outcome{"balanced": 1, "torque_diff": 0}
	gm := EvaluateGoals(c.Content.Goals, balanced)
	if !AllMet(gm) || len(gm) != 2 {
		t.Fatalf("goals: %v", gm)
	}
	if gm := EvaluateGoals(c.Content.Goals, Outcome{"balanced": 0, "torque_diff": 4.9}); gm[0].Met || gm[1].Met {
		t.Fatalf("unbalanced: %v", gm)
	}
	if !Mismatch(map[string]any{"goal_met": true}, Outcome{"balanced": 0}, EvaluateGoals(c.Content.Goals, Outcome{"balanced": 0})) ||
		!Mismatch(map[string]any{"balanced": true}, Outcome{"balanced": 0}, nil) || Mismatch(map[string]any{"torque_diff": 0.00001}, balanced, gm) {
		t.Fatal("mismatch")
	}
	b, _ := json.Marshal(RenderPayload(c, d, FixedSeed(uuid.Nil), 2))
	for _, leak := range []string{`"check"`, `"metric"`, `"operator"`, `"tolerance"`} {
		if strings.Contains(string(b), leak) {
			t.Fatalf("render payload leaks %s: %s", leak, b)
		}
	}
	id := uuid.New()
	if seed := FixedSeed(id); seed != FixedSeed(uuid.MustParse(id.String())) || seed < 0 || seed == FixedSeed(uuid.New()) || c.Seeding() != SeedFixed || c.MaxRuns() != 5 {
		t.Fatal("seed policy")
	}
	if err := Ready(); err != nil {
		t.Fatal(err)
	}
	if !SameParameters(map[string]any{"a": 1.0, "b": "x"}, map[string]any{"b": "x", "a": 1}) || SameParameters(map[string]any{"a": 1.0}, map[string]any{"a": 2.0}) {
		t.Fatal("same parameters")
	}
}
