package port

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
)

// ErrPrincipalUnknown: module identity knows no such user (or no platform scope for it).
var ErrPrincipalUnknown = errors.New("chat: principal unknown")

// PrincipalDirectory resolves the platform Principal of a staff member through the identity facade resolvePrincipal
// (x-allowed-callers: chat; F124-AC-12 checks that an on-call moderator holds moderation.case.act). Any failure other
// than ErrPrincipalUnknown means unavailable (fail closed).
type PrincipalDirectory interface {
	Platform(ctx context.Context, user uuid.UUID) (authz.Principal, error)
}
