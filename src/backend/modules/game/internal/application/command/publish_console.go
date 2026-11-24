package command

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Pin is the version the Game Operations Console publishes through the release core (F078): the approved version keeps
// its id and number (no new version), the release id of a scheduled publication is its schedule id.
type Pin struct {
	ConfigID     uuid.UUID
	VersionID    uuid.UUID
	VersionNo    int
	ReleaseID    uuid.UUID // uuid.Nil = a new release id
	Scheduled    bool
	ThumbnailURL string // catalog thumbnail when a render of an earlier version exists ("" = none yet)
}

// ParsedContent is the outcome of the type checks of one payload.
type ParsedContent struct {
	Key string         // item key of the type table (UUID or code)
	Doc map[string]any // canonical authored content
}

// Validate runs the type checks of one payload (JSON Schema + domain rules of F071–F077, F078-AC-04): a violation is a
// 422 problem naming the field.
func (h PublishGameContent) Validate(ctx context.Context, it ContentItem, now time.Time) (ParsedContent, error) {
	p, err := h.parseItem(ctx, it, now)
	if err != nil {
		return ParsedContent{}, err
	}
	return ParsedContent{Key: p.id, Doc: p.doc}, nil
}

// CheckReferences runs the type checks and the references of one payload against the published content (submit_review,
// F078-AC-07): areas, NPCs, quests, curves, items… must exist (422 ITEM_REF_INVALID). Runs in the caller transaction.
func (h PublishGameContent) CheckReferences(ctx context.Context, it ContentItem, now time.Time) error {
	p, err := h.parseItem(ctx, it, now)
	if err != nil {
		return err
	}
	return h.checkRefs(ctx, []parsedItem{p})
}

// PublishPinned publishes one approved console version in the caller transaction (platform scope): type checks,
// references, the version row of the type table, the pointer, the release row, the type hook (OnPublished: levels,
// collection items, event runs) and content.published. The snapshot cache is written after commit.
func (h PublishGameContent) PublishPinned(ctx context.Context, it ContentItem, pin Pin, note, key string, by uuid.UUID, now time.Time) (GameRelease, error) {
	p, err := h.parseItem(ctx, it, now)
	if err != nil {
		return GameRelease{}, err
	}
	if err := h.checkRefs(ctx, []parsedItem{p}); err != nil {
		return GameRelease{}, err
	}
	rel, _, err := h.publishOne(ctx, p, &pin, note, key, by, now)
	if err != nil {
		return rel, err
	}
	if h.AfterCommit != nil && h.Cache != nil {
		_ = h.AfterCommit(ctx, func(ctx context.Context) {
			_ = h.Cache.PutSnapshot(ctx, rel.Key, rel.Snapshot)
			_ = h.Cache.SetCurrent(ctx, rel.ItemType, rel.ItemID, rel.VersionID)
		})
	}
	return rel, nil
}

// ReleaseBlocks are the type blocks a release event of a version carries (content.published/rolled_back): the runtime
// area for module competition, the mini-game snapshot for module learning, the school catalog metadata.
func (h PublishGameContent) ReleaseBlocks(ctx context.Context, it ContentItem, versionID uuid.UUID, versionNo int, stored []byte, now time.Time) (map[string]any, error) {
	p, err := h.parseItem(ctx, it, now)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	if p.area != nil {
		out["world_area"] = runtimeArea(*p.area)
	}
	if p.minigame != nil {
		b, err := minigameBlock(p, versionID, versionNo, stored)
		if err != nil {
			return nil, err
		}
		out["minigame_config"] = b
	}
	if c := catalogBlock(p.Type, p.doc); c != nil {
		out["catalog"] = c
	}
	return out, nil
}

// catalogBlock is the school catalog metadata of a quest or mini-game config (F078-AC-22, content.published.catalog):
// bilingual title, grades, subject, objectives, estimated minutes; nil for the other types.
func catalogBlock(itemType string, doc map[string]any) map[string]any {
	if itemType != "quest" && itemType != "minigame_config" {
		return nil
	}
	title := map[string]any{"vi": ""}
	if n, ok := doc["name"].(map[string]any); ok {
		for _, l := range []string{"vi", "en"} {
			if s, ok := n[l].(string); ok && s != "" {
				title[l] = truncateRunes(s, 255)
			}
		}
	}
	grades := []int{}
	if gs, ok := doc["grade_levels"].([]any); ok {
		for _, g := range gs {
			if f, ok := g.(float64); ok && f >= 1 && f <= 9 {
				grades = append(grades, int(f))
			}
		}
	}
	objectives := []string{}
	if os, ok := doc["objective_ids"].([]any); ok {
		for _, o := range os {
			if s, ok := o.(string); ok && uuidPattern.MatchString(s) && len(objectives) < 50 {
				objectives = append(objectives, s)
			}
		}
	}
	var subject, minutes any
	if s, ok := doc["subject_id"].(string); ok && uuidPattern.MatchString(s) {
		subject = s
	}
	if m, ok := doc["estimated_minutes"].(float64); ok && m >= 1 && m <= 600 {
		minutes = int(m)
	}
	return map[string]any{"title": title, "grade_levels": grades, "subject_id": subject, "objective_ids": objectives,
		"estimated_minutes": minutes, "thumbnail_url": nil}
}

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// learningRefKeys maps the payload keys of learning content to content_refs.content_type (OQ F054-3, FR-CNT-008).
var learningRefKeys = map[string]string{"activity_id": "activity", "question_set_id": "question_set", "lesson_id": "lesson"}

// learningKinds are the content_ref.kind values of a learning reference (quest steps: {kind, id, version}).
var learningKinds = map[string]bool{"activity": true, "question_set": true, "lesson": true}

// contentRefs lists the learning content a game config references (content.published content_refs, OQ F054-3; checked at
// submit_review, F078-AC-07): every activity_id/question_set_id/lesson_id (challenge activity_refs: activity_version,
// mini-game question_source: version) and every {kind, id, version} reference of a quest step.
func contentRefs(doc map[string]any) []map[string]any {
	seen := map[string]bool{}
	var out []map[string]any
	add := func(t, id string, v any) {
		if !uuidPattern.MatchString(id) || seen[t+":"+id] || len(out) >= 200 {
			return
		}
		seen[t+":"+id] = true
		ref := map[string]any{"content_type": t, "content_id": id, "version": nil}
		if n, ok := v.(float64); ok && n >= 1 {
			ref["version"] = int(n)
		}
		out = append(out, ref)
	}
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			if k, ok := x["kind"].(string); ok && learningKinds[k] {
				if id, ok := x["id"].(string); ok {
					add(k, id, x["version"])
				}
			}
			for key, t := range learningRefKeys {
				if id, ok := x[key].(string); ok {
					v := x["version"]
					if av, ok := x["activity_version"]; ok && key == "activity_id" {
						v = av
					}
					add(t, id, v)
				}
			}
			keys := make([]string, 0, len(x))
			for k := range x {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				walk(x[k])
			}
		case []any:
			for _, e := range x {
				walk(e)
			}
		}
	}
	walk(doc)
	sort.SliceStable(out, func(i, j int) bool {
		return fmt.Sprint(out[i]["content_type"], out[i]["content_id"]) < fmt.Sprint(out[j]["content_type"], out[j]["content_id"])
	})
	return out
}

// ContentRefs is contentRefs for the console (submit_review check).
func ContentRefs(doc map[string]any) []map[string]any { return contentRefs(doc) }

func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// CatalogBlock is catalogBlock for the facade listAssignedGameActivities.
func CatalogBlock(itemType string, doc map[string]any) map[string]any {
	return catalogBlock(itemType, doc)
}
