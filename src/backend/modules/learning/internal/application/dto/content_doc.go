package dto

import (
	"encoding/json"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/snapshot"
)

func present(r json.RawMessage) bool { return len(r) > 0 && string(r) != "null" }

// QuestionRender is the authoring render payload of a version (option feedback merged back, F022-AC-10).
func QuestionRender(v QuestionVersion) json.RawMessage {
	var fb map[string]question.OptionFeedback
	if present(v.OptionFeedback) {
		if err := json.Unmarshal(v.OptionFeedback, &fb); err != nil {
			return v.RenderPayload
		}
	}
	r, err := question.MergeFeedback(v.RenderPayload, fb)
	if err != nil {
		return v.RenderPayload
	}
	return r
}

// QuestionDoc is the document of a question version at the pointers of QuestionAuthoringInput: stem, render payload
// (option feedback merged back), explanation and hints — the texts of F025; with answers, also type, answer key and
// scoring rule (CMS read and diff of F026).
func QuestionDoc(v QuestionVersion, answers bool) json.RawMessage {
	doc := map[string]json.RawMessage{"stem": v.Stem, "render_payload": QuestionRender(v)}
	if present(v.Explanation) {
		doc["explanation"] = v.Explanation
	}
	if present(v.Hints) {
		doc["hints"] = v.Hints
	}
	if answers {
		t, _ := json.Marshal(v.Type)
		doc["type"] = t
		doc["answer_key"], doc["scoring_rule"] = v.AnswerKey, v.ScoringRule
		if present(v.MathAssets) {
			doc["math_assets"] = v.MathAssets
		}
		meta, _ := json.Marshal(map[string]any{"difficulty": v.Difficulty, "grade_levels": v.GradeLevels, "tags": v.Tags,
			"objective_ids": v.ObjectiveIDs, "max_score": v.MaxScore})
		doc["meta"] = meta
	}
	b, _ := json.Marshal(doc)
	return b
}

// DocDoc is the document of a composed content version: title, summary and the body keys of its type.
func DocDoc(v DocVersion) json.RawMessage {
	doc := map[string]json.RawMessage{}
	_ = json.Unmarshal(v.Body, &doc)
	doc["title"] = v.Title
	if present(v.Summary) {
		doc["summary"] = v.Summary
	}
	b, _ := json.Marshal(doc)
	return b
}

// Snapshot is the immutable published form of a version (F026 US3): no answer data (snapshot.Strip), served by the
// VersionReader and cached in Valkey ew:learning:cv:<item_type>:<item_id>:<version_no>.
type Snapshot struct {
	ItemType        string          `json:"item_type"`
	ItemID          uuid.UUID       `json:"item_id"`
	VersionID       uuid.UUID       `json:"version_id"`
	Version         int             `json:"version"`
	ContentLanguage string          `json:"content_language"`
	GradeLevels     []int           `json:"grade_levels"`
	QuestionType    string          `json:"question_type,omitempty"`
	Content         json.RawMessage `json:"content"`
}

// BuildSnapshot returns the snapshot bytes of a version and their SHA-256: doc is stripped of answer data.
func BuildSnapshot(t contentmodel.ContentType, itemID, versionID uuid.UUID, versionNo int, lang string, grades []int, qtype string,
	doc json.RawMessage) ([]byte, string, error) {
	content, err := snapshot.Strip(doc)
	if err != nil {
		return nil, "", err
	}
	if grades == nil {
		grades = []int{}
	}
	b, err := json.Marshal(Snapshot{ItemType: string(t), ItemID: itemID, VersionID: versionID, Version: versionNo, ContentLanguage: lang,
		GradeLevels: grades, QuestionType: qtype, Content: content})
	if err != nil {
		return nil, "", err
	}
	return b, snapshot.SHA256(b), nil
}

// QuestionSnapshotDoc is the document of a question snapshot: the stored render payload (never the feedback) with the
// stem and the rendered formulas.
func QuestionSnapshotDoc(v QuestionVersion) json.RawMessage {
	doc := map[string]json.RawMessage{"stem": v.Stem, "render_payload": v.RenderPayload}
	if present(v.MathAssets) {
		doc["math_assets"] = v.MathAssets
	}
	b, _ := json.Marshal(doc)
	return b
}
