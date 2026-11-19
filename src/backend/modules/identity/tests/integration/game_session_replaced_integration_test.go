//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/gamelogin"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// gameEvent is an envelope of module game (producer game/core) about tenant.
func gameEvent(t *testing.T, typ, tenant string, payload map[string]any) outbox.Envelope {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	tid := uuid.MustParse(tenant)
	return outbox.Envelope{EventID: ids.New(), EventType: typ, SchemaVersion: 1, OccurredAt: time.Now().UTC(), RecordedAt: time.Now().UTC(),
		Producer: "game/core", TenantID: &tid, Actor: outbox.Actor{Type: "student"}, Subject: outbox.Subject{Type: "game_session",
			ID: uuid.NewString()}, IdempotencyKey: typ + ":" + uuid.NewString(), Payload: raw}
}

// F014-AC-04 (TC-AUTH-087, ADR-0086, T014): game_session.replaced revokes, with reason takeover, the game login
// session stored on the old device (old_device_id) and the session of old_keycloak_sid, and ends its Keycloak session
// (Admin API fake, as in F013); the refresh/resume of the old device → 401 REFRESH_TOKEN_INVALID; the new device keeps
// its session; a redelivered event has no second effect; a malformed payload is permanent (DLQ).
func TestRevokeOnSessionReplaced(t *testing.T) {
	ctx := context.Background()
	g := newGameIntegration(ctx, t, gameOpts{})
	a := seedTenant(ctx, t, g.platform, "pgd-takeover")
	an := g.student(ctx, t, a, "hs.takeover")
	devA, devB := uuid.New(), uuid.New()
	ra, rb := g.login(t, "hs.takeover", devA), g.login(t, "hs.takeover", devB)
	if ra.code != 200 || rb.code != 200 {
		t.Fatalf("logins: %d %d", ra.code, rb.code)
	}
	rtA, rtB := tokensOf(t, ra)["refresh_token"].(string), tokensOf(t, rb)["refresh_token"].(string)
	var sidA string
	if err := g.platform.QueryRowContext(ctx, `SELECT keycloak_sid FROM identity.game_auth_sessions WHERE refresh_token_hash = $1`,
		gamelogin.HashToken(rtA)).Scan(&sidA); err != nil {
		t.Fatal(err)
	}
	c := consumer.RevokeOnSessionReplaced{Revoker: g.auth}
	e := gameEvent(t, "game_session.replaced", a.tenant, map[string]any{"old_game_session_id": uuid.NewString(),
		"new_game_session_id": uuid.NewString(), "student_id": an.String(), "new_platform": "android", "old_keycloak_sid": sidA,
		"old_device_id": devA.String()})
	deleted := g.kc.deleted
	for range 2 { // redelivery: no second effect
		if err := c.Handle(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	if g.kc.deleted != deleted+1 {
		t.Fatalf("Keycloak sessions deleted = %d", g.kc.deleted-deleted)
	}
	if n := count(ctx, t, g.platform, `SELECT count(*) FROM identity.game_auth_sessions WHERE refresh_token_hash = $1
		AND revoke_reason = 'takeover' AND revoked_at IS NOT NULL`, gamelogin.HashToken(rtA)); n != 1 {
		t.Fatal("old device session not revoked with takeover")
	}
	if r := g.refresh(t, rtA, devA, a.tenant, "resume", "1.4.0"); r.code != 401 || codeOf(r) != string(problem.RefreshTokenInvalid) {
		t.Fatalf("old device resume: %d %v", r.code, r.body)
	}
	if r := g.refresh(t, rtB, devB, a.tenant, "resume", "1.4.0"); r.code != 200 {
		t.Fatalf("new device resume: %d %v", r.code, r.body)
	}
	bad := gameEvent(t, "game_session.replaced", a.tenant, map[string]any{"student_id": an.String()})
	if err := c.Handle(ctx, bad); !errors.Is(err, outbox.ErrPermanent) {
		t.Fatalf("malformed payload: %v", err)
	}
}

// F014-AC-10 (TC-AUTH-019, T021): student.game_logged_in sets identity.users.last_login_at (asynchronous, ADR-0037),
// idempotent on replay and never moving the time back.
func TestLastLoginOnGameLoggedIn(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	f := seedTenant(ctx, t, e.platform, "pgd-last-login")
	an := enrolled(ctx, t, e.platform, f, "hs.last.login")
	c := consumer.LastLoginOnGameLoggedIn{Users: command.RecordGameLogin{Users: write.LastLoginRepo{}}}
	handle := func(at time.Time) {
		t.Helper()
		ev := gameEvent(t, "student.game_logged_in", f.tenant, map[string]any{"student_id": an.String(), "tenant_id": f.tenant,
			"game_session_id": uuid.NewString(), "login_event_id": uuid.NewString(), "platform": "android", "login_at": at})
		sc, err := authz.EventScope(*ev.TenantID)
		if err != nil {
			t.Fatal(err)
		}
		if err := e.uow.InTenantTx(ctx, sc, func(ctx context.Context) error { return c.Handle(ctx, ev) }); err != nil {
			t.Fatal(err)
		}
	}
	lastLogin := func() time.Time {
		t.Helper()
		var at time.Time
		if err := e.platform.QueryRowContext(ctx, `SELECT last_login_at FROM identity.users WHERE user_id = $1`, an).Scan(&at); err != nil {
			t.Fatal(err)
		}
		return at.UTC()
	}
	t1 := time.Date(2026, 11, 19, 7, 0, 0, 0, time.UTC)
	handle(t1)
	if !lastLogin().Equal(t1) {
		t.Fatalf("last_login_at = %v", lastLogin())
	}
	handle(t1)                      // replay
	handle(t1.Add(-48 * time.Hour)) // late, older event
	if !lastLogin().Equal(t1) {
		t.Fatalf("moved back: %v", lastLogin())
	}
	handle(t1.Add(time.Hour))
	if !lastLogin().Equal(t1.Add(time.Hour)) {
		t.Fatalf("not moved forward: %v", lastLogin())
	}
	wrong := gameEvent(t, "student.game_logged_in", f.tenant, map[string]any{"student_id": an.String(), "tenant_id": uuid.NewString(),
		"login_at": t1})
	if err := c.Handle(ctx, wrong); !errors.Is(err, outbox.ErrPermanent) {
		t.Fatalf("payload tenant differs from the envelope: %v", err)
	}
}
