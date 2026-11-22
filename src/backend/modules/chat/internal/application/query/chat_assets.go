package query

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/service"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/chatasset"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// ChatAssets are the Game Studio reads of the sticker/GIF library (platform role with game.chat_assets.write, R20).
type ChatAssets struct {
	Read   service.ReadRunner
	Reader port.AssetReader
}

func assetRead(ctx context.Context, typ string) (chatasset.Type, authz.Principal, error) {
	t := chatasset.Type(typ)
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || !p.Valid() {
		return t, p, problem.New(problem.Unauthenticated, "")
	}
	if err := authz.Require(ctx, p, authz.PermGameChatAssetsWrite, authz.Resource{}); err != nil {
		return t, p, err
	}
	if p.UserType != authz.UserTypeStaff || !p.HasPlatformRole() {
		return t, p, problem.New(problem.PermissionDenied, "")
	}
	if !t.Valid() {
		return t, p, problem.New(problem.NotFound, "")
	}
	return t, p, nil
}

func (q ChatAssets) platform(ctx context.Context, p authz.Principal, fn func(ctx context.Context) error) error {
	d := p.PrincipalData
	d.TenantID = nil
	sc, err := authz.PlatformScope(authz.NewPrincipal(d))
	if err != nil {
		return problem.New(problem.PermissionDenied, "").Wrap(err)
	}
	return q.Read.InTenantRead(ctx, sc, fn)
}

// List is listGameConfigs / listGameConfigVersions of sticker-packs and gifs (newest first).
func (q ChatAssets) List(ctx context.Context, typ string, id *uuid.UUID, status string, limit int) (out []port.AssetVersion, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.ListChatAssets")
	defer func() { done(err) }()
	t, p, err := assetRead(ctx, typ)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > MaxPageLimit {
		limit = DefaultPageLimit
	}
	err = q.platform(ctx, p, func(ctx context.Context) error {
		var err error
		out, err = q.Reader.ListVersions(ctx, t, id, status, limit)
		return err
	})
	return out, err
}

// Get is getGameConfigVersion of sticker-packs and gifs.
func (q ChatAssets) Get(ctx context.Context, typ string, id uuid.UUID, version int) (v *port.AssetVersion, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.GetChatAsset")
	defer func() { done(err) }()
	t, p, err := assetRead(ctx, typ)
	if err != nil {
		return nil, err
	}
	err = q.platform(ctx, p, func(ctx context.Context) error {
		var err error
		v, err = q.Reader.GetVersion(ctx, t, id, version)
		return err
	})
	if err == nil && v == nil {
		err = problem.New(problem.NotFound, "")
	}
	return v, err
}
