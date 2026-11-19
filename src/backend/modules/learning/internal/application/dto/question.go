package dto

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

// Question is a question item (content.questions).
type Question struct {
	ID                 uuid.UUID
	Owner              contentmodel.Owner
	CurrentVersionID   *uuid.UUID
	LatestVersionNo    int
	RowVersion         int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
	ClonedFromID       *uuid.UUID
	ClonedFromTenantID *uuid.UUID
	ClonedAt           *time.Time
	ClonedBy           *uuid.UUID
}

// QuestionVersion is a version of a question with its server-only parts (content.question_versions).
type QuestionVersion struct {
	ID                  uuid.UUID
	QuestionID          uuid.UUID
	VersionNo           int
	Status              contentmodel.Status
	Type                string
	Stem                json.RawMessage
	RenderPayload       json.RawMessage // stored: without option feedback
	AnswerKey           json.RawMessage
	ScoringRule         json.RawMessage
	Explanation         json.RawMessage
	OptionFeedback      json.RawMessage
	Hints               json.RawMessage
	WorkedExampleRef    json.RawMessage
	MathAssets          json.RawMessage
	MinResponseMS       *int
	MaxScore            float64
	ExamOnly            bool
	EfficacyInstrument  bool
	Tags                []string
	Difficulty          int
	GradeLevels         []int
	SubjectID           uuid.UUID
	TopicID             uuid.UUID
	CurriculumVersionID uuid.UUID
	MediaRefs           []uuid.UUID
	ObjectiveIDs        []uuid.UUID
	ContentLanguage     string
	AuthorID            uuid.UUID
	ReviewerID          *uuid.UUID
	PublishedAt         *time.Time
	ChangeNote          *string
	BasedOnID           *uuid.UUID
	ContentHash         []byte
	Owner               contentmodel.Owner
	RowVersion          int64
	UpdatedAt           time.Time
}

// QuestionAuthoringView is schema QuestionAuthoring (ContentVersion + QuestionAuthoringInput): the CMS view with the
// answer key, the option feedback merged back into the render payload and the hints (F022-AC-10).
type QuestionAuthoringView struct {
	ContentType         string          `json:"content_type"`
	ContentID           uuid.UUID       `json:"content_id"`
	ContentVersionID    uuid.UUID       `json:"content_version_id"`
	Version             int             `json:"version"`
	Status              string          `json:"status"`
	OwnerScope          string          `json:"owner_scope"`
	TenantID            *uuid.UUID      `json:"tenant_id"`
	Assigned            bool            `json:"assigned"`
	CurrentVersionID    *uuid.UUID      `json:"current_version_id,omitempty"`
	AuthorID            uuid.UUID       `json:"author_id"`
	ReviewerID          *uuid.UUID      `json:"reviewer_id,omitempty"`
	PublishedAt         *time.Time      `json:"published_at,omitempty"`
	UpdatedAt           time.Time       `json:"updated_at"`
	ContentLanguage     string          `json:"content_language"`
	Type                string          `json:"type"`
	Stem                json.RawMessage `json:"stem"`
	RenderPayload       json.RawMessage `json:"render_payload"`
	AnswerKey           json.RawMessage `json:"answer_key"`
	ScoringRule         json.RawMessage `json:"scoring_rule"`
	Explanation         json.RawMessage `json:"explanation,omitempty"`
	Hints               json.RawMessage `json:"hints,omitempty"`
	WorkedExampleRef    json.RawMessage `json:"worked_example_ref,omitempty"`
	MinResponseMS       *int            `json:"min_response_ms,omitempty"`
	MaxScore            float64         `json:"max_score"`
	MathAssets          json.RawMessage `json:"math_assets,omitempty"`
	Tags                []string        `json:"tags"`
	Difficulty          int             `json:"difficulty"`
	GradeLevels         []int           `json:"grade_levels"`
	SubjectID           uuid.UUID       `json:"subject_id"`
	TopicID             uuid.UUID       `json:"topic_id"`
	CurriculumVersionID uuid.UUID       `json:"curriculum_version_id"`
	ObjectiveIDs        []uuid.UUID     `json:"objective_ids"`
	MediaRefs           []uuid.UUID     `json:"media_refs"`
	ExamOnly            bool            `json:"exam_only"`
	EfficacyInstrument  bool            `json:"efficacy_instrument"`
	ChangeNote          *string         `json:"change_note,omitempty"`
}

func nonNilStrings(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

func nonNilRaw(v json.RawMessage) json.RawMessage {
	if len(v) == 0 || string(v) == "null" {
		return nil
	}
	return v
}

// QuestionJSON builds the CMS view; renderWithFeedback is the render payload with the option feedback merged back.
func QuestionJSON(q Question, v QuestionVersion, renderWithFeedback json.RawMessage, sessionTenant *uuid.UUID) QuestionAuthoringView {
	assigned := q.Owner.TenantID == nil && sessionTenant != nil
	out := QuestionAuthoringView{ContentType: "question", ContentID: q.ID, ContentVersionID: v.ID, Version: v.VersionNo, Status: string(v.Status),
		OwnerScope: string(q.Owner.Scope), TenantID: q.Owner.TenantID, Assigned: assigned, CurrentVersionID: q.CurrentVersionID, AuthorID: v.AuthorID,
		ReviewerID: v.ReviewerID, UpdatedAt: v.UpdatedAt.UTC(), ContentLanguage: v.ContentLanguage, Type: v.Type, Stem: v.Stem,
		RenderPayload: renderWithFeedback, AnswerKey: v.AnswerKey, ScoringRule: v.ScoringRule, Explanation: nonNilRaw(v.Explanation),
		Hints: nonNilRaw(v.Hints), WorkedExampleRef: nonNilRaw(v.WorkedExampleRef), MinResponseMS: v.MinResponseMS, MaxScore: v.MaxScore,
		MathAssets: nonNilRaw(v.MathAssets), Tags: nonNilStrings(v.Tags), Difficulty: v.Difficulty, GradeLevels: v.GradeLevels,
		SubjectID: v.SubjectID, TopicID: v.TopicID, CurriculumVersionID: v.CurriculumVersionID, ObjectiveIDs: nonNil(v.ObjectiveIDs),
		MediaRefs: nonNil(v.MediaRefs), ExamOnly: v.ExamOnly, EfficacyInstrument: v.EfficacyInstrument, ChangeNote: v.ChangeNote}
	if v.PublishedAt != nil {
		t := v.PublishedAt.UTC()
		out.PublishedAt = &t
	}
	return out
}
