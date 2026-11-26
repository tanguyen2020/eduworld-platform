// Package i18ntext is the translation state of game texts (F082, CONTENT_MODEL §8, ADR-0056): which JSON pointers of a
// game config payload are bilingual game texts, whether a version is complete, partial (a required language missing) or
// outdated (the source text changed after the last translation), the stable key of a text in a translation file
// (<config_type>/<config_id>/<version>/<json-pointer>) and the version rule of an import (research R2). Pure Go, no I/O.
//
// A game text is a JSON object whose only keys are vi/en with at least one string value — the same rule as the
// TRANSLATION_INCOMPLETE check of submit_review (gameconfig.MissingTranslations), so the status and the gate never
// disagree (Clarifications 2026-11-26).
package i18ntext

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/unicode/norm"
)

// SourceLanguage is the original language of game texts (ADR-0056 §1); every other required language is a target.
const SourceLanguage = "vi"

// Translation statuses (TranslationStatus).
const (
	Complete = "complete"
	Partial  = "partial"
	Outdated = "outdated"
)

// ValidStatus reports a TranslationStatus value.
func ValidStatus(s string) bool { return s == Complete || s == Partial || s == Outdated }

// MaxPointers bounds missing_fields / outdated_fields (OpenAPI maxItems) and the stored missing pointers.
const MaxPointers = 500

// Text is one game text of a payload: its JSON pointer (RFC 6901) and its values by language.
type Text struct {
	Pointer string
	Values  map[string]string
}

// Meta records the last translation of a text (research R3): the SHA-256 of the NFC source when it was translated and
// the version it was translated in.
type Meta struct {
	SourceSHA256          string    `json:"source_sha256"`
	TranslatedInVersionID string    `json:"translated_in_version_id,omitempty"`
	TranslatedInVersion   int       `json:"translated_in_version"`
	TranslatedAt          time.Time `json:"translated_at"`
}

// OutdatedField is one outdated text (outdated_fields[]).
type OutdatedField struct {
	Pointer             string `json:"pointer"`
	TranslatedInVersion int    `json:"translated_in_version"`
}

// State is the translation state of one version.
type State struct {
	Status   string
	Missing  []string // pointers of the missing language (e.g. /name/en), sorted
	Outdated []OutdatedField
	Meta     map[string]Meta
}

// Input of Compute: the payload saved now, the payload it replaces (the same draft before the save, or the version it
// was copied from; nil for a first version) with its meta, the required languages and the version being saved.
type Input struct {
	Doc       map[string]any
	Prev      map[string]any
	PrevMeta  map[string]Meta
	Required  []string
	Version   int
	VersionID string
	Now       time.Time
}

// isText reports a bilingual game text: only the keys vi/en, with at least one string value.
func isText(m map[string]any) bool {
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

// escape is a JSON pointer reference token (RFC 6901).
func escape(k string) string { return strings.ReplaceAll(strings.ReplaceAll(k, "~", "~0"), "/", "~1") }

func unescape(k string) string {
	return strings.ReplaceAll(strings.ReplaceAll(k, "~1", "/"), "~0", "~")
}

// Texts returns every game text of doc, sorted by pointer.
func Texts(doc map[string]any) []Text {
	var out []Text
	var walk func(path string, v any)
	walk = func(path string, v any) {
		switch x := v.(type) {
		case map[string]any:
			if isText(x) {
				vals := map[string]string{}
				for k, val := range x {
					if s, ok := val.(string); ok {
						vals[k] = s
					}
				}
				out = append(out, Text{Pointer: path, Values: vals})
				return
			}
			for k, val := range x {
				walk(path+"/"+escape(k), val)
			}
		case []any:
			for i, e := range x {
				walk(path+"/"+strconv.Itoa(i), e)
			}
		}
	}
	walk("", doc)
	sort.Slice(out, func(i, j int) bool { return out[i].Pointer < out[j].Pointer })
	return out
}

// Hash is the SHA-256 of the NFC form of s (the source of a translation).
func Hash(s string) string {
	sum := sha256.Sum256([]byte(norm.NFC.String(strings.TrimSpace(s))))
	return hex.EncodeToString(sum[:])
}

func present(s string) bool { return strings.TrimSpace(s) != "" }

// Compute returns the translation state of in.Doc (F082-AC-08): a text lacking a required language is missing (partial);
// a complete text whose target is unchanged by this save while its source differs from the source it was translated
// from is outdated; a target changed by this save is recorded as translated in this version; a complete text without
// meta (first computation) is taken as current. Partial wins over outdated.
func Compute(in Input) State {
	prev := map[string]map[string]string{}
	if in.Prev != nil {
		for _, t := range Texts(in.Prev) {
			prev[t.Pointer] = t.Values
		}
	}
	st := State{Status: Complete, Meta: map[string]Meta{}}
	for _, t := range Texts(in.Doc) {
		complete := true
		for _, l := range in.Required {
			if !present(t.Values[l]) {
				st.Missing = append(st.Missing, t.Pointer+"/"+l)
				complete = false
			}
		}
		old, hasMeta := in.PrevMeta[t.Pointer]
		if !complete {
			if hasMeta {
				st.Meta[t.Pointer] = old
			}
			continue
		}
		src := Hash(t.Values[SourceLanguage])
		fresh := Meta{SourceSHA256: src, TranslatedInVersionID: in.VersionID, TranslatedInVersion: in.Version, TranslatedAt: in.Now.UTC()}
		changed := false
		if p, ok := prev[t.Pointer]; ok {
			for _, l := range in.Required {
				if l != SourceLanguage && norm.NFC.String(p[l]) != norm.NFC.String(t.Values[l]) {
					changed = true
				}
			}
		} else if in.Prev != nil {
			changed = true // a new text written in this save
		}
		switch {
		case changed || !hasMeta:
			st.Meta[t.Pointer] = fresh
		case old.SourceSHA256 != src:
			st.Meta[t.Pointer] = old
			st.Outdated = append(st.Outdated, OutdatedField{Pointer: t.Pointer, TranslatedInVersion: old.TranslatedInVersion})
		default:
			st.Meta[t.Pointer] = old
		}
	}
	switch {
	case len(st.Missing) > 0:
		st.Status = Partial
	case len(st.Outdated) > 0:
		st.Status = Outdated
	}
	if len(st.Missing) > MaxPointers {
		st.Missing = st.Missing[:MaxPointers]
	}
	if len(st.Outdated) > MaxPointers {
		st.Outdated = st.Outdated[:MaxPointers]
	}
	return st
}

// ValidationCopy returns a deep copy of doc in which a missing required language of a game text is filled from another
// language (or a placeholder when every one is empty): the type checks of a draft run on it, so a draft can be saved
// before it is translated while the stored payload keeps the gap (Clarifications 2026-11-26).
func ValidationCopy(doc map[string]any, required []string) map[string]any {
	var cp func(v any) any
	cp = func(v any) any {
		switch x := v.(type) {
		case map[string]any:
			out := make(map[string]any, len(x))
			for k, val := range x {
				out[k] = cp(val)
			}
			if isText(x) {
				fill := ""
				for _, l := range append([]string{SourceLanguage}, required...) {
					if s, _ := x[l].(string); present(s) {
						fill = s
						break
					}
				}
				if fill == "" {
					fill = "-"
				}
				for _, l := range required {
					if s, _ := out[l].(string); !present(s) {
						out[l] = fill
					}
				}
			}
			return out
		case []any:
			out := make([]any, len(x))
			for i, e := range x {
				out[i] = cp(e)
			}
			return out
		}
		return v
	}
	return cp(doc).(map[string]any)
}

// ErrPointer: the pointer does not address a game text of the payload.
var ErrPointer = errors.New("i18ntext: no game text at this pointer")

func resolve(doc map[string]any, pointer string) (map[string]any, bool) {
	if !strings.HasPrefix(pointer, "/") {
		return nil, false
	}
	var cur any = doc
	for _, tok := range strings.Split(pointer[1:], "/") {
		tok = unescape(tok)
		switch x := cur.(type) {
		case map[string]any:
			v, ok := x[tok]
			if !ok {
				return nil, false
			}
			cur = v
		case []any:
			i, err := strconv.Atoi(tok)
			if err != nil || i < 0 || i >= len(x) {
				return nil, false
			}
			cur = x[i]
		default:
			return nil, false
		}
	}
	m, ok := cur.(map[string]any)
	if !ok || !isText(m) {
		return nil, false
	}
	return m, true
}

// Get returns the values of the game text at pointer.
func Get(doc map[string]any, pointer string) (map[string]string, bool) {
	m, ok := resolve(doc, pointer)
	if !ok {
		return nil, false
	}
	out := map[string]string{}
	for k, v := range m {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	return out, true
}

// Set writes the text of one language at pointer (NFC), never another language (the import writes the target only).
func Set(doc map[string]any, pointer, lang, text string) error {
	m, ok := resolve(doc, pointer)
	if !ok {
		return ErrPointer
	}
	m[lang] = norm.NFC.String(text)
	return nil
}

// Key is the stable key of a text in a translation file: <config_type>/<config_id>/<version>/<json-pointer>.
type Key struct {
	ConfigType string
	ConfigID   string
	Version    int
	Pointer    string
}

var typeRe = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)

// String formats the key (the pointer keeps its leading slash).
func (k Key) String() string {
	return k.ConfigType + "/" + k.ConfigID + "/" + strconv.Itoa(k.Version) + k.Pointer
}

// ErrKey: a malformed translation key.
var ErrKey = errors.New("i18ntext: malformed key")

// ParseKey parses a key of a translation file.
func ParseKey(s string) (Key, error) {
	parts := strings.SplitN(s, "/", 4)
	if len(parts) != 4 || !typeRe.MatchString(parts[0]) || parts[1] == "" || parts[3] == "" {
		return Key{}, ErrKey
	}
	v, err := strconv.Atoi(parts[2])
	if err != nil || v < 1 {
		return Key{}, ErrKey
	}
	return Key{ConfigType: parts[0], ConfigID: parts[1], Version: v, Pointer: "/" + parts[3]}, nil
}

// Decision is what an import does with the texts of one item (research R2).
type Decision int

// Import decisions.
const (
	Skip       Decision = iota
	WriteDraft          // the newest version is a draft/rejected: write into it
	NewDraft            // the newest version is approved/published/archived: a new draft based on it
)

// Report error codes of an import (TranslationJob.report.errors[].code).
const (
	CodeKeyNotFound      = "key_not_found"
	CodeStaleVersion     = "stale_version"
	CodeVersionInReview  = "version_in_review"
	CodeUnsupportedType  = "unsupported_type"
	CodeTooLong          = "too_long"
	CodeInvalidLanguage  = "invalid_language"
	statusDraft          = "draft"
	statusRejected       = "rejected"
	statusInReview       = "in_review"
	statusApproved       = "approved"
	statusPublished      = "published"
	statusArchivedString = "archived"
)

// DecideImport applies research R2 to an item whose key names version keyVersion while its newest version is latest
// with latestStatus: only the newest version is translated; a version under review is never touched.
func DecideImport(keyVersion, latest int, latestStatus string) (Decision, string) {
	switch {
	case keyVersion > latest:
		return Skip, CodeKeyNotFound
	case keyVersion < latest:
		return Skip, CodeStaleVersion
	}
	switch latestStatus {
	case statusDraft, statusRejected:
		return WriteDraft, ""
	case statusApproved, statusPublished, statusArchivedString:
		return NewDraft, ""
	case statusInReview:
		return Skip, CodeVersionInReview
	}
	return Skip, CodeKeyNotFound
}

// MaxReportErrors bounds report.errors (OpenAPI maxItems); further errors only count as skipped.
const MaxReportErrors = 500

// ReportError is one line of an import/export report (report.errors[]): the key or file reference, a code and a short
// message — never the translated text.
type ReportError struct {
	Ref     string `json:"ref"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

// Report is TranslationJob.report.
type Report struct {
	Total   int           `json:"total"`
	Updated int           `json:"updated"`
	Skipped int           `json:"skipped"`
	Errors  []ReportError `json:"errors"`
}

// Skip counts a skipped line and records its error while the list has room.
func (r *Report) Skip(ref, code, message string) {
	r.Skipped++
	if len(r.Errors) < MaxReportErrors {
		if len(ref) > 255 {
			ref = ref[:255]
		}
		if len(message) > 500 {
			message = message[:500]
		}
		r.Errors = append(r.Errors, ReportError{Ref: ref, Code: code, Message: message})
	}
}

// Unit is one text of a translation file: its key, the source text, the target text and a note (missing/outdated).
type Unit struct {
	Key    string
	Source string
	Target string
	Note   string
}

// File is a decoded translation file (XLIFF 2.1 or CSV).
type File struct {
	SourceLanguage string
	TargetLanguage string // "" when the format does not say (CSV)
	Units          []Unit
}

// MaxTextLength is maxLength of a localized_text value (specs/schemas $defs/localized_text).
const MaxTextLength = 2000
