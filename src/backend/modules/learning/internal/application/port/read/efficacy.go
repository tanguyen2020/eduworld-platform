package read

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
)

// StudyCohortRead is a cohort of a study with its participant count.
type StudyCohortRead struct {
	write.StudyCohortRow
	Participants int
}

// WaveSessionRead is a session of a wave with the counts of its participations (never a score).
type WaveSessionRead struct {
	write.WaveSessionRow
	Invited, Completed, Absent int
}

// EfficacyReadModel reads the efficacy study (F035 US2–US3, US6) with explicit SQL: G tables have no tenant; T tables
// filter the tenants given (CrossTenantScope or the audited bypass efficacy_study) and deleted_at.
type EfficacyReadModel interface {
	Agreements(ctx context.Context, tenants []uuid.UUID, status string) ([]write.AgreementRow, error)
	Studies(ctx context.Context, status string, tenants []uuid.UUID) ([]write.StudyRow, error)
	Study(ctx context.Context, id uuid.UUID) (*write.StudyRow, error)
	// StudyCohorts lists the cohorts of a study; tenants nil = every tenant of the study.
	StudyCohorts(ctx context.Context, studyID uuid.UUID, tenants []uuid.UUID) ([]StudyCohortRead, error)
	Waves(ctx context.Context, studyID uuid.UUID) ([]write.WaveRow, error)
	WaveSessions(ctx context.Context, waveID uuid.UUID) ([]WaveSessionRead, error)
	Report(ctx context.Context, studyID, reportID uuid.UUID) (*write.EfficacyReportRow, error)
}

// Rows of the efficacy study shared with the write side (one row shape per table).
type (
	AgreementRow      = write.AgreementRow
	StudyRow          = write.StudyRow
	WaveRow           = write.WaveRow
	EfficacyReportRow = write.EfficacyReportRow
)

// ParticipantRow is a participant of a study.
type ParticipantRow = write.ParticipantRow

// SnapshotRow is a mastery snapshot of a participant.
type SnapshotRow = write.SnapshotRow

// EfficacyAnswerRow is one graded item of an efficacy test.
type EfficacyAnswerRow struct {
	StudentID         uuid.UUID
	TenantID          uuid.UUID
	SchoolID          uuid.UUID // the cohort of the student in the study
	WaveKind          string
	Form              string
	QuestionVersionID uuid.UUID
	ObjectiveIDs      []uuid.UUID
	Correct           bool
	Score, Max        float64
}

// EfficacyUsageRow is the use of the product by a student in a window.
type EfficacyUsageRow struct {
	StudentID        uuid.UUID
	ActiveSeconds    float64
	Days             int
	Answers          int
	RapidGuesses     int
	PracticeAttempts int
}

// EfficacyRecommendationRow is a recommendation generated for a student in a window.
type EfficacyRecommendationRow struct {
	StudentID    uuid.UUID
	ObjectiveIDs []uuid.UUID
	Followed     bool
}

// EfficacyDataReader reads the inputs of the efficacy report and of the pseudonymous dataset (F035 US6, US7), every tenant
// of the study under the audited bypass efficacy_study; withdrawn and excluded participants are returned with their
// status (the report counts them as attrition, the dataset leaves them out); soft-deleted snapshots are never returned.
type EfficacyDataReader interface {
	StudyOf(ctx context.Context, id uuid.UUID) (*write.StudyRow, error)
	CohortsOf(ctx context.Context, studyID uuid.UUID) ([]StudyCohortRead, error)
	ParticipantsOf(ctx context.Context, studyID uuid.UUID) ([]ParticipantRow, error)
	WavesOf(ctx context.Context, studyID uuid.UUID) ([]write.WaveRow, error)
	// Answers lists the items of the latest graded attempt of every student in the active sessions of a wave.
	Answers(ctx context.Context, waveID uuid.UUID) ([]EfficacyAnswerRow, error)
	SnapshotsOf(ctx context.Context, studyID uuid.UUID) ([]SnapshotRow, error)
	// Usage aggregates the practice of the students (not exams) between from and to.
	Usage(ctx context.Context, tenant uuid.UUID, students []uuid.UUID, from, to time.Time) ([]EfficacyUsageRow, error)
	// Recommendations lists the recommendations generated for the students between from and to.
	Recommendations(ctx context.Context, tenant uuid.UUID, students []uuid.UUID, from, to time.Time) ([]EfficacyRecommendationRow, error)
}
