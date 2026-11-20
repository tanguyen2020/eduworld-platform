// Package videolesson holds the pure rules of activity_type video_lesson (F065, VIDEO_LEARNING, ADR-0048): the
// config of a version and its checks, the render payload of the client, the watched ranges with their plausibility
// check and seek policy, and the completion rule. No I/O; the plugin and the attempt engine call it.
package videolesson

import (
	"encoding/json"
	"errors"
	"slices"
	"sort"
	"strconv"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

// Seek policies (interaction.allow_seek).
const (
	SeekFree         = "free"
	SeekBackwardOnly = "backward_only"
	SeekWatchedOnly  = "watched_only"
)

// Defaults of a version that does not set them (VIDEO_LEARNING §4, §7; CONFIG_DEFAULTS video_completion: watched ≥ 90 %
// + required cues answered, heartbeat 10 s).
const (
	DefaultMinWatchedRatio  = 0.9
	DefaultHeartbeatSeconds = 10
	// MaxChapters, MaxCues, MaxSegments, MaxBranches are the maxItems of video-lesson.schema.json.
	MaxChapters = 50
	MaxCues     = 30
	MaxSegments = 20
	MaxBranches = 20
)

// DefaultPlaybackRates is interaction.playback_rates when the version has none (VIDEO_LEARNING §3: 0.75, 1, 1.25).
var DefaultPlaybackRates = []float64{0.75, 1, 1.25}

// I18n is an i18nShort text ({vi, en}) kept as sent for the client.
type I18n map[string]string

// Chapter is content.chapters[].
type Chapter struct {
	ChapterID    string      `json:"chapter_id"`
	Title        I18n        `json:"title"`
	StartMS      int64       `json:"start_ms"`
	EndMS        int64       `json:"end_ms"`
	ObjectiveIDs []uuid.UUID `json:"objective_ids,omitempty"`
}

// Subtitle is content.subtitles[].
type Subtitle struct {
	Language        string    `json:"language"`
	SubtitleAssetID uuid.UUID `json:"subtitle_asset_id"`
	IsDefault       bool      `json:"is_default,omitempty"`
}

// Cue is content.embedded_questions[] (an embedded question, VIDEO_LEARNING §5).
type Cue struct {
	CueID               string    `json:"cue_id"`
	AtMS                int64     `json:"at_ms"`
	QuestionVersionID   uuid.UUID `json:"question_version_id"`
	Pause               *bool     `json:"pause,omitempty"`
	Required            *bool     `json:"required,omitempty"`
	AllowSkipAfterWrong *bool     `json:"allow_skip_after_wrong,omitempty"`
}

// IsRequired is required (default true).
func (c Cue) IsRequired() bool { return c.Required == nil || *c.Required }

// SkipAfterWrong is allow_skip_after_wrong (default true).
func (c Cue) SkipAfterWrong() bool { return c.AllowSkipAfterWrong == nil || *c.AllowSkipAfterWrong }

// Pauses is pause (default true).
func (c Cue) Pauses() bool { return c.Pause == nil || *c.Pause }

// Segment is content.segments[] (an interactive video part, VIDEO_LEARNING §6).
type Segment struct {
	SegmentID    string    `json:"segment_id"`
	VideoAssetID uuid.UUID `json:"video_asset_id"`
	IsEnding     bool      `json:"is_ending,omitempty"`
}

// Option is branches[].options[].
type Option struct {
	OptionID        string `json:"option_id"`
	Label           I18n   `json:"label"`
	TargetSegmentID string `json:"target_segment_id"`
}

// Branch is content.branches[] (a branch point).
type Branch struct {
	BranchPointID string   `json:"branch_point_id"`
	AtMS          int64    `json:"at_ms"`
	Prompt        I18n     `json:"prompt"`
	Options       []Option `json:"options"`
}

// Content is config.content of video-lesson.schema.json.
type Content struct {
	VideoAssetID      uuid.UUID  `json:"video_asset_id"`
	DurationMS        int64      `json:"duration_ms,omitempty"`
	TranscriptAssetID *uuid.UUID `json:"transcript_asset_id,omitempty"`
	Chapters          []Chapter  `json:"chapters,omitempty"`
	Subtitles         []Subtitle `json:"subtitles,omitempty"`
	Cues              []Cue      `json:"embedded_questions,omitempty"`
	Segments          []Segment  `json:"segments,omitempty"`
	Branches          []Branch   `json:"branches,omitempty"`
}

// Interaction is config.interaction.
type Interaction struct {
	AllowSeek            string    `json:"allow_seek,omitempty"`
	PlaybackRates        []float64 `json:"playback_rates,omitempty"`
	AllowSubtitlesToggle *bool     `json:"allow_subtitles_toggle,omitempty"`
	AutoplayNextChapter  *bool     `json:"autoplay_next_chapter,omitempty"`
}

// Completion is the completion part of config.completion_rule used by video.
type Completion struct {
	Mode                string   `json:"mode"`
	MinWatchedRatio     *float64 `json:"min_watched_ratio,omitempty"`
	RequireRequiredCues *bool    `json:"require_required_cues,omitempty"`
	MinScoreRatio       *float64 `json:"min_score_ratio,omitempty"`
}

// Config is the config of a video_lesson version.
type Config struct {
	Content        Content     `json:"content"`
	Interaction    Interaction `json:"interaction"`
	CompletionRule Completion  `json:"completion_rule"`
}

// ErrConfig: the config cannot be decoded.
var ErrConfig = errors.New("videolesson: config cannot be read")

// Parse decodes a config already checked against its JSON Schema.
func Parse(raw json.RawMessage) (Config, error) {
	var c Config
	if len(raw) == 0 || json.Unmarshal(raw, &c) != nil {
		return c, ErrConfig
	}
	return c, nil
}

// Seek is allow_seek (default watched_only).
func (c Config) Seek() string {
	switch c.Interaction.AllowSeek {
	case SeekFree, SeekBackwardOnly:
		return c.Interaction.AllowSeek
	}
	return SeekWatchedOnly
}

// Rates are the playback rates of the player.
func (c Config) Rates() []float64 {
	if len(c.Interaction.PlaybackRates) == 0 {
		return slices.Clone(DefaultPlaybackRates)
	}
	return slices.Clone(c.Interaction.PlaybackRates)
}

// MaxRate is the fastest playback rate (plausibility of the watched ranges, research R2).
func (c Config) MaxRate() float64 {
	m := 1.0
	for _, r := range c.Rates() {
		m = max(m, r)
	}
	return m
}

// MinWatchedRatio is completion_rule.min_watched_ratio, else video_completion.
func (c Config) MinWatchedRatio() float64 {
	if r := c.CompletionRule.MinWatchedRatio; r != nil {
		return min(max(*r, 0), 1)
	}
	return DefaultMinWatchedRatio
}

// RequireRequiredCues is completion_rule.require_required_cues (default true, video_completion).
func (c Config) RequireRequiredCues() bool {
	r := c.CompletionRule.RequireRequiredCues
	return r == nil || *r
}

// OrderedCues are the embedded questions by at_ms (then cue_id): item position i+1 is cue i (research R1).
func (c Config) OrderedCues() []Cue {
	out := slices.Clone(c.Content.Cues)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].AtMS != out[j].AtMS {
			return out[i].AtMS < out[j].AtMS
		}
		return out[i].CueID < out[j].CueID
	})
	return out
}

// CuePosition returns the item position of a cue (0 when unknown).
func (c Config) CuePosition(cueID string) int {
	for i, q := range c.OrderedCues() {
		if q.CueID == cueID {
			return i + 1
		}
	}
	return 0
}

// CueAt returns the cue of an item position.
func (c Config) CueAt(position int) (Cue, bool) {
	cs := c.OrderedCues()
	if position < 1 || position > len(cs) {
		return Cue{}, false
	}
	return cs[position-1], true
}

// Branch returns a branch point.
func (c Config) Branch(id string) (Branch, bool) {
	for _, b := range c.Content.Branches {
		if b.BranchPointID == id {
			return b, true
		}
	}
	return Branch{}, false
}

// Segment returns a segment.
func (c Config) Segment(id string) (Segment, bool) {
	for _, s := range c.Content.Segments {
		if s.SegmentID == id {
			return s, true
		}
	}
	return Segment{}, false
}

// Interactive reports a branched video (completion at an ending segment, VIDEO_LEARNING §6).
func (c Config) Interactive() bool { return len(c.Content.Branches) > 0 }

// AssetIDs are the media of the version: main video, segment videos, subtitles (and transcript).
func (c Config) AssetIDs() []uuid.UUID {
	out := []uuid.UUID{c.Content.VideoAssetID}
	for _, s := range c.Content.Segments {
		out = append(out, s.VideoAssetID)
	}
	for _, s := range c.Content.Subtitles {
		out = append(out, s.SubtitleAssetID)
	}
	return out
}

// Validate checks what the JSON Schema cannot (F065-AC-01, VIDEO_LEARNING §3): cue and branch times inside the video,
// chapters ordered without overlap, unique ids, branch targets that exist. The question versions of the cues are
// checked by the use case (published, auto-graded).
func (c Config) Validate() contentmodel.Violations {
	var out contentmodel.Violations
	add := func(f, r string) { out = append(out, contentmodel.Violation{Field: "/config/content/" + f, Reason: r}) }
	d := c.Content.DurationMS
	chapters := slices.Clone(c.Content.Chapters)
	for i, ch := range c.Content.Chapters {
		p := "chapters/" + strconv.Itoa(i)
		if ch.StartMS >= ch.EndMS {
			add(p+"/end_ms", "must be after start_ms")
		}
		if d > 0 && ch.EndMS > d {
			add(p+"/end_ms", "is after the end of the video")
		}
	}
	sort.SliceStable(chapters, func(i, j int) bool { return chapters[i].StartMS < chapters[j].StartMS })
	for i := 1; i < len(chapters); i++ {
		if chapters[i].StartMS < chapters[i-1].EndMS {
			add("chapters", "chapters "+chapters[i-1].ChapterID+" and "+chapters[i].ChapterID+" overlap")
		}
	}
	dup := func(list string, ids []string) {
		seen := map[string]bool{}
		for i, id := range ids {
			if seen[id] {
				add(list+"/"+strconv.Itoa(i), "repeats the id "+id)
			}
			seen[id] = true
		}
	}
	var chIDs, cueIDs, segIDs, brIDs []string
	for _, ch := range c.Content.Chapters {
		chIDs = append(chIDs, ch.ChapterID)
	}
	for i, q := range c.Content.Cues {
		cueIDs = append(cueIDs, q.CueID)
		if d > 0 && q.AtMS >= d {
			add("embedded_questions/"+strconv.Itoa(i)+"/at_ms", "must be before the end of the video (duration_ms)")
		}
	}
	for _, s := range c.Content.Segments {
		segIDs = append(segIDs, s.SegmentID)
	}
	dup("chapters", chIDs)
	dup("embedded_questions", cueIDs)
	dup("segments", segIDs)
	for i, b := range c.Content.Branches {
		p := "branches/" + strconv.Itoa(i)
		brIDs = append(brIDs, b.BranchPointID)
		if d > 0 && b.AtMS >= d {
			add(p+"/at_ms", "must be before the end of the video (duration_ms)")
		}
		var opts []string
		for j, o := range b.Options {
			opts = append(opts, o.OptionID)
			if !slices.Contains(segIDs, o.TargetSegmentID) {
				add(p+"/options/"+strconv.Itoa(j)+"/target_segment_id", "is not a segment of the video")
			}
		}
		dup(p+"/options", opts)
	}
	dup("branches", brIDs)
	if c.Interactive() {
		ending := false
		for _, s := range c.Content.Segments {
			ending = ending || s.IsEnding
		}
		if !ending {
			add("segments", "an interactive video needs an ending segment (is_ending)")
		}
	}
	return out
}
