//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/cache"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/facade"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// T039 (OQ F016-2, F045-3; receiver side with the real SQL adapters): cleanupStudentData soft-deletes the game profile
// and its outfits (blocked while a session is active), restoreStudentData brings them back (a nickname taken meanwhile
// is cleared), anonymizeUserData overwrites the nickname and drops the avatar, mergeUserData keeps the target's
// profile, moves outfits to free slots and is idempotent.
func TestProfileLifecycleFacade(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	f := facade.Facade{Lifecycle: command.StudentLifecycle{UoW: e.uow, Sessions: write.SessionRepo{}, Profiles: write.ProfileRepo{},
		Presets: write.PresetRepo{}}}
	sc, err := authz.EventScope(e.tenantA)
	if err != nil {
		t.Fatal(err)
	}
	fctx := tenancy.WithScope(ctx, sc)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	binh := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.ready(t, an, "Cú Mèo Chăm Chỉ 27")
	if r := e.do(t, http.MethodPut, "/game/avatar-presets/2", e.gtoken(an), th(an), map[string]any{"avatar_config": avatar()}); r.status != http.StatusOK {
		t.Fatal(r.raw)
	}
	// An active session blocks the cleanup.
	d := e.device(an)
	start := e.start(t, an, d, newKey(), nil)
	body := &api.CleanupStudentDataJSONRequestBody{CleanupRunId: uuid.New(), Reason: "inactive_cleanup"}
	r, err := f.CleanupStudentData(fctx, api.CleanupStudentDataRequestObject{StudentId: an.id, Body: body})
	if v, ok := r.(api.CleanupStudentData200JSONResponse); err != nil || !ok || v.Status != "blocked" {
		t.Fatalf("active session: %+v %v", r, err)
	}
	if res := e.end(t, an, d, start.body["game_session_id"].(string)); res.status != http.StatusNoContent {
		t.Fatal(res.raw)
	}
	r, err = f.CleanupStudentData(fctx, api.CleanupStudentDataRequestObject{StudentId: an.id, Body: body})
	if v, ok := r.(api.CleanupStudentData200JSONResponse); err != nil || !ok || v.Status != "done" {
		t.Fatalf("cleanup: %+v %v", r, err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.game_profiles WHERE student_id = $1 AND deleted_at IS NOT NULL
		AND delete_reason = 'inactive_cleanup'`, an.id) + count(ctx, t, e.platform, `SELECT count(*) FROM game.avatar_presets
		WHERE student_id = $1 AND deleted_at IS NOT NULL`, an.id); n != 2 {
		t.Fatalf("soft-deleted rows: %d", n)
	}
	// Meanwhile binh takes the nickname an had: the restore keeps the profile but clears the nickname.
	e.ready(t, binh, "cú mèo chăm chỉ 27")
	rr, err := f.RestoreStudentData(fctx, api.RestoreStudentDataRequestObject{StudentId: an.id,
		Body: &api.RestoreStudentDataJSONRequestBody{Reason: "student_logged_in_again"}})
	if v, ok := rr.(api.RestoreStudentData200JSONResponse); err != nil || !ok || v.Status != "done" {
		t.Fatalf("restore: %+v %v", rr, err)
	}
	if p := e.profile(t, an); p.body["nickname"] != nil || p.body["avatar_config"] == nil || p.body["setup_required"] != true {
		t.Fatalf("restored profile: %s", p.raw)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.avatar_presets WHERE student_id = $1 AND deleted_at IS NULL`, an.id); n != 1 {
		t.Fatalf("restored outfits: %d", n)
	}
	// Anonymization: nickname overwritten, avatar removed, statistics kept.
	a, err := f.AnonymizeUserData(fctx, api.AnonymizeUserDataRequestObject{UserId: binh.id,
		Body: &api.AnonymizeUserDataJSONRequestBody{ErasureRequestId: uuid.New(), Reason: "data_subject_request"}})
	if v, ok := a.(api.AnonymizeUserData200JSONResponse); err != nil || !ok || v.Status != "done" {
		t.Fatalf("anonymize: %+v %v", a, err)
	}
	if p := e.profile(t, binh); p.body["nickname"] != command.AnonymousNickname(binh.id) || p.body["avatar_config"] != nil ||
		p.body["avatar_created_at"] == nil {
		t.Fatalf("anonymized: %s", p.raw)
	}
	// Merge an into binh: binh keeps its profile, an's outfit of slot 2 moves (binh has none there).
	merge := &api.MergeUserDataJSONRequestBody{MergeId: uuid.New(), TargetUserId: binh.id, Reason: "same person"}
	m, err := f.MergeUserData(fctx, api.MergeUserDataRequestObject{UserId: an.id, Body: merge})
	if v, ok := m.(api.MergeUserData200JSONResponse); err != nil || !ok || v.Status != "done" {
		t.Fatalf("merge: %+v %v", m, err)
	}
	m, err = f.MergeUserData(fctx, api.MergeUserDataRequestObject{UserId: an.id, Body: merge})
	if v, ok := m.(api.MergeUserData200JSONResponse); err != nil || !ok || v.Status != "nothing_to_do" {
		t.Fatalf("merge again: %+v %v", m, err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.game_profiles WHERE student_id = $1 AND deleted_at IS NULL`, an.id); n != 0 {
		t.Fatal("source profile still live")
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.avatar_presets WHERE student_id = $1 AND slot = 2 AND deleted_at IS NULL`,
		binh.id); n != 1 {
		t.Fatalf("moved outfit: %d", n)
	}
	// A student without a profile of the target: the source profile moves as is.
	c := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	dd := uuid.New()
	e.ready(t, c, "Rái Cá Khéo Tay 44")
	m, err = f.MergeUserData(fctx, api.MergeUserDataRequestObject{UserId: c.id, Body: &api.MergeUserDataJSONRequestBody{MergeId: uuid.New(),
		TargetUserId: dd, Reason: "same person"}})
	if v, ok := m.(api.MergeUserData200JSONResponse); err != nil || !ok || v.Status != "done" {
		t.Fatalf("merge move: %+v %v", m, err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.game_profiles WHERE student_id = $1 AND nickname = 'Rái Cá Khéo Tay 44'
		AND deleted_at IS NULL`, dd); n != 1 {
		t.Fatalf("moved profile: %d", n)
	}
}

// T023: the Valkey adapter of the nickname options — options of one student only, TTL nickname_suggestion_ttl,
// cleared after the choice; Valkey losing them only means 409 NICKNAME_UNAVAILABLE (temporary data, ADR-0004).
func TestNicknameOptionStoreValkey(t *testing.T) {
	ctx := context.Background()
	_, addr := testkit.StartValkey(ctx, t)
	c, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{addr}, DisableCache: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Close)
	s := cache.NicknameOptionStore{C: c}
	an, binh := uuid.New(), uuid.New()
	o1, o2 := uuid.New(), uuid.New()
	if err := s.Save(ctx, an, map[uuid.UUID]string{o1: "Cú Mèo Chăm Chỉ 27", o2: "Sóc Nâu Vui Vẻ 12"}, 2*time.Second); err != nil {
		t.Fatal(err)
	}
	if n, ok, err := s.Get(ctx, an, o1); err != nil || !ok || n != "Cú Mèo Chăm Chỉ 27" {
		t.Fatalf("get: %q %v %v", n, ok, err)
	}
	if _, ok, err := s.Get(ctx, binh, o1); err != nil || ok {
		t.Fatal("option of another student found")
	}
	ttl, err := c.Do(ctx, c.B().Ttl().Key(cache.Key(an)).Build()).AsInt64()
	if err != nil || ttl <= 0 || ttl > 2 {
		t.Fatalf("ttl %d %v", ttl, err)
	}
	if err := s.Clear(ctx, an); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := s.Get(ctx, an, o2); ok {
		t.Fatal("cleared option found")
	}
	_ = s.Save(ctx, binh, map[uuid.UUID]string{o1: "Cá Heo Tò Mò 33"}, time.Second)
	time.Sleep(2100 * time.Millisecond)
	if _, ok, _ := s.Get(ctx, binh, o1); ok {
		t.Fatal("expired option found")
	}
}
