// Package params is the parameterized question support (F023 US4, QUESTION_BANK §5): the declaration of the
// variables of a version, the rejection sampler (reproducible by seed, at most param_sampling_max_tries tries,
// division by zero = not satisfied) and the expansion of {{name}} placeholders in rich contents. Pure Go.
package params

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"math/big"
	"math/rand/v2"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/expr"
)

// MaxTries is param_sampling_max_tries (QUESTION_BANK §5, CONFIG_DEFAULTS: 1000, scope code).
const MaxTries = 1000

// Limits of a declaration (question.schema.json#/$defs/params, F023-AC-20).
const (
	MaxVars        = 10
	MaxConstraints = 10
	MaxChoices     = 20
	maxRange       = 1_000_000
)

var nameRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,15}$`)

// PlaceholderRe matches {{name}} (never {{blank:…}} of fill_in_blank).
var PlaceholderRe = regexp.MustCompile(`\{\{\s*([a-z][a-z0-9_]{0,15})\s*\}\}`)

// ErrInfeasible: no value set satisfies the constraints within MaxTries.
var ErrInfeasible = errors.New("params: no value set satisfies the constraints")

// Var is one variable.
type Var struct {
	Type   string   `json:"type"`
	Min    *float64 `json:"min,omitempty"`
	Max    *float64 `json:"max,omitempty"`
	Step   *float64 `json:"step,omitempty"`
	Values []any    `json:"values,omitempty"`
}

// Spec is a params declaration.
type Spec struct {
	Vars        map[string]Var
	Constraints []string
	parsed      []expr.Node
}

// Names returns the variable names, sorted.
func (s Spec) Names() []string {
	out := make([]string, 0, len(s.Vars))
	for n := range s.Vars {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Declared returns the set of names.
func (s Spec) Declared() map[string]bool {
	m := map[string]bool{}
	for n := range s.Vars {
		m[n] = true
	}
	return m
}

// Parse decodes and checks a declaration (F023-AC-14, AC-15, AC-20): names, types, min ≤ max, step > 0, choices,
// constraints parsed by the safe evaluator.
func Parse(raw json.RawMessage) (Spec, []contentmodel.Violation) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return Spec{}, []contentmodel.Violation{{Field: "/render_payload/params", Reason: "must be an object"}}
	}
	s := Spec{Vars: map[string]Var{}}
	var vs []contentmodel.Violation
	add := func(f, r string) {
		vs = append(vs, contentmodel.Violation{Field: "/render_payload/params" + f, Reason: r})
	}
	if c, ok := m["constraints"]; ok {
		if err := json.Unmarshal(c, &s.Constraints); err != nil {
			add("/constraints", "must be a list of expressions")
		}
		delete(m, "constraints")
	}
	if len(m) == 0 {
		add("", "declares no variable")
	}
	if len(m) > MaxVars {
		add("", fmt.Sprintf("has more than %d variables", MaxVars))
	}
	if len(s.Constraints) > MaxConstraints {
		add("/constraints", fmt.Sprintf("has more than %d constraints", MaxConstraints))
	}
	for name, rv := range m {
		p := "/" + name
		if !nameRe.MatchString(name) {
			add(p, "must match ^[a-z][a-z0-9_]{0,15}$")
			continue
		}
		var v Var
		if err := json.Unmarshal(rv, &v); err != nil {
			add(p, "is not a variable declaration")
			continue
		}
		switch v.Type {
		case "int", "decimal":
			if v.Min == nil || v.Max == nil {
				add(p, "needs min and max")
				continue
			}
			if *v.Min > *v.Max {
				add(p+"/min", "is greater than max")
			}
			if v.Step != nil && *v.Step <= 0 {
				add(p+"/step", "must be greater than 0")
			}
			if v.Type == "int" && (*v.Min != float64(int64(*v.Min)) || *v.Max != float64(int64(*v.Max))) {
				add(p, "int bounds must be integers")
			}
			if v.Type == "decimal" && v.Step == nil {
				add(p+"/step", "is required for a decimal")
			}
			if *v.Max-*v.Min > maxRange {
				add(p, "range is too large")
			}
		case "choice":
			if len(v.Values) == 0 || len(v.Values) > MaxChoices {
				add(p+"/values", fmt.Sprintf("must have 1–%d values", MaxChoices))
			}
			for _, x := range v.Values {
				if _, ok := x.(float64); !ok {
					add(p+"/values", "choices are numbers")
					break
				}
			}
		default:
			add(p+"/type", "must be int, decimal or choice")
		}
		s.Vars[name] = v
	}
	if len(vs) > 0 {
		return s, vs
	}
	declared := s.Declared()
	for i, c := range s.Constraints {
		n, err := expr.Parse(c, declared)
		if err != nil {
			add(fmt.Sprintf("/constraints/%d", i), err.Error())
			continue
		}
		s.parsed = append(s.parsed, n)
	}
	return s, vs
}

// Values are the sampled values of one presentation.
type Values map[string]*big.Rat

// Strings formats values for Presentation.params (exact decimal strings).
func (v Values) Strings() map[string]string {
	out := map[string]string{}
	for k, r := range v {
		out[k] = format(r, ".")
	}
	return out
}

// FromStrings parses Presentation.params.
func FromStrings(m map[string]string) (Values, bool) {
	out := Values{}
	for k, s := range m {
		r, ok := new(big.Rat).SetString(s)
		if !ok {
			return nil, false
		}
		out[k] = r
	}
	return out, true
}

func rng(seed uint64, version uuid.UUID) *rand.Rand {
	h := fnv.New64a()
	_, _ = h.Write(version[:])
	_, _ = h.Write([]byte("params"))
	return rand.New(rand.NewPCG(seed, h.Sum64())) //nolint:gosec // reproducible sampling, not a secret
}

func ratOf(f float64) *big.Rat {
	r := new(big.Rat)
	r.SetString(fmt.Sprintf("%g", f))
	return r
}

func (s Spec) draw(r *rand.Rand) Values {
	out := Values{}
	for _, name := range s.Names() {
		v := s.Vars[name]
		switch v.Type {
		case "int", "decimal":
			step := ratOf(1)
			if v.Step != nil {
				step = ratOf(*v.Step)
			}
			lo, hi := ratOf(*v.Min), ratOf(*v.Max)
			span := new(big.Rat).Quo(new(big.Rat).Sub(hi, lo), step)
			n := new(big.Int).Quo(span.Num(), span.Denom()).Int64()
			k := r.Int64N(n + 1)
			out[name] = new(big.Rat).Add(lo, new(big.Rat).Mul(step, big.NewRat(k, 1)))
		case "choice":
			if f, ok := v.Values[r.IntN(len(v.Values))].(float64); ok {
				out[name] = ratOf(f)
			}
		}
	}
	return out
}

// Satisfied evaluates the constraints on v (division by zero = not satisfied).
func (s Spec) Satisfied(v Values) bool {
	for _, n := range s.parsed {
		ok, err := expr.Truth(n, v)
		if err != nil || !ok {
			return false
		}
	}
	return true
}

// Sample draws the values of a presentation (F023-AC-16): reproducible for (seed, version), every constraint
// satisfied; ErrInfeasible after MaxTries.
func (s Spec) Sample(seed uint64, version uuid.UUID) (Values, error) {
	r := rng(seed, version)
	for range MaxTries {
		v := s.draw(r)
		if s.Satisfied(v) {
			return v, nil
		}
	}
	return nil, ErrInfeasible
}

// Placeholders returns the {{name}} placeholders of a JSON document.
func Placeholders(doc json.RawMessage) []string {
	var out []string
	for _, m := range PlaceholderRe.FindAllStringSubmatch(string(doc), -1) {
		if !slices.Contains(out, m[1]) {
			out = append(out, m[1])
		}
	}
	return out
}

// Expression turns an answer expression written with placeholders ("{{a}} * {{b}}") into evaluator syntax.
func Expression(s string) string {
	return PlaceholderRe.ReplaceAllString(s, "$1")
}

// format writes r as a decimal (≤ 6 decimals, trailing zeros trimmed) with the decimal separator sep.
func format(r *big.Rat, sep string) string {
	if r == nil {
		return ""
	}
	if r.IsInt() {
		return r.Num().String()
	}
	s := strings.TrimRight(strings.TrimRight(r.FloatString(6), "0"), ".")
	return strings.Replace(s, ".", sep, 1)
}

// Format writes a value for content_language lang (vi: decimal comma, en: decimal point).
func Format(r *big.Rat, lang string) string {
	if lang == "en" {
		return format(r, ".")
	}
	return format(r, ",")
}

// Expand replaces the {{name}} placeholders of a JSON document by the formatted values (F023-AC-16); text choices are
// read from texts. JSON string escaping is kept since values are numbers or plain texts.
func Expand(doc json.RawMessage, v Values, texts map[string]string, lang string) json.RawMessage {
	if len(doc) == 0 {
		return doc
	}
	out := PlaceholderRe.ReplaceAllStringFunc(string(doc), func(m string) string {
		name := PlaceholderRe.FindStringSubmatch(m)[1]
		if t, ok := texts[name]; ok {
			b, _ := json.Marshal(t)
			return strings.Trim(string(b), `"`)
		}
		if r, ok := v[name]; ok && r != nil {
			return Format(r, lang)
		}
		return m
	})
	return json.RawMessage(out)
}
