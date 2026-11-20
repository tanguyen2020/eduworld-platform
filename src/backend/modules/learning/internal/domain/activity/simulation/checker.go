package simulation

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"
	"strconv"

	"github.com/google/uuid"
)

// Limits of an action log (SIMULATION_ENGINE §5): ≤ 500 actions, ≤ 64 KB, ≤ 20 arguments per action.
const (
	MaxActions  = 500
	MaxLogBytes = 65536
	MaxArgs     = 20
	MaxRunsCap  = 50
)

// Reject codes of a run (assessment.simulation_runs.reject_code; never student data).
const (
	RejectTooManyActions   = "too_many_actions"
	RejectTooLarge         = "log_too_large"
	RejectTimeOrder        = "time_not_increasing"
	RejectUnknownAction    = "unknown_action"
	RejectTooManyArgs      = "too_many_args"
	RejectArgument         = "argument_out_of_range"
	RejectPinnedValue      = "pinned_value_mismatch"
	RejectTemplateNotFound = "template_not_found"
)

// Action is one entry of an action log: t_ms relative to the start of the run.
type Action struct {
	TMS    int64          `json:"t_ms"`
	Action string         `json:"action"`
	Args   map[string]any `json:"args,omitempty"`
}

// Outcome is what a checker computes: metric → value (bool metrics are 0/1), rounded to 4 decimals.
type Outcome map[string]float64

// LogError is an invalid action log or argument (422 SIMULATION_LOG_INVALID, run rejected).
type LogError struct{ Code, Detail string }

func (e *LogError) Error() string { return "simulation: " + e.Code + ": " + e.Detail }

// Reject returns a LogError.
func Reject(code, detail string) error { return &LogError{Code: code, Detail: detail} }

// AsLogError reports a LogError.
func AsLogError(err error) (*LogError, bool) {
	var le *LogError
	ok := errors.As(err, &le)
	return le, ok
}

// Checker replays a run deterministically from the pinned parameters, the server seed and the action log; the same
// model runs in the Godot template for display only (research R4, shared vectors).
type Checker interface {
	// ValidateParameters checks the parameters of a version (the rules of parameter_schema, F066-AC-09).
	ValidateParameters(params map[string]any) error
	// Run computes the outcome; an action outside its arguments is a LogError.
	Run(params map[string]any, seed int64, log []Action) (Outcome, error)
}

var checkers = map[string]Checker{"physics.lever@1": Lever{}}

// CheckerFor returns the checker of a template version.
func CheckerFor(code string, version int) (Checker, bool) {
	c, ok := checkers[code+"@"+strconv.Itoa(version)]
	return c, ok
}

// ValidateLog checks the generic limits of a log against its template (F066-AC-13): count, size of the JSON sent,
// t_ms ≥ 0 and never decreasing, actions of the template, ≤ 20 arguments.
func ValidateLog(d Descriptor, rawSize int, log []Action) error {
	if len(log) > MaxActions {
		return Reject(RejectTooManyActions, "more than 500 actions")
	}
	if rawSize > MaxLogBytes {
		return Reject(RejectTooLarge, "action log larger than 64 KB")
	}
	var last int64
	for i, a := range log {
		at := "action " + strconv.Itoa(i)
		if a.TMS < 0 || a.TMS < last {
			return Reject(RejectTimeOrder, at+": t_ms decreases")
		}
		last = a.TMS
		if !d.HasAction(a.Action) {
			return Reject(RejectUnknownAction, at+": not an action of the template")
		}
		if len(a.Args) > MaxArgs {
			return Reject(RejectTooManyArgs, at+": more than 20 arguments")
		}
	}
	return nil
}

// MaxSeed bounds a simulation seed to 2^53 − 1 so it survives a JSON number in every client (Godot, JS) unchanged
// [Proposed F066].
const MaxSeed int64 = 1<<53 - 1

// FixedSeed is the seed of seed_policy fixed (research R5): the first 64 bits of SHA-256(activity_version_id), kept
// within MaxSeed.
func FixedSeed(versionID uuid.UUID) int64 {
	sum := sha256.Sum256(versionID[:])
	return int64(binary.BigEndian.Uint64(sum[:8])) & MaxSeed
}

// ClientSeed bounds a random seed within MaxSeed (seed_policy per_attempt).
func ClientSeed(seed int64) int64 { return max(seed, 0) & MaxSeed }

// Round4 rounds half away from zero to 4 decimals (the metric precision shared with the Godot model).
func Round4(x float64) float64 { return math.Round(x*1e4) / 1e4 }

// SameParameters reports that the parameters sent with a run are the pinned ones (JSON equality).
func SameParameters(pinned, sent map[string]any) bool {
	a, err1 := json.Marshal(pinned)
	b, err2 := json.Marshal(sent)
	if err1 != nil || err2 != nil {
		return false
	}
	var x, y any
	_ = json.Unmarshal(a, &x)
	_ = json.Unmarshal(b, &y)
	ja, _ := json.Marshal(x)
	jb, _ := json.Marshal(y)
	return string(ja) == string(jb)
}
