package facade

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/gamesession"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// uow is a TEST DOUBLE of write.UnitOfWork (runs fn inline).
type uow struct{}

func (uow) Do(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }
func (uow) InTenantTx(ctx context.Context, _ tenancy.Scope, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

// sessions is a TEST DOUBLE of write.SessionRepository: only Active is used by the lifecycle facade.
type sessions struct {
	write.SessionRepository
	active map[uuid.UUID]bool
}

func (s sessions) Active(_ context.Context, _, student uuid.UUID) (*gamesession.Session, error) {
	if s.active[student] {
		return &gamesession.Session{}, nil
	}
	return nil, nil
}

// Receiver-side facade test of cleanupStudentData / restoreStudentData (F016 T021): the examples of
// StudentLifecycleResult in specs/openapi/modules/game.internal.v1.yaml (blocked active_game_session,
// nothing_to_do), the tenant scope requirement and x-allowed-callers.
func TestStudentLifecycleFacade(t *testing.T) {
	busy, idle := uuid.New(), uuid.New()
	f := Facade{Lifecycle: command.StudentLifecycle{UoW: uow{}, Sessions: sessions{active: map[uuid.UUID]bool{busy: true}}}}
	sc, err := authz.EventScope(uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	ctx := tenancy.WithScope(context.Background(), sc)
	body := &api.CleanupStudentDataJSONRequestBody{CleanupRunId: uuid.New(), Reason: "inactive_cleanup"}

	resp, err := f.CleanupStudentData(ctx, api.CleanupStudentDataRequestObject{StudentId: busy, Body: body})
	v, ok := resp.(api.CleanupStudentData200JSONResponse)
	if err != nil || !ok || v.Status != "blocked" || v.BlockedReason == nil || *v.BlockedReason != "active_game_session" {
		t.Fatalf("active session: %+v %v", resp, err)
	}
	resp, err = f.CleanupStudentData(ctx, api.CleanupStudentDataRequestObject{StudentId: idle, Body: body})
	if v, ok := resp.(api.CleanupStudentData200JSONResponse); err != nil || !ok || v.Status != "nothing_to_do" || v.BlockedReason != nil {
		t.Fatalf("idle: %+v %v", resp, err)
	}
	if _, err := f.CleanupStudentData(context.Background(), api.CleanupStudentDataRequestObject{StudentId: idle, Body: body}); err == nil {
		t.Fatal("cleanup without a tenant scope")
	}
	r, err := f.RestoreStudentData(ctx, api.RestoreStudentDataRequestObject{StudentId: idle,
		Body: &api.RestoreStudentDataJSONRequestBody{Reason: "student_logged_in_again"}})
	if v, ok := r.(api.RestoreStudentData200JSONResponse); err != nil || !ok || v.Status != "nothing_to_do" {
		t.Fatalf("restore: %+v %v", r, err)
	}
	a, err := f.AnonymizeUserData(ctx, api.AnonymizeUserDataRequestObject{UserId: idle, Body: &api.AnonymizeUserDataJSONRequestBody{ErasureRequestId: uuid.New(), Reason: "data_subject_request"}})
	if v, ok := a.(api.AnonymizeUserData200JSONResponse); err != nil || !ok || v.Status != "nothing_to_do" {
		t.Fatalf("anonymize: %+v %v", a, err)
	}
	for _, o := range []string{"cleanupStudentData", "restoreStudentData", "anonymizeUserData"} {
		if c := api.AllowedCallers[o]; len(c) != 1 || c[0] != "identity" {
			t.Fatalf("x-allowed-callers %s: %v", o, c)
		}
	}
}
