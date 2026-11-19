package importer

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

// The import template of specs/examples (TEST FIXTURE shared with the portal).
func template(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "..", "..", "specs", "examples", "import", "questions.template.csv"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// F024-AC-13 (T024 golden): CSV, Excel, QTI-lite and JSON map to the same authoring rows; bad rows carry their number.
func TestQuestionParsers(t *testing.T) {
	csvData := template(t)
	rows, errs := Questions(KindQuestionsCSV, csvData)
	if len(errs) != 0 || len(rows) != 1 {
		t.Fatalf("csv: %v %v", rows, errs)
	}
	r := rows[0]
	if r.Row != 2 || r.Codes.Subject != "math" || r.Codes.Topic != "MATH.G3.T01" || r.Codes.Difficulty != 2 || len(r.Codes.Tags) != 2 ||
		!strings.Contains(string(r.Fields["answer_key"]), "o_b") {
		t.Fatalf("csv row %+v %+v", r, r.Codes)
	}
	bad := string(csvData) + strings.Replace(strings.Split(string(csvData), "\n")[1], `"2","vi"`, `"x","vi"`, 1) + "\n"
	rows, errs = Questions(KindQuestionsCSV, []byte(bad))
	if len(rows) != 1 || len(errs) != 1 || errs[0].Row != 3 || errs[0].Field != "difficulty" {
		t.Fatalf("bad row: %v %v", rows, errs)
	}
	if _, errs := Questions(KindQuestionsCSV, []byte("a,b\n")); len(errs) != 1 || errs[0].Code != "INVALID_FORMAT" {
		t.Fatalf("no data: %v", errs)
	}
	// Excel: the same cells.
	f := excelize.NewFile()
	recs := strings.Split(strings.TrimSpace(string(csvData)), "\n")
	for i, line := range recs {
		cells := splitCSV(t, line)
		for j, c := range cells {
			name, _ := excelize.CoordinatesToCellName(j+1, i+1)
			_ = f.SetCellValue("Sheet1", name, c)
		}
	}
	var xb bytes.Buffer
	if err := f.Write(&xb); err != nil {
		t.Fatal(err)
	}
	xrows, errs := Questions(KindQuestionsXLSX, xb.Bytes())
	if len(errs) != 0 || len(xrows) != 1 || string(xrows[0].Fields["stem"]) != string(rows[0].Fields["stem"]) {
		t.Fatalf("xlsx: %v %v", xrows, errs)
	}
	if _, errs := Questions(KindQuestionsXLSX, []byte("not excel")); len(errs) != 1 || errs[0].Code != "INVALID_FORMAT" {
		t.Fatalf("bad xlsx: %v", errs)
	}
	// content_json: authoring objects with ids.
	jrows, errs := Questions(KindContentJSON, []byte(`{"questions":[{"type":"true_false","stem":{"blocks":[]}},{"type":"numeric"}]}`))
	if len(errs) != 0 || len(jrows) != 2 || jrows[1].Row != 2 || jrows[0].Codes != nil {
		t.Fatalf("json: %v %v", jrows, errs)
	}
	if _, errs := Questions("lessons_docx", nil); len(errs) != 1 || errs[0].Code != "IMPORT_KIND_NOT_SUPPORTED" {
		t.Fatalf("docx: %v", errs)
	}
}

func splitCSV(t *testing.T, line string) []string {
	t.Helper()
	rows, errs := Questions(KindQuestionsCSV, []byte(strings.Split(string(template(t)), "\n")[0]+"\n"+line+"\n"))
	_ = rows
	_ = errs
	var out []string
	in, cell := false, strings.Builder{}
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case c == '"' && in && i+1 < len(line) && line[i+1] == '"':
			cell.WriteByte('"')
			i++
		case c == '"':
			in = !in
		case c == ',' && !in:
			out = append(out, cell.String())
			cell.Reset()
		default:
			cell.WriteByte(c)
		}
	}
	return append(out, cell.String())
}

// F024 research R2: QTI-lite choice, order and text entry items with metadata.csv; other interactions are refused.
func TestQTILite(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	add := func(name, body string) {
		w, _ := zw.Create(name)
		_, _ = w.Write([]byte(body))
	}
	add("metadata.csv", "identifier,subject_code,grade_levels,curriculum_code,topic_code,objective_codes,difficulty\n"+
		"q1,math,3,GDPT,T1,LO1,2\nq2,math,3,GDPT,T1,LO1,3\nq3,math,3,GDPT,T1,LO1,1\nq4,math,3,GDPT,T1,LO1,1\n")
	add("q1.xml", `<assessmentItem identifier="q1"><responseDeclaration identifier="RESPONSE"><correctResponse><value>B</value></correctResponse></responseDeclaration>
		<itemBody><choiceInteraction maxChoices="1"><prompt>7 x 8 = ?</prompt><simpleChoice identifier="A">54</simpleChoice><simpleChoice identifier="B">56</simpleChoice></choiceInteraction></itemBody></assessmentItem>`)
	add("q2.xml", `<assessmentItem identifier="q2"><responseDeclaration identifier="RESPONSE"><correctResponse><value>S1</value><value>S2</value><value>S3</value></correctResponse></responseDeclaration>
		<itemBody><orderInteraction><prompt>Sắp xếp</prompt><simpleChoice identifier="S2">b</simpleChoice><simpleChoice identifier="S1">a</simpleChoice><simpleChoice identifier="S3">c</simpleChoice></orderInteraction></itemBody></assessmentItem>`)
	add("q3.xml", `<assessmentItem identifier="q3"><responseDeclaration identifier="R1"><correctResponse><value>Hà Nội</value></correctResponse></responseDeclaration>
		<itemBody><p>Thủ đô là</p><textEntryInteraction responseIdentifier="R1"/></itemBody></assessmentItem>`)
	add("q4.xml", `<assessmentItem identifier="q4"><itemBody><extendedTextInteraction responseIdentifier="R"/></itemBody></assessmentItem>`)
	_ = zw.Close()
	rows, errs := Questions(KindQTILite, buf.Bytes())
	if len(rows) != 3 || len(errs) != 1 || errs[0].Row != 4 || errs[0].Code != "QTI_NOT_SUPPORTED" {
		t.Fatalf("qti: %d rows, %v", len(rows), errs)
	}
	types := []string{}
	for _, r := range rows {
		var typ string
		_ = json.Unmarshal(r.Fields["type"], &typ)
		types = append(types, typ)
	}
	if strings.Join(types, ",") != "single_choice,ordering,fill_in_blank" || !strings.Contains(string(rows[0].Fields["answer_key"]), `"o_b"`) ||
		!strings.Contains(string(rows[1].Fields["answer_key"]), `["i_s1","i_s2","i_s3"]`) || !strings.Contains(string(rows[2].Fields["stem"]), "{{blank:b_r1}}") {
		t.Fatalf("qti mapping: %v %s %s %s", types, rows[0].Fields["answer_key"], rows[1].Fields["answer_key"], rows[2].Fields["stem"])
	}
	if _, errs := Questions(KindQTILite, []byte("x")); len(errs) != 1 {
		t.Fatalf("not zip: %v", errs)
	}
}
