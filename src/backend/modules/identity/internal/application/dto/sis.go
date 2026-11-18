package dto

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// SisIssue is a reconciliation issue (SisReconciliationIssue of the spec).
type SisIssue struct {
	IssueID         uuid.UUID         `json:"issue_id"`
	TenantID        uuid.UUID         `json:"tenant_id"`
	SchoolID        *uuid.UUID        `json:"school_id"`
	ConnectionID    *uuid.UUID        `json:"sis_connection_id"`
	IssueType       string            `json:"issue_type"`
	EntityRefs      map[string]string `json:"entity_refs"`
	SuggestedAction *string           `json:"suggested_action"`
	Status          string            `json:"status"`
	Resolution      *string           `json:"resolution"`
	ResolutionNote  *string           `json:"resolution_note"`
	ResolvedBy      *uuid.UUID        `json:"resolved_by"`
	ResolvedAt      *time.Time        `json:"resolved_at"`
	CreatedAt       time.Time         `json:"created_at"`
}

// SisIssueRefs converts stored refs to the string map of the API.
func SisIssueRefs(refs map[string]any) map[string]string {
	out := make(map[string]string, len(refs))
	for k, v := range refs {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	return out
}

// HideConflictingUser removes the account holding a duplicated identifier: only System Admin sees it
// (ACCOUNT_PROVISIONING §2, "định danh đã được dùng" for the tenant).
func (i *SisIssue) HideConflictingUser() {
	if i.IssueType == "duplicate_identifier" || i.IssueType == "cleaned_up_student_in_sis" {
		delete(i.EntityRefs, "user_id")
	}
}

// SisImport is the SisImport of the spec.
type SisImport struct {
	ImportID        uuid.UUID                 `json:"import_id"`
	SchoolID        uuid.UUID                 `json:"school_id"`
	Status          string                    `json:"status"`
	DryRun          bool                      `json:"dry_run"`
	EntityKinds     []string                  `json:"entity_kinds"`
	Diff            map[string]map[string]int `json:"diff,omitempty"`
	Errors          []SisRecordError          `json:"errors,omitempty"`
	UploadURL       *string                   `json:"upload_url,omitempty"`
	UploadExpiresAt *time.Time                `json:"upload_expires_at,omitempty"`
	SyncRunID       *uuid.UUID                `json:"sync_run_id,omitempty"`
	RowsTotal       *int                      `json:"rows_total,omitempty"`
	RowsApplied     *int                      `json:"rows_applied,omitempty"`
	RowsRejected    *int                      `json:"rows_rejected,omitempty"`
	IssueCount      *int                      `json:"reconciliation_issue_count,omitempty"`
	CreatedAt       time.Time                 `json:"created_at"`
}

// SisRecordError is a SyncRecordError of the spec.
type SisRecordError struct {
	EntityKind string    `json:"entity_kind,omitempty"`
	ExternalID *string   `json:"external_id"`
	RowNo      *int      `json:"row_no"`
	Column     *string   `json:"column_name"`
	Code       string    `json:"error_code"`
	Message    string    `json:"message,omitempty"`
	OccurredAt time.Time `json:"occurred_at"`
}

// NewSisImport maps an import (diff and counters from the stats of its processed run).
func NewSisImport(id, school uuid.UUID, status string, dryRun bool, kinds []string, runID *uuid.UUID, stats map[string]int, created time.Time) SisImport {
	out := SisImport{ImportID: id, SchoolID: school, Status: status, DryRun: dryRun, EntityKinds: kinds, SyncRunID: runID, CreatedAt: created}
	if runID == nil {
		return out
	}
	diff := map[string]int{}
	for k, v := range stats {
		if a, ok := strings.CutPrefix(k, "diff_"); ok {
			diff[a] = v
		}
	}
	if len(diff) > 0 && len(kinds) == 1 {
		out.Diff = map[string]map[string]int{kinds[0]: diff}
	}
	for key, dst := range map[string]**int{"rows_total": &out.RowsTotal, "rows_applied": &out.RowsApplied, "rows_rejected": &out.RowsRejected,
		"issues": &out.IssueCount} {
		v := stats[key]
		*dst = &v
	}
	return out
}
