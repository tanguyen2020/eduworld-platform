package attemptview

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/audiolesson"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/videolesson"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/media"
)

// TheoraPrefix is the prefix of the Theora variants the game client plays (pkg/media VideoJobs: ogv_480, ogv_720).
const TheoraPrefix = "ogv_"

// AudioVariant is the Ogg Vorbis variant the game client plays (pkg/media AudioJobs).
const AudioVariant = "ogg"

// ErrTimeline: the published media config cannot be read.
var ErrTimeline = errors.New("attemptview: media config cannot be read")

// TimelineOf returns the media timeline of a version (video_lesson F065, audio_lesson F066); ok is false for the other
// types.
func TimelineOf(activityType string, raw json.RawMessage) (videolesson.Config, bool, error) {
	switch activity.Type(activityType) {
	case activity.VideoLesson:
		vc, err := videolesson.Parse(raw)
		if err != nil {
			return vc, true, ErrTimeline
		}
		return vc, true, nil
	case activity.AudioLesson:
		ac, err := audiolesson.Parse(raw)
		if err != nil {
			return videolesson.Config{}, true, ErrTimeline
		}
		return ac.Timeline(), true, nil
	}
	return videolesson.Config{}, false, nil
}

// SubtitleVariants are the variants tried, in order, for a WebVTT subtitle asset.
var SubtitleVariants = []string{"vtt", "original"}

// theora returns the Theora variants of an asset, sorted by name.
func theora(a media.Asset) []string {
	var out []string
	for k := range a.Variants {
		if strings.HasPrefix(k, TheoraPrefix) {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// MediaReady reports that the media of a timeline can be played: the main video and every segment video ready with a
// Theora variant (F065-AC-04), or the audio ready with its Ogg Vorbis variant (F066-AC-03).
func MediaReady(vc videolesson.Config, assets map[uuid.UUID]media.Asset) bool {
	if vc.Audio() {
		a, ok := assets[vc.Content.VideoAssetID]
		_, ogg := a.Variants[AudioVariant]
		return ok && a.Status == media.StatusReady && ogg
	}
	ids := []uuid.UUID{vc.Content.VideoAssetID}
	for _, s := range vc.Content.Segments {
		ids = append(ids, s.VideoAssetID)
	}
	for _, id := range ids {
		a, ok := assets[id]
		if !ok || a.Status != media.StatusReady || len(theora(a)) == 0 {
			return false
		}
	}
	return true
}

// DurationMS is duration_ms of the version, else the probed duration of the main video (0 when unknown).
func DurationMS(vc videolesson.Config, assets map[uuid.UUID]media.Asset) int64 {
	if vc.Content.DurationMS > 0 {
		return vc.Content.DurationMS
	}
	if a, ok := assets[vc.Content.VideoAssetID]; ok && a.DurationMS != nil {
		return int64(*a.DurationMS)
	}
	return 0
}

// MediaRefs are ActivityRender.media of a media version (F065-AC-02/03, F066-AC-02): every Theora variant of the main
// video and of each segment and the WebVTT of each subtitle language, or the Ogg Vorbis variant of an audio lesson;
// tenant media get signed URLs (ADR-0044, R31).
func MediaRefs(vc videolesson.Config, assets map[uuid.UUID]media.Asset, signer port.URLSigner, now time.Time) []dto.MediaRefView {
	out := []dto.MediaRefView{}
	ref := func(a media.Asset, variant, kind, lang, segment string) {
		v, ok := a.Variants[variant]
		if !ok || signer == nil {
			return
		}
		url, exp := signer.URL(a.Bucket, v.Key, now)
		out = append(out, dto.MediaRefView{AssetID: a.ID, URL: url, Kind: kind, Variant: variant, Language: lang, SegmentID: segment, ExpiresAt: exp})
	}
	video := func(id uuid.UUID, segment string) {
		if a, ok := assets[id]; ok && a.Status == media.StatusReady {
			for _, v := range theora(a) {
				ref(a, v, "video", "", segment)
			}
		}
	}
	if vc.Audio() {
		if a, ok := assets[vc.Content.VideoAssetID]; ok && a.Status == media.StatusReady {
			ref(a, AudioVariant, "audio", "", "")
		}
		return out
	}
	video(vc.Content.VideoAssetID, "")
	for _, s := range vc.Content.Segments {
		video(s.VideoAssetID, s.SegmentID)
	}
	for _, s := range vc.Content.Subtitles {
		a, ok := assets[s.SubtitleAssetID]
		if !ok || a.Status != media.StatusReady {
			continue
		}
		ref(a, textVariant(a), "subtitle", s.Language, "")
	}
	return out
}

// textVariant is the variant served for a text asset (subtitle, transcript): vtt, original, else the first by name.
func textVariant(a media.Asset) string {
	for _, v := range SubtitleVariants {
		if _, ok := a.Variants[v]; ok {
			return v
		}
	}
	keys := make([]string, 0, len(a.Variants))
	for k := range a.Variants {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) > 0 {
		return keys[0]
	}
	return ""
}

// TranscriptRef is the signed transcript of an audio lesson when show_transcript allows it (F066-AC-02): always, or
// after_complete once the attempt is submitted or graded; never otherwise.
func TranscriptRef(vc videolesson.Config, assets map[uuid.UUID]media.Asset, signer port.URLSigner, finished bool, now time.Time) *dto.MediaRefView {
	id := vc.Content.TranscriptAssetID
	if !vc.Audio() || id == nil || signer == nil {
		return nil
	}
	if vc.ShowTranscript != audiolesson.TranscriptAlways && (vc.ShowTranscript != audiolesson.TranscriptAfterComplete || !finished) {
		return nil
	}
	a, ok := assets[*id]
	if !ok || a.Status != media.StatusReady {
		return nil
	}
	variant := textVariant(a)
	v, ok := a.Variants[variant]
	if !ok {
		return nil
	}
	url, exp := signer.URL(a.Bucket, v.Key, now)
	return &dto.MediaRefView{AssetID: a.ID, URL: url, Kind: "transcript", Variant: variant, ExpiresAt: exp}
}

// ProgressState is ActivityProgressState of a media attempt (F065-AC-12, F066-AC-04): accepted ranges, plausible ratio,
// pending cues, the listening plays and whether the completion rule would be met now.
func ProgressState(vc videolesson.Config, p videolesson.Progress) dto.ProgressStateView {
	ranges := make([]dto.WatchedRangeView, 0, len(p.Ranges))
	for _, r := range p.Ranges {
		ranges = append(ranges, dto.WatchedRangeView{StartMS: r.StartMS, EndMS: r.EndMS})
	}
	return dto.ProgressStateView{PositionMS: p.PositionMS, WatchedRatio: p.Ratio, WatchedRanges: ranges, SegmentID: p.SegmentID,
		Completed:     videolesson.Evaluate(vc, p.Ratio, p.ProcessedCues, p.ReachedEnding).Completed,
		PendingCueIDs: videolesson.PendingCues(vc, p.ProcessedCues, false), PlaysUsed: p.PlaysUsed, MaxPlays: vc.MaxPlays}
}

// WithTimeline adds the media, the progress and the resume position of a media attempt to its view; an audio lesson
// also gets the probed duration, plays_used and resume_position_ms in its render payload and its transcript when
// show_transcript allows it (F066-AC-02, AC-07).
func WithTimeline(v dto.AttemptView, vc videolesson.Config, assets map[uuid.UUID]media.Asset, signer port.URLSigner, p *videolesson.Progress,
	now time.Time) dto.AttemptView {
	if v.Render != nil {
		v.Render.Media = MediaRefs(vc, assets, signer, now)
		finished := v.Status == "submitted" || v.Status == "graded"
		if t := TranscriptRef(vc, assets, signer, finished, now); t != nil {
			v.Render.Media = append(v.Render.Media, *t)
		}
	}
	if p != nil {
		st := ProgressState(vc, *p)
		pos := p.PositionMS
		v.Progress, v.ResumePositionMS = &st, &pos
	}
	if vc.Audio() && v.Render != nil && v.Render.RenderPayload != nil {
		v.Render.RenderPayload["duration_ms"] = DurationMS(vc, assets)
		if p != nil {
			v.Render.RenderPayload["plays_used"] = p.PlaysUsed
			v.Render.RenderPayload["resume_position_ms"] = p.PositionMS
		}
	}
	return v
}
