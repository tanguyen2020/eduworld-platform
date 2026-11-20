package audiolesson

import (
	"errors"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/videolesson"
)

// ErrPlayLimit: a new play is reported while max_plays plays are already used (422 AUDIO_PLAY_LIMIT_REACHED).
var ErrPlayLimit = errors.New("audiolesson: play limit reached")

// AcceptPlay counts a play (research R1): a play is a start from 0 or a replay after the end, reported once per new
// client_seq with play_started; pause, resume and seeking inside a play are not plays. With max_plays set, a play past
// the limit is refused and nothing changes; without it plays are unlimited.
func AcceptPlay(used int, maxPlays *int) (int, error) {
	if maxPlays != nil && used >= *maxPlays {
		return used, ErrPlayLimit
	}
	return used + 1, nil
}

// PlaysLeft is the plays left (-1 = unlimited).
func PlaysLeft(used int, maxPlays *int) int {
	if maxPlays == nil {
		return -1
	}
	return max(*maxPlays-used, 0)
}

// RenderPayload is ActivityRender.render_payload of an audio_lesson version (audio-lesson.schema.json#/$defs/render,
// F066-AC-02/03): segments, the questions with their item position and time, the interaction and the completion the
// client shows. plays_used, resume_position_ms and the probed duration are added from the attempt by the engine.
// Never an answer; the transcript is a media reference only when show_transcript allows it.
func RenderPayload(c Config) map[string]any {
	segments := make([]map[string]any, 0, len(c.Content.Segments))
	for _, s := range c.Content.Segments {
		seg := map[string]any{"segment_id": s.SegmentID, "start_ms": s.StartMS, "end_ms": s.EndMS}
		if len(s.Title) > 0 {
			seg["title"] = s.Title
		}
		segments = append(segments, seg)
	}
	tl := c.Timeline()
	questions := make([]map[string]any, 0, len(tl.Content.Cues))
	for i, q := range tl.OrderedCues() {
		item := map[string]any{"cue_id": q.CueID, "position": i + 1}
		if q.AtMS != AfterEnd {
			item["at_ms"] = q.AtMS
		}
		questions = append(questions, item)
	}
	var maxPlays any
	if c.Interaction.MaxPlays != nil {
		maxPlays = *c.Interaction.MaxPlays
	}
	return map[string]any{
		"duration_ms": int64(0), "heartbeat_seconds": videolesson.DefaultHeartbeatSeconds, "segments": segments, "questions": questions,
		"interaction": map[string]any{"max_plays": maxPlays, "allow_speed_change": c.SpeedChange(), "show_transcript": c.Transcript(),
			"playback_rates": c.Rates()},
		"plays_used": 0,
		"completion": map[string]any{"min_watched_ratio": tl.MinWatchedRatio()},
	}
}
