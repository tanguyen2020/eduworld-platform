// Package docxfixture builds small Word .docx files for the tests and the spike of F028 (TEST FIXTURE, never used
// by production code): paragraphs of text runs (plain, bold), Office Math (OMML) formulas — fractions, roots,
// superscripts, subscripts, matrices, n-ary operators, accents —, embedded images, MathType OLE objects with their
// WMF preview, external hyperlinks and an optional macro part. The content is placeholder Vietnamese math, written
// for the tests (no textbook content).
package docxfixture

import (
	"archive/zip"
	"bytes"
	"fmt"
	"html"
	"strings"
)

// Namespaces of WordprocessingML, Office Math, DrawingML and VML.
const (
	nsW      = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	nsM      = "http://schemas.openxmlformats.org/officeDocument/2006/math"
	nsR      = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	nsWP     = "http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
	nsA      = "http://schemas.openxmlformats.org/drawingml/2006/main"
	nsPic    = "http://schemas.openxmlformats.org/drawingml/2006/picture"
	nsV      = "urn:schemas-microsoft-com:vml"
	nsO      = "urn:schemas-microsoft-com:office:office"
	relImage = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/image"
	relOLE   = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/oleObject"
	relLink  = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink"
)

// PNG1x1 is a valid 1×1 PNG (TEST FIXTURE).
var PNG1x1 = []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01,
	0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x44, 0x41, 0x54, 0x78,
	0x9c, 0x63, 0xf8, 0xcf, 0xc0, 0xf0, 0x1f, 0x00, 0x05, 0x00, 0x01, 0xff, 0x89, 0x99, 0x3d, 0x1d, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45,
	0x4e, 0x44, 0xae, 0x42, 0x60, 0x82}

// wmfStub is the header of a placeable WMF (enough for a preview part; TEST FIXTURE).
var wmfStub = []byte{0xd7, 0xcd, 0xc6, 0x9a, 0, 0, 0, 0, 0, 0, 0x10, 0, 0x10, 0, 0x60, 0, 0, 0, 0, 0, 0x6c, 0x5b}

// Run is one piece of a paragraph.
type Run struct{ xml string }

// T is a plain text run.
func T(s string) Run {
	return Run{`<w:r><w:t xml:space="preserve">` + html.EscapeString(s) + `</w:t></w:r>`}
}

// B is a bold text run.
func B(s string) Run {
	return Run{`<w:r><w:rPr><w:b/></w:rPr><w:t xml:space="preserve">` + html.EscapeString(s) + `</w:t></w:r>`}
}

// Br is a line break inside a paragraph.
func Br() Run { return Run{`<w:r><w:br/></w:r>`} }

// M is an inline Office Math formula of the OMML elements (see the helpers below).
func M(omml ...string) Run { return Run{`<m:oMath>` + strings.Join(omml, "") + `</m:oMath>`} }

// MR is a math run (text inside a formula).
func MR(s string) string { return `<m:r><m:t>` + html.EscapeString(s) + `</m:t></m:r>` }

// MText is a math run in normal (non-italic) text, as Word writes words inside a formula.
func MText(s string) string {
	return `<m:r><m:rPr><m:nor/></m:rPr><w:rPr/><m:t xml:space="preserve">` + html.EscapeString(s) + `</m:t></m:r>`
}

// Frac is a fraction.
func Frac(num, den string) string {
	return `<m:f><m:num>` + num + `</m:num><m:den>` + den + `</m:den></m:f>`
}

// Sqrt is a square root.
func Sqrt(e string) string {
	return `<m:rad><m:radPr><m:degHide m:val="1"/></m:radPr><m:deg/><m:e>` + e + `</m:e></m:rad>`
}

// Root is a root of degree deg.
func Root(deg, e string) string {
	return `<m:rad><m:deg>` + deg + `</m:deg><m:e>` + e + `</m:e></m:rad>`
}

// Sup is a superscript.
func Sup(base, sup string) string {
	return `<m:sSup><m:e>` + base + `</m:e><m:sup>` + sup + `</m:sup></m:sSup>`
}

// Sub is a subscript.
func Sub(base, sub string) string {
	return `<m:sSub><m:e>` + base + `</m:e><m:sub>` + sub + `</m:sub></m:sSub>`
}

// Paren wraps e in parentheses (m:d).
func Paren(e string) string { return `<m:d><m:e>` + e + `</m:e></m:d>` }

// Matrix is a matrix of math runs in parentheses.
func Matrix(rows [][]string) string {
	var b strings.Builder
	b.WriteString(`<m:d><m:e><m:m>`)
	for _, r := range rows {
		b.WriteString(`<m:mr>`)
		for _, c := range r {
			b.WriteString(`<m:e>` + MR(c) + `</m:e>`)
		}
		b.WriteString(`</m:mr>`)
	}
	b.WriteString(`</m:m></m:e></m:d>`)
	return b.String()
}

// Nary is an n-ary operator (∑, ∫, ∏) with limits.
func Nary(chr, sub, sup, e string) string {
	return `<m:nary><m:naryPr><m:chr m:val="` + chr + `"/></m:naryPr><m:sub>` + sub + `</m:sub><m:sup>` + sup + `</m:sup><m:e>` + e + `</m:e></m:nary>`
}

// Vec is a vector arrow accent.
func Vec(e string) string {
	return `<m:acc><m:accPr><m:chr m:val="⃗"/></m:accPr><m:e>` + e + `</m:e></m:acc>`
}

// Bar is an overbar.
func Bar(e string) string {
	return `<m:bar><m:barPr><m:pos m:val="top"/></m:barPr><m:e>` + e + `</m:e></m:bar>`
}

// Func is a function application (sin x).
func Func(name, e string) string {
	return `<m:func><m:fName><m:r><m:rPr><m:sty m:val="p"/></m:rPr><m:t>` + name + `</m:t></m:r></m:fName><m:e>` + e + `</m:e></m:func>`
}

// Lim is a limit (lim under).
func Lim(under, e string) string {
	return `<m:func><m:fName><m:limLow><m:e><m:r><m:rPr><m:sty m:val="p"/></m:rPr><m:t>lim</m:t></m:r></m:e><m:lim>` + under +
		`</m:lim></m:limLow></m:fName><m:e>` + e + `</m:e></m:func>`
}

// EqArr is an equation array (system of equations) in a left brace.
func EqArr(lines ...string) string {
	var b strings.Builder
	b.WriteString(`<m:d><m:dPr><m:begChr m:val="{"/><m:endChr m:val=""/></m:dPr><m:e><m:eqArr>`)
	for _, l := range lines {
		b.WriteString(`<m:e>` + l + `</m:e>`)
	}
	b.WriteString(`</m:eqArr></m:e></m:d>`)
	return b.String()
}

// Para is a paragraph.
type Para struct {
	runs    []Run
	display string // OMML of a display formula paragraph (m:oMathPara)
}

// Doc is a document under construction.
type Doc struct {
	paras  []Para
	rels   []string
	media  map[string][]byte
	embeds map[string][]byte
	nextID int
	Macro  bool // adds word/vbaProject.bin (a .docm part a .docx must not carry)
}

// New returns an empty document.
func New() *Doc { return &Doc{media: map[string][]byte{}, embeds: map[string][]byte{}, nextID: 10} }

// P appends a paragraph of runs.
func (d *Doc) P(runs ...Run) *Doc { d.paras = append(d.paras, Para{runs: runs}); return d }

// Display appends a display formula paragraph.
func (d *Doc) Display(omml ...string) *Doc {
	d.paras = append(d.paras, Para{display: strings.Join(omml, "")})
	return d
}

func (d *Doc) rel(typ, target string, external bool) string {
	d.nextID++
	id := fmt.Sprintf("rId%d", d.nextID)
	mode := ""
	if external {
		mode = ` TargetMode="External"`
	}
	d.rels = append(d.rels, fmt.Sprintf(`<Relationship Id="%s" Type="%s" Target="%s"%s/>`, id, typ, html.EscapeString(target), mode))
	return id
}

// Image is an embedded picture (DrawingML inline) with its alt text (descr); data is stored as word/media/<name>.
func (d *Doc) Image(name, alt string, data []byte) Run {
	d.media[name] = data
	id := d.rel(relImage, "media/"+name, false)
	d.nextID++
	return Run{fmt.Sprintf(`<w:r><w:drawing><wp:inline><wp:extent cx="914400" cy="914400"/><wp:docPr id="%d" name="Picture %d" descr="%s"/>`+
		`<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:pic><pic:nvPicPr><pic:cNvPr id="%d" name="%s"/>`+
		`<pic:cNvPicPr/></pic:nvPicPr><pic:blipFill><a:blip r:embed="%s"/><a:stretch><a:fillRect/></a:stretch></pic:blipFill><pic:spPr>`+
		`<a:xfrm><a:off x="0" y="0"/><a:ext cx="914400" cy="914400"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></pic:spPr>`+
		`</pic:pic></a:graphicData></a:graphic></wp:inline></w:drawing></w:r>`, d.nextID, d.nextID, html.EscapeString(alt), d.nextID, name, id)}
}

// OLEEquation is a MathType (Equation.DSMT4) OLE object with its WMF preview, as legacy Word files carry formulas.
func (d *Doc) OLEEquation(name string) Run {
	d.media[name+".wmf"] = wmfStub
	img := d.rel(relImage, "media/"+name+".wmf", false)
	d.embeds[name+".bin"] = []byte("TEST FIXTURE OLE")
	ole := d.rel(relOLE, "embeddings/"+name+".bin", false)
	d.nextID++
	return Run{fmt.Sprintf(`<w:r><w:object><v:shape id="_x0000_i%d" type="#_x0000_t75" style="width:30pt;height:15pt">`+
		`<v:imagedata r:id="%s" o:title=""/></v:shape><o:OLEObject Type="Embed" ProgID="Equation.DSMT4" ShapeID="_x0000_i%d" `+
		`DrawAspect="Content" ObjectID="_%d" r:id="%s"/></w:object></w:r>`, d.nextID, img, d.nextID, d.nextID, ole)}
}

// Link is an external hyperlink.
func (d *Doc) Link(url, text string) Run {
	id := d.rel(relLink, url, true)
	return Run{`<w:hyperlink r:id="` + id + `"><w:r><w:t xml:space="preserve">` + html.EscapeString(text) + `</w:t></w:r></w:hyperlink>`}
}

// Bytes writes the .docx (a zip of Office Open XML parts).
func (d *Doc) Bytes() []byte {
	var body strings.Builder
	for _, p := range d.paras {
		if p.display != "" {
			body.WriteString(`<w:p><m:oMathPara><m:oMath>` + p.display + `</m:oMath></m:oMathPara></w:p>`)
			continue
		}
		body.WriteString(`<w:p>`)
		for _, r := range p.runs {
			body.WriteString(r.xml)
		}
		body.WriteString(`</w:p>`)
	}
	doc := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<w:document xmlns:w="` + nsW + `" xmlns:m="` + nsM + `" xmlns:r="` + nsR + `" xmlns:wp="` + nsWP + `" xmlns:a="` + nsA +
		`" xmlns:pic="` + nsPic + `" xmlns:v="` + nsV + `" xmlns:o="` + nsO + `"><w:body>` + body.String() + `<w:sectPr/></w:body></w:document>`
	ct := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/><Default Extension="png" ContentType="image/png"/>` +
		`<Default Extension="jpeg" ContentType="image/jpeg"/><Default Extension="gif" ContentType="image/gif"/>` +
		`<Default Extension="wmf" ContentType="image/x-wmf"/>` +
		`<Default Extension="bin" ContentType="application/vnd.openxmlformats-officedocument.oleObject"/>` +
		`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`
	rootRels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`
	docRels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		strings.Join(d.rels, "") + `</Relationships>`
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	put := func(name string, data []byte) {
		w, err := z.Create(name)
		if err != nil {
			panic(err)
		}
		if _, err := w.Write(data); err != nil {
			panic(err)
		}
	}
	put("[Content_Types].xml", []byte(ct))
	put("_rels/.rels", []byte(rootRels))
	put("word/document.xml", []byte(doc))
	put("word/_rels/document.xml.rels", []byte(docRels))
	for _, n := range sortedKeys(d.media) {
		put("word/media/"+n, d.media[n])
	}
	for _, n := range sortedKeys(d.embeds) {
		put("word/embeddings/"+n, d.embeds[n])
	}
	if d.Macro {
		put("word/vbaProject.bin", []byte("TEST FIXTURE MACRO"))
	}
	if err := z.Close(); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func sortedKeys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
