package mathx

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
)

// Rendered is a stored formula: its LaTeX, the key of its sanitized SVG (math/<sha256>.svg) and its speech text.
type Rendered struct {
	Latex      string `json:"latex"`
	SVGAsset   string `json:"svg_asset"`
	SpeechText string `json:"speech_text"`
}

// SVGKeyPattern is the shape of a stored formula key (question.schema.json#/$defs/mathDisplay/svg_asset).
var SVGKeyPattern = regexp.MustCompile(`^math/[0-9a-f]{64}\.svg$`)

// Hash returns the hex sha256 of a sanitized SVG: its identity and immutable key (F027-AC-15).
func Hash(sanitized []byte) string {
	h := sha256.Sum256(sanitized)
	return hex.EncodeToString(h[:])
}

// Key returns the object key of a sanitized SVG hash.
func Key(sha string) string { return "math/" + sha + ".svg" }

// Block is the math block of a render payload (F027-AC-14, ADR-0055 §5): the client shows the SVG of math_ref and
// reads speech_text, so it never needs MathJax. The LaTeX stays out of the student payload (F022 render contract,
// OQ F027-3); WithLatex keeps it for CMS views and snapshots.
func Block(r Rendered, withLatex bool) map[string]any {
	b := map[string]any{"type": "math", "math_ref": r.SVGAsset, "speech_text": r.SpeechText}
	if withLatex {
		b["latex"] = r.Latex
	}
	return b
}

// Transform rewrites the math blocks ({"type":"math","latex":…}) of a decoded rich content in place with Block,
// using the rendered formulas by LaTeX; withLatex keeps the LaTeX in the block. A formula without rendering loses
// its LaTeX in a student payload (the save rules refuse it before, F022-AC-05).
func Transform(v any, rendered []Rendered, withLatex bool) any {
	byLatex := make(map[string]Rendered, len(rendered))
	for _, r := range rendered {
		byLatex[r.Latex] = r
	}
	var rec func(any) any
	rec = func(x any) any {
		switch t := x.(type) {
		case map[string]any:
			if t["type"] == "math" {
				if l, ok := t["latex"].(string); ok {
					if !withLatex {
						delete(t, "latex")
					}
					if r, ok := byLatex[l]; ok {
						for k, val := range Block(r, withLatex) {
							t[k] = val
						}
					}
				}
			}
			for k, val := range t {
				t[k] = rec(val)
			}
			return t
		case []any:
			for i := range t {
				t[i] = rec(t[i])
			}
			return t
		}
		return x
	}
	return rec(v)
}
