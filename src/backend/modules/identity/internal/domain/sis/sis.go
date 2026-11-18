// Package sis holds the SIS sync rules of module identity (SIS_INTEGRATION §4–§6, ADR-0021, ADR-0046): the
// typed records of the contract DTOs, their canonical hash (idempotent apply), the staging action and the
// mapping of SIS record status to the user/enrollment status.
package sis

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
)

// Record kinds, in apply order (F011-AC-04: school → class → student → enrollment).
const (
	KindSchool     = "school"
	KindClass      = "class"
	KindStudent    = "student"
	KindEnrollment = "enrollment"
)

// ApplyOrder is the order in which kinds are applied.
var ApplyOrder = []string{KindSchool, KindClass, KindStudent, KindEnrollment}

// Staging actions.
const (
	ActionCreate     = "create"
	ActionUpdate     = "update"
	ActionNoop       = "noop"
	ActionDeactivate = "deactivate"
)

// Issue types of the conflict matrix (SIS_INTEGRATION §6.3, F011-AC-14).
const (
	IssueDuplicateIdentifier   = "duplicate_identifier"
	IssueMissingClass          = "missing_class"
	IssueClassSchoolMismatch   = "class_school_mismatch"
	IssueOrphanEnrollment      = "orphan_enrollment"
	IssueMultiSchoolEnrollment = "multi_school_enrollment"
	IssueUnmappedSchool        = "unmapped_school"
	IssueCleanedUpStudentInSIS = "cleaned_up_student_in_sis"
	IssueTransferConfirmation  = "transfer_confirmation"
	IssueDeactivationReview    = "deactivation_review"
	IssueGradeMismatch         = "grade_mismatch"
)

// Hash is the canonical SHA-256 of a record without its updated_at (re-importing the same data is a noop
// even when the file gives a new snapshot time, F011-AC-05).
func Hash(data map[string]any) string {
	clean := make(map[string]any, len(data))
	for k, v := range data {
		if k != "updated_at" {
			clean[k] = v
		}
	}
	b, _ := json.Marshal(sorted(clean))
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// sorted returns a value whose JSON encoding is canonical (encoding/json already sorts map keys; nested
// maps are normalized the same way).
func sorted(v any) any {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		m := make(map[string]any, len(t))
		for _, k := range keys {
			m[k] = sorted(t[k])
		}
		return m
	default:
		return v
	}
}

// ActionFor decides the staging action from the stored hash of the source key.
func ActionFor(found bool, storedHash, hash string) string {
	switch {
	case !found:
		return ActionCreate
	case storedHash == hash:
		return ActionNoop
	default:
		return ActionUpdate
	}
}

// StatusChange is the effect of a SIS record status on the user and the enrollment (F011-AC-23).
type StatusChange struct {
	UserStatus      string // "" = unchanged
	CloseEnrollment string // "" = keep; "ended" or "transferred"
}

// ErrUnknownStatus: a record status outside the contract enum (already rejected by the validator).
var ErrUnknownStatus = errors.New("sis: unknown record status")

// StatusEffect maps a SIS record status: inactive/graduated → deactivated + enrollment ended; suspended →
// locked; transferred_out → enrollment transferred; active → active.
func StatusEffect(recordStatus string) (StatusChange, error) {
	switch recordStatus {
	case "active":
		return StatusChange{UserStatus: "active"}, nil
	case "inactive", "graduated":
		return StatusChange{UserStatus: "deactivated", CloseEnrollment: "ended"}, nil
	case "suspended":
		return StatusChange{UserStatus: "locked"}, nil
	case "transferred_out":
		return StatusChange{CloseEnrollment: "transferred"}, nil
	}
	return StatusChange{}, ErrUnknownStatus
}

// Str returns a string field of a record ("" when absent).
func Str(d map[string]any, k string) string {
	if s, ok := d[k].(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

// Int returns an integer field of a record (0 when absent).
func Int(d map[string]any, k string) int {
	switch v := d[k].(type) {
	case int:
		return v
	case float64:
		return int(v)
	}
	return 0
}

// Date parses a YYYY-MM-DD field (nil when absent).
func Date(d map[string]any, k string) *time.Time {
	s := Str(d, k)
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return nil
	}
	return &t
}

// LoginUsername returns login.username of a student record.
func LoginUsername(d map[string]any) string {
	if l, ok := d["login"].(map[string]any); ok {
		if s, ok := l["username"].(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// MissingReviewRatio is the share of missing records above which deactivation needs a review
// (sis_missing_review_ratio, CONFIG_DEFAULTS; SIS_INTEGRATION §5) [Proposed].
const MissingReviewRatio = 0.05

// DeactivationReviewNeeded applies the missing_in_source rule (SIS_INTEGRATION §5, F011-AC-24): records
// missing from a full import are only marked; after two consecutive runs above the ratio a review issue
// is raised instead of deactivating.
func DeactivationReviewNeeded(missing, total int, previousRunAbove bool, ratio float64) (above, review bool) {
	if total == 0 {
		return false, false
	}
	above = float64(missing)/float64(total) > ratio
	return above, above && previousRunAbove
}

// NationalIDHash is the keyed hash of a national student id (EduWorld never stores the id itself,
// SIS_INTEGRATION §2): HMAC-SHA256 over the id without spaces, upper-cased. A keyless hash of a short
// numeric id could be reversed by enumeration, so no key means no hash (transfer matching off).
func NationalIDHash(key []byte, id string) string {
	id = strings.ToUpper(strings.Join(strings.Fields(id), ""))
	if len(key) == 0 || id == "" {
		return ""
	}
	m := hmac.New(sha256.New, key)
	m.Write([]byte(id))
	return hex.EncodeToString(m.Sum(nil))
}
