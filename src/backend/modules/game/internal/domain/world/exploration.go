package world

import (
	"math"
	"slices"
)

// Exploration is the exploration_progress of a student in an area.
type Exploration struct {
	Discoveries  []string
	Collectibles []string
	Completed    bool // completed_at set once
}

// Record adds a found interactable (F071-AC-24): false when it was already found (409 DISCOVERY_ALREADY_FOUND).
func (e *Exploration) Record(id, kind string) bool {
	list := &e.Discoveries
	if kind == KindCollectible {
		list = &e.Collectibles
	}
	if slices.Contains(*list, id) {
		return false
	}
	*list = append(*list, id)
	return true
}

// Found reports an id already found.
func (e Exploration) Found(id string) bool {
	return slices.Contains(e.Discoveries, id) || slices.Contains(e.Collectibles, id)
}

// Completion is the percentage of the current area version found (WORLD_DESIGN §8): found anchors that still exist in
// the layout over the anchors of the layout, two decimals, never above 100 (an anchor removed by a new version keeps
// its id in the progress but no longer counts). The quest part of the percentage is F072. An area without anchors is 0.
func (e Exploration) Completion(a Area) Completion {
	var c Completion
	for _, it := range a.Layout.Interactables {
		switch it.Kind {
		case KindDiscovery:
			c.DiscoveriesTotal++
			if slices.Contains(e.Discoveries, it.ID) {
				c.DiscoveriesFound++
			}
		case KindCollectible:
			c.CollectiblesTotal++
			if slices.Contains(e.Collectibles, it.ID) {
				c.CollectiblesFound++
			}
		}
	}
	found, total := c.DiscoveriesFound+c.CollectiblesFound, c.DiscoveriesTotal+c.CollectiblesTotal
	if total > 0 {
		c.Pct = math.Min(100, math.Round(float64(found)*10000/float64(total))/100)
	}
	return c
}

// Completion is the exploration figures of an area version.
type Completion struct {
	Pct                                  float64
	DiscoveriesFound, DiscoveriesTotal   int
	CollectiblesFound, CollectiblesTotal int
}
