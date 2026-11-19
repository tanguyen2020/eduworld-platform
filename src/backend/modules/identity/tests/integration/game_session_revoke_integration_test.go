//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/gamelogin"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// fixtureEvent loads a T045 fixture and points it at the given student and tenant.
func fixtureEvent(t *testing.T, name string, user uuid.UUID, tenant string) outbox.Envelope {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "fixtures", "events", name))
	if err != nil {
		t.Fatal(err)
	}
	s := strings.ReplaceAll(string(raw), "0193a5f4-6c1e-7b2a-9d1f-000000000005", user.String())
	s = strings.ReplaceAll(s, "0193a5f4-6c1e-7b2a-9d1f-000000000001", tenant)
	var e outbox.Envelope
	if err := json.Unmarshal([]byte(s), &e); err != nil {
		t.Fatal(err)
	}
	return e
}

// F013-AC-29 (TC-AUTH-087): user.access_revoked, user.status_changed (locked) and student.cleaned_up revoke
// the game sessions of the student (revoked_at, reason, Keycloak session deleted); a repeated event has no
// effect; the next refresh is REFRESH_TOKEN_INVALID; an activation event revokes nothing.
func TestGameSessionRevokeConsumer(t *testing.T) {
	ctx := context.Background()
	g := newGameIntegration(ctx, t, gameOpts{})
	a := seedTenant(ctx, t, g.platform, "pgd-revoke")
	an := g.student(ctx, t, a, "hs.thu.hoi")
	c := consumer.GameAuthRevoke{Revoker: g.auth}
	cases := []struct {
		fixture, reason string
	}{
		{"user.access_revoked.locked.json", gamelogin.RevokeAccountLocked},
		{"user.access_revoked.password_changed.json", gamelogin.RevokePasswordChanged},
		{"user.status_changed.locked.json", gamelogin.RevokeAccountLocked},
		{"student.cleaned_up.json", gamelogin.RevokeCleanup},
	}
	for _, tc := range cases {
		dev := uuid.New()
		r := g.login(t, "hs.thu.hoi", dev)
		if r.code != 200 {
			t.Fatalf("%s login: %d %v", tc.fixture, r.code, r.body)
		}
		rt := tokensOf(t, r)["refresh_token"].(string)
		deleted := g.kc.deleted
		e := fixtureEvent(t, tc.fixture, an, a.tenant)
		for range 2 { // redelivery: no second effect
			if err := c.Handle(ctx, e); err != nil {
				t.Fatalf("%s: %v", tc.fixture, err)
			}
		}
		if g.kc.deleted != deleted+1 {
			t.Fatalf("%s: Keycloak sessions deleted = %d", tc.fixture, g.kc.deleted-deleted)
		}
		if n := count(ctx, t, g.platform, `SELECT count(*) FROM identity.game_auth_sessions WHERE refresh_token_hash = $1 AND revoke_reason = $2
			AND revoked_at IS NOT NULL`, gamelogin.HashToken(rt), tc.reason); n != 1 {
			t.Fatalf("%s: session not revoked with %s", tc.fixture, tc.reason)
		}
		if r := g.refresh(t, rt, dev, a.tenant, "renew", ""); r.code != 401 || codeOf(r) != string(problem.RefreshTokenInvalid) {
			t.Fatalf("%s: refresh after revoke: %d %v", tc.fixture, r.code, r.body)
		}
	}
	// Activation revokes nothing.
	r := g.login(t, "hs.thu.hoi", uuid.New())
	rt := tokensOf(t, r)["refresh_token"].(string)
	if err := c.Handle(ctx, fixtureEvent(t, "user.status_changed.activated.json", an, a.tenant)); err != nil {
		t.Fatal(err)
	}
	if n := count(ctx, t, g.platform, `SELECT count(*) FROM identity.game_auth_sessions WHERE refresh_token_hash = $1 AND revoked_at IS NULL`, gamelogin.HashToken(rt)); n != 1 {
		t.Fatal("activation revoked a session")
	}
	if n := count(ctx, t, g.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded' AND envelope::text LIKE '%auth.game_session_revoked%'`); n != 4 {
		t.Fatalf("revocation audits = %d", n)
	}
	// A malformed payload is permanent (DLQ), not retried.
	bad := fixtureEvent(t, "student.cleaned_up.json", an, a.tenant)
	bad.Payload = json.RawMessage(`{"student_id":"x"}`)
	if err := c.Handle(ctx, bad); !errors.Is(err, outbox.ErrPermanent) {
		t.Fatalf("malformed payload: %v", err)
	}
}
