package dto

import "encoding/json"

// QuestionRow is one question of an import file (F024 US4) after parsing: the row number of its error report, the
// curriculum codes to resolve (CSV, Excel, QTI-lite) and the authoring JSON (QuestionAuthoringInput fields; ids are
// set directly by content_json).
type QuestionRow struct {
	Row    int
	Codes  *QuestionCodes
	Fields map[string]json.RawMessage
}

// QuestionCodes are the curriculum codes of a row (specs/schemas/import/questions-row.schema.json).
type QuestionCodes struct {
	Subject           string
	Grades            []int
	Curriculum        string
	CurriculumVersion int // 0 = current release
	Topic             string
	Objectives        []string
	Difficulty        int
	Language          string
	Tags              []string
}
