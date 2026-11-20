// Package export writes the export files of F044 US1 (DASHBOARD_DATA §10, SCHOOL_LMS §10): CSV (UTF-8 with BOM), XLSX
// (excelize stream writer) and PDF (go-pdf/fpdf with the Be Vietnam Pro font of design/fonts, OFL), each with the watermark
// at the top and at the bottom and, when given, the disclaimer of the reference report at the top and at the bottom.
package export

import (
	"bytes"
	_ "embed"
	"encoding/csv"
	"fmt"

	"github.com/go-pdf/fpdf"
	"github.com/xuri/excelize/v2"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
)

// beVietnamPro is design/fonts/BeVietnamPro/BeVietnamPro-Regular.ttf (SIL Open Font License, see design/fonts/BeVietnamPro/LICENSE):
// the PDF needs a Unicode font for Vietnamese.
//
//go:embed fonts/BeVietnamPro-Regular.ttf
var beVietnamPro []byte

// Writers implements port.ReportWriters.
type Writers struct{}

var _ port.ReportWriters = Writers{}

// New implements port.ReportWriters.
func (Writers) New(format string, meta port.ReportMeta) (port.ReportWriter, error) {
	switch format {
	case "csv":
		return newCSV(meta), nil
	case "xlsx":
		return newXLSX(meta)
	case "pdf":
		return newPDF(meta), nil
	}
	return nil, fmt.Errorf("export: unknown format %q", format)
}

// preamble are the lines written before the header and after the last row.
func preamble(meta port.ReportMeta) []string {
	lines := []string{meta.Watermark}
	if meta.Title != "" {
		lines = append([]string{meta.Title}, lines...)
	}
	if meta.Disclaimer != "" {
		lines = append(lines, meta.Disclaimer)
	}
	return lines
}

func footer(meta port.ReportMeta) []string {
	lines := []string{}
	if meta.Disclaimer != "" {
		lines = append(lines, meta.Disclaimer)
	}
	return append(lines, meta.Watermark)
}

// ---------- CSV

type csvWriter struct {
	buf  bytes.Buffer
	w    *csv.Writer
	meta port.ReportMeta
}

func newCSV(meta port.ReportMeta) *csvWriter {
	c := &csvWriter{meta: meta}
	c.buf.Write([]byte{0xef, 0xbb, 0xbf}) // UTF-8 BOM for spreadsheet tools
	c.w = csv.NewWriter(&c.buf)
	for _, l := range preamble(meta) {
		_ = c.w.Write([]string{"# " + l})
	}
	return c
}

func (c *csvWriter) Header(cols []string) error { return c.w.Write(cols) }
func (c *csvWriter) Row(v []string) error       { return c.w.Write(v) }
func (c *csvWriter) Close() ([]byte, error) {
	for _, l := range footer(c.meta) {
		if err := c.w.Write([]string{"# " + l}); err != nil {
			return nil, err
		}
	}
	c.w.Flush()
	return c.buf.Bytes(), c.w.Error()
}

// ---------- XLSX

type xlsxWriter struct {
	f    *excelize.File
	sw   *excelize.StreamWriter
	row  int
	meta port.ReportMeta
}

const sheet = "Report"

func newXLSX(meta port.ReportMeta) (*xlsxWriter, error) {
	f := excelize.NewFile()
	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		return nil, err
	}
	// The watermark is also the page header/footer when the sheet is printed.
	if err := f.SetHeaderFooter(sheet, &excelize.HeaderFooterOptions{OddHeader: "&L" + escapeHF(meta.Watermark),
		OddFooter: "&L" + escapeHF(meta.Watermark)}); err != nil {
		return nil, err
	}
	sw, err := f.NewStreamWriter(sheet)
	if err != nil {
		return nil, err
	}
	x := &xlsxWriter{f: f, sw: sw, meta: meta}
	for _, l := range preamble(meta) {
		if err := x.write([]string{l}); err != nil {
			return nil, err
		}
	}
	return x, nil
}

func escapeHF(s string) string {
	return string(bytes.ReplaceAll([]byte(s), []byte("&"), []byte("&&")))
}

func (x *xlsxWriter) write(v []string) error {
	x.row++
	cell, err := excelize.CoordinatesToCellName(1, x.row)
	if err != nil {
		return err
	}
	vals := make([]any, len(v))
	for i, s := range v {
		vals[i] = s
	}
	return x.sw.SetRow(cell, vals)
}

func (x *xlsxWriter) Header(cols []string) error { return x.write(cols) }
func (x *xlsxWriter) Row(v []string) error       { return x.write(v) }
func (x *xlsxWriter) Close() ([]byte, error) {
	defer func() { _ = x.f.Close() }()
	for _, l := range footer(x.meta) {
		if err := x.write([]string{l}); err != nil {
			return nil, err
		}
	}
	if err := x.sw.Flush(); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := x.f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ---------- PDF (landscape A4 table; the watermark is on every page)

type pdfWriter struct {
	pdf    *fpdf.Fpdf
	meta   port.ReportMeta
	widths []float64
}

const pdfFont = "BeVietnamPro"

func newPDF(meta port.ReportMeta) *pdfWriter {
	p := fpdf.New("L", "mm", "A4", "")
	p.AddUTF8FontFromBytes(pdfFont, "", beVietnamPro)
	p.SetFont(pdfFont, "", 8)
	p.SetHeaderFunc(func() {
		p.SetFont(pdfFont, "", 7)
		p.CellFormat(0, 5, meta.Watermark, "", 1, "L", false, 0, "")
		p.SetFont(pdfFont, "", 8)
	})
	p.SetFooterFunc(func() {
		p.SetY(-12)
		p.SetFont(pdfFont, "", 7)
		p.CellFormat(0, 5, fmt.Sprintf("%s — %d", meta.Watermark, p.PageNo()), "", 0, "L", false, 0, "")
	})
	p.AddPage()
	if meta.Title != "" {
		p.SetFont(pdfFont, "", 12)
		p.MultiCell(0, 6, meta.Title, "", "L", false)
		p.SetFont(pdfFont, "", 8)
	}
	if meta.Disclaimer != "" {
		p.MultiCell(0, 4, meta.Disclaimer, "", "L", false)
	}
	p.Ln(2)
	return &pdfWriter{pdf: p, meta: meta}
}

func (w *pdfWriter) Header(cols []string) error {
	pageW, _ := w.pdf.GetPageSize()
	l, _, r, _ := w.pdf.GetMargins()
	each := (pageW - l - r) / float64(max(len(cols), 1))
	w.widths = make([]float64, len(cols))
	for i := range cols {
		w.widths[i] = each
	}
	return w.Row(cols)
}

func (w *pdfWriter) Row(v []string) error {
	for i, s := range v {
		width := 30.0
		if i < len(w.widths) {
			width = w.widths[i]
		}
		w.pdf.CellFormat(width, 5, s, "1", 0, "L", false, 0, "")
	}
	w.pdf.Ln(-1)
	return w.pdf.Error()
}

func (w *pdfWriter) Close() ([]byte, error) {
	w.pdf.Ln(2)
	if w.meta.Disclaimer != "" {
		w.pdf.MultiCell(0, 4, w.meta.Disclaimer, "", "L", false)
	}
	var buf bytes.Buffer
	if err := w.pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
