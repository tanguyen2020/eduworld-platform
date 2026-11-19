package question

import (
	"encoding/json"
	"math/big"
	"slices"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/expr"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/params"
)

// paramTypes are the types whose render payload may declare params (question.schema.json).
var paramTypes = []Type{SingleChoice, Numeric}

// paramsOf returns render_payload.params, nil when absent.
func paramsOf(render json.RawMessage) json.RawMessage {
	var m map[string]json.RawMessage
	if unmarshal(render, &m) != nil {
		return nil
	}
	p := m["params"]
	if len(p) == 0 || string(p) == "null" {
		return nil
	}
	return p
}

// withoutParams removes the params declaration from a render payload (F023-AC-18).
func withoutParams(render json.RawMessage) json.RawMessage {
	var m map[string]json.RawMessage
	if unmarshal(render, &m) != nil {
		return render
	}
	delete(m, "params")
	b, err := json.Marshal(m)
	if err != nil {
		return render
	}
	return b
}

// answerExpression returns the expression of a numeric answer key written over params ("{{a}} * {{b}}").
func answerExpression(key json.RawMessage) (string, bool) {
	var k struct {
		Value json.RawMessage `json:"value"`
	}
	if unmarshal(key, &k) != nil {
		return "", false
	}
	var s string
	if json.Unmarshal(k.Value, &s) != nil {
		return "", false
	}
	return s, true
}

// validateParams applies F023-AC-14, AC-15: the declaration, every {{name}} of the version declared, constraints
// and the answer expression parsed by the safe evaluator, and one value set satisfying the constraints.
func validateParams(a Authoring) []Violation {
	raw := paramsOf(a.RenderPayload)
	expression, isExpr := answerExpression(a.AnswerKey)
	if raw == nil {
		if a.Type == Numeric && isExpr {
			return []Violation{{Field: "/answer_key/value", Reason: "an expression needs render_payload.params"}}
		}
		return nil
	}
	if !slices.Contains(paramTypes, a.Type) {
		return []Violation{{Field: "/render_payload/params", Reason: "only single_choice and numeric questions are parameterized"}}
	}
	spec, vs := params.Parse(raw)
	if len(vs) > 0 {
		return vs
	}
	declared := spec.Declared()
	docs := append([]json.RawMessage{a.Stem, withoutParams(a.RenderPayload), a.AnswerKey, a.Explanation}, a.Hints...)
	for _, d := range docs {
		for _, name := range params.Placeholders(d) {
			if !declared[name] {
				vs = append(vs, Violation{Field: "/render_payload/params", Reason: "{{" + name + "}} is not declared"})
			}
		}
	}
	if isExpr {
		if _, err := expr.Parse(params.Expression(expression), declared); err != nil {
			vs = append(vs, Violation{Field: "/answer_key/value", Reason: err.Error()})
		}
	}
	if len(vs) > 0 {
		return vs
	}
	vals, err := spec.Sample(0, uuid.Nil)
	if err != nil {
		return []Violation{{Field: "/render_payload/params/constraints", Reason: "no value set satisfies the constraints"}}
	}
	if isExpr {
		n, _ := expr.Parse(params.Expression(expression), declared)
		if _, err := expr.Number(n, vals); err != nil {
			return []Violation{{Field: "/answer_key/value", Reason: "the expression is not a number for the sampled values"}}
		}
	}
	return nil
}

// expand draws the values of a presentation and substitutes them in the stem and the render payload; the numeric
// answer key is computed on the same values (F023-AC-16, AC-17).
func expand(c Content, seed uint64, stored map[string]string) (Content, map[string]string, error) {
	raw := paramsOf(c.Render)
	if raw == nil {
		return c, nil, nil
	}
	spec, vs := params.Parse(raw)
	if len(vs) > 0 {
		return c, nil, ErrInvalidResponse
	}
	vals, ok := params.FromStrings(stored)
	if !ok || len(stored) == 0 {
		var err error
		if vals, err = spec.Sample(seed, c.VersionID); err != nil {
			return c, nil, err
		}
	}
	c.Stem = params.Expand(c.Stem, vals, nil, c.Language)
	c.Render = params.Expand(withoutParams(c.Render), vals, nil, c.Language)
	if e, ok := answerExpression(c.AnswerKey); ok {
		n, err := expr.Parse(params.Expression(e), spec.Declared())
		if err != nil {
			return c, nil, err
		}
		v, err := expr.Number(n, vals)
		if err != nil {
			return c, nil, err
		}
		c.AnswerKey = withValue(c.AnswerKey, v)
	}
	return c, vals.Strings(), nil
}

func withValue(key json.RawMessage, v *big.Rat) json.RawMessage {
	var m map[string]json.RawMessage
	if unmarshal(key, &m) != nil {
		return key
	}
	m["value"] = json.RawMessage(v.FloatString(10))
	b, err := json.Marshal(m)
	if err != nil {
		return key
	}
	return b
}

// GradeIn scores a response against the presentation it was shown with: parameterized questions are graded on the
// values of the presentation (F023-AC-17).
func (r *Registry) GradeIn(c Content, p Presentation, response json.RawMessage) (GradeResult, error) {
	ec, _, err := expand(c, p.Seed, p.Params)
	if err != nil {
		return GradeResult{}, err
	}
	return r.Grade(ec, response)
}
