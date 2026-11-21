// Package placement holds where a student belongs (ADR-0095 mục 2, IDENTITY_MAPPING §6): exactly one school of a tenant
// and one grade (org.students.school_id, grade_level). There is no class and no enrollment: the effective placement is a
// student with status 'active' that is not soft-deleted. Every change of tenant, school, grade or effective status is
// published as student.placement_changed (owner F010) by the application layer.
package placement

import (
	"errors"

	"github.com/google/uuid"
)

// Status of a placement (student.placement_changed.status).
const (
	StatusActive   = "active"   // org.students.status 'active' and not soft-deleted
	StatusInactive = "inactive" // deactivated, cleaned up, merged, soft-deleted
)

// Sources of a placement change (student.placement_changed.source).
const (
	SourceSIS            = "sis"
	SourceManual         = "manual"
	SourcePlatformCommon = "platform_common"
	SourcePlatformSaga   = "platform_saga"
	SourceLifecycle      = "lifecycle"
)

// Changed fields of student.placement_changed.changed_fields.
const (
	FieldTenant = "tenant_id"
	FieldSchool = "school_id"
	FieldGrade  = "grade_level"
	FieldStatus = "status"
)

// Errors of the placement rules.
var (
	ErrInvalid   = errors.New("placement: invalid placement")
	ErrUnchanged = errors.New("placement: the student is already placed there")
)

// MinGrade and MaxGrade bound grade_level (students_grade_chk).
const (
	MinGrade = 1
	MaxGrade = 9
)

// ValidGrade reports whether g is a grade of the platform (1…9).
func ValidGrade(g int) bool { return g >= MinGrade && g <= MaxGrade }

// Placement is the tenant, school, grade and effective status of a student.
type Placement struct {
	TenantID uuid.UUID
	SchoolID uuid.UUID
	Grade    int
	Active   bool
}

// Status is the event status of the placement.
func (p Placement) Status() string {
	if p.Active {
		return StatusActive
	}
	return StatusInactive
}

// Validate checks the placement fields.
func (p Placement) Validate() error {
	if p.TenantID == uuid.Nil || p.SchoolID == uuid.Nil || !ValidGrade(p.Grade) {
		return ErrInvalid
	}
	return nil
}

// Record is a stored placement with the version of the org.students row and the current academic year of its school.
type Record struct {
	Placement
	Version        int64
	AcademicYearID *uuid.UUID
}

// Changed returns the changed fields from before to after; a first placement (before nil) changes every field.
func Changed(before *Placement, after Placement) []string {
	if before == nil {
		return []string{FieldTenant, FieldSchool, FieldGrade, FieldStatus}
	}
	var out []string
	if before.TenantID != after.TenantID {
		out = append(out, FieldTenant)
	}
	if before.SchoolID != after.SchoolID {
		out = append(out, FieldSchool)
	}
	if before.Grade != after.Grade {
		out = append(out, FieldGrade)
	}
	if before.Active != after.Active {
		out = append(out, FieldStatus)
	}
	return out
}

// Kind of a move.
type Kind string

const (
	KindGrade  Kind = "grade"  // same school, other grade (lên khối)
	KindSchool Kind = "school" // other school of the same tenant (F010-AC-14)
	KindTenant Kind = "tenant" // other tenant: the membership changes (F010-AC-15)
)

// Move checks a move of an effective student to target and returns its kind. The status is not part of a move: a
// student that is not effective cannot be moved (ErrInvalid), and moving to the same school and grade is ErrUnchanged.
func Move(current, target Placement) (Kind, error) {
	if !current.Active {
		return "", ErrInvalid
	}
	if err := target.Validate(); err != nil {
		return "", err
	}
	switch {
	case target.TenantID != current.TenantID:
		return KindTenant, nil
	case target.SchoolID != current.SchoolID:
		return KindSchool, nil
	case target.Grade != current.Grade:
		return KindGrade, nil
	}
	return "", ErrUnchanged
}
