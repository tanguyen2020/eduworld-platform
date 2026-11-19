// Package mathx is the domain of the formulas of learning content (F027 US3, ADR-0055, CONTENT_MODEL §9.1): the
// LaTeX allowlist (no links, HTML, styles, macro definitions or loaders; ams/mhchem environments and a limited
// \color), the sanitizer of the SVG rendered by the portal (MathJax 4) and the math block of a render payload.
// LaTeX and SVG are untrusted input. Pure Go, no I/O.
package mathx

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Limits of a formula (CONFIG_DEFAULTS latex_max_length / latex_max_depth / math_svg_max, ADR-0055 §4).
type Limits struct {
	LatexMaxLength int // 2000 characters
	LatexMaxDepth  int // 20
	SVGMaxBytes    int // 256 KB
}

// DefaultLimits are the [Proposed] code defaults of CONFIG_DEFAULTS.
var DefaultLimits = Limits{LatexMaxLength: 2000, LatexMaxDepth: 20, SVGMaxBytes: 256 << 10}

// Error is a rejected formula: the reason is safe to return to the editor (no formula content).
type Error struct {
	Reason string
}

func (e Error) Error() string { return "mathx: " + e.Reason }

// forbidden commands (ADR-0055 §7) and macro definitions/loaders.
var forbidden = []string{"href", "url", "class", "style", "require", "unicode", "cssId", "def", "gdef", "edef", "xdef", "let",
	"newcommand", "renewcommand", "providecommand", "DeclareMathOperator", "newenvironment", "renewenvironment", "input", "include",
	"includegraphics", "write", "immediate", "catcode", "csname", "endcsname", "expandafter", "futurelet", "openout", "data", "texttip",
	"toggle", "mathtip", "action", "tooltip"}

// environments allowed in \begin{…} (amsmath + array).
var environments = []string{"matrix", "pmatrix", "bmatrix", "Bmatrix", "vmatrix", "Vmatrix", "smallmatrix", "cases", "dcases", "rcases",
	"aligned", "align", "align*", "alignedat", "alignat", "alignat*", "gathered", "gather", "gather*", "split", "array", "subarray",
	"equation", "equation*", "multline", "multline*", "eqnarray", "eqnarray*"}

// colors allowed in \color / \textcolor ("color" extension, limited palette).
var colors = []string{"black", "red", "blue", "green", "orange", "purple", "gray", "grey", "brown", "teal", "magenta", "cyan"}

// ValidateLatex checks a formula against the allowlist (F027-AC-12): length, nesting depth of groups and
// \left…\right, forbidden commands (\href, \url, \html…, \class, \style, \require, \unicode, macro definitions),
// environments of ams and the limited color palette.
func ValidateLatex(latex string, lim Limits) error {
	if strings.TrimSpace(latex) == "" {
		return Error{Reason: "the formula is empty"}
	}
	if lim.LatexMaxLength > 0 && utf8.RuneCountInString(latex) > lim.LatexMaxLength {
		return Error{Reason: fmt.Sprintf("the formula is longer than %d characters (latex_max_length)", lim.LatexMaxLength)}
	}
	depth, maxDepth := 0, 0
	var stack []byte // '{' or 'l' (\left)
	rs := []rune(latex)
	for i := 0; i < len(rs); i++ {
		switch c := rs[i]; c {
		case '\\':
			j := i + 1
			for j < len(rs) && unicode.IsLetter(rs[j]) {
				j++
			}
			if j == i+1 { // control symbol (\{, \\, \,): skip the escaped character
				i++
				continue
			}
			name := string(rs[i+1 : j])
			i = j - 1
			if err := checkCommand(name, rs[j:]); err != nil {
				return err
			}
			switch name {
			case "left":
				stack = append(stack, 'l')
				depth++
			case "right":
				if len(stack) == 0 || stack[len(stack)-1] != 'l' {
					return Error{Reason: `\right without \left`}
				}
				stack = stack[:len(stack)-1]
				depth--
			}
		case '{':
			stack = append(stack, '{')
			depth++
		case '}':
			if len(stack) == 0 || stack[len(stack)-1] != '{' {
				return Error{Reason: "unbalanced braces"}
			}
			stack = stack[:len(stack)-1]
			depth--
		case '%':
			return Error{Reason: "comments are not allowed"}
		}
		maxDepth = max(maxDepth, depth)
		if lim.LatexMaxDepth > 0 && maxDepth > lim.LatexMaxDepth {
			return Error{Reason: fmt.Sprintf("the formula is nested deeper than %d (latex_max_depth)", lim.LatexMaxDepth)}
		}
	}
	if len(stack) != 0 {
		return Error{Reason: "unbalanced braces or \\left without \\right"}
	}
	return nil
}

// checkCommand applies the rules of one command; rest is the text after its name.
func checkCommand(name string, rest []rune) error {
	if slices.Contains(forbidden, name) || strings.HasPrefix(name, "html") || (strings.HasPrefix(name, "new") && name != "newline") ||
		strings.HasPrefix(name, "renew") || strings.HasPrefix(name, "def") {
		return Error{Reason: `the command \` + name + " is not allowed"}
	}
	switch name {
	case "begin", "end":
		env, ok := braced(rest)
		if !ok || !slices.Contains(environments, env) {
			return Error{Reason: "the environment " + env + " is not allowed"}
		}
	case "color", "textcolor":
		c, ok := braced(rest)
		if !ok || !slices.Contains(colors, c) {
			return Error{Reason: "only the colors " + strings.Join(colors, ", ") + " are allowed"}
		}
	case "colorbox", "fcolorbox", "definecolor":
		return Error{Reason: `the command \` + name + " is not allowed"}
	}
	return nil
}

// braced returns the content of the group {…} at the start of rest (spaces skipped).
func braced(rest []rune) (string, bool) {
	s := strings.TrimLeftFunc(string(rest), unicode.IsSpace)
	if !strings.HasPrefix(s, "{") {
		return "", false
	}
	end := strings.IndexByte(s, '}')
	if end < 0 {
		return "", false
	}
	return strings.TrimSpace(s[1:end]), true
}
