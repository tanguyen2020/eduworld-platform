package gameconfig

import (
	"slices"
	"testing"
)

// F079-AC-16: render_status of a version from its ready thumbnail and its render_thumbnail jobs.
func TestThumbnailState(t *testing.T) {
	for _, c := range []struct {
		ready bool
		job   RenderStatus
		want  string
	}{
		{true, "", ThumbnailReady},
		{true, RenderDead, ThumbnailReady},
		{false, RenderQueued, ThumbnailPending},
		{false, RenderFailed, ThumbnailPending},
		{false, RenderDead, ThumbnailFailed},
		{false, "", ThumbnailNotApplicable},
		{false, RenderDone, ThumbnailNotApplicable}, // the render asset is gone: no image to show
	} {
		if got := ThumbnailState(c.ready, c.job); got != c.want {
			t.Errorf("ready=%v job=%q: %s, want %s", c.ready, c.job, got, c.want)
		}
	}
}

// F079-AC-07: the reference pickers offer the values every registered platform build holds (the same set submit_review
// checks against each newest manifest).
func TestIntersectManifests(t *testing.T) {
	if m := IntersectManifests(nil); m.Areas == nil || len(m.Areas)+len(m.NPCs)+len(m.Scenes)+len(m.Interactables) != 0 {
		t.Fatalf("no manifest: %+v", m)
	}
	win := ClientManifest{Areas: []string{"main_hub", "learning_zone.math", "main_hub"}, NPCs: []string{"npc.b", "npc.a"},
		Scenes: []string{"res://a.tscn"}, Interactables: []string{"i1", "i2"}}
	web := ClientManifest{Areas: []string{"learning_zone.math", "main_hub", "web_only"}, NPCs: []string{"npc.a"}, Scenes: nil,
		Interactables: []string{"i2", "i3"}}
	m := IntersectManifests([]ClientManifest{win, web})
	if !slices.Equal(m.Areas, []string{"learning_zone.math", "main_hub"}) || !slices.Equal(m.NPCs, []string{"npc.a"}) ||
		len(m.Scenes) != 0 || m.Scenes == nil || !slices.Equal(m.Interactables, []string{"i2"}) {
		t.Fatalf("intersection: %+v", m)
	}
	if one := IntersectManifests([]ClientManifest{win}); !slices.Equal(one.Areas, []string{"learning_zone.math", "main_hub"}) ||
		!slices.Equal(one.NPCs, []string{"npc.a", "npc.b"}) {
		t.Fatalf("one manifest (sorted, unique): %+v", one)
	}
	// Whatever the intersection holds passes Missing of every manifest.
	refs := Refs{Areas: m.Areas, NPCs: m.NPCs, Scenes: m.Scenes, Interactables: m.Interactables}
	if len(win.Missing(refs))+len(web.Missing(refs)) != 0 {
		t.Fatal("an intersected value fails submit_review")
	}
}

// F079-AC-10, AC-15: the weekly progress of a simulation.
func TestSimulateWeeks(t *testing.T) {
	pts := SimulateWeeks(300, 5, 3, func(total int64) int { return int(total/1000) + 1 })
	want := []SimPoint{{Week: 1, Level: 2, Total: 1500}, {Week: 2, Level: 4, Total: 3000}, {Week: 3, Level: 5, Total: 4500}}
	if !slices.Equal(pts, want) {
		t.Fatalf("points %+v, want %+v", pts, want)
	}
}
