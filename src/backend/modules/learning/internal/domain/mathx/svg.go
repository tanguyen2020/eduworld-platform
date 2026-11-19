package mathx

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
)

// Profile is the allowlist of a sanitizer: the SVG of a formula (MathJax 4 output) or an SVG image of the media
// library (CONTENT_MODEL §7 "SVG (sanitize)").
type Profile struct {
	Elements []string
	Attrs    []string
}

// MathProfile allows what MathJax 4 SVG output uses (research F027 R2): svg, g, path, rect, line, use (internal
// only), defs, title, desc, plus data-* attributes.
var MathProfile = Profile{
	Elements: []string{"svg", "g", "path", "rect", "line", "use", "defs", "title", "desc"},
	Attrs: []string{"width", "height", "viewBox", "style", "role", "focusable", "aria-hidden", "aria-label", "aria-labelledby", "id", "d", "x", "y",
		"x1", "y1", "x2", "y2", "transform", "fill", "stroke", "stroke-width", "href", "class", "preserveAspectRatio", "version", "overflow"},
}

// ImageProfile allows the static drawing subset of SVG images (no script, animation, foreignObject, image, a).
var ImageProfile = Profile{
	Elements: append(slices.Clone(MathProfile.Elements), "circle", "ellipse", "polygon", "polyline", "text", "tspan", "linearGradient",
		"radialGradient", "stop", "clipPath", "mask", "pattern", "symbol", "marker"),
	Attrs: append(slices.Clone(MathProfile.Attrs), "cx", "cy", "r", "rx", "ry", "fx", "fy", "points", "offset", "stop-color", "stop-opacity",
		"opacity", "fill-opacity", "stroke-opacity", "stroke-linecap", "stroke-linejoin", "stroke-dasharray", "stroke-dashoffset",
		"stroke-miterlimit", "font-size", "font-family", "font-weight", "font-style", "text-anchor", "dominant-baseline", "gradientUnits",
		"gradientTransform", "spreadMethod", "clip-path", "clipPathUnits", "mask", "maskUnits", "patternUnits", "patternTransform", "fill-rule",
		"clip-rule", "dx", "dy", "markerWidth", "markerHeight", "refX", "refY", "orient", "marker-start", "marker-mid", "marker-end"),
}

const (
	svgNS   = "http://www.w3.org/2000/svg"
	xlinkNS = "http://www.w3.org/1999/xlink"
	// maxNesting bounds the element depth of a sanitized SVG.
	maxNesting = 64
)

// SanitizeSVG parses svg and rebuilds it from the allowlist of p (F027-AC-13, SC-004): unknown elements are dropped
// with their content (script, foreignObject, image, a, animate…), so are event attributes (on*), references that
// are not internal (#id), and styles with url(…) other than url(#id), expressions or imports. The input and the
// output must be at most maxBytes; the root must be <svg>.
func SanitizeSVG(svg []byte, p Profile, maxBytes int) ([]byte, error) {
	if len(bytes.TrimSpace(svg)) == 0 {
		return nil, Error{Reason: "the SVG is missing"}
	}
	if maxBytes > 0 && len(svg) > maxBytes {
		return nil, Error{Reason: fmt.Sprintf("the SVG is larger than %d bytes (math_svg_max)", maxBytes)}
	}
	d := xml.NewDecoder(bytes.NewReader(svg))
	d.Strict = true
	var out bytes.Buffer
	depth, skip, root := 0, 0, false
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, Error{Reason: "the SVG cannot be parsed"}
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if depth > maxNesting {
				return nil, Error{Reason: "the SVG is nested too deeply"}
			}
			if skip > 0 {
				skip++
				continue
			}
			name := t.Name.Local
			if depth == 1 {
				if name != "svg" || (t.Name.Space != "" && t.Name.Space != svgNS) {
					return nil, Error{Reason: "the root element is not <svg>"}
				}
				root = true
			}
			if (t.Name.Space != "" && t.Name.Space != svgNS) || !slices.Contains(p.Elements, name) {
				skip = 1
				continue
			}
			if name == "use" && !internalRef(t.Attr) {
				skip = 1
				continue
			}
			out.WriteString("<" + name)
			if depth == 1 {
				out.WriteString(` xmlns="` + svgNS + `" xmlns:xlink="` + xlinkNS + `"`)
			}
			for _, a := range t.Attr {
				if n, v, ok := keepAttr(a, p); ok {
					out.WriteString(" " + n + `="`)
					_ = xml.EscapeText(&out, []byte(v))
					out.WriteString(`"`)
				}
			}
			out.WriteString(">")
		case xml.EndElement:
			depth--
			if skip > 0 {
				skip--
				continue
			}
			out.WriteString("</" + t.Name.Local + ">")
		case xml.CharData:
			if skip == 0 && depth > 0 {
				_ = xml.EscapeText(&out, t)
			}
		}
	}
	if !root || depth != 0 {
		return nil, Error{Reason: "the SVG has no complete <svg> root"}
	}
	if maxBytes > 0 && out.Len() > maxBytes {
		return nil, Error{Reason: fmt.Sprintf("the SVG is larger than %d bytes (math_svg_max)", maxBytes)}
	}
	return out.Bytes(), nil
}

func internalRef(attrs []xml.Attr) bool {
	for _, a := range attrs {
		if a.Name.Local == "href" {
			return strings.HasPrefix(strings.TrimSpace(a.Value), "#")
		}
	}
	return false
}

// keepAttr returns the output name and value of an allowed attribute.
func keepAttr(a xml.Attr, p Profile) (string, string, bool) {
	local, v := a.Name.Local, a.Value
	switch {
	case a.Name.Space == "xmlns" || local == "xmlns" || strings.HasPrefix(strings.ToLower(local), "on"):
		return "", "", false
	case local == "href":
		if !strings.HasPrefix(strings.TrimSpace(v), "#") {
			return "", "", false
		}
		if a.Name.Space == xlinkNS || a.Name.Space == "xlink" {
			return "xlink:href", v, true
		}
		return "href", v, true
	case a.Name.Space != "":
		return "", "", false
	case strings.HasPrefix(local, "data-"):
		return local, v, true
	case !slices.Contains(p.Attrs, local):
		return "", "", false
	}
	if !safeValue(v) {
		return "", "", false
	}
	return local, v, true
}

// safeValue refuses values that load or run something: url() other than url(#id), javascript:, expression(),
// @import, escapes and behaviors.
func safeValue(v string) bool {
	l := strings.ToLower(strings.Join(strings.Fields(v), ""))
	for _, bad := range []string{"javascript:", "expression(", "@import", "\\", "behavior:", "-moz-binding", "data:"} {
		if strings.Contains(l, bad) {
			return false
		}
	}
	for i := strings.Index(l, "url("); i >= 0; {
		if !strings.HasPrefix(l[i+4:], "#") && !strings.HasPrefix(l[i+4:], "'#") && !strings.HasPrefix(l[i+4:], `"#`) {
			return false
		}
		next := strings.Index(l[i+4:], "url(")
		if next < 0 {
			break
		}
		i += 4 + next
	}
	return true
}
