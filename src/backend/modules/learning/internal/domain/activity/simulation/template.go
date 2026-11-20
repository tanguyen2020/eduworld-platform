// Package simulation holds the pure rules of activity_type simulation (F066, SIMULATION_ENGINE, ADR-0052): the
// template catalog (descriptors shipped with the code), the config of a version and its checks, the seed policy, the
// action log limits, the deterministic checkers that replay a run on the server and the goal evaluation. The CMS only
// parameterizes a packaged template; no script or code is ever loaded from content. No I/O.
package simulation

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
)

// Template statuses (content.simulation_templates.status).
const (
	StatusActive  = "active"
	StatusRetired = "retired"
)

// ActionDef is a descriptor action with the JSON Schema of its arguments (editor, client).
type ActionDef struct {
	Action     string          `json:"action"`
	ArgsSchema json.RawMessage `json:"args_schema,omitempty"`
}

// MetricDef is a metric the checker computes; display metrics are returned to the student after a run.
type MetricDef struct {
	Metric  string `json:"metric"`
	Unit    string `json:"unit,omitempty"`
	Display bool   `json:"display"`
}

// Descriptor describes a packaged template version (research R2): templates/<code>/v<n>/descriptor.json.
type Descriptor struct {
	TemplateCode     string             `json:"template_code"`
	TemplateVersion  int                `json:"template_version"`
	SimulationKind   string             `json:"simulation_kind"`
	Title            map[string]string  `json:"title"`
	MinClientVersion string             `json:"min_client_version"`
	Status           string             `json:"status"`
	CheckerAvailable bool               `json:"checker_available"`
	Constants        map[string]float64 `json:"constants,omitempty"`
	ParameterSchema  json.RawMessage    `json:"parameter_schema"`
	Actions          []ActionDef        `json:"actions"`
	Metrics          []MetricDef        `json:"metrics"`
}

// HasAction reports an action of the template.
func (d Descriptor) HasAction(a string) bool {
	for _, x := range d.Actions {
		if x.Action == a {
			return true
		}
	}
	return false
}

// Metric returns a metric of the template.
func (d Descriptor) Metric(m string) (MetricDef, bool) {
	for _, x := range d.Metrics {
		if x.Metric == m {
			return x, true
		}
	}
	return MetricDef{}, false
}

//go:embed templates
var templateFS embed.FS

var catalog = mustLoad()

func mustLoad() []Descriptor {
	out, err := load(templateFS)
	if err != nil {
		panic(err)
	}
	return out
}

func load(fsys fs.FS) ([]Descriptor, error) {
	var out []Descriptor
	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "descriptor.json" {
			return err
		}
		b, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		var desc Descriptor
		if err := json.Unmarshal(b, &desc); err != nil {
			return fmt.Errorf("simulation: %s: %w", path, err)
		}
		if desc.TemplateCode == "" || desc.TemplateVersion < 1 || (desc.Status != StatusActive && desc.Status != StatusRetired) {
			return fmt.Errorf("simulation: %s: incomplete descriptor", path)
		}
		out = append(out, desc)
		return nil
	})
	sort.Slice(out, func(i, j int) bool {
		if out[i].TemplateCode != out[j].TemplateCode {
			return out[i].TemplateCode < out[j].TemplateCode
		}
		return out[i].TemplateVersion < out[j].TemplateVersion
	})
	return out, err
}

// Catalog returns the template versions shipped with this release (listSimulationTemplates), by code and version.
func Catalog() []Descriptor { return append([]Descriptor(nil), catalog...) }

// Lookup returns one template version (active or retired: a running attempt keeps its template, Edge Cases).
func Lookup(code string, version int) (Descriptor, bool) {
	for _, d := range catalog {
		if d.TemplateCode == code && d.TemplateVersion == version {
			return d, true
		}
	}
	return Descriptor{}, false
}

// Resolve returns the template version a version of the content plays: the highest active version ≥ minVersion
// [Proposed F066]. False when the template is unknown, retired or older than minVersion.
func Resolve(code string, minVersion int) (Descriptor, bool) {
	var best Descriptor
	found := false
	for _, d := range catalog {
		if d.TemplateCode == code && d.TemplateVersion >= minVersion && d.Status == StatusActive && (!found || d.TemplateVersion > best.TemplateVersion) {
			best, found = d, true
		}
	}
	return best, found
}

// Ready reports that every template that declares a checker has one registered (readiness, research R2).
func Ready() error {
	for _, d := range catalog {
		if _, ok := CheckerFor(d.TemplateCode, d.TemplateVersion); d.CheckerAvailable && !ok {
			return fmt.Errorf("simulation: template %s v%d declares a checker but none is registered", d.TemplateCode, d.TemplateVersion)
		}
	}
	return nil
}
