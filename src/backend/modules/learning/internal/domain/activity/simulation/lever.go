package simulation

import (
	"fmt"
	"math"
	"strconv"
)

// LeverG is the gravity constant of physics.lever v1 (descriptor constants.g).
const LeverG = 9.8

// Lever is the checker of physics.lever v1 (research R4): a static lever, masses placed on discrete slots of each
// arm, torque Σ m·g·d per side with d = arm_length_m · slot / slots_per_side. Actions: place_mass {mass_index, side,
// slot} (moves a placed mass), remove_mass {mass_index}, release {} (the outcome is the state at the last release;
// without release nothing is measured). Metrics: torque_left, torque_right, torque_diff (left − right), balanced (1
// when both sides hold a mass and |diff| < 1e-6), masses_placed. The Godot template computes the same model in
// lever_model.gd; specs/examples/simulation/physics.lever/v1/vectors.json binds both.
type Lever struct{}

var leverParams = []string{"arm_length_m", "slots_per_side", "mass_1_kg", "mass_2_kg", "mass_3_kg", "mass_4_kg"}

type leverSetup struct {
	arm    float64
	slots  int
	masses []float64
}

func number(v any) (float64, bool) {
	f, ok := v.(float64)
	if !ok {
		if i, isInt := v.(int); isInt {
			return float64(i), true
		}
	}
	return f, ok && !math.IsNaN(f) && !math.IsInf(f, 0)
}

func whole(v any) (int, bool) {
	f, ok := number(v)
	if !ok || f != math.Trunc(f) {
		return 0, false
	}
	return int(f), true
}

func (Lever) setup(p map[string]any) (leverSetup, error) {
	for k := range p {
		known := false
		for _, n := range leverParams {
			known = known || k == n
		}
		if !known {
			return leverSetup{}, fmt.Errorf("unknown parameter %s", k)
		}
	}
	var s leverSetup
	var ok bool
	if s.arm, ok = number(p["arm_length_m"]); !ok || s.arm < 0.2 || s.arm > 5 {
		return s, fmt.Errorf("arm_length_m must be between 0.2 and 5")
	}
	if s.slots, ok = whole(p["slots_per_side"]); !ok || s.slots < 1 || s.slots > 10 {
		return s, fmt.Errorf("slots_per_side must be an integer between 1 and 10")
	}
	for i := 1; i <= 4; i++ {
		v, present := p["mass_"+strconv.Itoa(i)+"_kg"]
		if !present {
			if i <= 2 {
				return s, fmt.Errorf("mass_%d_kg is required", i)
			}
			continue
		}
		if len(s.masses) != i-1 {
			return s, fmt.Errorf("mass_%d_kg needs mass_%d_kg", i, i-1)
		}
		m, ok := number(v)
		if !ok || m < 0.1 || m > 50 {
			return s, fmt.Errorf("mass_%d_kg must be between 0.1 and 50", i)
		}
		s.masses = append(s.masses, m)
	}
	return s, nil
}

// ValidateParameters implements Checker.
func (l Lever) ValidateParameters(p map[string]any) error {
	_, err := l.setup(p)
	return err
}

type placement struct {
	side int // -1 left, 1 right, 0 not placed
	slot int
}

// Run implements Checker; the seed is not used by this deterministic model.
func (l Lever) Run(p map[string]any, _ int64, log []Action) (Outcome, error) {
	s, err := l.setup(p)
	if err != nil {
		return nil, Reject(RejectPinnedValue, err.Error())
	}
	state := make([]placement, len(s.masses))
	measured := make([]placement, len(s.masses))
	for i, a := range log {
		at := "action " + strconv.Itoa(i)
		switch a.Action {
		case "place_mass":
			if len(a.Args) != 3 {
				return nil, Reject(RejectArgument, at+": place_mass takes mass_index, side, slot")
			}
			idx, ok := whole(a.Args["mass_index"])
			if !ok || idx < 0 || idx >= len(s.masses) {
				return nil, Reject(RejectArgument, at+": mass_index out of range")
			}
			slot, ok := whole(a.Args["slot"])
			if !ok || slot < 1 || slot > s.slots {
				return nil, Reject(RejectArgument, at+": slot out of range")
			}
			side := 0
			switch a.Args["side"] {
			case "left":
				side = -1
			case "right":
				side = 1
			default:
				return nil, Reject(RejectArgument, at+": side must be left or right")
			}
			state[idx] = placement{side: side, slot: slot}
		case "remove_mass":
			if len(a.Args) != 1 {
				return nil, Reject(RejectArgument, at+": remove_mass takes mass_index")
			}
			idx, ok := whole(a.Args["mass_index"])
			if !ok || idx < 0 || idx >= len(s.masses) {
				return nil, Reject(RejectArgument, at+": mass_index out of range")
			}
			state[idx] = placement{}
		case "release":
			if len(a.Args) != 0 {
				return nil, Reject(RejectArgument, at+": release takes no argument")
			}
			copy(measured, state)
		default:
			return nil, Reject(RejectUnknownAction, at+": not an action of physics.lever")
		}
	}
	return s.measure(measured), nil
}

func (s leverSetup) measure(st []placement) Outcome {
	var left, right float64
	var nl, nr int
	for i, p := range st {
		d := s.arm * float64(p.slot) / float64(s.slots)
		switch p.side {
		case -1:
			left += s.masses[i] * LeverG * d
			nl++
		case 1:
			right += s.masses[i] * LeverG * d
			nr++
		}
	}
	diff := left - right
	balanced := 0.0
	if nl > 0 && nr > 0 && math.Abs(diff) < 1e-6 {
		balanced = 1
	}
	return Outcome{"torque_left": Round4(left), "torque_right": Round4(right), "torque_diff": Round4(diff), "balanced": balanced,
		"masses_placed": float64(nl + nr)}
}
