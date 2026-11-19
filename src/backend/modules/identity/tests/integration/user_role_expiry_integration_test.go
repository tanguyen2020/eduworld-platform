//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
)

// F012 T032: user roles past valid_to are closed by the job user-role-expiry with user_role.changed op expired.
func TestUserRoleExpiry(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	a := seedTenant(ctx, t, e.platform, "pgd-exp")
	u := staffUser(ctx, t, e, "gv.exp", &a.tenant)
	grantRole(ctx, t, e, u, "school_admin", &a.tenant, "tenant", nil, "now() - interval '1 minute'")
	grantRole(ctx, t, e, u, "academic_manager", &a.tenant, "tenant", nil, "now() + interval '1 day'")
	job := command.ExpireUserRoles{UoW: e.uow, Expiry: write.UserRoleRepo{}, Repo: write.UserRoleRepo{}, Events: e.deps.Events, Audit: e.deps.Audit}
	if n, err := job.RunOnce(ctx); err != nil || n != 1 {
		t.Fatalf("expired = %d %v", n, err)
	}
	if n, err := job.RunOnce(ctx); err != nil || n != 0 {
		t.Fatalf("second run = %d %v", n, err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM identity.user_roles WHERE user_id = $1 AND delete_reason = 'expired'`, u); n != 1 {
		t.Fatalf("expired rows = %d", n)
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'user_role.changed'
		AND envelope->'payload'->>'op' = 'expired' AND envelope->'payload'->>'actor_id' IS NULL`); n != 1 {
		t.Fatalf("expired events = %d", n)
	}
}
