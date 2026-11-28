//go:build integration

package integration

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/facade"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// T039 — F136-AC-28 (TC-TEN-001, TC-TEN-011): uniqueness, history and reset stay in the tenant of the session: the same
// nickname is allowed in two tenants; X-Tenant-Id of another tenant is 403 TENANT_MISMATCH.
func TestNicknameTenantIsolation(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	dung := e.newStudent(ctx, t, e.tenantB, e.schoolB, "active")
	for _, s := range []student{an, dung} {
		if r := e.typeNickname(t, s, "Siêu Nhân Sao Băng"); r.status != http.StatusOK {
			t.Fatalf("%v: %d %s", s.tenant, r.status, r.raw)
		}
	}
	h := map[string]string{"X-Tenant-Id": e.tenantB.String(), "Content-Type": "application/merge-patch+json"}
	if r := e.do(t, http.MethodPatch, "/game/profile", e.gtoken(an), h, map[string]any{"nickname": "Rồng Xanh"}); r.status != http.StatusForbidden ||
		r.code() != string(problem.TenantMismatch) {
		t.Fatalf("X-Tenant-Id of another tenant: %d %s", r.status, r.raw)
	}
	if total := count(ctx, t, e.platform, `SELECT count(*) FROM game.game_profiles WHERE nickname_key = 'siêu nhân sao băng'`); total != 2 {
		t.Fatalf("rows in both tenants: %d", total)
	}
}

// T039 — F136-AC-31 (TC-GAM-014): anonymizeUserData replaces the nickname of the profile (and its key) and every typed
// nickname of the append-only history through game.anonymize_nickname_changes (SECURITY DEFINER), with an audit; no row
// is deleted; the application role still cannot UPDATE the history directly.
func TestAnonymizeNicknameHistory(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.ready(t, an, "Cú Mèo Chăm Chỉ 27")
	if r := e.typeNickname(t, an, "Minh Nhà Bên Hồ"); r.status != http.StatusOK {
		t.Fatalf("typed: %d %s", r.status, r.raw)
	}
	rows := count(ctx, t, e.platform, `SELECT count(*) FROM game.nickname_changes WHERE student_id = $1`, an.id)
	f := facade.Facade{Lifecycle: command.StudentLifecycle{UoW: e.uow, Sessions: write.SessionRepo{}, Profiles: write.ProfileRepo{},
		Presets: write.PresetRepo{}, Audit: write.Auditor{R: e.rec}}}
	sc, err := authz.EventScope(e.tenantA)
	if err != nil {
		t.Fatal(err)
	}
	a, err := f.AnonymizeUserData(tenancy.WithScope(ctx, sc), api.AnonymizeUserDataRequestObject{UserId: an.id,
		Body: &api.AnonymizeUserDataJSONRequestBody{ErasureRequestId: uuid.New(), Reason: "data_subject_request"}})
	if v, ok := a.(api.AnonymizeUserData200JSONResponse); err != nil || !ok || v.Status != "done" {
		t.Fatalf("anonymize: %+v %v", a, err)
	}
	anon := command.AnonymousNickname(an.id)
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.game_profiles WHERE student_id = $1 AND nickname = $2
		AND nickname_key = lower($2)`, an.id, anon); n != 1 {
		t.Fatalf("profile: %d", n)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.nickname_changes WHERE student_id = $1
		AND (old_nickname IN ('Cú Mèo Chăm Chỉ 27', 'Minh Nhà Bên Hồ') OR new_nickname IN ('Cú Mèo Chăm Chỉ 27', 'Minh Nhà Bên Hồ'))`, an.id); n != 0 {
		t.Fatalf("history keeps %d old nicknames", n)
	}
	if got := count(ctx, t, e.platform, `SELECT count(*) FROM game.nickname_changes WHERE student_id = $1`, an.id); got != rows {
		t.Fatalf("history rows %d → %d: nothing deleted", rows, got)
	}
	if e.audits(ctx, t, "game_profile.nickname_history_anonymized") != 1 {
		t.Fatal("anonymization audit")
	}
	if _, err := e.app.ExecContext(ctx, `UPDATE game.nickname_changes SET new_nickname = 'x' WHERE student_id = $1`, an.id); err == nil {
		t.Fatal("game_app may update the append-only history")
	}
}

// T007 — R12 (expand, existing rows): the backfill of the F136 migration gives every live nickname its key (NFC, trimmed,
// collapsed, lower case); when two live rows of a tenant fold to the same key (possible only for old rows that differ in
// Unicode form), the oldest keeps the key and the others keep their nickname without key — nothing is deleted or
// renamed, and the unique index can be created. The block runs again without change (idempotent).
func TestNicknameKeyBackfill(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	raw, err := os.ReadFile(filepath.Join(testkit.RepoRoot(t), "src", "backend", "modules", "game", "migrations",
		"20270204000136_f136_nickname.sql"))
	if err != nil {
		t.Fatal(err)
	}
	s := strings.ReplaceAll(string(raw), "\r\n", "\n")
	i, j := strings.Index(s, "-- BEGIN backfill\n"), strings.Index(s, "-- END backfill")
	if i < 0 || j < i {
		t.Fatal("no backfill block in the migration")
	}
	backfill := s[i:j]
	a, b, c := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active"), e.newStudent(ctx, t, e.tenantA, e.schoolA, "active"),
		e.newStudent(ctx, t, e.tenantB, e.schoolB, "active")
	insert := func(st student, nick, created string) {
		if _, err := e.platform.ExecContext(ctx, `INSERT INTO game.game_profiles (game_profile_id, tenant_id, student_id, school_id,
			created_at, updated_at, settings, nickname) VALUES ($1, $2, $3, $4, $5::timestamptz, now(), '{}', $6)`, uuid.New(), st.tenant, st.id,
			st.school, created, nick); err != nil {
			t.Fatal(err)
		}
	}
	insert(a, "Siêu  Nhân", "2026-11-01T00:00:00Z") // decomposed, double space: the oldest
	insert(b, "Siêu Nhân", "2026-11-02T00:00:00Z")    // precomposed: same key
	insert(c, "Siêu Nhân", "2026-11-03T00:00:00Z")    // another tenant: its own key
	if _, err := e.migrator.ExecContext(ctx, `DROP INDEX game.game_profiles_nickname_key_uq`); err != nil {
		t.Fatal(err)
	}
	if _, err := e.platform.ExecContext(ctx, `UPDATE game.game_profiles SET nickname_key = NULL WHERE student_id IN ($1, $2, $3)`, a.id, b.id, c.id); err != nil {
		t.Fatal(err)
	}
	for run := 0; run < 2; run++ {
		if _, err := e.migrator.ExecContext(ctx, backfill); err != nil {
			t.Fatalf("backfill run %d: %v", run, err)
		}
	}
	key := func(st student) *string {
		var k *string
		if err := e.platform.QueryRowContext(ctx, `SELECT nickname_key FROM game.game_profiles WHERE student_id = $1`, st.id).Scan(&k); err != nil {
			t.Fatal(err)
		}
		return k
	}
	if k := key(a); k == nil || *k != "siêu nhân" {
		t.Fatalf("oldest row key %v", k)
	}
	if k := key(b); k != nil {
		t.Fatalf("duplicate row got key %q", *k)
	}
	if k := key(c); k == nil || *k != "siêu nhân" {
		t.Fatalf("other tenant key %v", k)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.game_profiles WHERE student_id = $1 AND nickname = 'Siêu Nhân'`, b.id); n != 1 {
		t.Fatal("the duplicate keeps its nickname")
	}
	if _, err := e.migrator.ExecContext(ctx, `CREATE UNIQUE INDEX game_profiles_nickname_key_uq ON game.game_profiles (tenant_id, nickname_key)
		WHERE deleted_at IS NULL AND nickname_key IS NOT NULL`); err != nil {
		t.Fatalf("unique index after the backfill: %v", err)
	}
}
