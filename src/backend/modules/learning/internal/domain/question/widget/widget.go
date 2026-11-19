// Package widget is the registry of the interactive question widgets (F023 US3, QUESTION_BANK §3.11, FR-QB-009):
// each widget version has a configuration check, a replay of the client action log and pure Go validators
// registered at build time. Nothing written by an author is ever executed. Pure Go, no I/O.
package widget

import (
	"bytes"
	"encoding/json"
	"errors"
	"sort"
)

// Errors of a widget.
var (
	ErrUnknown       = errors.New("widget: not registered")
	ErrBadConfig     = errors.New("widget: invalid config")
	ErrIllegalAction = errors.New("widget: illegal action")
)

// Action is one entry of an action log (interactiveResponse.action_log).
type Action struct {
	TMs    int            `json:"t_ms"`
	Action string         `json:"action"`
	Args   map[string]any `json:"args,omitempty"`
}

// Widget is one registered widget version.
type Widget interface {
	Code() string
	Version() string
	// ConfigSchema is the JSON Schema of config and answer_key.params (specs/schemas/question-widgets).
	ConfigSchema() json.RawMessage
	// Validators lists the validator names (<code>.<name>).
	Validators() []string
	// CheckConfig validates config and the params of validator (F023-AC-09); reasons are safe to show.
	CheckConfig(config json.RawMessage, validator string, params json.RawMessage) []string
	// Replay rebuilds the final state from config and the action log (F023-AC-11); illegal actions are errors.
	Replay(config json.RawMessage, log []Action) (json.RawMessage, error)
	// Same reports whether two states are equal for the widget.
	Same(a, b json.RawMessage) bool
	// Check runs validator on a state.
	Check(validator string, config, params, state json.RawMessage) (bool, error)
}

// Registry holds the widgets by code and version.
type Registry struct{ widgets map[string]Widget }

// NewRegistry registers ws.
func NewRegistry(ws ...Widget) *Registry {
	r := &Registry{widgets: map[string]Widget{}}
	for _, w := range ws {
		r.widgets[w.Code()+"@"+w.Version()] = w
	}
	return r
}

// Default is the registry of this release: balance_scale@1.0.
func Default() *Registry { return NewRegistry(BalanceScaleV1{}) }

// Get returns a widget version.
func (r *Registry) Get(code, version string) (Widget, error) {
	if r == nil {
		return nil, ErrUnknown
	}
	w, ok := r.widgets[code+"@"+version]
	if !ok {
		return nil, ErrUnknown
	}
	return w, nil
}

// Info describes a widget version (listQuestionWidgets).
type Info struct {
	Widget        string          `json:"widget"`
	WidgetVersion string          `json:"widget_version"`
	ConfigSchema  json.RawMessage `json:"config_schema"`
	Validators    []string        `json:"validators"`
}

// List returns the registered widgets sorted by code and version.
func (r *Registry) List() []Info {
	out := make([]Info, 0, len(r.widgets))
	for _, w := range r.widgets {
		out = append(out, Info{Widget: w.Code(), WidgetVersion: w.Version(), ConfigSchema: w.ConfigSchema(), Validators: w.Validators()})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Widget != out[j].Widget {
			return out[i].Widget < out[j].Widget
		}
		return out[i].WidgetVersion < out[j].WidgetVersion
	})
	return out
}

func bytesReader(b []byte) *bytes.Reader { return bytes.NewReader(b) }
