package facade

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/contract"
)

// fakeRecipients is a TEST DOUBLE of query.AssignmentRecipients (R17): the page of the spec example, the cursor and limit
// it was asked for; another assignment → 404.
type fakeRecipients struct {
	assignment uuid.UUID
	after      *uuid.UUID
	limit      int
}

func (f *fakeRecipients) Recipients(_ context.Context, id uuid.UUID, after *uuid.UUID, limit int) (query.RecipientPage, error) {
	f.after, f.limit = after, limit
	if id != f.assignment {
		return query.RecipientPage{}, problem.New(problem.NotFound, "")
	}
	return query.RecipientPage{Title: "Ôn tập bảng nhân", StudentIDs: []uuid.UUID{uuid.MustParse("0193a5f4-6c1e-7b2a-9d1f-000000000002"),
		uuid.MustParse("0193a5f4-6c1e-7b2a-9d1f-000000000003")}}, nil
}

// call goes through pkg/modcall as module caller would (x-allowed-callers, tenant_scope, caller in ctx).
func call(t *testing.T, f Facade, caller string, req learningapi.ListAssignmentRecipientsRequestObject) (learningapi.ListAssignmentRecipientsResponseObject, error) {
	t.Helper()
	sc, err := authz.EventScope(uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	op := modcall.Op{Caller: caller, Callee: "learning", Operation: "listAssignmentRecipients", Kind: modcall.Read,
		AllowedCallers: learningapi.AllowedCallers["listAssignmentRecipients"], ContextRequired: learningapi.ContextRequired["listAssignmentRecipients"]}
	return modcall.Call(tenancy.WithScope(context.Background(), sc), op, func(ctx context.Context) (learningapi.ListAssignmentRecipientsResponseObject, error) {
		return f.ListAssignmentRecipients(ctx, req)
	})
}

// Receiver-side facade test of listAssignmentRecipients (F123 T014, TC-SVC-005/006): the answer matches the example and
// schema of the internal spec, the cursor is the last student id, limit 1..500 (default 500), unknown assignment → 404,
// only module chat may call it (module game → CALLER_NOT_ALLOWED, also when the facade is called without modcall).
func TestListAssignmentRecipientsFacade(t *testing.T) {
	spec, err := contract.LoadFacade("learning")
	if err != nil {
		t.Fatal(err)
	}
	_, examples, err := spec.Examples("listAssignmentRecipients")
	if err != nil || len(examples[200]) == 0 {
		t.Fatalf("spec example: %v", err)
	}
	if c := learningapi.AllowedCallers["listAssignmentRecipients"]; len(c) != 1 || c[0] != "chat" {
		t.Fatalf("x-allowed-callers: %v", c)
	}
	rec := &fakeRecipients{assignment: uuid.New()}
	f := Facade{Recipients: rec}
	resp, err := call(t, f, "chat", learningapi.ListAssignmentRecipientsRequestObject{AssignmentId: rec.assignment})
	v, ok := resp.(learningapi.ListAssignmentRecipients200JSONResponse)
	if err != nil || !ok || v.Title != "Ôn tập bảng nhân" || len(v.StudentIds) != 2 || v.Page.HasMore || rec.limit != query.MaxRecipientPage || rec.after != nil {
		t.Fatalf("page: %+v %v (limit %d)", resp, err, rec.limit)
	}
	if err := spec.CheckResponse("listAssignmentRecipients", 200, v); err != nil {
		t.Fatalf("answer breaks the contract: %v", err)
	}
	cursor, limit := "0193a5f4-6c1e-7b2a-9d1f-000000000002", 2
	if _, err := call(t, f, "chat", learningapi.ListAssignmentRecipientsRequestObject{AssignmentId: rec.assignment,
		Params: learningapi.ListAssignmentRecipientsParams{Cursor: &cursor, Limit: &limit}}); err != nil || rec.after == nil ||
		rec.after.String() != cursor || rec.limit != 2 {
		t.Fatalf("cursor: %v %v %d", err, rec.after, rec.limit)
	}
	code := func(err error) problem.Code {
		var pe *problem.Error
		if !errors.As(err, &pe) {
			return ""
		}
		return pe.Code
	}
	bad, over := "not-a-uuid", 501
	if _, err := call(t, f, "chat", learningapi.ListAssignmentRecipientsRequestObject{AssignmentId: rec.assignment,
		Params: learningapi.ListAssignmentRecipientsParams{Cursor: &bad}}); code(err) != problem.ValidationFailed {
		t.Fatalf("bad cursor: %v", err)
	}
	if _, err := call(t, f, "chat", learningapi.ListAssignmentRecipientsRequestObject{AssignmentId: rec.assignment,
		Params: learningapi.ListAssignmentRecipientsParams{Limit: &over}}); code(err) != problem.ValidationFailed {
		t.Fatalf("limit > 500: %v", err)
	}
	if _, err := call(t, f, "chat", learningapi.ListAssignmentRecipientsRequestObject{AssignmentId: uuid.New()}); code(err) != problem.NotFound {
		t.Fatalf("unknown assignment: %v", err)
	}
	if _, err := call(t, f, "game", learningapi.ListAssignmentRecipientsRequestObject{AssignmentId: rec.assignment}); code(err) != problem.CallerNotAllowed {
		t.Fatalf("caller game: %v", err)
	}
	if _, err := f.ListAssignmentRecipients(context.Background(), learningapi.ListAssignmentRecipientsRequestObject{AssignmentId: rec.assignment}); code(err) != problem.CallerNotAllowed {
		t.Fatalf("no modcall caller: %v", err)
	}
}
