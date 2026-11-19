package command

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

func principal(ctx context.Context) (authz.Principal, error) {
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || !p.Valid() {
		return authz.Principal{}, problem.New(problem.Unauthenticated, "")
	}
	return p, nil
}

// platformOp authorizes a platform operation of a facade (the original Principal travels in ctx, ADR-0074 §3):
// the permission and platform staff (no session tenant), else 403 PERMISSION_DENIED.
func platformOp(ctx context.Context, perm authz.Permission) (authz.Principal, error) {
	p, err := principal(ctx)
	if err != nil {
		return p, err
	}
	if err := authz.Require(ctx, p, perm, authz.Resource{}); err != nil {
		return p, err
	}
	if !p.IsPlatform() {
		return p, problem.New(problem.PermissionDenied, "")
	}
	return p, nil
}

// invalid maps a domain rule violation to 422 VALIDATION_FAILED with its field(s).
func invalid(err error) error {
	var ie contentmodel.ErrInvalid
	var vs contentmodel.Violations
	switch {
	case errors.As(err, &ie):
		return problem.Invalid(ie.Field, problem.SubInvalid, ie.Reason)
	case errors.As(err, &vs):
		pe := &problem.Error{Code: problem.ValidationFailed, Detail: "request validation failed"}
		for _, v := range vs {
			sub := problem.SubInvalid
			if v.Required {
				sub = problem.SubRequired
			}
			pe.AddField(v.Field, sub, v.Reason)
		}
		return pe
	}
	return err
}

// resourceType parses a resource type owned by module learning (422 otherwise, F020-AC-06).
func resourceType(s string) (contentmodel.ResourceType, error) {
	rt, err := contentmodel.ParseResourceType(s)
	if err != nil {
		return "", invalid(err)
	}
	return rt, nil
}

// auditIn records e attributed to tenant (envelope tenant_id of audit.recorded) so the tenant's audit log
// shows platform operations on its data (assignment, clone — F020-AC-20, F020-AC-30). The data scope of the
// transaction is unchanged: only the audit context carries the tenant.
func auditIn(ctx context.Context, a write.Auditor, tenant uuid.UUID, e audit.Event) error {
	sc, err := authz.EventScope(tenant)
	if err != nil {
		return err
	}
	return a.Record(tenancy.WithScope(ctx, sc), e)
}

func now(clock func() time.Time) time.Time {
	if clock != nil {
		return clock().UTC()
	}
	return time.Now().UTC()
}

// module labels data access and bypass metrics (eduworld_tenant_filter_bypass_total{module}).
const module = "learning"
