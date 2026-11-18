package dto

import (
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
