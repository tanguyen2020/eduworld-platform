package question

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/normalize"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/widget"
)

// Full returns the registry of the eleven types (F022 + F023: short_answer, image_based, interactive) with the
// widgets of the release.
func Full() *Registry {
	r := Base()
	for _, h := range []Handler{shortAnswer{}, imageBased{}, interactive{widgets: widget.Default()}} {
		r.handlers[h.Type()] = h
	}
	return r
}

// Widgets returns the widget registry of the interactive handler (nil when interactive is not enabled).
func (r *Registry) Widgets() *widget.Registry {
	if h, ok := r.handlers[Interactive].(interactive); ok {
		return h.widgets
	}
	return nil
}

// IntegrityGrader is implemented by handlers whose grade can detect a forged response (interactive: replayed state
// differs from the state sent, F023-AC-11).
type IntegrityGrader interface {
	GradeIntegrity(c Content, response json.RawMessage) (fraction float64, parts []Part, selected []string, mismatch bool, err error)
}

// ---------- short_answer (§3.5)

type shortAnswer struct{}

type shortPayload struct {
	MaxLength *int   `json:"max_length"`
	Keyboard  string `json:"keyboard"`
}

func (shortAnswer) Type() Type               { return ShortAnswer }
func (shortAnswer) PartialMethods() []string { return nil }

// Validate (F023-AC-01, AC-20): at least one accepted answer of 1–255 characters, fuzzy distance 1–2 and length ≥ 3.
func (shortAnswer) Validate(a Authoring, _ *IDSet) []Violation {
	var k textMatch
	if unmarshal(a.AnswerKey, &k) != nil {
		return []Violation{{Field: "/answer_key", Reason: "is not a short_answer answer key"}}
	}
	var v []Violation
	if len(k.Accepted) == 0 || len(k.Accepted) > 50 {
		v = append(v, Violation{Field: "/answer_key/accepted", Reason: "needs 1–50 accepted answers"})
	}
	for i, s := range k.Accepted {
		if n := len([]rune(s)); n < 1 || n > 255 {
			v = append(v, Violation{Field: ptr("/answer_key/accepted", i), Reason: "must have 1–255 characters"})
		}
	}
	if f := k.Fuzzy; f != nil && f.Enabled {
		if f.MaxEditDistance < 1 || f.MaxEditDistance > 2 {
			v = append(v, Violation{Field: "/answer_key/fuzzy/max_edit_distance", Reason: "must be 1 or 2"})
		}
		if f.MinLength < 3 {
			v = append(v, Violation{Field: "/answer_key/fuzzy/min_length", Reason: "must be at least 3"})
		}
	}
	return v
}

// FuzzyAnywhere reports a text match with fuzzy enabled anywhere in an answer key (short_answer, blanks of
// fill_in_blank); F023-AC-03 is checked against the subject by the use case.
func FuzzyAnywhere(answerKey json.RawMessage) bool {
	found := false
	walk(answerKey, func(key string, val any) {
		if f, ok := val.(map[string]any); ok && key == "fuzzy" && f["enabled"] == true {
			found = true
		}
	})
	return found
}

// Grade (F023-AC-02, AC-04): the normalization of fill_in_blank, missing diacritics when allowed, fuzzy over
// min_length, English rules for content_language en.
func (shortAnswer) Grade(c Content, resp json.RawMessage) (float64, []Part, []string, error) {
	var k textMatch
	var p shortPayload
	var r struct {
		Text *string `json:"text"`
	}
	_ = unmarshal(c.AnswerKey, &k)
	_ = unmarshal(c.Render, &p)
	max := 500
	if p.MaxLength != nil {
		max = *p.MaxLength
	}
	if decodeStrict(resp, &r) != nil || r.Text == nil || *r.Text == "" || len([]rune(*r.Text)) > max {
		return 0, nil, nil, ErrInvalidResponse
	}
	ok := normalize.Equal(*r.Text, k.normalize(), c.Language)
	frac := 0.0
	if ok {
		frac = 1
	}
	return frac, []Part{{ID: "text", Correct: ok}}, nil, nil
}

func (shortAnswer) Render(c Content, _ Shuffler) (map[string]any, error) {
	var p shortPayload
	if err := unmarshal(c.Render, &p); err != nil {
		return nil, err
	}
	out := map[string]any{"max_length": 100, "keyboard": "text"}
	if p.MaxLength != nil {
		out["max_length"] = *p.MaxLength
	}
	if p.Keyboard != "" {
		out["keyboard"] = p.Keyboard
	}
	return out, nil
}

// ---------- image_based (§3.10)

type imageBased struct{}

type imageSize struct {
	W float64 `json:"w"`
	H float64 `json:"h"`
}

type dropPoint struct {
	ID string  `json:"id"`
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
}

type imagePayload struct {
	Interaction   string        `json:"interaction"`
	ImageMediaRef uuid.UUID     `json:"image_media_ref"`
	ImageSize     imageSize     `json:"image_size"`
	MaxSelections *int          `json:"max_selections"`
	Labels        []optionShape `json:"labels"`
	DropPoints    []dropPoint   `json:"drop_points"`
}

type region struct {
	ID     string       `json:"id"`
	Shape  string       `json:"shape"`
	CX     float64      `json:"cx"`
	CY     float64      `json:"cy"`
	R      float64      `json:"r"`
	X      float64      `json:"x"`
	Y      float64      `json:"y"`
	W      float64      `json:"w"`
	H      float64      `json:"h"`
	Points [][2]float64 `json:"points"`
}

type imageKey struct {
	Regions    []region          `json:"regions"`
	Correct    []string          `json:"correct_region_ids"`
	Placements map[string]string `json:"placements"`
}

// Limits of image_based (F023-AC-20).
const (
	MaxRegions       = 20
	MaxPolygonPoints = 64
	MaxImagePoints   = 10
	MaxLabels        = 16
)

func (imageBased) Type() Type               { return ImageBased }
func (imageBased) PartialMethods() []string { return []string{PartialProportional} }

func (p imagePayload) maxSelections() int {
	if p.MaxSelections == nil {
		return 1
	}
	return *p.MaxSelections
}

func (p imagePayload) inside(x, y float64) bool {
	return x >= 0 && y >= 0 && x <= p.ImageSize.W && y <= p.ImageSize.H
}

// Validate (F023-AC-05, AC-20): the image is a media of the version, regions are valid shapes inside the image, the
// correct regions exist and fit max_selections; labels place every label on an existing drop point.
func (imageBased) Validate(a Authoring, set *IDSet) []Violation {
	var p imagePayload
	var k imageKey
	if unmarshal(a.RenderPayload, &p) != nil || unmarshal(a.AnswerKey, &k) != nil {
		return []Violation{{Field: "/render_payload", Reason: "is not an image_based payload"}}
	}
	var v []Violation
	add := func(f, r string) { v = append(v, Violation{Field: f, Reason: r}) }
	if !slices.Contains(a.MediaRefs, p.ImageMediaRef) {
		add("/render_payload/image_media_ref", "must be listed in media_refs")
	}
	if !(p.ImageSize.W > 0 && p.ImageSize.H > 0) {
		add("/render_payload/image_size", "must be positive")
	}
	switch p.Interaction {
	case "hotspot":
		if len(k.Regions) == 0 || len(k.Regions) > MaxRegions {
			add("/answer_key/regions", fmt.Sprintf("needs 1–%d regions", MaxRegions))
		}
		var rids []string
		for i, r := range k.Regions {
			pp := ptr("/answer_key/regions", i)
			set.Add(pp+"/id", r.ID)
			rids = append(rids, r.ID)
			for _, msg := range checkRegion(r, p) {
				add(pp, msg)
			}
		}
		if len(k.Correct) == 0 {
			add("/answer_key/correct_region_ids", "needs at least one region")
		}
		for i, id := range k.Correct {
			if !slices.Contains(rids, id) {
				add(ptr("/answer_key/correct_region_ids", i), "is not a region")
			}
		}
		if p.maxSelections() < len(k.Correct) {
			add("/render_payload/max_selections", "is smaller than the number of correct regions")
		}
	case "label":
		if len(p.Labels) == 0 || len(p.Labels) > MaxLabels || len(p.DropPoints) == 0 || len(p.DropPoints) > MaxLabels {
			add("/render_payload", fmt.Sprintf("needs 1–%d labels and drop points", MaxLabels))
		}
		addIDs(set, "labels", p.Labels)
		var dids []string
		for i, d := range p.DropPoints {
			set.Add(ptr("/render_payload/drop_points", i, "id"), d.ID)
			dids = append(dids, d.ID)
			if !p.inside(d.X, d.Y) {
				add(ptr("/render_payload/drop_points", i), "is outside the image")
			}
		}
		if len(k.Placements) == 0 {
			add("/answer_key/placements", "is required for label")
		}
		for lid, did := range k.Placements {
			if !slices.Contains(ids(p.Labels), lid) {
				add(ptr("/answer_key/placements", lid), "is not a label")
			}
			if !slices.Contains(dids, did) {
				add(ptr("/answer_key/placements", lid), "is not a drop point")
			}
		}
	default:
		add("/render_payload/interaction", "must be hotspot or label")
	}
	return v
}

func checkRegion(r region, p imagePayload) []string {
	var out []string
	switch r.Shape {
	case "circle":
		if !(r.R > 0) || !p.inside(r.CX-r.R, r.CY-r.R) || !p.inside(r.CX+r.R, r.CY+r.R) {
			out = append(out, "a circle needs r > 0 inside the image")
		}
	case "rect":
		if !(r.W > 0 && r.H > 0) || !p.inside(r.X, r.Y) || !p.inside(r.X+r.W, r.Y+r.H) {
			out = append(out, "a rect needs w, h > 0 inside the image")
		}
	case "polygon":
		if len(r.Points) < 3 || len(r.Points) > MaxPolygonPoints {
			out = append(out, fmt.Sprintf("a polygon needs 3–%d points", MaxPolygonPoints))
			break
		}
		for _, pt := range r.Points {
			if !p.inside(pt[0], pt[1]) {
				out = append(out, "a polygon point is outside the image")
				break
			}
		}
		if selfIntersects(r.Points) {
			out = append(out, "the polygon crosses itself")
		}
	default:
		out = append(out, "shape must be circle, rect or polygon")
	}
	return out
}

// contains reports whether (x, y) is in r: circle and rect boundaries included, polygon by the even-odd rule.
func (r region) contains(x, y float64) bool {
	switch r.Shape {
	case "circle":
		dx, dy := x-r.CX, y-r.CY
		return dx*dx+dy*dy <= r.R*r.R
	case "rect":
		return x >= r.X && x <= r.X+r.W && y >= r.Y && y <= r.Y+r.H
	case "polygon":
		in := false
		n := len(r.Points)
		for i, j := 0, n-1; i < n; j, i = i, i+1 {
			xi, yi, xj, yj := r.Points[i][0], r.Points[i][1], r.Points[j][0], r.Points[j][1]
			if (yi > y) != (yj > y) && x < (xj-xi)*(y-yi)/(yj-yi)+xi {
				in = !in
			}
		}
		return in
	}
	return false
}

// selfIntersects reports two non-adjacent edges that cross.
func selfIntersects(pts [][2]float64) bool {
	n := len(pts)
	cross := func(o, a, b [2]float64) float64 { return (a[0]-o[0])*(b[1]-o[1]) - (a[1]-o[1])*(b[0]-o[0]) }
	for i := range n {
		a1, a2 := pts[i], pts[(i+1)%n]
		for j := i + 1; j < n; j++ {
			if j == i || (j+1)%n == i || (i+1)%n == j {
				continue
			}
			b1, b2 := pts[j], pts[(j+1)%n]
			d1, d2 := cross(b1, b2, a1), cross(b1, b2, a2)
			d3, d4 := cross(a1, a2, b1), cross(a1, a2, b2)
			if ((d1 > 0) != (d2 > 0)) && ((d3 > 0) != (d4 > 0)) && d1 != 0 && d2 != 0 && d3 != 0 && d4 != 0 {
				return true
			}
		}
	}
	return false
}

// Grade (F023-AC-07, AC-08): hotspot — the set of regions hit equals the correct set and the points fit
// max_selections; label — each placement compared, proportional parts.
func (imageBased) Grade(c Content, resp json.RawMessage) (float64, []Part, []string, error) {
	var p imagePayload
	var k imageKey
	_ = unmarshal(c.Render, &p)
	_ = unmarshal(c.AnswerKey, &k)
	switch p.Interaction {
	case "hotspot":
		var r struct {
			Points []struct {
				X *float64 `json:"x"`
				Y *float64 `json:"y"`
			} `json:"points"`
		}
		if decodeStrict(resp, &r) != nil || len(r.Points) == 0 || len(r.Points) > MaxImagePoints {
			return 0, nil, nil, ErrInvalidResponse
		}
		hit := map[string]bool{}
		for _, pt := range r.Points {
			if pt.X == nil || pt.Y == nil || !p.inside(*pt.X, *pt.Y) {
				return 0, nil, nil, ErrInvalidResponse
			}
			for _, rg := range k.Regions {
				if rg.contains(*pt.X, *pt.Y) {
					hit[rg.ID] = true
				}
			}
		}
		var parts []Part
		ok := len(r.Points) <= p.maxSelections()
		for _, id := range k.Correct {
			parts = append(parts, Part{ID: id, Correct: hit[id]})
			if !hit[id] {
				ok = false
			}
		}
		for id := range hit {
			if !slices.Contains(k.Correct, id) {
				ok = false
				parts = append(parts, Part{ID: id, Correct: false})
			}
		}
		frac := 0.0
		if ok {
			frac = 1
		} else if c.Scoring.Mode == ModePartial && len(r.Points) <= p.maxSelections() {
			frac = countParts(parts, len(parts))
		}
		return frac, parts, nil, nil
	case "label":
		var r struct {
			Placements map[string]string `json:"label_placements"`
		}
		if decodeStrict(resp, &r) != nil || len(r.Placements) == 0 || len(r.Placements) > MaxLabels {
			return 0, nil, nil, ErrInvalidResponse
		}
		var dids []string
		for _, d := range p.DropPoints {
			dids = append(dids, d.ID)
		}
		for lid, did := range r.Placements {
			if !slices.Contains(ids(p.Labels), lid) || !slices.Contains(dids, did) {
				return 0, nil, nil, ErrInvalidResponse
			}
		}
		var parts []Part
		for _, lid := range ids(p.Labels) {
			want, scored := k.Placements[lid]
			if !scored {
				continue
			}
			parts = append(parts, Part{ID: lid, Correct: r.Placements[lid] == want})
		}
		return countParts(parts, len(parts)), parts, nil, nil
	}
	return 0, nil, nil, ErrInvalidResponse
}

// Render (F023-AC-06): never regions, correct ids or placements; labels shuffled.
func (imageBased) Render(c Content, shuffle Shuffler) (map[string]any, error) {
	var p imagePayload
	if err := unmarshal(c.Render, &p); err != nil {
		return nil, err
	}
	out := map[string]any{"interaction": p.Interaction, "image_media_ref": p.ImageMediaRef, "image_size": p.ImageSize}
	switch p.Interaction {
	case "hotspot":
		out["max_selections"] = p.maxSelections()
	case "label":
		out["labels"] = order(p.Labels, shuffle("labels", ids(p.Labels)))
		out["drop_points"] = p.DropPoints
	}
	return out, nil
}

// WidgetOf returns the widget@version of an interactive render payload (metric label).
func WidgetOf(render json.RawMessage) string {
	var p interactivePayload
	if unmarshal(render, &p) != nil || p.Widget == "" {
		return "unknown"
	}
	return p.Widget + "@" + p.WidgetVersion
}

// ---------- interactive (§3.11)

type interactive struct{ widgets *widget.Registry }

type interactivePayload struct {
	Widget        string          `json:"widget"`
	WidgetVersion string          `json:"widget_version"`
	Config        json.RawMessage `json:"config"`
}

type interactiveKey struct {
	Validator string          `json:"validator"`
	Params    json.RawMessage `json:"params"`
}

// MaxActionLog bounds interactiveResponse.action_log (F023-AC-12).
const MaxActionLog = 500

func (interactive) Type() Type               { return Interactive }
func (interactive) PartialMethods() []string { return nil }

// Validate (F023-AC-09): a registered widget version, a validator of that widget, config and params valid for it.
func (h interactive) Validate(a Authoring, _ *IDSet) []Violation {
	var p interactivePayload
	var k interactiveKey
	if unmarshal(a.RenderPayload, &p) != nil || unmarshal(a.AnswerKey, &k) != nil {
		return []Violation{{Field: "/render_payload", Reason: "is not an interactive payload"}}
	}
	w, err := h.widgets.Get(p.Widget, p.WidgetVersion)
	if err != nil {
		return []Violation{{Field: "/render_payload/widget", Reason: "is not a registered widget version"}}
	}
	if !slices.Contains(w.Validators(), k.Validator) {
		return []Violation{{Field: "/answer_key/validator", Reason: "is not a validator of " + p.Widget + "@" + p.WidgetVersion}}
	}
	var v []Violation
	for _, msg := range w.CheckConfig(p.Config, k.Validator, k.Params) {
		v = append(v, Violation{Field: "/render_payload/config", Reason: msg})
	}
	return v
}

func (h interactive) Grade(c Content, resp json.RawMessage) (float64, []Part, []string, error) {
	f, parts, sel, _, err := h.GradeIntegrity(c, resp)
	return f, parts, sel, err
}

// GradeIntegrity (F023-AC-10, AC-11, AC-12): only the registered Go validator decides; with require_action_log the
// log is replayed from the initial config and a state that differs scores 0 with integrity_mismatch.
func (h interactive) GradeIntegrity(c Content, resp json.RawMessage) (float64, []Part, []string, bool, error) {
	var p interactivePayload
	var k interactiveKey
	_ = unmarshal(c.Render, &p)
	_ = unmarshal(c.AnswerKey, &k)
	var r struct {
		State     json.RawMessage `json:"state"`
		ActionLog []widget.Action `json:"action_log"`
	}
	if decodeStrict(resp, &r) != nil || len(r.State) == 0 || len(r.ActionLog) > MaxActionLog {
		return 0, nil, nil, false, ErrInvalidResponse
	}
	w, err := h.widgets.Get(p.Widget, p.WidgetVersion)
	if err != nil {
		return 0, nil, nil, false, err
	}
	if c.Scoring.RequireActionLog {
		replayed, err := w.Replay(p.Config, r.ActionLog)
		if err != nil || !w.Same(replayed, r.State) {
			return 0, []Part{{ID: "state", Correct: false}}, nil, true, nil //nolint:nilerr // a log that cannot be replayed is a forged response: score 0 and flag it (F023-AC-11)
		}
	}
	ok, err := w.Check(k.Validator, p.Config, k.Params, r.State)
	if errors.Is(err, widget.ErrIllegalAction) {
		return 0, nil, nil, false, ErrInvalidResponse
	}
	if err != nil {
		return 0, nil, nil, false, err
	}
	frac := 0.0
	if ok {
		frac = 1
	}
	return frac, []Part{{ID: "state", Correct: ok}}, nil, false, nil
}

func (interactive) Render(c Content, _ Shuffler) (map[string]any, error) {
	var p interactivePayload
	if err := unmarshal(c.Render, &p); err != nil {
		return nil, err
	}
	return map[string]any{"widget": p.Widget, "widget_version": p.WidgetVersion, "config": p.Config}, nil
}
