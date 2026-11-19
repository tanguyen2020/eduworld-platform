package platformadmin

import (
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Reason limits of restore and merge (RestoreRequest, UserMergeCommand: 3–1000 characters, ADR-0041).
const (
	ReasonMin = 3
	ReasonMax = 1000
)

// ValidReason reports a reason of 3–1000 characters (trimmed).
func ValidReason(r string) bool {
	n := utf8.RuneCountInString(strings.TrimSpace(r))
	return n >= ReasonMin && utf8.RuneCountInString(r) <= ReasonMax
}

// Merge conflicts (409 CONFLICT, F045-AC-31).
var (
	ErrMergeSameUser        = errors.New("platformadmin: an account cannot be merged into itself")
	ErrMergeUserType        = errors.New("platformadmin: both accounts must have the same user type")
	ErrMergeAlreadyMerged   = errors.New("platformadmin: the source account was already merged")
	ErrMergeAnonymized      = errors.New("platformadmin: an anonymized account cannot be merged")
	ErrMergeIssueNotPending = errors.New("platformadmin: the reconciliation issue is not a pending merge request")
)

// Issue types a merge can resolve (F011) and the resolution a School Admin sets to propose it.
const (
	IssueDuplicateIdentifier = "duplicate_identifier"
	IssuePossibleDuplicate   = "possible_duplicate"
	ResolutionMergeRequested = "merge_requested"
	ResolutionMerged         = "merged"
)

// MergeAccount is what a merge needs to know about one account.
type MergeAccount struct {
	UserID     uuid.UUID
	UserType   string
	Status     string
	Anonymized bool
	MergedAway bool // already the source of a merge (identity.user_merges.from_user_id)
}

// MergeIssue is the reconciliation issue that proposed the merge.
type MergeIssue struct {
	Type       string
	Status     string
	Resolution string
}

// CheckMerge applies the invariants of IDENTITY_MAPPING §7 / F045-AC-31 (the caller checks the memberships in
// the target tenant first: a missing one is 404).
func CheckMerge(from, into MergeAccount, issue MergeIssue) error {
	switch {
	case from.UserID == into.UserID:
		return ErrMergeSameUser
	case from.UserType != into.UserType:
		return ErrMergeUserType
	case from.MergedAway:
		return ErrMergeAlreadyMerged
	case from.Anonymized || into.Anonymized || from.Status == "anonymized" || into.Status == "anonymized":
		return ErrMergeAnonymized
	case (issue.Type != IssueDuplicateIdentifier && issue.Type != IssuePossibleDuplicate) || issue.Status != "open" ||
		issue.Resolution != ResolutionMergeRequested:
		return ErrMergeIssueNotPending
	}
	return nil
}

// Merge saga step outcome statuses exposed by getUserMerge (UserMergeJob.steps[].status).
func MergeStepStatus(sagaStepStatus string) string {
	switch sagaStepStatus {
	case "done", "nothing_to_do", "blocked", "failed":
		return sagaStepStatus
	case "compensated":
		return "done"
	default:
		return "pending"
	}
}

// MergeStatus maps a saga run status to UserMergeJob.status (blocked is a failed merge, F045-AC-29).
func MergeStatus(sagaStatus string) string {
	switch sagaStatus {
	case "succeeded":
		return "succeeded"
	case "blocked", "failed":
		return "failed"
	default:
		return "running"
	}
}
