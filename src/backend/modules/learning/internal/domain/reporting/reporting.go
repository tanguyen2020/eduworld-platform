// Package reporting holds the rules of the asynchronous exports of F044 US1 (DASHBOARD_DATA §10, SCHOOL_LMS §10): the
// report types per route, the request checks, the limits, the job states, the storage key and the watermark. Pure Go.
package reporting

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
)

// Scope is the route that created the job.
type Scope string

// Scopes of a job (learning.report_jobs.scope_level).
const (
	ScopeLMS      Scope = "lms"
	ScopeTenant   Scope = "tenant"
	ScopePlatform Scope = "platform"
)

// Report types.
const (
	TypeAssignmentResults   = "assignment_results"
	TypeCourseResults       = "course_results"
	TypeRegulationReference = "regulation_reference"
	TypeSkillMap            = "skill_map"
	TypeSchoolMonthly       = "school_monthly"
	TypeLearningSummary     = "learning_summary"
	TypeClassProgress       = "class_progress"
	TypeGameSummary         = "game_summary"
	TypeEfficacyDataset     = "efficacy_dataset"
)

// Formats.
const (
	FormatCSV  = "csv"
	FormatXLSX = "xlsx"
	FormatPDF  = "pdf"
)

// Statuses of a job.
const (
	StatusQueued    = "queued"
	StatusRunning   = "running"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
	StatusExpired   = "expired"
)

// types are the report types a route serves in this release. game_summary belongs to F080 and efficacy_dataset to F035:
// the contract lists them, the request is refused until those features are delivered [Proposed].
var types = map[Scope][]string{
	ScopeLMS:      {TypeAssignmentResults, TypeCourseResults, TypeRegulationReference, TypeSkillMap, TypeSchoolMonthly},
	ScopeTenant:   {TypeLearningSummary, TypeClassProgress, TypeAssignmentResults},
	ScopePlatform: {TypeLearningSummary},
}

// Config holds the export parameters (CONFIG_DEFAULTS, owner DASHBOARD_DATA §10; a settings reader does not exist yet, the
// values are the registered defaults).
type Config struct {
	MaxRows              int64         // dashboard_export_max_rows
	MaxConcurrentPerUser int           // dashboard_export_max_concurrent_per_user
	LinkTTL              time.Duration // dashboard_export_link_ttl_minutes (= report_link_ttl)
	FileRetention        time.Duration // dashboard_export_file_retention_days
	MaxRangeDays         int           // dashboard_max_range_days
	MinCellSize          int           // analytics_min_cell_size
	BatchRows            int           // replica batch size (DASHBOARD_DATA §10)
	StaleAfter           time.Duration // a running job without progress is retried after this [Proposed]
	MaxAttempts          int           // runs of a job before it fails [Proposed]
}

// DefaultConfig returns the CONFIG_DEFAULTS values.
func DefaultConfig() Config {
	return Config{MaxRows: 1_000_000, MaxConcurrentPerUser: 2, LinkTTL: 15 * time.Minute, FileRetention: 7 * 24 * time.Hour,
		MaxRangeDays: 730, MinCellSize: 5, BatchRows: 10_000, StaleAfter: 30 * time.Minute, MaxAttempts: 3}
}

// Request is a validated export request.
type Request struct {
	Scope        Scope
	Type         string
	Format       string
	SchoolID     *uuid.UUID
	ClassID      *uuid.UUID
	CourseID     *uuid.UUID
	AssignmentID *uuid.UUID
	SubjectID    *uuid.UUID
	Period       string
	AcademicYear int // start year of the academic year of Period (0 = current)
	From, To     *time.Time
	SchoolIDs    []uuid.UUID
	TenantIDs    []uuid.UUID
	GradeLevel   *int
}

// FieldError is a request field that is missing or invalid (422 VALIDATION_FAILED).
type FieldError struct {
	Field, Reason string
	Required      bool
}

func (e *FieldError) Error() string { return e.Field + ": " + e.Reason }

// ErrRangeTooLarge: to − from exceeds dashboard_max_range_days (422 DASHBOARD_RANGE_TOO_LARGE).
var ErrRangeTooLarge = errors.New("reporting: range too large")

// ErrNotAvailable: the report type belongs to a later feature (422 VALIDATION_FAILED on report_type).
var ErrNotAvailable = errors.New("reporting: report type not available in this release")

func required(field string) error {
	return &FieldError{Field: field, Reason: "is required", Required: true}
}

// Validate checks the request against its route (F044-AC-07).
func (r Request) Validate(cfg Config) error {
	if !slices.Contains(types[r.Scope], r.Type) {
		if r.Type == TypeGameSummary || r.Type == TypeEfficacyDataset {
			return &FieldError{Field: "report_type", Reason: ErrNotAvailable.Error()}
		}
		return &FieldError{Field: "report_type", Reason: "is not a report type of this route"}
	}
	switch r.Format {
	case FormatCSV, FormatXLSX:
	case FormatPDF:
		if r.Scope != ScopeLMS {
			return &FieldError{Field: "format", Reason: "pdf is only available for LMS reports"}
		}
	default:
		return &FieldError{Field: "format", Reason: "must be csv, xlsx or pdf"}
	}
	if r.From != nil && r.To != nil {
		if r.To.Before(*r.From) {
			return &FieldError{Field: "to", Reason: "is before from"}
		}
		if int(r.To.Sub(*r.From).Hours()/24) > cfg.MaxRangeDays {
			return ErrRangeTooLarge
		}
	}
	need := func(ok bool, field string) error {
		if !ok {
			return required(field)
		}
		return nil
	}
	var errs []error
	switch r.Type {
	case TypeAssignmentResults:
		if r.Scope == ScopeLMS {
			errs = append(errs, need(r.AssignmentID != nil, "assignment_id"))
		}
	case TypeCourseResults:
		errs = append(errs, need(r.CourseID != nil, "course_id"))
	case TypeRegulationReference:
		errs = append(errs, need(r.SchoolID != nil, "school_id"), need(r.SubjectID != nil, "subject_id"), need(r.Period != "", "period"))
	case TypeSkillMap:
		errs = append(errs, need(r.ClassID != nil, "class_id"), need(r.SubjectID != nil, "subject_id"))
	case TypeSchoolMonthly:
		errs = append(errs, need(r.SchoolID != nil, "school_id"), need(r.From != nil, "from"), need(r.To != nil, "to"))
	}
	if r.Scope != ScopeLMS {
		errs = append(errs, need(r.From != nil, "from"), need(r.To != nil, "to"))
	}
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}

// Extension of a format.
func Extension(format string) string { return format }

// ContentType of a format.
func ContentType(format string) string {
	switch format {
	case FormatXLSX:
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case FormatPDF:
		return "application/pdf"
	}
	return "text/csv; charset=utf-8"
}

// StorageKey is the object key of a file in ew-exports: exports/<tenant_id|platform>/<report_id>.<ext>. The report id is a
// random UUIDv7 that only its creator reads (the key is never listed nor guessable from the link).
func StorageKey(tenant *uuid.UUID, reportID uuid.UUID, format string) string {
	owner := "platform"
	if tenant != nil {
		owner = tenant.String()
	}
	return fmt.Sprintf("exports/%s/%s.%s", owner, reportID, Extension(format))
}

// Watermark is the line written at the top (and bottom) of every file (F044-AC-03): who exported and when (UTC+7).
func Watermark(user uuid.UUID, at time.Time) string {
	z := time.FixedZone("ICT", 7*3600)
	return fmt.Sprintf("EduWorld — xuất bởi %s lúc %s — chỉ dùng nội bộ nhà trường", user, at.In(z).Format("2006-01-02 15:04 MST"))
}

// Active reports a job that counts against dashboard_export_max_concurrent_per_user.
func Active(status string) bool { return status == StatusQueued || status == StatusRunning }

// Downloadable reports whether a link may be issued at now.
func Downloadable(status string, expires *time.Time, now time.Time) bool {
	return status == StatusSucceeded && expires != nil && now.Before(*expires)
}
