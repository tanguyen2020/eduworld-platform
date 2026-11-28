package contract

import (
	"context"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/facade"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/contract"
)

// lifecycleOps are the operations of the identity sagas served by module chat (F115 T042).
var lifecycleOps = []string{"cleanupStudentData", "restoreStudentData", "anonymizeUserData", "mergeUserData"}

// fakeLifecycle is a TEST DOUBLE (R17) of command.UserLifecycle: it answers the configured outcome and counts the calls
// (the real use case is tested on PostgreSQL in tests/integration/user_lifecycle_test.go).
type fakeLifecycle struct {
	status, reason string
	calls          int
	merge          command.MergeInput
}

func (f *fakeLifecycle) Cleanup(context.Context, uuid.UUID) (string, string, error) {
	f.calls++
	return f.status, f.reason, nil
}

func (f *fakeLifecycle) Restore(context.Context, uuid.UUID) (string, error) {
	f.calls++
	return f.status, nil
}

func (f *fakeLifecycle) Anonymize(context.Context, uuid.UUID) (string, error) {
	f.calls++
	return f.status, nil
}

func (f *fakeLifecycle) Merge(_ context.Context, in command.MergeInput) (string, string, error) {
	f.calls++
	f.merge = in
	return f.status, f.reason, nil
}

// sagaCtx is the context of a saga step of module identity: Principal, the TenantScope of the step and its
// Idempotency-Key.
func sagaCtx(p authz.Principal, tenant uuid.UUID) context.Context {
	sc, _ := authz.EventScope(tenant)
	return modcall.WithIdempotencyKey(tenancy.WithScope(authztest.Ctx(context.Background(), p), sc), uuid.NewString())
}

// F115 T042 (receiving side, OQ F016-2/F045-3): the lifecycle operations are callable by module identity only, in
// process, with a tenant scope and an Idempotency-Key (x-allowed-callers, x-context-required of the internal spec); a
// call of another module or without the context never reaches the use case; the answers of every outcome validate
// against StudentLifecycleResult / UserLifecycleResult; not wired is 503.
func TestChatLifecycleFacadeContract(t *testing.T) {
	for _, op := range lifecycleOps {
		if got := api.AllowedCallers[op]; !slices.Equal(got, []string{"identity"}) {
			t.Fatalf("%s x-allowed-callers %v", op, got)
		}
		if got := api.ContextRequired[op]; !slices.Contains(got, "tenant_scope") || !slices.Contains(got, "idempotency_key") {
			t.Fatalf("%s x-context-required %v", op, got)
		}
	}
	spec, err := contract.LoadFacade("chat")
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeLifecycle{status: "done"}
	f := facade.Facade{Lifecycle: fake}
	student, target := uuid.New(), uuid.New()
	ctx := sagaCtx(authztest.Platform("system_admin"), authztest.TenantA)
	cleanup := api.CleanupStudentDataRequestObject{StudentId: student, Body: &api.CleanupStudentDataJSONRequestBody{CleanupRunId: uuid.New(),
		Reason: "inactive_cleanup"}}
	merge := api.MergeUserDataRequestObject{UserId: student, Body: &api.MergeUserDataJSONRequestBody{MergeId: uuid.New(), TargetUserId: target,
		Reason: "same person"}}
	calls := map[string]func(ctx context.Context) (any, error){
		"cleanupStudentData": func(ctx context.Context) (any, error) { return f.CleanupStudentData(ctx, cleanup) },
		"restoreStudentData": func(ctx context.Context) (any, error) {
			return f.RestoreStudentData(ctx, api.RestoreStudentDataRequestObject{StudentId: student,
				Body: &api.RestoreStudentDataJSONRequestBody{Reason: "student_logged_in_again"}})
		},
		"anonymizeUserData": func(ctx context.Context) (any, error) {
			return f.AnonymizeUserData(ctx, api.AnonymizeUserDataRequestObject{UserId: student,
				Body: &api.AnonymizeUserDataJSONRequestBody{ErasureRequestId: uuid.New(), Reason: "retention_expired"}})
		},
		"mergeUserData": func(ctx context.Context) (any, error) { return f.MergeUserData(ctx, merge) },
	}
	for _, status := range []string{"done", "nothing_to_do"} {
		fake.status = status
		for op, call := range calls {
			res, err := modcall.Call(ctx, modcall.Op{Caller: "identity", Callee: "chat", Operation: op, Kind: modcall.Saga,
				AllowedCallers: api.AllowedCallers[op], ContextRequired: api.ContextRequired[op]}, call)
			if err != nil {
				t.Fatalf("%s: %v", op, err)
			}
			if err := spec.CheckResponse(op, 200, res); err != nil {
				t.Fatalf("contract %s %s: %v", op, status, err)
			}
		}
	}
	if fake.merge.From != student || fake.merge.Into != target || fake.merge.MergeID != merge.Body.MergeId {
		t.Fatalf("merge input %+v", fake.merge)
	}
	// blocked answers: open_moderation_case of a cleanup (StudentLifecycleResult enum) and the free reason of a merge.
	fake.status, fake.reason = "blocked", "open_moderation_case"
	res, err := f.CleanupStudentData(ctx, cleanup)
	if err != nil || *res.(api.CleanupStudentData200JSONResponse).BlockedReason != api.OpenModerationCase {
		t.Fatalf("blocked cleanup: %+v %v", res, err)
	}
	if err := spec.CheckResponse("cleanupStudentData", 200, res); err != nil {
		t.Fatalf("contract blocked cleanup: %v", err)
	}
	fake.reason = "the source and target users are the same"
	mres, err := f.MergeUserData(ctx, merge)
	if err != nil || spec.CheckResponse("mergeUserData", 200, mres) != nil {
		t.Fatalf("blocked merge: %+v %v", mres, err)
	}
	// Callers other than identity and calls without the saga context never reach the use case.
	n := fake.calls
	for _, caller := range []string{"game", "competition", "learning", "chat"} {
		for op, call := range calls {
			_, err := modcall.Call(ctx, modcall.Op{Caller: caller, Callee: "chat", Operation: op, Kind: modcall.Saga,
				AllowedCallers: api.AllowedCallers[op], ContextRequired: api.ContextRequired[op]}, call)
			if trashCode(err) != problem.CallerNotAllowed {
				t.Fatalf("%s by %s: %v", op, caller, err)
			}
		}
	}
	noKey := tenancy.WithScope(authztest.Ctx(context.Background(), authztest.Platform("system_admin")), mustEventScope(t))
	noScope := modcall.WithIdempotencyKey(authz.WithPrincipal(context.Background(), authztest.Platform("system_admin")), uuid.NewString())
	for op, call := range calls {
		for name, c := range map[string]context.Context{"no key": noKey, "no scope": noScope} {
			if _, err := modcall.Call(c, modcall.Op{Caller: "identity", Callee: "chat", Operation: op, Kind: modcall.Saga,
				AllowedCallers: api.AllowedCallers[op], ContextRequired: api.ContextRequired[op]}, call); err == nil {
				t.Fatalf("%s %s accepted", op, name)
			}
		}
	}
	if fake.calls != n {
		t.Fatalf("refused calls reached the use case: %d", fake.calls-n)
	}
	// Not wired (a binary without the use cases): 503, never a silent success.
	for op := range calls {
		var err error
		switch op {
		case "cleanupStudentData":
			_, err = facade.Facade{}.CleanupStudentData(ctx, cleanup)
		case "restoreStudentData":
			_, err = facade.Facade{}.RestoreStudentData(ctx, api.RestoreStudentDataRequestObject{StudentId: student})
		case "anonymizeUserData":
			_, err = facade.Facade{}.AnonymizeUserData(ctx, api.AnonymizeUserDataRequestObject{UserId: student})
		case "mergeUserData":
			_, err = facade.Facade{}.MergeUserData(ctx, merge)
		}
		if trashCode(err) != problem.Unavailable {
			t.Fatalf("%s not wired: %v", op, err)
		}
	}
	// A body is required (the generated strict server would answer 422).
	if _, err := f.MergeUserData(ctx, api.MergeUserDataRequestObject{UserId: student}); trashCode(err) != problem.ValidationFailed {
		t.Fatalf("merge without body: %v", err)
	}
}

func mustEventScope(t *testing.T) tenancy.Scope {
	t.Helper()
	sc, err := authz.EventScope(authztest.TenantA)
	if err != nil {
		t.Fatal(err)
	}
	return sc
}

// F115 T042 authorization (R7, R8): mergeUserData acts for the System Admin who approved the merge and needs
// platform.user.merge — a tenant user, a student, platform staff without the permission or a missing Principal are refused
// before any data access (the use case below has no store: a refused call never reaches one); every lifecycle step needs
// the single tenant of the saga step (no scope → 401, a platform scope without one tenant → 403).
func TestChatLifecycleAuthorization(t *testing.T) {
	h := command.UserLifecycle{}
	in := command.MergeInput{MergeID: uuid.New(), From: uuid.New(), Into: uuid.New()}
	denied := map[string]authz.Principal{
		"tenant admin A":   authztest.Staff(authztest.TenantA, "tenant_admin"),
		"school admin B":   authztest.Staff(authztest.TenantB, "school_admin"),
		"student":          authztest.Student(authztest.TenantA),
		"content admin":    authztest.Platform("content_admin"),
		"safety moderator": authztest.Platform("safety_moderator"),
		"no role (staff)":  authztest.NoRole(authztest.TenantA),
	}
	for who, p := range denied {
		if _, _, err := h.Merge(sagaCtx(p, authztest.TenantA), in); trashCode(err) != problem.PermissionDenied {
			t.Errorf("merge by %s: %v, want PERMISSION_DENIED", who, err)
		}
	}
	if _, _, err := h.Merge(context.Background(), in); trashCode(err) != problem.Unauthenticated {
		t.Errorf("merge without Principal: %v", err)
	}
	sa := authztest.Platform("system_admin")
	platform, err := authz.PlatformScope(sa)
	if err != nil {
		t.Fatal(err)
	}
	cross := tenancy.WithScope(authztest.Ctx(context.Background(), sa), platform)
	steps := map[string]func(ctx context.Context) error{
		"cleanup": func(ctx context.Context) error {
			_, _, err := h.Cleanup(ctx, in.From)
			return err
		},
		"restore": func(ctx context.Context) error {
			_, err := h.Restore(ctx, in.From)
			return err
		},
		"anonymize": func(ctx context.Context) error {
			_, err := h.Anonymize(ctx, in.From)
			return err
		},
		"merge": func(ctx context.Context) error {
			_, _, err := h.Merge(ctx, in)
			return err
		},
	}
	for name, step := range steps {
		if name != "merge" {
			if got := trashCode(step(authz.WithPrincipal(context.Background(), sa))); got != problem.Unauthenticated {
				t.Errorf("%s without scope: %q", name, got)
			}
		}
		if got := trashCode(step(cross)); got != problem.PermissionDenied {
			t.Errorf("%s in a platform scope: %q", name, got)
		}
	}
	// The same source and target is answered blocked without any data access.
	if s, reason, err := h.Merge(sagaCtx(sa, authztest.TenantA), command.MergeInput{MergeID: uuid.New(), From: in.From, Into: in.From}); err != nil ||
		s != command.LifecycleBlocked || reason == "" {
		t.Fatalf("self merge: %s %q %v", s, reason, err)
	}
}
