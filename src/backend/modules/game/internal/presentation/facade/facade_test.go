package facade

import (
	"context"
	"testing"

	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/gamesession"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/profile"
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

// profiles is a TEST DOUBLE (R17) of write.ProfileRepository: live and removed profiles per student; the SQL adapter
// is covered by tests/integration/profile_lifecycle_test.go.
type profiles struct {
	write.ProfileRepository
	live, removed map[uuid.UUID]bool
	anonymized    map[uuid.UUID]string
}

func (p *profiles) Get(_ context.Context, _, s uuid.UUID) (*profile.Profile, error) {
	if p.live[s] {
		return &profile.Profile{StudentID: s}, nil
	}
	return nil, nil
}

func (p *profiles) SoftDelete(_ context.Context, _, s uuid.UUID, _ string, _ time.Time) (bool, error) {
	if !p.live[s] {
		return false, nil
	}
	delete(p.live, s)
	p.removed[s] = true
	return true, nil
}

func (p *profiles) Restore(_ context.Context, _, s uuid.UUID, _ string) (bool, error) {
	if !p.removed[s] {
		return false, nil
	}
	delete(p.removed, s)
	p.live[s] = true
	return true, nil
}

func (p *profiles) Anonymize(_ context.Context, _, s uuid.UUID, anon string) (bool, error) {
	if !p.live[s] && !p.removed[s] {
		return false, nil
	}
	p.anonymized[s] = anon
	return true, nil
}

// AnonymizeHistory: the history of the TEST DOUBLE has no row (the SQL function is covered by
// tests/integration/nickname_privacy_test.go).
func (p *profiles) AnonymizeHistory(context.Context, uuid.UUID, uuid.UUID, string) (int, error) {
	return 0, nil
}

func (p *profiles) Reassign(_ context.Context, _, from, into uuid.UUID) (bool, error) {
	if !p.live[from] {
		return false, nil
	}
	delete(p.live, from)
	p.live[into] = true
	return true, nil
}

// presets is a TEST DOUBLE of write.PresetRepository (merge only).
type presets struct{ write.PresetRepository }

func (presets) MoveStudent(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string, time.Time) error {
	return nil
}

// Receiver-side facade test of cleanupStudentData / restoreStudentData (F016 T021): the examples of
// StudentLifecycleResult in specs/openapi/modules/game.internal.v1.yaml (blocked active_game_session,
// nothing_to_do), the tenant scope requirement and x-allowed-callers.
func TestStudentLifecycleFacade(t *testing.T) {
	busy, idle := uuid.New(), uuid.New()
	ps := &profiles{live: map[uuid.UUID]bool{}, removed: map[uuid.UUID]bool{}, anonymized: map[uuid.UUID]string{}}
	f := Facade{Lifecycle: command.StudentLifecycle{UoW: uow{}, Sessions: sessions{active: map[uuid.UUID]bool{busy: true}},
		Profiles: ps, Presets: presets{}}}
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
	for _, o := range []string{"cleanupStudentData", "restoreStudentData", "anonymizeUserData", "mergeUserData"} {
		if c := api.AllowedCallers[o]; len(c) != 1 || c[0] != "identity" {
			t.Fatalf("x-allowed-callers %s: %v", o, c)
		}
	}
}

// Receiver-side facade test of the game profile lifecycle (F070 T039, OQ F016-2, F045-3): cleanup soft-deletes the
// profile (done), restore brings it back, anonymize overwrites the nickname, mergeUserData keeps one profile and is
// idempotent per merge_id (a second call has nothing to do); an active session blocks cleanup and merge.
func TestProfileLifecycleFacade(t *testing.T) {
	busy, a, b := uuid.New(), uuid.New(), uuid.New()
	ps := &profiles{live: map[uuid.UUID]bool{busy: true, a: true, b: true}, removed: map[uuid.UUID]bool{}, anonymized: map[uuid.UUID]string{}}
	f := Facade{Lifecycle: command.StudentLifecycle{UoW: uow{}, Sessions: sessions{active: map[uuid.UUID]bool{busy: true}},
		Profiles: ps, Presets: presets{}}}
	sc, err := authz.EventScope(uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	ctx := tenancy.WithScope(context.Background(), sc)
	body := &api.CleanupStudentDataJSONRequestBody{CleanupRunId: uuid.New(), Reason: "inactive_cleanup"}
	r, err := f.CleanupStudentData(ctx, api.CleanupStudentDataRequestObject{StudentId: a, Body: body})
	if v, ok := r.(api.CleanupStudentData200JSONResponse); err != nil || !ok || v.Status != "done" || ps.live[a] {
		t.Fatalf("cleanup: %+v %v", r, err)
	}
	rr, err := f.RestoreStudentData(ctx, api.RestoreStudentDataRequestObject{StudentId: a,
		Body: &api.RestoreStudentDataJSONRequestBody{Reason: "student_logged_in_again"}})
	if v, ok := rr.(api.RestoreStudentData200JSONResponse); err != nil || !ok || v.Status != "done" || !ps.live[a] {
		t.Fatalf("restore: %+v %v", rr, err)
	}
	an, err := f.AnonymizeUserData(ctx, api.AnonymizeUserDataRequestObject{UserId: a,
		Body: &api.AnonymizeUserDataJSONRequestBody{ErasureRequestId: uuid.New(), Reason: "data_subject_request"}})
	if v, ok := an.(api.AnonymizeUserData200JSONResponse); err != nil || !ok || v.Status != "done" || ps.anonymized[a] != command.AnonymousNickname(a) {
		t.Fatalf("anonymize: %+v %v", an, err)
	}
	merge := &api.MergeUserDataJSONRequestBody{MergeId: uuid.New(), TargetUserId: b, Reason: "same person"}
	m, err := f.MergeUserData(ctx, api.MergeUserDataRequestObject{UserId: a, Body: merge})
	if v, ok := m.(api.MergeUserData200JSONResponse); err != nil || !ok || v.Status != "done" || ps.live[a] || !ps.live[b] || !ps.removed[a] {
		t.Fatalf("merge: %+v %v", m, err)
	}
	m, err = f.MergeUserData(ctx, api.MergeUserDataRequestObject{UserId: a, Body: merge})
	if v, ok := m.(api.MergeUserData200JSONResponse); err != nil || !ok || v.Status != "nothing_to_do" {
		t.Fatalf("merge again: %+v %v", m, err)
	}
	m, err = f.MergeUserData(ctx, api.MergeUserDataRequestObject{UserId: busy, Body: merge})
	if v, ok := m.(api.MergeUserData200JSONResponse); err != nil || !ok || v.Status != "blocked" || v.BlockedReason == nil ||
		*v.BlockedReason != "active_game_session" {
		t.Fatalf("merge busy: %+v %v", m, err)
	}
	if _, err := f.MergeUserData(ctx, api.MergeUserDataRequestObject{UserId: b, Body: merge}); err == nil {
		t.Fatal("merge into itself")
	}
}
