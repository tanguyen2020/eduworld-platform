package question

import (
	"crypto/sha256"
	"encoding/json"

	"golang.org/x/text/unicode/norm"
)

// NFC returns doc with every string normalized to Unicode NFC (F022-AC-05: decomposed Vietnamese input is stored
// composed). Invalid JSON is returned unchanged (the schema validator reports it).
func NFC(doc json.RawMessage) json.RawMessage {
	if len(doc) == 0 || string(doc) == "null" {
		return doc
	}
	var v any
	if err := json.Unmarshal(doc, &v); err != nil {
		return doc
	}
	var rec func(any) any
	rec = func(x any) any {
		switch t := x.(type) {
		case string:
			return norm.NFC.String(t)
		case map[string]any:
			out := make(map[string]any, len(t))
			for k, val := range t {
				out[norm.NFC.String(k)] = rec(val)
			}
			return out
		case []any:
			for i := range t {
				t[i] = rec(t[i])
			}
			return t
		}
		return x
	}
	b, err := json.Marshal(rec(v))
	if err != nil {
		return doc
	}
	return b
}

// ContentHash is the sha256 of the canonical JSON (sorted keys) of the parts that define a version.
func ContentHash(parts ...json.RawMessage) []byte {
	h := sha256.New()
	for _, p := range parts {
		var v any
		if len(p) > 0 && json.Unmarshal(p, &v) == nil {
			b, _ := json.Marshal(v) // encoding/json sorts map keys
			h.Write(b)
		}
		h.Write([]byte{0})
	}
	return h.Sum(nil)
}
