// Package gameadmin holds the authorization of the Game Operations Console (F078 T013, research R3, ADMIN_PORTAL §2.2,
// §2.3, §3.3, §3.12): the write permission of each config type, and the explicit authz.Require of every use case. The
// route guards (internal gateway, azp eduworld-portal, user_type staff, platform role — ADR-0027, ADR-0094) run before;
// these checks are the business authorization (R7): a tenant role never holds game.* (R20, catalog check at startup).
package gameadmin

import (
	"context"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/gameconfig"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// writePermissions maps each config type to its write permission (plan.md table; Clarifications 2026-11-17).
var writePermissions = map[string]authz.Permission{
	"quests":             authz.PermGameQuestWrite,
	"minigame-configs":   authz.PermGameMinigameWrite,
	"npc-dialogues":      authz.PermGameNpcWrite,
	"world-areas":        authz.PermGameWorldWrite,
	"story-chapters":     authz.PermGameWorldWrite,
	"items":              authz.PermGameItemWrite,
	"shop-items":         authz.PermGameShopWrite,
	"collections":        authz.PermGameCollectionWrite,
	"reward-rules":       authz.PermGameRewardRuleWrite,
	"progression-curves": authz.PermGameProgressionWrite,
	"progression-levels": authz.PermGameProgressionWrite,
	"challenges":         authz.PermGameChallengeWrite,
	"bosses":             authz.PermGameBossWrite,
	"achievements":       authz.PermGameAchievementWrite,
	"currencies":         authz.PermGameEconomyWrite,
	"events":             authz.PermGameEventWrite,
	"event-templates":    authz.PermGameEventWrite,
	"season-tracks":      authz.PermGameSeasonTrackWrite,
}

// WritePermission returns the write permission of a config type.
func WritePermission(configType string) (authz.Permission, bool) {
	p, ok := writePermissions[configType]
	return p, ok
}

// WritePermissions returns every write permission of the console (media upload: any of them).
func WritePermissions() []authz.Permission {
	seen := map[authz.Permission]bool{}
	var out []authz.Permission
	for _, p := range writePermissions {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

// Principal returns the platform principal of the request and its PlatformScope: game config data has no tenant
// (tenant_id NULL, owner_scope platform); X-Tenant-Id is ignored on these routes (F078-AC-03).
func Principal(ctx context.Context) (authz.Principal, tenancy.Scope, error) {
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || !p.Valid() {
		return p, nil, problem.New(problem.Unauthenticated, "")
	}
	if p.TenantID != nil || !p.HasPlatformRole() {
		authz.RecordDenied(ctx, "platform_role_required", map[string]any{"user_id": p.UserID.String()})
		return p, nil, problem.New(problem.PermissionDenied, "")
	}
	sc, err := authz.PlatformScope(p)
	if err != nil {
		return p, nil, err
	}
	return p, sc, nil
}

// Type resolves a config type served by module game (404 NOT_FOUND otherwise, F078-AC-03).
func Type(configType string) (gameconfig.Type, error) {
	t, err := gameconfig.Lookup(configType)
	if err != nil {
		return t, problem.New(problem.NotFound, "")
	}
	return t, nil
}

// RequireWrite authorizes the write permission of the type (create, new version, draft, submit, archive).
func RequireWrite(ctx context.Context, p authz.Principal, configType string) error {
	perm, ok := WritePermission(configType)
	if !ok {
		return problem.New(problem.NotFound, "")
	}
	return authz.Require(ctx, p, perm, authz.Resource{})
}

// RequireRead authorizes a read: the write permission of the type, game.content.review or game.content.publish
// (F078-AC-09); a denial is audited as authz.denied for the write permission.
func RequireRead(ctx context.Context, p authz.Principal, configType string) error {
	perm, ok := WritePermission(configType)
	if !ok {
		return problem.New(problem.NotFound, "")
	}
	if p.Has(perm) || p.Has(authz.PermGameContentReview) || p.Has(authz.PermGameContentPublish) {
		return nil
	}
	return authz.Require(ctx, p, perm, authz.Resource{})
}

// RequireReview authorizes approve/reject.
func RequireReview(ctx context.Context, p authz.Principal) error {
	return authz.Require(ctx, p, authz.PermGameContentReview, authz.Resource{})
}

// RequirePublish authorizes publish, scheduled publish, its cancellation, rollback and unpublish (Clarifications
// 2026-11-16: game.content.publish, role liveops).
func RequirePublish(ctx context.Context, p authz.Principal) error {
	return authz.Require(ctx, p, authz.PermGameContentPublish, authz.Resource{})
}

// RequireAnyWrite authorizes the media upload (any game write permission).
func RequireAnyWrite(ctx context.Context, p authz.Principal) error {
	for _, perm := range WritePermissions() {
		if p.Has(perm) {
			return nil
		}
	}
	return authz.Require(ctx, p, authz.PermGameQuestWrite, authz.Resource{})
}

// RequireAnyRead authorizes the read of a media asset (any game write permission, review or publish).
func RequireAnyRead(ctx context.Context, p authz.Principal) error {
	if p.Has(authz.PermGameContentReview) || p.Has(authz.PermGameContentPublish) {
		return nil
	}
	return RequireAnyWrite(ctx, p)
}
