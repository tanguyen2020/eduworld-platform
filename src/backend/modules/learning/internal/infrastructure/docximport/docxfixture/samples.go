package docxfixture

// Formula is one OMML sample of the spike with the LaTeX Pandoc 3.6 is expected to produce (research.md R2).
type Formula struct {
	Name string
	OMML string
}

// Formulas is the OMML catalog of the spike: the constructs of grade 1–9 math, physics and chemistry (TEST FIXTURE).
func Formulas() []Formula {
	return []Formula{
		{"fraction", Frac(MR("3"), MR("4"))},
		{"mixed_fraction", MR("2") + Frac(MR("1"), MR("3"))},
		{"nested_fraction", Frac(MR("1"), MR("1+")+Frac(MR("1"), MR("x")))},
		{"sqrt", Sqrt(MR("x+1"))},
		{"cube_root", Root(MR("3"), MR("8"))},
		{"superscript", Sup(MR("x"), MR("2")) + MR("+") + Sup(MR("y"), MR("3"))},
		{"subscript", Sub(MR("a"), MR("n")) + MR("=") + Sub(MR("a"), MR("1")) + MR("+(n−1)d")},
		{"chem_subscript", Sub(MR("H"), MR("2")) + MR("O")},
		{"matrix_2x2", Matrix([][]string{{"1", "2"}, {"3", "4"}})},
		{"sum", Nary("∑", MR("i=1"), MR("n"), MR("i"))},
		{"integral", Nary("∫", MR("0"), MR("1"), Sup(MR("x"), MR("2"))+MR("dx"))},
		{"vector", Vec(MR("AB"))},
		{"overbar", Bar(MR("abc"))},
		{"sin", Func("sin", MR("x"))},
		{"limit", Lim(MR("x→0"), Frac(Func("sin", MR("x")), MR("x")))},
		{"system", EqArr(MR("x+y=5"), MR("x−y=1"))},
		{"relations", MR("a≤b, c≥d, e≠f, x∈ℝ, A⊂B")},
		{"greek_degree", MR("α+β=90°")},
		{"pm_times_div", MR("±3×4÷2")},
		{"text_in_math", MR("v=") + Frac(MText("quãng đường"), MText("thời gian"))},
		{"parentheses", Paren(Frac(MR("a"), MR("b"))) + Sup(MR(""), MR("2"))},
		{"percent", MR("25%")},
	}
}

// Header writes the metadata block of the template (CONTENT_PRODUCTION_PLAN §8 step 1, research.md R4).
func Header(d *Doc, subject, grade, curriculum, topic string) {
	d.P(T("Môn: " + subject))
	d.P(T("Khối: " + grade))
	d.P(T("Chương trình: " + curriculum))
	d.P(T("Chủ đề: " + topic))
}

// Sample is a template document of n single-choice questions with formulas (fraction, root, power, matrix by turn),
// the correct option in bold, two hints, an explanation, an LO and a difficulty per question (TEST FIXTURE).
func Sample(n int, subject, grade, curriculum, topic, lo string) []byte {
	d := New()
	Header(d, subject, grade, curriculum, topic)
	forms := []string{Frac(MR("3"), MR("4")), Sqrt(MR("16")), Sup(MR("2"), MR("3")), Matrix([][]string{{"1", "0"}, {"0", "1"}})}
	for i := 1; i <= n; i++ {
		f := forms[(i-1)%len(forms)]
		d.P(B("Câu "+itoa(i)+"."), T(" Tính giá trị của biểu thức "), M(f), T(" (câu mẫu số "+itoa(i)+")."))
		d.P(T("A. "), M(MR(itoa(i))))
		d.P(B("B. "), B("Đáp án đúng "), M(MR(itoa(i+1))))
		d.P(T("C. Phương án nhiễu thứ nhất"))
		d.P(T("D. Phương án nhiễu thứ hai"))
		d.P(T("Gợi ý 1: Đọc kỹ đề bài."))
		d.P(T("Gợi ý 2: Viết lại biểu thức "), M(f), T("."))
		d.P(T("Giải thích: Lời giải mẫu cho câu " + itoa(i) + "."))
		d.P(T("LO: " + lo))
		d.P(T("Độ khó: " + itoa(1+(i-1)%5)))
	}
	return d.Bytes()
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}
