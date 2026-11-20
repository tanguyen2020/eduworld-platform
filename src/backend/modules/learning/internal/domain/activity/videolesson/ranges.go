package videolesson

import (
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
)

// MaxRanges is the bound of watched_ranges (video-lesson / ActivityProgressReport maxItems 64).
const MaxRanges = 64

// Range is a watched span [StartMS, EndMS) of the video.
type Range struct {
	StartMS int64 `json:"start_ms"`
	EndMS   int64 `json:"end_ms"`
}

// Merge sorts, clips to [0, duration] (duration ≤ 0 = unknown) and joins overlapping or touching ranges; empty ranges
// are dropped. Beyond MaxRanges the ranges separated by the smallest gaps are joined.
func Merge(rs []Range, duration int64) []Range {
	var in []Range
	for _, r := range rs {
		s, e := max(r.StartMS, 0), r.EndMS
		if duration > 0 {
			e = min(e, duration)
		}
		if e > s {
			in = append(in, Range{s, e})
		}
	}
	sort.Slice(in, func(i, j int) bool { return in[i].StartMS < in[j].StartMS })
	var out []Range
	for _, r := range in {
		if n := len(out); n > 0 && r.StartMS <= out[n-1].EndMS {
			out[n-1].EndMS = max(out[n-1].EndMS, r.EndMS)
			continue
		}
		out = append(out, r)
	}
	for len(out) > MaxRanges {
		best := 1
		for i := 2; i < len(out); i++ {
			if out[i].StartMS-out[i-1].EndMS < out[best].StartMS-out[best-1].EndMS {
				best = i
			}
		}
		out[best-1].EndMS = out[best].EndMS
		out = append(out[:best], out[best+1:]...)
	}
	return out
}

// WatchedMS is the total watched time of merged ranges.
func WatchedMS(rs []Range) int64 {
	var n int64
	for _, r := range rs {
		n += r.EndMS - r.StartMS
	}
	return n
}

// Ratio is watched ÷ duration in [0, 1] (0 when the duration is unknown).
func Ratio(rs []Range, duration int64) float64 {
	if duration <= 0 {
		return 0
	}
	return min(1, float64(WatchedMS(rs))/float64(duration))
}

// Frontier is the edge of the accepted watched part: the furthest end of the ranges (0 when nothing is watched).
func Frontier(rs []Range) int64 {
	var f int64
	for _, r := range rs {
		f = max(f, r.EndMS)
	}
	return f
}

// subtract returns the parts of r not covered by the merged ranges cover.
func subtract(r Range, cover []Range) []Range {
	out := []Range{r}
	for _, c := range cover {
		var next []Range
		for _, x := range out {
			if c.EndMS <= x.StartMS || c.StartMS >= x.EndMS {
				next = append(next, x)
				continue
			}
			if c.StartMS > x.StartMS {
				next = append(next, Range{x.StartMS, c.StartMS})
			}
			if c.EndMS < x.EndMS {
				next = append(next, Range{c.EndMS, x.EndMS})
			}
		}
		out = next
	}
	return out
}

// Accepted is the outcome of a heartbeat after the plausibility check.
type Accepted struct {
	Ranges    []Range
	AddedMS   int64
	TrimmedMS int64
}

// AcceptRanges merges the ranges of a heartbeat into the stored ones (F065-AC-05, research R2): the newly watched time
// cannot exceed the real time since the previous heartbeat × the fastest playback rate; the earliest new parts are
// kept and the rest is trimmed (an integrity signal, never a penalty).
func AcceptRanges(prev, incoming []Range, elapsedMS int64, maxRate float64, duration int64) Accepted {
	prev = Merge(prev, duration)
	budget := int64(float64(max(elapsedMS, 0)) * max(maxRate, 1))
	var fresh []Range
	for _, r := range Merge(incoming, duration) {
		fresh = append(fresh, subtract(r, prev)...)
	}
	sort.Slice(fresh, func(i, j int) bool { return fresh[i].StartMS < fresh[j].StartMS })
	out := Accepted{}
	kept := prev
	for _, r := range fresh {
		n := r.EndMS - r.StartMS
		take := min(n, budget-out.AddedMS)
		if take > 0 {
			kept = append(kept, Range{r.StartMS, r.StartMS + take})
			out.AddedMS += take
		}
		out.TrimmedMS += n - max(take, 0)
	}
	out.Ranges = Merge(kept, duration)
	return out
}

// ErrSeekNotAllowed: the position is beyond what the seek policy allows (422 VIDEO_SEEK_NOT_ALLOWED).
var ErrSeekNotAllowed = errors.New("videolesson: seek beyond the watched part")

// CheckSeek applies allow_seek to a reported position (F065-AC-06, VIDEO_LEARNING §7): watched_only and backward_only
// refuse a position past the frontier of the accepted ranges until the attempt is completed; free accepts any position
// (the watched ratio still counts the plausible part only). The frontier includes the ranges of this heartbeat.
func CheckSeek(policy string, positionMS, frontierMS int64, completed bool) error {
	if completed || policy == SeekFree {
		return nil
	}
	if positionMS > frontierMS {
		return ErrSeekNotAllowed
	}
	return nil
}

// Milestones are the watched ratios of video.progressed (VIDEO_LEARNING §10), one bit each.
var Milestones = []float64{0.25, 0.5, 0.75, 1}

// NewMilestones returns the milestones reached by ratio not yet in emitted (bitmask) and the new bitmask.
func NewMilestones(ratio float64, emitted int) ([]float64, int) {
	var out []float64
	for i, m := range Milestones {
		bit := 1 << i
		if emitted&bit == 0 && ratio+1e-9 >= m {
			out = append(out, m)
			emitted |= bit
		}
	}
	return out, emitted
}

// Progress is the stored progress of a media attempt (assessment.video_progress, one row per attempt; shared with
// audio_lesson in F066).
type Progress struct {
	AttemptID       uuid.UUID
	TenantID        uuid.UUID
	SchoolID        uuid.UUID
	PositionMS      int64
	Ranges          []Range
	Ratio           float64
	LastHeartbeatAt time.Time
	ClientSeq       int64
	// Milestones is the bitmask of the video.progressed milestones already written (25/50/75/100 %).
	Milestones int
	// ProcessedCues are the cues closed by an answer (CueProcessed), in answer order.
	ProcessedCues []string
	// SegmentID is the segment chosen at the last branch point (empty = main video).
	SegmentID string
	// ReachedEnding: a branch choice led to an ending segment.
	ReachedEnding bool
	Version       int64
	// PlaysUsed are the listening plays accepted by the server (audio_lesson, F066 research R1; always 0 for video).
	PlaysUsed int
}

// NewProgress is the progress row of a started attempt (position 0, nothing watched).
func NewProgress(attemptID, tenantID, schoolID uuid.UUID, now time.Time) Progress {
	return Progress{AttemptID: attemptID, TenantID: tenantID, SchoolID: schoolID, Ranges: []Range{}, LastHeartbeatAt: now,
		ProcessedCues: []string{}}
}
