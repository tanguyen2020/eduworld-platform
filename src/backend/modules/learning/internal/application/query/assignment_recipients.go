package query

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// MaxRecipientPage bounds one page of listAssignmentRecipients (≤ 500, contracts §2 of F123).
const MaxRecipientPage = 500

// RecipientPage is one page of students receiving an assignment.
type RecipientPage struct {
	Title      string // title of the assignment typed by the school (no student data)
	StudentIDs []uuid.UUID
	Next       *uuid.UUID // last student id of the page when more follow
}

// AssignmentRecipients serves listAssignmentRecipients for module chat (F123-AC-02): the students of an assignment of
// the TenantScope of the call minus the excused ones. No write, no event (R26).
type AssignmentRecipients struct {
	Session read.Session
	Model   read.AssignmentRecipientReadModel
}

// Recipients returns one page (limit 1..500, default 500) after the cursor student; an assignment of another tenant or
// unknown is 404 NOT_FOUND.
func (q AssignmentRecipients) Recipients(ctx context.Context, id uuid.UUID, after *uuid.UUID, limit int) (out RecipientPage, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListAssignmentRecipients")
	defer func() { done(err) }()
	s, ok := tenancy.ScopeFrom(ctx)
	if !ok {
		return out, problem.New(problem.TenantHeaderRequired, "")
	}
	if _, single := s.SessionTenantID(); !single {
		return out, problem.New(problem.PermissionDenied, "a single tenant scope is required")
	}
	if limit <= 0 || limit > MaxRecipientPage {
		limit = MaxRecipientPage
	}
	var exists bool
	var ids []uuid.UUID
	var title string
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		var err error
		title, exists, ids, err = q.Model.Recipients(ctx, id, after, limit)
		return err
	})
	if err != nil {
		return out, err
	}
	if !exists {
		return out, problem.New(problem.NotFound, "")
	}
	if len(ids) > limit {
		ids = ids[:limit]
		last := ids[limit-1]
		out.Next = &last
	}
	out.Title, out.StudentIDs = title, ids
	return out, nil
}
