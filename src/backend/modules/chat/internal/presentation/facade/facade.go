// Package facade implements the in-process facade of module chat (chatapi, generated from
// specs/openapi/modules/chat.internal.v1.yaml): module competition calls moderateDisplayText through pkg/modcall before
// storing a party or team name (ADR-0074, AGENTS R29); no route of this package is mounted.
package facade

import (
	"context"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// DisplayTextChecker is the display-text check of the moderation pipeline (pipeline.Validator.CheckDisplayText).
type DisplayTextChecker interface {
	CheckDisplayText(ctx context.Context, tenant uuid.UUID, text string) (bool, error)
}

// Facade serves the operations of module chat called by other modules.
type Facade struct {
	Moderation DisplayTextChecker
}

// ModerateDisplayText implements chatapi.StrictServerInterface (x-allowed-callers: competition, x-context-required:
// tenant_scope): the decision only — never the matched term, its position or category (R15).
func (f Facade) ModerateDisplayText(ctx context.Context, req api.ModerateDisplayTextRequestObject) (api.ModerateDisplayTextResponseObject, error) {
	if req.Body == nil {
		return nil, problem.Invalid("body", problem.SubRequired, "display text check required")
	}
	b := *req.Body
	if n := utf8.RuneCountInString(b.Text); n == 0 || n > 255 {
		return nil, problem.Invalid("text", problem.SubInvalid, "1 to 255 characters")
	}
	if b.Context != api.PartyName && b.Context != api.TeamName {
		return nil, problem.Invalid("context", problem.SubInvalid, "party_name or team_name")
	}
	// The tenant of the body must be the tenant of the caller's scope (never taken from input alone, R8).
	sc, ok := tenancy.ScopeFrom(ctx)
	if !ok {
		return nil, problem.New(problem.Unauthenticated, "")
	}
	if id, single := sc.SessionTenantID(); !single || id != b.TenantId {
		return nil, problem.New(problem.PermissionDenied, "")
	}
	allowed, err := f.Moderation.CheckDisplayText(ctx, b.TenantId, b.Text)
	if err != nil {
		return nil, err
	}
	d := api.DisplayTextCheckResultDecisionBlocked
	if allowed {
		d = api.DisplayTextCheckResultDecisionAllowed
	}
	return api.ModerateDisplayText200JSONResponse{Decision: d}, nil
}
