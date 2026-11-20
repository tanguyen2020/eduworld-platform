package dto

import (
	"time"

	"github.com/google/uuid"
)

// DashboardMeta is the metadata of every dashboard answer (DashboardMeta, DASHBOARD_DATA §8): the grain used, the oldest
// computed_at of the rows read and whether a provisional row (is_final = false) was read.
type DashboardMeta struct {
	Grain     string    `json:"grain"`
	DataAsOf  time.Time `json:"data_as_of"`
	IsPartial bool      `json:"is_partial"`
	From      string    `json:"from,omitempty"`
	To        string    `json:"to,omitempty"`
}

// AnalyticsPointView is AnalyticsPoint.
type AnalyticsPointView struct {
	PeriodStart string   `json:"period_start"`
	Value       *float64 `json:"value"`
	Suppressed  bool     `json:"suppressed,omitempty"`
	IsFinal     bool     `json:"is_final"`
}

// AssignmentClassProgress is one class of DashboardAssignmentProgress.by_class.
type AssignmentClassProgress struct {
	ClassID    uuid.UUID `json:"class_id"`
	Submitted  int       `json:"submitted"`
	Total      int       `json:"total"`
	NotStarted int       `json:"not_started"`
	Missed     int       `json:"missed"`
}

// AssignmentProgressView is DashboardAssignmentProgress.
type AssignmentProgressView struct {
	AssignmentID uuid.UUID                 `json:"assignment_id"`
	Title        string                    `json:"title"`
	SubjectID    uuid.UUID                 `json:"subject_id"`
	DueAt        time.Time                 `json:"due_at"`
	Status       string                    `json:"status"`
	Submitted    int                       `json:"submitted"`
	Total        int                       `json:"total"`
	NotStarted   int                       `json:"not_started"`
	Missed       int                       `json:"missed"`
	ByClass      []AssignmentClassProgress `json:"by_class"`
}

// DashboardClassView is one row of SchoolDashboard.widgets.classes.
type DashboardClassView struct {
	ClassID                  uuid.UUID `json:"class_id"`
	Name                     string    `json:"name,omitempty"`
	GradeLevel               int       `json:"grade_level,omitempty"`
	ActiveStudents           *int      `json:"active_students"`
	Accuracy                 *float64  `json:"accuracy"`
	AssignmentCompletionRate *float64  `json:"assignment_completion_rate"`
}

// WeakObjectiveView is one row of SchoolDashboard.widgets.weak_objectives.
type WeakObjectiveView struct {
	ObjectiveID          uuid.UUID   `json:"objective_id"`
	Code                 string      `json:"code"`
	YccdRef              string      `json:"yccd_ref,omitempty"`
	BelowProficientRatio float64     `json:"below_proficient_ratio"`
	ClassIDs             []uuid.UUID `json:"class_ids"`
}

// AttentionView is one student needing attention (reasons of the AttentionReason enum).
type AttentionView struct {
	StudentID uuid.UUID  `json:"student_id"`
	ClassID   *uuid.UUID `json:"class_id,omitempty"`
	Reasons   []string   `json:"reasons"`
}

// SelfStudyView is SchoolDashboard.widgets.self_study.
type SelfStudyView struct {
	AvgActiveMinutesPerStudent   *float64 `json:"avg_active_minutes_per_student"`
	RecommendationStartedRate    *float64 `json:"recommendation_started_rate"`
	ObjectivesImprovedAfterRecom int      `json:"objectives_improved_after_recommendation"`
}

// SchoolWidgets is SchoolDashboard.widgets: no XP, Game Rating, ranking nor chat field (F043-AC-11).
type SchoolWidgets struct {
	ActiveStudents                   int                      `json:"active_students"`
	AssignmentsOpen                  int                      `json:"assignments_open"`
	AssignmentsDueSoon               int                      `json:"assignments_due_soon"`
	AssignmentCompletionRate         *float64                 `json:"assignment_completion_rate"`
	Accuracy                         *float64                 `json:"accuracy"`
	RapidGuessRatio                  *float64                 `json:"rapid_guess_ratio"`
	SafetyNotificationsUnacknowledge *int                     `json:"safety_notifications_unacknowledged"`
	ActiveMinutesSeries              []AnalyticsPointView     `json:"active_minutes_series"`
	Assignments                      []AssignmentProgressView `json:"assignments"`
	Classes                          []DashboardClassView     `json:"classes"`
	WeakObjectives                   []WeakObjectiveView      `json:"weak_objectives"`
	StudentsNeedingAttention         []AttentionView          `json:"students_needing_attention"`
	StudentsNeedingAttentionTotal    int                      `json:"students_needing_attention_total"`
	SelfStudy                        SelfStudyView            `json:"self_study"`
}

// SchoolDashboardView is SchoolDashboard.
type SchoolDashboardView struct {
	DashboardMeta
	SchoolID  uuid.UUID     `json:"school_id"`
	ClassID   *uuid.UUID    `json:"class_id,omitempty"`
	SubjectID *uuid.UUID    `json:"subject_id,omitempty"`
	Widgets   SchoolWidgets `json:"widgets"`
}

// ObjectiveDistributionView is one column of the class heatmap.
type ObjectiveDistributionView struct {
	ObjectiveID          uuid.UUID      `json:"objective_id"`
	Code                 string         `json:"code"`
	YccdRef              string         `json:"yccd_ref,omitempty"`
	StudentsWithEvidence int            `json:"students_with_evidence"`
	BelowProficientRatio *float64       `json:"below_proficient_ratio"`
	LevelCounts          map[string]int `json:"level_counts"`
}

// ClassAttentionView is one row of ClassDashboard.students_needing_attention.
type ClassAttentionView struct {
	StudentID        uuid.UUID   `json:"student_id"`
	Reasons          []string    `json:"reasons"`
	WeakObjectiveIDs []uuid.UUID `json:"weak_objective_ids"`
}

// ClassStudentView is one student of the class list.
type ClassStudentView struct {
	StudentID           uuid.UUID `json:"student_id"`
	ActivitiesCompleted int       `json:"activities_completed"`
	LessonsCompleted    int       `json:"lessons_completed"`
	Accuracy            *float64  `json:"accuracy"`
	ActiveMinutes       int       `json:"active_minutes"`
	RapidGuessRatio     *float64  `json:"rapid_guess_ratio"`
	RapidGuessFlag      bool      `json:"rapid_guess_flag"`
	Reasons             []string  `json:"reasons"`
}

// PageView is PageInfo.
type PageView struct {
	NextCursor *string `json:"next_cursor"`
	HasMore    bool    `json:"has_more"`
	Limit      int     `json:"limit"`
}

// ClassStudentsPage is ClassDashboard.students.
type ClassStudentsPage struct {
	Data []ClassStudentView `json:"data"`
	Page PageView           `json:"page"`
}

// ClassDashboardView is ClassDashboard.
type ClassDashboardView struct {
	DashboardMeta
	ClassID                  uuid.UUID                   `json:"class_id"`
	SchoolID                 uuid.UUID                   `json:"school_id"`
	SubjectID                *uuid.UUID                  `json:"subject_id,omitempty"`
	GeneratedAt              time.Time                   `json:"generated_at"`
	ActiveStudents7d         int                         `json:"active_students_7d"`
	AssignmentsOpen          int                         `json:"assignments_open"`
	Accuracy                 *float64                    `json:"accuracy"`
	RapidGuessRatio          *float64                    `json:"rapid_guess_ratio"`
	Assignments              []AssignmentProgressView    `json:"assignments"`
	ObjectiveDistribution    []ObjectiveDistributionView `json:"objective_distribution"`
	StudentsNeedingAttention []ClassAttentionView        `json:"students_needing_attention"`
	Students                 ClassStudentsPage           `json:"students"`
}

// AnalyticsSeriesView is AnalyticsSeries.
type AnalyticsSeriesView struct {
	DashboardMeta
	Metric string               `json:"metric"`
	Unit   string               `json:"unit,omitempty"`
	Points []AnalyticsPointView `json:"points"`
}

// TopicMasteryView is one topic of a subject of the student profile.
type TopicMasteryView struct {
	TopicID   uuid.UUID `json:"topic_id"`
	TopicName string    `json:"topic_name,omitempty"`
	LevelDist []int     `json:"level_dist"`
}

// ProfileSubjectView is one subject of the student profile.
type ProfileSubjectView struct {
	SubjectID           uuid.UUID          `json:"subject_id"`
	Accuracy            *float64           `json:"accuracy"`
	ActiveMinutes       int                `json:"active_minutes"`
	ActivitiesCompleted int                `json:"activities_completed"`
	LessonsCompleted    int                `json:"lessons_completed"`
	MasteryByTopic      []TopicMasteryView `json:"mastery_by_topic"`
}

// TrendWeekView is one weekly mastery snapshot.
type TrendWeekView struct {
	WeekStart     string    `json:"week_start"`
	SubjectID     uuid.UUID `json:"subject_id"`
	MeanMastery   float64   `json:"mean_mastery"`
	MasteredCount int       `json:"mastered_count"`
}

// ProfileAssignmentView is one assignment of the history (no answer, F043-AC-24).
type ProfileAssignmentView struct {
	AssignmentID uuid.UUID  `json:"assignment_id"`
	Title        string     `json:"title,omitempty"`
	SubjectID    uuid.UUID  `json:"subject_id"`
	Status       string     `json:"status"`
	Score        *float64   `json:"score"`
	MaxScore     *float64   `json:"max_score"`
	DueAt        time.Time  `json:"due_at"`
	SubmittedAt  *time.Time `json:"submitted_at"`
}

// ProfileRecommendationView is one recommendation shown to the student.
type ProfileRecommendationView struct {
	RecommendationID uuid.UUID  `json:"recommendation_id"`
	RuleCode         string     `json:"rule_code"`
	ReasonCode       string     `json:"reason_code"`
	SubjectID        *uuid.UUID `json:"subject_id,omitempty"`
}

// StudentLearningProfileView is StudentLearningProfile: no Game Rating, game position, friends nor chat (F043-AC-24).
type StudentLearningProfileView struct {
	DashboardMeta
	StudentID             uuid.UUID                   `json:"student_id"`
	SchoolID              uuid.UUID                   `json:"school_id"`
	Subjects              []ProfileSubjectView        `json:"subjects"`
	TrendWeeks            []TrendWeekView             `json:"trend_weeks"`
	RecentAttempts        []ProfileAssignmentView     `json:"recent_attempts"`
	ActiveRecommendations []ProfileRecommendationView `json:"active_recommendations"`
}

// SummaryCellView is one cell of LearningSummary.cells (metrics nil when suppressed).
type SummaryCellView struct {
	TenantID         *uuid.UUID `json:"tenant_id,omitempty"`
	SchoolID         *uuid.UUID `json:"school_id,omitempty"`
	GradeLevel       int        `json:"grade_level"`
	SubjectID        uuid.UUID  `json:"subject_id"`
	Students         *int       `json:"students"`
	ActiveStudents   *int       `json:"active_students"`
	ActiveMinutesAvg *float64   `json:"active_minutes_avg"`
	AttemptsComplete *int       `json:"attempts_completed"`
	Accuracy         *float64   `json:"accuracy"`
	Suppressed       bool       `json:"suppressed"`
}

// SummaryGroupView is one row of LearningSummary.by_school / by_tenant.
type SummaryGroupView struct {
	SchoolID       *uuid.UUID `json:"school_id,omitempty"`
	TenantID       *uuid.UUID `json:"tenant_id,omitempty"`
	ActiveStudents *int       `json:"active_students"`
	Accuracy       *float64   `json:"accuracy"`
}

// LearningSummaryView is LearningSummary (totals over the visible cells only, F043-AC-25).
type LearningSummaryView struct {
	DashboardMeta
	ActiveStudents           *int               `json:"active_students"`
	ActiveMinutesAvg         *float64           `json:"active_minutes_avg"`
	AttemptsCompleted        *int               `json:"attempts_completed"`
	Accuracy                 *float64           `json:"accuracy"`
	AssignmentCompletionRate *float64           `json:"assignment_completion_rate"`
	BySchool                 []SummaryGroupView `json:"by_school,omitempty"`
	ByTenant                 []SummaryGroupView `json:"by_tenant,omitempty"`
	SuppressedCells          int                `json:"suppressed_cells"`
	Cells                    []SummaryCellView  `json:"cells"`
	ExcludedTenantIDs        []uuid.UUID        `json:"excluded_tenant_ids,omitempty"`
}
