// Package audiolesson holds the pure rules of activity_type audio_lesson (F066, AUDIO_LANGUAGE_LEARNING §2,
// ADR-0048): the config of a version and its checks, the listening plays (research R1), the render payload, and the
// timeline the attempt engine shares with video_lesson (progress, resume, embedded questions, completion — F065). No
// I/O; there is no recording or voice path (ADR-0059, ADR-0072).
package audiolesson

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/videolesson"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

// Transcript display modes (interaction.show_transcript).
const (
	TranscriptNever         = "never"
	TranscriptAfterComplete = "after_complete"
	TranscriptAlways        = "always"
)

// MaxSegments and MaxQuestions are the maxItems of audio-lesson.schema.json.
const (
	MaxSegments  = 50
	MaxQuestions = 30
	// AfterEnd orders a question without at_ms after every timed question (asked once the audio is over).
	AfterEnd int64 = 1 << 40
)

// SpeedRates are the playback rates of a version with allow_speed_change (STUDENT_GAME_UX_LEARNING §6: 0.75× and 1×).
var SpeedRates = []float64{0.75, 1}

// Segment is content.segments[].
type Segment struct {
	SegmentID string           `json:"segment_id"`
	Title     videolesson.I18n `json:"title,omitempty"`
	StartMS   int64            `json:"start_ms"`
	EndMS     int64            `json:"end_ms"`
}

// Question is content.questions[] (an auto-graded question, at a time of the audio or after it).
type Question struct {
	QuestionVersionID uuid.UUID `json:"question_version_id"`
	AtMS              *int64    `json:"at_ms,omitempty"`
}

// Content is config.content of audio-lesson.schema.json.
type Content struct {
	AudioAssetID      uuid.UUID  `json:"audio_asset_id"`
	TranscriptAssetID *uuid.UUID `json:"transcript_asset_id,omitempty"`
	Segments          []Segment  `json:"segments,omitempty"`
	Questions         []Question `json:"questions,omitempty"`
}

// Interaction is config.interaction.
type Interaction struct {
	MaxPlays         *int   `json:"max_plays,omitempty"`
	AllowSpeedChange *bool  `json:"allow_speed_change,omitempty"`
	ShowTranscript   string `json:"show_transcript,omitempty"`
}

// Config is the config of an audio_lesson version.
type Config struct {
	Content        Content                `json:"content"`
	Interaction    Interaction            `json:"interaction"`
	CompletionRule videolesson.Completion `json:"completion_rule"`
}

// ErrConfig: the config cannot be decoded.
var ErrConfig = errors.New("audiolesson: config cannot be read")

// Parse decodes a config already checked against its JSON Schema.
func Parse(raw json.RawMessage) (Config, error) {
	var c Config
	if len(raw) == 0 || json.Unmarshal(raw, &c) != nil {
		return c, ErrConfig
	}
	return c, nil
}

// Transcript is show_transcript (default after_complete).
func (c Config) Transcript() string {
	switch c.Interaction.ShowTranscript {
	case TranscriptNever, TranscriptAlways:
		return c.Interaction.ShowTranscript
	}
	return TranscriptAfterComplete
}

// SpeedChange is allow_speed_change (default true).
func (c Config) SpeedChange() bool {
	return c.Interaction.AllowSpeedChange == nil || *c.Interaction.AllowSpeedChange
}

// Rates are the playback rates of the player: 0.75× and 1× with allow_speed_change, else 1× only.
func (c Config) Rates() []float64 {
	if c.SpeedChange() {
		return slices.Clone(SpeedRates)
	}
	return []float64{1}
}

// CueID is the id of question i (declaration order) in the timeline: q01…q30.
func CueID(i int) string { return fmt.Sprintf("q%02d", i+1) }

// Timeline is the shared media timeline of the version (F065 machinery): the audio plays as the main media, each
// question is a non-required cue at its at_ms (after the end when absent), seeking is free inside a play, the
// rates follow allow_speed_change and the completion rule is the one of the version (default video_completion).
// Questions are not required cues [Proposed F066]: an unanswered question scores 0 and never blocks the submit.
func (c Config) Timeline() videolesson.Config {
	notRequired := false
	cues := make([]videolesson.Cue, 0, len(c.Content.Questions))
	for i, q := range c.Content.Questions {
		at := AfterEnd
		if q.AtMS != nil {
			at = *q.AtMS
		}
		cues = append(cues, videolesson.Cue{CueID: CueID(i), AtMS: at, QuestionVersionID: q.QuestionVersionID, Required: &notRequired})
	}
	return videolesson.Config{
		Content:        videolesson.Content{VideoAssetID: c.Content.AudioAssetID, TranscriptAssetID: c.Content.TranscriptAssetID, Cues: cues},
		Interaction:    videolesson.Interaction{AllowSeek: videolesson.SeekFree, PlaybackRates: c.Rates()},
		CompletionRule: c.CompletionRule, Kind: videolesson.KindAudio, MaxPlays: c.Interaction.MaxPlays,
		ShowTranscript: c.Transcript(),
	}
}

// Validate checks what the JSON Schema cannot (F066-AC-01): segments with start_ms < end_ms, inside the audio when its
// duration is known (durationMS > 0), without overlap and with unique ids; question times before the end of the
// audio. The question versions are checked by the use case (published, auto-graded).
func (c Config) Validate(durationMS int64) contentmodel.Violations {
	var out contentmodel.Violations
	add := func(f, r string) { out = append(out, contentmodel.Violation{Field: "/config/content/" + f, Reason: r}) }
	seen := map[string]bool{}
	for i, s := range c.Content.Segments {
		p := "segments/" + strconv.Itoa(i)
		if s.StartMS >= s.EndMS {
			add(p+"/end_ms", "must be after start_ms")
		}
		if durationMS > 0 && s.EndMS > durationMS {
			add(p+"/end_ms", "is after the end of the audio")
		}
		if seen[s.SegmentID] {
			add(p+"/segment_id", "repeats the id "+s.SegmentID)
		}
		seen[s.SegmentID] = true
	}
	sorted := slices.Clone(c.Content.Segments)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].StartMS < sorted[j].StartMS })
	for i := 1; i < len(sorted); i++ {
		if sorted[i].StartMS < sorted[i-1].EndMS {
			add("segments", "segments "+sorted[i-1].SegmentID+" and "+sorted[i].SegmentID+" overlap")
		}
	}
	for i, q := range c.Content.Questions {
		if q.AtMS != nil && durationMS > 0 && *q.AtMS >= durationMS {
			add("questions/"+strconv.Itoa(i)+"/at_ms", "must be before the end of the audio")
		}
	}
	return out
}

// ValidatePublish is the rule added at publish (F066-AC-01, Clarifications 2026-11-16): a transcript is required.
func (c Config) ValidatePublish() contentmodel.Violations {
	if c.Content.TranscriptAssetID == nil {
		return contentmodel.Violations{{Field: "/config/content/transcript_asset_id", Reason: "is required to publish an audio lesson", Required: true}}
	}
	return nil
}

// QuestionIDs are the question versions in timeline order (item position i+1 is entry i).
func (c Config) QuestionIDs() []uuid.UUID {
	out := []uuid.UUID{}
	for _, q := range c.Timeline().OrderedCues() {
		out = append(out, q.QuestionVersionID)
	}
	return out
}

// AssetIDs are the media of the version: the audio, then the transcript.
func (c Config) AssetIDs() []uuid.UUID {
	out := []uuid.UUID{c.Content.AudioAssetID}
	if c.Content.TranscriptAssetID != nil {
		out = append(out, *c.Content.TranscriptAssetID)
	}
	return out
}
