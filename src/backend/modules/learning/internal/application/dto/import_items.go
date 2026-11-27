package dto

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/docximport"
)

// ImportItem is one question of a lessons_docx job (content.import_items, F028).
type ImportItem struct {
	ID                uuid.UUID
	ImportID          uuid.UUID
	Owner             contentmodel.Owner
	No                int
	Label             string
	Status            string
	QuestionType      *string
	StemPreview       string
	Issues            []docximport.Issue
	Formulas          []docximport.Formula
	Images            []docximport.ImageRef
	Payload           ImportPayload
	QuestionID        *uuid.UUID
	QuestionVersionID *uuid.UUID
	UpdatedAt         time.Time
}

// ImportPayload is what a draft is created from: the authoring fields of createQuestion and the curriculum codes.
type ImportPayload struct {
	Fields map[string]json.RawMessage `json:"fields"`
	Codes  docximport.Codes           `json:"codes"`
}

// ImportItemView is schema ContentImportItem.
type ImportItemView struct {
	ImportID          uuid.UUID          `json:"import_id"`
	ItemNo            int                `json:"item_no"`
	SourceLabel       string             `json:"source_label"`
	Status            string             `json:"status"`
	QuestionType      *string            `json:"question_type,omitempty"`
	StemPreview       string             `json:"stem_preview"`
	Issues            []docximport.Issue `json:"issues"`
	Formulas          []formulaView      `json:"formulas"`
	Images            []imageView        `json:"images"`
	Question          map[string]any     `json:"question,omitempty"`
	QuestionID        *uuid.UUID         `json:"question_id,omitempty"`
	QuestionVersionID *uuid.UUID         `json:"question_version_id,omitempty"`
	UpdatedAt         time.Time          `json:"updated_at"`
}

type formulaView struct {
	Latex   string `json:"latex"`
	Display bool   `json:"display"`
}

type imageView struct {
	Ref  string `json:"ref"`
	Alt  string `json:"alt"`
	MIME string `json:"mime,omitempty"`
}

// ImportItemJSON maps an item; the zip path of a picture stays on the server.
func ImportItemJSON(it ImportItem) ImportItemView {
	v := ImportItemView{ImportID: it.ImportID, ItemNo: it.No, SourceLabel: it.Label, Status: it.Status, QuestionType: it.QuestionType,
		StemPreview: it.StemPreview, Issues: it.Issues, Formulas: []formulaView{}, Images: []imageView{}, QuestionID: it.QuestionID,
		QuestionVersionID: it.QuestionVersionID, UpdatedAt: it.UpdatedAt.UTC()}
	if v.Issues == nil {
		v.Issues = []docximport.Issue{}
	}
	for _, f := range it.Formulas {
		v.Formulas = append(v.Formulas, formulaView(f))
	}
	for _, i := range it.Images {
		v.Images = append(v.Images, imageView{Ref: i.Ref, Alt: i.Alt, MIME: i.MIME})
	}
	if len(it.Payload.Fields) > 0 {
		q := map[string]any{}
		for k, raw := range it.Payload.Fields {
			q[k] = raw
		}
		c := it.Payload.Codes
		q["subject_code"], q["grade_levels"], q["curriculum_code"], q["topic_code"] = c.Subject, c.Grades, c.Curriculum, c.Topic
		q["objective_codes"], q["difficulty"], q["content_language"] = c.Objectives, c.Difficulty, c.Language
		v.Question = q
	}
	return v
}
