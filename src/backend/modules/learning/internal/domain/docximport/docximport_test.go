package docximport

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/mathx"
)

// Test helpers: paragraphs of the neutral model (TEST FIXTURE, placeholder Vietnamese math).
func tx(s string) Inline       { return Inline{Kind: Text, Text: s} }
func bd(s string) Inline       { return Inline{Kind: Text, Text: s, Bold: true} }
func mx(l string) Inline       { return Inline{Kind: Math, Latex: l} }
func dm(l string) Inline       { return Inline{Kind: Math, Latex: l, Display: true} }
func para(ins ...Inline) Block { return Block{Inlines: ins} }
func doc(bs ...Block) Document { return Document{Blocks: bs} }
func header() []Block {
	return []Block{para(tx("Đề kiểm tra mẫu")), para(tx("Môn: TOAN")), para(tx("Khối: 6, 7")), para(tx("Chương trình: GDPT2018-TOAN")),
		para(tx("Chủ đề: T6-C1"))}
}

func refs() func() string {
	n := 0
	return func() string { n++; return fmt.Sprintf("00000000-0000-4000-8000-%012d", n) }
}

func analyse(t *testing.T, bs ...Block) []Item {
	t.Helper()
	h, raws := Split(doc(append(header(), bs...)...))
	out := make([]Item, len(raws))
	newRef := refs()
	for i, r := range raws {
		out[i] = Build(Parse(r), h, DefaultLimits, newRef)
	}
	return out
}

func codesOf(it Item) []string {
	var out []string
	for _, i := range it.Issues {
		out = append(out, i.Code)
	}
	return out
}

func asJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// F028-AC-02: the template — header, "Câu n.", options with the bold answer, hints, explanation, LO, difficulty.
func TestBuildSingleChoice(t *testing.T) {
	items := analyse(t,
		para(bd("Câu 1."), tx(" Tính "), mx(`\frac{3}{4}`), tx(" của 8.")),
		para(tx("A. 4")),
		para(bd("B."), tx(" "), mx(`6`)),
		para(tx("C. 2")),
		para(tx("Gợi ý 1: Chia 8 cho 4.")),
		para(tx("Gợi ý 2: Nhân với 3.")),
		para(tx("Giải thích: Lời giải mẫu.")),
		para(tx("Dòng tiếp theo của lời giải.")),
		para(tx("LO: T6-C1-LO1, T6-C1-LO2")),
		para(tx("Độ khó: 2")),
	)
	if len(items) != 1 {
		t.Fatalf("items %d", len(items))
	}
	it := items[0]
	if it.Status() != StatusReady || it.Type != "single_choice" || it.Label != "Câu 1" || it.No != 1 {
		t.Fatalf("item %+v", it)
	}
	want := `{"answer_key":{"correct_option_id":"o_b"},"explanation":{"blocks":[{"text":"Lời giải mẫu.","type":"paragraph"},` +
		`{"text":"Dòng tiếp theo của lời giải.","type":"paragraph"}]},"hints":[{"blocks":[{"text":"Chia 8 cho 4.","type":"paragraph"}]},` +
		`{"blocks":[{"text":"Nhân với 3.","type":"paragraph"}]}],"render_payload":{"options":[{"content":{"blocks":[{"text":"4","type":"paragraph"}]},"id":"o_a"},` +
		`{"content":{"blocks":[{"text":"\\(6\\)","type":"paragraph"}]},"id":"o_b"},{"content":{"blocks":[{"text":"2","type":"paragraph"}]},"id":"o_c"}]},` +
		`"scoring_rule":{"mode":"all_or_nothing"},"stem":{"blocks":[{"text":"Tính \\(\\frac{3}{4}\\) của 8.","type":"paragraph"}]},"type":"single_choice"}`
	if got := asJSON(t, it.Fields); got != want {
		t.Fatalf("fields\n got %s\nwant %s", got, want)
	}
	c := it.Codes
	if c.Subject != "TOAN" || !slices.Equal(c.Grades, []int{6, 7}) || c.Curriculum != "GDPT2018-TOAN" || c.Topic != "T6-C1" ||
		!slices.Equal(c.Objectives, []string{"T6-C1-LO1", "T6-C1-LO2"}) || c.Difficulty != 2 || c.Language != "vi" {
		t.Fatalf("codes %+v", c)
	}
	if !slices.Equal(it.Formulas, []Formula{{Latex: `\frac{3}{4}`}, {Latex: "6"}}) {
		t.Fatalf("formulas %+v", it.Formulas)
	}
	if it.StemPreview != `Tính \frac{3}{4} của 8.` {
		t.Fatalf("preview %q", it.StemPreview)
	}
}

// F028-AC-02: several correct answers ("Đáp án:" line) make a multiple_choice question; the topic is overridden.
func TestBuildMultipleChoiceAndOverrides(t *testing.T) {
	items := analyse(t,
		para(tx("Câu 3: Chọn các số chẵn.")),
		para(tx("A) 2")), para(tx("B) 3")), para(tx("C) 4")), para(tx("D) 5")),
		para(tx("Đáp án: A, C")),
		para(tx("Chủ đề: T6-C2")),
		para(tx("LO: T6-C2-LO1")),
		para(tx("Độ khó: 1")),
		para(tx("Câu 3. Câu trùng số.")),
		para(bd("A. 1")), para(tx("B. 2")),
		para(tx("LO: X")), para(tx("Độ khó: 3")),
	)
	if items[0].Type != "multiple_choice" || items[0].Status() != StatusReady || items[0].Codes.Topic != "T6-C2" {
		t.Fatalf("item 1 %+v", items[0])
	}
	if got := asJSON(t, items[0].Fields["answer_key"]); got != `{"correct_option_ids":["o_a","o_c"]}` {
		t.Fatalf("answer %s", got)
	}
	if items[1].No != 2 || items[1].Label != "Câu 3" || items[1].Codes.Topic != "T6-C1" {
		t.Fatalf("item 2 %+v", items[1])
	}
	h, raws := Split(doc(append(header(), para(tx("Câu 1. a")), para(tx("Câu 1. b")))...))
	_ = h
	if rep := Repeated(raws); rep[2].Code != CodeQuestionNumberRepeat || len(rep) != 1 {
		t.Fatalf("repeated %+v", rep)
	}
}

// F028-AC-05: the errors and warnings of a question.
func TestBuildIssues(t *testing.T) {
	cases := []struct {
		name   string
		blocks []Block
		want   []string
		status string
	}{
		{"no answer", []Block{para(tx("Câu 1. Đề")), para(tx("A. x")), para(tx("B. y")), para(tx("LO: L")), para(tx("Độ khó: 1"))},
			[]string{CodeAnswerMissing}, StatusError},
		{"conflict", []Block{para(tx("Câu 1. Đề")), para(bd("A. x")), para(tx("B. y")), para(tx("Đáp án: B")), para(tx("LO: L")), para(tx("Độ khó: 1"))},
			[]string{CodeAnswerConflict}, StatusError},
		{"unknown option", []Block{para(tx("Câu 1. Đề")), para(tx("A. x")), para(tx("B. y")), para(tx("Đáp án: E")), para(tx("LO: L")), para(tx("Độ khó: 1"))},
			[]string{CodeAnswerUnknownOption}, StatusError},
		{"one option", []Block{para(tx("Câu 1. Đề")), para(bd("A. x")), para(tx("LO: L")), para(tx("Độ khó: 1"))},
			[]string{CodeOptionsTooFew}, StatusError},
		{"multiple needs 3", []Block{para(tx("Câu 1. Đề")), para(bd("A. x")), para(bd("B. y")), para(tx("LO: L")), para(tx("Độ khó: 1"))},
			[]string{CodeOptionsTooFew}, StatusError},
		{"order", []Block{para(tx("Câu 1. Đề")), para(bd("A. x")), para(tx("C. y")), para(tx("LO: L")), para(tx("Độ khó: 1"))},
			[]string{CodeOptionOrder}, StatusError},
		{"not a choice question", []Block{para(tx("Câu 1. Viết đoạn văn.")), para(tx("LO: L")), para(tx("Độ khó: 1"))},
			[]string{CodeUnsupportedFormat}, StatusError},
		{"metadata", []Block{para(tx("Câu 1. Đề")), para(bd("A. x")), para(tx("B. y")), para(tx("Độ khó: 9"))},
			[]string{CodeMetadataMissing, CodeDifficultyInvalid}, StatusError},
		{"empty stem", []Block{para(tx("Câu 1.")), para(bd("A. x")), para(tx("B. y")), para(tx("LO: L")), para(tx("Độ khó: 1"))},
			[]string{CodeStemEmpty}, StatusError},
		{"forbidden formula", []Block{para(tx("Câu 1. "), mx(`\href{http://x}{y}`)), para(bd("A. x")), para(tx("B. y")), para(tx("LO: L")), para(tx("Độ khó: 1"))},
			[]string{CodeMathNotAllowed}, StatusError},
		{"mathtype", []Block{para(tx("Câu 1. "), Inline{Kind: OLE}), para(bd("A. x")), para(tx("B. y")), para(tx("LO: L")), para(tx("Độ khó: 1"))},
			[]string{CodeFormulaNeedsRewrite}, StatusError},
		{"gif", []Block{para(tx("Câu 1. Hình"), Inline{Kind: Image, Path: "word/media/a.gif", Alt: "hình"}), para(bd("A. x")), para(tx("B. y")), para(tx("LO: L")), para(tx("Độ khó: 1"))},
			[]string{CodeImageTypeNotAllowed}, StatusError},
		{"no alt", []Block{para(tx("Câu 1. Hình"), Inline{Kind: Image, Path: "word/media/a.png"}), para(bd("A. x")), para(tx("B. y")), para(tx("LO: L")), para(tx("Độ khó: 1"))},
			[]string{CodeImageAltMissing}, StatusError},
		{"long text", []Block{para(tx("Câu 1. " + strings.Repeat("x", 1001))), para(bd("A. x")), para(tx("B. y")), para(tx("LO: L")), para(tx("Độ khó: 1"))},
			[]string{CodeTextTooLong}, StatusError},
		{"hints", []Block{para(tx("Câu 1. Đề")), para(bd("A. x")), para(tx("B. y")), para(tx("Gợi ý 1: a")), para(tx("Gợi ý 2: a")), para(tx("Gợi ý 3: a")),
			para(tx("Gợi ý 4: a")), para(tx("Gợi ý 5: a")), para(tx("Gợi ý 6: a")), para(tx("LO: L")), para(tx("Độ khó: 1"))},
			[]string{CodeHintsTooMany}, StatusError},
		{"table", []Block{{Inlines: []Inline{tx("Câu 1. Bảng")}, Notes: []Issue{Err(CodeUnsupportedContent, "", "table")}}, para(bd("A. x")), para(tx("B. y")), para(tx("LO: L")), para(tx("Độ khó: 1"))},
			[]string{CodeUnsupportedContent}, StatusError},
		{"link warning", []Block{{Inlines: []Inline{tx("Câu 1. Xem thêm")}, Notes: []Issue{Warn(CodeExternalLinkRemoved, "", "link")}}, para(bd("A. x")), para(tx("B. y")), para(tx("LO: L")), para(tx("Độ khó: 1"))},
			[]string{CodeExternalLinkRemoved}, StatusWarning},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			it := analyse(t, c.blocks...)[0]
			got := codesOf(it)
			for _, w := range c.want {
				if !slices.Contains(got, w) {
					t.Errorf("issues %v, want %s", got, w)
				}
			}
			if it.Status() != c.status {
				t.Errorf("status %s, want %s (%v)", it.Status(), c.status, got)
			}
		})
	}
}

// F028-AC-03: inline formulas in text, display formulas as math blocks, pictures as image blocks with placeholders.
func TestBuildFormulasAndImages(t *testing.T) {
	it := analyse(t,
		para(tx("Câu 1. Giải hệ")),
		para(dm("\\left\\{ \\begin{array}{r}\nx + y = 5 \\\\\nx - y = 1\n\\end{array} \\right.\\ ")),
		para(tx("Hình: "), Inline{Kind: Image, Path: "word/media/image1.png", Alt: "Hình tam giác"}, tx(" bên.")),
		para(bd("A. "), mx(`x = 3`)), para(tx("B. "), mx(`x = 2`)),
		para(tx("LO: L")), para(tx("Độ khó: 3")),
	)[0]
	if it.Status() != StatusReady {
		t.Fatalf("issues %+v", it.Issues)
	}
	stem := asJSON(t, it.Fields["stem"])
	want := `{"blocks":[{"text":"Giải hệ","type":"paragraph"},{"latex":"\\left\\{ \\begin{array}{r} x + y = 5 \\\\ x - y = 1 \\end{array} \\right.","type":"math"},` +
		`{"text":"Hình:","type":"paragraph"},{"alt":"Hình tam giác","media_ref":"00000000-0000-4000-8000-000000000001","type":"image"},{"text":"bên.","type":"paragraph"}]}`
	if stem != want {
		t.Fatalf("stem\n got %s\nwant %s", stem, want)
	}
	if len(it.Images) != 1 || it.Images[0].MIME != "image/png" || it.Images[0].Path != "word/media/image1.png" || !it.Formulas[0].Display {
		t.Fatalf("images %+v formulas %+v", it.Images, it.Formulas)
	}
}

// Research R2: the LaTeX Pandoc 3.6.4 writes for the OMML catalog passes the shared allowlist after normalization.
func TestPandocLatexAllowlist(t *testing.T) {
	for name, l := range map[string]string{
		"fraction": `\frac{3}{4}`, "mixed_fraction": `2\frac{1}{3}`, "nested_fraction": `\frac{1}{1 + \frac{1}{x}}`,
		"sqrt": `\sqrt{x + 1}`, "cube_root": `\sqrt[3]{8}`, "superscript": `x^{2} + y^{3}`, "subscript": `a_{n} = a_{1} + (n - 1)d`,
		"chem_subscript": `H_{2}O`, "matrix_2x2": "\\begin{pmatrix}\n1 & 2 \\\\\n3 & 4\n\\end{pmatrix}", "sum": `\sum_{i = 1}^{n}i`,
		"integral": `\int_{0}^{1}{x^{2}dx}`, "vector": `\overrightarrow{AB}`, "overbar": `\overline{abc}`, "sin": `\sin x`,
		"limit":     `\lim_{x \rightarrow 0}\frac{\sin x}{x}`,
		"system":    "\\left\\{ \\begin{array}{r}\nx + y = 5 \\\\\nx - y = 1\n\\end{array} \\right.\\ ",
		"relations": `a \leq b,\ c \geq d,\ e \neq f,\ x \in {\mathbb{R}},\ A \subset B`, "greek_degree": `\alpha + \beta = 90{^\circ}`,
		"pm_times_div": `\pm 3 \times 4 \div 2`, "text_in_math": `v = \frac{\text{quãng đường}}{\text{thời gian}}`,
		"parentheses": `\left( \frac{a}{b} \right)^{2}`, "percent": `25\%`,
	} {
		if err := mathx.ValidateLatex(NormalizeLatex(l), mathx.DefaultLimits); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	if got := NormalizeLatex("\\left\\{ x \\right.\\ "); got != `\left\{ x \right.` {
		t.Errorf("normalize %q", got)
	}
	if got := NormalizeLatex(`a \\`); got != `a \\` {
		t.Errorf("row break kept: %q", got)
	}
}

func TestSplitHeaderVariants(t *testing.T) {
	h, qs := Split(doc(para(tx("Môn học: TOAN")), para(tx("Lớp: 8")), para(tx("Phiên bản chương trình: 2")), para(tx("Chương trình: C")),
		para(tx("Chủ đề: T")), para(tx("Ngôn ngữ: en")), para(tx("CÂU 1) Đề"))))
	if h.Subject != "TOAN" || h.Grades != "8" || h.CurriculumVersion != "2" || h.Curriculum != "C" || h.Topic != "T" || h.Language != "en" {
		t.Fatalf("header %+v", h)
	}
	if len(qs) != 1 || plain(qs[0].Blocks[0]) != "Đề" {
		t.Fatalf("questions %+v", qs)
	}
	// a label split across bold runs ("Câu", " ", "1.") is removed exactly
	_, qs = Split(doc(para(bd("Câu"), bd(" "), bd("1."), tx(" Tính "), mx("x"))))
	if got := qs[0].Blocks[0].Inlines; len(got) != 2 || got[0].Text != "Tính " {
		t.Fatalf("strip %+v", got)
	}
	if StatusOf(nil) != StatusReady || !Importable(StatusDraftFailed) || Importable(StatusCreated) || Importable(StatusError) {
		t.Fatal("statuses")
	}
}
