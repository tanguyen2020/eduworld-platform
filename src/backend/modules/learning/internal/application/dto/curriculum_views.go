package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/curriculum"
)

// JSON views of the curriculum API (specs/openapi/eduworld-api.v1.yaml schemas Curriculum, CurriculumVersion,
// Subject, Topic, LearningObjective, Competency, Textbook*, LoMappingList, ContentImportJob). Commands store them
// as the idempotent response (idempotency.SetResult) and handlers write them, so both are byte-identical.

// VersionRefView is CurriculumVersionRef.
type VersionRefView struct {
	CurriculumVersionID uuid.UUID `json:"curriculum_version_id"`
	Version             int       `json:"version"`
	VersionLabel        string    `json:"version_label"`
	Status              string    `json:"status"`
}

func refView(r *VersionRef) *VersionRefView {
	if r == nil {
		return nil
	}
	return &VersionRefView{CurriculumVersionID: r.ID, Version: r.VersionNo, VersionLabel: r.Label, Status: string(r.Status)}
}

// CurriculumView is schema Curriculum.
type CurriculumView struct {
	CurriculumID    uuid.UUID         `json:"curriculum_id"`
	Code            string            `json:"code"`
	Name            map[string]string `json:"name"`
	Framework       *string           `json:"framework,omitempty"`
	OwnerScope      string            `json:"owner_scope"`
	TenantID        *uuid.UUID        `json:"tenant_id"`
	CurrentVersion  *VersionRefView   `json:"current_version"`
	LatestVersion   *VersionRefView   `json:"latest_version,omitempty"`
	LatestVersionNo int               `json:"latest_version_no"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// CurriculumJSON maps a curriculum.
func CurriculumJSON(c Curriculum) CurriculumView {
	return CurriculumView{CurriculumID: c.ID, Code: c.Code, Name: c.Name, Framework: c.Framework, OwnerScope: string(c.Owner.Scope),
		TenantID: c.Owner.TenantID, CurrentVersion: refView(c.Current), LatestVersion: refView(c.Latest), LatestVersionNo: c.LatestVersionNo,
		CreatedAt: c.CreatedAt.UTC(), UpdatedAt: c.UpdatedAt.UTC()}
}

// SubjectGradeView is schema SubjectGrade.
type SubjectGradeView struct {
	SubjectID      uuid.UUID `json:"subject_id"`
	GradeLevel     int       `json:"grade_level"`
	PeriodsPerYear *int      `json:"periods_per_year,omitempty"`
}

// VersionView is schema CurriculumVersion.
type VersionView struct {
	CurriculumVersionID uuid.UUID            `json:"curriculum_version_id"`
	CurriculumID        uuid.UUID            `json:"curriculum_id"`
	Version             int                  `json:"version"`
	VersionLabel        string               `json:"version_label"`
	Status              string               `json:"status"`
	IsCurrent           bool                 `json:"is_current"`
	EffectiveFrom       *string              `json:"effective_from,omitempty"`
	EffectiveTo         *string              `json:"effective_to,omitempty"`
	SupersedesVersionID *uuid.UUID           `json:"supersedes_version_id,omitempty"`
	BasedOnVersionID    *uuid.UUID           `json:"based_on_version_id,omitempty"`
	ChangeNote          *string              `json:"change_note,omitempty"`
	ContentLanguage     string               `json:"content_language"`
	AuthorID            uuid.UUID            `json:"author_id"`
	ReviewerID          *uuid.UUID           `json:"reviewer_id,omitempty"`
	PublishedAt         *time.Time           `json:"published_at,omitempty"`
	SubjectGrades       []SubjectGradeView   `json:"subject_grades"`
	Warnings            []curriculum.Warning `json:"warnings,omitempty"`
	UpdatedAt           time.Time            `json:"updated_at"`
}

// DateString formats a calendar date (YYYY-MM-DD).
func DateString(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.DateOnly)
	return &s
}

// VersionJSON maps a version.
func VersionJSON(v CurriculumVersion) VersionView {
	out := VersionView{CurriculumVersionID: v.ID, CurriculumID: v.CurriculumID, Version: v.VersionNo, VersionLabel: v.Label, Status: string(v.Status),
		IsCurrent: v.IsCurrent, EffectiveFrom: DateString(v.EffectiveFrom), EffectiveTo: DateString(v.EffectiveTo), SupersedesVersionID: v.SupersedesID,
		BasedOnVersionID: v.BasedOnID, ChangeNote: v.ChangeNote, ContentLanguage: v.ContentLanguage, AuthorID: v.AuthorID, ReviewerID: v.ReviewerID,
		SubjectGrades: make([]SubjectGradeView, len(v.SubjectGrades)), Warnings: v.Warnings, UpdatedAt: v.UpdatedAt.UTC()}
	if v.PublishedAt != nil {
		t := v.PublishedAt.UTC()
		out.PublishedAt = &t
	}
	for i, sg := range v.SubjectGrades {
		out.SubjectGrades[i] = SubjectGradeView{SubjectID: sg.SubjectID, GradeLevel: sg.GradeLevel, PeriodsPerYear: sg.PeriodsPerYear}
	}
	return out
}

// SubjectView is schema Subject.
type SubjectView struct {
	SubjectID   uuid.UUID         `json:"subject_id"`
	Code        string            `json:"code"`
	Name        map[string]string `json:"name"`
	GradeLevels []int             `json:"grade_levels"`
	IconKey     *string           `json:"icon_key,omitempty"`
}

// SubjectJSON maps a subject.
func SubjectJSON(s Subject) SubjectView {
	return SubjectView{SubjectID: s.ID, Code: s.Code, Name: s.Name, GradeLevels: s.GradeLevels, IconKey: s.IconKey}
}

// TopicView is schema Topic.
type TopicView struct {
	TopicID             uuid.UUID  `json:"topic_id"`
	CurriculumVersionID uuid.UUID  `json:"curriculum_version_id"`
	ParentTopicID       *uuid.UUID `json:"parent_topic_id,omitempty"`
	Code                string     `json:"code"`
	Name                string     `json:"name"`
	SubjectID           uuid.UUID  `json:"subject_id"`
	GradeLevel          int        `json:"grade_level"`
	SortOrder           int        `json:"sort_order"`
	RowVersion          int64      `json:"row_version"` // ETag W/"<row_version>" for If-Match from a list (OQ F053)
}

// TopicJSON maps a topic.
func TopicJSON(t Topic) TopicView {
	return TopicView{TopicID: t.ID, CurriculumVersionID: t.VersionID, ParentTopicID: t.ParentID, Code: t.Code, Name: t.Name, SubjectID: t.SubjectID,
		GradeLevel: t.GradeLevel, SortOrder: t.SortOrder, RowVersion: t.RowVersion}
}

// ObjectiveView is schema LearningObjective.
type ObjectiveView struct {
	ObjectiveID         uuid.UUID   `json:"objective_id"`
	CurriculumVersionID uuid.UUID   `json:"curriculum_version_id"`
	Code                string      `json:"code"`
	StableKey           string      `json:"stable_key"`
	Description         string      `json:"description"`
	TopicID             uuid.UUID   `json:"topic_id"`
	SortOrder           int         `json:"sort_order"`
	BloomLevel          *string     `json:"bloom_level,omitempty"`
	YccdRef             *string     `json:"yccd_ref,omitempty"`
	CompetencyIDs       []uuid.UUID `json:"competency_ids"`
	PrerequisiteIDs     []uuid.UUID `json:"prerequisite_ids"`
	RowVersion          int64       `json:"row_version"` // ETag W/"<row_version>" for If-Match from a list (OQ F053)
}

func nonNil(v []uuid.UUID) []uuid.UUID {
	if v == nil {
		return []uuid.UUID{}
	}
	return v
}

// ObjectiveJSON maps an objective.
func ObjectiveJSON(o Objective) ObjectiveView {
	return ObjectiveView{ObjectiveID: o.ID, CurriculumVersionID: o.VersionID, Code: o.Code, StableKey: o.StableKey, Description: o.Description,
		TopicID: o.TopicID, SortOrder: o.SortOrder, BloomLevel: o.BloomLevel, YccdRef: o.YccdRef, CompetencyIDs: nonNil(o.CompetencyIDs),
		PrerequisiteIDs: nonNil(o.PrerequisiteIDs), RowVersion: o.RowVersion}
}

// CompetencyView is schema Competency.
type CompetencyView struct {
	CompetencyID        uuid.UUID         `json:"competency_id"`
	CurriculumVersionID uuid.UUID         `json:"curriculum_version_id"`
	Code                string            `json:"code"`
	Name                map[string]string `json:"name"`
	Kind                string            `json:"kind"`
	SubjectID           *uuid.UUID        `json:"subject_id,omitempty"`
}

// CompetencyJSON maps a competency.
func CompetencyJSON(c Competency) CompetencyView {
	return CompetencyView{CompetencyID: c.ID, CurriculumVersionID: c.VersionID, Code: c.Code, Name: c.Name, Kind: c.Kind, SubjectID: c.SubjectID}
}

// SeriesView is schema TextbookSeries.
type SeriesView struct {
	SeriesID  uuid.UUID `json:"series_id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	Publisher *string   `json:"publisher,omitempty"`
}

// SeriesJSON maps a series.
func SeriesJSON(s TextbookSeries) SeriesView {
	return SeriesView{SeriesID: s.ID, Code: s.Code, Name: s.Name, Publisher: s.Publisher}
}

// TextbookView is schema Textbook.
type TextbookView struct {
	TextbookID uuid.UUID `json:"textbook_id"`
	SeriesID   uuid.UUID `json:"series_id"`
	SubjectID  uuid.UUID `json:"subject_id"`
	GradeLevel int       `json:"grade_level"`
	Volume     int       `json:"volume"`
}

// TextbookJSON maps a textbook.
func TextbookJSON(t Textbook) TextbookView {
	return TextbookView{TextbookID: t.ID, SeriesID: t.SeriesID, SubjectID: t.SubjectID, GradeLevel: t.GradeLevel, Volume: t.Volume}
}

// SectionView is schema TextbookSection.
type SectionView struct {
	SectionID    uuid.UUID   `json:"section_id"`
	TextbookID   uuid.UUID   `json:"textbook_id"`
	Chapter      *string     `json:"chapter,omitempty"`
	LessonNo     int         `json:"lesson_no"`
	Title        string      `json:"title"`
	PageRange    *string     `json:"page_range,omitempty"`
	ObjectiveIDs []uuid.UUID `json:"objective_ids"`
}

// SectionJSON maps a section.
func SectionJSON(s TextbookSection) SectionView {
	return SectionView{SectionID: s.ID, TextbookID: s.TextbookID, Chapter: s.Chapter, LessonNo: s.LessonNo, Title: s.Title, PageRange: s.PageRange,
		ObjectiveIDs: nonNil(s.ObjectiveIDs)}
}

// LoMappingListView is schema LoMappingList.
type LoMappingListView struct {
	CurriculumVersionID uuid.UUID            `json:"curriculum_version_id"`
	SupersedesVersionID *uuid.UUID           `json:"supersedes_version_id,omitempty"`
	Items               []curriculum.Mapping `json:"items"`
	UnmappedLoIDs       []uuid.UUID          `json:"unmapped_lo_ids"`
}

// ImportJobView is schema ContentImportJob.
type ImportJobView struct {
	ImportID      uuid.UUID        `json:"import_id"`
	Kind          string           `json:"kind"`
	Status        string           `json:"status"`
	DryRun        bool             `json:"dry_run"`
	CreatedCount  int              `json:"created_count"`
	RejectedCount int              `json:"rejected_count"`
	Errors        []ImportRowError `json:"errors"`
	Result        map[string]any   `json:"result,omitempty"`
	CreatedAt     time.Time        `json:"created_at"`
	FinishedAt    *time.Time       `json:"finished_at,omitempty"`
}

// ImportJobJSON maps an import job.
func ImportJobJSON(j ImportJob) ImportJobView {
	errs := j.Errors
	if errs == nil {
		errs = []ImportRowError{}
	}
	out := ImportJobView{ImportID: j.ID, Kind: j.Kind, Status: j.Status, DryRun: j.DryRun, CreatedCount: j.CreatedCount,
		RejectedCount: j.RejectedCount, Errors: errs, CreatedAt: j.CreatedAt.UTC()}
	if len(j.Result) > 0 {
		out.Result = j.Result
	}
	if j.FinishedAt != nil {
		t := j.FinishedAt.UTC()
		out.FinishedAt = &t
	}
	return out
}
