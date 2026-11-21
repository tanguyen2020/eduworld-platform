package natscmd

import (
	"context"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// studentContext builds the request context of a party command of the realtime gateway (F101 T036): the Principal of
// the student and the TenantScope of the ticket, from the Ew-* headers only — never from the payload (F101-AC-20).
// Only the NATS user of binary realtime may publish ew.cmd.> (SECURITY_ARCHITECTURE §8.1); the gateway accepts game
// tickets of students only (R23), and the use cases check membership of the party in that tenant.
func studentContext(ctx context.Context, c rt.Caller) (context.Context, error) {
	tenant := c.TenantID
	p := authz.NewPrincipal(authz.PrincipalData{UserID: c.UserID, UserType: authz.UserTypeStudent, TenantID: &tenant})
	sc, err := authz.ScopeFor(p)
	if err != nil {
		return ctx, err
	}
	return tenancy.WithScope(authz.WithPrincipal(ctx, p), sc), nil
}
