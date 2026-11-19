package dto

import "time"

// CurriculumFile is the tree of one curriculum version in an import/export file
// (specs/schemas/curriculum-import.schema.json). Row is the line of the element in the file (error report).
type CurriculumFile struct {
	FormatVersion int
	Curriculum    FileCurriculum
	Version       FileVersion
	Competencies  []FileCompetency
	SubjectGrades []FileSubjectGrade
}

// FileCurriculum is the curriculum header.
type FileCurriculum struct {
	Code      string
	Name      map[string]string
	Framework *string
	Row       int
}

// FileVersion is the version header.
type FileVersion struct {
	Label           string
	EffectiveFrom   *time.Time
	EffectiveTo     *time.Time
	ChangeNote      *string
	ContentLanguage string
	Row             int
}

// FileCompetency is a competency of the version.
type FileCompetency struct {
	Code        string
	Name        map[string]string
	Kind        string
	SubjectCode string
	Row         int
}

// FileSubjectGrade is a subject × grade and its topics.
type FileSubjectGrade struct {
	SubjectCode    string
	GradeLevel     int
	PeriodsPerYear *int
	Topics         []FileTopic
	Row            int
}

// FileTopic is a topic and its objectives.
type FileTopic struct {
	Code       string
	Name       string
	ParentCode string
	SortOrder  int
	Objectives []FileObjective
	Row        int
}

// FileObjective is a learning objective.
type FileObjective struct {
	Code              string
	Description       string
	BloomLevel        string
	SortOrder         int
	YccdRef           string
	CompetencyCodes   []string
	PrerequisiteCodes []string
	Row               int
}
