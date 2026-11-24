package gameconfig

import (
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// Refs are the references of a payload checked at submit_review (CONTENT_MODEL §6, CMS-D-009): the world ids that must
// exist in the newest client manifest and the game media assets that must be ready.
type Refs struct {
	Areas, NPCs, Scenes, Interactables []string
	// Assets are the game media asset ids (fields *_asset_id, or a string value "asset:<uuid>").
	Assets []string
}

// assetRef is the form of a media reference inside a free string field (e.g. item asset_refs.model).
var assetRef = regexp.MustCompile(`^asset:([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})$`)

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// CollectRefs walks a payload: keys area_id/*_area_id(s), npc_id/*_npc_id, scene_ref, interactable_id(s) and media refs.
func CollectRefs(doc map[string]any) Refs {
	set := map[string]map[string]bool{"area": {}, "npc": {}, "scene": {}, "interactable": {}, "asset": {}}
	addStr := func(kind string, v any) {
		switch x := v.(type) {
		case string:
			if x != "" {
				set[kind][x] = true
			}
		case []any:
			for _, e := range x {
				if s, ok := e.(string); ok && s != "" {
					set[kind][s] = true
				}
			}
		}
	}
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, val := range x {
				switch {
				case k == "area_id" || strings.HasSuffix(k, "_area_id") || k == "area_ids" || strings.HasSuffix(k, "_area_ids"):
					addStr("area", val)
				case k == "npc_id" || strings.HasSuffix(k, "_npc_id"):
					addStr("npc", val)
				case k == "scene_ref":
					addStr("scene", val)
				case k == "interactable_id" || k == "interactable_ids":
					addStr("interactable", val)
				case strings.HasSuffix(k, "_asset_id"):
					if s, ok := val.(string); ok && uuidRe.MatchString(s) {
						set["asset"][s] = true
					}
				}
				walk(val)
			}
		case []any:
			for _, e := range x {
				walk(e)
			}
		case string:
			if m := assetRef.FindStringSubmatch(x); m != nil {
				set["asset"][m[1]] = true
			}
		}
	}
	walk(doc)
	keys := func(k string) []string {
		out := make([]string, 0, len(set[k]))
		for s := range set[k] {
			out = append(out, s)
		}
		sort.Strings(out)
		return out
	}
	return Refs{Areas: keys("area"), NPCs: keys("npc"), Scenes: keys("scene"), Interactables: keys("interactable"), Assets: keys("asset")}
}

// ClientManifest is the content of game.client_manifests.manifest of the newest Godot build.
type ClientManifest struct {
	Areas         []string `json:"area_ids"`
	NPCs          []string `json:"npc_ids"`
	Scenes        []string `json:"scene_refs"`
	Interactables []string `json:"interactable_ids"`
}

// Missing returns the references absent from the client manifest ("area:x", "npc:y", "scene:z", "interactable:w").
// selfArea is the area an area payload defines (a new area is checked against the build too: its scene must exist).
func (m ClientManifest) Missing(r Refs) []string {
	var out []string
	check := func(kind string, have, want []string) {
		for _, w := range want {
			if !slices.Contains(have, w) {
				out = append(out, kind+":"+w)
			}
		}
	}
	check("area", m.Areas, r.Areas)
	check("npc", m.NPCs, r.NPCs)
	check("scene", m.Scenes, r.Scenes)
	check("interactable", m.Interactables, r.Interactables)
	return out
}

// MissingTranslations returns the JSON pointers of the bilingual texts ({"vi": …, "en": …}, ADR-0056) of doc lacking one
// of the required languages (game_text_required_languages, NFR-I18N-002).
func MissingTranslations(doc map[string]any, required []string) []string {
	var out []string
	var walk func(path string, v any)
	walk = func(path string, v any) {
		switch x := v.(type) {
		case map[string]any:
			if isI18n(x) {
				for _, l := range required {
					if s, _ := x[l].(string); strings.TrimSpace(s) == "" {
						out = append(out, path)
						break
					}
				}
				return
			}
			keys := make([]string, 0, len(x))
			for k := range x {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				walk(path+"/"+escape(k), x[k])
			}
		case []any:
			for i, e := range x {
				walk(path+"/"+strconv.Itoa(i), e)
			}
		}
	}
	walk("", doc)
	return out
}

// isI18n reports a bilingual text: only the keys vi/en, with at least one string value.
func isI18n(m map[string]any) bool {
	if len(m) == 0 || len(m) > 2 {
		return false
	}
	str := false
	for k, v := range m {
		if k != "vi" && k != "en" {
			return false
		}
		if _, ok := v.(string); ok {
			str = true
		}
	}
	return str
}
