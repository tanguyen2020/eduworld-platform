//go:build integration

package integration

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"testing"

	"github.com/google/uuid"

	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/facade"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// recipients calls listAssignmentRecipients through pkg/modcall as caller in the event scope of tenant (as the chat
// consumer does), with the generated x-allowed-callers.
func recipients(ctx context.Context, f facade.Facade, caller string, tenant uuid.UUID, id uuid.UUID, cursor *string, limit *int) (learningapi.ListAssignmentRecipientsResponseObject, error) {
	sc, err := authz.EventScope(tenant)
	if err != nil {
		return nil, err
	}
	ctx = tenancy.WithScope(ctx, sc)
	op := modcall.Op{Caller: caller, Callee: "learning", Operation: "listAssignmentRecipients", Kind: modcall.Read,
		AllowedCallers: learningapi.AllowedCallers["listAssignmentRecipients"], ContextRequired: learningapi.ContextRequired["listAssignmentRecipients"]}
	return modcall.Call(ctx, op, func(ctx context.Context) (learningapi.ListAssignmentRecipientsResponseObject, error) {
		return f.ListAssignmentRecipients(ctx, learningapi.ListAssignmentRecipientsRequestObject{AssignmentId: id,
			Params: learningapi.ListAssignmentRecipientsParams{Cursor: cursor, Limit: limit}})
	})
}

// F123-AC-02 (receiving side, T010): listAssignmentRecipients returns the targeted students of the assignment minus the
// excused one, paged by student id; another tenant gets 404 (TC-TEN-001); only module chat may call it (TC-SVC-005/006:
// x-allowed-callers through pkg/modcall, and again inside the facade).
func TestAssignmentRecipientsFacade(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.asgSetup(ctx, t)
	id := uuid.MustParse(c.openAsg(t, k, nil))
	f := facade.Facade{Recipients: query.AssignmentRecipients{Session: c.read, Model: read.AssignmentRecipientModel{}}}
	an, binh := c.users[hsAn].UserID, c.users[hsBinh].UserID

	list := func(cursor *string, limit *int) learningapi.ListAssignmentRecipients200JSONResponse {
		t.Helper()
		resp, err := recipients(ctx, f, "chat", c.tenantA, id, cursor, limit)
		if err != nil {
			t.Fatalf("recipients: %v", err)
		}
		return resp.(learningapi.ListAssignmentRecipients200JSONResponse)
	}
	all := list(nil, nil)
	if all.Title != "Ôn tập bảng nhân" || len(all.StudentIds) != 2 || !slices.Contains(all.StudentIds, an) || !slices.Contains(all.StudentIds, binh) || all.Page.HasMore {
		t.Fatalf("recipients before the excuse: %+v", all)
	}
	// Paging: one per page, keyset on the student id.
	one := 1
	p1 := list(nil, &one)
	if len(p1.StudentIds) != 1 || !p1.Page.HasMore || p1.Page.NextCursor == nil {
		t.Fatalf("page 1: %+v", p1)
	}
	p2 := list(p1.Page.NextCursor, &one)
	if len(p2.StudentIds) != 1 || p2.Page.HasMore || p2.StudentIds[0] == p1.StudentIds[0] {
		t.Fatalf("page 2: %+v", p2)
	}
	// The excused student is not a recipient.
	must(t, c.do(t, http.MethodPost, assignmentsPath+"/"+id.String()+"/students/"+binh.String()+":excuse", adA, idem(),
		map[string]any{"reason": "Ốm dài ngày"}), http.StatusOK, "excuse")
	if got := list(nil, nil); len(got.StudentIds) != 1 || got.StudentIds[0] != an {
		t.Fatalf("recipients after the excuse: %+v", got)
	}
	// Another tenant: 404, never the students of tenant A.
	var pe *problem.Error
	if _, err := recipients(ctx, f, "chat", c.tenantB, id, nil, nil); !errors.As(err, &pe) || pe.Code != problem.NotFound {
		t.Fatalf("tenant B: %v", err)
	}
	if _, err := recipients(ctx, f, "chat", c.tenantA, uuid.New(), nil, nil); !errors.As(err, &pe) || pe.Code != problem.NotFound {
		t.Fatalf("unknown assignment: %v", err)
	}
	// Only module chat (x-allowed-callers), even when the facade is called directly with another caller.
	if _, err := recipients(ctx, f, "game", c.tenantA, id, nil, nil); !errors.As(err, &pe) || pe.Code != problem.CallerNotAllowed {
		t.Fatalf("caller game: %v", err)
	}
	sc, _ := authz.EventScope(c.tenantA)
	if _, err := f.ListAssignmentRecipients(tenancy.WithScope(ctx, sc), learningapi.ListAssignmentRecipientsRequestObject{AssignmentId: id}); !errors.As(err, &pe) ||
		pe.Code != problem.CallerNotAllowed {
		t.Fatalf("direct call without caller: %v", err)
	}
	bad := "not-a-uuid"
	if _, err := recipients(ctx, f, "chat", c.tenantA, id, &bad, nil); !errors.As(err, &pe) || pe.Code != problem.ValidationFailed {
		t.Fatalf("bad cursor: %v", err)
	}
}
