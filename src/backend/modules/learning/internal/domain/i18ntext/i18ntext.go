// Package i18ntext holds the language rules of learning content texts (CONTENT_MODEL §8, ADR-0056, F025 US4): a learning
// text (i18nText, i18nShort/Medium/Long) is either a plain string written in the content_language of the version or an
// object {vi, en}; the content_language text is mandatory, the other one optional. The package walks a content
// document, computes translation_status and resolves the text of one language with fallback.
package i18ntext

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Languages of the content data (keys of the i18n objects; never vi-VN/en-US, CONTENT_MODEL §8).
const (
	VI = "vi"
	EN = "en"
)

// Status is translation_status.
type Status string

// Translation statuses.
const (
	Complete Status = "complete"
	Partial  Status = "partial"
	Outdated Status = "outdated"
)

// textKeys are the keys whose plain string value is a learning text written in content_language (rich blocks,
// image alt, captions, formula speech text, titles and instructions) [Proposed: the closed list of F025].
var textKeys = []string{"text", "alt", "caption", "speech_text", "title", "summary", "instructions", "label", "prompt", "placeholder"}

// Text is one learning text of a document.
type Text struct {
	Pointer string            // JSON pointer of the text in the document
	Plain   bool              // a plain string (written in content_language)
	Values  map[string]string // language → text (one entry for a plain string, keyed by content_language)
}

// Other returns the language that is not lang.
func Other(lang string) string {
	if lang == EN {
		return VI
	}
	return EN
}

// isI18n reports an object {vi?, en?} with at least one string value and no other key.
func isI18n(m map[string]any) (map[string]string, bool) {
	if len(m) == 0 || len(m) > 2 {
		return nil, false
	}
	out := map[string]string{}
	for k, v := range m {
		s, ok := v.(string)
		if !ok || (k != VI && k != EN) {
			return nil, false
		}
		out[k] = s
	}
	return out, true
}

func escape(k string) string { return strings.ReplaceAll(strings.ReplaceAll(k, "~", "~0"), "/", "~1") }

// Texts returns the learning texts of doc in document order (object keys sorted), lang being its content_language.
func Texts(doc json.RawMessage, lang string) []Text {
	var v any
	if len(doc) == 0 || json.Unmarshal(doc, &v) != nil {
		return nil
	}
	var out []Text
	var walk func(ptr string, x any)
	walk = func(ptr string, x any) {
		switch t := x.(type) {
		case map[string]any:
			keys := make([]string, 0, len(t))
			for k := range t {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				p := ptr + "/" + escape(k)
				switch val := t[k].(type) {
				case string:
					if slices.Contains(textKeys, k) {
						out = append(out, Text{Pointer: p, Plain: true, Values: map[string]string{lang: val}})
					}
				case map[string]any:
					if vals, ok := isI18n(val); ok {
						out = append(out, Text{Pointer: p, Values: vals})
						continue
					}
					walk(p, val)
				default:
					walk(p, val)
				}
			}
		case []any:
			for i, val := range t {
				walk(ptr+"/"+strconv.Itoa(i), val)
			}
		}
	}
	walk("", v)
	return out
}

// MissingSource returns the pointers of the texts without a version in lang, the content_language
// (422 TRANSLATION_INCOMPLETE, F025-AC-18).
func MissingSource(doc json.RawMessage, lang string) []string {
	var out []string
	for _, t := range Texts(doc, lang) {
		if strings.TrimSpace(t.Values[lang]) == "" {
			out = append(out, t.Pointer)
		}
	}
	return out
}

// Mark records the source text a translation was made from (translation_meta[pointer]).
type Mark struct {
	SourceSHA256 string    `json:"source_sha256"`
	TranslatedAt time.Time `json:"translated_at"`
}

// Meta is translation_meta: the marks by JSON pointer.
type Meta map[string]Mark

// ParseMeta decodes translation_meta (an absent or invalid value is empty).
func ParseMeta(raw json.RawMessage) Meta {
	m := Meta{}
	if len(raw) > 0 && string(raw) != "null" {
		_ = json.Unmarshal(raw, &m)
	}
	return m
}

// JSON encodes the meta (nil when empty).
func (m Meta) JSON() json.RawMessage {
	if len(m) == 0 {
		return nil
	}
	b, _ := json.Marshal(m)
	return b
}

func hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// Track updates the marks of a draft save (F025-AC-19): a translation that is new or changed since prev gets the hash
// of its current source text; a translation that disappeared loses its mark; an unchanged translation keeps its mark,
// so a later change of the source text makes it outdated.
func Track(prev, next json.RawMessage, lang string, meta Meta, now time.Time) Meta {
	other := Other(lang)
	before := map[string]string{}
	for _, t := range Texts(prev, lang) {
		before[t.Pointer] = t.Values[other]
	}
	out := Meta{}
	for _, t := range Texts(next, lang) {
		tr := t.Values[other]
		if tr == "" {
			continue
		}
		if m, ok := meta[t.Pointer]; ok && before[t.Pointer] == tr {
			out[t.Pointer] = m
			continue
		}
		out[t.Pointer] = Mark{SourceSHA256: hash(t.Values[lang]), TranslatedAt: now.UTC()}
	}
	return out
}

// ComputeStatus returns translation_status of doc (CONTENT_MODEL §8): partial when a text has no translation, outdated
// when a translation was made from another source text, complete otherwise (a document without text is complete).
func ComputeStatus(doc json.RawMessage, lang string, meta Meta) Status {
	other := Other(lang)
	outdated := false
	for _, t := range Texts(doc, lang) {
		if t.Values[other] == "" {
			return Partial
		}
		if m, ok := meta[t.Pointer]; ok && m.SourceSHA256 != hash(t.Values[lang]) {
			outdated = true
		}
	}
	if outdated {
		return Outdated
	}
	return Complete
}

// Locale maps an Accept-Language header to a content language key (vi when nothing matches).
func Locale(acceptLanguage string) string {
	for _, part := range strings.Split(acceptLanguage, ",") {
		tag := strings.ToLower(strings.TrimSpace(strings.SplitN(part, ";", 2)[0]))
		switch {
		case tag == EN || strings.HasPrefix(tag, "en-"):
			return EN
		case tag == VI || strings.HasPrefix(tag, "vi-"):
			return VI
		}
	}
	return VI
}

// Resolve returns doc with every i18n object replaced by the text of want, or of lang (the content_language) when
// there is no translation, and the language actually served (F025-AC-20: the Content-Language of the answer, so the
// client labels a fallback). Plain strings are written in lang.
func Resolve(doc json.RawMessage, want, lang string) (json.RawMessage, string) {
	var v any
	if len(doc) == 0 || json.Unmarshal(doc, &v) != nil {
		return doc, lang
	}
	served := want
	var rec func(x any) any
	rec = func(x any) any {
		switch t := x.(type) {
		case map[string]any:
			if vals, ok := isI18n(t); ok {
				if s, ok := vals[want]; ok && s != "" {
					return s
				}
				served = lang
				if s, ok := vals[lang]; ok {
					return s
				}
				return vals[Other(lang)]
			}
			for k, val := range t {
				if s, ok := val.(string); ok && slices.Contains(textKeys, k) && want != lang && s != "" {
					served = lang
				}
				t[k] = rec(val)
			}
			return t
		case []any:
			for i, val := range t {
				t[i] = rec(val)
			}
			return t
		}
		return x
	}
	out, err := json.Marshal(rec(v))
	if err != nil {
		return doc, lang
	}
	return out, served
}

// ErrNoText is a pointer that does not address a learning text of the document.
var ErrNoText = errors.New("i18ntext: the pointer is not a learning text")

func unescape(k string) string {
	return strings.ReplaceAll(strings.ReplaceAll(k, "~1", "/"), "~0", "~")
}

// SetTranslation writes text as the target language version of the learning text at pointer (translation import,
// F025-AC-23): a plain string written in lang becomes {lang: string, target: text}; an i18n object gets its target key.
func SetTranslation(doc json.RawMessage, pointer, lang, target, text string) (json.RawMessage, error) {
	var root any
	if err := json.Unmarshal(doc, &root); err != nil || !strings.HasPrefix(pointer, "/") {
		return doc, ErrNoText
	}
	parts := strings.Split(pointer[1:], "/")
	var parent any
	cur := root
	for _, p := range parts {
		parent = cur
		switch t := cur.(type) {
		case map[string]any:
			cur = t[unescape(p)]
		case []any:
			i, err := strconv.Atoi(p)
			if err != nil || i < 0 || i >= len(t) {
				return doc, ErrNoText
			}
			cur = t[i]
		default:
			return doc, ErrNoText
		}
	}
	last := unescape(parts[len(parts)-1])
	var next any
	switch t := cur.(type) {
	case string:
		if !slices.Contains(textKeys, last) {
			return doc, ErrNoText
		}
		next = map[string]any{lang: t, target: text}
	case map[string]any:
		if _, ok := isI18n(t); !ok {
			return doc, ErrNoText
		}
		t[target] = text
		next = t
	default:
		return doc, ErrNoText
	}
	switch p := parent.(type) {
	case map[string]any:
		p[last] = next
	case []any:
		i, _ := strconv.Atoi(last)
		p[i] = next
	}
	b, err := json.Marshal(root)
	if err != nil {
		return doc, err
	}
	return b, nil
}

// OutdatedFields returns the JSON pointers whose translation was made from another source text (ContentVersion.outdated_fields,
// the per-field "outdated" badge of the portal, F053).
func OutdatedFields(doc json.RawMessage, lang string, meta Meta) []string {
	other := Other(lang)
	var out []string
	for _, t := range Texts(doc, lang) {
		if m, ok := meta[t.Pointer]; ok && t.Values[other] != "" && m.SourceSHA256 != hash(t.Values[lang]) {
			out = append(out, t.Pointer)
		}
	}
	return out
}
