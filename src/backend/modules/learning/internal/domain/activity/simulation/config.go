package simulation

import (
	"encoding/json"
	"errors"
	"math"
	"strconv"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

// Seed policies (content.seed_policy).
const (
	SeedFixed      = "fixed"
	SeedPerAttempt = "per_attempt"
)

// Verification statuses of a run.
const (
	Verified      = "verified"
	Rejected      = "rejected"
	NotVerifiable = "not_verifiable"
)

// Check is goals[].check: the server-only condition of a goal (never sent to the client, research R3).
type Check struct {
	Metric    string   `json:"metric"`
	Operator  string   `json:"operator"`
	Value     float64  `json:"value"`
	Tolerance *float64 `json:"tolerance,omitempty"`
}

// Goal is content.goals[].
type Goal struct {
	GoalID      string            `json:"goal_id"`
	Description map[string]string `json:"description"`
	Check       Check             `json:"check"`
}

// Step is content.steps[].
type Step struct {
	StepID         string            `json:"step_id"`
	Instruction    map[string]string `json:"instruction"`
	SafetyRequired bool              `json:"safety_required,omitempty"`
}

// QuestionRef is content.questions[] (a conclusion question, item kind simulation_question).
type QuestionRef struct {
	QuestionVersionID uuid.UUID `json:"question_version_id"`
}

// Content is config.content of simulation.schema.json.
type Content struct {
	SimulationKind     string         `json:"simulation_kind"`
	TemplateCode       string         `json:"template_code"`
	MinTemplateVersion int            `json:"min_template_version"`
	Parameters         map[string]any `json:"parameters"`
	Goals              []Goal         `json:"goals"`
	Steps              []Step         `json:"steps,omitempty"`
	Questions          []QuestionRef  `json:"questions,omitempty"`
	SeedPolicy         string         `json:"seed_policy,omitempty"`
}

// Interaction is config.interaction.
type Interaction struct {
	MaxRuns          *int  `json:"max_runs,omitempty"`
	ShowMeasurements *bool `json:"show_measurements,omitempty"`
}

// Config is the config of a simulation version.
type Config struct {
	Content     Content     `json:"content"`
	Interaction Interaction `json:"interaction"`
}

// ErrConfig: the config cannot be decoded.
var ErrConfig = errors.New("simulation: config cannot be read")

// Parse decodes a config already checked against its JSON Schema.
func Parse(raw json.RawMessage) (Config, error) {
	var c Config
	if len(raw) == 0 || json.Unmarshal(raw, &c) != nil {
		return c, ErrConfig
	}
	if c.Content.Parameters == nil {
		c.Content.Parameters = map[string]any{}
	}
	return c, nil
}

// Seeding is seed_policy (default per_attempt).
func (c Config) Seeding() string {
	if c.Content.SeedPolicy == SeedFixed {
		return SeedFixed
	}
	return SeedPerAttempt
}

// MaxRuns is interaction.max_runs, capped by the schema maximum (50) when absent [Proposed F066: bounded runs].
func (c Config) MaxRuns() int {
	if c.Interaction.MaxRuns != nil {
		return min(max(*c.Interaction.MaxRuns, 1), MaxRunsCap)
	}
	return MaxRunsCap
}

// Template returns the template version the config plays (Resolve).
func (c Config) Template() (Descriptor, bool) {
	return Resolve(c.Content.TemplateCode, c.Content.MinTemplateVersion)
}

// QuestionIDs are the conclusion questions in declaration order (item position i+1 is entry i, never shuffled).
func (c Config) QuestionIDs() []uuid.UUID {
	out := []uuid.UUID{}
	for _, q := range c.Content.Questions {
		out = append(out, q.QuestionVersionID)
	}
	return out
}

// Validate checks what the JSON Schema cannot (F066-AC-09): an active template version ≥ min_template_version of
// the same kind, parameters valid for its checker (the rules of parameter_schema), goal metrics of the template,
// unique goal and step ids. A retired template refuses new drafts (Edge Cases).
func (c Config) Validate() contentmodel.Violations {
	var out contentmodel.Violations
	add := func(f, r string) { out = append(out, contentmodel.Violation{Field: "/config/content/" + f, Reason: r}) }
	d, ok := c.Template()
	if !ok {
		add("template_code", "has no active template version ≥ min_template_version")
		return out
	}
	if d.SimulationKind != c.Content.SimulationKind {
		add("simulation_kind", "is not the kind of the template ("+d.SimulationKind+")")
	}
	if ch, ok := CheckerFor(d.TemplateCode, d.TemplateVersion); ok {
		if err := ch.ValidateParameters(c.Content.Parameters); err != nil {
			add("parameters", err.Error())
		}
	}
	seen := map[string]bool{}
	for i, g := range c.Content.Goals {
		p := "goals/" + strconv.Itoa(i)
		if seen[g.GoalID] {
			add(p+"/goal_id", "repeats the id "+g.GoalID)
		}
		seen[g.GoalID] = true
		if _, ok := d.Metric(g.Check.Metric); !ok {
			add(p+"/check/metric", "is not a metric of the template")
		}
		if g.Check.Operator == "abs_diff_lte" && g.Check.Tolerance == nil {
			add(p+"/check/tolerance", "is required with abs_diff_lte")
		}
	}
	steps := map[string]bool{}
	for i, s := range c.Content.Steps {
		if steps[s.StepID] {
			add("steps/"+strconv.Itoa(i)+"/step_id", "repeats the id "+s.StepID)
		}
		steps[s.StepID] = true
	}
	return out
}

// GoalMet is the result of one goal in a run.
type GoalMet struct {
	GoalID string `json:"goal_id"`
	Met    bool   `json:"met"`
}

// Met applies a check to an outcome; a metric the outcome lacks is not met.
func (k Check) Met(o Outcome) bool {
	x, ok := o[k.Metric]
	if !ok {
		return false
	}
	const eps = 1e-9
	switch k.Operator {
	case "eq":
		return math.Abs(x-k.Value) <= eps
	case "lt":
		return x < k.Value-eps
	case "lte":
		return x <= k.Value+eps
	case "gt":
		return x > k.Value+eps
	case "gte":
		return x >= k.Value-eps
	case "abs_diff_lte":
		tol := 0.0
		if k.Tolerance != nil {
			tol = *k.Tolerance
		}
		return math.Abs(x-k.Value) <= tol+eps
	}
	return false
}

// EvaluateGoals applies every goal to an outcome (nil outcome: none met).
func EvaluateGoals(goals []Goal, o Outcome) []GoalMet {
	out := make([]GoalMet, 0, len(goals))
	for _, g := range goals {
		out = append(out, GoalMet{GoalID: g.GoalID, Met: o != nil && g.Check.Met(o)})
	}
	return out
}

// AllMet reports every goal met (false without goals).
func AllMet(gm []GoalMet) bool {
	for _, g := range gm {
		if !g.Met {
			return false
		}
	}
	return len(gm) > 0
}

// Display keeps the display metrics of the template (verified_outcome sent back, research R3).
func Display(d Descriptor, o Outcome) map[string]float64 {
	out := map[string]float64{}
	for k, v := range o {
		if m, ok := d.Metric(k); ok && m.Display {
			out[k] = v
		}
	}
	return out
}

// Mismatch reports a client_outcome that contradicts the verified outcome (F066-AC-12): a reported metric of the
// outcome with another value, or a reported goal_met / goals_met other than the verified one.
func Mismatch(client map[string]any, o Outcome, gm []GoalMet) bool {
	for k, v := range client {
		switch k {
		case "goal_met":
			if b, ok := v.(bool); ok && b != AllMet(gm) {
				return true
			}
		default:
			if x, ok := o[k]; ok {
				f, isNum := number(v)
				if b, isBool := v.(bool); isBool {
					f, isNum = 0, true
					if b {
						f = 1
					}
				}
				if isNum && math.Abs(f-x) > 1e-4 {
					return true
				}
			}
		}
	}
	return false
}

// RenderPayload is ActivityRender.render_payload of a simulation version (simulation.schema.json#/$defs/render,
// F066-AC-10): the pinned template, parameters, goals without their check, steps, interaction and question positions;
// seed and runs_used come from the attempt.
func RenderPayload(c Config, d Descriptor, seed int64, runsUsed int) map[string]any {
	goals := make([]map[string]any, 0, len(c.Content.Goals))
	for _, g := range c.Content.Goals {
		goals = append(goals, map[string]any{"goal_id": g.GoalID, "description": g.Description})
	}
	steps := make([]map[string]any, 0, len(c.Content.Steps))
	for _, s := range c.Content.Steps {
		steps = append(steps, map[string]any{"step_id": s.StepID, "instruction": s.Instruction, "safety_required": s.SafetyRequired})
	}
	questions := make([]map[string]any, 0, len(c.Content.Questions))
	for i := range c.Content.Questions {
		questions = append(questions, map[string]any{"position": i + 1})
	}
	var maxRuns any
	if c.Interaction.MaxRuns != nil {
		maxRuns = c.MaxRuns()
	}
	show := c.Interaction.ShowMeasurements == nil || *c.Interaction.ShowMeasurements
	version := d.TemplateVersion
	if version == 0 {
		version = max(c.Content.MinTemplateVersion, 1)
	}
	return map[string]any{
		"simulation_kind": c.Content.SimulationKind, "template_code": c.Content.TemplateCode, "template_version": version,
		"parameters": c.Content.Parameters, "goals": goals, "steps": steps, "questions": questions,
		"interaction": map[string]any{"max_runs": maxRuns, "show_measurements": show},
		"seed":        max(seed, 0), "runs_used": runsUsed,
	}
}
