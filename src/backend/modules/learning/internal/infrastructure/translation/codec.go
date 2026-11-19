// Package translation is the file codec of the translation jobs of F025 US5: XLIFF 2.1 (one <unit> per text, the key
// as unit id) and CSV (key, source, target).
package translation

import (
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"errors"
	"fmt"
	"io"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
)

// Codec implements port.TranslationCodec.
type Codec struct{}

var _ port.TranslationCodec = Codec{}

// MaxUnits bounds a decoded file (500 items × their texts) [Proposed].
const MaxUnits = 20000

type xliff struct {
	XMLName xml.Name `xml:"urn:oasis:names:tc:xliff:document:2.0 xliff"`
	Version string   `xml:"version,attr"`
	SrcLang string   `xml:"srcLang,attr"`
	TrgLang string   `xml:"trgLang,attr"`
	File    struct {
		ID    string `xml:"id,attr"`
		Units []struct {
			ID      string `xml:"id,attr"`
			Segment struct {
				Source string `xml:"source"`
				Target string `xml:"target"`
			} `xml:"segment"`
		} `xml:"unit"`
	} `xml:"file"`
}

// ErrFormat is an unknown format or an unreadable file.
var ErrFormat = errors.New("translation: unreadable file")

// Encode implements port.TranslationCodec.
func (Codec) Encode(format, source, target string, units []port.TranslationUnit) ([]byte, string, error) {
	switch format {
	case "csv":
		var b bytes.Buffer
		w := csv.NewWriter(&b)
		_ = w.Write([]string{"key", source, target})
		for _, u := range units {
			_ = w.Write([]string{u.Key, u.Source, u.Target})
		}
		w.Flush()
		return b.Bytes(), "text/csv", w.Error()
	case "xliff":
		var x xliff
		x.Version, x.SrcLang, x.TrgLang, x.File.ID = "2.1", source, target, "learning"
		for _, u := range units {
			var unit = struct {
				ID      string `xml:"id,attr"`
				Segment struct {
					Source string `xml:"source"`
					Target string `xml:"target"`
				} `xml:"segment"`
			}{ID: u.Key}
			unit.Segment.Source, unit.Segment.Target = u.Source, u.Target
			x.File.Units = append(x.File.Units, unit)
		}
		b, err := xml.MarshalIndent(x, "", "  ")
		return append([]byte(xml.Header), b...), "application/xliff+xml", err
	}
	return nil, "", fmt.Errorf("%w: format %s", ErrFormat, format)
}

// Decode implements port.TranslationCodec.
func (Codec) Decode(format string, data []byte) ([]port.TranslationUnit, error) {
	var out []port.TranslationUnit
	switch format {
	case "csv":
		r := csv.NewReader(bytes.NewReader(data))
		r.FieldsPerRecord = 3
		header := true
		for {
			rec, err := r.Read()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("%w: %w", ErrFormat, err)
			}
			if header {
				header = false
				continue
			}
			out = append(out, port.TranslationUnit{Key: rec[0], Source: rec[1], Target: rec[2]})
		}
	case "xliff":
		var x xliff
		if err := xml.Unmarshal(data, &x); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrFormat, err)
		}
		for _, u := range x.File.Units {
			out = append(out, port.TranslationUnit{Key: u.ID, Source: u.Segment.Source, Target: u.Segment.Target})
		}
	default:
		return nil, fmt.Errorf("%w: format %s", ErrFormat, format)
	}
	if len(out) > MaxUnits {
		return nil, fmt.Errorf("%w: more than %d units", ErrFormat, MaxUnits)
	}
	return out, nil
}
