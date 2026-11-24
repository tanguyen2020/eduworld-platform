package gameconfig

import (
	"slices"
)

// Thumbnail states of a version (GameConfigVersion.render_status, F079-AC-16).
const (
	ThumbnailReady         = "ready"
	ThumbnailPending       = "pending"
	ThumbnailFailed        = "failed"
	ThumbnailNotApplicable = "not_applicable"
)

// ThumbnailState is the render_status of a version: ready when a ready render_thumbnail asset is linked to it;
// otherwise from its render_thumbnail job — queued or failed (a retry is pending) → pending, dead (retries exhausted)
// → failed; no job, or a done job whose asset is gone, → not_applicable (a version that is not published or has no 3D
// model gets no render).
func ThumbnailState(ready bool, job RenderStatus) string {
	switch {
	case ready:
		return ThumbnailReady
	case job == RenderQueued || job == RenderFailed:
		return ThumbnailPending
	case job == RenderDead:
		return ThumbnailFailed
	}
	return ThumbnailNotApplicable
}

// IntersectManifests is the client manifest the reference pickers of the console offer (getGameClientManifest,
// F079-AC-07): the values present in the newest manifest of every registered platform — exactly what passes Missing of
// each of them at submit_review. Lists are sorted and unique; no manifest gives empty lists.
func IntersectManifests(ms []ClientManifest) ClientManifest {
	out := ClientManifest{Areas: []string{}, NPCs: []string{}, Scenes: []string{}, Interactables: []string{}}
	if len(ms) == 0 {
		return out
	}
	inter := func(pick func(ClientManifest) []string) []string {
		res := []string{}
		for _, v := range uniqueSorted(pick(ms[0])) {
			ok := true
			for _, m := range ms[1:] {
				if !slices.Contains(pick(m), v) {
					ok = false
					break
				}
			}
			if ok {
				res = append(res, v)
			}
		}
		return res
	}
	out.Areas = inter(func(m ClientManifest) []string { return m.Areas })
	out.NPCs = inter(func(m ClientManifest) []string { return m.NPCs })
	out.Scenes = inter(func(m ClientManifest) []string { return m.Scenes })
	out.Interactables = inter(func(m ClientManifest) []string { return m.Interactables })
	return out
}

func uniqueSorted(in []string) []string {
	out := slices.Clone(in)
	slices.Sort(out)
	return slices.Compact(out)
}

// SimPoint is the progress at the end of a simulated week (simulateGameConfig points[]).
type SimPoint struct {
	Week  int
	Level int
	Total int64
}

// SimulateWeeks is the weekly progress of a simulation (F079-AC-10, AC-15): after week w the total is perPlayDay ×
// daysPerWeek × w and the level is levelAt(total) (the curve level or the season tier).
func SimulateWeeks(perPlayDay int64, daysPerWeek, weeks int, levelAt func(total int64) int) []SimPoint {
	out := make([]SimPoint, 0, weeks)
	for w := 1; w <= weeks; w++ {
		total := perPlayDay * int64(daysPerWeek) * int64(w)
		out = append(out, SimPoint{Week: w, Level: levelAt(total), Total: total})
	}
	return out
}
