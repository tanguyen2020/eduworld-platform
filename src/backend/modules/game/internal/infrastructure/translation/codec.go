// Package translation holds the translation file formats of module game (F082, research R4): a minimal XLIFF 2.1 read
// and written with encoding/xml (no DTD, no entity — XXE is refused) and a UTF-8 CSV with a BOM (key,source,target,note).
package translation

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/i18ntext"
)

// MaxFileBytes bounds what a decoder reads (the document kind limit of game media, 5 MB).
const MaxFileBytes = 5 << 20

func malformed(format string, err error) error {
	return fmt.Errorf("%w: %s: %w", port.ErrMalformedFile, format, err)
}

// XLIFF implements port.TranslationCodec for XLIFF 2.1.
type XLIFF struct{}

var _ port.TranslationCodec = XLIFF{}

const xliffNS = "urn:oasis:names:tc:xliff:document:2.0"

type xDoc struct {
	XMLName xml.Name `xml:"xliff"`
	NS      string   `xml:"xmlns,attr,omitempty"`
	Version string   `xml:"version,attr"`
	SrcLang string   `xml:"srcLang,attr"`
	TrgLang string   `xml:"trgLang,attr,omitempty"`
	Files   []xFile  `xml:"file"`
}

type xFile struct {
	ID    string  `xml:"id,attr"`
	Units []xUnit `xml:"unit"`
}

type xUnit struct {
	ID      string     `xml:"id,attr"`
	Notes   *xNotes    `xml:"notes,omitempty"`
	Segment []xSegment `xml:"segment"`
}

type xNotes struct {
	Note []string `xml:"note"`
}

type xSegment struct {
	Source string  `xml:"source"`
	Target *string `xml:"target"`
}

// Extension implements port.TranslationCodec.
func (XLIFF) Extension() string { return "xlf" }

// ContentType implements port.TranslationCodec.
func (XLIFF) ContentType() string { return "application/xliff+xml" }

// Encode implements port.TranslationCodec: one <file> per config type (the first part of the key), units in key order.
func (XLIFF) Encode(w io.Writer, f i18ntext.File) error {
	d := xDoc{NS: xliffNS, Version: "2.1", SrcLang: f.SourceLanguage, TrgLang: f.TargetLanguage}
	idx := map[string]int{}
	for _, u := range f.Units {
		typ := u.Key
		if i := strings.IndexByte(typ, '/'); i > 0 {
			typ = typ[:i]
		}
		i, ok := idx[typ]
		if !ok {
			i = len(d.Files)
			idx[typ] = i
			d.Files = append(d.Files, xFile{ID: typ})
		}
		tgt := u.Target
		xu := xUnit{ID: u.Key, Segment: []xSegment{{Source: u.Source, Target: &tgt}}}
		if u.Note != "" {
			xu.Notes = &xNotes{Note: []string{u.Note}}
		}
		d.Files[i].Units = append(d.Files[i].Units, xu)
	}
	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(d); err != nil {
		return err
	}
	_, err := io.WriteString(w, "\n")
	return err
}

// Decode implements port.TranslationCodec. A DOCTYPE (DTD, entities) is refused before parsing; encoding/xml never
// resolves external entities, and an undefined entity is an error (Strict).
func (XLIFF) Decode(r io.Reader) (i18ntext.File, error) {
	b, err := io.ReadAll(io.LimitReader(r, MaxFileBytes+1))
	if err != nil {
		return i18ntext.File{}, err
	}
	if len(b) > MaxFileBytes {
		return i18ntext.File{}, malformed("xliff", errors.New("file too large"))
	}
	if bytes.Contains(bytes.ToUpper(b), []byte("<!DOCTYPE")) || bytes.Contains(bytes.ToUpper(b), []byte("<!ENTITY")) {
		return i18ntext.File{}, malformed("xliff", errors.New("DTD and entities are not allowed"))
	}
	dec := xml.NewDecoder(bytes.NewReader(b))
	dec.Strict = true
	var d xDoc
	if err := dec.Decode(&d); err != nil {
		return i18ntext.File{}, malformed("xliff", err)
	}
	if d.Version != "2.1" && d.Version != "2.0" {
		return i18ntext.File{}, malformed("xliff", fmt.Errorf("version %q (2.1 expected)", d.Version))
	}
	out := i18ntext.File{SourceLanguage: d.SrcLang, TargetLanguage: d.TrgLang}
	for _, f := range d.Files {
		for _, u := range f.Units {
			unit := i18ntext.Unit{Key: u.ID}
			if u.Notes != nil && len(u.Notes.Note) > 0 {
				unit.Note = u.Notes.Note[0]
			}
			for i, s := range u.Segment {
				if i > 0 { // several segments of one text are joined (the export writes one)
					unit.Source += " "
					unit.Target += " "
				}
				unit.Source += s.Source
				if s.Target != nil {
					unit.Target += *s.Target
				}
			}
			out.Units = append(out.Units, unit)
		}
	}
	return out, nil
}

// CSV implements port.TranslationCodec for the CSV format (UTF-8 with a BOM, header key,source,target,note).
type CSV struct{}

var _ port.TranslationCodec = CSV{}

var csvHeader = []string{"key", "source", "target", "note"}

const bom = "\xEF\xBB\xBF"

// Extension implements port.TranslationCodec.
func (CSV) Extension() string { return "csv" }

// ContentType implements port.TranslationCodec.
func (CSV) ContentType() string { return "text/csv" }

// Encode implements port.TranslationCodec.
func (CSV) Encode(w io.Writer, f i18ntext.File) error {
	if _, err := io.WriteString(w, bom); err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	if err := cw.Write(csvHeader); err != nil {
		return err
	}
	for _, u := range f.Units {
		if err := cw.Write([]string{u.Key, u.Source, u.Target, u.Note}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// Decode implements port.TranslationCodec.
func (CSV) Decode(r io.Reader) (i18ntext.File, error) {
	br := bufio.NewReader(io.LimitReader(r, MaxFileBytes+1))
	if p, err := br.Peek(3); err == nil && string(p) == bom {
		_, _ = br.Discard(3)
	}
	cr := csv.NewReader(br)
	cr.FieldsPerRecord = -1
	head, err := cr.Read()
	if err != nil {
		return i18ntext.File{}, malformed("csv", err)
	}
	if len(head) < 3 || strings.TrimSpace(head[0]) != "key" || strings.TrimSpace(head[1]) != "source" || strings.TrimSpace(head[2]) != "target" {
		return i18ntext.File{}, malformed("csv", errors.New("header key,source,target,note expected"))
	}
	var out i18ntext.File
	for {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return i18ntext.File{}, malformed("csv", err)
		}
		if len(rec) < 3 {
			return i18ntext.File{}, malformed("csv", errors.New("a row has fewer than 3 columns"))
		}
		u := i18ntext.Unit{Key: rec[0], Source: rec[1], Target: rec[2]}
		if len(rec) > 3 {
			u.Note = rec[3]
		}
		out.Units = append(out.Units, u)
	}
	return out, nil
}
