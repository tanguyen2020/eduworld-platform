package docxfixture

import (
	"os"
	"path/filepath"
	"testing"
)

// TestWriteSpikeFixtures writes the spike documents to $EW_DOCX_FIXTURE_DIR (research.md; skipped otherwise).
func TestWriteSpikeFixtures(t *testing.T) {
	dir := os.Getenv("EW_DOCX_FIXTURE_DIR")
	if dir == "" {
		t.Skip("EW_DOCX_FIXTURE_DIR not set")
	}
	write := func(name string, b []byte) {
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range Formulas() {
		write("formula_"+f.Name+".docx", New().P(T("Công thức: "), M(f.OMML)).Bytes())
		write("display_"+f.Name+".docx", New().Display(f.OMML).Bytes())
	}
	all := New()
	for _, f := range Formulas() {
		all.P(T(f.Name+": "), M(f.OMML))
	}
	write("formulas.docx", all.Bytes())
	write("sample_8.docx", Sample(8, "TOAN", "6", "GDPT2018-TOAN", "T6-C1", "T6-C1-LO1"))
	write("sample_50.docx", Sample(50, "TOAN", "6", "GDPT2018-TOAN", "T6-C1", "T6-C1-LO1"))
	d := New()
	d.P(T("Câu 1. Ảnh: "), d.Image("image1.png", "Hình tam giác mẫu", PNG1x1))
	d.P(T("Câu 2. MathType: "), d.OLEEquation("ole1"))
	d.P(T("Câu 3. Liên kết: "), d.Link("https://example.invalid/x", "xem thêm"))
	d.P(T("Câu 4. Ảnh GIF: "), d.Image("image2.gif", "", []byte("GIF89a TEST FIXTURE")))
	d.Macro = true
	write("objects.docx", d.Bytes())
}
