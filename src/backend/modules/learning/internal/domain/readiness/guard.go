// Package readiness is the PublishReadinessGuard of F025 (CONTENT_MODEL §6, §7, §8, §8.1): the checks a learning content
// version must pass before submit_review, run again by the publish of F026 with the same codes. Every rule returns
// violations with a JSON pointer; the guard reports the violations of the first failing rule kind, in a fixed order:
// schema and metadata (VALIDATION_FAILED), media not ready (MEDIA_NOT_READY), g1_3 audio (AUDIO_REQUIRED), the
// content_language text (TRANSLATION_INCOMPLETE).
package readiness

import (
	"encoding/json"
	"slices"
	"sort"
	"strconv"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/i18ntext"
)

// Code is the problem code of a violation.
type Code string

// Codes, in the order the guard reports them.
const (
	CodeValidation            Code = "VALIDATION_FAILED"
	CodeMediaNotReady         Code = "MEDIA_NOT_READY"
	CodeAudioRequired         Code = "AUDIO_REQUIRED"
	CodeTranslationIncomplete Code = "TRANSLATION_INCOMPLETE"
)

var order = []Code{CodeValidation, CodeMediaNotReady, CodeAudioRequired, CodeTranslationIncomplete}

// Violation is one failed check.
type Violation struct {
	Code     Code
	Pointer  string
	Required bool
	Reason   string
	AssetID  uuid.UUID // MEDIA_NOT_READY
}

// MediaReady is the status of a ready asset (media.StatusReady, CONTENT_MODEL §7).
const MediaReady = "ready"

// Subject is the version under check.
type Subject struct {
	Type            contentmodel.ContentType
	GradeLevels     []int
	ContentLanguage string
	// Doc is the content of the version in the shape of its authoring input (question: stem, render_payload,
	// explanation, hints; lesson: title, summary, blocks…): pointers are reported in that shape.
	Doc json.RawMessage
	// Meta are the schema and metadata violations found by the validators of the type (application layer).
	Meta contentmodel.Violations
	// Media is the status of every referenced asset ("" when it is not visible).
	Media             map[uuid.UUID]string
	AudioExemptReason string
}

// Rule is one check of the guard.
type Rule interface {
	Check(s Subject) []Violation
}

// Result is the outcome of the guard.
type Result struct {
	Code       Code        // "" when the version is ready
	Violations []Violation // violations of Code
	Warnings   []Violation // never blocking (choices without audio, CONTENT_MODEL §8.1)
}

// OK reports a ready version.
func (r Result) OK() bool { return r.Code == "" }

// Guard is PublishReadinessGuard.
type Guard struct{ Rules []Rule }

// Default is the guard of submit_review and publish: schema, media, audio, language.
func Default() Guard {
	return Guard{Rules: []Rule{SchemaRule{}, MediaRule{}, AudioRule{}, LanguageRule{}}}
}

// Check runs every rule and keeps the violations of the first code in order.
func (g Guard) Check(s Subject) Result {
	by := map[Code][]Violation{}
	for _, r := range g.Rules {
		for _, v := range r.Check(s) {
			by[v.Code] = append(by[v.Code], v)
		}
	}
	res := Result{Warnings: AudioRule{}.Warnings(s)}
	for _, c := range order {
		if len(by[c]) > 0 {
			res.Code, res.Violations = c, by[c]
			return res
		}
	}
	return res
}

// SchemaRule reports the schema and metadata violations of the type validators (F025-AC-02).
type SchemaRule struct{}

// Check implements Rule.
func (SchemaRule) Check(s Subject) []Violation {
	out := make([]Violation, 0, len(s.Meta))
	for _, m := range s.Meta {
		out = append(out, Violation{Code: CodeValidation, Pointer: m.Field, Required: m.Required, Reason: m.Reason})
	}
	return out
}

// MediaRule: every referenced asset is ready (F025-AC-03, CONTENT_MODEL §7).
type MediaRule struct{}

// Check implements Rule.
func (MediaRule) Check(s Subject) []Violation {
	var out []Violation
	for id, st := range s.Media {
		if st != MediaReady {
			out = append(out, Violation{Code: CodeMediaNotReady, Pointer: "/media_refs", AssetID: id, Reason: "media is " + orUnknown(st)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AssetID.String() < out[j].AssetID.String() })
	return out
}

func orUnknown(s string) string {
	if s == "" {
		return "not visible"
	}
	return s
}

// G13 reports a version for grades 1–3 (grade_levels intersects {1, 2, 3}, CONTENT_MODEL §8.1).
func G13(grades []int) bool {
	return slices.ContainsFunc(grades, func(g int) bool { return g >= 1 && g <= 3 })
}

// AudioRule: content for grades 1–3 has the voice-over of its stem (question), instructions (activity) or worked example
// steps (lesson, learning material) unless an exemption is declared (F025-AC-15, AC-16).
type AudioRule struct{}

// Check implements Rule.
func (AudioRule) Check(s Subject) []Violation {
	if !G13(s.GradeLevels) || s.AudioExemptReason != "" {
		return nil
	}
	var out []Violation
	for _, p := range requiredAudio(s) {
		out = append(out, Violation{Code: CodeAudioRequired, Pointer: p, Required: true, Reason: "audio is required for grades 1–3"})
	}
	return out
}

// Warnings lists text choices without audio of a g1_3 question (recommended, not blocking).
func (AudioRule) Warnings(s Subject) []Violation {
	if !G13(s.GradeLevels) || s.Type != contentmodel.TypeQuestion {
		return nil
	}
	var doc struct {
		RenderPayload struct {
			Options []struct {
				Content map[string]json.RawMessage `json:"content"`
			} `json:"options"`
		} `json:"render_payload"`
	}
	if json.Unmarshal(s.Doc, &doc) != nil {
		return nil
	}
	var out []Violation
	for i, o := range doc.RenderPayload.Options {
		if _, ok := o.Content["audio_ref"]; !ok && o.Content != nil {
			out = append(out, Violation{Code: CodeAudioRequired, Pointer: "/render_payload/options/" + strconv.Itoa(i) + "/content/audio_ref",
				Reason: "audio is recommended for text choices of grades 1–3"})
		}
	}
	return out
}

func present(m map[string]json.RawMessage, k string) bool {
	v, ok := m[k]
	return ok && len(v) > 0 && string(v) != "null" && string(v) != `""`
}

// requiredAudio returns the pointers of the missing audio references.
func requiredAudio(s Subject) []string {
	var doc map[string]json.RawMessage
	if json.Unmarshal(s.Doc, &doc) != nil {
		return nil
	}
	switch s.Type {
	case contentmodel.TypeQuestion:
		var stem map[string]json.RawMessage
		_ = json.Unmarshal(doc["stem"], &stem)
		if !present(stem, "audio_ref") {
			return []string{"/stem/audio_ref"}
		}
	case contentmodel.TypeActivity:
		if !present(doc, "instructions_audio_ref") {
			return []string{"/instructions_audio_ref"}
		}
	case contentmodel.TypeLesson, contentmodel.TypeLearningMaterial:
		var blocks []struct {
			Kind  string                       `json:"kind"`
			Steps []map[string]json.RawMessage `json:"steps"`
		}
		_ = json.Unmarshal(doc["blocks"], &blocks)
		var out []string
		for i, b := range blocks {
			if b.Kind != "worked_example" {
				continue
			}
			for j, st := range b.Steps {
				if !present(st, "audio_ref") {
					out = append(out, "/blocks/"+strconv.Itoa(i)+"/steps/"+strconv.Itoa(j)+"/audio_ref")
				}
			}
		}
		return out
	}
	return nil
}

// LanguageRule: every learning text has its content_language version (F025-AC-18).
type LanguageRule struct{}

// Check implements Rule.
func (LanguageRule) Check(s Subject) []Violation {
	var out []Violation
	for _, p := range i18ntext.MissingSource(s.Doc, s.ContentLanguage) {
		out = append(out, Violation{Code: CodeTranslationIncomplete, Pointer: p, Required: true,
			Reason: "the " + s.ContentLanguage + " text (content_language) is required"})
	}
	return out
}
