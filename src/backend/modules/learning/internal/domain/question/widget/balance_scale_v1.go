package widget

import (
	"encoding/json"
	"fmt"
	"slices"
)

// BalanceScaleV1 is balance_scale@1.0 (QUESTION_BANK §3.11): weights on the left pan are fixed; the student places
// available weights (each at most once) on the right pan within max_moves; balance_scale.equal checks that the right
// pan equals the target (default: the left pan).
type BalanceScaleV1 struct{}

type balanceConfig struct {
	LeftWeights      []int `json:"left_weights"`
	AvailableWeights []int `json:"available_weights"`
	MaxMoves         int   `json:"max_moves"`
}

type balanceState struct {
	RightWeights []int `json:"right_weights"`
}

type balanceParams struct {
	Target *int `json:"target"`
}

// ValidatorEqual is the validator of balance_scale@1.0.
const ValidatorEqual = "balance_scale.equal"

// balanceSchema is specs/schemas/question-widgets/balance_scale.v1.schema.json (config + params).
var balanceSchema = json.RawMessage(`{"type":"object","properties":{"config":{"type":"object","required":["left_weights","available_weights","max_moves"],` +
	`"properties":{"left_weights":{"type":"array","minItems":1,"maxItems":10,"items":{"type":"integer","minimum":1,"maximum":1000}},` +
	`"available_weights":{"type":"array","minItems":1,"maxItems":10,"items":{"type":"integer","minimum":1,"maximum":1000}},` +
	`"max_moves":{"type":"integer","minimum":1,"maximum":50}},"additionalProperties":false},` +
	`"params":{"type":"object","properties":{"target":{"type":"integer","minimum":1,"maximum":10000}},"additionalProperties":false}}}`)

// Code implements Widget.
func (BalanceScaleV1) Code() string { return "balance_scale" }

// Version implements Widget.
func (BalanceScaleV1) Version() string { return "1.0" }

// ConfigSchema implements Widget.
func (BalanceScaleV1) ConfigSchema() json.RawMessage { return balanceSchema }

// Validators implements Widget.
func (BalanceScaleV1) Validators() []string { return []string{ValidatorEqual} }

func strictly(raw json.RawMessage, v any) error {
	if len(raw) == 0 {
		return nil
	}
	d := json.NewDecoder(bytesReader(raw))
	d.DisallowUnknownFields()
	return d.Decode(v)
}

// CheckConfig implements Widget.
func (BalanceScaleV1) CheckConfig(config json.RawMessage, validator string, params json.RawMessage) []string {
	var out []string
	var c balanceConfig
	if err := strictly(config, &c); err != nil {
		return []string{"config is not a balance_scale@1.0 config"}
	}
	if len(c.LeftWeights) == 0 || len(c.LeftWeights) > 10 || len(c.AvailableWeights) == 0 || len(c.AvailableWeights) > 10 {
		out = append(out, "left_weights and available_weights need 1–10 weights")
	}
	for _, w := range append(slices.Clone(c.LeftWeights), c.AvailableWeights...) {
		if w < 1 || w > 1000 {
			out = append(out, "weights are 1–1000")
			break
		}
	}
	if c.MaxMoves < 1 || c.MaxMoves > 50 {
		out = append(out, "max_moves is 1–50")
	}
	if validator != ValidatorEqual {
		out = append(out, "validator must be "+ValidatorEqual)
	}
	var p balanceParams
	if err := strictly(params, &p); err != nil {
		out = append(out, "params of balance_scale.equal are {target}")
	} else if p.Target != nil && (*p.Target < 1 || *p.Target > 10000) {
		out = append(out, "target is 1–10000")
	}
	return out
}

// Replay implements Widget: place {w} puts an available weight on the right pan, remove {w} takes it back.
func (BalanceScaleV1) Replay(config json.RawMessage, log []Action) (json.RawMessage, error) {
	var c balanceConfig
	if err := json.Unmarshal(config, &c); err != nil {
		return nil, ErrBadConfig
	}
	if len(log) > c.MaxMoves {
		return nil, fmt.Errorf("%w: more than max_moves", ErrIllegalAction)
	}
	available := slices.Clone(c.AvailableWeights)
	right := []int{}
	for _, a := range log {
		f, ok := a.Args["w"].(float64)
		w := int(f)
		if !ok || float64(w) != f {
			return nil, fmt.Errorf("%w: %s needs an integer w", ErrIllegalAction, a.Action)
		}
		switch a.Action {
		case "place":
			i := slices.Index(available, w)
			if i < 0 {
				return nil, fmt.Errorf("%w: weight %d is not available", ErrIllegalAction, w)
			}
			available = slices.Delete(available, i, i+1)
			right = append(right, w)
		case "remove":
			i := slices.Index(right, w)
			if i < 0 {
				return nil, fmt.Errorf("%w: weight %d is not on the scale", ErrIllegalAction, w)
			}
			right = slices.Delete(right, i, i+1)
			available = append(available, w)
		default:
			return nil, fmt.Errorf("%w: %s", ErrIllegalAction, a.Action)
		}
	}
	return json.Marshal(balanceState{RightWeights: right})
}

func sortedState(raw json.RawMessage) ([]int, bool) {
	var s balanceState
	if err := strictly(raw, &s); err != nil || len(s.RightWeights) > 10 {
		return nil, false
	}
	out := slices.Clone(s.RightWeights)
	slices.Sort(out)
	return out, true
}

// Same implements Widget (order of the weights does not matter).
func (BalanceScaleV1) Same(a, b json.RawMessage) bool {
	x, ok1 := sortedState(a)
	y, ok2 := sortedState(b)
	return ok1 && ok2 && slices.Equal(x, y)
}

// Check implements Widget.
func (BalanceScaleV1) Check(validator string, config, params, state json.RawMessage) (bool, error) {
	if validator != ValidatorEqual {
		return false, ErrUnknown
	}
	var c balanceConfig
	var p balanceParams
	if json.Unmarshal(config, &c) != nil || strictly(params, &p) != nil {
		return false, ErrBadConfig
	}
	right, ok := sortedState(state)
	if !ok {
		return false, ErrIllegalAction
	}
	sum := func(ws []int) int {
		t := 0
		for _, w := range ws {
			t += w
		}
		return t
	}
	target := sum(c.LeftWeights)
	if p.Target != nil {
		target = *p.Target
	}
	return sum(right) == target, nil
}
