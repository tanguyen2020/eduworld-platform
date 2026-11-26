package modules

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/contract"
)

func tenantCtx(t *testing.T, tenant uuid.UUID) context.Context {
	t.Helper()
	sc, err := authz.EventScope(tenant)
	if err != nil {
		t.Fatal(err)
	}
	return tenancy.WithScope(context.Background(), sc)
}

// Caller-side facade test of listAssignmentRecipients (F123 T014, TC-SVC-004) against the generated fake (TEST DOUBLE
// learningapi.Fake) answering the example of the internal spec: caller chat with the TenantScope of the event, the
// cursor of the previous page passed on, the last page ends the walk, 404 (answer or problem) → ErrAssignmentNotFound,
// any other failure → retryable error (never a partial batch), no tenant scope → refused before the call, late wiring.
func TestLearningRecipientsFacade(t *testing.T) {
	spec, err := contract.LoadFacade("learning")
	if err != nil {
		t.Fatal(err)
	}
	_, examples, err := spec.Examples("listAssignmentRecipients")
	if err != nil || len(examples[200]) == 0 {
		t.Fatalf("spec example: %v %v", examples, err)
	}
	if err := spec.CheckResponse("listAssignmentRecipients", 200, examples[200][0]); err != nil {
		t.Fatalf("spec example: %v", err)
	}
	next := "0193a5f4-6c1e-7b2a-9d1f-000000000003"
	page1 := learningapi.ListAssignmentRecipients200JSONResponse{Title: "Ôn tập bảng nhân",
		StudentIds: []uuid.UUID{uuid.MustParse("0193a5f4-6c1e-7b2a-9d1f-000000000002"), uuid.MustParse(next)},
		Page:       learningapi.CommonInternalCommonV1PageInfo{HasMore: true, NextCursor: &next}}
	page2 := learningapi.ListAssignmentRecipients200JSONResponse{Title: "Ôn tập bảng nhân", StudentIds: []uuid.UUID{}}
	// The answers of the fake follow the contract.
	for _, pg := range []learningapi.ListAssignmentRecipients200JSONResponse{page1, page2} {
		if err := spec.CheckResponse("listAssignmentRecipients", 200, pg); err != nil {
			t.Fatalf("fake answer breaks the contract: %v", err)
		}
	}
	tenant, assignment := uuid.New(), uuid.New()
	var failure error
	var notFound, problem404 bool
	var cursors []string
	fake := &learningapi.Fake{ListAssignmentRecipientsFunc: func(ctx context.Context, r learningapi.ListAssignmentRecipientsRequestObject) (learningapi.ListAssignmentRecipientsResponseObject, error) {
		sc, ok := tenancy.ScopeFrom(ctx)
		if modcall.CallerFrom(ctx) != "chat" || !ok || r.AssignmentId != assignment {
			t.Fatalf("request: %s %v %+v", modcall.CallerFrom(ctx), ok, r)
		}
		if id, single := sc.SessionTenantID(); !single || id != tenant {
			t.Fatalf("scope: %v", sc)
		}
		c := ""
		if r.Params.Cursor != nil {
			c = *r.Params.Cursor
		}
		cursors = append(cursors, c)
		switch {
		case failure != nil:
			return nil, failure
		case problem404:
			return nil, problem.New(problem.NotFound, "")
		case notFound:
			return learningapi.ListAssignmentRecipients404ApplicationProblemPlusJSONResponse{}, nil
		}
		if c == "" {
			return page1, nil
		}
		return page2, nil
	}}
	l := Learning{F: fake}
	p, err := l.Page(tenantCtx(t, tenant), assignment, "")
	if err != nil || p.Title != "Ôn tập bảng nhân" || len(p.StudentIDs) != 2 || p.Next != next {
		t.Fatalf("first page: %+v %v", p, err)
	}
	p, err = l.Page(tenantCtx(t, tenant), assignment, p.Next)
	if err != nil || p.Next != "" || len(p.StudentIDs) != 0 || cursors[1] != next {
		t.Fatalf("last page: %+v %v %v", p, err, cursors)
	}
	notFound = true
	if _, err := l.Page(tenantCtx(t, tenant), assignment, ""); !errors.Is(err, port.ErrAssignmentNotFound) {
		t.Fatalf("404 answer: %v", err)
	}
	notFound, problem404 = false, true
	if _, err := l.Page(tenantCtx(t, tenant), assignment, ""); !errors.Is(err, port.ErrAssignmentNotFound) {
		t.Fatalf("404 problem: %v", err)
	}
	problem404, failure = false, errors.New("TEST DOUBLE: learning unavailable")
	if _, err := l.Page(tenantCtx(t, tenant), assignment, ""); err == nil || errors.Is(err, port.ErrAssignmentNotFound) {
		t.Fatalf("failure must be retryable: %v", err)
	}
	failure = nil
	calls := len(cursors)
	if _, err := l.Page(context.Background(), assignment, ""); err == nil || len(cursors) != calls {
		t.Fatalf("no tenant scope must be refused before the call: %v", err)
	}
	var late LateLearning
	if _, err := late.Page(tenantCtx(t, tenant), assignment, ""); !errors.Is(err, ErrLearningNotWired) {
		t.Fatalf("not wired: %v", err)
	}
	late.Set(fake)
	if p, err := late.Page(tenantCtx(t, tenant), assignment, ""); err != nil || len(p.StudentIDs) != 2 {
		t.Fatalf("late: %+v %v", p, err)
	}
}
