// Package facade implements the in-process facade of module chat (chatapi, generated from
// specs/openapi/modules/chat.internal.v1.yaml): module competition calls moderateDisplayText through pkg/modcall before
// storing a party or team name (ADR-0074, AGENTS R29); no route of this package is mounted.
package facade

import (
	"context"
	"fmt"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// DisplayTextChecker is the display-text check of the moderation pipeline (pipeline.Validator.CheckDisplayText) and the
// screening of school texts (pipeline.Validator.ScreenTexts, F077).
type DisplayTextChecker interface {
	CheckDisplayText(ctx context.Context, tenant uuid.UUID, text string) (bool, error)
	ScreenTexts(ctx context.Context, tenant uuid.UUID, texts []string) ([]int, error)
}

// ScreenText implements chatapi.StrictServerInterface (x-allowed-callers: game, x-context-required: tenant_scope — school
// event texts, F077-AC-23): allowed and the positions of the texts that are not clean only — never the matched term, its
// position or category (R15). An error of the pipeline is returned (the caller answers 503 and stores nothing).
func (f Facade) ScreenText(ctx context.Context, req api.ScreenTextRequestObject) (api.ScreenTextResponseObject, error) {
	if req.Body == nil {
		return nil, problem.Invalid("body", problem.SubRequired, "text screening request required")
	}
	b := *req.Body
	if len(b.Texts) == 0 || len(b.Texts) > 10 {
		return nil, problem.Invalid("texts", problem.SubInvalid, "1 to 10 texts")
	}
	for i, t := range b.Texts {
		if n := utf8.RuneCountInString(t); n == 0 || n > 4000 {
			return nil, problem.Invalid(fmt.Sprintf("texts.%d", i), problem.SubInvalid, "1 to 4000 characters")
		}
	}
	if !b.Locale.Valid() {
		return nil, problem.Invalid("locale", problem.SubInvalid, "vi or en")
	}
	// The tenant of the body must be the tenant of the caller's scope (never taken from input alone, R8).
	sc, ok := tenancy.ScopeFrom(ctx)
	if !ok {
		return nil, problem.New(problem.Unauthenticated, "")
	}
	if id, single := sc.SessionTenantID(); !single || id != b.TenantId {
		return nil, problem.New(problem.PermissionDenied, "")
	}
	violations, err := f.Moderation.ScreenTexts(ctx, b.TenantId, b.Texts)
	if err != nil {
		return nil, err
	}
	if violations == nil {
		violations = []int{}
	}
	return api.ScreenText200JSONResponse{Allowed: len(violations) == 0, Violations: violations}, nil
}

// Facade serves the operations of module chat called by other modules.
type Facade struct {
	Moderation DisplayTextChecker
	// Assignments serves the tenant assignments of the sticker packs (F082 T040); nil = 503 UNAVAILABLE.
	Assignments Assignments
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
