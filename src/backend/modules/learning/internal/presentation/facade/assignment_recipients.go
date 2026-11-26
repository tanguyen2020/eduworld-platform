package facade

import (
	"context"

	"github.com/google/uuid"

	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// AssignmentRecipients reads the students receiving an assignment (query.AssignmentRecipients, F123).
type AssignmentRecipients interface {
	Recipients(ctx context.Context, id uuid.UUID, after *uuid.UUID, limit int) (query.RecipientPage, error)
}

// ListAssignmentRecipients implements listAssignmentRecipients (x-allowed-callers: chat, F123-AC-02): the targeted
// students minus the excused ones, ≤ 500 per page, tenant of the TenantScope of the call; the cursor is the last student
// id of the previous page. Only module chat may call it (checked again here, besides pkg/modcall); no route serves it.
func (f Facade) ListAssignmentRecipients(ctx context.Context, req learningapi.ListAssignmentRecipientsRequestObject) (learningapi.ListAssignmentRecipientsResponseObject, error) {
	if modcall.CallerFrom(ctx) != "chat" {
		return nil, problem.New(problem.CallerNotAllowed, "")
	}
	if f.Recipients == nil {
		return nil, problem.New(problem.Internal, "assignment recipients are not wired")
	}
	var after *uuid.UUID
	if c := req.Params.Cursor; c != nil && *c != "" {
		id, err := uuid.Parse(*c)
		if err != nil {
			return nil, problem.New(problem.ValidationFailed, "invalid cursor")
		}
		after = &id
	}
	limit := query.MaxRecipientPage
	if req.Params.Limit != nil {
		if *req.Params.Limit < 1 || *req.Params.Limit > query.MaxRecipientPage {
			return nil, problem.New(problem.ValidationFailed, "limit must be 1..500")
		}
		limit = *req.Params.Limit
	}
	p, err := f.Recipients.Recipients(ctx, req.AssignmentId, after, limit)
	if err != nil {
		return nil, err
	}
	out := learningapi.ListAssignmentRecipients200JSONResponse{Title: p.Title, StudentIds: p.StudentIDs,
		Page: learningapi.CommonInternalCommonV1PageInfo{HasMore: p.Next != nil}}
	if out.StudentIds == nil {
		out.StudentIds = []uuid.UUID{}
	}
	if p.Next != nil {
		s := p.Next.String()
		out.Page.NextCursor = &s
	}
	return out, nil
}
