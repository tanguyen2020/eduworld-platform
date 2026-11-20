package videolesson

// RenderPayload is ActivityRender.render_payload of a video_lesson version (video-lesson.schema.json#/$defs/render,
// F065-AC-02/03): chapters, cues with their item position, branch points without their target segments, segments,
// subtitle languages, the interaction and the completion rule the client shows. Never an answer or a target.
func RenderPayload(c Config) map[string]any {
	chapters := make([]map[string]any, 0, len(c.Content.Chapters))
	for _, ch := range c.Content.Chapters {
		chapters = append(chapters, map[string]any{"chapter_id": ch.ChapterID, "title": ch.Title, "start_ms": ch.StartMS, "end_ms": ch.EndMS})
	}
	cues := make([]map[string]any, 0, len(c.Content.Cues))
	for i, q := range c.OrderedCues() {
		cues = append(cues, map[string]any{"cue_id": q.CueID, "at_ms": q.AtMS, "position": i + 1, "required": q.IsRequired(),
			"allow_skip_after_wrong": q.SkipAfterWrong(), "pause": q.Pauses()})
	}
	branches := make([]map[string]any, 0, len(c.Content.Branches))
	for _, b := range c.Content.Branches {
		opts := make([]map[string]any, 0, len(b.Options))
		for _, o := range b.Options {
			opts = append(opts, map[string]any{"option_id": o.OptionID, "label": o.Label})
		}
		branches = append(branches, map[string]any{"branch_point_id": b.BranchPointID, "at_ms": b.AtMS, "prompt": b.Prompt, "options": opts})
	}
	segments := make([]map[string]any, 0, len(c.Content.Segments))
	for _, s := range c.Content.Segments {
		segments = append(segments, map[string]any{"segment_id": s.SegmentID, "is_ending": s.IsEnding})
	}
	subtitles := make([]map[string]any, 0, len(c.Content.Subtitles))
	for _, s := range c.Content.Subtitles {
		subtitles = append(subtitles, map[string]any{"language": s.Language, "is_default": s.IsDefault})
	}
	toggle := c.Interaction.AllowSubtitlesToggle == nil || *c.Interaction.AllowSubtitlesToggle
	autoplay := c.Interaction.AutoplayNextChapter == nil || *c.Interaction.AutoplayNextChapter
	return map[string]any{
		"duration_ms": max(c.Content.DurationMS, 0), "heartbeat_seconds": DefaultHeartbeatSeconds,
		"chapters": chapters, "cues": cues, "branches": branches, "segments": segments, "subtitles": subtitles,
		"interaction": map[string]any{"allow_seek": c.Seek(), "playback_rates": c.Rates(), "allow_subtitles_toggle": toggle,
			"autoplay_next_chapter": autoplay},
		"completion": map[string]any{"min_watched_ratio": c.MinWatchedRatio(), "require_required_cues": c.RequireRequiredCues()},
	}
}
