//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// F014-AC-27 (OQ-17, CHILD_SAFETY §4.2): with daily_play_limit_minutes on the school (projected from school.updated),
// the job curfew-session-closer warns once curfew_warning_minutes before the limit (session.session.ending_soon),
// then ends the session with end_reason daily_limit (login event, game_session.ended, session.session.ended with
// reopen_at); createGameSession and takeover are refused with 403 DAILY_PLAY_LIMIT_REACHED + reopen_at = 00:00 ICT
// the next day, and work again the next day. A school without the limit and the common tenant are not limited.
func TestDailyPlayLimit(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	if _, err := e.migrator.ExecContext(ctx, `UPDATE ref.schools SET settings_subset = '{"daily_play_limit_minutes": 60}' WHERE school_id = $1`,
		e.schoolA); err != nil {
		t.Fatal(err)
	}
	common, commonSchool := uuid.New(), uuid.New()
	if _, err := e.migrator.ExecContext(ctx, `INSERT INTO ref.tenants (tenant_id, code, tenant_type, status, source_version)
		VALUES ($1, 'common', 'common', 'active', 1)`, common); err != nil {
		t.Fatal(err)
	}
	if _, err := e.migrator.ExecContext(ctx, `INSERT INTO ref.schools (school_id, tenant_id, settings_subset, source_version)
		VALUES ($1, $2, '{"daily_play_limit_minutes": 15}', 1)`, commonSchool, common); err != nil {
		t.Fatal(err)
	}
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")    // school with a 60-minute limit
	free := e.newStudent(ctx, t, e.tenantA, e.schoolA2, "active") // same tenant, school without limit
	chung := e.newStudent(ctx, t, common, commonSchool, "active") // common tenant: never limited
	e.clock.Set(time.Date(2026, 11, 19, 8, 0, 0, 0, ict))
	ids := map[string]string{}
	for name, s := range map[string]student{"an": an, "free": free, "chung": chung} {
		r := e.start(t, s, e.device(s), newKey(), nil)
		if r.status != http.StatusCreated {
			t.Fatalf("%s: %d %s", name, r.status, r.raw)
		}
		ids[name] = r.body["game_session_id"].(string)
	}
	tick := func(at time.Time) int {
		t.Helper()
		e.clock.Set(at)
		n, err := e.sessions.CloseSessionsAtCurfew(ctx, 10)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	warned := func() int {
		n := 0
		for _, m := range e.pushes.all() {
			if m == "ending_soon:"+an.id.String()+":"+ids["an"]+":daily_limit" {
				n++
			}
		}
		return n
	}
	if n := tick(time.Date(2026, 11, 19, 8, 49, 30, 0, ict)); n != 0 || warned() != 0 { // 10.5 min left
		t.Fatalf("08:49:30: ended %d, warned %d", n, warned())
	}
	if n := tick(time.Date(2026, 11, 19, 8, 50, 20, 0, ict)); n != 0 || warned() != 1 { // 9m40s left: warn once
		t.Fatalf("08:50:20: ended %d, warned %d %v", n, warned(), e.pushes.all())
	}
	if n := tick(time.Date(2026, 11, 19, 8, 51, 20, 0, ict)); n != 0 || warned() != 1 {
		t.Fatalf("08:51:20: ended %d, warned %d", n, warned())
	}
	end := time.Date(2026, 11, 19, 9, 0, 10, 0, ict)
	if n := tick(end); n != 1 {
		t.Fatalf("09:00:10: ended %d", n)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.game_sessions s JOIN game.login_events l USING (game_session_id)
		WHERE s.game_session_id = $1 AND s.end_reason = 'daily_limit' AND l.end_reason = 'daily_limit' AND s.ended_at = $2`, ids["an"], end); n != 1 {
		t.Fatal("session not ended with daily_limit")
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.game_sessions WHERE ended_at IS NULL AND game_session_id IN ($1, $2)`,
		ids["free"], ids["chung"]); n != 2 {
		t.Fatal("a session without limit (other school, common tenant) was ended")
	}
	if !slices.Contains(e.pushes.all(), "ended:"+an.id.String()+":"+ids["an"]+":daily_limit:reopen") {
		t.Fatalf("session.session.ended daily_limit: %v", e.pushes.all())
	}
	found := false
	for _, p := range e.payloads(ctx, t, "game_session.ended") {
		if p["game_session_id"] == ids["an"] && p["end_reason"] == "daily_limit" {
			found = true
		}
	}
	if !found {
		t.Fatal("game_session.ended end_reason daily_limit")
	}
	// The limit is reached: a new session and a takeover are refused until 00:00 ICT the next day.
	reopen := time.Date(2026, 11, 20, 0, 0, 0, 0, ict)
	e.clock.Set(time.Date(2026, 11, 19, 15, 0, 0, 0, ict))
	for _, r := range []resp{e.start(t, an, e.device(an), newKey(), nil), e.takeover(t, an, e.device(an))} {
		if r.status != http.StatusForbidden || r.code() != string(problem.DailyPlayLimitReached) {
			t.Fatalf("at the limit: %d %s", r.status, r.raw)
		}
		if at, _ := time.Parse(time.RFC3339, r.body["reopen_at"].(string)); !at.Equal(reopen) {
			t.Fatalf("reopen_at %v", r.body["reopen_at"])
		}
	}
	e.clock.Set(reopen.Add(30 * time.Second))
	if r := e.start(t, an, e.device(an), newKey(), nil); r.status != http.StatusCreated {
		t.Fatalf("next day: %d %s", r.status, r.raw)
	}
}

// OQ F014-1 (T044): ref.students.sis_connection_id follows student.sis_link_created / student.sis_link_superseded /
// student.synced, and the login event of a new session records it (no call to module identity, R30).
func TestLoginEventSISConnection(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	c := consumer.IdentityProjection{Upsert: command.UpsertProjection{Refs: write.RefRepo{}}}
	handle := func(typ string, tenant uuid.UUID, payload map[string]any) error {
		t.Helper()
		raw, _ := json.Marshal(payload)
		env := outbox.Envelope{EventID: uuid.Must(uuid.NewV7()), EventType: typ, SchemaVersion: 1, OccurredAt: time.Now().UTC(),
			RecordedAt: time.Now().UTC(), Producer: "identity/test", TenantID: &tenant, Payload: raw}
		sc, err := authz.EventScope(tenant)
		if err != nil {
			t.Fatal(err)
		}
		return e.uow.InTenantTx(ctx, sc, func(ctx context.Context) error { return c.Handle(ctx, env) })
	}
	first, second := uuid.New(), uuid.New()
	if err := handle("student.sis_link_created", e.tenantA, map[string]any{"user_id": an.id.String(), "tenant_id": e.tenantA.String(),
		"school_id": e.schoolA.String(), "sis_connection_id": first.String(), "student_external_id": "HS-1", "via": "sync"}); err != nil {
		t.Fatal(err)
	}
	r := e.start(t, an, e.device(an), newKey(), nil)
	if r.status != http.StatusCreated {
		t.Fatalf("start: %d %s", r.status, r.raw)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.login_events WHERE game_session_id = $1 AND sis_connection_id = $2`,
		r.body["game_session_id"], first); n != 1 {
		t.Fatal("login event without the SIS connection")
	}
	if err := handle("student.sis_link_superseded", e.tenantA, map[string]any{"user_id": an.id.String(), "old_sis_connection_id": first.String(),
		"old_student_external_id": "HS-1", "new_sis_connection_id": second.String(), "new_student_external_id": "HS-9", "cross_tenant": false}); err != nil {
		t.Fatal(err)
	}
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM ref.students WHERE student_id = $1 AND sis_connection_id = $2`, an.id, second); n != 1 {
		t.Fatal("superseded link not projected")
	}
	third := uuid.New()
	if err := handle("student.synced", e.tenantA, map[string]any{"student_id": an.id.String(), "school_id": e.schoolA.String(),
		"action": "update", "sync_run_id": uuid.NewString(), "sis_connection_id": third.String(), "source_version": 5}); err != nil {
		t.Fatal(err)
	}
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM ref.students WHERE student_id = $1 AND sis_connection_id = $2`, an.id, third); n != 1 {
		t.Fatal("student.synced connection not projected")
	}
	// A link event whose payload tenant differs from the envelope is refused; one of another tenant changes nothing.
	if err := handle("student.sis_link_created", e.tenantA, map[string]any{"user_id": an.id.String(), "tenant_id": e.tenantB.String(),
		"school_id": e.schoolB.String(), "sis_connection_id": first.String(), "student_external_id": "HS-1", "via": "sync"}); err == nil {
		t.Fatal("tenant mismatch accepted")
	}
	if err := handle("student.sis_link_superseded", e.tenantB, map[string]any{"user_id": an.id.String(), "old_sis_connection_id": third.String(),
		"old_student_external_id": "HS-9", "new_sis_connection_id": first.String(), "new_student_external_id": "HS-1", "cross_tenant": true}); err != nil {
		t.Fatal(err)
	}
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM ref.students WHERE student_id = $1 AND sis_connection_id = $2`, an.id, third); n != 1 {
		t.Fatal("an event of another tenant changed the student")
	}
}
