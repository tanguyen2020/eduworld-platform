// Package command holds the write-context use cases of module identity: each handler is a typed
// interface (no mediator, ADR-0087) that runs UseCase → validate → authz.Require → UnitOfWork.Do, writing
// events and audit to the outbox of the same transaction (AGENTS R10, R26).
package command

import (
	"context"
	"errors"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/school"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/tenant"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/user"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

func principal(ctx context.Context) (authz.Principal, error) {
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || !p.Valid() {
		return authz.Principal{}, problem.New(problem.Unauthenticated, "")
	}
	return p, nil
}

// invalid maps a domain rule violation to 422 VALIDATION_FAILED with the field (F010-AC-05).
func invalid(err error) error {
	var te tenant.ErrInvalid
	var se school.ErrInvalid
	var ue user.ErrInvalid
	switch {
	case errors.As(err, &te):
		return problem.Invalid(te.Field, problem.SubInvalid, te.Reason)
	case errors.As(err, &se):
		return problem.Invalid(se.Field, problem.SubInvalid, se.Reason)
	case errors.As(err, &ue):
		return problem.Invalid(ue.Field, problem.SubInvalid, ue.Reason)
	}
	return err
}
