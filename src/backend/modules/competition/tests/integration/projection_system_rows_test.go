//go:build integration

package integration

import (
	"context"
	"slices"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/persistence"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
)

// F010 Clarifications 2026-11-28: the events the identity migrations write for the tenant common and its virtual school
// reach competition.projection_identity (in the event TenantScope the outbox consumer opens): ref.tenants and
// ref.schools hold `common`; handling them again (re-delivery past processed_events) changes nothing.
func TestProjectionReceivesIdentitySystemRows(t *testing.T) {
	e := setup(t, envOpts{})
	ctx := context.Background()
	h := consumer.IdentityProjection{P: command.Projection{Refs: persistence.Refs{}}}
	apply := func() int {
		n := 0
		for _, env := range e.cl.IdentitySystemEvents(ctx, t) {
			if !slices.Contains(messaging.ProjectionIdentityEvents, env.EventType) || env.TenantID == nil {
				continue // province.updated: not projected by competition
			}
			scope, err := authz.EventScope(*env.TenantID)
			if err != nil {
				t.Fatal(err)
			}
			if err := e.uow.InTenantTx(ctx, scope, func(ctx context.Context) error { return h.Handle(ctx, env) }); err != nil {
				t.Fatalf("%s: %v", env.EventType, err)
			}
			n++
		}
		return n
	}
	if n := apply(); n != 2 {
		t.Fatalf("%d system events for competition, want tenant.created and school.updated", n)
	}
	check := func() {
		t.Helper()
		if n := e.count(`SELECT count(*) FROM ref.tenants WHERE code = 'common' AND tenant_type = 'common' AND status = 'active'`); n != 1 {
			t.Fatal("ref.tenants has no common tenant")
		}
		if n := e.count(`SELECT count(*) FROM ref.schools s JOIN ref.tenants t ON t.tenant_id = s.tenant_id WHERE t.code = 'common'`); n != 1 {
			t.Fatal("ref.schools has no virtual school of common")
		}
	}
	check()
	apply()
	check()
}
