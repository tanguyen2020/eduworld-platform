package docximport

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	dom "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/docximport"
	fx "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/docximport/docxfixture"
)

func golden(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/pandoc-3.6.4-" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func analyse(d dom.Document) []dom.Item {
	h, raws := dom.Split(d)
	out := make([]dom.Item, len(raws))
	n := 0
	for i, r := range raws {
		out[i] = dom.Build(dom.Parse(r), h, dom.DefaultLimits, func() string { n++; return fmt.Sprintf("00000000-0000-4000-8000-%012d", n) })
	}
	return out
}

// F028-AC-01, AC-02, AC-03 (research R2): the recorded AST of the 8-question sample becomes 8 ready questions with
// their formulas, bold answer, hints, explanation and metadata.
func TestGoldenSample(t *testing.T) {
	d, err := toDocument(golden(t, "sample_8"), nil)
	if err != nil {
		t.Fatal(err)
	}
	items := analyse(d)
	if len(items) != 8 {
		t.Fatalf("items %d", len(items))
	}
	for _, it := range items {
		if it.Status() != dom.StatusReady || it.Type != "single_choice" || len(it.Formulas) < 2 {
			t.Fatalf("%s: %s %s %+v %+v", it.Label, it.Status(), it.Type, it.Issues, it.Formulas)
		}
	}
	if items[3].Formulas[0].Latex != `\begin{pmatrix} 1 & 0 \\ 0 & 1 \end{pmatrix}` || items[0].Codes.Topic != "T6-C1" {
		t.Fatalf("matrix %+v codes %+v", items[3].Formulas, items[0].Codes)
	}
}

// Research R3: objects — embedded picture with alt, MathType OLE (with the zip scan), external link, GIF.
func TestGoldenObjects(t *testing.T) {
	d, err := toDocument(golden(t, "objects"), map[string]bool{"word/media/ole1.wmf": true})
	if err != nil {
		t.Fatal(err)
	}
	items := analyse(d)
	want := []string{"", dom.CodeFormulaNeedsRewrite, dom.CodeExternalLinkRemoved, dom.CodeImageTypeNotAllowed}
	for i, it := range items {
		if want[i] == "" {
			if len(it.Images) != 1 || it.Images[0].Path != "word/media/image1.png" || it.Images[0].Alt != "Hình tam giác mẫu" {
				t.Fatalf("image %+v", it.Images)
			}
			continue
		}
		found := false
		for _, is := range it.Issues {
			found = found || is.Code == want[i]
		}
		if !found {
			t.Fatalf("%s: %+v, want %s", it.Label, it.Issues, want[i])
		}
	}
	// every formula of the catalog reaches the model
	d, err = toDocument(golden(t, "formulas"), nil)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, b := range d.Blocks {
		for _, in := range b.Inlines {
			if in.Kind == dom.Math {
				n++
			}
		}
	}
	if n != len(fx.Formulas()) {
		t.Fatalf("formulas %d, want %d", n, len(fx.Formulas()))
	}
	if _, err := toDocument([]byte(`{"blocks":[]}`), nil); !errors.Is(err, port.ErrDocxInvalid) {
		t.Fatalf("no api version: %v", err)
	}
}

func zipOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for n, c := range files {
		w, _ := z.Create(n)
		_, _ = w.Write([]byte(c))
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// F028-AC-04, AC-06: the zip checks before Pandoc runs.
func TestInspect(t *testing.T) {
	d := fx.New()
	d.P(fx.T("Câu 1. "), d.OLEEquation("ole1"))
	ins, err := inspect(d.Bytes(), DefaultZipLimits)
	if err != nil || !ins.olePreviews["word/media/ole1.wmf"] {
		t.Fatalf("ole: %v %+v", err, ins)
	}
	m := fx.New()
	m.P(fx.T("x"))
	m.Macro = true
	if _, err := inspect(m.Bytes(), DefaultZipLimits); !errors.Is(err, port.ErrDocxMacro) {
		t.Fatalf("macro: %v", err)
	}
	macroType := zipOf(t, map[string]string{"word/document.xml": "<w:document/>", "[Content_Types].xml": "application/vnd.ms-word.document.macroEnabled.main+xml"})
	if _, err := inspect(macroType, DefaultZipLimits); !errors.Is(err, port.ErrDocxMacro) {
		t.Fatalf("macro type: %v", err)
	}
	for name, data := range map[string][]byte{"garbage": []byte("not a zip"), "xlsx": zipOf(t, map[string]string{"[Content_Types].xml": "x", "xl/workbook.xml": "x"})} {
		if _, err := inspect(data, DefaultZipLimits); !errors.Is(err, port.ErrDocxInvalid) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if _, err := inspect(fx.New().P(fx.T("x")).Bytes(), ZipLimits{MaxEntries: 2, MaxUncompressed: 1 << 20}); !errors.Is(err, port.ErrDocxTooComplex) {
		t.Fatalf("entries: %v", err)
	}
	big := zipOf(t, map[string]string{"word/document.xml": strings.Repeat("a", 4096), "[Content_Types].xml": "x"})
	if _, err := inspect(big, ZipLimits{MaxEntries: 10, MaxUncompressed: 1024}); !errors.Is(err, port.ErrDocxTooComplex) {
		t.Fatalf("uncompressed: %v", err)
	}
	img := fx.New()
	img.P(img.Image("image1.png", "alt", fx.PNG1x1))
	b, err := Pandoc{}.Media(img.Bytes(), "word/media/image1.png")
	if err != nil || !bytes.Equal(b, fx.PNG1x1) {
		t.Fatalf("media: %v", err)
	}
	if _, err := (Pandoc{}).Media(img.Bytes(), "word/media/none.png"); !errors.Is(err, port.ErrDocxMediaNotFound) {
		t.Fatalf("media missing: %v", err)
	}
}

// TestHelperProcess stands in for Pandoc in TestPandocProcess (TEST FIXTURE: sleeps or fails as asked).
func TestHelperProcess(t *testing.T) {
	switch os.Getenv("EW_DOCX_HELPER") {
	case "sleep":
		time.Sleep(10 * time.Second)
		os.Exit(0)
	case "fail":
		os.Exit(64)
	}
}

// F028-AC-06: timeout, exit status and missing program of the external process.
func TestPandocProcess(t *testing.T) {
	doc := fx.New().P(fx.T("Câu 1. x")).Bytes()
	helper := func(mode string) Pandoc {
		return Pandoc{Command: []string{os.Args[0], "-test.run=TestHelperProcess", "--"}, Env: append(os.Environ(), "EW_DOCX_HELPER="+mode),
			Timeout: 300 * time.Millisecond}
	}
	start := time.Now()
	if _, err := helper("sleep").Convert(context.Background(), doc); !errors.Is(err, port.ErrDocxTimeout) || time.Since(start) > 8*time.Second {
		t.Fatalf("timeout: %v after %s", err, time.Since(start))
	}
	if _, err := helper("fail").Convert(context.Background(), doc); !errors.Is(err, port.ErrDocxInvalid) {
		t.Fatalf("exit: %v", err)
	}
	if _, err := (Pandoc{Command: []string{"ew-pandoc-missing-for-test"}}).Convert(context.Background(), doc); !errors.Is(err, port.ErrDocxNoConverter) {
		t.Fatalf("missing: %v", err)
	}
}

// PandocFromEnv is Pandoc run by EW_PANDOC_CMD (e.g. "docker run --rm -i --network none eduworld/media-tools:local
// pandoc"), or nil.
func pandocFromEnv() *Pandoc {
	c := strings.Fields(os.Getenv("EW_PANDOC_CMD"))
	if len(c) == 0 {
		return nil
	}
	return &Pandoc{Command: c, Env: os.Environ(), Timeout: 2 * time.Minute}
}

// F028-AC-01, AC-03, AC-06 with the real Pandoc 3.6 of eduworld/media-tools (skipped without EW_PANDOC_CMD).
func TestPandocReal(t *testing.T) {
	p := pandocFromEnv()
	if p == nil {
		t.Skip("EW_PANDOC_CMD not set (quickstart §3)")
	}
	ctx := context.Background()
	d, err := p.Convert(ctx, fx.Sample(50, "TOAN", "6", "GDPT2018-TOAN", "T6-C1", "T6-C1-LO1"))
	if err != nil {
		t.Fatal(err)
	}
	items := analyse(d)
	if len(items) != 50 {
		t.Fatalf("items %d", len(items))
	}
	for _, it := range items {
		if it.Status() != dom.StatusReady {
			t.Fatalf("%s: %+v", it.Label, it.Issues)
		}
	}
	all := fx.New()
	for _, f := range fx.Formulas() {
		all.P(fx.T(f.Name+": "), fx.M(f.OMML))
	}
	d, err = p.Convert(ctx, all.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Blocks) != len(fx.Formulas()) {
		t.Fatalf("blocks %d", len(d.Blocks))
	}
	ole := fx.New()
	ole.P(fx.T("Câu 1. "), ole.OLEEquation("ole1"))
	if d, err = p.Convert(ctx, ole.Bytes()); err != nil || len(d.Blocks) != 1 || d.Blocks[0].Inlines[len(d.Blocks[0].Inlines)-1].Kind != dom.OLE {
		t.Fatalf("ole: %v %+v", err, d)
	}
	if _, err := p.Convert(ctx, zipOf(t, map[string]string{"word/document.xml": "<broken", "[Content_Types].xml": "x"})); !errors.Is(err, port.ErrDocxInvalid) {
		t.Fatalf("broken: %v", err)
	}
}
