// Package gameadmin holds what the write and read use cases of the competition configurations share (F108 US1, API_DESIGN
// §9.13b, ADMIN_PORTAL §2.2, §3.4): the write permission of each config type, the explicit authz.Require of every use
// case, and the contract views. The route guards (internal gateway, azp eduworld-portal, user_type staff, platform role —
// ADR-0027, ADR-0094) run before; these checks are the business authorization (R7): a tenant role never holds
// competition.config.* nor game.content.* (R20, catalog check at startup).
package gameadmin

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/gameconfig"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// writePermissions maps each config type to its write permission (F108-AC-02; tournament and challenge templates share
// competition.config.tournament_template.write; competition-rewards uses competition.config.reward.write — RB-415).
var writePermissions = map[string]authz.Permission{
	gameconfig.TypeMatchmaking:  authz.PermCompetitionConfigMatchmakingWrite,
	gameconfig.TypeRanking:      authz.PermCompetitionConfigRankingWrite,
	gameconfig.TypeSeasons:      authz.PermCompetitionConfigSeasonWrite,
	gameconfig.TypeTournamentTp: authz.PermCompetitionConfigTournamentTemplateWrite,
	gameconfig.TypeChallengeTp:  authz.PermCompetitionConfigTournamentTemplateWrite,
	gameconfig.TypeLeaderboards: authz.PermCompetitionConfigLeaderboardWrite,
	gameconfig.TypeRewards:      authz.PermCompetitionConfigRewardWrite,
}

// WritePermission returns the write permission of a config type.
func WritePermission(configType string) (authz.Permission, bool) {
	p, ok := writePermissions[configType]
	return p, ok
}

// Principal returns the platform principal of the request and its PlatformScope: competition configurations have no
// tenant (G, owner_scope platform); X-Tenant-Id is ignored on these routes. A principal without a platform role or bound
// to a tenant is 403 PERMISSION_DENIED with authz.denied (F108-AC-03).
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

// Type resolves a config type served by module competition (404 NOT_FOUND otherwise).
func Type(configType string) (gameconfig.Type, error) {
	t, err := gameconfig.Lookup(configType)
	if err != nil {
		return t, problem.New(problem.NotFound, "")
	}
	return t, nil
}

// RequireWrite authorizes the write permission of the type (create, draft, submit, archive).
func RequireWrite(ctx context.Context, p authz.Principal, configType string) error {
	perm, ok := WritePermission(configType)
	if !ok {
		return problem.New(problem.NotFound, "")
	}
	return authz.Require(ctx, p, perm, authz.Resource{})
}

// RequireRead authorizes a read: the write permission of the type, game.content.review or game.content.publish; a
// denial is audited as authz.denied for the write permission.
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

// RequireReview authorizes approve/reject (game.content.review).
func RequireReview(ctx context.Context, p authz.Principal) error {
	return authz.Require(ctx, p, authz.PermGameContentReview, authz.Resource{})
}

// RequirePublish authorizes publish, rollback and unpublish (game.content.publish).
func RequirePublish(ctx context.Context, p authz.Principal) error {
	return authz.Require(ctx, p, authz.PermGameContentPublish, authz.Resource{})
}

// ETagOf is the entity tag of a version row (its optimistic revision).
func ETagOf(revision int64) string { return `"` + strconv.FormatInt(revision, 10) + `"` }

// ConfigVersionView is GameConfigVersion of the contract; ETag is the header of the response.
type ConfigVersionView struct {
	ConfigType      string         `json:"config_type"`
	ConfigID        uuid.UUID      `json:"config_id"`
	ConfigVersionID uuid.UUID      `json:"config_version_id"`
	Version         int            `json:"version"`
	Status          string         `json:"status"`
	Title           *string        `json:"title,omitempty"`
	Payload         map[string]any `json:"payload"`
	EffectiveFrom   *time.Time     `json:"effective_from,omitempty"`
	AuthorID        *uuid.UUID     `json:"author_id,omitempty"`
	ReviewerID      *uuid.UUID     `json:"reviewer_id,omitempty"`
	ReviewNote      *string        `json:"review_note,omitempty"`
	ChangeNote      *string        `json:"change_note,omitempty"`
	BasedOnVersion  *int           `json:"based_on_version,omitempty"`
	IsCurrent       bool           `json:"is_current"`
	PublishedAt     *time.Time     `json:"published_at,omitempty"`
	UpdatedAt       time.Time      `json:"updated_at"`
	ETag            string         `json:"-"`
}

// VersionView builds the view of a version (basedOn: the number of its source version, when known).
func VersionView(t gameconfig.Type, it port.ConfigItem, v port.ConfigVersion, basedOn *int) ConfigVersionView {
	var doc map[string]any
	_ = json.Unmarshal(v.Payload, &doc)
	if doc == nil {
		doc = map[string]any{}
	}
	out := ConfigVersionView{ConfigType: t.Name, ConfigID: v.ConfigID, ConfigVersionID: v.ID, Version: v.No, Status: v.Status, Payload: doc,
		EffectiveFrom: v.EffectiveFrom, AuthorID: v.AuthorID, ReviewerID: v.ReviewerID, ReviewNote: v.ReviewNote, ChangeNote: v.ChangeNote,
		BasedOnVersion: basedOn, IsCurrent: it.Current != nil && *it.Current == v.ID, PublishedAt: v.PublishedAt, UpdatedAt: v.UpdatedAt,
		ETag: ETagOf(v.Revision)}
	if v.Title != "" {
		title := v.Title
		out.Title = &title
	}
	return out
}

// ReleaseView is ContentRelease of the contract (item_type = config_type, item_id = config_id).
type ReleaseView struct {
	ReleaseID     uuid.UUID `json:"release_id"`
	ItemType      string    `json:"item_type"`
	ItemID        uuid.UUID `json:"item_id"`
	Action        string    `json:"action"`
	FromVersion   *int      `json:"from_version,omitempty"`
	ToVersion     *int      `json:"to_version,omitempty"`
	Reason        *string   `json:"reason,omitempty"`
	Status        string    `json:"status"`
	ReleasedBy    uuid.UUID `json:"released_by"`
	ReleasedAt    time.Time `json:"released_at"`
	EffectiveFrom time.Time `json:"effective_from"`
}

// ReleaseOf builds the view of a release row; nos maps version ids to their numbers.
func ReleaseOf(configType string, r port.ConfigRelease, nos map[uuid.UUID]int) ReleaseView {
	no := func(id *uuid.UUID) *int {
		if id == nil {
			return nil
		}
		if n, ok := nos[*id]; ok {
			return &n
		}
		return nil
	}
	return ReleaseView{ReleaseID: r.ID, ItemType: configType, ItemID: r.ConfigID, Action: r.Action, FromVersion: no(r.From), ToVersion: no(r.To),
		Reason: r.Reason, Status: "applied", ReleasedBy: r.By, ReleasedAt: r.At, EffectiveFrom: r.EffectiveFrom}
}
