package query

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/lms"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/mastery"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// LMS reads the school LMS of F040: staff reads in the scope of their grants (school, subject), student reads of their
// own classes. No write, no event (R26).
type LMS struct {
	Session  read.Session
	Model    read.LMSReadModel
	Progress read.ProgressReadModel
	Cursors  *page.Codec
	Clock    func() time.Time
}

func (q LMS) now() time.Time {
	if q.Clock != nil {
		return q.Clock().UTC()
	}
	return time.Now().UTC()
}

func staffPrincipal(ctx context.Context) (authz.Principal, error) {
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

// visible reports whether a grant holding perm covers the school and the subject.
func visible(p authz.Principal, perm authz.Permission, school uuid.UUID, subject *uuid.UUID) bool {
	for _, g := range p.Grants {
		if !holdsIn(g.Permissions, perm) {
			continue
		}
		if (g.SchoolID == nil || *g.SchoolID == school) && (g.SubjectID == nil || (subject != nil && *g.SubjectID == *subject)) {
			return true
		}
	}
	return false
}

func holdsIn(perms []authz.Permission, perm authz.Permission) bool {
	for _, x := range perms {
		if x == perm {
			return true
		}
	}
	return false
}

// authorizeSchool is command.AuthorizeLMS for the reads: 403 without the permission, 404 for a school outside every
// grant, 403 for a subject outside a subject-limited grant.
func authorizeSchool(ctx context.Context, p authz.Principal, perm authz.Permission, school uuid.UUID, subject *uuid.UUID) error {
	if !holds(p, perm) {
		return authz.Require(ctx, p, perm, authz.Resource{TenantID: p.TenantID, SchoolID: &school, SubjectID: subject})
	}
	if !coversSchool(p, perm, school) {
		_ = authz.Require(ctx, p, perm, authz.Resource{TenantID: p.TenantID, SchoolID: &school})
		return problem.New(problem.NotFound, "")
	}
	return authz.Require(ctx, p, perm, authz.Resource{TenantID: p.TenantID, SchoolID: &school, SubjectID: subject})
}

// Courses is listCourses (F040-AC-04): the courses of the tenant the grants of lms.class.read cover (School Admin: its
// school; Academic Manager: its scope; Tenant Admin: the tenant, read only).
func (q LMS) Courses(ctx context.Context, f read.CourseFilter, in PageInput) (out Page[dto.CourseView], err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListCourses")
	defer func() { done(err) }()
	p, err := staffPrincipal(ctx)
	if err != nil {
		return out, err
	}
	if !holds(p, authz.PermLmsClassRead) {
		return out, authz.Require(ctx, p, authz.PermLmsClassRead, authz.Resource{TenantID: p.TenantID})
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		cs, err := q.Model.Courses(ctx, f)
		if err != nil {
			return err
		}
		var mine []dto.CourseView
		for _, c := range cs {
			subject := c.SubjectID
			if visible(p, authz.PermLmsClassRead, c.SchoolID, &subject) {
				mine = append(mine, c)
			}
		}
		out, err = paginate(mine, func(c dto.CourseView) uuid.UUID { return c.CourseID }, in, q.Cursors)
		return err
	})
	return out, err
}

func (q LMS) course(ctx context.Context, p authz.Principal, id uuid.UUID) (*dto.CourseView, error) {
	c, err := q.Model.Course(ctx, id)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, problem.New(problem.NotFound, "")
	}
	subject := c.SubjectID
	if err := authorizeSchool(ctx, p, authz.PermLmsClassRead, c.SchoolID, &subject); err != nil {
		return nil, err
	}
	return c, nil
}

// Paths is listLearningPaths (F040-AC-08): the own path of the course, otherwise the template of its textbook series.
func (q LMS) Paths(ctx context.Context, courseID uuid.UUID) (out []dto.LearningPathView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListLearningPaths")
	defer func() { done(err) }()
	p, err := staffPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	out = []dto.LearningPathView{}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		c, err := q.course(ctx, p, courseID)
		if err != nil {
			return err
		}
		path, err := q.Model.CoursePath(ctx, *c)
		if err != nil || path == nil {
			return err
		}
		out = append(out, *path)
		return nil
	})
	return out, err
}

// ScheduleInput are the filters of listScheduleItems.
type ScheduleInput struct {
	CourseID *uuid.UUID
	ClassID  *uuid.UUID
	From, To string
}

func dateRange(from, to string, maxDays int) error {
	f, err1 := time.Parse(time.DateOnly, from)
	t, err2 := time.Parse(time.DateOnly, to)
	if err1 != nil || err2 != nil {
		return problem.Invalid("/from", problem.SubInvalid, "from and to are dates")
	}
	if t.Before(f) {
		return problem.Invalid("/to", problem.SubInvalid, "is before from")
	}
	if maxDays > 0 && t.Sub(f) > time.Duration(maxDays)*24*time.Hour {
		return problem.Invalid("/to", problem.SubInvalid, "the range is at most 62 days")
	}
	return nil
}

// Schedule is listScheduleItems (F040-AC-13): the items of the courses in scope in the date range.
func (q LMS) Schedule(ctx context.Context, in ScheduleInput, pg PageInput) (out Page[dto.ScheduleItemView], err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListScheduleItems")
	defer func() { done(err) }()
	p, err := staffPrincipal(ctx)
	if err != nil {
		return out, err
	}
	if err := dateRange(in.From, in.To, 0); err != nil {
		return out, err
	}
	if !holds(p, authz.PermLmsClassRead) {
		return out, authz.Require(ctx, p, authz.PermLmsClassRead, authz.Resource{TenantID: p.TenantID})
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		f := read.ScheduleFilter{ClassID: in.ClassID, From: in.From, To: in.To}
		if in.CourseID != nil {
			if _, err := q.course(ctx, p, *in.CourseID); err != nil {
				return err
			}
			f.CourseIDs = []uuid.UUID{*in.CourseID}
		}
		items, err := q.Model.Schedule(ctx, f)
		if err != nil {
			return err
		}
		var mine []dto.ScheduleItemView
		for _, s := range items {
			subject := s.SubjectID
			if visible(p, authz.PermLmsClassRead, s.SchoolID, &subject) {
				mine = append(mine, s)
			}
		}
		out, err = paginate(mine, func(s dto.ScheduleItemView) uuid.UUID { return s.ScheduleItemID }, pg, q.Cursors)
		return err
	})
	return out, err
}

// StudyGroups is listStudyGroups (F040-AC-18).
func (q LMS) StudyGroups(ctx context.Context, school uuid.UUID, classID *uuid.UUID, grade *int, in PageInput) (out Page[dto.StudyGroupView], err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListStudyGroups")
	defer func() { done(err) }()
	p, err := staffPrincipal(ctx)
	if err != nil {
		return out, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		if err := authorizeSchool(ctx, p, authz.PermLmsClassRead, school, nil); err != nil {
			return err
		}
		gs, err := q.Model.StudyGroups(ctx, school, classID, grade)
		if err != nil {
			return err
		}
		out, err = paginate(gs, func(g dto.StudyGroupView) uuid.UUID { return g.StudyGroupID }, in, q.Cursors)
		return err
	})
	return out, err
}

// StudyGroup is getStudyGroup (F040-AC-18): with member_ids and the version for the ETag.
func (q LMS) StudyGroup(ctx context.Context, school, id uuid.UUID) (out dto.StudyGroupView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.GetStudyGroup")
	defer func() { done(err) }()
	p, err := staffPrincipal(ctx)
	if err != nil {
		return out, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		if err := authorizeSchool(ctx, p, authz.PermLmsClassRead, school, nil); err != nil {
			return err
		}
		g, err := q.Model.StudyGroup(ctx, school, id)
		if err != nil {
			return err
		}
		if g == nil {
			return problem.New(problem.NotFound, "")
		}
		out = *g
		return nil
	})
	return out, err
}

// CurriculumSettings is getSchoolCurriculumSettings (F040-AC-03).
func (q LMS) CurriculumSettings(ctx context.Context, school uuid.UUID, yearID *uuid.UUID) (out dto.CurriculumSettingsView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.GetSchoolCurriculum")
	defer func() { done(err) }()
	p, err := staffPrincipal(ctx)
	if err != nil {
		return out, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		if err := authorizeSchool(ctx, p, authz.PermLmsClassRead, school, nil); err != nil {
			return err
		}
		out, err = q.Model.CurriculumSettings(ctx, school, yearID)
		return err
	})
	return out, err
}

// studentCourses are the active courses of the classes of the student of the session.
func (q LMS) studentCourses(ctx context.Context, studentID uuid.UUID, subject *uuid.UUID) ([]dto.CourseView, error) {
	classes, err := q.Model.StudentClasses(ctx, studentID)
	if err != nil || len(classes) == 0 {
		return nil, err
	}
	return q.Model.Courses(ctx, read.CourseFilter{ClassIDs: classes, SubjectID: subject, Status: lms.CourseActive})
}

// MyLearningPaths is listMyLearningPaths (F040-AC-09, AC-11): the paths of the active courses of the student's classes
// with the state of each node (unlock mode, manual unlocks, completion, mastery after decay).
func (q LMS) MyLearningPaths(ctx context.Context, subject *uuid.UUID) (out dto.MyLearningPathsView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListMyLearningPaths")
	defer func() { done(err) }()
	p, err := student(ctx)
	if err != nil {
		return out, err
	}
	out.Items = []dto.MyLearningPathView{}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		courses, err := q.studentCourses(ctx, p.UserID, subject)
		if err != nil || len(courses) == 0 {
			return err
		}
		levelOf, err := q.levels(ctx, p.UserID)
		if err != nil {
			return err
		}
		for _, c := range courses {
			path, err := q.Model.CoursePath(ctx, c)
			if err != nil {
				return err
			}
			if path == nil {
				continue
			}
			item, err := q.myPath(ctx, p.UserID, c, *path, levelOf)
			if err != nil {
				return err
			}
			out.Items = append(out.Items, item)
		}
		return nil
	})
	return out, err
}

// levels returns the level after decay of the objectives of a student.
func (q LMS) levels(ctx context.Context, studentID uuid.UUID) (func(uuid.UUID) string, error) {
	cfg, err := q.Progress.ActiveModel(ctx)
	if err != nil {
		return nil, err
	}
	params := mastery.DefaultParams()
	if len(cfg.Params) > 0 {
		if err := json.Unmarshal(cfg.Params, &params); err != nil {
			return nil, err
		}
	}
	m := mastery.WeightedEvidence{ModelVersion: cfg.ModelVersion, P: params}
	ms, err := q.Progress.Masteries(ctx, studentID, nil)
	if err != nil {
		return nil, err
	}
	now, levels := q.now(), map[uuid.UUID]string{}
	for _, o := range ms {
		_, l := m.Effective(o.State, now)
		levels[o.ObjectiveID] = string(l)
	}
	return func(id uuid.UUID) string {
		if l, ok := levels[id]; ok {
			return l
		}
		return string(mastery.NotStarted)
	}, nil
}

func (q LMS) myPath(ctx context.Context, studentID uuid.UUID, c dto.CourseView, path dto.LearningPathView, levelOf func(uuid.UUID) string) (
	dto.MyLearningPathView, error) {
	item := dto.MyLearningPathView{CourseID: c.CourseID, SubjectID: c.SubjectID, PathID: path.PathID, Title: path.Title, UnlockMode: path.UnlockMode,
		IsTemplate: path.IsTemplate, Nodes: []dto.MyPathNodeView{}}
	contents, nodeIDs := make([]uuid.UUID, 0, len(path.Nodes)), make([]uuid.UUID, 0, len(path.Nodes))
	for _, n := range path.Nodes {
		contents, nodeIDs = append(contents, n.ContentID), append(nodeIDs, n.NodeID)
	}
	done, err := q.Model.CompletedContent(ctx, studentID, contents)
	if err != nil {
		return item, err
	}
	unlocked, err := q.Model.UnlockedNodes(ctx, studentID, nodeIDs)
	if err != nil {
		return item, err
	}
	progress := make([]lms.NodeProgress, len(path.Nodes))
	for i, n := range path.Nodes {
		progress[i] = lms.NodeProgress{NodeID: n.NodeID, Position: n.Position, ObjectiveIDs: n.ObjectiveIDs, Completed: done[n.ContentID],
			Unlocked: unlocked[n.NodeID]}
	}
	states := lms.NodeStates(path.UnlockMode, progress, levelOf)
	for i, n := range path.Nodes {
		item.Nodes = append(item.Nodes, dto.MyPathNodeView{NodeID: n.NodeID, Position: n.Position, ContentType: n.ContentRef.ContentType,
			ContentID: n.ContentID, ContentVersionID: n.ContentRef.ContentVersionID, Title: n.Title, State: states[i]})
	}
	return item, nil
}

// MySchedule is listMySchedule (F040-AC-16): the items of the active courses of the student's classes, ≤ 62 days.
func (q LMS) MySchedule(ctx context.Context, from, to string) (out dto.MyScheduleView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListMySchedule")
	defer func() { done(err) }()
	p, err := student(ctx)
	if err != nil {
		return out, err
	}
	if err := dateRange(from, to, lms.MaxScheduleDays); err != nil {
		return out, err
	}
	out.Items = []dto.MyScheduleItemView{}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		courses, err := q.studentCourses(ctx, p.UserID, nil)
		if err != nil || len(courses) == 0 {
			return err
		}
		ids := make([]uuid.UUID, len(courses))
		for i, c := range courses {
			ids[i] = c.CourseID
		}
		items, err := q.Model.Schedule(ctx, read.ScheduleFilter{CourseIDs: ids, From: from, To: to})
		if err != nil {
			return err
		}
		for _, s := range items {
			out.Items = append(out.Items, dto.MyScheduleItemView{ScheduleItemID: s.ScheduleItemID, CourseID: s.CourseID, SubjectID: s.SubjectID,
				Date: s.Date, Period: s.Period, Kind: s.Kind, RefType: s.RefType, RefID: s.RefID})
		}
		return nil
	})
	return out, err
}
