package dto

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ContentVersionView is schema ContentVersion (transitionContentVersion, F025): the workflow state of one version of
// any learning content type; RowVersion is its ETag.
type ContentVersionView struct {
	ContentType           string        `json:"content_type"`
	ContentID             uuid.UUID     `json:"content_id"`
	ContentVersionID      uuid.UUID     `json:"content_version_id"`
	Version               int           `json:"version"`
	Status                string        `json:"status"`
	OwnerScope            string        `json:"owner_scope"`
	TenantID              *uuid.UUID    `json:"tenant_id"`
	Assigned              bool          `json:"assigned"`
	AuthorID              uuid.UUID     `json:"author_id"`
	ReviewerID            *uuid.UUID    `json:"reviewer_id,omitempty"`
	PublishedAt           *time.Time    `json:"published_at,omitempty"`
	UpdatedAt             time.Time     `json:"updated_at"`
	ContentLanguage       string        `json:"content_language"`
	TranslationStatus     *string       `json:"translation_status,omitempty"`
	AudioExemptReason     *string       `json:"audio_exempt_reason,omitempty"`
	AudioExemptApprovedBy *uuid.UUID    `json:"audio_exempt_approved_by,omitempty"`
	OutdatedFields        []string      `json:"outdated_fields,omitempty"`
	Warnings              []WarningView `json:"warnings,omitempty"`
	IsCurrent             bool          `json:"-"`
	RowVersion            int64         `json:"-"`
}

// WarningView is one non-blocking warning (a choice of a g1_3 question without audio, CONTENT_MODEL §8.1).
type WarningView struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ReviewTaskView is schema ContentReviewTask (listContentReviewTasks, F025 US2).
type ReviewTaskView struct {
	TaskID            uuid.UUID  `json:"task_id"`
	ContentType       string     `json:"content_type"`
	ContentID         uuid.UUID  `json:"content_id"`
	ContentVersionID  uuid.UUID  `json:"content_version_id"`
	Version           int        `json:"version"`
	Title             string     `json:"title,omitempty"`
	Lane              string     `json:"lane"`
	Status            string     `json:"status"`
	AuthorID          uuid.UUID  `json:"author_id"`
	ReviewerID        *uuid.UUID `json:"reviewer_id,omitempty"`
	SchoolID          *uuid.UUID `json:"school_id,omitempty"`
	TenantID          *uuid.UUID `json:"tenant_id,omitempty"`
	SubmittedAt       time.Time  `json:"submitted_at"`
	DueAt             *time.Time `json:"due_at,omitempty"`
	ResolvedAt        *time.Time `json:"resolved_at,omitempty"`
	AudioExemptReason string     `json:"audio_exempt_reason,omitempty"`
	// FieldComments are the per-field comments of a rejected task (F025 T046).
	FieldComments json.RawMessage `json:"field_comments,omitempty"`
}

// TranslationReport is TranslationJob.report.
type TranslationReport struct {
	Total   int                      `json:"total"`
	Updated int                      `json:"updated"`
	Skipped int                      `json:"skipped"`
	Errors  []TranslationReportError `json:"errors"`
}

// TranslationReportError is one line of TranslationJob.report.errors (≤ 500).
type TranslationReportError struct {
	Ref     string `json:"ref"`
	Message string `json:"message"`
}

// TranslationJobView is schema TranslationJob (F025 US5); ResultKey is the object of the export file.
type TranslationJobView struct {
	JobID       uuid.UUID          `json:"job_id"`
	Kind        string             `json:"kind"`
	Status      string             `json:"status"`
	DownloadURL string             `json:"download_url,omitempty"`
	ExpiresAt   *time.Time         `json:"expires_at,omitempty"`
	Report      *TranslationReport `json:"report,omitempty"`
	ResultKey   *string            `json:"-"`
	TenantID    *uuid.UUID         `json:"-"`
}
