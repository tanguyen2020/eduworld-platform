package chatasset

import (
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// Change is one difference between two payloads at a JSON pointer (ContentVersionDiff.changes).
type Change struct {
	Path   string `json:"path"`
	Op     string `json:"op"` // added, removed, changed
	Before any    `json:"before,omitempty"`
	After  any    `json:"after,omitempty"`
}

// MaxChanges bounds a diff (ContentVersionDiff.changes maxItems).
const MaxChanges = 2000

// readOnlyFields are the signed preview URLs added to a response payload: never stored, never diffed.
var readOnlyFields = map[string]bool{"preview_url": true, "url": true}

func esc(k string) string { return strings.ReplaceAll(strings.ReplaceAll(k, "~", "~0"), "/", "~1") }

// Diff lists the changes from a to b by JSON pointer (F082-AC-04, diffGameConfigVersions of sticker-packs/gifs): objects
// key by key, arrays index by index, scalars by value; at most MaxChanges (truncated reports more).
func Diff(a, b map[string]any) (changes []Change, truncated bool) {
	var walk func(path string, x, y any)
	walk = func(path string, x, y any) {
		if truncated {
			return
		}
		add := func(c Change) {
			if len(changes) >= MaxChanges {
				truncated = true
				return
			}
			changes = append(changes, c)
		}
		xm, xok := x.(map[string]any)
		ym, yok := y.(map[string]any)
		if xok && yok {
			keys := map[string]bool{}
			for k := range xm {
				keys[k] = true
			}
			for k := range ym {
				keys[k] = true
			}
			sorted := make([]string, 0, len(keys))
			for k := range keys {
				if !readOnlyFields[k] {
					sorted = append(sorted, k)
				}
			}
			sort.Strings(sorted)
			for _, k := range sorted {
				p := path + "/" + esc(k)
				xv, inX := xm[k]
				yv, inY := ym[k]
				switch {
				case !inX:
					add(Change{Path: p, Op: "added", After: yv})
				case !inY:
					add(Change{Path: p, Op: "removed", Before: xv})
				default:
					walk(p, xv, yv)
				}
			}
			return
		}
		xa, xaok := x.([]any)
		ya, yaok := y.([]any)
		if xaok && yaok {
			n := max(len(xa), len(ya))
			for i := 0; i < n; i++ {
				p := path + "/" + strconv.Itoa(i)
				switch {
				case i >= len(xa):
					add(Change{Path: p, Op: "added", After: ya[i]})
				case i >= len(ya):
					add(Change{Path: p, Op: "removed", Before: xa[i]})
				default:
					walk(p, xa[i], ya[i])
				}
			}
			return
		}
		if !reflect.DeepEqual(x, y) {
			add(Change{Path: path, Op: "changed", Before: x, After: y})
		}
	}
	walk("", a, b)
	return changes, truncated
}

// MissingTranslations returns the fields of a pack lacking a required language (NFR-I18N-002, game_text_required_languages):
// name_<l> and stickers/<i>/alt_text_<l> for each required language; the hints are optional.
func (p Pack) MissingTranslations(required []string) []string {
	var out []string
	get := func(l string, vi, en string) string {
		if l == "en" {
			return en
		}
		return vi
	}
	for _, l := range required {
		if l != "vi" && l != "en" {
			continue
		}
		if strings.TrimSpace(get(l, p.NameVI, p.NameEN)) == "" {
			out = append(out, "/name_"+l)
		}
	}
	for i, s := range p.Stickers {
		for _, l := range required {
			if l != "vi" && l != "en" {
				continue
			}
			if strings.TrimSpace(get(l, s.AltVI, s.AltEN)) == "" {
				out = append(out, "/stickers/"+strconv.Itoa(i)+"/alt_text_"+l)
			}
		}
	}
	return out
}
