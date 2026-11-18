// Package enrollment holds the Enrollment aggregate (ADR-0060, IDENTITY_MAPPING §6): one student, one
// school, one class; at most one active enrollment; a transfer closes the current enrollment and opens
// the next one in the same transaction, keeping the history.
package enrollment

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Status of an enrollment.
type Status string

const (
	StatusActive      Status = "active"
	StatusTransferred Status = "transferred"
	StatusEnded       Status = "ended"
)

// Source of an enrollment.
const (
	SourceSIS            = "sis"
	SourceManual         = "manual"
	SourcePlatformCommon = "platform_common"
)

// Errors of the aggregate.
var (
	ErrNotActive   = errors.New("enrollment: not active")
	ErrSameClass   = errors.New("enrollment: already in this class")
	ErrBeforeStart = errors.New("enrollment: transfer date before the start of the current enrollment")
	ErrInvalid     = errors.New("enrollment: invalid placement")
)

// Placement is where a student is enrolled.
type Placement struct {
	TenantID uuid.UUID
	SchoolID uuid.UUID
	ClassID  uuid.UUID
}

// Enrollment is the aggregate.
type Enrollment struct {
	ID        uuid.UUID
	StudentID uuid.UUID
	Placement
	Status    Status
	StartDate time.Time
	EndDate   *time.Time
	Source    string
	Version   int64
}

// day truncates to a calendar date in UTC.
func day(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// Open creates an active enrollment.
func Open(id, student uuid.UUID, at Placement, start time.Time, source string) (*Enrollment, error) {
	if student == uuid.Nil || at.TenantID == uuid.Nil || at.SchoolID == uuid.Nil || at.ClassID == uuid.Nil {
		return nil, ErrInvalid
	}
	return &Enrollment{ID: id, StudentID: student, Placement: at, Status: StatusActive, StartDate: day(start), Source: source, Version: 1}, nil
}

// Close ends an active enrollment with status transferred or ended at date on.
func (e *Enrollment) Close(status Status, on time.Time) error {
	if e.Status != StatusActive {
		return ErrNotActive
	}
	if status != StatusTransferred && status != StatusEnded {
		return ErrInvalid
	}
	end := day(on)
	if end.Before(e.StartDate) {
		return ErrBeforeStart
	}
	e.Status, e.EndDate = status, &end
	return nil
}

// Kind of transfer.
type Kind string

const (
	KindClass  Kind = "class"  // same school (F010-AC-13)
	KindSchool Kind = "school" // other school, same tenant (F010-AC-14)
	KindTenant Kind = "tenant" // other tenant: membership changes (F010-AC-15)
)

// Transfer is the result of moving a student: the closed and the opened enrollment.
type Transfer struct {
	Kind   Kind
	Closed *Enrollment
	Opened *Enrollment
}

// TransferTo closes current (status transferred) and opens an enrollment at target on the same day.
func TransferTo(current *Enrollment, newID uuid.UUID, target Placement, on time.Time, source string) (Transfer, error) {
	if current.Status != StatusActive {
		return Transfer{}, ErrNotActive
	}
	if target == current.Placement {
		return Transfer{}, ErrSameClass
	}
	kind := KindClass
	switch {
	case target.TenantID != current.TenantID:
		kind = KindTenant
	case target.SchoolID != current.SchoolID:
		kind = KindSchool
	}
	if err := current.Close(StatusTransferred, on); err != nil {
		return Transfer{}, err
	}
	opened, err := Open(newID, current.StudentID, target, on, source)
	if err != nil {
		return Transfer{}, err
	}
	return Transfer{Kind: kind, Closed: current, Opened: opened}, nil
}
