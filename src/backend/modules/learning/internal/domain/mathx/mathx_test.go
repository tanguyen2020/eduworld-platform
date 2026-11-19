package mathx

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// F027-AC-12 (TC-MATH-002): the LaTeX allowlist.
func TestValidateLatex(t *testing.T) {
	ok := []string{`\frac{3}{4}`, `x^{2}+\sqrt{y}`, `\ce{H2O + CO2 -> H2CO3}`, `\pu{3 m/s}`, `\left( \frac{a}{b} \right)`,
		`\begin{pmatrix} 1 & 2 \\ 3 & 4 \end{pmatrix}`, `\color{red}{x}`, `a \neq b \newline c`, `\{1,2\}`, `\text{Tốc độ} = 5`}
	for _, l := range ok {
		if err := ValidateLatex(l, DefaultLimits); err != nil {
			t.Errorf("%q refused: %v", l, err)
		}
	}
	bad := map[string]string{
		`\href{http://x}{click}`:               "href",
		`\url{http://x}`:                       "url",
		`\htmlClass{a}{x}`:                     "htmlClass",
		`\class{a}{x}`:                         "class",
		`\style{color:red}{x}`:                 "style",
		`\require{action}`:                     "require",
		`\unicode{x1F600}`:                     "unicode",
		`\def\x{1}`:                            "def",
		`\newcommand{\x}{1}`:                   "newcommand",
		`\renewcommand{\x}{1}`:                 "renewcommand",
		`\let\a\b`:                             "let",
		`\begin{tikzpicture}\end{tikzpicture}`: "environment",
		`\color{#ff0000}{x}`:                   "colors",
		`\colorbox{red}{x}`:                    "colorbox",
		`{x`:                                   "unbalanced",
		`x}`:                                   "unbalanced",
		`\left( x`:                             "left",
		`x % comment`:                          "comments",
		"":                                     "empty",
	}
	for l, want := range bad {
		err := ValidateLatex(l, DefaultLimits)
		var e Error
		if !errors.As(err, &e) || !strings.Contains(e.Reason, want) {
			t.Errorf("%q: %v, want reason with %q", l, err, want)
		}
	}
	if err := ValidateLatex(strings.Repeat("x", 2001), DefaultLimits); err == nil || !strings.Contains(err.Error(), "latex_max_length") {
		t.Errorf("length: %v", err)
	}
	deep := strings.Repeat("{", 21) + "x" + strings.Repeat("}", 21)
	if err := ValidateLatex(deep, DefaultLimits); err == nil || !strings.Contains(err.Error(), "latex_max_depth") {
		t.Errorf("depth: %v", err)
	}
	if err := ValidateLatex(strings.Repeat("{", 20)+"x"+strings.Repeat("}", 20), DefaultLimits); err != nil {
		t.Errorf("depth 20: %v", err)
	}
}

// A MathJax 4 style SVG (TEST FIXTURE).
const mathJaxSVG = `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="2.1ex" height="3.3ex" ` +
	`viewBox="0 -1000 928 1458" role="img" focusable="false" style="vertical-align: -1.036ex;">` +
	`<defs><path id="MJX-1-TEX-N-33" d="M127 463Q100 463 85 480T69 524"></path></defs>` +
	`<g stroke="currentColor" fill="currentColor" stroke-width="0" transform="scale(1,-1)"><g data-mml-node="math">` +
	`<use data-c="33" xlink:href="#MJX-1-TEX-N-33"></use><rect width="700" height="60" x="120" y="220"></rect></g></g></svg>`

// F027-AC-13, SC-004: scripts, event handlers, external references and foreign content are removed.
func TestSanitizeSVG(t *testing.T) {
	clean, err := SanitizeSVG([]byte(mathJaxSVG), MathProfile, DefaultLimits.SVGMaxBytes)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`xlink:href="#MJX-1-TEX-N-33"`, `data-mml-node="math"`, `viewBox="0 -1000 928 1458"`, "<path", "<rect"} {
		if !bytes.Contains(clean, []byte(want)) {
			t.Errorf("lost %s: %s", want, clean)
		}
	}
	again, err := SanitizeSVG(clean, MathProfile, DefaultLimits.SVGMaxBytes)
	if err != nil || !bytes.Equal(again, clean) || Hash(again) != Hash(clean) {
		t.Errorf("sanitize is not idempotent: %v", err)
	}
	evil := `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" onload="alert(1)">` +
		`<script>alert(2)</script><g onclick="steal()" style="fill:url(http://evil/x)"><path d="M0 0" onmouseover="x()"/></g>` +
		`<use xlink:href="http://evil/sprite.svg#a"/><foreignObject><div xmlns="http://www.w3.org/1999/xhtml">hi</div></foreignObject>` +
		`<image href="http://evil/p.png"/><a href="javascript:alert(3)"><path d="M1 1"/></a>` +
		`<rect style="behavior:url(x.htc)" width="1"/><path fill="url(#grad)" d="M2 2"/></svg>`
	out, err := SanitizeSVG([]byte(evil), MathProfile, DefaultLimits.SVGMaxBytes)
	if err != nil {
		t.Fatal(err)
	}
	s := strings.ToLower(string(out))
	for _, bad := range []string{"<script", "onload", "onclick", "onmouseover", "http://evil", "foreignobject", "<image", "<a ", "javascript:",
		"behavior", "alert"} {
		if strings.Contains(s, bad) {
			t.Errorf("kept %q: %s", bad, out)
		}
	}
	if !strings.Contains(s, `fill="url(#grad)"`) {
		t.Errorf("internal url(#…) dropped: %s", out)
	}
	for name, in := range map[string]string{
		"missing":    "",
		"unparsable": "<svg><g></svg>",
		"not svg":    `<html><svg/></html>`,
		"entity":     `<!DOCTYPE svg [<!ENTITY x "y">]><svg>&x;</svg>`,
	} {
		if _, err := SanitizeSVG([]byte(in), MathProfile, DefaultLimits.SVGMaxBytes); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	big := `<svg xmlns="http://www.w3.org/2000/svg">` + strings.Repeat(`<path d="M0 0"/>`, 20000) + `</svg>`
	if _, err := SanitizeSVG([]byte(big), MathProfile, DefaultLimits.SVGMaxBytes); err == nil || !strings.Contains(err.Error(), "math_svg_max") {
		t.Errorf("size: %v", err)
	}
	img := `<svg xmlns="http://www.w3.org/2000/svg"><circle cx="1" cy="1" r="1"/><text x="0">A</text></svg>`
	if out, err := SanitizeSVG([]byte(img), ImageProfile, 0); err != nil || !bytes.Contains(out, []byte("<circle")) {
		t.Errorf("image profile: %s %v", out, err)
	}
	if out, _ := SanitizeSVG([]byte(img), MathProfile, 0); bytes.Contains(out, []byte("<circle")) {
		t.Errorf("math profile kept circle: %s", out)
	}
}

// F027-AC-14: the math block of a render payload.
func TestTransform(t *testing.T) {
	var v any
	_ = json.Unmarshal([]byte(`{"blocks":[{"type":"math","latex":"\\frac{3}{4}"},{"type":"paragraph","text":"x"}]}`), &v)
	key := Key(strings.Repeat("c", 64))
	out := Transform(v, []Rendered{{Latex: `\frac{3}{4}`, SVGAsset: key, SpeechText: "ba phần tư"}}, false)
	b, _ := json.Marshal(out)
	for _, want := range []string{`"math_ref":"` + key + `"`, `"speech_text":"ba phần tư"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("missing %s in %s", want, b)
		}
	}
	if strings.Contains(string(b), "latex") {
		t.Errorf("student payload keeps the LaTeX: %s", b)
	}
	var w any
	_ = json.Unmarshal([]byte(`{"type":"math","latex":"x^2"}`), &w)
	b, _ = json.Marshal(Transform(w, []Rendered{{Latex: "x^2", SVGAsset: key, SpeechText: "x bình phương"}}, true))
	if !strings.Contains(string(b), `"latex":"x^2"`) || !strings.Contains(string(b), "math_ref") {
		t.Errorf("CMS block %s", b)
	}
	if !SVGKeyPattern.MatchString(key) {
		t.Errorf("key %s", key)
	}
}
