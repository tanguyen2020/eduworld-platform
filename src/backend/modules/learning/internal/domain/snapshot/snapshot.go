// Package snapshot builds the immutable snapshot of a published learning content version (F026, ADR-0016, ADR-0043,
// CONTENT_MODEL §5.4): the content without any answer data, its SHA-256 and the JSON pointer diff of two versions.
package snapshot

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// AnswerKeys are removed from every snapshot (F026-AC-12, QUESTION_BANK §2): answer keys of every question type,
// scoring, explanations, feedback, misconception tags, hints, response timing.
var AnswerKeys = []string{"answer_key", "scoring_rule", "accepted_answers", "accepted", "correct_option_id", "correct_option_ids",
	"statement_values", "explanation", "feedback", "option_feedback", "misconception_tag", "hints", "min_response_ms"}

// Strip returns doc without any key of AnswerKeys at any depth.
func Strip(doc json.RawMessage) (json.RawMessage, error) {
	var v any
	if err := json.Unmarshal(doc, &v); err != nil {
		return nil, err
	}
	var rec func(any) any
	rec = func(x any) any {
		switch t := x.(type) {
		case map[string]any:
			for k, val := range t {
				if slices.Contains(AnswerKeys, k) {
					delete(t, k)
					continue
				}
				t[k] = rec(val)
			}
		case []any:
			for i, val := range t {
				t[i] = rec(val)
			}
		}
		return x
	}
	return json.Marshal(rec(v))
}

// Leaks returns the JSON pointers of answer keys in doc (contract test of the snapshots, SC-002).
func Leaks(doc json.RawMessage) []string {
	var v any
	if json.Unmarshal(doc, &v) != nil {
		return nil
	}
	var out []string
	var rec func(string, any)
	rec = func(p string, x any) {
		switch t := x.(type) {
		case map[string]any:
			for k, val := range t {
				if slices.Contains(AnswerKeys, k) {
					out = append(out, p+"/"+k)
				}
				rec(p+"/"+k, val)
			}
		case []any:
			for i, val := range t {
				rec(p+"/"+strconv.Itoa(i), val)
			}
		}
	}
	rec("", v)
	sort.Strings(out)
	return out
}

// SHA256 is the hex digest of the snapshot bytes.
func SHA256(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// Op is the kind of a change.
type Op string

// Change kinds.
const (
	Added   Op = "added"
	Removed Op = "removed"
	Changed Op = "changed"
)

// Change is one difference between two documents at a JSON pointer.
type Change struct {
	Path   string `json:"path"`
	Op     Op     `json:"op"`
	Before any    `json:"before,omitempty"`
	After  any    `json:"after,omitempty"`
}

// MaxChanges bounds a diff (ContentVersionDiff.changes maxItems).
const MaxChanges = 2000

func esc(k string) string { return strings.ReplaceAll(strings.ReplaceAll(k, "~", "~0"), "/", "~1") }

// Diff returns the changes from a to b by JSON pointer, objects key by key and arrays index by index (F026-AC-21).
func Diff(a, b json.RawMessage) []Change {
	var x, y any
	_ = json.Unmarshal(a, &x)
	_ = json.Unmarshal(b, &y)
	var out []Change
	var rec func(p string, x, y any)
	rec = func(p string, x, y any) {
		if len(out) >= MaxChanges {
			return
		}
		mx, okx := x.(map[string]any)
		my, oky := y.(map[string]any)
		if okx && oky {
			keys := map[string]bool{}
			for k := range mx {
				keys[k] = true
			}
			for k := range my {
				keys[k] = true
			}
			ks := make([]string, 0, len(keys))
			for k := range keys {
				ks = append(ks, k)
			}
			sort.Strings(ks)
			for _, k := range ks {
				vx, inx := mx[k]
				vy, iny := my[k]
				switch {
				case !inx:
					out = append(out, Change{Path: p + "/" + esc(k), Op: Added, After: vy})
				case !iny:
					out = append(out, Change{Path: p + "/" + esc(k), Op: Removed, Before: vx})
				default:
					rec(p+"/"+esc(k), vx, vy)
				}
			}
			return
		}
		ax, okx := x.([]any)
		ay, oky := y.([]any)
		if okx && oky {
			for i := 0; i < max(len(ax), len(ay)); i++ {
				ip := p + "/" + strconv.Itoa(i)
				switch {
				case i >= len(ax):
					out = append(out, Change{Path: ip, Op: Added, After: ay[i]})
				case i >= len(ay):
					out = append(out, Change{Path: ip, Op: Removed, Before: ax[i]})
				default:
					rec(ip, ax[i], ay[i])
				}
			}
			return
		}
		if !reflect.DeepEqual(x, y) {
			out = append(out, Change{Path: p, Op: Changed, Before: x, After: y})
		}
	}
	rec("", x, y)
	return out
}

// HideAnswers drops the changes under an answer key (a caller without learning_content.review, F026-AC-21).
func HideAnswers(cs []Change) []Change {
	out := cs[:0:0]
	for _, c := range cs {
		hidden := false
		for _, seg := range strings.Split(c.Path, "/") {
			if slices.Contains(AnswerKeys, seg) {
				hidden = true
				break
			}
		}
		if !hidden {
			out = append(out, c)
		}
	}
	return out
}

// Canonical re-encodes a JSON document with sorted keys (stable SHA-256).
func Canonical(doc json.RawMessage) ([]byte, error) {
	var v any
	dec := json.NewDecoder(bytes.NewReader(doc))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return json.Marshal(v)
}
