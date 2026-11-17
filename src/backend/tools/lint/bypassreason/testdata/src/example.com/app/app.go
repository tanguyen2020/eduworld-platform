package app

import (
	"context"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

const myReason tenancy.BypassReason = "trash"

func uses(ctx context.Context, dyn tenancy.BypassReason, s string) {
	_, _ = tenancy.Bypass(ctx, tenancy.ReasonTrash)
	_, _ = tenancy.Bypass(ctx, tenancy.ReasonModerationCase, nil)
	_, _ = tenancy.Bypass(ctx, "trash")                       // want `Bypass reason must be a tenancy.Reason\* constant`
	_, _ = tenancy.Bypass(ctx, tenancy.BypassReason("debug")) // want `Bypass reason must be a tenancy.Reason\* constant`
	_, _ = tenancy.Bypass(ctx, dyn)                           // want `Bypass reason must be a tenancy.Reason\* constant`
	_, _ = tenancy.Bypass(ctx, myReason)                      // want `Bypass reason must be a tenancy.Reason\* constant`
	_, _ = tenancy.Bypass(ctx, tenancy.BypassReason(s))       // want `Bypass reason must be a tenancy.Reason\* constant`
	_, _ = authz.CrossTenantScope(authz.Principal{}, nil, tenancy.ReasonTrash)
	_, _ = authz.CrossTenantScope(authz.Principal{}, nil, "x") // want `Bypass reason must be a tenancy.Reason\* constant`
	f := tenancy.Bypass                                        // want `tenancy.Bypass must be called directly`
	_, _ = f(ctx, "x")
}
