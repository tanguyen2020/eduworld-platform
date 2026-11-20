package query

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/assignment"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/i18ntext"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// SchoolAssignments reads the school assignments of F041: staff reads in the scope of their grants (school, subject),
// the assignments of the student, the catalog of game activities. No write, no event (R26).
type SchoolAssignments struct {
	Session read.Session
	Model   read.SchoolAssignmentReadModel
	Cursors *page.Codec
	Clock   func() time.Time
}

// keyset decodes the cursor (the id of the last row) of a keyset list.
func keyset(in PageInput, codec *page.Codec) (*uuid.UUID, int, error) {
	cur, err := codec.Decode(in.Cursor)
	if err != nil {
		return nil, 0, problem.New(problem.BadRequest, "invalid cursor")
	}
	limit := page.Limit(in.Limit)
	if cur == nil {
		return nil, limit, nil
	}
	id := cur.ID
	return &id, limit, nil
}

// pageOf builds a page of a keyset list read with limit + 1 rows.
func pageOf[T any](rows []T, limit int, id func(T) uuid.UUID, codec *page.Codec) Page[T] {
	out := Page[T]{Data: rows, Limit: limit}
	if len(rows) > limit {
		out.Data, out.HasMore = rows[:limit], true
		s := codec.Encode(page.Cursor{ID: id(rows[limit-1])})
		out.NextCursor = &s
	}
	if out.Data == nil {
		out.Data = []T{}
	}
	return out
}

// scopes are the (school, subject) pairs the grants holding perm cover.
func scopes(p authz.Principal, perm authz.Permission) []read.AssignmentScope {
	var out []read.AssignmentScope
	for _, g := range p.Grants {
		if holdsIn(g.Permissions, perm) {
			out = append(out, read.AssignmentScope{SchoolID: g.SchoolID, SubjectID: g.SubjectID})
		}
	}
	return out
}

// AssignmentListInput are the filters of listAssignments.
type AssignmentListInput struct {
	ClassID  *uuid.UUID
	CourseID *uuid.UUID
	Status   string
}

// Assignments is listAssignments (F041-AC-26): lms.assignment.read; the assignments of the schools and subjects of the
// grants of the reader in the session tenant (tenant_ids of a tenant user are ignored), with their completion.
func (q SchoolAssignments) Assignments(ctx context.Context, in AssignmentListInput, pg PageInput) (out Page[dto.AssignmentView], err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListAssignments")
	defer func() { done(err) }()
	p, err := staffPrincipal(ctx)
	if err != nil {
		return out, err
	}
	if !holds(p, authz.PermLmsAssignmentRead) {
		return out, authz.Require(ctx, p, authz.PermLmsAssignmentRead, authz.Resource{TenantID: p.TenantID})
	}
	after, limit, err := keyset(pg, q.Cursors)
	if err != nil {
		return out, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		recs, err := q.Model.Assignments(ctx, read.AssignmentFilter{Scopes: scopes(p, authz.PermLmsAssignmentRead), ClassID: in.ClassID,
			CourseID: in.CourseID, Status: in.Status, After: after, Limit: limit + 1})
		if err != nil {
			return err
		}
		views, err := q.withCompletion(ctx, recs)
		if err != nil {
			return err
		}
		out = pageOf(views, limit, func(v dto.AssignmentView) uuid.UUID { return v.AssignmentID }, q.Cursors)
		return nil
	})
	return out, err
}

func (q SchoolAssignments) withCompletion(ctx context.Context, recs []read.AssignmentRecord) ([]dto.AssignmentView, error) {
	idList := make([]uuid.UUID, len(recs))
	for i, r := range recs {
		idList[i] = r.View.AssignmentID
	}
	comp, err := q.Model.Completion(ctx, idList)
	if err != nil {
		return nil, err
	}
	out := make([]dto.AssignmentView, len(recs))
	for i, r := range recs {
		out[i] = r.View
		c := comp[r.View.AssignmentID]
		out[i].Completion = &c
	}
	return out, nil
}

// readable reads an assignment and authorizes perm on its school and subject (404 outside the schools of the reader).
func (q SchoolAssignments) readable(ctx context.Context, p authz.Principal, perm authz.Permission, id uuid.UUID) (*read.AssignmentRecord, error) {
	r, err := q.Model.Assignment(ctx, id)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, problem.New(problem.NotFound, "")
	}
	if err := authorizeSchool(ctx, p, perm, r.View.SchoolID, &r.SubjectID); err != nil {
		return nil, err
	}
	return r, nil
}

// Assignment is getAssignment (F041-AC-26): with its completion and the row version for the ETag.
func (q SchoolAssignments) Assignment(ctx context.Context, id uuid.UUID) (out dto.AssignmentView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.GetAssignment")
	defer func() { done(err) }()
	p, err := staffPrincipal(ctx)
	if err != nil {
		return out, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		r, err := q.readable(ctx, p, authz.PermLmsAssignmentRead, id)
		if err != nil {
			return err
		}
		views, err := q.withCompletion(ctx, []read.AssignmentRecord{*r})
		if err != nil {
			return err
		}
		out = views[0]
		return nil
	})
	return out, err
}

// SubmissionListInput are the filters of listAssignmentSubmissions.
type SubmissionListInput struct {
	Status  string
	ClassID *uuid.UUID
}

// Submissions is listAssignmentSubmissions (F041-AC-25): every targeted student with status, score, attempts, submission
// time and rapid guesses, cursor on the student id.
func (q SchoolAssignments) Submissions(ctx context.Context, id uuid.UUID, in SubmissionListInput, pg PageInput) (out Page[dto.AssignmentSubmissionView], err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListAssignmentSubmissions")
	defer func() { done(err) }()
	p, err := staffPrincipal(ctx)
	if err != nil {
		return out, err
	}
	after, limit, err := keyset(pg, q.Cursors)
	if err != nil {
		return out, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		if _, err := q.readable(ctx, p, authz.PermLmsAssignmentRead, id); err != nil {
			return err
		}
		rows, err := q.Model.Submissions(ctx, read.SubmissionFilter{AssignmentID: id, Status: in.Status, ClassID: in.ClassID, After: after,
			Limit: limit + 1})
		if err != nil {
			return err
		}
		out = pageOf(rows, limit, func(s dto.AssignmentSubmissionView) uuid.UUID { return s.StudentID }, q.Cursors)
		return nil
	})
	return out, err
}

// contentKind is MyAssignment.content_kind of a content type [Proposed: a mini-game is played as an activity].
func contentKind(t string) string {
	switch t {
	case assignment.ContentBlueprint:
		return "assessment"
	case assignment.ContentMinigame:
		return "activity"
	}
	return t
}

// myStatus is MyAssignment.status of a submission: submitted or late is graded once the results are visible, missed is
// overdue.
func myStatus(sub string, results bool) string {
	switch sub {
	case "submitted", "late":
		if results {
			return "graded"
		}
		return "submitted"
	case "missed":
		return "overdue"
	}
	return sub
}

// MyAssignments is listMyAssignments (F041-AC-12, OQ F062-1): the open, closed and released assignments of the student
// with the status of the submission, the attempt in progress, the attempts left, the personal due date and the name of
// the subject; status filters on MyAssignment.status.
func (q SchoolAssignments) MyAssignments(ctx context.Context, status string, pg PageInput) (out Page[dto.MyAssignmentView], err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListMyAssignments")
	defer func() { done(err) }()
	p, err := student(ctx)
	if err != nil {
		return out, err
	}
	after, limit, err := keyset(pg, q.Cursors)
	if err != nil {
		return out, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		recs, err := q.Model.MyAssignments(ctx, p.UserID, after, 500)
		if err != nil {
			return err
		}
		var views []dto.MyAssignmentView
		for _, r := range recs {
			a := r.Assignment
			v := dto.MyAssignmentView{AssignmentID: a.AssignmentID, Title: a.Title, SubjectID: r.SubjectID, SubjectName: r.SubjectName,
				ContentKind: contentKind(a.ContentRef.ContentType), ContentRef: a.ContentRef.ContentID, OpenAt: a.OpenAt, DueAt: a.DueAt,
				Status: myStatus(r.Submission, assignment.ResultsVisible(a.ResultRelease, assignment.Status(a.Status))), AttemptID: r.CurrentAttemptID,
				ExamSessionID: a.ExamSessionID, ResultRelease: a.ResultRelease, LatePolicy: a.LatePolicy}
			if r.DueAtOverride != nil {
				v.DueAt = *r.DueAtOverride
			}
			left := max(0, a.MaxAttempts-r.AttemptsUsed)
			v.AttemptsLeft = &left
			if status != "" && v.Status != status {
				continue
			}
			views = append(views, v)
		}
		if len(views) > limit+1 {
			views = views[:limit+1]
		}
		out = pageOf(views, limit, func(v dto.MyAssignmentView) uuid.UUID { return v.AssignmentID }, q.Cursors)
		return nil
	})
	return out, err
}

// GameActivityInput are the filters of listGameActivities.
type GameActivityInput struct {
	Grade        *int
	ObjectiveIDs []uuid.UUID
	Language     string
}

// GameActivities is listGameActivities (F041-AC-30): lms.assignment.read; the quests and mini-games published and assigned
// to the tenant by module game (empty until module game publishes its catalog), by objective and grade.
func (q SchoolAssignments) GameActivities(ctx context.Context, in GameActivityInput, pg PageInput) (out Page[dto.GameActivityView], err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListGameActivities")
	defer func() { done(err) }()
	p, err := staffPrincipal(ctx)
	if err != nil {
		return out, err
	}
	if !holds(p, authz.PermLmsAssignmentRead) {
		return out, authz.Require(ctx, p, authz.PermLmsAssignmentRead, authz.Resource{TenantID: p.TenantID})
	}
	after, limit, err := keyset(pg, q.Cursors)
	if err != nil {
		return out, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		rows, err := q.Model.GameActivities(ctx, read.GameActivityFilter{Grade: in.Grade, ObjectiveIDs: slices.Clone(in.ObjectiveIDs), After: after,
			Limit: limit + 1})
		if err != nil {
			return err
		}
		lang := i18ntext.Locale(in.Language)
		for i := range rows {
			resolved, _ := i18ntext.Resolve(rows[i].TitleI18n, lang, lang)
			var title string
			if json.Unmarshal(resolved, &title) == nil {
				rows[i].Title = title
			}
		}
		out = pageOf(rows, limit, func(g dto.GameActivityView) uuid.UUID { return g.ContentID }, q.Cursors)
		return nil
	})
	return out, err
}

// ResultsVisible reports whether the results of an attempt of an assignment are visible to its student now
// (F041-AC-19; the review of the attempt only shows its submitted state before).
func (q SchoolAssignments) ResultsVisible(ctx context.Context, assignmentID uuid.UUID) (bool, error) {
	release, status, ok, err := q.Model.Release(ctx, assignmentID)
	if err != nil || !ok {
		return false, err
	}
	return assignment.ResultsVisible(release, assignment.Status(status)), nil
}
