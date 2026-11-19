package contentmodel

import (
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Language of the content (ADR-0056, CONTENT_MODEL §8).
type Language string

// Content languages.
const (
	LangVI Language = "vi"
	LangEN Language = "en"
)

// Limits of the metadata (CONTENT_MODEL §3, DATABASE_DESIGN §1.4).
const (
	MaxTags      = 20
	MaxTagLen    = 64
	MaxNoteLen   = 500
	MaxObjective = 50
)

// ContentMeta is the mandatory metadata of every content version (content_meta, CONTENT_MODEL §3).
type ContentMeta struct {
	GradeLevels          []int
	SubjectID            uuid.UUID
	TopicID              *uuid.UUID
	ObjectiveIDs         []uuid.UUID
	Difficulty           *int
	VersionNo            int
	Status               Status
	AuthorID             uuid.UUID
	ReviewerID           *uuid.UUID
	PublishedAt          *time.Time
	Owner                Owner
	ContentLanguage      Language
	CurriculumVersionID  *uuid.UUID
	Tags                 []string
	EstimatedDurationSec *int
	ChangeNote           string
}

// Violation is one invalid or missing field (errors[] of 422 VALIDATION_FAILED).
type Violation struct {
	Field    string
	Required bool // missing (sub-code REQUIRED) rather than invalid (INVALID)
	Reason   string
}

// Violations lists every invalid field of a validation (F020-AC-07: each missing field is reported).
type Violations []Violation

func (v Violations) Error() string {
	parts := make([]string, len(v))
	for i, x := range v {
		parts[i] = x.Field + " " + x.Reason
	}
	return "contentmodel: " + strings.Join(parts, "; ")
}

// needsObjectives: question, question set and activity assess/teach learning objectives (CONTENT_MODEL §3).
func needsObjectives(t ContentType) bool {
	return t == TypeQuestion || t == TypeQuestionSet || t == TypeActivity
}

// Validate checks the metadata of a version of type t; it returns nil or Violations listing every field.
func (m ContentMeta) Validate(t ContentType) error {
	var v Violations
	req := func(field, reason string) { v = append(v, Violation{Field: field, Required: true, Reason: reason}) }
	bad := func(field, reason string) { v = append(v, Violation{Field: field, Reason: reason}) }

	if _, err := ParseContentType(string(t)); err != nil {
		bad("content_type", "is not a learning content type")
	}
	switch {
	case len(m.GradeLevels) == 0:
		req("grade_levels", "is required")
	case len(m.GradeLevels) > 9:
		bad("grade_levels", "has more than 9 grades")
	default:
		for _, g := range m.GradeLevels {
			if g < 1 || g > 9 {
				bad("grade_levels", "grades must be 1–9")
				break
			}
		}
	}
	if m.SubjectID == uuid.Nil {
		req("subject_id", "is required")
	}
	if m.TopicID == nil || *m.TopicID == uuid.Nil {
		req("topic_id", "is required")
	}
	if needsObjectives(t) && len(m.ObjectiveIDs) == 0 {
		req("objective_ids", "is required for "+string(t))
	}
	if len(m.ObjectiveIDs) > MaxObjective || slices.Contains(m.ObjectiveIDs, uuid.Nil) {
		bad("objective_ids", "must hold at most 50 ids")
	}
	switch {
	case t == TypeQuestion && m.Difficulty == nil:
		req("difficulty", "is required for question")
	case m.Difficulty != nil && (*m.Difficulty < 1 || *m.Difficulty > 5):
		bad("difficulty", "must be 1–5")
	}
	if m.VersionNo < 1 {
		req("version", "must be a positive version number")
	}
	if !m.Status.Valid() {
		req("status", "is required")
	}
	if m.AuthorID == uuid.Nil {
		req("author_id", "is required")
	}
	if m.Status.AtLeastApproved() && (m.ReviewerID == nil || *m.ReviewerID == uuid.Nil) {
		req("reviewer_id", "is required once approved")
	}
	if (m.Status == StatusPublished || m.Status == StatusArchived) && m.PublishedAt == nil {
		req("published_at", "is required once published")
	}
	if _, err := NewOwner(m.Owner.Scope, m.Owner.TenantID); err != nil {
		var e ErrInvalid
		if errors.As(err, &e) {
			bad(e.Field, e.Reason)
		}
	}
	switch m.ContentLanguage {
	case LangVI, LangEN:
	case "":
		req("content_language", "is required")
	default:
		bad("content_language", "must be vi or en")
	}
	if m.CurriculumVersionID == nil || *m.CurriculumVersionID == uuid.Nil {
		req("curriculum_version_id", "is required")
	}
	if len(m.Tags) > MaxTags {
		bad("tags", "has more than 20 tags")
	}
	for _, tag := range m.Tags {
		if tag == "" || utf8.RuneCountInString(tag) > MaxTagLen {
			bad("tags", "each tag must be 1–64 characters")
			break
		}
	}
	if m.EstimatedDurationSec != nil && *m.EstimatedDurationSec < 0 {
		bad("estimated_duration_sec", "must not be negative")
	}
	if utf8.RuneCountInString(m.ChangeNote) > MaxNoteLen {
		bad("change_note", "is longer than 500 characters")
	}
	if len(v) == 0 {
		return nil
	}
	return v
}
