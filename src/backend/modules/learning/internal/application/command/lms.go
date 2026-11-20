package command

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/lms"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Events and audit actions of the LMS (F040 plan, ADR-0084).
const (
	EventStudyGroupCreated       = "study_group.created"
	EventStudyGroupUpdated       = "study_group.updated"
	EventStudyGroupMemberChanged = "study_group.member_changed"
	EventStudyGroupDeleted       = "study_group.deleted"
	EventScheduleItemDeleted     = "schedule_item.deleted"
)

// LMSMetrics counts eduworld_lms_courses_provisioned_total{result} and eduworld_lms_path_node_unlocks_total{source}.
type LMSMetrics interface {
	CourseProvisioned(result string)
	NodeUnlocked(source string)
}

// LMS holds the write use cases of the school LMS (F040): courses, school curriculum, learning paths, schedule, study
// groups. Every use case authorizes explicitly with the school and subject of the resource (authz.Require, R7).
type LMS struct {
	UoW     write.UnitOfWork
	Store   write.LMSStore
	Read    read.Session // platform reference checks (RESOURCE_NOT_ASSIGNED)
	Events  write.EventPublisher
	Audit   write.Auditor
	Clock   func() time.Time
	Metrics LMSMetrics
}

func (h LMS) now() time.Time { return now(h.Clock).UTC() }

// staffOf is the principal of a portal call: staff of the session tenant (System Admin with its X-Tenant-Id included).
func staffOf(ctx context.Context) (authz.Principal, error) {
	p, err := principal(ctx)
	if err != nil {
		return p, err
	}
	if p.UserType == authz.UserTypeStudent {
		return p, problem.New(problem.PortalAccessStaffOnly, "")
	}
	if p.TenantID == nil {
		return p, problem.New(problem.TenantHeaderRequired, "")
	}
	return p, nil
}

// SchoolCovered reports whether a grant holding perm covers the school (the subject aside).
func SchoolCovered(p authz.Principal, perm authz.Permission, school uuid.UUID) bool {
	for _, g := range p.Grants {
		if slices.Contains(g.Permissions, perm) && (g.SchoolID == nil || *g.SchoolID == school) {
			return true
		}
	}
	return false
}

// AuthorizeLMS authorizes perm on a resource of a school and subject (F040-AC-24…AC-26): without the permission 403
// PERMISSION_DENIED; a school outside every grant 404 NOT_FOUND (no existence leak); a subject outside a
// subject-limited grant 403 PERMISSION_DENIED; denials are audited (authz.denied).
func AuthorizeLMS(ctx context.Context, p authz.Principal, perm authz.Permission, school uuid.UUID, subject *uuid.UUID) error {
	if !holdsPerm(p, perm) {
		return authz.Require(ctx, p, perm, authz.Resource{TenantID: p.TenantID, SchoolID: &school, SubjectID: subject})
	}
	if !SchoolCovered(p, perm, school) {
		_ = authz.Require(ctx, p, perm, authz.Resource{TenantID: p.TenantID, SchoolID: &school})
		return problem.New(problem.NotFound, "")
	}
	return authz.Require(ctx, p, perm, authz.Resource{TenantID: p.TenantID, SchoolID: &school, SubjectID: subject})
}

func (h LMS) audit(ctx context.Context, action, targetType string, target uuid.UUID, school uuid.UUID, reason string, meta map[string]any) error {
	return h.Audit.Record(ctx, audit.Event{Action: action, TargetType: targetType, TargetID: target.String(), Result: audit.ResultSuccess,
		Reason: reason, Metadata: meta, SchoolID: school.String()})
}

func (h LMS) publish(ctx context.Context, typ, subjectType string, subject uuid.UUID, tenant uuid.UUID, payload any) error {
	t := tenant
	return h.Events.Publish(ctx, write.Event{Type: typ, SubjectType: subjectType, SubjectID: subject.String(), TenantID: &t, Payload: payload})
}

func violations(vs []lms.Violation) error {
	if len(vs) == 0 {
		return nil
	}
	return problem.Invalid(vs[0].Field, problem.SubInvalid, vs[0].Reason)
}

// ---------- courses

// ClassSynced is the class of a class.synced event (identity) a course is provisioned for.
type ClassSynced struct {
	TenantID       uuid.UUID
	ClassID        uuid.UUID
	SchoolID       uuid.UUID
	Grade          int
	AcademicYear   string
	AcademicYearID *uuid.UUID
	Closed         bool
}

// EnsureCoursesForClass provisions one course per subject of the grade of a class (F040-AC-01, AC-02), in the transaction
// and TenantScope of the consumer: the curriculum version of the school for the academic year and grade, otherwise the
// latest published version the tenant owns or is assigned; a redelivered event creates nothing more; a closed class
// archives its courses.
func (h LMS) EnsureCoursesForClass(ctx context.Context, c ClassSynced) (err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.EnsureCoursesForClass")
	defer func() { done(err) }()
	if c.Closed {
		_, err := h.Store.ArchiveClassCourses(ctx, c.ClassID)
		return err
	}
	year := lms.AcademicYearID(c.TenantID, c.AcademicYear)
	if c.AcademicYearID != nil {
		year = *c.AcademicYearID
	}
	version, err := h.Store.SettingFor(ctx, c.SchoolID, year, c.Grade)
	if err != nil {
		return err
	}
	if version == nil {
		if version, err = h.Store.LatestCurriculumVersion(ctx, c.Grade); err != nil || version == nil {
			h.countCourse("no_curriculum")
			return err
		}
	}
	subjects, err := h.Store.SubjectsOfGrade(ctx, *version, c.Grade)
	if err != nil {
		return err
	}
	for _, s := range subjects {
		ok, err := h.Store.InsertCourse(ctx, write.CourseRow{ID: ids.New(), TenantID: c.TenantID, SchoolID: c.SchoolID, ClassID: c.ClassID,
			SubjectID: s, Grade: c.Grade, AcademicYearID: year, AcademicYear: c.AcademicYear, CurriculumVersionID: *version})
		if err != nil {
			return err
		}
		if ok {
			h.countCourse("created")
		} else {
			h.countCourse("exists")
		}
	}
	return nil
}

func (h LMS) countCourse(result string) {
	if h.Metrics != nil {
		h.Metrics.CourseProvisioned(result)
	}
}

func courseView(c write.CourseRow) dto.CourseView {
	return dto.CourseView{CourseID: c.ID, TenantID: c.TenantID, SchoolID: c.SchoolID, ClassID: c.ClassID, SubjectID: c.SubjectID, Grade: c.Grade,
		AcademicYearID: c.AcademicYearID, CurriculumVersionID: c.CurriculumVersionID, TextbookSeriesID: c.TextbookSeriesID, Status: c.Status,
		Version: c.Version}
}

func (h LMS) course(ctx context.Context, p authz.Principal, perm authz.Permission, id uuid.UUID) (*write.CourseRow, error) {
	c, err := h.Store.Course(ctx, id)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, problem.New(problem.NotFound, "")
	}
	subject := c.SubjectID
	if err := AuthorizeLMS(ctx, p, perm, c.SchoolID, &subject); err != nil {
		return nil, err
	}
	return c, nil
}

// UpdateCourse shows or hides a course (F040-AC-05): lms.course.write in the school and subject, If-Match, audit.
func (h LMS) UpdateCourse(ctx context.Context, id uuid.UUID, status string, ifMatch int64) (out dto.CourseView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.UpdateCourse")
	defer func() { done(err) }()
	p, err := staffOf(ctx)
	if err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		c, err := h.course(ctx, p, authz.PermLmsCourseWrite, id)
		if err != nil {
			return err
		}
		if err := lms.SetCourseStatus(c.Status, status); err != nil {
			return problem.Invalid("/status", problem.SubInvalid, "a course is shown (active) or hidden")
		}
		ok, err := h.Store.UpdateCourseStatus(ctx, id, status, ifMatch)
		if err != nil {
			return err
		}
		if !ok {
			return problem.New(problem.VersionMismatch, "")
		}
		if err := h.audit(ctx, "course.updated", "course", id, c.SchoolID, "", map[string]any{"status": status}); err != nil {
			return err
		}
		c.Status, c.Version = status, ifMatch+1
		out = courseView(*c)
		return nil
	})
	return out, err
}

// ---------- curriculum of the school

// CurriculumSettingInput is one item of SchoolCurriculumSettingsInput.
type CurriculumSettingInput struct {
	AcademicYearID      uuid.UUID `json:"academic_year_id"`
	GradeLevel          *int      `json:"grade_level"`
	CurriculumVersionID uuid.UUID `json:"curriculum_version_id"`
}

// visibleVersion returns a version visible to the tenant: invisible but existing platform content is 403
// RESOURCE_NOT_ASSIGNED (ADR-0029), unknown is 422 at ptr, not published 409 CONTENT_NOT_PUBLISHED.
func (h LMS) visibleVersion(ctx context.Context, find func(ctx context.Context) (*write.VersionAccess, error), ptr string) (*write.VersionAccess, error) {
	v, err := find(ctx)
	if err != nil {
		return nil, err
	}
	if v == nil {
		if h.Read != nil {
			ps, err := authz.SystemPlatformScope("learning-reference-check")
			if err != nil {
				return nil, err
			}
			var platform *write.VersionAccess
			if err := h.Read.Read(tenancy.WithScope(ctx, ps), func(ctx context.Context) error {
				var err error
				platform, err = find(ctx)
				return err
			}); err != nil {
				return nil, err
			}
			if platform != nil && platform.Platform {
				return nil, problem.New(problem.ResourceNotAssigned, "the platform content is not assigned to the tenant")
			}
		}
		return nil, problem.Invalid(ptr, problem.SubInvalid, "is not a version visible to the tenant")
	}
	if v.Status != "published" {
		return nil, problem.New(problem.ContentNotPublished, "the version is not published")
	}
	return v, nil
}

// PutCurriculumSettings sets the curriculum versions of a school per academic year and grade (F040-AC-03): published
// versions of the tenant or of assigned platform curricula only, If-Match on the settings of the school, audit; the
// courses already created keep their version.
func (h LMS) PutCurriculumSettings(ctx context.Context, school uuid.UUID, items []CurriculumSettingInput, ifMatch int64) (out dto.CurriculumSettingsView,
	err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.SetSchoolCurriculum")
	defer func() { done(err) }()
	p, err := staffOf(ctx)
	if err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := AuthorizeLMS(ctx, p, authz.PermSchoolSettingsWrite, school, nil); err != nil {
			return err
		}
		_, version, err := h.Store.CurriculumSettings(ctx, school)
		if err != nil {
			return err
		}
		if version != ifMatch {
			return problem.New(problem.VersionMismatch, "")
		}
		seen := map[string]bool{}
		var years []uuid.UUID
		rows := make([]write.CurriculumSettingRow, 0, len(items))
		for i, it := range items {
			key := it.AcademicYearID.String() + "|"
			if it.GradeLevel != nil {
				key += itoa(*it.GradeLevel)
			}
			if seen[key] {
				return problem.Invalid("/items/"+itoa(i), problem.SubInvalid, "one version per academic year and grade")
			}
			seen[key] = true
			vid := it.CurriculumVersionID
			if _, err := h.visibleVersion(ctx, func(ctx context.Context) (*write.VersionAccess, error) {
				return h.Store.CurriculumVersion(ctx, vid)
			}, "/items/"+itoa(i)+"/curriculum_version_id"); err != nil {
				return err
			}
			if !slices.Contains(years, it.AcademicYearID) {
				years = append(years, it.AcademicYearID)
			}
			rows = append(rows, write.CurriculumSettingRow{ID: ids.New(), TenantID: *p.TenantID, SchoolID: school, AcademicYearID: it.AcademicYearID,
				Grade: it.GradeLevel, CurriculumVersionID: vid, SetBy: p.UserID})
		}
		if err := h.Store.ReplaceCurriculumSettings(ctx, school, years, rows, p.UserID); err != nil {
			return err
		}
		if err := h.audit(ctx, "school_curriculum.set", "school", school, school, "", map[string]any{"items": len(rows)}); err != nil {
			return err
		}
		settings, version, err := h.Store.CurriculumSettings(ctx, school)
		if err != nil {
			return err
		}
		out = dto.CurriculumSettingsView{SchoolID: school, Items: []dto.CurriculumSettingView{}, Version: version}
		for _, s := range settings {
			out.Items = append(out.Items, dto.CurriculumSettingView{AcademicYearID: s.AcademicYearID, GradeLevel: s.Grade,
				CurriculumVersionID: s.CurriculumVersionID, SetBy: s.SetBy, UpdatedAt: s.UpdatedAt})
		}
		return nil
	})
	return out, err
}

// ---------- learning paths

// PathNodeInput is one node of LearningPathInput.
type PathNodeInput struct {
	Position   int `json:"position"`
	ContentRef struct {
		ContentType      string    `json:"content_type"`
		ContentVersionID uuid.UUID `json:"content_version_id"`
	} `json:"content_ref"`
	UnlockRule json.RawMessage `json:"unlock_rule"`
}

// PathInput is LearningPathInput.
type PathInput struct {
	Title      string          `json:"title"`
	UnlockMode string          `json:"unlock_mode"`
	Nodes      []PathNodeInput `json:"nodes"`
}

// nodesOf validates a path input and resolves every node to a published version visible to the tenant (F040-AC-06,
// AC-07); nodes of module game (quest) are refused until the game integration [Proposed].
func (h LMS) nodesOf(ctx context.Context, in PathInput) ([]write.NodeRow, error) {
	ns := make([]lms.NodeInput, len(in.Nodes))
	for i, n := range in.Nodes {
		ns[i] = lms.NodeInput{Position: n.Position, ContentType: n.ContentRef.ContentType, ContentVersionID: n.ContentRef.ContentVersionID}
	}
	if err := violations(lms.ValidatePath(in.Title, in.UnlockMode, ns)); err != nil {
		return nil, err
	}
	out := make([]write.NodeRow, 0, len(in.Nodes))
	for i, n := range in.Nodes {
		ptr := "/nodes/" + itoa(i) + "/content_ref"
		if n.ContentRef.ContentType == "quest" {
			return nil, problem.Invalid(ptr+"/content_type", problem.SubInvalid, "quest nodes come with the game integration")
		}
		typ, vid := n.ContentRef.ContentType, n.ContentRef.ContentVersionID
		v, err := h.visibleVersion(ctx, func(ctx context.Context) (*write.VersionAccess, error) {
			return h.Store.ContentVersion(ctx, typ, vid)
		}, ptr+"/content_version_id")
		if err != nil {
			return nil, err
		}
		out = append(out, write.NodeRow{Position: n.Position, ContentType: typ, ContentID: v.ItemID, ContentVersionID: vid, Title: v.Title,
			ObjectiveIDs: v.ObjectiveIDs, UnlockRule: n.UnlockRule})
	}
	slices.SortFunc(out, func(a, b write.NodeRow) int { return a.Position - b.Position })
	return out, nil
}

// PathView is the LearningPath of a stored path.
func PathView(p write.PathRow, courseID uuid.UUID) dto.LearningPathView {
	v := dto.LearningPathView{PathID: p.ID, CourseID: courseID, Title: p.Title, UnlockMode: p.UnlockMode, Nodes: []dto.PathNodeView{},
		IsTemplate: p.TenantID == nil, TextbookSeriesID: p.TextbookSeriesID, Version: p.Version, UpdatedAt: p.UpdatedAt}
	for _, n := range p.Nodes {
		nv := dto.PathNodeView{NodeID: n.ID, Position: n.Position, ContentRef: dto.ContentRefView{ContentType: n.ContentType,
			ContentVersionID: n.ContentVersionID}, ContentID: n.ContentID, Title: n.Title, ObjectiveIDs: n.ObjectiveIDs}
		if len(n.UnlockRule) > 0 && string(n.UnlockRule) != "{}" && string(n.UnlockRule) != "null" {
			nv.UnlockRule = n.UnlockRule
		}
		v.Nodes = append(v.Nodes, nv)
	}
	return v
}

// CreatePath creates the learning path of a course (F040-AC-06, AC-07): lms.learning_path.write in the school and
// subject, nodes pinned to published versions visible to the tenant, Idempotency-Key, audit learning_path.created; a
// second active path of the course is 409 CONFLICT (update it instead) [Proposed].
func (h LMS) CreatePath(ctx context.Context, courseID uuid.UUID, in PathInput) (out dto.LearningPathView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.CreateLearningPath")
	defer func() { done(err) }()
	p, err := staffOf(ctx)
	if err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		c, err := h.course(ctx, p, authz.PermLmsLearningPathWrite, courseID)
		if err != nil {
			return err
		}
		nodes, err := h.nodesOf(ctx, in)
		if err != nil {
			return err
		}
		for i := range nodes {
			nodes[i].ID = ids.New()
		}
		tenant, school, subject, grade := *p.TenantID, c.SchoolID, c.SubjectID, c.Grade
		row := write.PathRow{ID: ids.New(), TenantID: &tenant, CourseID: &courseID, SchoolID: &school, TextbookSeriesID: c.TextbookSeriesID,
			SubjectID: &subject, Grade: &grade, Title: strings.TrimSpace(in.Title), UnlockMode: in.UnlockMode, Status: "active", By: p.UserID,
			Version: 1, UpdatedAt: h.now(), Nodes: nodes}
		if err := h.Store.InsertPath(ctx, row); err != nil {
			return err
		}
		if err := h.audit(ctx, "learning_path.created", "learning_path", row.ID, school, "", map[string]any{"course_id": courseID.String(),
			"nodes": len(nodes)}); err != nil {
			return err
		}
		out = PathView(row, courseID)
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// UpdatePath saves a learning path directly (F040-AC-06, RB-216): no content version nor workflow, If-Match, audit; a
// node keeping its content keeps its node_id (progress and unlocks stay).
func (h LMS) UpdatePath(ctx context.Context, courseID, pathID uuid.UUID, in PathInput, ifMatch int64) (out dto.LearningPathView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.UpdateLearningPath")
	defer func() { done(err) }()
	p, err := staffOf(ctx)
	if err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		c, err := h.course(ctx, p, authz.PermLmsLearningPathWrite, courseID)
		if err != nil {
			return err
		}
		old, err := h.Store.Path(ctx, pathID)
		if err != nil {
			return err
		}
		if old == nil || old.CourseID == nil || *old.CourseID != courseID {
			return problem.New(problem.NotFound, "")
		}
		nodes, err := h.nodesOf(ctx, in)
		if err != nil {
			return err
		}
		used := map[uuid.UUID]bool{}
		for i := range nodes {
			for _, o := range old.Nodes {
				if o.ContentID == nodes[i].ContentID && !used[o.ID] {
					nodes[i].ID, used[o.ID] = o.ID, true
					break
				}
			}
		}
		next := *old
		next.Title, next.UnlockMode, next.By, next.Nodes = strings.TrimSpace(in.Title), in.UnlockMode, p.UserID, nodes
		ok, err := h.Store.UpdatePath(ctx, next, ifMatch)
		if err != nil {
			return err
		}
		if !ok {
			return problem.New(problem.VersionMismatch, "")
		}
		if err := h.audit(ctx, "learning_path.updated", "learning_path", pathID, c.SchoolID, "", map[string]any{"course_id": courseID.String(),
			"nodes": len(nodes)}); err != nil {
			return err
		}
		saved, err := h.Store.Path(ctx, pathID)
		if err != nil {
			return err
		}
		out = PathView(*saved, courseID)
		return nil
	})
	return out, err
}

// UnlockNode opens a node of a path for one student of the class of the course (F040-AC-10): a reason, the student in
// the class (404 otherwise), Idempotency-Key, audit path_node.unlocked with the reason.
func (h LMS) UnlockNode(ctx context.Context, courseID, pathID, nodeID, studentID uuid.UUID, reason string) (err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.UnlockPathNode")
	defer func() { done(err) }()
	p, err := staffOf(ctx)
	if err != nil {
		return err
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return problem.Invalid("/reason", problem.SubRequired, "a reason is required")
	}
	return h.UoW.Do(ctx, func(ctx context.Context) error {
		c, err := h.course(ctx, p, authz.PermLmsLearningPathWrite, courseID)
		if err != nil {
			return err
		}
		path, err := h.Store.Path(ctx, pathID)
		if err != nil {
			return err
		}
		if path == nil || path.CourseID == nil || *path.CourseID != courseID ||
			!slices.ContainsFunc(path.Nodes, func(n write.NodeRow) bool { return n.ID == nodeID }) {
			return problem.New(problem.NotFound, "")
		}
		in, err := h.Store.InClass(ctx, c.ClassID, []uuid.UUID{studentID})
		if err != nil {
			return err
		}
		if len(in) == 0 {
			return problem.New(problem.NotFound, "")
		}
		if _, err := h.Store.InsertUnlock(ctx, *p.TenantID, c.SchoolID, pathID, nodeID, studentID, reason, p.UserID); err != nil {
			return err
		}
		if h.Metrics != nil {
			h.Metrics.NodeUnlocked("manual")
		}
		return h.audit(ctx, "path_node.unlocked", "path_node", nodeID, c.SchoolID, reason, map[string]any{"student_id": studentID.String(),
			"path_id": pathID.String()})
	})
}

// ---------- schedule

// ScheduleInput is ScheduleItemInput.
type ScheduleInput struct {
	CourseID uuid.UUID  `json:"course_id"`
	Date     string     `json:"date"`
	Period   *int       `json:"period"`
	Kind     string     `json:"kind"`
	RefID    *uuid.UUID `json:"ref_id"`
	Note     *string    `json:"note"`
}

func scheduleView(r write.ScheduleRow) dto.ScheduleItemView {
	return dto.ScheduleItemView{ScheduleItemID: r.ID, CourseID: r.CourseID, SchoolID: r.SchoolID, Date: r.Date, Period: r.Period, Kind: r.Kind,
		RefType: r.RefType, RefID: r.RefID, Note: r.Note, Version: r.Version}
}

func validDate(s string) error {
	if _, err := time.Parse(time.DateOnly, s); err != nil {
		return problem.Invalid("/date", problem.SubInvalid, "is not a date")
	}
	return nil
}

// CreateSchedule adds a schedule item to a course (F040-AC-13): lms.schedule.write in the school and subject, audit.
func (h LMS) CreateSchedule(ctx context.Context, in ScheduleInput) (out dto.ScheduleItemView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.CreateScheduleItem")
	defer func() { done(err) }()
	p, err := staffOf(ctx)
	if err != nil {
		return out, err
	}
	if err := validDate(in.Date); err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		c, err := h.course(ctx, p, authz.PermLmsScheduleWrite, in.CourseID)
		if err != nil {
			return err
		}
		by := p.UserID
		r := write.ScheduleRow{ID: ids.New(), TenantID: *p.TenantID, SchoolID: c.SchoolID, CourseID: c.ID, Date: in.Date, Period: in.Period,
			Kind: in.Kind, RefID: in.RefID, Note: in.Note, CreatedBy: &by, Version: 1}
		if _, err := h.Store.InsertSchedule(ctx, r); err != nil {
			return err
		}
		if err := h.audit(ctx, "schedule_item.created", "schedule_item", r.ID, c.SchoolID, "", map[string]any{"course_id": c.ID.String(),
			"kind": in.Kind}); err != nil {
			return err
		}
		out = scheduleView(r)
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// UpdateSchedule changes a schedule item (F040-AC-13): the item and the target course in scope, If-Match, audit.
func (h LMS) UpdateSchedule(ctx context.Context, id uuid.UUID, in ScheduleInput, ifMatch int64) (out dto.ScheduleItemView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.UpdateScheduleItem")
	defer func() { done(err) }()
	p, err := staffOf(ctx)
	if err != nil {
		return out, err
	}
	if err := validDate(in.Date); err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		r, err := h.Store.Schedule(ctx, id)
		if err != nil {
			return err
		}
		if r == nil {
			return problem.New(problem.NotFound, "")
		}
		if _, err := h.course(ctx, p, authz.PermLmsScheduleWrite, r.CourseID); err != nil {
			return err
		}
		c, err := h.course(ctx, p, authz.PermLmsScheduleWrite, in.CourseID)
		if err != nil {
			return err
		}
		r.CourseID, r.SchoolID, r.Date, r.Period, r.Kind, r.RefID, r.Note = c.ID, c.SchoolID, in.Date, in.Period, in.Kind, in.RefID, in.Note
		ok, err := h.Store.UpdateSchedule(ctx, *r, ifMatch)
		if err != nil {
			return err
		}
		if !ok {
			return problem.New(problem.VersionMismatch, "")
		}
		r.Version = ifMatch + 1
		if err := h.audit(ctx, "schedule_item.updated", "schedule_item", id, c.SchoolID, "", nil); err != nil {
			return err
		}
		out = scheduleView(*r)
		return nil
	})
	return out, err
}

// DeleteSchedule soft-deletes a schedule item (F040-AC-14): audit, schedule_item.deleted, gone from every read.
func (h LMS) DeleteSchedule(ctx context.Context, id uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.DeleteScheduleItem")
	defer func() { done(err) }()
	p, err := staffOf(ctx)
	if err != nil {
		return err
	}
	return h.UoW.Do(ctx, func(ctx context.Context) error {
		r, err := h.Store.Schedule(ctx, id)
		if err != nil {
			return err
		}
		if r == nil {
			return problem.New(problem.NotFound, "")
		}
		if _, err := h.course(ctx, p, authz.PermLmsScheduleWrite, r.CourseID); err != nil {
			return err
		}
		if _, err := h.Store.DeleteSchedule(ctx, id, p.UserID, "user_request"); err != nil {
			return err
		}
		if err := h.audit(ctx, "schedule_item.deleted", "schedule_item", id, r.SchoolID, "", nil); err != nil {
			return err
		}
		return h.publish(ctx, EventScheduleItemDeleted, "schedule_item", id, r.TenantID, map[string]any{"schedule_item_id": id,
			"course_id": r.CourseID, "school_id": r.SchoolID, "tenant_id": r.TenantID, "deleted_by": p.UserID, "reason": "user_request"})
	})
}

// ScheduleFromEvent is a due date or an exam of an event of F041/F042 for a course.
type ScheduleFromEvent struct {
	EventID  uuid.UUID
	TenantID uuid.UUID
	CourseID uuid.UUID
	Kind     string // assignment_due or exam
	Date     string
	RefType  string
	RefID    uuid.UUID
}

// UpsertScheduleFromEvent adds the schedule item of an assignment or exam of a course once (F040-AC-15): keyed by the
// source event (and redelivery is processed once by the consumer); an unknown course adds nothing.
func (h LMS) UpsertScheduleFromEvent(ctx context.Context, e ScheduleFromEvent) (err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.UpsertScheduleFromEvent")
	defer func() { done(err) }()
	c, err := h.Store.Course(ctx, e.CourseID)
	if err != nil || c == nil {
		return err
	}
	src, ref, refType := e.EventID, e.RefID, e.RefType
	_, err = h.Store.InsertSchedule(ctx, write.ScheduleRow{ID: ids.New(), TenantID: e.TenantID, SchoolID: c.SchoolID, CourseID: c.ID,
		Date: e.Date, Kind: e.Kind, RefType: &refType, RefID: &ref, SourceEventID: &src, Version: 1})
	return err
}

// ---------- study groups

// GroupInput is StudyGroupInput.
type GroupInput struct {
	Name       string      `json:"name"`
	ClassID    *uuid.UUID  `json:"class_id"`
	GradeLevel *int        `json:"grade_level"`
	MemberIDs  []uuid.UUID `json:"member_ids"`
}

// GroupView is the StudyGroup of a stored group.
func GroupView(g write.StudyGroupRow) dto.StudyGroupView {
	return dto.StudyGroupView{StudyGroupID: g.ID, SchoolID: g.SchoolID, Name: g.Name, ClassID: g.ClassID, GradeLevel: g.Grade, MemberIDs: g.Members,
		MemberCount: len(g.Members), CreatedBy: g.CreatedBy, UpdatedAt: g.UpdatedAt, Version: g.Version}
}

// checkGroup validates a study group of a school (F040-AC-22): name and members, the class of the school (its grade),
// otherwise a grade; members active students of the school, and of the class when there is one; the error never
// says which student failed (no leak of students of another school).
func (h LMS) checkGroup(ctx context.Context, school uuid.UUID, in GroupInput) (int, error) {
	if err := violations(lms.ValidateGroup(in.Name, in.MemberIDs)); err != nil {
		return 0, err
	}
	grade := 0
	if in.ClassID != nil {
		c, err := h.Store.Class(ctx, *in.ClassID)
		if err != nil {
			return 0, err
		}
		if c == nil || c.SchoolID != school {
			return 0, problem.Invalid("/class_id", problem.SubInvalid, "is not a class of the school")
		}
		grade = c.Grade
	} else {
		if in.GradeLevel == nil {
			return 0, problem.Invalid("/grade_level", problem.SubRequired, "a group without class needs a grade")
		}
		grade = *in.GradeLevel
	}
	active, err := h.Store.ActiveStudents(ctx, school, in.MemberIDs)
	if err != nil {
		return 0, err
	}
	if len(active) != len(in.MemberIDs) {
		return 0, problem.Invalid("/member_ids", problem.SubInvalid, "every member is an active student of the school")
	}
	if in.ClassID != nil {
		inClass, err := h.Store.InClass(ctx, *in.ClassID, in.MemberIDs)
		if err != nil {
			return 0, err
		}
		if len(inClass) != len(in.MemberIDs) {
			return 0, problem.Invalid("/member_ids", problem.SubInvalid, "every member studies in the class")
		}
	}
	return grade, nil
}

// CreateStudyGroup creates a study group of a school (F040-AC-17, AC-21…AC-23): lms.study_group.write in the school,
// Idempotency-Key, outbox study_group.created and audit in the transaction; a duplicate name is 409 CONFLICT.
func (h LMS) CreateStudyGroup(ctx context.Context, school uuid.UUID, in GroupInput) (out dto.StudyGroupView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.CreateStudyGroup")
	defer func() { done(err) }()
	p, err := staffOf(ctx)
	if err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := AuthorizeLMS(ctx, p, authz.PermLmsStudyGroupWrite, school, nil); err != nil {
			return err
		}
		grade, err := h.checkGroup(ctx, school, in)
		if err != nil {
			return err
		}
		g := write.StudyGroupRow{ID: ids.New(), TenantID: *p.TenantID, SchoolID: school, ClassID: in.ClassID, Grade: grade,
			Name: strings.TrimSpace(in.Name), Members: in.MemberIDs, CreatedBy: p.UserID, UpdatedBy: p.UserID, Version: 1, UpdatedAt: h.now()}
		if err := h.Store.InsertGroup(ctx, g); err != nil {
			return err
		}
		if err := h.publish(ctx, EventStudyGroupCreated, "study_group", g.ID, g.TenantID, map[string]any{"study_group_id": g.ID,
			"school_id": school, "class_id": g.ClassID, "grade_level": grade, "name": g.Name, "member_ids": g.Members, "version": 1,
			"created_by": p.UserID}); err != nil {
			return err
		}
		if err := h.audit(ctx, "study_group.created", "study_group", g.ID, school, "", map[string]any{"member_count": len(g.Members)}); err != nil {
			return err
		}
		out = GroupView(g)
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// UpdateStudyGroup renames a group or replaces its members (F040-AC-19, AC-21…AC-23): If-Match, the removed members
// soft-deleted, study_group.updated and/or study_group.member_changed (ids only) and audit in the transaction.
func (h LMS) UpdateStudyGroup(ctx context.Context, school, id uuid.UUID, in GroupInput, ifMatch int64) (out dto.StudyGroupView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.UpdateStudyGroup")
	defer func() { done(err) }()
	p, err := staffOf(ctx)
	if err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := AuthorizeLMS(ctx, p, authz.PermLmsStudyGroupWrite, school, nil); err != nil {
			return err
		}
		g, err := h.Store.Group(ctx, school, id)
		if err != nil {
			return err
		}
		if g == nil {
			return problem.New(problem.NotFound, "")
		}
		grade, err := h.checkGroup(ctx, school, in)
		if err != nil {
			return err
		}
		added, removed := lms.Diff(g.Members, in.MemberIDs)
		next := *g
		next.Name, next.ClassID, next.Grade, next.Members, next.UpdatedBy = strings.TrimSpace(in.Name), in.ClassID, grade, in.MemberIDs, p.UserID
		ok, err := h.Store.UpdateGroup(ctx, next, added, removed, ifMatch)
		if err != nil {
			return err
		}
		if !ok {
			return problem.New(problem.VersionMismatch, "")
		}
		next.Version, next.UpdatedAt = ifMatch+1, h.now()
		if next.Name != g.Name || next.Grade != g.Grade || !sameID(next.ClassID, g.ClassID) {
			if err := h.publish(ctx, EventStudyGroupUpdated, "study_group", id, g.TenantID, map[string]any{"study_group_id": id,
				"school_id": school, "class_id": next.ClassID, "grade_level": grade, "name": next.Name, "version": next.Version,
				"updated_by": p.UserID}); err != nil {
				return err
			}
		}
		if len(added)+len(removed) > 0 {
			if err := h.publish(ctx, EventStudyGroupMemberChanged, "study_group", id, g.TenantID, map[string]any{"study_group_id": id,
				"school_id": school, "added_member_ids": added, "removed_member_ids": removed, "member_count": len(next.Members),
				"version": next.Version, "changed_by": p.UserID}); err != nil {
				return err
			}
		}
		if err := h.audit(ctx, "study_group.updated", "study_group", id, school, "", map[string]any{"added": len(added),
			"removed": len(removed)}); err != nil {
			return err
		}
		out = GroupView(next)
		return nil
	})
	return out, err
}

func sameID(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// DeleteStudyGroup soft-deletes a group and its members (F040-AC-20, AC-23): study_group.deleted and audit.
func (h LMS) DeleteStudyGroup(ctx context.Context, school, id uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.DeleteStudyGroup")
	defer func() { done(err) }()
	p, err := staffOf(ctx)
	if err != nil {
		return err
	}
	return h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := AuthorizeLMS(ctx, p, authz.PermLmsStudyGroupWrite, school, nil); err != nil {
			return err
		}
		g, err := h.Store.Group(ctx, school, id)
		if err != nil {
			return err
		}
		if g == nil {
			return problem.New(problem.NotFound, "")
		}
		if err := h.Store.DeleteGroup(ctx, *g, p.UserID); err != nil {
			return err
		}
		if err := h.publish(ctx, EventStudyGroupDeleted, "study_group", id, g.TenantID, map[string]any{"study_group_id": id,
			"school_id": school, "version": g.Version + 1, "deleted_by": p.UserID}); err != nil {
			return err
		}
		return h.audit(ctx, "study_group.deleted", "study_group", id, school, "", nil)
	})
}

// RemoveScheduleOfRef soft-deletes the schedule items of a cancelled assignment (F041-1, consumer learning.lms_schedule):
// schedule_item.deleted per item with reason cancelled; nothing left → nothing (idempotent).
func (h LMS) RemoveScheduleOfRef(ctx context.Context, refType string, refID, by uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.RemoveScheduleOfRef")
	defer func() { done(err) }()
	items, err := h.Store.ScheduleOfRef(ctx, refType, refID)
	if err != nil {
		return err
	}
	for _, r := range items {
		ok, err := h.Store.DeleteSchedule(ctx, r.ID, by, "cancelled")
		if err != nil || !ok {
			return err
		}
		if err := h.publish(ctx, EventScheduleItemDeleted, "schedule_item", r.ID, r.TenantID, map[string]any{"schedule_item_id": r.ID,
			"course_id": r.CourseID, "school_id": r.SchoolID, "tenant_id": r.TenantID, "deleted_by": by, "reason": "cancelled"}); err != nil {
			return err
		}
	}
	return nil
}
