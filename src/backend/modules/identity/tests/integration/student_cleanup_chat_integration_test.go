//go:build integration

package integration

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"

	chatapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/saga"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/participants"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// fakeChatSaga is a TEST DOUBLE (R17) of the lifecycle operations of the chat facade (cleanupStudentData,
// restoreStudentData): it blocks the students of `block` with open_moderation_case like module chat does while a
// moderation case of the student is open, and records the call context. The real facade is tested by module chat
// (tests/integration/user_lifecycle_test.go).
type fakeChatSaga struct {
	mu                sync.Mutex
	block             map[uuid.UUID]bool
	cleaned, restored map[uuid.UUID]int
	callers, keys     []string
	tenants           []uuid.UUID
}

func (f *fakeChatSaga) seen(ctx context.Context) {
	f.callers = append(f.callers, modcall.CallerFrom(ctx))
	k, _ := modcall.IdempotencyKeyFrom(ctx)
	f.keys = append(f.keys, k)
	if sc, ok := tenancy.ScopeFrom(ctx); ok {
		if id, single := sc.SessionTenantID(); single {
			f.tenants = append(f.tenants, id)
		}
	}
}

func (f *fakeChatSaga) CleanupStudentData(ctx context.Context, req chatapi.CleanupStudentDataRequestObject) (chatapi.CleanupStudentDataResponseObject, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seen(ctx)
	if f.block[req.StudentId] {
		r := chatapi.OpenModerationCase
		return chatapi.CleanupStudentData200JSONResponse{StudentId: req.StudentId, Status: "blocked", BlockedReason: &r}, nil
	}
	f.cleaned[req.StudentId]++
	return chatapi.CleanupStudentData200JSONResponse{StudentId: req.StudentId, Status: "done"}, nil
}

func (f *fakeChatSaga) RestoreStudentData(ctx context.Context, req chatapi.RestoreStudentDataRequestObject) (chatapi.RestoreStudentDataResponseObject, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seen(ctx)
	f.restored[req.StudentId]++
	return chatapi.RestoreStudentData200JSONResponse{StudentId: req.StudentId, Status: "done"}, nil
}

// F115 T042 (identity saga with the chat participant, OQ F016-2): the student_cleanup saga calls the chat step through
// participants.Chat + pkg/modcall (caller identity, TenantScope of the student's tenant, Idempotency-Key of the step);
// done → the student is soft-deleted; blocked open_moderation_case → the saga stops, the done game step is compensated
// with restoreStudentData and the student stays as it was (ACCOUNT_PROVISIONING §6b).
func TestStudentCleanupChatStep(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	a := seedTenant(ctx, t, e.platform, "pgd-clchat-a")
	ta := uuid.MustParse(a.tenant)
	ok, held := enrolled(ctx, t, e.platform, a, "hs.clchat.1"), enrolled(ctx, t, e.platform, a, "hs.clchat.2")
	for _, id := range []uuid.UUID{ok, held} {
		if _, err := e.platform.ExecContext(ctx, `UPDATE identity.users SET status = 'active' WHERE user_id = $1`, id); err != nil {
			t.Fatal(err)
		}
	}
	game := newFakeParticipant("game")
	chat := &fakeChatSaga{block: map[uuid.UUID]bool{held: true}, cleaned: map[uuid.UUID]int{}, restored: map[uuid.UUID]int{}}
	parts := []port.StudentDataParticipant{game, participants.Chat{F: chat}}
	eng := saga.Engine{UoW: e.uow, Repo: write.SagaRepo{}}
	cleanup := command.StudentCleanup{UoW: e.uow, Repo: write.CleanupRepo{}, Saga: eng, Participants: parts, Lifecycle: write.LifecycleRepo{},
		Events: e.deps.Events, Audit: e.deps.Audit}
	start := func(id uuid.UUID) string {
		t.Helper()
		subject := map[string]any{"student_id": id, "tenant_id": ta, "run_id": uuid.New(), "cutoff_days": 90}
		o, err := eng.Start(ctx, cleanup.Definition(), &ta, subject, "test:"+uuid.NewString(), nil)
		if err != nil {
			t.Fatalf("cleanup %s: %v", id, err)
		}
		return o.Status
	}
	if st := start(ok); st != saga.StatusSucceeded {
		t.Fatalf("cleanup: %s", st)
	}
	if count(ctx, t, e.platform, `SELECT count(*) FROM identity.users WHERE user_id = $1 AND delete_reason = 'inactive_cleanup'`, ok) != 1 ||
		chat.cleaned[ok] != 1 || game.cleaned[ok] != 1 {
		t.Fatalf("cleaned: chat=%d game=%d", chat.cleaned[ok], game.cleaned[ok])
	}
	if st := start(held); st == saga.StatusSucceeded {
		t.Fatalf("cleanup with an open moderation case: %s", st)
	}
	if count(ctx, t, e.platform, `SELECT count(*) FROM identity.users WHERE user_id = $1 AND deleted_at IS NULL`, held) != 1 {
		t.Fatal("a student with an open moderation case was removed")
	}
	if game.cleaned[held] != 1 || game.restored[held] != 1 || chat.cleaned[held] != 0 || chat.restored[held] != 0 {
		t.Fatalf("compensation: game cleaned=%d restored=%d, chat cleaned=%d restored=%d", game.cleaned[held], game.restored[held],
			chat.cleaned[held], chat.restored[held])
	}
	if count(ctx, t, e.platform, `SELECT count(*) FROM platform.saga_steps st JOIN platform.saga_runs r ON r.saga_id = st.saga_id
		WHERE r.subject->>'student_id' = $1 AND st.service = 'chat' AND st.response->>'blocked_reason' = 'open_moderation_case'`, held.String()) != 1 {
		t.Fatal("blocked reason of the chat step not recorded")
	}
	for i, c := range chat.callers {
		if c != "identity" || chat.keys[i] == "" || chat.tenants[i] != ta {
			t.Fatalf("call %d: caller %q key %q tenant %v", i, c, chat.keys[i], chat.tenants)
		}
	}
}
