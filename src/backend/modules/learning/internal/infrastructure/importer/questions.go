package importer

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
)

// Question import kinds (F024 US4).
const (
	KindQuestionsCSV  = "questions_csv"
	KindQuestionsXLSX = "questions_xlsx"
	KindQTILite       = "questions_qti_lite"
	KindContentJSON   = "content_json"
)

// MaxImportRows bounds the rows of one question import.
const MaxImportRows = 5000

var jsonColumns = []string{"stem", "render_payload", "answer_key", "scoring_rule", "explanation", "hints", "math_assets"}

func rowErr(row int, field, code, msg string) dto.ImportRowError {
	return dto.ImportRowError{Row: row, Field: field, Code: code, Message: msg}
}

// Questions parses a question import file of kind into rows (F024-AC-13, AC-15): a malformed file is one error of
// row 1 (INVALID_FORMAT); a malformed row is reported with its number and the parsing continues.
func Questions(kind string, data []byte) ([]dto.QuestionRow, []dto.ImportRowError) {
	switch kind {
	case KindQuestionsCSV:
		r := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))))
		r.FieldsPerRecord = -1
		recs, err := r.ReadAll()
		if err != nil {
			return nil, []dto.ImportRowError{rowErr(1, "", "INVALID_FORMAT", "the CSV cannot be read")}
		}
		return table(recs)
	case KindQuestionsXLSX:
		f, err := excelize.OpenReader(bytes.NewReader(data))
		if err != nil {
			return nil, []dto.ImportRowError{rowErr(1, "", "INVALID_FORMAT", "the Excel file cannot be read")}
		}
		defer func() { _ = f.Close() }()
		sheets := f.GetSheetList()
		if len(sheets) == 0 {
			return nil, []dto.ImportRowError{rowErr(1, "", "INVALID_FORMAT", "the Excel file has no sheet")}
		}
		recs, err := f.GetRows(sheets[0])
		if err != nil {
			return nil, []dto.ImportRowError{rowErr(1, "", "INVALID_FORMAT", "the first sheet cannot be read")}
		}
		return table(recs)
	case KindContentJSON:
		var doc struct {
			Questions []map[string]json.RawMessage `json:"questions"`
		}
		if err := json.Unmarshal(data, &doc); err != nil || len(doc.Questions) == 0 {
			return nil, []dto.ImportRowError{rowErr(1, "", "INVALID_FORMAT", `the file must be {"questions": [QuestionAuthoringInput…]}`)}
		}
		if len(doc.Questions) > MaxImportRows {
			return nil, []dto.ImportRowError{rowErr(1, "", "FILE_TOO_LARGE", "more than 5000 questions")}
		}
		out := make([]dto.QuestionRow, len(doc.Questions))
		for i, q := range doc.Questions {
			out[i] = dto.QuestionRow{Row: i + 1, Fields: q}
		}
		return out, nil
	case KindQTILite:
		return qtiLite(data)
	}
	return nil, []dto.ImportRowError{rowErr(1, "kind", "IMPORT_KIND_NOT_SUPPORTED", "unsupported import kind")}
}

// table maps a header row and data rows (row numbers count the header as row 1).
func table(recs [][]string) ([]dto.QuestionRow, []dto.ImportRowError) {
	if len(recs) < 2 {
		return nil, []dto.ImportRowError{rowErr(1, "", "INVALID_FORMAT", "the file has no data row")}
	}
	if len(recs)-1 > MaxImportRows {
		return nil, []dto.ImportRowError{rowErr(1, "", "FILE_TOO_LARGE", "more than 5000 rows")}
	}
	idx := map[string]int{}
	for i, h := range recs[0] {
		idx[strings.TrimSpace(strings.ToLower(h))] = i
	}
	for _, c := range []string{"type", "subject_code", "grade_levels", "curriculum_code", "topic_code", "objective_codes", "difficulty", "stem",
		"render_payload", "answer_key", "scoring_rule"} {
		if _, ok := idx[c]; !ok {
			return nil, []dto.ImportRowError{rowErr(1, c, "INVALID_FORMAT", "missing column "+c)}
		}
	}
	var out []dto.QuestionRow
	var errs []dto.ImportRowError
	for n, rec := range recs[1:] {
		row := n + 2
		get := func(c string) string {
			i, ok := idx[c]
			if !ok || i >= len(rec) {
				return ""
			}
			return strings.TrimSpace(rec[i])
		}
		if strings.Join(rec, "") == "" {
			continue
		}
		qr, err := rowOf(row, get)
		if err != nil {
			errs = append(errs, *err)
			continue
		}
		out = append(out, qr)
	}
	return out, errs
}

func rowOf(row int, get func(string) string) (dto.QuestionRow, *dto.ImportRowError) {
	codes := &dto.QuestionCodes{Subject: get("subject_code"), Curriculum: get("curriculum_code"), Topic: get("topic_code"), Language: get("content_language")}
	fail := func(field, msg string) (dto.QuestionRow, *dto.ImportRowError) {
		e := rowErr(row, field, "INVALID_VALUE", msg)
		return dto.QuestionRow{}, &e
	}
	for _, g := range splitList(get("grade_levels")) {
		n, err := strconv.Atoi(g)
		if err != nil {
			return fail("grade_levels", "must be grades 1–9 separated by |")
		}
		codes.Grades = append(codes.Grades, n)
	}
	codes.Objectives = splitList(get("objective_codes"))
	codes.Tags = splitList(get("tags"))
	d, err := strconv.Atoi(get("difficulty"))
	if err != nil {
		return fail("difficulty", "must be 1–5")
	}
	codes.Difficulty = d
	if v := get("curriculum_version"); v != "" {
		if codes.CurriculumVersion, err = strconv.Atoi(v); err != nil {
			return fail("curriculum_version", "must be a version number")
		}
	}
	fields := map[string]json.RawMessage{}
	t, _ := json.Marshal(get("type"))
	fields["type"] = t
	for _, c := range jsonColumns {
		v := get(c)
		if v == "" {
			continue
		}
		if !json.Valid([]byte(v)) {
			return fail(c, "is not valid JSON")
		}
		fields[c] = json.RawMessage(v)
	}
	if v := get("max_score"); v != "" {
		if _, err := strconv.ParseFloat(v, 64); err != nil {
			return fail("max_score", "must be a number")
		}
		fields["max_score"] = json.RawMessage(v)
	}
	return dto.QuestionRow{Row: row, Codes: codes, Fields: fields}, nil
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, "|") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ---------- QTI-lite (research F024 R2)

type qtiValue struct {
	Text string `xml:",chardata"`
}

type qtiItem struct {
	XMLName    xml.Name `xml:"assessmentItem"`
	Identifier string   `xml:"identifier,attr"`
	Responses  []struct {
		Identifier string     `xml:"identifier,attr"`
		Correct    []qtiValue `xml:"correctResponse>value"`
	} `xml:"responseDeclaration"`
	Body struct {
		Inner  []byte `xml:",innerxml"`
		Prompt []struct {
			Text string `xml:",chardata"`
		} `xml:"p"`
		Choice *struct {
			MaxChoices int    `xml:"maxChoices,attr"`
			Prompt     string `xml:"prompt"`
			Choices    []struct {
				ID   string `xml:"identifier,attr"`
				Text string `xml:",chardata"`
			} `xml:"simpleChoice"`
		} `xml:"choiceInteraction"`
		Order *struct {
			Prompt  string `xml:"prompt"`
			Choices []struct {
				ID   string `xml:"identifier,attr"`
				Text string `xml:",chardata"`
			} `xml:"simpleChoice"`
		} `xml:"orderInteraction"`
		Text *struct {
			ResponseIdentifier string `xml:"responseIdentifier,attr"`
		} `xml:"textEntryInteraction"`
		Extended *struct{} `xml:"extendedTextInteraction"`
	} `xml:"itemBody"`
}

func para(text string) json.RawMessage {
	b, _ := json.Marshal(map[string]any{"blocks": []any{map[string]any{"type": "paragraph", "text": strings.TrimSpace(text)}}})
	return b
}

// qtiID maps a QTI identifier to an itemId (^[a-z]_[a-z0-9_]{1,24}$).
func qtiID(prefix, id string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(id) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		}
	}
	s := b.String()
	if s == "" {
		s = "x"
	}
	if len(s) > 24 {
		s = s[:24]
	}
	return prefix + "_" + s
}

// qtiLite reads a zip of QTI 2.1 assessmentItem files plus metadata.csv (columns identifier + the code columns of
// questions-row.schema.json): choiceInteraction → single/multiple_choice, orderInteraction → ordering,
// textEntryInteraction → fill_in_blank; other interactions are rejected per item. responseProcessing is never run.
func qtiLite(data []byte) ([]dto.QuestionRow, []dto.ImportRowError) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, []dto.ImportRowError{rowErr(1, "", "INVALID_FORMAT", "the QTI-lite package is not a zip")}
	}
	files := map[string]*zip.File{}
	var names []string
	for _, f := range zr.File {
		files[f.Name] = f
		if strings.HasSuffix(strings.ToLower(f.Name), ".xml") {
			names = append(names, f.Name)
		}
	}
	slices.Sort(names)
	meta, ok := files["metadata.csv"]
	if !ok {
		return nil, []dto.ImportRowError{rowErr(1, "metadata.csv", "INVALID_FORMAT", "the package has no metadata.csv")}
	}
	recs, err := readCSV(meta)
	if err != nil || len(recs) < 2 {
		return nil, []dto.ImportRowError{rowErr(1, "metadata.csv", "INVALID_FORMAT", "metadata.csv cannot be read")}
	}
	idx := map[string]int{}
	for i, h := range recs[0] {
		idx[strings.TrimSpace(strings.ToLower(h))] = i
	}
	metaBy := map[string][]string{}
	for _, r := range recs[1:] {
		if i, ok := idx["identifier"]; ok && i < len(r) {
			metaBy[strings.TrimSpace(r[i])] = r
		}
	}
	var out []dto.QuestionRow
	var errs []dto.ImportRowError
	for n, name := range names {
		row := n + 1
		b, err := readAll(files[name])
		if err != nil {
			errs = append(errs, rowErr(row, path.Base(name), "INVALID_FORMAT", "cannot read the item"))
			continue
		}
		var it qtiItem
		if err := xml.Unmarshal(b, &it); err != nil {
			errs = append(errs, rowErr(row, path.Base(name), "INVALID_FORMAT", "not a QTI assessmentItem"))
			continue
		}
		rec, ok := metaBy[it.Identifier]
		if !ok {
			errs = append(errs, rowErr(row, "identifier", "INVALID_VALUE", "no metadata.csv row for "+it.Identifier))
			continue
		}
		get := func(c string) string {
			i, ok := idx[c]
			if !ok || i >= len(rec) {
				return ""
			}
			return strings.TrimSpace(rec[i])
		}
		qr, e := qtiRow(row, it, get)
		if e != nil {
			errs = append(errs, *e)
			continue
		}
		out = append(out, qr)
	}
	return out, errs
}

func readCSV(f *zip.File) ([][]string, error) {
	b, err := readAll(f)
	if err != nil {
		return nil, err
	}
	r := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(b, []byte("\xef\xbb\xbf"))))
	r.FieldsPerRecord = -1
	return r.ReadAll()
}

func readAll(f *zip.File) ([]byte, error) {
	if f.UncompressedSize64 > 1<<20 {
		return nil, errors.New("entry too large")
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	return io.ReadAll(io.LimitReader(rc, 1<<20))
}

func qtiRow(row int, it qtiItem, get func(string) string) (dto.QuestionRow, *dto.ImportRowError) {
	base, e := rowOf(row, func(c string) string {
		switch c {
		case "type", "stem", "render_payload", "answer_key", "scoring_rule":
			return ""
		}
		return get(c)
	})
	if e != nil {
		return base, e
	}
	fail := func(msg string) (dto.QuestionRow, *dto.ImportRowError) {
		er := rowErr(row, "itemBody", "QTI_NOT_SUPPORTED", msg)
		return dto.QuestionRow{}, &er
	}
	correct := map[string][]string{}
	for _, r := range it.Responses {
		for _, v := range r.Correct {
			correct[r.Identifier] = append(correct[r.Identifier], strings.TrimSpace(v.Text))
		}
	}
	rule := json.RawMessage(`{"mode":"all_or_nothing"}`)
	var typ string
	var render, key any
	stem := ""
	for _, p := range it.Body.Prompt {
		stem += " " + p.Text
	}
	switch {
	case it.Body.Choice != nil:
		c := it.Body.Choice
		if c.Prompt != "" {
			stem += " " + c.Prompt
		}
		var opts []any
		for _, ch := range c.Choices {
			opts = append(opts, map[string]any{"id": qtiID("o", ch.ID), "content": para(ch.Text)})
		}
		var ids []string
		for _, v := range correct["RESPONSE"] {
			ids = append(ids, qtiID("o", v))
		}
		if c.MaxChoices == 1 {
			if len(ids) != 1 {
				return fail("a single choice needs one correct response")
			}
			typ, render, key = "single_choice", map[string]any{"options": opts, "shuffle_options": true}, map[string]any{"correct_option_id": ids[0]}
		} else {
			typ, render, key = "multiple_choice", map[string]any{"options": opts, "shuffle_options": true, "show_correct_count": false},
				map[string]any{"correct_option_ids": ids}
		}
	case it.Body.Order != nil:
		o := it.Body.Order
		if o.Prompt != "" {
			stem += " " + o.Prompt
		}
		var items []any
		for _, ch := range o.Choices {
			items = append(items, map[string]any{"id": qtiID("i", ch.ID), "content": para(ch.Text)})
		}
		var order []string
		for _, v := range correct["RESPONSE"] {
			order = append(order, qtiID("i", v))
		}
		typ, render, key = "ordering", map[string]any{"items": items}, map[string]any{"order": order}
	case it.Body.Text != nil:
		id := it.Body.Text.ResponseIdentifier
		vals := correct[id]
		if len(vals) == 0 {
			return fail("the text entry has no correct response")
		}
		b := qtiID("b", id)
		typ, render = "fill_in_blank", map[string]any{"blanks": []any{map[string]any{"id": b, "input": "text", "max_length": 64}}}
		key = map[string]any{"blanks": map[string]any{b: map[string]any{"accepted": vals, "match": "normalized"}}}
	default:
		return fail("only choiceInteraction, orderInteraction and textEntryInteraction are supported")
	}
	stemDoc := para(stem)
	if typ == "fill_in_blank" {
		stemDoc = para(strings.TrimSpace(stem) + " {{blank:" + qtiID("b", it.Body.Text.ResponseIdentifier) + "}}")
	}
	base.Fields["type"] = mustJSON(typ)
	base.Fields["stem"] = stemDoc
	base.Fields["render_payload"] = mustJSON(render)
	base.Fields["answer_key"] = mustJSON(key)
	base.Fields["scoring_rule"] = rule
	return base, nil
}

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("importer: %v", err))
	}
	return b
}

// QuestionCodec implements command.QuestionImportCodec.
type QuestionCodec struct{}

// Questions implements command.QuestionImportCodec.
func (QuestionCodec) Questions(kind string, data []byte) ([]dto.QuestionRow, []dto.ImportRowError) {
	return Questions(kind, data)
}
