package query

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/service"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/chatasset"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// ChatAssets are the Game Studio reads of the sticker/GIF library (platform role with game.chat_assets.write, R20).
type ChatAssets struct {
	Read    service.ReadRunner
	Reader  port.AssetReader
	Cursors *page.Codec
	// Storage signs the preview URLs of unpublished files (nil: none).
	Storage port.AssetStorage
}

// PreviewTTL is media_presign_ttl (15 minutes): validity of a preview URL of the library.
const PreviewTTL = 15 * time.Minute

// assetRead authorizes a Game Studio read of the library (platform staff only, R20).
func assetRead(ctx context.Context, typ string) (chatasset.Type, authz.Principal, error) {
	t := chatasset.Type(typ)
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || !p.Valid() {
		return t, p, problem.New(problem.Unauthenticated, "")
	}
	// Reads follow action.game_ops.chat_assets.view: game.chat_assets.write, game.content.review or game.content.publish
	// (the reviewer and Live-ops open the version they approve/publish and need its ETag for If-Match — same rule as
	// gameadmin.RequireRead of module game); a denial is audited for game.chat_assets.write.
	if !p.Has(authz.PermGameChatAssetsWrite) && !p.Has(authz.PermGameContentReview) && !p.Has(authz.PermGameContentPublish) {
		if err := authz.Require(ctx, p, authz.PermGameChatAssetsWrite, authz.Resource{}); err != nil {
			return t, p, err
		}
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

// ListInput is the query of listGameConfigs for sticker-packs/gifs (F082-AC-01, AC-05).
type ListInput struct {
	Status   string
	AuthorID *uuid.UUID
	Tag      string
	Cursor   string
	Limit    int
}

// AssetPage is one page of the library.
type AssetPage struct {
	Rows       []port.AssetVersion
	NextCursor *string
	Limit      int
}

var validStatus = map[string]bool{"": true, "draft": true, "in_review": true, "approved": true, "rejected": true, "published": true, "archived": true}

// List is listGameConfigs of sticker-packs and gifs: one row per item (its newest version, or its newest version of
// status), filtered by author and GIF tag, keyset paged in the query (F082 Clarifications 2026-11-26).
func (q ChatAssets) List(ctx context.Context, typ string, in ListInput) (out AssetPage, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.ListChatAssets")
	defer func() { done(err) }()
	t, p, err := assetRead(ctx, typ)
	if err != nil {
		return out, err
	}
	switch {
	case !validStatus[in.Status]:
		return out, problem.Invalid("status", problem.SubInvalid, "unknown status")
	case in.Tag != "" && t != chatasset.TypeGifs:
		return out, problem.Invalid("tag", problem.SubInvalid, "filter of config_type gifs only")
	case in.Tag != "" && !chatasset.ValidTag(in.Tag):
		return out, problem.Invalid("tag", problem.SubInvalid, "^[a-z0-9 _-]{1,64}$")
	}
	limit := in.Limit
	if limit <= 0 || limit > MaxPageLimit {
		limit = DefaultPageLimit
	}
	f := port.AssetFilter{Type: t, Status: in.Status, AuthorID: in.AuthorID, Tag: in.Tag, Limit: limit + 1}
	if in.Cursor != "" {
		if q.Cursors == nil {
			return out, problem.New(problem.BadRequest, "invalid cursor")
		}
		c, err := q.Cursors.Decode(in.Cursor)
		if err != nil || c == nil {
			return out, problem.New(problem.BadRequest, "invalid cursor")
		}
		at, err := time.Parse(time.RFC3339Nano, c.Key)
		if err != nil {
			return out, problem.New(problem.BadRequest, "invalid cursor")
		}
		id := c.ID
		f.AfterAt, f.AfterID = &at, &id
	}
	err = q.platform(ctx, p, func(ctx context.Context) error {
		rows, err := q.Reader.ListItems(ctx, f)
		if err != nil {
			return err
		}
		out = AssetPage{Rows: rows, Limit: limit}
		if len(rows) > limit {
			out.Rows = rows[:limit]
			last := out.Rows[limit-1]
			if q.Cursors != nil {
				next := q.Cursors.Encode(page.Cursor{Key: last.UpdatedAt.UTC().Format(time.RFC3339Nano), ID: last.ConfigID})
				out.NextCursor = &next
			}
		}
		return nil
	})
	return out, err
}

// AssetVersionPage is listGameConfigVersions of one pack/GIF.
type AssetVersionPage struct {
	Rows       []port.AssetVersion
	Current    *int
	NextCursor *string
	Limit      int
}

// Versions is listGameConfigVersions of sticker-packs and gifs, newest first, keyset on the version number.
func (q ChatAssets) Versions(ctx context.Context, typ string, id uuid.UUID, cursor string, limit int) (out AssetVersionPage, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.ListChatAssetVersions")
	defer func() { done(err) }()
	t, p, err := assetRead(ctx, typ)
	if err != nil {
		return out, err
	}
	if limit <= 0 || limit > MaxPageLimit {
		limit = DefaultPageLimit
	}
	var before *int
	if cursor != "" {
		c, err := q.decode(cursor)
		if err != nil {
			return out, err
		}
		n, err := strconv.Atoi(c.Key)
		if err != nil {
			return out, problem.New(problem.BadRequest, "invalid cursor")
		}
		before = &n
	}
	err = q.platform(ctx, p, func(ctx context.Context) error {
		rows, found, err := q.Reader.ListItemVersions(ctx, t, id, before, limit+1)
		if err != nil {
			return err
		}
		if !found {
			return problem.New(problem.NotFound, "")
		}
		out = AssetVersionPage{Rows: rows, Limit: limit}
		if len(rows) > limit {
			out.Rows = rows[:limit]
			next := q.Cursors.Encode(page.Cursor{Key: strconv.Itoa(out.Rows[limit-1].Version), ID: id})
			out.NextCursor = &next
		}
		out.Current, err = q.Reader.CurrentVersion(ctx, t, id)
		return err
	})
	return out, err
}

func (q ChatAssets) decode(cursor string) (*page.Cursor, error) {
	if q.Cursors == nil {
		return nil, problem.New(problem.BadRequest, "invalid cursor")
	}
	c, err := q.Cursors.Decode(cursor)
	if err != nil || c == nil {
		return nil, problem.New(problem.BadRequest, "invalid cursor")
	}
	return c, nil
}

// AssetReleasePage is listGameConfigReleases of one pack/GIF.
type AssetReleasePage struct {
	Rows       []port.AssetRelease
	NextCursor *string
	Limit      int
}

// Releases is listGameConfigReleases of sticker-packs and gifs (publish, rollback, unpublish), newest first.
func (q ChatAssets) Releases(ctx context.Context, typ string, id uuid.UUID, cursor string, limit int) (out AssetReleasePage, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.ListChatAssetReleases")
	defer func() { done(err) }()
	t, p, err := assetRead(ctx, typ)
	if err != nil {
		return out, err
	}
	if limit <= 0 || limit > MaxPageLimit {
		limit = DefaultPageLimit
	}
	var beforeAt *time.Time
	var beforeID *uuid.UUID
	if cursor != "" {
		c, err := q.decode(cursor)
		if err != nil {
			return out, err
		}
		at, err := time.Parse(time.RFC3339Nano, c.Key)
		if err != nil {
			return out, problem.New(problem.BadRequest, "invalid cursor")
		}
		cid := c.ID
		beforeAt, beforeID = &at, &cid
	}
	err = q.platform(ctx, p, func(ctx context.Context) error {
		if _, found, err := q.Reader.ListItemVersions(ctx, t, id, nil, 1); err != nil {
			return err
		} else if !found {
			return problem.New(problem.NotFound, "")
		}
		rows, err := q.Reader.Releases(ctx, t, id, beforeAt, beforeID, limit+1)
		if err != nil {
			return err
		}
		out = AssetReleasePage{Rows: rows, Limit: limit}
		if len(rows) > limit {
			out.Rows = rows[:limit]
			last := out.Rows[limit-1]
			next := q.Cursors.Encode(page.Cursor{Key: last.At.UTC().Format(time.RFC3339Nano), ID: last.ReleaseID})
			out.NextCursor = &next
		}
		return nil
	})
	return out, err
}

// Diff is diffGameConfigVersions of sticker-packs and gifs: the changes from version `with` to version `no`.
func (q ChatAssets) Diff(ctx context.Context, typ string, id uuid.UUID, no, with int) (changes []chatasset.Change, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.DiffChatAssetVersions")
	defer func() { done(err) }()
	t, p, err := assetRead(ctx, typ)
	if err != nil {
		return nil, err
	}
	if with < 1 {
		return nil, problem.Invalid("with", problem.SubInvalid, "a version number")
	}
	err = q.platform(ctx, p, func(ctx context.Context) error {
		a, err := q.Reader.GetVersion(ctx, t, id, with)
		if err != nil {
			return err
		}
		b, err := q.Reader.GetVersion(ctx, t, id, no)
		if err != nil {
			return err
		}
		if a == nil || b == nil {
			return problem.New(problem.NotFound, "")
		}
		var x, y map[string]any
		_ = json.Unmarshal(a.Payload, &x)
		_ = json.Unmarshal(b.Payload, &y)
		changes, _ = chatasset.Diff(x, y)
		if changes == nil {
			changes = []chatasset.Change{}
		}
		return nil
	})
	return changes, err
}

// PreviewURLs signs a short GET (media_presign_ttl) for every file of a payload, so Game Studio shows an unpublished
// pack/GIF from the private bucket (F082-AC-07; never through the public CDN before publish — R31): stickers[].preview_url,
// GIF preview_url (static) and url (animated). Read-only fields, never stored.
func (q ChatAssets) PreviewURLs(ctx context.Context, t chatasset.Type, payload map[string]any) {
	if q.Storage == nil || payload == nil {
		return
	}
	sign := func(key any) (string, bool) {
		k, ok := key.(string)
		if !ok || !chatasset.ValidKey(k) {
			return "", false
		}
		u, err := q.Storage.PresignGet(ctx, k, PreviewTTL)
		return u, err == nil
	}
	if t == chatasset.TypeStickerPacks {
		stickers, _ := payload["stickers"].([]any)
		for _, s := range stickers {
			if m, ok := s.(map[string]any); ok {
				if u, ok := sign(m["asset_key"]); ok {
					m["preview_url"] = u
				}
			}
		}
		return
	}
	if u, ok := sign(payload["preview_key"]); ok {
		payload["preview_url"] = u
	}
	if u, ok := sign(payload["asset_key"]); ok {
		payload["url"] = u
	}
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
