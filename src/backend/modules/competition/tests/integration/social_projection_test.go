//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/persistence"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// F095 T021 (spec F091 A-1): the consumer competition.projection_social keeps ref.friendships and ref.blocks from the
// relation events of module chat in the event tenant: added/blocked → active, removed/unblocked → inactive, an older
// source_version is ignored, a row never moves to another tenant, and a payload tenant different from the envelope
// goes to the DLQ at once.
func TestSocialProjection(t *testing.T) {
	e := setup(t, envOpts{})
	h := consumer.SocialProjection{P: command.Projection{Refs: persistence.Refs{}}}
	run := func(typ string, tenant uuid.UUID, payload map[string]any) error {
		b, _ := json.Marshal(payload)
		scope, err := authz.EventScope(tenant)
		if err != nil {
			t.Fatal(err)
		}
		tt := tenant
		env := outbox.Envelope{EventID: uuid.New(), EventType: typ, OccurredAt: time.Now(), TenantID: &tt, Payload: b}
		return e.uow.InTenantTx(context.Background(), scope, func(ctx context.Context) error { return h.Handle(ctx, env) })
	}
	lo, hi := "00000000-0000-7000-8000-00000000000a", "00000000-0000-7000-8000-00000000000b"
	active := func(q string, args ...any) bool {
		var a bool
		if err := e.su.QueryRowContext(context.Background(), q, args...).Scan(&a); err != nil {
			t.Fatal(err)
		}
		return a
	}
	pair := map[string]any{"tenant_id": e.tenantA, "user_low_id": lo, "user_high_id": hi}
	with := func(m map[string]any, kv ...any) map[string]any {
		out := map[string]any{}
		for k, v := range m {
			out[k] = v
		}
		for i := 0; i+1 < len(kv); i += 2 {
			out[kv[i].(string)] = kv[i+1]
		}
		return out
	}
	if err := run("friend.added", e.tenantA, with(pair, "source_version", 10)); err != nil {
		t.Fatal(err)
	}
	if !active(`SELECT active FROM ref.friendships WHERE user_low_id = $1 AND user_high_id = $2 AND tenant_id = $3`, lo, hi, e.tenantA) {
		t.Fatal("friendship not projected")
	}
	if err := run("friend.removed", e.tenantA, with(pair, "reason", "blocked", "source_version", 5)); err != nil { // older: ignored
		t.Fatal(err)
	}
	if !active(`SELECT active FROM ref.friendships WHERE user_low_id = $1`, lo) {
		t.Fatal("an older friend.removed was applied")
	}
	if err := run("friend.removed", e.tenantA, with(pair, "reason", "removed", "source_version", 20)); err != nil {
		t.Fatal(err)
	}
	if active(`SELECT active FROM ref.friendships WHERE user_low_id = $1`, lo) {
		t.Fatal("friend.removed not applied")
	}
	// The same pair announced by another tenant never moves the row (refused by the upsert condition or by RLS).
	_ = run("friend.added", e.tenantB, map[string]any{"tenant_id": e.tenantB, "user_low_id": lo, "user_high_id": hi,
		"source_version": 30})
	if n := e.count(`SELECT count(*) FROM ref.friendships WHERE user_low_id = $1 AND tenant_id = $2 AND NOT active`, lo, e.tenantA); n != 1 {
		t.Fatal("the friendship moved to another tenant")
	}
	block := map[string]any{"tenant_id": e.tenantA, "blocker_id": hi, "blocked_id": lo}
	if err := run("user.blocked", e.tenantA, with(block, "source_version", 40)); err != nil {
		t.Fatal(err)
	}
	if !active(`SELECT active FROM ref.blocks WHERE blocker_id = $1 AND blocked_id = $2`, hi, lo) {
		t.Fatal("block not projected")
	}
	if err := run("user.unblocked", e.tenantA, with(block, "source_version", 41)); err != nil {
		t.Fatal(err)
	}
	if active(`SELECT active FROM ref.blocks WHERE blocker_id = $1 AND blocked_id = $2`, hi, lo) {
		t.Fatal("unblock not projected")
	}
	if err := run("user.blocked", e.tenantA, map[string]any{"tenant_id": e.tenantB, "blocker_id": hi, "blocked_id": lo,
		"source_version": 50}); !errors.Is(err, outbox.ErrPermanent) {
		t.Fatalf("payload tenant differs from the envelope: %v", err)
	}
}
