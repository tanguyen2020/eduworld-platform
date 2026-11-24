package gameconfig

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

// MaxChanges bounds the diff (ContentVersionDiff.changes maxItems).
const MaxChanges = 2000

// serverFields are the payload fields the server fills per version: they differ between any two versions and say
// nothing about the authored content.
var serverFields = map[string]bool{"version": true, "status": true}

// Diff lists the changes from a to b by field path (F078-AC-09): objects are compared key by key, arrays index by index,
// scalars by value; the version fields the server owns are skipped. At most MaxChanges changes; truncated reports more.
func Diff(t Type, a, b map[string]any) (changes []Change, truncated bool) {
	skip := map[string]bool{}
	for k := range serverFields {
		skip["/"+k] = true
	}
	if t.VersionIDField != "" {
		skip["/"+t.VersionIDField] = true
	}
	var walk func(path string, x, y any)
	walk = func(path string, x, y any) {
		if skip[path] || truncated {
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
			keys := make([]string, 0, len(xm)+len(ym))
			for k := range xm {
				keys = append(keys, k)
			}
			for k := range ym {
				if _, ok := xm[k]; !ok {
					keys = append(keys, k)
				}
			}
			sort.Strings(keys)
			for _, k := range keys {
				p := path + "/" + escape(k)
				xv, inX := xm[k]
				yv, inY := ym[k]
				switch {
				case !inX:
					if !skip[p] {
						add(Change{Path: p, Op: "added", After: yv})
					}
				case !inY:
					if !skip[p] {
						add(Change{Path: p, Op: "removed", Before: xv})
					}
				default:
					walk(p, xv, yv)
				}
			}
			return
		}
		xa, xok := x.([]any)
		ya, yok := y.([]any)
		if xok && yok {
			for i := 0; i < max(len(xa), len(ya)); i++ {
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

// escape encodes a key as a JSON pointer token (RFC 6901).
func escape(k string) string {
	return strings.ReplaceAll(strings.ReplaceAll(k, "~", "~0"), "/", "~1")
}
