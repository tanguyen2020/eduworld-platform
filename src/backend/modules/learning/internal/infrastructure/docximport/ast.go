package docximport

import (
	"encoding/json"
	"path"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	dom "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/docximport"
)

// node is one element of the Pandoc JSON AST ({"t": type, "c": content}).
type node struct {
	T string          `json:"t"`
	C json.RawMessage `json:"c"`
}

// astDoc is the top level of the Pandoc JSON AST (pandoc-types 1.23).
type astDoc struct {
	API    []int  `json:"pandoc-api-version"`
	Blocks []node `json:"blocks"`
}

// converter turns the AST into the paragraph model.
type converter struct {
	ole    map[string]bool
	blocks []dom.Block
}

// toDocument converts the JSON AST of Pandoc (research R2, R3): paragraphs, headings and lines keep their text,
// bold and formulas; lists are flattened (warning), tables refused (error on their question), notes and raw OOXML
// dropped (warning), external links reduced to their text (warning). Text is put in Unicode NFC.
func toDocument(raw []byte, ole map[string]bool) (dom.Document, error) {
	var d astDoc
	if err := json.Unmarshal(raw, &d); err != nil || len(d.API) == 0 || d.API[0] != 1 {
		return dom.Document{}, port.ErrDocxInvalid
	}
	c := &converter{ole: ole}
	c.blockList(d.Blocks, nil)
	return dom.Document{Blocks: c.blocks}, nil
}

func (c *converter) blockList(bs []node, notes []dom.Issue) {
	for _, b := range bs {
		c.block(b, notes)
		notes = nil
	}
}

func (c *converter) block(b node, notes []dom.Issue) {
	switch b.T {
	case "Para", "Plain":
		var ins []node
		_ = json.Unmarshal(b.C, &ins)
		c.para(ins, notes)
	case "Header": // [level, attr, inlines]
		var parts []json.RawMessage
		var ins []node
		if json.Unmarshal(b.C, &parts) == nil && len(parts) == 3 {
			_ = json.Unmarshal(parts[2], &ins)
		}
		c.para(ins, notes)
	case "LineBlock": // [[inlines]]
		var lines [][]node
		_ = json.Unmarshal(b.C, &lines)
		for _, l := range lines {
			c.para(l, notes)
			notes = nil
		}
	case "CodeBlock": // [attr, text]
		var parts []json.RawMessage
		var s string
		if json.Unmarshal(b.C, &parts) == nil && len(parts) == 2 {
			_ = json.Unmarshal(parts[1], &s)
		}
		c.blocks = append(c.blocks, dom.Block{Inlines: []dom.Inline{{Kind: dom.Text, Text: norm.NFC.String(s)}}, Notes: notes})
	case "BlockQuote":
		var bs []node
		_ = json.Unmarshal(b.C, &bs)
		c.blockList(bs, notes)
	case "Div", "Figure": // [attr, blocks] / [attr, caption, blocks]
		var parts []json.RawMessage
		var bs []node
		if json.Unmarshal(b.C, &parts) == nil && len(parts) >= 2 {
			_ = json.Unmarshal(parts[len(parts)-1], &bs)
		}
		c.blockList(bs, notes)
	case "BulletList", "OrderedList": // [[blocks]] / [attrs, [[blocks]]]
		var items [][]node
		if b.T == "OrderedList" {
			var parts []json.RawMessage
			if json.Unmarshal(b.C, &parts) == nil && len(parts) == 2 {
				_ = json.Unmarshal(parts[1], &items)
			}
		} else {
			_ = json.Unmarshal(b.C, &items)
		}
		notes = append(notes, dom.Warn(dom.CodeListFlattened, "", "a Word list was flattened: type \"Câu n.\" and \"A.\" by hand"))
		for _, it := range items {
			c.blockList(it, notes)
			notes = nil
		}
	case "DefinitionList":
		notes = append(notes, dom.Warn(dom.CodeListFlattened, "", "a definition list was flattened"))
		c.blocks = append(c.blocks, dom.Block{Notes: notes})
	case "Table":
		c.blocks = append(c.blocks, dom.Block{Notes: append(notes, dom.Err(dom.CodeUnsupportedContent, "", "tables cannot be imported; add the table in the editor"))})
	case "HorizontalRule", "RawBlock", "Null":
		if len(notes) > 0 {
			c.blocks = append(c.blocks, dom.Block{Notes: notes})
		}
	default:
		c.blocks = append(c.blocks, dom.Block{Notes: append(notes, dom.Err(dom.CodeUnsupportedContent, "", "unsupported content: "+b.T))})
	}
}

func (c *converter) para(ins []node, notes []dom.Issue) {
	blk := dom.Block{Notes: notes}
	c.inlines(&blk, ins, false)
	c.blocks = append(c.blocks, blk)
}

func (c *converter) text(blk *dom.Block, s string, bold bool) {
	s = norm.NFC.String(s)
	if n := len(blk.Inlines); n > 0 && blk.Inlines[n-1].Kind == dom.Text && blk.Inlines[n-1].Bold == bold {
		blk.Inlines[n-1].Text += s
		return
	}
	blk.Inlines = append(blk.Inlines, dom.Inline{Kind: dom.Text, Text: s, Bold: bold})
}

func (c *converter) note(blk *dom.Block, i dom.Issue) {
	for _, x := range blk.Notes {
		if x.Code == i.Code {
			return
		}
	}
	blk.Notes = append(blk.Notes, i)
}

func (c *converter) inlines(blk *dom.Block, ins []node, bold bool) {
	for _, in := range ins {
		switch in.T {
		case "Str":
			var s string
			_ = json.Unmarshal(in.C, &s)
			c.text(blk, s, bold)
		case "Space", "SoftBreak":
			c.text(blk, " ", bold)
		case "LineBreak":
			blk.Inlines = append(blk.Inlines, dom.Inline{Kind: dom.Break})
		case "Strong":
			var sub []node
			_ = json.Unmarshal(in.C, &sub)
			c.inlines(blk, sub, true)
		case "Emph", "Underline", "Strikeout", "SmallCaps":
			var sub []node
			_ = json.Unmarshal(in.C, &sub)
			c.inlines(blk, sub, bold)
		case "Span", "Quoted", "Cite": // [attr|type|citations, inlines]
			var parts []json.RawMessage
			var sub []node
			if json.Unmarshal(in.C, &parts) == nil && len(parts) == 2 {
				_ = json.Unmarshal(parts[1], &sub)
			}
			if in.T == "Quoted" {
				c.text(blk, "“", bold)
				c.inlines(blk, sub, bold)
				c.text(blk, "”", bold)
				continue
			}
			c.inlines(blk, sub, bold)
		case "Superscript", "Subscript":
			var sub []node
			_ = json.Unmarshal(in.C, &sub)
			tmp := dom.Block{}
			c.inlines(&tmp, sub, false)
			s := scriptLatex(flat(tmp))
			if s == "" {
				continue
			}
			op := "^"
			if in.T == "Subscript" {
				op = "_"
			}
			blk.Inlines = append(blk.Inlines, dom.Inline{Kind: dom.Math, Latex: op + "{" + s + "}"})
			c.note(blk, dom.Warn(dom.CodeTextScriptConverted, "", "superscript/subscript text was turned into a formula"))
		case "Code": // [attr, text]
			var parts []json.RawMessage
			var s string
			if json.Unmarshal(in.C, &parts) == nil && len(parts) == 2 {
				_ = json.Unmarshal(parts[1], &s)
			}
			c.text(blk, s, bold)
		case "Math": // [{"t": InlineMath|DisplayMath}, latex]
			var parts []json.RawMessage
			var kind node
			var latex string
			if json.Unmarshal(in.C, &parts) == nil && len(parts) == 2 {
				_ = json.Unmarshal(parts[0], &kind)
				_ = json.Unmarshal(parts[1], &latex)
			}
			blk.Inlines = append(blk.Inlines, dom.Inline{Kind: dom.Math, Latex: norm.NFC.String(latex), Display: kind.T == "DisplayMath"})
		case "Image": // [attr, alt inlines, [src, title]]
			var parts []json.RawMessage
			var alt []node
			var target []string
			if json.Unmarshal(in.C, &parts) == nil && len(parts) == 3 {
				_ = json.Unmarshal(parts[1], &alt)
				_ = json.Unmarshal(parts[2], &target)
			}
			src := ""
			if len(target) > 0 {
				src = target[0]
			}
			p := path.Join("word", src)
			if strings.Contains(src, "://") || c.ole[p] {
				if c.ole[p] {
					blk.Inlines = append(blk.Inlines, dom.Inline{Kind: dom.OLE})
				} else {
					c.note(blk, dom.Err(dom.CodeImageNotFound, "", "a linked (not embedded) picture cannot be imported"))
				}
				continue
			}
			tmp := dom.Block{}
			c.inlines(&tmp, alt, false)
			blk.Inlines = append(blk.Inlines, dom.Inline{Kind: dom.Image, Path: p, Alt: strings.TrimSpace(flat(tmp))})
		case "Link": // [attr, inlines, [url, title]]
			var parts []json.RawMessage
			var sub []node
			var target []string
			if json.Unmarshal(in.C, &parts) == nil && len(parts) == 3 {
				_ = json.Unmarshal(parts[1], &sub)
				_ = json.Unmarshal(parts[2], &target)
			}
			c.inlines(blk, sub, bold)
			if len(target) > 0 && !strings.HasPrefix(target[0], "#") {
				c.note(blk, dom.Warn(dom.CodeExternalLinkRemoved, "", "a link was removed (its text is kept)"))
			}
		case "Note":
			c.note(blk, dom.Warn(dom.CodeFootnoteRemoved, "", "a footnote or comment was removed"))
		case "RawInline":
			// raw OOXML is dropped
		}
	}
}

// flat is the text of a block of text pieces.
func flat(b dom.Block) string {
	var s strings.Builder
	for _, in := range b.Inlines {
		switch in.Kind {
		case dom.Text:
			s.WriteString(in.Text)
		case dom.Math:
			s.WriteString(in.Latex)
		}
	}
	return s.String()
}

// scriptLatex is the LaTeX of superscript/subscript text: letters, digits and + - = ( ) as they are; any other
// character is dropped (it cannot be written safely without a command).
func scriptLatex(s string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(s) {
		switch {
		case r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r)), strings.ContainsRune("+-=() ", r):
			b.WriteRune(r)
		case r == '−':
			b.WriteRune('-')
		}
	}
	return strings.TrimSpace(b.String())
}
