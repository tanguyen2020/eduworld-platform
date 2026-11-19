package question

import (
	"encoding/json"
	"math/big"
	"regexp"
	"slices"
	"sort"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/normalize"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/numeric"
)

// Shapes of the render payloads and answer keys (question.schema.json $defs). Unknown fields are refused by the
// JSON Schema before the semantic validation; decoding here is lenient.

type optionShape struct {
	ID      string          `json:"id"`
	Content json.RawMessage `json:"content"`
}

func ids(opts []optionShape) []string {
	out := make([]string, len(opts))
	for i, o := range opts {
		out[i] = o.ID
	}
	return out
}

func addIDs(set *IDSet, list string, opts []optionShape) {
	for i, o := range opts {
		set.Add(ptr("/render_payload", list, i, "id"), o.ID)
	}
}

// order returns opts in the order of shuffled ids.
func order(opts []optionShape, shuffled []string) []map[string]any {
	by := map[string]optionShape{}
	for _, o := range opts {
		by[o.ID] = o
	}
	out := make([]map[string]any, 0, len(opts))
	for _, id := range shuffled {
		out = append(out, map[string]any{"id": id, "content": by[id].Content})
	}
	return out
}

func boolOr(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}

func unmarshal(raw json.RawMessage, v any) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, v)
}

// countParts returns the fraction of correct parts.
func countParts(parts []Part, total int) float64 {
	if total == 0 {
		return 0
	}
	c := 0
	for _, p := range parts {
		if p.Correct {
			c++
		}
	}
	return float64(c) / float64(total)
}

// ---------- single_choice (§3.1)

type singleChoice struct{}

type choicePayload struct {
	Options          []optionShape `json:"options"`
	ShuffleOptions   *bool         `json:"shuffle_options"`
	ShowCorrectCount bool          `json:"show_correct_count"`
}

func (singleChoice) Type() Type               { return SingleChoice }
func (singleChoice) PartialMethods() []string { return nil }

func (singleChoice) Validate(a Authoring, set *IDSet) []Violation {
	var p choicePayload
	var k struct {
		Correct string `json:"correct_option_id"`
	}
	if unmarshal(a.RenderPayload, &p) != nil || unmarshal(a.AnswerKey, &k) != nil {
		return []Violation{{Field: "/render_payload", Reason: "is not a single_choice payload"}}
	}
	addIDs(set, "options", p.Options)
	if !slices.Contains(ids(p.Options), k.Correct) {
		return []Violation{{Field: "/answer_key/correct_option_id", Reason: "must be the id of exactly one option"}}
	}
	return nil
}

func (singleChoice) Grade(c Content, resp json.RawMessage) (float64, []Part, []string, error) {
	var p choicePayload
	var k struct {
		Correct string `json:"correct_option_id"`
	}
	var r struct {
		OptionID *string `json:"option_id"`
	}
	_ = unmarshal(c.Render, &p)
	_ = unmarshal(c.AnswerKey, &k)
	if decodeStrict(resp, &r) != nil || r.OptionID == nil || !slices.Contains(ids(p.Options), *r.OptionID) {
		return 0, nil, nil, ErrInvalidResponse
	}
	ok := *r.OptionID == k.Correct
	frac := 0.0
	if ok {
		frac = 1
	}
	return frac, []Part{{ID: *r.OptionID, Correct: ok}}, []string{*r.OptionID}, nil
}

func (singleChoice) Render(c Content, shuffle Shuffler) (map[string]any, error) {
	var p choicePayload
	if err := unmarshal(c.Render, &p); err != nil {
		return nil, err
	}
	o := ids(p.Options)
	if boolOr(p.ShuffleOptions, true) {
		o = shuffle("options", o)
	}
	return map[string]any{"options": order(p.Options, o)}, nil
}

// ---------- multiple_choice (§3.2)

type multipleChoice struct{}

type multiKey struct {
	Correct []string `json:"correct_option_ids"`
}

func (multipleChoice) Type() Type { return MultipleChoice }
func (multipleChoice) PartialMethods() []string {
	return []string{PartialRightMinusWrg, PartialProportional}
}

func (multipleChoice) Validate(a Authoring, set *IDSet) []Violation {
	var p choicePayload
	var k multiKey
	if unmarshal(a.RenderPayload, &p) != nil || unmarshal(a.AnswerKey, &k) != nil {
		return []Violation{{Field: "/render_payload", Reason: "is not a multiple_choice payload"}}
	}
	addIDs(set, "options", p.Options)
	var v []Violation
	all := ids(p.Options)
	if len(k.Correct) == 0 || len(k.Correct) > len(all) {
		v = append(v, Violation{Field: "/answer_key/correct_option_ids", Reason: "must hold 1 to the number of options"})
	}
	for i, id := range k.Correct {
		if !slices.Contains(all, id) {
			v = append(v, Violation{Field: ptr("/answer_key/correct_option_ids", i), Reason: "is not an option id"})
		}
	}
	return v
}

func (multipleChoice) Grade(c Content, resp json.RawMessage) (float64, []Part, []string, error) {
	var p choicePayload
	var k multiKey
	var r struct {
		OptionIDs []string `json:"option_ids"`
	}
	_ = unmarshal(c.Render, &p)
	_ = unmarshal(c.AnswerKey, &k)
	all := ids(p.Options)
	if decodeStrict(resp, &r) != nil || len(r.OptionIDs) == 0 || len(r.OptionIDs) > 10 || hasDup(r.OptionIDs) {
		return 0, nil, nil, ErrInvalidResponse
	}
	correct, wrong := 0, 0
	parts := make([]Part, 0, len(r.OptionIDs))
	for _, id := range r.OptionIDs {
		if !slices.Contains(all, id) {
			return 0, nil, nil, ErrInvalidResponse
		}
		ok := slices.Contains(k.Correct, id)
		if ok {
			correct++
		} else {
			wrong++
		}
		parts = append(parts, Part{ID: id, Correct: ok})
	}
	n := float64(len(k.Correct))
	var frac float64
	switch {
	case correct == len(k.Correct) && wrong == 0:
		frac = 1
	case c.Scoring.Mode != ModePartial:
		frac = 0
	case c.Scoring.PartialMethod == PartialProportional:
		if wrong == 0 {
			frac = float64(correct) / n
		}
	default: // right_minus_wrong
		frac = max(0, float64(correct-wrong)/n)
	}
	return frac, parts, slices.Clone(r.OptionIDs), nil
}

func (multipleChoice) Render(c Content, shuffle Shuffler) (map[string]any, error) {
	var p choicePayload
	var k multiKey
	if err := unmarshal(c.Render, &p); err != nil {
		return nil, err
	}
	o := ids(p.Options)
	if boolOr(p.ShuffleOptions, true) {
		o = shuffle("options", o)
	}
	out := map[string]any{"options": order(p.Options, o)}
	if p.ShowCorrectCount {
		_ = unmarshal(c.AnswerKey, &k)
		out["correct_count"] = len(k.Correct) // allowed display information (spec edge case)
	}
	return out, nil
}

func hasDup(xs []string) bool {
	seen := map[string]bool{}
	for _, x := range xs {
		if seen[x] {
			return true
		}
		seen[x] = true
	}
	return false
}

// ---------- true_false (§3.3)

type trueFalse struct{}

type tfPayload struct {
	Labels     json.RawMessage `json:"labels"`
	Statements []optionShape   `json:"statements"`
}

type tfKey struct {
	Value           *bool           `json:"value"`
	StatementValues map[string]bool `json:"statement_values"`
}

func (trueFalse) Type() Type               { return TrueFalse }
func (trueFalse) PartialMethods() []string { return []string{PartialProportional} }

func (trueFalse) Validate(a Authoring, set *IDSet) []Violation {
	var p tfPayload
	var k tfKey
	if unmarshal(a.RenderPayload, &p) != nil || unmarshal(a.AnswerKey, &k) != nil {
		return []Violation{{Field: "/render_payload", Reason: "is not a true_false payload"}}
	}
	addIDs(set, "statements", p.Statements)
	switch {
	case len(p.Statements) == 0:
		if k.Value == nil {
			return []Violation{{Field: "/answer_key/value", Reason: "is required without statements"}}
		}
		if a.Scoring.Mode == ModePartial {
			return []Violation{{Field: "/scoring_rule/mode", Reason: "partial needs statements"}}
		}
	default:
		var v []Violation
		if len(k.StatementValues) != len(p.Statements) {
			v = append(v, Violation{Field: "/answer_key/statement_values", Reason: "must give a value to every statement"})
		}
		for id := range k.StatementValues {
			if !slices.Contains(ids(p.Statements), id) {
				v = append(v, Violation{Field: ptr("/answer_key/statement_values", id), Reason: "is not a statement id"})
			}
		}
		return v
	}
	return nil
}

func (trueFalse) Grade(c Content, resp json.RawMessage) (float64, []Part, []string, error) {
	var p tfPayload
	var k tfKey
	var r struct {
		Value           *bool            `json:"value"`
		StatementValues *map[string]bool `json:"statement_values"`
	}
	_ = unmarshal(c.Render, &p)
	_ = unmarshal(c.AnswerKey, &k)
	if decodeStrict(resp, &r) != nil || (r.Value == nil) == (r.StatementValues == nil) {
		return 0, nil, nil, ErrInvalidResponse
	}
	if len(p.Statements) == 0 {
		if r.Value == nil || k.Value == nil {
			return 0, nil, nil, ErrInvalidResponse
		}
		ok := *r.Value == *k.Value
		if ok {
			return 1, []Part{{ID: "value", Correct: true}}, nil, nil
		}
		return 0, []Part{{ID: "value", Correct: false}}, nil, nil
	}
	if r.StatementValues == nil || len(*r.StatementValues) == 0 || len(*r.StatementValues) > 8 {
		return 0, nil, nil, ErrInvalidResponse
	}
	sids := ids(p.Statements)
	for id := range *r.StatementValues {
		if !slices.Contains(sids, id) {
			return 0, nil, nil, ErrInvalidResponse
		}
	}
	parts := make([]Part, 0, len(sids))
	for _, id := range sids {
		v, given := (*r.StatementValues)[id]
		parts = append(parts, Part{ID: id, Correct: given && v == k.StatementValues[id]})
	}
	return countParts(parts, len(sids)), parts, nil, nil
}

func (trueFalse) Render(c Content, shuffle Shuffler) (map[string]any, error) {
	var p tfPayload
	if err := unmarshal(c.Render, &p); err != nil {
		return nil, err
	}
	out := map[string]any{}
	if len(p.Labels) > 0 {
		out["labels"] = p.Labels
	}
	if len(p.Statements) > 0 {
		out["statements"] = order(p.Statements, ids(p.Statements)) // statement order is part of the question
	}
	return out, nil
}

// ---------- fill_in_blank (§3.4)

type fillInBlank struct{}

type blankShape struct {
	ID        string   `json:"id"`
	Input     string   `json:"input"`
	MaxLength *int     `json:"max_length,omitempty"`
	Choices   []string `json:"choices,omitempty"`
}

type fillPayload struct {
	Blanks   []blankShape `json:"blanks"`
	WordBank []string     `json:"word_bank"`
}

type textMatch struct {
	Accepted               []string `json:"accepted"`
	Match                  string   `json:"match"`
	AllowDiacriticsMissing bool     `json:"allow_diacritics_missing"`
	CaseSensitive          bool     `json:"case_sensitive"`
	Fuzzy                  *struct {
		Enabled         bool `json:"enabled"`
		MaxEditDistance int  `json:"max_edit_distance"`
		MinLength       int  `json:"min_length"`
	} `json:"fuzzy"`
}

func (m textMatch) normalize() normalize.Match {
	out := normalize.Match{Accepted: m.Accepted, Mode: m.Match, AllowDiacriticsMissing: m.AllowDiacriticsMissing, CaseSensitive: m.CaseSensitive}
	if out.Mode == "" {
		out.Mode = "normalized"
	}
	if m.Fuzzy != nil {
		out.Fuzzy = &normalize.Fuzzy{Enabled: m.Fuzzy.Enabled, MaxEditDistance: m.Fuzzy.MaxEditDistance, MinLength: m.Fuzzy.MinLength}
	}
	return out
}

var blankRe = regexp.MustCompile(`\{\{blank:([^}]*)\}\}`)

func (fillInBlank) Type() Type               { return FillInBlank }
func (fillInBlank) PartialMethods() []string { return []string{PartialProportional} }

func (fillInBlank) Validate(a Authoring, set *IDSet) []Violation {
	var p fillPayload
	var k struct {
		Blanks map[string]textMatch `json:"blanks"`
	}
	if unmarshal(a.RenderPayload, &p) != nil || unmarshal(a.AnswerKey, &k) != nil {
		return []Violation{{Field: "/render_payload", Reason: "is not a fill_in_blank payload"}}
	}
	var v []Violation
	var bids []string
	for i, b := range p.Blanks {
		set.Add(ptr("/render_payload/blanks", i, "id"), b.ID)
		bids = append(bids, b.ID)
		if _, ok := k.Blanks[b.ID]; !ok {
			v = append(v, Violation{Field: ptr("/answer_key/blanks", b.ID), Reason: "the blank has no accepted answer"})
		}
	}
	for id := range k.Blanks {
		if !slices.Contains(bids, id) {
			v = append(v, Violation{Field: ptr("/answer_key/blanks", id), Reason: "is not a blank of render_payload"})
		}
	}
	inStem := map[string]bool{}
	walk(a.Stem, func(key string, val any) {
		texts := []string{}
		switch t := val.(type) {
		case string:
			if key == "text" || key == "vi" || key == "en" {
				texts = append(texts, t)
			}
		}
		for _, s := range texts {
			for _, m := range blankRe.FindAllStringSubmatch(s, -1) {
				inStem[m[1]] = true
			}
		}
	})
	for id := range inStem {
		if !slices.Contains(bids, id) {
			v = append(v, Violation{Field: "/stem", Reason: "{{blank:" + id + "}} is not a blank of render_payload"})
		}
	}
	for _, id := range bids {
		if !inStem[id] {
			v = append(v, Violation{Field: ptr("/render_payload/blanks", id), Reason: "the blank does not appear in the stem"})
		}
	}
	return v
}

func (fillInBlank) Grade(c Content, resp json.RawMessage) (float64, []Part, []string, error) {
	var p fillPayload
	var k struct {
		Blanks map[string]textMatch `json:"blanks"`
	}
	var r struct {
		Blanks map[string]string `json:"blanks"`
	}
	_ = unmarshal(c.Render, &p)
	_ = unmarshal(c.AnswerKey, &k)
	if decodeStrict(resp, &r) != nil || len(r.Blanks) == 0 || len(r.Blanks) > 10 {
		return 0, nil, nil, ErrInvalidResponse
	}
	var bids []string
	for _, b := range p.Blanks {
		bids = append(bids, b.ID)
	}
	for id, val := range r.Blanks {
		if !slices.Contains(bids, id) || len([]rune(val)) > 200 {
			return 0, nil, nil, ErrInvalidResponse
		}
	}
	parts := make([]Part, 0, len(bids))
	for _, id := range bids {
		val, given := r.Blanks[id]
		parts = append(parts, Part{ID: id, Correct: given && normalize.Equal(val, k.Blanks[id].normalize(), c.Language)})
	}
	return countParts(parts, len(bids)), parts, nil, nil
}

func (fillInBlank) Render(c Content, shuffle Shuffler) (map[string]any, error) {
	var p fillPayload
	if err := unmarshal(c.Render, &p); err != nil {
		return nil, err
	}
	out := map[string]any{"blanks": p.Blanks}
	if len(p.WordBank) > 0 {
		out["word_bank"] = shuffle("word_bank", p.WordBank)
	}
	return out, nil
}

// ---------- numeric (§3.6)

type numericType struct{}

type numericPayload struct {
	Keyboard           string  `json:"keyboard,omitempty"`
	UnitLabel          string  `json:"unit_label,omitempty"`
	AllowFractionInput bool    `json:"allow_fraction_input"`
	DecimalSeparator   *string `json:"decimal_separator,omitempty"`
}

type numericKey struct {
	Value     json.RawMessage `json:"value"`
	Tolerance *struct {
		Type  string       `json:"type"`
		Value *json.Number `json:"value"`
	} `json:"tolerance"`
	AcceptEquivalentForms *bool `json:"accept_equivalent_forms"`
}

func (k numericKey) parse() (*big.Rat, numeric.Tolerance, error) {
	var lit json.Number
	if err := json.Unmarshal(k.Value, &lit); err != nil {
		return nil, numeric.Tolerance{}, numeric.ErrNotANumber
	}
	v, err := numeric.FromJSON(lit.String())
	if err != nil {
		return nil, numeric.Tolerance{}, err
	}
	t := numeric.Tolerance{Type: "exact"}
	if k.Tolerance != nil {
		t.Type = k.Tolerance.Type
		if k.Tolerance.Value != nil {
			tv, err := numeric.FromJSON(k.Tolerance.Value.String())
			if err != nil || tv.Sign() < 0 {
				return nil, t, numeric.ErrNotANumber
			}
			t.Value = tv
		}
	}
	return v, t, nil
}

func (numericType) Type() Type               { return Numeric }
func (numericType) PartialMethods() []string { return nil }

func (numericType) Validate(a Authoring, _ *IDSet) []Violation {
	var k numericKey
	if unmarshal(a.AnswerKey, &k) != nil {
		return []Violation{{Field: "/answer_key", Reason: "is not a numeric answer key"}}
	}
	if _, _, err := k.parse(); err != nil {
		return []Violation{{Field: "/answer_key/value", Reason: "must be a decimal number and the tolerance at least 0"}}
	}
	return nil
}

func (numericType) Grade(c Content, resp json.RawMessage) (float64, []Part, []string, error) {
	var p numericPayload
	var k numericKey
	var r struct {
		Value *string `json:"value"`
	}
	_ = unmarshal(c.Render, &p)
	_ = unmarshal(c.AnswerKey, &k)
	if decodeStrict(resp, &r) != nil || r.Value == nil || *r.Value == "" || len(*r.Value) > numeric.MaxLen {
		return 0, nil, nil, ErrInvalidResponse
	}
	v, tol, err := k.parse()
	if err != nil {
		return 0, nil, nil, ErrInvalidResponse
	}
	x, err := numeric.Parse(*r.Value, numeric.Options{Language: c.Language, AllowFraction: p.AllowFractionInput,
		EquivalentForms: boolOr(k.AcceptEquivalentForms, true)})
	ok := err == nil && numeric.Within(x, v, tol)
	if ok {
		return 1, []Part{{ID: "value", Correct: true}}, nil, nil
	}
	return 0, []Part{{ID: "value", Correct: false}}, nil, nil
}

func (numericType) Render(c Content, _ Shuffler) (map[string]any, error) {
	var p numericPayload
	if err := unmarshal(c.Render, &p); err != nil {
		return nil, err
	}
	out := map[string]any{"keyboard": "numeric", "allow_fraction_input": p.AllowFractionInput}
	if p.UnitLabel != "" {
		out["unit_label"] = p.UnitLabel
	}
	if p.DecimalSeparator != nil {
		out["decimal_separator"] = *p.DecimalSeparator
	}
	return out, nil
}

// ---------- matching (§3.7)

type matching struct{}

type matchingPayload struct {
	Left    []optionShape `json:"left"`
	Right   []optionShape `json:"right"`
	Shuffle *bool         `json:"shuffle"`
}

type pairsKey struct {
	Pairs [][]string `json:"pairs"`
}

func (matching) Type() Type               { return Matching }
func (matching) PartialMethods() []string { return []string{PartialProportional} }

func (matching) Validate(a Authoring, set *IDSet) []Violation {
	var p matchingPayload
	var k pairsKey
	if unmarshal(a.RenderPayload, &p) != nil || unmarshal(a.AnswerKey, &k) != nil {
		return []Violation{{Field: "/render_payload", Reason: "is not a matching payload"}}
	}
	addIDs(set, "left", p.Left)
	addIDs(set, "right", p.Right)
	var v []Violation
	seenLeft := map[string]bool{}
	for i, pr := range k.Pairs {
		if len(pr) != 2 {
			v = append(v, Violation{Field: ptr("/answer_key/pairs", i), Reason: "must be [left id, right id]"})
			continue
		}
		if !slices.Contains(ids(p.Left), pr[0]) {
			v = append(v, Violation{Field: ptr("/answer_key/pairs", i, 0), Reason: "is not a left id"})
		}
		if !slices.Contains(ids(p.Right), pr[1]) {
			v = append(v, Violation{Field: ptr("/answer_key/pairs", i, 1), Reason: "is not a right id"})
		}
		if seenLeft[pr[0]] {
			v = append(v, Violation{Field: ptr("/answer_key/pairs", i, 0), Reason: "the left item is paired twice"})
		}
		seenLeft[pr[0]] = true
	}
	return v
}

func (matching) Grade(c Content, resp json.RawMessage) (float64, []Part, []string, error) {
	var p matchingPayload
	var k pairsKey
	var r struct {
		Pairs [][]string `json:"pairs"`
	}
	_ = unmarshal(c.Render, &p)
	_ = unmarshal(c.AnswerKey, &k)
	if decodeStrict(resp, &r) != nil || len(r.Pairs) == 0 || len(r.Pairs) > 10 {
		return 0, nil, nil, ErrInvalidResponse
	}
	key := map[string]string{}
	for _, pr := range k.Pairs {
		key[pr[0]] = pr[1]
	}
	given := map[string]string{}
	for _, pr := range r.Pairs {
		if len(pr) != 2 || !slices.Contains(ids(p.Left), pr[0]) || !slices.Contains(ids(p.Right), pr[1]) || given[pr[0]] != "" {
			return 0, nil, nil, ErrInvalidResponse
		}
		given[pr[0]] = pr[1]
	}
	lefts := make([]string, 0, len(key))
	for l := range key {
		lefts = append(lefts, l)
	}
	sort.Strings(lefts)
	parts := make([]Part, 0, len(lefts))
	for _, l := range lefts {
		parts = append(parts, Part{ID: l, Correct: given[l] == key[l]})
	}
	frac := countParts(parts, len(lefts))
	for l := range given {
		if _, inKey := key[l]; !inKey && frac == 1 {
			frac = float64(len(lefts)) / float64(len(lefts)+1) // a decoy left paired is never full marks
		}
	}
	return frac, parts, nil, nil
}

func (matching) Render(c Content, shuffle Shuffler) (map[string]any, error) {
	var p matchingPayload
	if err := unmarshal(c.Render, &p); err != nil {
		return nil, err
	}
	l, r := ids(p.Left), ids(p.Right)
	if boolOr(p.Shuffle, true) {
		l, r = shuffle("left", l), shuffle("right", r)
	}
	return map[string]any{"left": order(p.Left, l), "right": order(p.Right, r)}, nil
}

// ---------- ordering (§3.8)

type ordering struct{}

type orderingPayload struct {
	Items []optionShape `json:"items"`
}

type orderKey struct {
	Order []string `json:"order"`
}

func (ordering) Type() Type               { return Ordering }
func (ordering) PartialMethods() []string { return []string{PartialPosition, PartialAdjacentPairs} }

func (ordering) Validate(a Authoring, set *IDSet) []Violation {
	var p orderingPayload
	var k orderKey
	if unmarshal(a.RenderPayload, &p) != nil || unmarshal(a.AnswerKey, &k) != nil {
		return []Violation{{Field: "/render_payload", Reason: "is not an ordering payload"}}
	}
	addIDs(set, "items", p.Items)
	var v []Violation
	if len(p.Items) < 3 {
		v = append(v, Violation{Field: "/render_payload/items", Reason: "needs at least 3 items"})
	}
	a1, b1 := slices.Clone(ids(p.Items)), slices.Clone(k.Order)
	slices.Sort(a1)
	slices.Sort(b1)
	if !slices.Equal(a1, b1) {
		v = append(v, Violation{Field: "/answer_key/order", Reason: "must be a permutation of every item id"})
	}
	return v
}

func (ordering) Grade(c Content, resp json.RawMessage) (float64, []Part, []string, error) {
	var p orderingPayload
	var k orderKey
	var r struct {
		Order []string `json:"order"`
	}
	_ = unmarshal(c.Render, &p)
	_ = unmarshal(c.AnswerKey, &k)
	if decodeStrict(resp, &r) != nil || len(r.Order) < 3 || len(r.Order) > 10 || hasDup(r.Order) || len(r.Order) != len(k.Order) {
		return 0, nil, nil, ErrInvalidResponse
	}
	for _, id := range r.Order {
		if !slices.Contains(ids(p.Items), id) {
			return 0, nil, nil, ErrInvalidResponse
		}
	}
	n := len(k.Order)
	parts := make([]Part, n)
	for i, id := range r.Order {
		parts[i] = Part{ID: id, Correct: k.Order[i] == id}
	}
	if slices.Equal(r.Order, k.Order) {
		return 1, parts, nil, nil
	}
	if c.Scoring.PartialMethod == PartialPosition {
		return countParts(parts, n), parts, nil, nil
	}
	pos := map[string]int{}
	for i, id := range k.Order {
		pos[id] = i
	}
	adj := 0
	for i := 0; i+1 < n; i++ {
		if pos[r.Order[i+1]] == pos[r.Order[i]]+1 {
			adj++
		}
	}
	return float64(adj) / float64(n-1), parts, nil, nil
}

func (ordering) Render(c Content, shuffle Shuffler) (map[string]any, error) {
	var p orderingPayload
	var k orderKey
	if err := unmarshal(c.Render, &p); err != nil {
		return nil, err
	}
	_ = unmarshal(c.AnswerKey, &k)
	o := ids(p.Items)
	shown := o
	for try := 0; try < 10; try++ {
		shown = shuffle("items", o)
		if !slices.Equal(shown, k.Order) {
			break
		}
	}
	if slices.Equal(shown, k.Order) && len(shown) > 1 {
		shown = append(slices.Clone(shown[1:]), shown[0]) // rotate: never the right order (§3.8)
	}
	return map[string]any{"items": order(p.Items, shown)}, nil
}

// ---------- drag_drop (§3.9)

type dragDrop struct{}

type zoneShape struct {
	ID       string          `json:"id"`
	Label    json.RawMessage `json:"label"`
	Capacity *int            `json:"capacity"`
	Rect     json.RawMessage `json:"rect,omitempty"`
}

type dragPayload struct {
	Background json.RawMessage `json:"background_media_ref"`
	Draggables []optionShape   `json:"draggables"`
	Zones      []zoneShape     `json:"zones"`
	Reusable   bool            `json:"reusable_draggables"`
}

type placementsKey struct {
	Placements map[string]string `json:"placements"`
}

func (dragDrop) Type() Type               { return DragDrop }
func (dragDrop) PartialMethods() []string { return []string{PartialProportional} }

func zoneIDs(zs []zoneShape) []string {
	out := make([]string, len(zs))
	for i, z := range zs {
		out[i] = z.ID
	}
	return out
}

func (dragDrop) Validate(a Authoring, set *IDSet) []Violation {
	var p dragPayload
	var k placementsKey
	if unmarshal(a.RenderPayload, &p) != nil || unmarshal(a.AnswerKey, &k) != nil {
		return []Violation{{Field: "/render_payload", Reason: "is not a drag_drop payload"}}
	}
	addIDs(set, "draggables", p.Draggables)
	for i, z := range p.Zones {
		set.Add(ptr("/render_payload/zones", i, "id"), z.ID)
	}
	var v []Violation
	used := map[string]int{}
	for d, z := range k.Placements {
		if !slices.Contains(ids(p.Draggables), d) {
			v = append(v, Violation{Field: ptr("/answer_key/placements", d), Reason: "is not a draggable id"})
		}
		if !slices.Contains(zoneIDs(p.Zones), z) {
			v = append(v, Violation{Field: ptr("/answer_key/placements", d), Reason: "is not a zone id"})
		}
		used[z]++
	}
	for _, z := range p.Zones {
		if z.Capacity != nil && used[z.ID] > *z.Capacity {
			v = append(v, Violation{Field: ptr("/render_payload/zones", z.ID, "capacity"), Reason: "the answer key places more draggables than the capacity"})
		}
	}
	return v
}

func (dragDrop) Grade(c Content, resp json.RawMessage) (float64, []Part, []string, error) {
	var p dragPayload
	var k placementsKey
	var r struct {
		Placements map[string]string `json:"placements"`
	}
	_ = unmarshal(c.Render, &p)
	_ = unmarshal(c.AnswerKey, &k)
	if decodeStrict(resp, &r) != nil || len(r.Placements) == 0 || len(r.Placements) > 16 {
		return 0, nil, nil, ErrInvalidResponse
	}
	zones := map[string]zoneShape{}
	for _, z := range p.Zones {
		zones[z.ID] = z
	}
	for d, z := range r.Placements {
		if _, ok := zones[z]; !ok || !slices.Contains(ids(p.Draggables), d) {
			return 0, nil, nil, ErrInvalidResponse
		}
	}
	// Over capacity: the placements of a zone beyond its capacity (in id order) are wrong.
	over := map[string]bool{}
	byZone := map[string][]string{}
	for d, z := range r.Placements {
		byZone[z] = append(byZone[z], d)
	}
	for z, ds := range byZone {
		sort.Strings(ds)
		if cp := zones[z].Capacity; cp != nil && len(ds) > *cp {
			for _, d := range ds[*cp:] {
				over[d] = true
			}
		}
	}
	keyIDs := make([]string, 0, len(k.Placements))
	for d := range k.Placements {
		keyIDs = append(keyIDs, d)
	}
	sort.Strings(keyIDs)
	parts := make([]Part, 0, len(keyIDs))
	for _, d := range keyIDs {
		z, placed := r.Placements[d]
		parts = append(parts, Part{ID: d, Correct: placed && z == k.Placements[d] && !over[d]})
	}
	decoys := 0
	for d := range r.Placements {
		if _, inKey := k.Placements[d]; !inKey {
			decoys++ // a decoy placed in any zone is wrong (§3.9)
			parts = append(parts, Part{ID: d, Correct: false})
		}
	}
	return countParts(parts, len(keyIDs)+decoys), parts, nil, nil
}

func (dragDrop) Render(c Content, shuffle Shuffler) (map[string]any, error) {
	var p dragPayload
	if err := unmarshal(c.Render, &p); err != nil {
		return nil, err
	}
	out := map[string]any{"draggables": order(p.Draggables, shuffle("draggables", ids(p.Draggables))), "zones": p.Zones,
		"reusable_draggables": p.Reusable}
	if len(p.Background) > 0 && string(p.Background) != "null" {
		out["background_media_ref"] = p.Background
	}
	return out, nil
}
