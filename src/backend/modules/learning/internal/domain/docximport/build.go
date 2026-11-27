package docximport

import (
	"errors"
	"fmt"
	"path"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/mathx"
)

// Formula is one distinct formula of a question: its normalized LaTeX and whether it stands on its own line.
type Formula struct {
	Latex   string `json:"latex"`
	Display bool   `json:"display"`
}

// ImageRef is one embedded picture of a question: Ref is the placeholder media_ref of the content, replaced by the
// id of the media asset when the draft is created.
type ImageRef struct {
	Ref  string `json:"ref"`
	Path string `json:"path"`
	Alt  string `json:"alt"`
	MIME string `json:"mime"`
}

// Codes are the curriculum codes of a question (header, overridden per question).
type Codes struct {
	Subject           string   `json:"subject_code"`
	Grades            []int    `json:"grade_levels"`
	Curriculum        string   `json:"curriculum_code"`
	CurriculumVersion int      `json:"curriculum_version,omitempty"`
	Topic             string   `json:"topic_code"`
	Objectives        []string `json:"objective_codes"`
	Difficulty        int      `json:"difficulty"`
	Language          string   `json:"content_language"`
}

// Item is the analysis of one question: what the report shows and what a draft is created from.
type Item struct {
	No          int
	Label       string
	Type        string // single_choice, multiple_choice or "" (not built)
	StemPreview string
	Issues      []Issue
	Formulas    []Formula
	Images      []ImageRef
	// Fields are the authoring fields of createQuestion (type, stem, render_payload, answer_key, scoring_rule,
	// hints, explanation) as JSON values.
	Fields map[string]any
	Codes  Codes
}

// Status is the analysis status of the item.
func (it Item) Status() string { return StatusOf(it.Issues) }

// mimeOf is the MIME type of an embedded picture accepted by the media kind image (CONTENT_MODEL §7), "" otherwise.
func mimeOf(p string) string {
	switch strings.ToLower(path.Ext(p)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	}
	return ""
}

// builder turns blocks into rich contents and collects the formulas, images and issues of one question.
type builder struct {
	lim      Limits
	newRef   func() string
	issues   []Issue
	formulas []Formula
	seen     map[string]bool
	images   []ImageRef
}

func (b *builder) add(i Issue) {
	for _, x := range b.issues {
		if x.Code == i.Code && x.Field == i.Field {
			return
		}
	}
	b.issues = append(b.issues, i)
}

func (b *builder) formula(latex string, display bool, field string) (string, bool) {
	l := NormalizeLatex(latex)
	if err := mathx.ValidateLatex(l, mathx.DefaultLimits); err != nil {
		var me mathx.Error
		msg := "the formula is not allowed"
		if errors.As(err, &me) {
			msg = me.Reason
		}
		b.add(Err(CodeMathNotAllowed, field, msg))
		return "", false
	}
	if !b.seen[l] {
		b.seen[l] = true
		b.formulas = append(b.formulas, Formula{Latex: l, Display: display})
	}
	return l, true
}

// rich builds the richContent ({blocks}) of paragraphs: text with inline formulas as \( … \), display formulas as
// math blocks, pictures as image blocks; nil when empty.
func (b *builder) rich(blocks []Block, field string) map[string]any {
	var out []any
	for _, blk := range blocks {
		var text strings.Builder
		flush := func() {
			t := strings.TrimSpace(text.String())
			text.Reset()
			if t == "" {
				return
			}
			if utf8.RuneCountInString(t) > b.lim.TextMax {
				b.add(Err(CodeTextTooLong, field, fmt.Sprintf("a paragraph is longer than %d characters", b.lim.TextMax)))
			}
			out = append(out, map[string]any{"type": "paragraph", "text": t})
		}
		for _, in := range blk.Inlines {
			switch in.Kind {
			case Text:
				text.WriteString(in.Text)
			case Break:
				text.WriteString("\n")
			case Math:
				l, ok := b.formula(in.Latex, in.Display, field)
				if !ok {
					continue
				}
				if !in.Display {
					text.WriteString(`\(` + l + `\)`)
					continue
				}
				flush()
				if utf8.RuneCountInString(l) > b.lim.LatexMax {
					b.add(Err(CodeTextTooLong, field, fmt.Sprintf("a display formula is longer than %d characters", b.lim.LatexMax)))
				}
				out = append(out, map[string]any{"type": "math", "latex": l})
			case OLE:
				b.add(Err(CodeFormulaNeedsRewrite, field, "a MathType/Equation object must be converted to an Office Math formula"))
			case Image:
				mime := mimeOf(in.Path)
				alt := strings.TrimSpace(in.Alt)
				switch {
				case mime == "":
					b.add(Err(CodeImageTypeNotAllowed, field, "pictures must be PNG, JPEG or WebP"))
					continue
				case alt == "":
					b.add(Err(CodeImageAltMissing, field, "a picture has no alternative text"))
					continue
				case utf8.RuneCountInString(alt) > b.lim.AltMax:
					alt = string([]rune(alt)[:b.lim.AltMax])
				}
				flush()
				ref := b.newRef()
				b.images = append(b.images, ImageRef{Ref: ref, Path: in.Path, Alt: alt, MIME: mime})
				out = append(out, map[string]any{"type": "image", "media_ref": ref, "alt": alt})
			}
		}
		flush()
	}
	if len(out) == 0 {
		return nil
	}
	return map[string]any{"blocks": out}
}

// Build analyses a parsed question with the header h: content of a single_choice (one correct option) or
// multiple_choice question, formulas through the mathx allowlist, pictures, metadata, and every issue found.
// newRef returns a fresh placeholder media_ref (a UUID string).
func Build(p Parsed, h Header, lim Limits, newRef func() string) Item {
	b := &builder{lim: lim, newRef: newRef, seen: map[string]bool{}}
	for _, n := range p.Notes {
		b.add(n)
	}
	it := Item{No: p.No, Label: p.Label}
	stem := b.rich(p.Stem, "stem")
	if stem == nil {
		b.add(Err(CodeStemEmpty, "stem", "the question has no stem"))
	}
	it.StemPreview = preview(p.Stem, lim.PreviewMax)

	// options and answer
	var bold []byte
	options := make([]any, 0, len(p.Options))
	for i, o := range p.Options {
		if o.Letter != byte('A'+i) {
			b.add(Err(CodeOptionOrder, "options", "options must be labelled A, B, C… in order"))
		}
		if o.Bold {
			bold = append(bold, o.Letter)
		}
		field := "options/" + string(o.Letter)
		c := b.rich(o.Blocks, field)
		if c == nil {
			b.add(Err(CodeOptionEmpty, field, "option "+string(o.Letter)+" is empty"))
			continue
		}
		options = append(options, map[string]any{"id": optionID(o.Letter), "content": c})
	}
	correct := bold
	if p.Answer != nil {
		if len(bold) > 0 && !sameLetters(bold, p.Answer) {
			b.add(Err(CodeAnswerConflict, "answer", "the bold options differ from the \"Đáp án\" line"))
		}
		correct = p.Answer
	}
	if len(p.AnswerBad) > 0 {
		b.add(Err(CodeAnswerUnknownOption, "answer", "the \"Đáp án\" line names no option: "+truncate(strings.Join(p.AnswerBad, ", "), 64)))
	}
	correct = dedupe(correct)
	for _, l := range correct {
		if !slices.ContainsFunc(p.Options, func(o Option) bool { return o.Letter == l }) {
			b.add(Err(CodeAnswerUnknownOption, "answer", "the answer names option "+string(l)+" which does not exist"))
		}
	}
	switch {
	case len(p.Options) == 0 && p.Answer == nil:
		b.add(Err(CodeUnsupportedFormat, "options", "only single and multiple choice questions (options A., B., …) can be imported"))
	case len(correct) == 0:
		b.add(Err(CodeAnswerMissing, "answer", "mark the correct option in bold or add a \"Đáp án:\" line"))
	case len(correct) == 1:
		it.Type = "single_choice"
		if len(p.Options) < 2 {
			b.add(Err(CodeOptionsTooFew, "options", "a single choice question needs at least 2 options"))
		} else if len(p.Options) > 8 {
			b.add(Err(CodeOptionsTooMany, "options", "a single choice question has at most 8 options"))
		}
	default:
		it.Type = "multiple_choice"
		if len(p.Options) < 3 {
			b.add(Err(CodeOptionsTooFew, "options", "a multiple choice question needs at least 3 options"))
		} else if len(p.Options) > 10 {
			b.add(Err(CodeOptionsTooMany, "options", "a multiple choice question has at most 10 options"))
		}
	}

	// hints and explanation
	if len(p.Hints) > lim.MaxHints {
		b.add(Err(CodeHintsTooMany, "hints", fmt.Sprintf("at most %d hints", lim.MaxHints)))
	}
	var hints []any
	for i, h := range p.Hints {
		if c := b.rich(h, "hints/"+strconv.Itoa(i+1)); c != nil {
			hints = append(hints, c)
		}
	}
	expl := b.rich(p.Explanation, "explanation")

	// metadata
	it.Codes = codes(b, p, h)

	if len(b.formulas) > lim.MaxFormulas {
		b.add(Err(CodeFormulasTooMany, "math_assets", fmt.Sprintf("at most %d distinct formulas", lim.MaxFormulas)))
	}
	if len(b.images) > lim.MaxImages {
		b.add(Err(CodeImagesTooMany, "media_refs", fmt.Sprintf("at most %d pictures", lim.MaxImages)))
	}
	// the item keeps at most the limits (the question is in error beyond them)
	if len(b.formulas) > lim.MaxFormulas {
		b.formulas = b.formulas[:lim.MaxFormulas]
	}
	if len(b.images) > lim.MaxImages {
		b.images = b.images[:lim.MaxImages]
	}
	if len(b.issues) > MaxIssues {
		b.issues = b.issues[:MaxIssues]
	}
	it.Formulas, it.Images, it.Issues = b.formulas, b.images, b.issues
	if it.Type != "" {
		f := map[string]any{"type": it.Type, "scoring_rule": map[string]any{"mode": "all_or_nothing"},
			"render_payload": map[string]any{"options": options}}
		if stem != nil {
			f["stem"] = stem
		}
		ids := make([]string, len(correct))
		for i, l := range correct {
			ids[i] = optionID(l)
		}
		if it.Type == "single_choice" && len(ids) == 1 {
			f["answer_key"] = map[string]any{"correct_option_id": ids[0]}
		} else {
			f["answer_key"] = map[string]any{"correct_option_ids": ids}
		}
		if len(hints) > 0 {
			f["hints"] = hints
		}
		if expl != nil {
			f["explanation"] = expl
		}
		it.Fields = f
	}
	return it
}

func codes(b *builder, p Parsed, h Header) Codes {
	c := Codes{Subject: strings.TrimSpace(h.Subject), Curriculum: strings.TrimSpace(h.Curriculum), Topic: strings.TrimSpace(h.Topic),
		Objectives: p.Objectives, Language: strings.ToLower(strings.TrimSpace(h.Language))}
	if t := strings.TrimSpace(p.Topic); t != "" {
		c.Topic = t
	}
	missing := func(field, what string) { b.add(Err(CodeMetadataMissing, field, what+" is missing")) }
	if c.Subject == "" {
		missing("subject_code", "\"Môn:\" (subject code)")
	}
	if c.Curriculum == "" {
		missing("curriculum_code", "\"Chương trình:\" (curriculum code)")
	}
	if c.Topic == "" {
		missing("topic_code", "\"Chủ đề:\" (topic code)")
	}
	if len(c.Objectives) == 0 {
		missing("objective_codes", "\"LO:\" (learning objective codes)")
	}
	for _, g := range strings.FieldsFunc(h.Grades, func(r rune) bool { return r == ',' || r == ';' || r == ' ' }) {
		n, err := strconv.Atoi(g)
		if err != nil || n < 1 || n > 9 {
			b.add(Err(CodeMetadataMissing, "grade_levels", "\"Khối:\" must list grades 1–9"))
			c.Grades = nil
			break
		}
		if !slices.Contains(c.Grades, n) {
			c.Grades = append(c.Grades, n)
		}
	}
	if len(c.Grades) == 0 && !slices.ContainsFunc(b.issues, func(i Issue) bool { return i.Field == "grade_levels" }) {
		missing("grade_levels", "\"Khối:\" (grades)")
	}
	if v := strings.TrimSpace(h.CurriculumVersion); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			b.add(Err(CodeMetadataMissing, "curriculum_version", "\"Phiên bản chương trình:\" must be a version number"))
		}
		c.CurriculumVersion = n
	}
	switch c.Language {
	case "", "vi", "tiếng việt":
		c.Language = "vi"
	case "en", "tiếng anh":
		c.Language = "en"
	default:
		b.add(Err(CodeMetadataMissing, "content_language", "\"Ngôn ngữ:\" must be vi or en"))
	}
	if d := strings.TrimSpace(p.Difficulty); d == "" {
		missing("difficulty", "\"Độ khó:\" (1–5)")
	} else {
		n, err := strconv.Atoi(d)
		if err != nil || n < 1 || n > 5 {
			b.add(Err(CodeDifficultyInvalid, "difficulty", "\"Độ khó:\" must be 1–5"))
		}
		c.Difficulty = n
	}
	return c
}

// Repeated returns a warning for every question whose "Câu n" number was already used (both are kept).
func Repeated(qs []Raw) map[int]Issue {
	out := map[int]Issue{}
	seen := map[int]bool{}
	for _, q := range qs {
		if seen[q.Number] {
			out[q.No] = Warn(CodeQuestionNumberRepeat, "", q.Label+" appears more than once")
		}
		seen[q.Number] = true
	}
	return out
}

func optionID(l byte) string { return "o_" + strings.ToLower(string(l)) }

func sameLetters(a, b []byte) bool {
	x, y := dedupe(a), dedupe(b)
	slices.Sort(x)
	slices.Sort(y)
	return slices.Equal(x, y)
}

func dedupe(ls []byte) []byte {
	var out []byte
	for _, l := range ls {
		if !slices.Contains(out, l) {
			out = append(out, l)
		}
	}
	return out
}

func preview(blocks []Block, max int) string {
	var parts []string
	for _, b := range blocks {
		if t := plain(b); t != "" {
			parts = append(parts, t)
		}
	}
	return truncate(strings.Join(parts, " "), max)
}

func truncate(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max])
}
