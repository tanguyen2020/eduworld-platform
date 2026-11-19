package question

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

// Handler is one question type: semantic validation, scorer and renderer (plan F022 "registry"). Validate reports
// violations as JSON pointers; Grade returns the fraction of the maximum (0–1) under the partial method of the rule
// and the parts, or ErrInvalidResponse; Render builds the white-listed payload, shuffling lists with shuffle.
type Handler interface {
	Type() Type
	// PartialMethods are the partial_method values allowed with mode partial (empty = all_or_nothing only).
	PartialMethods() []string
	Validate(a Authoring, ids *IDSet) []Violation
	Grade(c Content, response json.RawMessage) (fraction float64, parts []Part, selected []string, err error)
	Render(c Content, shuffle Shuffler) (map[string]any, error)
}

// Violation is one invalid field (a JSON pointer).
type Violation = contentmodel.Violation

// Shuffler returns ids in the presentation order of the list named name.
type Shuffler func(name string, ids []string) []string

// IDSet collects the itemIds of a version: they must be unique across every list (QUESTION_BANK §2).
type IDSet struct {
	seen map[string]string
	v    []Violation
}

// Add records id found at pointer ptr.
func (s *IDSet) Add(ptr, id string) {
	if s.seen == nil {
		s.seen = map[string]string{}
	}
	switch {
	case !ValidItemID(id):
		s.v = append(s.v, Violation{Field: ptr, Reason: "must match ^[a-z]_[a-z0-9_]{1,24}$ (no A–D, no index)"})
	case s.seen[id] != "":
		s.v = append(s.v, Violation{Field: ptr, Reason: "duplicates the id at " + s.seen[id]})
	default:
		s.seen[id] = ptr
	}
}

// Registry holds the handlers by type.
type Registry struct{ handlers map[Type]Handler }

// NewRegistry registers hs.
func NewRegistry(hs ...Handler) *Registry {
	r := &Registry{handlers: map[Type]Handler{}}
	for _, h := range hs {
		r.handlers[h.Type()] = h
	}
	return r
}

// Base returns the registry of the 8 base types of F022.
func Base() *Registry {
	return NewRegistry(singleChoice{}, multipleChoice{}, trueFalse{}, fillInBlank{}, numericType{}, matching{}, ordering{}, dragDrop{})
}

// Handler returns the handler of t or ErrTypeNotEnabled.
func (r *Registry) Handler(t Type) (Handler, error) {
	h, ok := r.handlers[t]
	if !ok {
		return nil, ErrTypeNotEnabled
	}
	return h, nil
}

// Types lists the enabled types.
func (r *Registry) Types() []Type {
	out := make([]Type, 0, len(r.handlers))
	for t := range r.handlers {
		out = append(out, t)
	}
	slices.Sort(out)
	return out
}

// Limits of a version (CONFIG_DEFAULTS assets_per_version_max, QUESTION_BANK §2).
type Limits struct {
	AssetsPerVersion int // assets_per_version_max (50)
}

// MaxHints is the largest number of hint levels (QUESTION_BANK §2).
const MaxHints = 5

// Validate checks the semantic rules of an authoring version (F022-AC-02…05): enabled type, scoring rule and partial
// method of the type, max_score > 0, unique and well-formed ids, type rules, hints, media count and a rendered asset
// for every formula. Structural rules (JSON Schema) are checked before by the caller.
func (r *Registry) Validate(a Authoring, lim Limits) error {
	h, err := r.Handler(a.Type)
	if err != nil {
		return err
	}
	var b violations
	if !(a.MaxScore > 0) || math.IsInf(a.MaxScore, 0) {
		b.add("/max_score", "must be greater than 0")
	}
	switch a.Scoring.Mode {
	case ModeAllOrNothing:
	case ModePartial:
		if !slices.Contains(h.PartialMethods(), a.Scoring.PartialMethod) {
			b.add("/scoring_rule/partial_method", fmt.Sprintf("must be one of %v for %s", h.PartialMethods(), a.Type))
		}
	default:
		b.add("/scoring_rule/mode", "must be all_or_nothing or partial")
	}
	if m := a.Scoring.MinScore; m != nil && (*m < 0 || *m >= a.MaxScore) {
		b.add("/scoring_rule/min_score", "must be at least 0 and below max_score")
	}
	ids := &IDSet{}
	for _, v := range h.Validate(a, ids) {
		b.v = append(b.v, v)
	}
	for _, v := range ids.v {
		b.v = append(b.v, v)
	}
	if len(a.Hints) > MaxHints {
		b.add("/hints", "has more than 5 levels")
	}
	if a.MinResponseMS != nil && (*a.MinResponseMS < 0 || *a.MinResponseMS > 600000) {
		b.add("/min_response_ms", "must be 0–600000")
	}
	docs := append([]json.RawMessage{a.Stem, a.RenderPayload, a.Explanation}, a.Hints...)
	media, formulas := 0, map[string]bool{}
	for _, d := range docs {
		walk(d, func(key string, val any) {
			switch key {
			case "media_ref", "background_media_ref", "audio_ref":
				if s, ok := val.(string); ok && s != "" {
					media++
				}
			case "latex":
				if s, ok := val.(string); ok {
					formulas[s] = true
				}
			}
		})
	}
	if lim.AssetsPerVersion > 0 && media > lim.AssetsPerVersion {
		b.add("/media_refs", "has more than "+strconv.Itoa(lim.AssetsPerVersion)+" media (assets_per_version_max)")
	}
	assets := map[string]bool{}
	for _, m := range a.MathAssets {
		assets[m.Latex] = true
	}
	for f := range formulas {
		if !assets[f] {
			b.add("/math_assets", "a formula has no rendered asset (math_assets with the same latex)")
			break
		}
	}
	return b.err()
}

// walk calls fn for every key/value of the JSON document (objects at any depth).
func walk(doc json.RawMessage, fn func(key string, val any)) {
	if len(doc) == 0 {
		return
	}
	var v any
	if err := json.Unmarshal(doc, &v); err != nil {
		return
	}
	var rec func(any)
	rec = func(x any) {
		switch t := x.(type) {
		case map[string]any:
			for k, val := range t {
				fn(k, val)
				rec(val)
			}
		case []any:
			for _, val := range t {
				rec(val)
			}
		}
	}
	rec(v)
}

// Grade scores a response with the handler of the type (F022-AC-14…21): only the answer key, the scoring rule and
// the response are inputs — never a score, is_correct or answer sent by a client. The score is in [min_score,
// max_score]; is_correct means the maximum.
func (r *Registry) Grade(c Content, response json.RawMessage) (GradeResult, error) {
	h, err := r.Handler(c.Type)
	if err != nil {
		return GradeResult{}, err
	}
	frac, parts, selected, err := safeGrade(h, c, response)
	if err != nil {
		return GradeResult{}, err
	}
	if c.Scoring.Mode != ModePartial && frac < 1 {
		frac = 0
	}
	frac = math.Max(0, math.Min(1, frac))
	score := round4(frac * c.MaxScore)
	if m := c.Scoring.MinScore; m != nil && score < *m {
		score = *m
	}
	return GradeResult{Score: score, MaxScore: c.MaxScore, IsCorrect: frac >= 1, Parts: parts, SelectedOptionIDs: selected}, nil
}

// safeGrade turns any panic of a scorer on malformed stored data into an invalid response (never a 500, AC-19).
func safeGrade(h Handler, c Content, resp json.RawMessage) (frac float64, parts []Part, selected []string, err error) {
	defer func() {
		if p := recover(); p != nil {
			frac, parts, selected, err = 0, nil, nil, ErrInvalidResponse
		}
	}()
	return h.Grade(c, resp)
}

func round4(x float64) float64 { return math.Round(x*10000) / 10000 }

// Render builds the render DTO of a version for one attempt (F022-AC-22, AC-23, AC-25): lists are shuffled with a
// PRNG seeded by (seed, version id), ids are kept; formulas become math_ref + speech_text.
func (r *Registry) Render(c Content, seed uint64) (Rendered, Presentation, error) {
	h, err := r.Handler(c.Type)
	if err != nil {
		return Rendered{}, Presentation{}, err
	}
	hs := fnv.New64a()
	_, _ = hs.Write(c.VersionID[:])
	rng := rand.New(rand.NewPCG(seed, hs.Sum64())) //nolint:gosec // reproducible presentation order, not a secret
	p := Presentation{QuestionVersionID: c.VersionID, Seed: seed, Orders: map[string][]string{}}
	shuffle := func(name string, ids []string) []string {
		out := slices.Clone(ids)
		rng.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
		return out
	}
	payload, err := h.Render(c, shuffle)
	if err != nil {
		return Rendered{}, Presentation{}, err
	}
	for _, name := range append(slices.Clone(optionLists), "word_bank") {
		switch list := payload[name].(type) {
		case []map[string]any:
			for _, it := range list {
				p.Orders[name] = append(p.Orders[name], fmt.Sprint(it["id"]))
			}
		case []string:
			p.Orders[name] = slices.Clone(list)
		}
	}
	if rich, err := renderOptions(payload, c.MathAssets); err == nil {
		payload = rich
	}
	stem, err := renderRich(c.Stem, c.MathAssets)
	if err != nil {
		return Rendered{}, Presentation{}, err
	}
	if len(p.Orders) == 0 {
		p.Orders = nil
	}
	return Rendered{QuestionVersionID: c.VersionID, Type: c.Type, Stem: stem, Payload: payload, MaxScore: c.MaxScore}, p, nil
}

// renderOptions rewrites the formulas of the option contents of a payload (math_ref instead of LaTeX).
func renderOptions(payload map[string]any, assets []MathAsset) (map[string]any, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	v, err := renderRich(b, assets)
	if err != nil {
		return nil, err
	}
	m, _ := v.(map[string]any)
	return m, nil
}

// renderRich rewrites a rich content for the client: math blocks carry math_ref (SVG asset) and speech_text instead of
// LaTeX (ADR-0055); nil stays nil.
func renderRich(doc json.RawMessage, assets []MathAsset) (any, error) {
	if len(doc) == 0 || bytes.Equal(doc, []byte("null")) {
		return nil, nil
	}
	var v any
	if err := json.Unmarshal(doc, &v); err != nil {
		return nil, err
	}
	byLatex := map[string]MathAsset{}
	for _, a := range assets {
		byLatex[a.Latex] = a
	}
	var rec func(any) any
	rec = func(x any) any {
		switch t := x.(type) {
		case map[string]any:
			if t["type"] == "math" {
				if l, ok := t["latex"].(string); ok {
					delete(t, "latex")
					if a, ok := byLatex[l]; ok {
						t["math_ref"], t["speech_text"] = a.SVGAsset, a.SpeechText
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
	return rec(v), nil
}

// decodeStrict decodes a response: unknown fields, trailing data or a wrong shape are ErrInvalidResponse.
func decodeStrict(raw json.RawMessage, v any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return ErrInvalidResponse
	}
	if d.More() {
		return ErrInvalidResponse
	}
	return nil
}

func ptr(base string, parts ...any) string {
	var b strings.Builder
	b.WriteString(base)
	for _, p := range parts {
		b.WriteByte('/')
		_, _ = fmt.Fprint(&b, p)
	}
	return b.String()
}
