package docximport

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Header is the metadata block before the first question (CONTENT_PRODUCTION_PLAN §8 step 1).
type Header struct {
	Subject           string
	Grades            string
	Curriculum        string
	CurriculumVersion string
	Topic             string
	Language          string
}

// Raw is the paragraphs of one question: Label "Câu n", No its position (1-based), Blocks without the label.
type Raw struct {
	No     int
	Label  string
	Number int
	Blocks []Block
}

var (
	reQuestion = regexp.MustCompile(`^câu\s*(\d{1,4})\s*[.:)]\s*`)
	reOption   = regexp.MustCompile(`^([a-h])\s*[.)]\s*`)
	reAnswer   = regexp.MustCompile(`^đáp\s*án\s*(đúng)?\s*:\s*`)
	reHint     = regexp.MustCompile(`^gợi\s*ý\s*(\d{1,2})?\s*:\s*`)
	reExplain  = regexp.MustCompile(`^(giải\s*thích|lời\s*giải)\s*:\s*`)
	reLO       = regexp.MustCompile(`^lo\s*:\s*`)
	reDiff     = regexp.MustCompile(`^độ\s*khó\s*:\s*`)
	reTopic    = regexp.MustCompile(`^chủ\s*đề\s*:\s*`)
	reSubject  = regexp.MustCompile(`^môn(\s*học)?\s*:\s*`)
	reGrade    = regexp.MustCompile(`^(khối|lớp)\s*:\s*`)
	reCurVer   = regexp.MustCompile(`^phiên\s*bản\s*chương\s*trình\s*:\s*`)
	reCur      = regexp.MustCompile(`^chương\s*trình\s*:\s*`)
	reLang     = regexp.MustCompile(`^ngôn\s*ngữ\s*:\s*`)
)

// lead is the text of a block before its first non-text piece (where labels are written).
func lead(b Block) string {
	var s strings.Builder
	for _, in := range b.Inlines {
		if in.Kind != Text {
			break
		}
		s.WriteString(in.Text)
	}
	return strings.TrimLeftFunc(s.String(), unicode.IsSpace)
}

// match returns the length in runes of the label re at the start of the lead of b (lower case), or -1.
func match(re *regexp.Regexp, b Block) (int, []string) {
	l := strings.ToLower(lead(b))
	m := re.FindStringSubmatch(l)
	if m == nil {
		return -1, nil
	}
	return utf8.RuneCountInString(m[0]), m
}

// strip removes the leading spaces, then n runes of text (the label matched on lead), then the spaces after it.
func strip(b Block, n int) Block {
	out := Block{Notes: b.Notes}
	started, done := false, false
	for _, in := range b.Inlines {
		if done || in.Kind != Text {
			done = true
			out.Inlines = append(out.Inlines, in)
			continue
		}
		t := in.Text
		if !started {
			if t = strings.TrimLeftFunc(t, unicode.IsSpace); t == "" {
				continue
			}
			started = true
		}
		rs := []rune(t)
		if len(rs) <= n {
			n -= len(rs)
			continue
		}
		in.Text = strings.TrimLeftFunc(string(rs[n:]), unicode.IsSpace)
		n, done = 0, true
		if in.Text != "" {
			out.Inlines = append(out.Inlines, in)
		}
	}
	// the text right after the label starts without spaces
	for i := range out.Inlines {
		if out.Inlines[i].Kind != Text {
			break
		}
		if out.Inlines[i].Text = strings.TrimLeftFunc(out.Inlines[i].Text, unicode.IsSpace); out.Inlines[i].Text != "" {
			break
		}
	}
	return out
}

// plain is the text of a block with every formula as its LaTeX (label values: codes, numbers, letters).
func plain(b Block) string {
	var s strings.Builder
	for _, in := range b.Inlines {
		switch in.Kind {
		case Text:
			s.WriteString(in.Text)
		case Math:
			s.WriteString(in.Latex)
		case Break:
			s.WriteString(" ")
		}
	}
	return strings.TrimSpace(s.String())
}

// Split reads the header and cuts the document into questions: every paragraph starting with "Câu n." opens a
// question; paragraphs before the first one are the header (unknown lines, such as a title, are ignored).
func Split(d Document) (Header, []Raw) {
	var h Header
	var qs []Raw
	for _, b := range d.Blocks {
		if n, m := match(reQuestion, b); n >= 0 {
			num, _ := strconv.Atoi(m[1])
			qs = append(qs, Raw{No: len(qs) + 1, Label: "Câu " + m[1], Number: num})
			if rest := strip(b, n); len(rest.Inlines) > 0 || len(rest.Notes) > 0 {
				qs[len(qs)-1].Blocks = append(qs[len(qs)-1].Blocks, rest)
			}
			continue
		}
		if len(qs) > 0 {
			qs[len(qs)-1].Blocks = append(qs[len(qs)-1].Blocks, b)
			continue
		}
		val := func(re *regexp.Regexp) (string, bool) {
			n, _ := match(re, b)
			if n < 0 {
				return "", false
			}
			return plain(strip(b, n)), true
		}
		// "Phiên bản chương trình:" before "Chương trình:" (the second is a suffix of the first).
		for _, f := range []struct {
			re  *regexp.Regexp
			dst *string
		}{{reSubject, &h.Subject}, {reGrade, &h.Grades}, {reCurVer, &h.CurriculumVersion}, {reCur, &h.Curriculum},
			{reTopic, &h.Topic}, {reLang, &h.Language}} {
			if v, ok := val(f.re); ok {
				*f.dst = v
				break
			}
		}
	}
	return h, qs
}

// Option is one choice of a question.
type Option struct {
	Letter byte // 'A'…'H'
	Bold   bool // the letter is in bold (marks the correct answer)
	Blocks []Block
}

// Parsed is a question read with the template.
type Parsed struct {
	Raw
	Stem        []Block
	Options     []Option
	Answer      []byte // letters of the "Đáp án:" line; nil when absent
	AnswerBad   []string
	Hints       [][]Block
	Explanation []Block
	Objectives  []string
	Difficulty  string
	Topic       string
	Notes       []Issue
}

// letterBold reports whether the first non-space rune of a block is in bold.
func letterBold(b Block) bool {
	for _, in := range b.Inlines {
		if in.Kind != Text {
			return false
		}
		if strings.TrimSpace(in.Text) != "" {
			return in.Bold
		}
	}
	return false
}

// Parse reads the parts of a question: stem, options, answer line, hints, explanation and per-question metadata.
// A paragraph without a label continues the part being written (stem, the last option, hint or explanation).
func Parse(r Raw) Parsed {
	p := Parsed{Raw: r}
	type part int
	const (
		inStem part = iota
		inOption
		inHint
		inExplain
	)
	cur := inStem
	for _, b := range r.Blocks {
		p.Notes = append(p.Notes, b.Notes...)
		if n, m := match(reOption, b); n >= 0 {
			p.Options = append(p.Options, Option{Letter: strings.ToUpper(m[1])[0], Bold: letterBold(b), Blocks: nonEmpty(strip(b, n))})
			cur = inOption
			continue
		}
		if n, _ := match(reAnswer, b); n >= 0 {
			p.Answer = []byte{}
			for _, tok := range strings.FieldsFunc(plain(strip(b, n)), func(r rune) bool { return r == ',' || r == ';' || unicode.IsSpace(r) }) {
				t := strings.ToUpper(strings.TrimRight(tok, "."))
				if len(t) == 1 && t[0] >= 'A' && t[0] <= 'H' {
					p.Answer = append(p.Answer, t[0])
				} else {
					p.AnswerBad = append(p.AnswerBad, t)
				}
			}
			continue
		}
		if n, _ := match(reHint, b); n >= 0 {
			p.Hints = append(p.Hints, nonEmpty(strip(b, n)))
			cur = inHint
			continue
		}
		if n, _ := match(reExplain, b); n >= 0 {
			p.Explanation = append(p.Explanation, nonEmpty(strip(b, n))...)
			cur = inExplain
			continue
		}
		if n, _ := match(reLO, b); n >= 0 {
			for _, c := range strings.FieldsFunc(plain(strip(b, n)), func(r rune) bool { return r == ',' || r == ';' }) {
				if c = strings.TrimSpace(c); c != "" {
					p.Objectives = append(p.Objectives, c)
				}
			}
			continue
		}
		if n, _ := match(reDiff, b); n >= 0 {
			p.Difficulty = plain(strip(b, n))
			continue
		}
		if n, _ := match(reTopic, b); n >= 0 {
			p.Topic = plain(strip(b, n))
			continue
		}
		switch cur {
		case inStem:
			p.Stem = append(p.Stem, b)
		case inOption:
			o := &p.Options[len(p.Options)-1]
			o.Blocks = append(o.Blocks, b)
		case inHint:
			h := &p.Hints[len(p.Hints)-1]
			*h = append(*h, b)
		case inExplain:
			p.Explanation = append(p.Explanation, b)
		}
	}
	return p
}

func nonEmpty(b Block) []Block {
	if len(b.Inlines) == 0 {
		return nil
	}
	return []Block{b}
}
