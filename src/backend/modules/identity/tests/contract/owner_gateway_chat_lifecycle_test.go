package contract

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	chatapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/modules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/participants"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// fakeChatLifecycle is a TEST DOUBLE (R17) of the lifecycle operations of the chat facade (chatapi.StrictServerInterface
// subset): it records the call context and answers like specs/openapi/modules/chat.internal.v1.yaml
// (StudentLifecycleResult, UserLifecycleResult). The real facade is tested by module chat
// (tests/integration/user_lifecycle_test.go, tests/contract/lifecycle_facade_test.go).
type fakeChatLifecycle struct {
	fakeLearning
	blocked bool
	merge   *chatapi.MergeUserDataJSONRequestBody
	cleanup *chatapi.CleanupStudentDataJSONRequestBody
}

func (f *fakeChatLifecycle) CleanupStudentData(ctx context.Context, req chatapi.CleanupStudentDataRequestObject) (chatapi.CleanupStudentDataResponseObject, error) {
	if err := f.record(ctx, "cleanupStudentData"); err != nil {
		return nil, err
	}
	f.cleanup = req.Body
	if f.blocked {
		r := chatapi.OpenModerationCase
		return chatapi.CleanupStudentData200JSONResponse{StudentId: req.StudentId, Status: "blocked", BlockedReason: &r}, nil
	}
	return chatapi.CleanupStudentData200JSONResponse{StudentId: req.StudentId, Status: "done"}, nil
}

func (f *fakeChatLifecycle) RestoreStudentData(ctx context.Context, req chatapi.RestoreStudentDataRequestObject) (chatapi.RestoreStudentDataResponseObject, error) {
	if err := f.record(ctx, "restoreStudentData"); err != nil {
		return nil, err
	}
	return chatapi.RestoreStudentData200JSONResponse{StudentId: req.StudentId, Status: "done"}, nil
}

func (f *fakeChatLifecycle) AnonymizeUserData(ctx context.Context, req chatapi.AnonymizeUserDataRequestObject) (chatapi.AnonymizeUserDataResponseObject, error) {
	if err := f.record(ctx, "anonymizeUserData"); err != nil {
		return nil, err
	}
	return chatapi.AnonymizeUserData200JSONResponse{UserId: req.UserId, Status: "done"}, nil
}

func (f *fakeChatLifecycle) MergeUserData(ctx context.Context, req chatapi.MergeUserDataRequestObject) (chatapi.MergeUserDataResponseObject, error) {
	if err := f.record(ctx, "mergeUserData"); err != nil {
		return nil, err
	}
	f.merge = req.Body
	return chatapi.MergeUserData200JSONResponse{UserId: req.UserId, Status: "done"}, nil
}

// F115 T042 (caller side, OQ F016-2/F045-3): the merge step and the lifecycle steps of module chat go through pkg/modcall
// with the saga deadline, the Principal, the tenant scope and the Idempotency-Key of the step; a blocked cleanup carries
// open_moderation_case to the saga (compensation of the done steps); not wired is 503.
func TestOwnerGatewayChatLifecycle(t *testing.T) {
	sa := authztest.Platform("system_admin")
	tenant := uuid.New()
	sc, err := authz.EventScope(tenant)
	if err != nil {
		t.Fatal(err)
	}
	merge := uuid.New()
	ctx := modcall.WithIdempotencyKey(tenancy.WithScope(authztest.Ctx(context.Background(), sa), sc), merge.String())
	from, into := uuid.New(), uuid.New()
	fake := &fakeChatLifecycle{}
	res, err := (modules.Gateway{ChatMerge: fake}).MergeUserData(ctx, "chat", from, port.MergeUserDataRequest{MergeID: merge, Into: into,
		Reason: "same person"})
	if err != nil || res.Status != "done" || fake.merge == nil || fake.merge.TargetUserId != into || fake.merge.MergeId != merge ||
		fake.merge.Reason != "same person" {
		t.Fatalf("merge: %+v %v %+v", res, err, fake.merge)
	}
	c := fake.seen[0]
	if c.caller != "identity" || c.principal.UserID != sa.UserID || !c.scope || c.key != merge.String() || c.deadline <= 0 ||
		c.deadline > modcall.DefaultDeadlines[modcall.Saga] {
		t.Fatalf("call context %+v", c)
	}
	if _, err := (modules.Gateway{}).MergeUserData(ctx, "chat", from, port.MergeUserDataRequest{}); probCode(err) != problem.Unavailable {
		t.Fatalf("not wired: %v", err)
	}
	// Lifecycle participants (cleanup, restore, anonymize).
	part := participants.Chat{F: fake}
	if part.Name() != "chat" || (participants.ChatAnonymize{}).Name() != "chat" {
		t.Fatal(part.Name())
	}
	last := time.Now()
	run := uuid.New()
	if r, err := part.Cleanup(ctx, from, run, &last); err != nil || r.Status != "done" || fake.cleanup.CleanupRunId != run ||
		fake.cleanup.Reason != "inactive_cleanup" {
		t.Fatalf("cleanup: %+v %v", r, err)
	}
	fake.blocked = true
	if r, err := part.Cleanup(ctx, from, run, nil); err != nil || r.Status != "blocked" || r.BlockedReason != "open_moderation_case" {
		t.Fatalf("blocked cleanup: %+v %v", r, err)
	}
	if r, err := part.Restore(ctx, from, true); err != nil || r.Status != "done" {
		t.Fatalf("restore: %+v %v", r, err)
	}
	if r, err := (participants.ChatAnonymize{F: fake}).Anonymize(ctx, from, uuid.New(), "data_subject_request"); err != nil || r.Status != "done" {
		t.Fatalf("anonymize: %+v %v", r, err)
	}
	for _, s := range fake.seen[1:] {
		if s.caller != "identity" || !s.scope {
			t.Fatalf("%s: %+v", s.op, s)
		}
	}
	if len(fake.seen) != 5 {
		t.Fatalf("calls: %+v", fake.seen)
	}
	// Without a tenant scope modcall refuses before the facade runs (x-context-required: tenant_scope).
	noScope := modcall.WithIdempotencyKey(authz.WithPrincipal(context.Background(), sa), merge.String())
	if _, err := part.Cleanup(noScope, from, run, nil); err == nil || len(fake.seen) != 5 {
		t.Fatalf("no scope: %v (%d calls)", err, len(fake.seen))
	}
}
