// Package schedule is the domain of ops-scheduler (F130 US5, DEPLOYMENT §4.4): the scale schedule of one
// environment (deploy/swarm/<env>/scale-schedule.yaml) and its validation rules. Schema of the file:
// deploy/swarm/schema/scale-schedule.schema.json (the CI check of deploy-config.yml validates the same rules).
package schedule

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Timezone of every window: school hours are ICT (warm_up_windows, CONFIG_DEFAULTS).
const Timezone = "Asia/Ho_Chi_Minh"

// Limits mirror the JSON schema (ADR-0041: every string is bounded).
const (
	maxStackLen   = 32
	maxWindowLen  = 32
	maxServiceLen = 64
	maxWindows    = 12
	maxServices   = 64
	maxReplicas   = 64
	maxLead       = 60
)

var (
	nameRe = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	hhmmRe = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)
)

// Window is one peak window of the day, [Start, End) in ICT.
type Window struct {
	Name  string `yaml:"name"`
	Start string `yaml:"start"`
	End   string `yaml:"end"`
}

// Minutes returns [start, end) of the window in minutes after midnight.
func (w Window) Minutes() (int, int) { return hhmm(w.Start), hhmm(w.End) }

// Service is the replica plan of one Swarm service of the stack.
type Service struct {
	Name     string `yaml:"name"`
	Baseline int    `yaml:"baseline"`
	Peak     int    `yaml:"peak"`
}

// Schedule is the whole file.
type Schedule struct {
	Version     int       `yaml:"version"`
	Stack       string    `yaml:"stack"`
	Timezone    string    `yaml:"timezone"`
	LeadMinutes int       `yaml:"lead_minutes"`
	Windows     []Window  `yaml:"windows"`
	Services    []Service `yaml:"services"`
}

// Problem is one validation error with its position in the file.
type Problem struct {
	Line, Column int
	Path         string
	Message      string
}

func (p Problem) String() string {
	return fmt.Sprintf("line %d col %d %s: %s", p.Line, p.Column, p.Path, p.Message)
}

// ValidationError lists every problem of a schedule file.
type ValidationError struct{ Problems []Problem }

func (e *ValidationError) Error() string {
	out := make([]string, 0, len(e.Problems))
	for _, p := range e.Problems {
		out = append(out, p.String())
	}
	return "invalid scale schedule: " + strings.Join(out, "; ")
}

// Parse decodes and validates a schedule. knownServices are the Swarm service names of the environment stacks
// (nil skips that check, e.g. when ops-scheduler runs without the stack files).
func Parse(data []byte, knownServices map[string]bool) (*Schedule, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("scale schedule: %w", err)
	}
	var s Schedule
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&s); err != nil {
		return nil, &ValidationError{Problems: []Problem{{Path: "/", Message: err.Error()}}}
	}
	pos := positions(&root)
	if probs := s.validate(knownServices, pos); len(probs) > 0 {
		return nil, &ValidationError{Problems: probs}
	}
	return &s, nil
}

func hhmm(s string) int {
	var h, m int
	_, _ = fmt.Sscanf(s, "%d:%d", &h, &m)
	return h*60 + m
}

func (s *Schedule) validate(known map[string]bool, pos posMap) []Problem {
	var out []Problem
	add := func(path, format string, a ...any) {
		l, c := pos.at(path)
		out = append(out, Problem{Line: l, Column: c, Path: path, Message: fmt.Sprintf(format, a...)})
	}
	if s.Version != 1 {
		add("/version", "must be 1")
	}
	checkName(add, "/stack", s.Stack, maxStackLen)
	if s.Timezone != "" && s.Timezone != Timezone {
		add("/timezone", "must be %s", Timezone)
	}
	if s.LeadMinutes < 0 || s.LeadMinutes > maxLead {
		add("/lead_minutes", "must be between 0 and %d", maxLead)
	}
	if len(s.Windows) == 0 || len(s.Windows) > maxWindows {
		add("/windows", "must have 1..%d windows", maxWindows)
	}
	type span struct{ a, b, i int }
	var spans []span
	for i, w := range s.Windows {
		base := fmt.Sprintf("/windows/%d", i)
		checkName(add, base+"/name", w.Name, maxWindowLen)
		okStart, okEnd := hhmmRe.MatchString(w.Start), hhmmRe.MatchString(w.End)
		if !okStart {
			add(base+"/start", "must be HH:MM")
		}
		if !okEnd {
			add(base+"/end", "must be HH:MM")
		}
		if okStart && okEnd {
			a, b := w.Minutes()
			if a >= b {
				add(base+"/end", "must be after start (windows do not cross midnight)")
			} else {
				spans = append(spans, span{a - s.LeadMinutes, b, i})
			}
		}
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].a < spans[j].a })
	for i := 1; i < len(spans); i++ {
		if spans[i].a < spans[i-1].b {
			add(fmt.Sprintf("/windows/%d", spans[i].i), "overlaps window %q (lead_minutes included)", s.Windows[spans[i-1].i].Name)
		}
	}
	if len(s.Services) == 0 || len(s.Services) > maxServices {
		add("/services", "must have 1..%d services", maxServices)
	}
	seen := map[string]bool{}
	for i, svc := range s.Services {
		base := fmt.Sprintf("/services/%d", i)
		if checkName(add, base+"/name", svc.Name, maxServiceLen) {
			if seen[svc.Name] {
				add(base+"/name", "duplicate service %q", svc.Name)
			}
			seen[svc.Name] = true
			if known != nil && !known[svc.Name] {
				add(base+"/name", "service %q is not in the stack files of this environment", svc.Name)
			}
		}
		if svc.Baseline < 1 || svc.Baseline > maxReplicas {
			add(base+"/baseline", "must be between 1 and %d (missing?)", maxReplicas)
		}
		if svc.Peak < 1 || svc.Peak > maxReplicas {
			add(base+"/peak", "must be between 1 and %d (missing?)", maxReplicas)
		}
		if svc.Peak >= 1 && svc.Peak < svc.Baseline {
			add(base+"/peak", "peak %d is lower than baseline %d", svc.Peak, svc.Baseline)
		}
	}
	return out
}

func checkName(add func(string, string, ...any), path, v string, limit int) bool {
	switch {
	case v == "":
		add(path, "is required")
	case len(v) > limit:
		add(path, "longer than %d characters", limit)
	case !nameRe.MatchString(v):
		add(path, "must match %s", nameRe)
	default:
		return true
	}
	return false
}

// posMap maps JSON-pointer-like paths to the line/column of their YAML node (error positions, F130-AC-24).
type posMap map[string][2]int

// at returns the position of path, else of its nearest parent (a missing key points at its parent mapping).
func (m posMap) at(path string) (int, int) {
	for p := path; ; {
		if v, ok := m[p]; ok {
			return v[0], v[1]
		}
		i := strings.LastIndex(p, "/")
		if i <= 0 {
			v := m["/"]
			return v[0], v[1]
		}
		p = p[:i]
	}
}

func positions(root *yaml.Node) posMap {
	out := posMap{}
	var walk func(n *yaml.Node, path string)
	walk = func(n *yaml.Node, path string) {
		key := path
		if key == "" {
			key = "/"
		}
		if _, ok := out[key]; !ok {
			out[key] = [2]int{n.Line, n.Column}
		}
		switch n.Kind {
		case yaml.DocumentNode:
			for _, c := range n.Content {
				walk(c, path)
			}
		case yaml.MappingNode:
			for i := 0; i+1 < len(n.Content); i += 2 {
				child := path + "/" + n.Content[i].Value
				out[child] = [2]int{n.Content[i].Line, n.Content[i].Column}
				walk(n.Content[i+1], child)
			}
		case yaml.SequenceNode:
			for i, c := range n.Content {
				walk(c, fmt.Sprintf("%s/%d", path, i))
			}
		}
	}
	walk(root, "")
	return out
}
