package schedule

import (
	"fmt"
	"time"
)

// Planner computes the desired replica count of every scheduled Swarm service at a given instant
// (F130-AC-22): peak from lead_minutes before a window starts until the window ends, baseline otherwise.
type Planner struct {
	s   *Schedule
	loc *time.Location
}

// NewPlanner binds a validated schedule to the ICT location.
func NewPlanner(s *Schedule) (*Planner, error) {
	loc, err := time.LoadLocation(Timezone)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", Timezone, err)
	}
	return &Planner{s: s, loc: loc}, nil
}

// Desired is the plan of one service at an instant.
type Desired struct {
	Service  string
	Replicas int
	Window   string // window name when in (or warming up for) a peak window, "" for baseline
}

// Next returns the desired replicas of every scheduled service at now.
func (p *Planner) Next(now time.Time) []Desired {
	window := p.activeWindow(now)
	out := make([]Desired, 0, len(p.s.Services))
	for _, svc := range p.s.Services {
		d := Desired{Service: svc.Name, Replicas: svc.Baseline}
		if window != "" {
			d.Replicas, d.Window = svc.Peak, window
		}
		out = append(out, d)
	}
	return out
}

// activeWindow returns the window whose [start - lead, end) contains now (ICT), or "".
func (p *Planner) activeWindow(now time.Time) string {
	t := now.In(p.loc)
	minute := t.Hour()*60 + t.Minute()
	for _, w := range p.s.Windows {
		a, b := w.Minutes()
		if minute >= a-p.s.LeadMinutes && minute < b {
			return w.Name
		}
	}
	return ""
}
