//go:build integration

package integration

import (
	"context"
	"slices"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/consumer"
)

// F010 Clarifications 2026-11-28: the events the identity migrations write for the tenant common and its virtual
// school, relayed as the relay would, reach game.projection_identity — ref.tenants and ref.schools hold `common`, so
// the assignment of platform content to common (FK ref.tenants) and the quest assignments of the seed work.
func TestProjectionReceivesIdentitySystemRows(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	b := newBus(ctx, t)
	b.run(ctx, t, consumer.IdentityProjectionOptions(consumer.IdentityProjection{Upsert: command.UpsertProjection{Refs: write.RefRepo{}}},
		e.uow, e.app, b.js))
	n := 0
	for _, env := range e.cl.IdentitySystemEvents(ctx, t) {
		if !slices.Contains(messaging.ProjectionIdentityEvents, env.EventType) {
			continue // province.updated: not projected by game
		}
		b.publishEnvelope(ctx, t, env)
		e.processed(ctx, t, messaging.ProjectionIdentityName, env)
		n++
	}
	if n != 2 {
		t.Fatalf("%d system events for game, want tenant.created and school.updated", n)
	}
	if c := count(ctx, t, e.migrator, `SELECT count(*) FROM ref.tenants WHERE code = 'common' AND tenant_type = 'common'
		AND status = 'active'`); c != 1 {
		t.Fatal("ref.tenants has no common tenant")
	}
	if c := count(ctx, t, e.migrator, `SELECT count(*) FROM ref.schools s JOIN ref.tenants t ON t.tenant_id = s.tenant_id
		WHERE t.code = 'common' AND s.status = 'active'`); c != 1 {
		t.Fatal("ref.schools has no virtual school of common")
	}
}
