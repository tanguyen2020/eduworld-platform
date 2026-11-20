package attemptview

import (
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/videolesson"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/media"
)

// TheoraPrefix is the prefix of the Theora variants the game client plays (pkg/media VideoJobs: ogv_480, ogv_720).
const TheoraPrefix = "ogv_"

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

// VideoReady reports that the main video and every segment video are ready with a Theora variant (F065-AC-04).
func VideoReady(vc videolesson.Config, assets map[uuid.UUID]media.Asset) bool {
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

// MediaRefs are ActivityRender.media of a video version (F065-AC-02/03): every Theora variant of the main video and of
// each segment, and the WebVTT of each subtitle language; tenant media get signed URLs (ADR-0044, R31).
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
	video(vc.Content.VideoAssetID, "")
	for _, s := range vc.Content.Segments {
		video(s.VideoAssetID, s.SegmentID)
	}
	for _, s := range vc.Content.Subtitles {
		a, ok := assets[s.SubtitleAssetID]
		if !ok || a.Status != media.StatusReady {
			continue
		}
		variant := ""
		for _, v := range SubtitleVariants {
			if _, ok := a.Variants[v]; ok {
				variant = v
				break
			}
		}
		if variant == "" {
			keys := make([]string, 0, len(a.Variants))
			for k := range a.Variants {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			if len(keys) > 0 {
				variant = keys[0]
			}
		}
		ref(a, variant, "subtitle", s.Language, "")
	}
	return out
}

// ProgressState is ActivityProgressState of a media attempt (F065-AC-12): accepted ranges, plausible ratio, pending
// cues and whether the completion rule would be met now.
func ProgressState(vc videolesson.Config, p videolesson.Progress) dto.ProgressStateView {
	ranges := make([]dto.WatchedRangeView, 0, len(p.Ranges))
	for _, r := range p.Ranges {
		ranges = append(ranges, dto.WatchedRangeView{StartMS: r.StartMS, EndMS: r.EndMS})
	}
	return dto.ProgressStateView{PositionMS: p.PositionMS, WatchedRatio: p.Ratio, WatchedRanges: ranges, SegmentID: p.SegmentID,
		Completed:     videolesson.Evaluate(vc, p.Ratio, p.ProcessedCues, p.ReachedEnding).Completed,
		PendingCueIDs: videolesson.PendingCues(vc, p.ProcessedCues, false)}
}

// WithTimeline adds the media, the progress and the resume position of a media attempt to its view.
func WithTimeline(v dto.AttemptView, vc videolesson.Config, assets map[uuid.UUID]media.Asset, signer port.URLSigner, p *videolesson.Progress,
	now time.Time) dto.AttemptView {
	if v.Render != nil {
		v.Render.Media = MediaRefs(vc, assets, signer, now)
	}
	if p != nil {
		st := ProgressState(vc, *p)
		pos := p.PositionMS
		v.Progress, v.ResumePositionMS = &st, &pos
	}
	return v
}
