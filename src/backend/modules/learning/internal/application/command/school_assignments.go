package command

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/assignment"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Events of the school assignments (EVENT_DESIGN §4.3, F041).
const (
	EventAssignmentCreated           = "assignment.created"
	EventAssignmentPublished         = "assignment.published"
	EventAssignmentClosed            = "assignment.closed"
	EventAssignmentResultsReleased   = "assignment.results_released"
	EventAssignmentCompleted         = "assignment.completed"
	EventAssignmentSubmissionUpdated = "assignment.submission_updated"
	EventAssignmentReminded          = "assignment.reminded"
	EventAssignmentCancelled         = "assignment.cancelled"
)

// AssignmentConfig are the parameters of the assignments (CONFIG_DEFAULTS): graded_assignment_release_default,
// late_window_hours (maximum), assignment_remind_limit and the day of a reminder (ICT). The defaults apply until module
// learning reads the platform and tenant settings [Proposed, like attempt_start_rate].
type AssignmentConfig struct {
	GradedFeedback string
	GradedRelease  string
	MaxLateHours   int
	RemindLimit    int
	Location       *time.Location
}

// DefaultAssignmentConfig is the CONFIG_DEFAULTS of F041.
func DefaultAssignmentConfig() AssignmentConfig {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		loc = time.FixedZone("ICT", 7*3600)
	}
	return AssignmentConfig{GradedFeedback: assignment.FeedbackAfterClose, GradedRelease: assignment.ReleaseAfterClose, MaxLateHours: 168,
		RemindLimit: 1, Location: loc}
}

// SchoolAssignmentMetrics counts eduworld_lms_assignments_total{status}, eduworld_lms_submissions_updated_total{status} and
// eduworld_lms_assignment_reminders_total{result}; nil = not counted.
type SchoolAssignmentMetrics interface {
	AssignmentStatus(status string)
	SubmissionUpdated(status string)
	Reminder(result string)
}

// Assignments holds the write use cases of the school assignments (F041): create, update, publish, cancel, archive,
// release the results, remind, extend and excuse one student, the scheduler that opens and closes, the submission read
// model fed by the attempt events, the enrollment changes and the gate of the attempts started for an assignment.
// Every staff use case authorizes explicitly with the school and subject of the assignment (AuthorizeLMS, R7).
type Assignments struct {
	UoW     write.UnitOfWork
	Store   write.SchoolAssignmentStore
	Read    read.Session // platform reference checks (RESOURCE_NOT_ASSIGNED)
	Exams   port.ExamSessionCreator
	Events  write.EventPublisher
	Audit   write.Auditor
	Clock   func() time.Time
	Config  AssignmentConfig
	Metrics SchoolAssignmentMetrics
}

func (h Assignments) now() time.Time { return now(h.Clock).UTC() }

func (h Assignments) config() AssignmentConfig {
	c := h.Config
	d := DefaultAssignmentConfig()
	if c.GradedFeedback == "" {
		c.GradedFeedback = d.GradedFeedback
	}
	if c.GradedRelease == "" {
		c.GradedRelease = d.GradedRelease
	}
	if c.MaxLateHours <= 0 {
		c.MaxLateHours = d.MaxLateHours
	}
	if c.RemindLimit <= 0 {
		c.RemindLimit = d.RemindLimit
	}
	if c.Location == nil {
		c.Location = d.Location
	}
	return c
}

func (h Assignments) count(status string) {
	if h.Metrics != nil {
		h.Metrics.AssignmentStatus(status)
	}
}

func (h Assignments) audit(ctx context.Context, action string, a write.AssignmentRow, reason string, meta map[string]any) error {
	return h.Audit.Record(ctx, audit.Event{Action: action, TargetType: "assignment", TargetID: a.ID.String(), Result: audit.ResultSuccess,
		Reason: reason, Metadata: meta, SchoolID: a.SchoolID.String()})
}

func (h Assignments) publish(ctx context.Context, typ string, a write.AssignmentRow, payload any) error {
	t := a.TenantID
	version := 1
	if typ == EventAssignmentCreated || typ == EventAssignmentPublished {
		version = AssignmentCreatedVersion
	}
	return h.Events.Publish(ctx, write.Event{Type: typ, Version: version, SubjectType: "assignment", SubjectID: a.ID.String(), TenantID: &t,
		Payload: payload})
}

// AssignmentTargetInput is AssignmentTarget.
type AssignmentTargetInput struct {
	TargetType        string      `json:"target_type"`
	GradeLevel        *int        `json:"grade_level"`
	IncludeStudentIDs []uuid.UUID `json:"include_student_ids"`
	ExcludeStudentIDs []uuid.UUID `json:"exclude_student_ids"`
	StudentIDs        []uuid.UUID `json:"student_ids"`
}

// target is the domain view of an AssignmentTarget (list lengths only).
func (t AssignmentTargetInput) target() assignment.Target {
	g := 0
	if t.GradeLevel != nil {
		g = *t.GradeLevel
	}
	return assignment.Target{Type: t.TargetType, Grade: g, Include: len(t.IncludeStudentIDs), Exclude: len(t.ExcludeStudentIDs),
		StudentIDs: len(t.StudentIDs)}
}

// LatePolicyInput is AssignmentCreate.late_policy.
type LatePolicyInput struct {
	AllowLate       *bool `json:"allow_late"`
	LateWindowHours *int  `json:"late_window_hours"`
}

// AssignmentInput is AssignmentCreate (and the merge of AssignmentUpdate over a stored assignment).
type AssignmentInput struct {
	TenantID   *uuid.UUID `json:"tenant_id"`
	SchoolID   uuid.UUID  `json:"school_id"`
	SubjectID  uuid.UUID  `json:"subject_id"`
	Title      string     `json:"title"`
	ContentRef struct {
		ContentType     string     `json:"content_type"`
		ContentID       uuid.UUID  `json:"content_id"`
		PinnedVersionID *uuid.UUID `json:"pinned_version_id"`
	} `json:"content_ref"`
	Instructions   *string                 `json:"instructions"`
	FollowLatest   bool                    `json:"follow_latest"`
	Mode           string                  `json:"mode"`
	Targets        []AssignmentTargetInput `json:"targets"`
	OpenAt         time.Time               `json:"open_at"`
	DueAt          time.Time               `json:"due_at"`
	LatePolicy     *LatePolicyInput        `json:"late_policy"`
	MaxAttempts    *int                    `json:"max_attempts"`
	ScorePolicy    string                  `json:"score_policy"`
	FeedbackTiming string                  `json:"feedback_timing"`
	ResultRelease  string                  `json:"result_release"`
}

func (in AssignmentInput) fields() assignment.Fields {
	f := assignment.Fields{Title: in.Title, Instructions: in.Instructions, Mode: in.Mode, FollowLatest: in.FollowLatest, OpenAt: in.OpenAt,
		DueAt: in.DueAt, ScorePolicy: in.ScorePolicy, FeedbackTiming: in.FeedbackTiming, ResultRelease: in.ResultRelease}
	if in.LatePolicy != nil {
		if in.LatePolicy.AllowLate != nil {
			f.AllowLate = *in.LatePolicy.AllowLate
		}
		if in.LatePolicy.LateWindowHours != nil {
			f.LateHours = *in.LatePolicy.LateWindowHours
		}
	}
	if in.MaxAttempts != nil {
		f.MaxAttempts = *in.MaxAttempts
	}
	for _, t := range in.Targets {
		f.Targets = append(f.Targets, t.target())
	}
	return f
}

func assignmentViolations(vs []assignment.Violation) error {
	if len(vs) == 0 {
		return nil
	}
	pe := &problem.Error{Code: problem.ValidationFailed, Detail: "request validation failed"}
	for _, v := range vs {
		pe.AddField(v.Field, problem.SubInvalid, v.Reason)
	}
	return pe
}

// AssignmentViewOf is the Assignment of a stored row (completion is added by the queries).
func AssignmentViewOf(a write.AssignmentRow) dto.AssignmentView {
	v := dto.AssignmentView{AssignmentID: a.ID, TenantID: a.TenantID, SchoolID: a.SchoolID, SubjectID: a.SubjectID, Title: a.Title,
		Instructions: a.Instructions, ContentRef: dto.AssignmentContentRefView{ContentType: a.ContentType, ContentID: a.ContentID,
			PinnedVersionID: a.PinnedVersionID}, FollowLatest: a.FollowLatest, Mode: a.Mode, Targets: []dto.AssignmentTargetView{}, OpenAt: a.OpenAt,
		DueAt: a.DueAt, LatePolicy: dto.LatePolicyView{AllowLate: a.AllowLate, LateWindowHours: a.LateHours}, MaxAttempts: a.MaxAttempts,
		ScorePolicy: a.ScorePolicy, FeedbackTiming: a.FeedbackTiming, ResultRelease: a.ResultRelease, Status: a.Status, CreatedBy: a.CreatedBy,
		CreatedByRole: a.CreatedByRole, ExamSessionID: a.ExamSessionID, ResultsReleasedAt: a.ReleasedAt, RowVersion: a.Version}
	for _, t := range a.Targets {
		tv := dto.AssignmentTargetView{TargetType: t.Type, GradeLevel: t.Grade, IncludeStudentIDs: t.Include, ExcludeStudentIDs: t.Exclude,
			StudentIDs: t.StudentIDs}
		if a.Status != string(assignment.StatusDraft) {
			n := t.StudentCount
			tv.StudentCount = &n
		}
		v.Targets = append(v.Targets, tv)
	}
	return v
}

// AssignmentCreatedVersion is the schema_version of assignment.created and assignment.published (v2: grade targets and
// subject_id, ADR-0095 mục 7 — the only consumer in the repo, the LMS schedule of F040, is removed).
const AssignmentCreatedVersion = 2

// assignmentPayload is payload assignment.created.v2 (also assignment.published): counts only, no student list.
func assignmentPayload(a write.AssignmentRow) map[string]any {
	targets := make([]map[string]any, len(a.Targets))
	for i, t := range a.Targets {
		x := map[string]any{"target_type": t.Type, "grade_level": t.Grade, "student_count": t.StudentCount}
		if t.Type == assignment.TargetGrade {
			x["included_student_count"], x["excluded_student_count"] = len(t.Include), len(t.Exclude)
		}
		targets[i] = x
	}
	return map[string]any{"assignment_id": a.ID, "school_id": a.SchoolID, "subject_id": a.SubjectID, "created_by": a.CreatedBy,
		"created_by_role": a.CreatedByRole, "content_type": a.ContentType, "content_id": a.ContentID, "pinned_version_id": a.PinnedVersionID,
		"follow_latest": a.FollowLatest, "mode": a.Mode, "status": a.Status, "targets": targets, "open_at": a.OpenAt, "due_at": a.DueAt,
		"max_attempts": a.MaxAttempts, "feedback_timing": a.FeedbackTiming, "result_release": a.ResultRelease,
		"objective_ids": nonNilUUIDs(a.ObjectiveIDs)}
}

// deniedOutside records authz.denied of a denied use case outside its rolled back transaction (F041-AC-04, AC-27).
func deniedOutside(ctx context.Context, err error) error {
	if pe, ok := problemOf(err); ok && pe.Code == problem.PermissionDenied {
		_ = denied(ctx, "lms_assignment")
	}
	return err
}

// roleOf is created_by_role: system_admin for a System Admin acting for a tenant (F041-AC-28), school_admin when a
// school_admin grant covers the school, otherwise academic_manager.
func roleOf(p authz.Principal, school uuid.UUID) string {
	for _, g := range p.Grants {
		if g.Role == "system_admin" {
			return "system_admin"
		}
	}
	for _, g := range p.Grants {
		if g.Role == "school_admin" && (g.SchoolID == nil || *g.SchoolID == school) {
			return "school_admin"
		}
	}
	return "academic_manager"
}

// schoolOfTenant is 404 for a school that is not a school of the session tenant (R8: no existence leak).
func (h Assignments) schoolOfTenant(ctx context.Context, school uuid.UUID) error {
	ok, err := h.Store.SchoolOfTenant(ctx, school)
	if err != nil {
		return err
	}
	if !ok {
		return problem.New(problem.NotFound, "")
	}
	return nil
}

// schoolSubject authorizes perm on the school and subject of a new assignment (F041-AC-04, AC-05, AC-26; ADR-0095: no
// course): a school outside every grant is 404, another subject 403, a subject the tenant cannot see 422 at subject_id.
func (h Assignments) schoolSubject(ctx context.Context, p authz.Principal, perm authz.Permission, school, subject uuid.UUID) error {
	if school == uuid.Nil {
		return problem.Invalid("/school_id", problem.SubRequired, "is required")
	}
	if subject == uuid.Nil {
		return problem.Invalid("/subject_id", problem.SubRequired, "is required")
	}
	if err := AuthorizeLMS(ctx, p, perm, school, &subject); err != nil {
		return err
	}
	if err := h.schoolOfTenant(ctx, school); err != nil {
		return err
	}
	ok, err := h.Store.SubjectVisible(ctx, subject)
	if err != nil {
		return err
	}
	if !ok {
		return problem.Invalid("/subject_id", problem.SubInvalid, "is not a subject visible to the tenant")
	}
	return nil
}

// assignment reads an assignment (locked) and authorizes perm on its school and subject.
func (h Assignments) assignment(ctx context.Context, p authz.Principal, perm authz.Permission, id uuid.UUID) (*write.AssignmentRow, error) {
	a, err := h.Store.Assignment(ctx, id, true)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, problem.New(problem.NotFound, "")
	}
	if err := AuthorizeLMS(ctx, p, perm, a.SchoolID, &a.SubjectID); err != nil {
		return nil, err
	}
	return a, nil
}

// platformHas reports whether find sees a platform item under the platform scope (403 RESOURCE_NOT_ASSIGNED).
func (h Assignments) platformHas(ctx context.Context, find func(ctx context.Context) (*write.VersionAccess, error)) (bool, error) {
	if h.Read == nil {
		return false, nil
	}
	ps, err := authz.SystemPlatformScope("learning-reference-check")
	if err != nil {
		return false, err
	}
	var v *write.VersionAccess
	if err := h.Read.Read(tenancy.WithScope(ctx, ps), func(ctx context.Context) error {
		var err error
		v, err = find(ctx)
		return err
	}); err != nil {
		return false, err
	}
	return v != nil && v.Platform, nil
}

// content resolves the content of an assignment (F041-AC-01…AC-03, AC-09, AC-30): published and visible to the tenant
// (409 CONTENT_NOT_PUBLISHED, 403 RESOURCE_NOT_ASSIGNED, 422 at content_ref otherwise); the version pinned (the current one
// by default, none with follow_latest); an activity type of the release; a graded blueprint needs exam sessions (F042);
// quests and mini-games come from the catalog of module game.
func (h Assignments) content(ctx context.Context, a *write.AssignmentRow, pinned *uuid.UUID) error {
	switch a.ContentType {
	case assignment.ContentQuest, assignment.ContentMinigame:
		g, err := h.Store.GameActivity(ctx, a.ContentType, a.ContentID)
		if err != nil {
			return err
		}
		if g == nil || g.Status != "published" {
			return problem.New(problem.ResourceNotAssigned, "the game activity is not published for the tenant")
		}
		v := g.CurrentVersionID
		if pinned != nil && *pinned != v {
			return problem.Invalid("/content_ref/pinned_version_id", problem.SubInvalid, "is not the published version of the game activity")
		}
		a.PinnedVersionID, a.ObjectiveIDs = &v, limitIDs(g.ObjectiveIDs, 50)
		if a.FollowLatest {
			a.PinnedVersionID = nil
		}
		return nil
	case assignment.ContentBlueprint:
		if h.Exams == nil || !h.Exams.Available() {
			// F041-AC-09: exam sessions come with F042; practice on a blueprint is an exam session too [Proposed].
			return problem.Invalid("/content_ref", problem.SubInvalid, "an assessment blueprint is delivered as an exam session, not available yet")
		}
	case assignment.ContentActivity, assignment.ContentQuestionSet, assignment.ContentLesson:
	default:
		return problem.Invalid("/content_ref/content_type", problem.SubInvalid, "is not a content type of an assignment")
	}
	var v *write.VersionAccess
	var versionID uuid.UUID
	var err error
	if pinned != nil {
		versionID = *pinned
		find := func(ctx context.Context) (*write.VersionAccess, error) {
			return h.Store.ContentVersion(ctx, a.ContentType, versionID)
		}
		if v, err = find(ctx); err != nil {
			return err
		}
		if v == nil {
			if ok, err := h.platformHas(ctx, find); err != nil || ok {
				if err == nil {
					err = problem.New(problem.ResourceNotAssigned, "the platform content is not assigned to the tenant")
				}
				return err
			}
			return problem.Invalid("/content_ref/pinned_version_id", problem.SubInvalid, "is not a version visible to the tenant")
		}
		if v.ItemID != a.ContentID {
			return problem.Invalid("/content_ref/pinned_version_id", problem.SubInvalid, "is not a version of the content")
		}
	} else {
		find := func(ctx context.Context) (*write.VersionAccess, error) {
			v, _, err := h.Store.CurrentVersion(ctx, a.ContentType, a.ContentID)
			return v, err
		}
		if v, versionID, err = h.Store.CurrentVersion(ctx, a.ContentType, a.ContentID); err != nil {
			return err
		}
		if v == nil {
			if ok, err := h.platformHas(ctx, find); err != nil || ok {
				if err == nil {
					err = problem.New(problem.ResourceNotAssigned, "the platform content is not assigned to the tenant")
				}
				return err
			}
			return problem.Invalid("/content_ref/content_id", problem.SubInvalid, "is not a content visible to the tenant")
		}
	}
	if v.Status != "published" || versionID == uuid.Nil {
		return problem.New(problem.ContentNotPublished, "the content is not published")
	}
	if v.SubjectID != nil && *v.SubjectID != a.SubjectID {
		// ADR-0095: the subject is chosen directly; it must be the subject of the content when the content has one.
		return problem.Invalid("/subject_id", problem.SubInvalid, "is not the subject of the content")
	}
	if a.ContentType == assignment.ContentActivity {
		typ, err := h.Store.ActivityType(ctx, versionID)
		if err != nil {
			return err
		}
		if !slices.Contains(assignment.SupportedActivityTypes, typ) {
			return problem.New(problem.ActivityTypeNotSupported, "only automatically graded activities are assigned in this release")
		}
		a.ActivityType = &typ
	}
	a.PinnedVersionID, a.ObjectiveIDs = &versionID, limitIDs(v.ObjectiveIDs, 50)
	if a.FollowLatest {
		a.PinnedVersionID = nil
	}
	return nil
}

func limitIDs(xs []uuid.UUID, n int) []uuid.UUID {
	if len(xs) > n {
		return xs[:n]
	}
	return xs
}

// sortedIDs returns the distinct ids in a stable order.
func sortedIDs(xs []uuid.UUID) []uuid.UUID {
	out := slices.Clone(xs)
	slices.SortFunc(out, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })
	return slices.Compact(out)
}

// schoolStudents checks that every listed student is an active student of the school (ADR-0095 mục 3): a student of
// another school or tenant is 422 at ptr (the request names the student; nothing about the other school is returned).
func (h Assignments) schoolStudents(ctx context.Context, school uuid.UUID, ids []uuid.UUID, ptr string) ([]uuid.UUID, error) {
	uniq := sortedIDs(ids)
	if len(uniq) == 0 {
		return uniq, nil
	}
	found, err := h.Store.ActiveStudents(ctx, school, uniq)
	if err != nil {
		return nil, err
	}
	if len(found) != len(uniq) {
		return nil, problem.Invalid(ptr, problem.SubInvalid, "every student must be an active student of the school of the assignment")
	}
	return uniq, nil
}

// targets checks the targets of an assignment (ADR-0095 mục 3, F041-AC-05): a grade of the school with included students
// of the school (another grade) and excluded students, or a list of students of the school; a student of another school
// or tenant is 422.
func (h Assignments) targets(ctx context.Context, school uuid.UUID, in []AssignmentTargetInput) ([]write.TargetRow, error) {
	out := make([]write.TargetRow, 0, len(in))
	for i, t := range in {
		at := "/targets/" + itoa(i)
		row := write.TargetRow{Type: t.TargetType}
		var err error
		switch t.TargetType {
		case assignment.TargetGrade:
			g := *t.GradeLevel
			row.Grade = &g
			if row.Include, err = h.schoolStudents(ctx, school, t.IncludeStudentIDs, at+"/include_student_ids"); err != nil {
				return nil, err
			}
			// An excluded student may have left the school since: only the ids are kept (exclude wins).
			row.Exclude = sortedIDs(t.ExcludeStudentIDs)
		case assignment.TargetStudents:
			if row.StudentIDs, err = h.schoolStudents(ctx, school, t.StudentIDs, at+"/student_ids"); err != nil {
				return nil, err
			}
			row.StudentCount = len(row.StudentIDs)
		}
		out = append(out, row)
	}
	return out, nil
}

// CreateAssignment creates a draft assignment (F041-AC-01…AC-07, AC-09, AC-28): lms.assignment.write in the school and
// subject (ADR-0095: chosen directly, no course), published content visible to the tenant pinned at creation, outbox assignment.created and
// audit in the transaction (Idempotency-Key).
func (h Assignments) CreateAssignment(ctx context.Context, in AssignmentInput) (out dto.AssignmentView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.CreateAssignment")
	defer func() { done(err) }()
	p, err := staffOf(ctx)
	if err != nil {
		return out, err
	}
	if in.TenantID != nil && *in.TenantID != *p.TenantID {
		return out, problem.Invalid("/tenant_id", problem.SubInvalid, "is not the tenant of the session (X-Tenant-Id)")
	}
	cfg := h.config()
	f := in.fields()
	f.Defaults(cfg.GradedFeedback, cfg.GradedRelease)
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.schoolSubject(ctx, p, authz.PermLmsAssignmentWrite, in.SchoolID, in.SubjectID); err != nil {
			return err
		}
		if err := assignmentViolations(f.Validate(cfg.MaxLateHours)); err != nil {
			return err
		}
		at := h.now()
		a := write.AssignmentRow{ID: ids.New(), TenantID: *p.TenantID, SchoolID: in.SchoolID, SubjectID: in.SubjectID,
			CreatedBy: p.UserID, CreatedByRole: roleOf(p, in.SchoolID), Title: strings.TrimSpace(f.Title), Instructions: f.Instructions,
			ContentType: in.ContentRef.ContentType, ContentID: in.ContentRef.ContentID, FollowLatest: f.FollowLatest, Mode: f.Mode,
			OpenAt: f.OpenAt.UTC(), DueAt: f.DueAt.UTC(), AllowLate: f.AllowLate, LateHours: f.LateHours, MaxAttempts: f.MaxAttempts,
			ScorePolicy: f.ScorePolicy, FeedbackTiming: f.FeedbackTiming, ResultRelease: f.ResultRelease, Status: string(assignment.StatusDraft),
			Version: 1, CreatedAt: at, UpdatedAt: at}
		if err := h.content(ctx, &a, in.ContentRef.PinnedVersionID); err != nil {
			return err
		}
		if a.Targets, err = h.targets(ctx, a.SchoolID, in.Targets); err != nil {
			return err
		}
		if err := h.Store.InsertAssignment(ctx, a); err != nil {
			return err
		}
		if err := h.publish(ctx, EventAssignmentCreated, a, assignmentPayload(a)); err != nil {
			return err
		}
		if err := h.audit(ctx, "assignment.created", a, "", map[string]any{"created_by_role": a.CreatedByRole, "content_type": a.ContentType,
			"mode": a.Mode, "target_count": len(a.Targets)}); err != nil {
			return err
		}
		h.count(a.Status)
		out = AssignmentViewOf(a)
		return idempotency.SetResult(ctx, out)
	})
	return out, deniedOutside(ctx, err)
}

// AssignmentPatch is a decoded AssignmentUpdate: the names of the fields present and their merged values.
type AssignmentPatch struct {
	Present []string
	Input   AssignmentInput
}

// inputOf is the AssignmentInput of a stored assignment (the base of a merge patch).
func inputOf(a write.AssignmentRow) AssignmentInput {
	in := AssignmentInput{SchoolID: a.SchoolID, SubjectID: a.SubjectID, Title: a.Title, Instructions: a.Instructions, FollowLatest: a.FollowLatest, Mode: a.Mode,
		OpenAt: a.OpenAt, DueAt: a.DueAt, ScorePolicy: a.ScorePolicy, FeedbackTiming: a.FeedbackTiming, ResultRelease: a.ResultRelease}
	in.ContentRef.ContentType, in.ContentRef.ContentID, in.ContentRef.PinnedVersionID = a.ContentType, a.ContentID, a.PinnedVersionID
	allow, hours, attempts := a.AllowLate, a.LateHours, a.MaxAttempts
	in.LatePolicy, in.MaxAttempts = &LatePolicyInput{AllowLate: &allow, LateWindowHours: &hours}, &attempts
	for _, t := range a.Targets {
		in.Targets = append(in.Targets, AssignmentTargetInput{TargetType: t.Type, GradeLevel: t.Grade, IncludeStudentIDs: t.Include,
			ExcludeStudentIDs: t.Exclude, StudentIDs: t.StudentIDs})
	}
	return in
}

// BaseInput returns the AssignmentInput of an assignment the presentation merges a patch over (F041-AC-10).
func (h Assignments) BaseInput(ctx context.Context, id uuid.UUID) (AssignmentInput, error) {
	p, err := staffOf(ctx)
	if err != nil {
		return AssignmentInput{}, err
	}
	var in AssignmentInput
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		a, err := h.assignment(ctx, p, authz.PermLmsAssignmentWrite, id)
		if err != nil {
			return err
		}
		in = inputOf(*a)
		return nil
	})
	return in, err
}

// UpdateAssignment applies a merge patch (F041-AC-09, AC-10): If-Match on the row version (412), every field in draft,
// due_at/instructions/late_policy when open, 409 ASSIGNMENT_NOT_EDITABLE otherwise; the result is validated again and
// audited.
func (h Assignments) UpdateAssignment(ctx context.Context, id uuid.UUID, patch AssignmentPatch, ifMatch int64) (out dto.AssignmentView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.UpdateAssignment")
	defer func() { done(err) }()
	p, err := staffOf(ctx)
	if err != nil {
		return out, err
	}
	cfg := h.config()
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		a, err := h.assignment(ctx, p, authz.PermLmsAssignmentWrite, id)
		if err != nil {
			return err
		}
		if !assignment.CanEdit(assignment.Status(a.Status), patch.Present) {
			return problem.New(problem.AssignmentNotEditable, "this field cannot change in the status "+a.Status)
		}
		if err := checkRowVersion(a.Version, ifMatch); err != nil {
			return err
		}
		in := patch.Input
		f := in.fields()
		f.Defaults(cfg.GradedFeedback, cfg.GradedRelease)
		if err := assignmentViolations(f.Validate(cfg.MaxLateHours)); err != nil {
			return err
		}
		prev := *a
		a.Title, a.Instructions, a.FollowLatest, a.Mode = strings.TrimSpace(f.Title), f.Instructions, f.FollowLatest, f.Mode
		a.OpenAt, a.DueAt, a.AllowLate, a.LateHours, a.MaxAttempts = f.OpenAt.UTC(), f.DueAt.UTC(), f.AllowLate, f.LateHours, f.MaxAttempts
		a.ScorePolicy, a.FeedbackTiming, a.ResultRelease, a.UpdatedBy = f.ScorePolicy, f.FeedbackTiming, f.ResultRelease, &p.UserID
		replace := slices.Contains(patch.Present, "targets")
		if a.Status == string(assignment.StatusDraft) {
			if slices.ContainsFunc(patch.Present, func(s string) bool { return s == "mode" || s == "follow_latest" }) || replace {
				if err := h.content(ctx, a, a.PinnedVersionID); err != nil {
					return err
				}
			}
			if replace {
				if a.Targets, err = h.targets(ctx, a.SchoolID, in.Targets); err != nil {
					return err
				}
			}
		}
		ok, err := h.Store.UpdateAssignment(ctx, *a, prev.Version, replace)
		if err != nil {
			return err
		}
		if !ok {
			return problem.New(problem.VersionMismatch, "")
		}
		if err := h.audit(ctx, "assignment.updated", *a, "", map[string]any{"fields": patch.Present}); err != nil {
			return err
		}
		a.Version++
		out = AssignmentViewOf(*a)
		return nil
	})
	return out, deniedOutside(ctx, err)
}

// snapshot lists the students targeted by an assignment at publish (F041-AC-08, ADR-0095 mục 3): the active students of
// each targeted grade of the school plus the included students minus the excluded ones (exclude wins over include), and
// the listed students still active in the school; a student held by several targets once, with the grade the student is
// in when targeted (recorded at event time).
func (h Assignments) snapshot(ctx context.Context, a write.AssignmentRow) ([]write.TargetStudent, error) {
	var grades, includes, lists [][]uuid.UUID
	var excludes []uuid.UUID
	gradeOf := map[uuid.UUID]int{}
	for _, t := range a.Targets {
		switch t.Type {
		case assignment.TargetGrade:
			xs, err := h.Store.GradeStudents(ctx, a.SchoolID, *t.Grade)
			if err != nil {
				return nil, err
			}
			for _, x := range xs {
				gradeOf[x] = *t.Grade
			}
			grades = append(grades, xs)
			inc, err := h.Store.ActiveStudents(ctx, a.SchoolID, t.Include)
			if err != nil {
				return nil, err
			}
			includes = append(includes, inc)
			excludes = append(excludes, t.Exclude...)
		default:
			xs, err := h.Store.ActiveStudents(ctx, a.SchoolID, t.StudentIDs)
			if err != nil {
				return nil, err
			}
			lists = append(lists, xs)
		}
	}
	ids := assignment.Audience(grades, includes, lists, excludes)
	out := make([]write.TargetStudent, 0, len(ids))
	for _, id := range ids {
		s := write.TargetStudent{StudentID: id, Source: "publish"}
		if g, ok := gradeOf[id]; ok {
			s.Grade = &g
		} else {
			pl, err := h.Store.Placement(ctx, id)
			if err != nil {
				return nil, err
			}
			if pl != nil {
				g := pl.Grade
				s.Grade = &g
			}
		}
		out = append(out, s)
	}
	return out, nil
}

// PublishAssignment publishes a draft (F041-AC-08, AC-17): the content is checked again, the targeted students are
// snapshotted with their submissions, the assignment becomes scheduled (open_at ahead) or open; outbox
// assignment.published and audit in the transaction.
func (h Assignments) PublishAssignment(ctx context.Context, id uuid.UUID, ifMatch int64) (out dto.AssignmentView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.PublishAssignment")
	defer func() { done(err) }()
	p, err := staffOf(ctx)
	if err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		a, err := h.assignment(ctx, p, authz.PermLmsAssignmentWrite, id)
		if err != nil {
			return err
		}
		if err := checkRowVersion(a.Version, ifMatch); err != nil {
			return err
		}
		if a.Status != string(assignment.StatusDraft) {
			return problem.New(problem.AssignmentNotEditable, "only a draft is published")
		}
		pinned := a.PinnedVersionID
		if err := h.content(ctx, a, pinned); err != nil {
			return err
		}
		students, err := h.snapshot(ctx, *a)
		if err != nil {
			return err
		}
		if _, err := h.Store.InsertTargetStudents(ctx, *a, students); err != nil {
			return err
		}
		if err := h.examSession(ctx, a, students); err != nil {
			return err
		}
		now := h.now()
		prev := a.Version
		a.Status, a.PublishedAt, a.UpdatedBy = string(assignment.PublishStatus(a.OpenAt, now)), &now, &p.UserID
		ok, err := h.Store.UpdateAssignment(ctx, *a, prev, false)
		if err != nil {
			return err
		}
		if !ok {
			return problem.New(problem.VersionMismatch, "")
		}
		fresh, err := h.Store.Assignment(ctx, a.ID, false)
		if err != nil {
			return err
		}
		if fresh == nil {
			return problem.New(problem.NotFound, "")
		}
		if err := h.publish(ctx, EventAssignmentPublished, *fresh, assignmentPayload(*fresh)); err != nil {
			return err
		}
		if err := h.audit(ctx, "assignment.published", *fresh, "", map[string]any{"status": fresh.Status, "student_count": len(students)}); err != nil {
			return err
		}
		h.count(fresh.Status)
		out = AssignmentViewOf(*fresh)
		return idempotency.SetResult(ctx, out)
	})
	return out, deniedOutside(ctx, err)
}

// examSession creates the exam session of an assignment on a blueprint in the publish transaction (F042-AC-31): the
// students snapshotted, the window open_at → due_at (+ late window), the pinned blueprint version and the release
// policy of the assignment; its id is stored on the assignment.
func (h Assignments) examSession(ctx context.Context, a *write.AssignmentRow, students []write.TargetStudent) error {
	if a.ContentType != assignment.ContentBlueprint || h.Exams == nil || !h.Exams.Available() || a.PinnedVersionID == nil {
		return nil
	}
	ids := make([]uuid.UUID, len(students))
	for i, s := range students {
		ids[i] = s.StudentID
	}
	closeAt := a.DueAt
	if a.AllowLate {
		closeAt = closeAt.Add(time.Duration(a.LateHours) * time.Hour)
	}
	id, err := h.Exams.Create(ctx, port.ExamSessionRequest{AssignmentID: a.ID, TenantID: a.TenantID, SchoolID: a.SchoolID, SubjectID: a.SubjectID,
		BlueprintID: a.ContentID, BlueprintVerID: *a.PinnedVersionID, OpenAt: a.OpenAt, CloseAt: closeAt, StudentIDs: ids,
		FeedbackTiming: a.FeedbackTiming, ResultRelease: a.ResultRelease, Title: a.Title, MaxAttempts: a.MaxAttempts, ScorePolicy: a.ScorePolicy})
	if err != nil {
		return err
	}
	a.ExamSessionID = &id
	return nil
}

// transition loads an assignment with perm, checks it with ok and saves it changed by apply, with audit (and the events
// of after).
func (h Assignments) transition(ctx context.Context, name string, id uuid.UUID, perm authz.Permission, check func(a *write.AssignmentRow) error,
	apply func(ctx context.Context, p authz.Principal, a *write.AssignmentRow) error) (out dto.AssignmentView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command."+name)
	defer func() { done(err) }()
	p, err := staffOf(ctx)
	if err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		a, err := h.assignment(ctx, p, perm, id)
		if err != nil {
			return err
		}
		if err := check(a); err != nil {
			return err
		}
		prev := a.Version
		a.UpdatedBy = &p.UserID
		if err := apply(ctx, p, a); err != nil {
			return err
		}
		ok, err := h.Store.UpdateAssignment(ctx, *a, prev, false)
		if err != nil {
			return err
		}
		if !ok {
			return problem.New(problem.VersionMismatch, "")
		}
		a.Version++
		h.count(a.Status)
		out = AssignmentViewOf(*a)
		return idempotency.SetResult(ctx, out)
	})
	return out, deniedOutside(ctx, err)
}

func validReason(reason string) error {
	if n := utf8.RuneCountInString(strings.TrimSpace(reason)); n == 0 || utf8.RuneCountInString(reason) > assignment.MaxReason {
		return problem.Invalid("/reason", problem.SubInvalid, "must have 1 to 500 characters")
	}
	return nil
}

// CancelAssignment cancels a scheduled assignment, or an open one nobody started (F041-AC-11): a reason, audit; the exam
// session of a graded assignment is cancelled too (F042).
func (h Assignments) CancelAssignment(ctx context.Context, id uuid.UUID, reason string) (dto.AssignmentView, error) {
	if err := validReason(reason); err != nil {
		return dto.AssignmentView{}, err
	}
	return h.transition(ctx, "CancelAssignment", id, authz.PermLmsAssignmentWrite, func(a *write.AssignmentRow) error {
		return nil
	}, func(ctx context.Context, p authz.Principal, a *write.AssignmentRow) error {
		has, err := h.Store.HasSubmission(ctx, a.ID)
		if err != nil {
			return err
		}
		if !assignment.CanCancel(assignment.Status(a.Status), has) {
			return problem.New(problem.AssignmentNotEditable, "only a scheduled assignment, or an open one nobody started, is cancelled")
		}
		if a.ExamSessionID != nil && h.Exams != nil {
			if err := h.Exams.Cancel(ctx, *a.ExamSessionID, reason); err != nil && !errors.Is(err, port.ErrExamSessionUnavailable) {
				return err
			}
		}
		a.Status = string(assignment.StatusCancelled)
		if err := h.publish(ctx, EventAssignmentCancelled, *a, map[string]any{"assignment_id": a.ID, "school_id": a.SchoolID,
			"exam_session_id": a.ExamSessionID, "cancelled_at": h.now(), "cancelled_by": p.UserID, "reason": reason}); err != nil {
			return err
		}
		return h.audit(ctx, "assignment.cancelled", *a, reason, nil)
	})
}

// ArchiveAssignment archives an assignment whose results are released, or a closed one with immediate results.
func (h Assignments) ArchiveAssignment(ctx context.Context, id uuid.UUID) (dto.AssignmentView, error) {
	return h.transition(ctx, "ArchiveAssignment", id, authz.PermLmsAssignmentWrite, func(a *write.AssignmentRow) error {
		if !assignment.CanArchive(assignment.Status(a.Status), a.ResultRelease) {
			return problem.New(problem.AssignmentNotEditable, "only an assignment with released results is archived")
		}
		return nil
	}, func(ctx context.Context, p authz.Principal, a *write.AssignmentRow) error {
		a.Status = string(assignment.StatusArchived)
		return h.audit(ctx, "assignment.archived", *a, "", nil)
	})
}

// ReleaseAssignmentResults releases the results of a closed manual assignment (F041-AC-20, AC-21): lms.result.release,
// 409 CONFLICT before the close or for another release policy, outbox assignment.results_released and audit.
func (h Assignments) ReleaseAssignmentResults(ctx context.Context, id uuid.UUID) (dto.AssignmentView, error) {
	return h.transition(ctx, "ReleaseAssignmentResults", id, authz.PermLmsResultRelease, func(a *write.AssignmentRow) error {
		if a.Status != string(assignment.StatusClosed) || a.ResultRelease != assignment.ReleaseManual {
			return problem.New(problem.Conflict, "only a closed assignment with a manual release is released")
		}
		return nil
	}, func(ctx context.Context, p authz.Principal, a *write.AssignmentRow) error {
		now := h.now()
		a.Status, a.ReleasedAt = string(assignment.StatusResultsReleased), &now
		if err := h.publish(ctx, EventAssignmentResultsReleased, *a, map[string]any{"assignment_id": a.ID, "school_id": a.SchoolID,
			"released_at": now, "released_by": p.UserID, "trigger": "manual"}); err != nil {
			return err
		}
		return h.audit(ctx, "assignment.results_released", *a, "", nil)
	})
}
