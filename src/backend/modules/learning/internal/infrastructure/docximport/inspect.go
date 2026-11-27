// Package docximport converts Word .docx files for the import lessons_docx (F028, research R1–R3): the zip is checked
// in Go (size, number of parts, unpacked size, macros, MathType OLE objects), then Pandoc runs as an external process
// (`pandoc --sandbox -f docx -t json`, stdin/stdout, timeout, heap limit, empty environment) and its JSON AST becomes
// the neutral paragraph model of domain/docximport. Embedded pictures are read from the zip, never by Pandoc.
package docximport

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"path"
	"strings"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
)

// ZipLimits bound the zip of a .docx (CONFIG_DEFAULTS docx_zip_entries_max, docx_uncompressed_max).
type ZipLimits struct {
	MaxEntries      int
	MaxUncompressed int64
	MaxMedia        int64 // one embedded picture (media_max_bytes_by_kind image)
}

// DefaultZipLimits are the code defaults of CONFIG_DEFAULTS.
var DefaultZipLimits = ZipLimits{MaxEntries: 1000, MaxUncompressed: 50 << 20, MaxMedia: 10 << 20}

const docxMain = "word/document.xml"

// inspection is what the zip check learns: the pictures that are previews of MathType/Equation OLE objects.
type inspection struct {
	olePreviews map[string]bool // zip paths (word/media/…)
}

// inspect checks the zip of a .docx: a valid zip with word/document.xml, at most MaxEntries parts and MaxUncompressed
// bytes once unpacked (declared sizes; reads are bounded too), no macro part (vbaProject.bin, macro-enabled content
// type). It finds the OLE equation objects of the main document.
func inspect(data []byte, lim ZipLimits) (inspection, error) {
	out := inspection{olePreviews: map[string]bool{}}
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return out, port.ErrDocxInvalid
	}
	if len(z.File) > lim.MaxEntries {
		return out, port.ErrDocxTooComplex
	}
	var total uint64
	var main, rels, types *zip.File
	for _, f := range z.File {
		total += f.UncompressedSize64
		name := strings.ToLower(f.Name)
		switch {
		case strings.HasSuffix(name, "vbaproject.bin"), strings.HasSuffix(name, "vbadata.xml"):
			return out, port.ErrDocxMacro
		case f.Name == docxMain:
			main = f
		case f.Name == "word/_rels/document.xml.rels":
			rels = f
		case f.Name == "[Content_Types].xml":
			types = f
		}
	}
	if total > uint64(lim.MaxUncompressed) { //nolint:gosec // positive limit
		return out, port.ErrDocxTooComplex
	}
	if main == nil || types == nil {
		return out, port.ErrDocxInvalid
	}
	ct, err := readPart(types, lim.MaxUncompressed)
	if err != nil {
		return out, err
	}
	if bytes.Contains(bytes.ToLower(ct), []byte("macroenabled")) {
		return out, port.ErrDocxMacro
	}
	targets := map[string]string{}
	if rels != nil {
		b, err := readPart(rels, lim.MaxUncompressed)
		if err != nil {
			return out, err
		}
		var rs struct {
			Rel []struct {
				ID     string `xml:"Id,attr"`
				Target string `xml:"Target,attr"`
				Mode   string `xml:"TargetMode,attr"`
			} `xml:"Relationship"`
		}
		if xml.Unmarshal(b, &rs) != nil {
			return out, port.ErrDocxInvalid
		}
		for _, r := range rs.Rel {
			if r.Mode != "External" {
				targets[r.ID] = path.Join("word", r.Target)
			}
		}
	}
	doc, err := readPart(main, lim.MaxUncompressed)
	if err != nil {
		return out, err
	}
	if err := scanOLE(doc, targets, out.olePreviews); err != nil {
		return out, port.ErrDocxInvalid
	}
	return out, nil
}

func readPart(f *zip.File, max int64) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, port.ErrDocxInvalid
	}
	defer func() { _ = rc.Close() }()
	b, err := io.ReadAll(io.LimitReader(rc, max+1))
	if err != nil {
		return nil, port.ErrDocxInvalid
	}
	if int64(len(b)) > max {
		return nil, port.ErrDocxTooComplex
	}
	return b, nil
}

// scanOLE marks the preview pictures (v:imagedata r:id) of the w:object elements holding an OLE equation
// (o:OLEObject ProgID Equation.*: MathType Equation.DSMT4, Word 3.0 Equation.3).
func scanOLE(doc []byte, targets map[string]string, previews map[string]bool) error {
	d := xml.NewDecoder(bytes.NewReader(doc))
	depth, objDepth := 0, -1
	var images []string
	equation := false
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			switch {
			case t.Name.Local == "object" && objDepth < 0:
				objDepth, images, equation = depth, nil, false
			case objDepth >= 0 && t.Name.Local == "imagedata":
				for _, a := range t.Attr {
					if a.Name.Local == "id" {
						images = append(images, a.Value)
					}
				}
			case objDepth >= 0 && t.Name.Local == "OLEObject":
				for _, a := range t.Attr {
					if a.Name.Local == "ProgID" && strings.HasPrefix(a.Value, "Equation.") {
						equation = true
					}
				}
			}
		case xml.EndElement:
			if depth == objDepth {
				if equation {
					for _, id := range images {
						if p, ok := targets[id]; ok {
							previews[p] = true
						}
					}
				}
				objDepth = -1
			}
			depth--
		}
	}
}

// media reads one part of the zip (an embedded picture), at most max bytes.
func media(data []byte, name string, max int64) ([]byte, error) {
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, port.ErrDocxInvalid
	}
	for _, f := range z.File {
		if f.Name == name {
			if f.UncompressedSize64 > uint64(max) { //nolint:gosec // positive limit
				return nil, port.ErrDocxTooComplex
			}
			return readPart(f, max)
		}
	}
	return nil, port.ErrDocxMediaNotFound
}
