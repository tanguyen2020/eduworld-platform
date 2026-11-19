package command

import (
	"context"
	"encoding/json"
	"slices"
	"strconv"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/media"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// MediaRefs returns the distinct media assets referenced by a version: the explicit list plus every media key of
// its rich contents (media_ref, audio_ref, background_media_ref, image_media_ref), in first-seen order.
func MediaRefs(explicit []uuid.UUID, docs ...json.RawMessage) []uuid.UUID {
	out := slices.Clone(explicit)
	seen := map[uuid.UUID]bool{}
	for _, id := range out {
		seen[id] = true
	}
	add := func(s string) {
		if id, err := uuid.Parse(s); err == nil && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	for _, d := range docs {
		if isNull(d) {
			continue
		}
		var v any
		if json.Unmarshal(d, &v) != nil {
			continue
		}
		var rec func(any)
		rec = func(x any) {
			switch t := x.(type) {
			case map[string]any:
				for k, val := range t {
					if s, ok := val.(string); ok && slices.Contains(media.RefKeys, k) {
						add(s)
					}
					rec(val)
				}
			case []any:
				for _, val := range t {
					rec(val)
				}
			}
		}
		rec(v)
	}
	return out
}

// checkAssets applies the media rules of a version save (F027-AC-11): at most assets_per_version_max distinct
// assets, each visible in the scope (own tenant, or platform assigned) — an unknown asset is 422 at its pointer.
func checkAssets(ctx context.Context, store write.MediaStore, refs []uuid.UUID) error {
	if len(refs) > AssetsPerVersionMax {
		return problem.Invalid("/media_refs", problem.SubInvalid, "has more than "+strconv.Itoa(AssetsPerVersionMax)+
			" media (assets_per_version_max)")
	}
	if store == nil || len(refs) == 0 {
		return nil
	}
	found, err := store.Assets(ctx, refs)
	if err != nil {
		return err
	}
	for i, id := range refs {
		if _, ok := found[id]; !ok {
			return problem.Invalid("/media_refs/"+strconv.Itoa(i), problem.SubInvalid, "is not a media asset of the library")
		}
	}
	return nil
}

func (h Questions) checkMedia(ctx context.Context, v dto.QuestionVersion, authRender json.RawMessage) error {
	if h.Media == nil {
		return nil
	}
	docs := append([]json.RawMessage{v.Stem, authRender, v.Explanation}, hintsOf(v.Hints)...)
	return checkAssets(ctx, h.Media, MediaRefs(v.MediaRefs, docs...))
}

func hintsOf(raw json.RawMessage) []json.RawMessage {
	var hs []json.RawMessage
	if !isNull(raw) {
		_ = json.Unmarshal(raw, &hs)
	}
	return hs
}

// recordUsages makes media_usages of the version match its references (F027-AC-10, AC-11).
func (h Questions) recordUsages(ctx context.Context, v dto.QuestionVersion) error {
	if h.Media == nil {
		return nil
	}
	docs := append([]json.RawMessage{v.Stem, v.RenderPayload, v.OptionFeedback, v.Explanation}, hintsOf(v.Hints)...)
	return h.Media.ReplaceUsages(ctx, write.UsageSet{Owner: v.Owner, ItemType: contentmodel.TypeQuestion, ItemID: v.QuestionID, VersionID: v.ID,
		VersionNo: v.VersionNo, AssetIDs: MediaRefs(v.MediaRefs, docs...)})
}
