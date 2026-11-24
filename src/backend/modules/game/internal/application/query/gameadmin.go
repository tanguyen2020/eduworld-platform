package query

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/gameadmin"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/gameconfig"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// MediaURLs builds the CDN URL of a public object ("" when no CDN base URL is configured).
type MediaURLs interface {
	URL(bucket, key string) string
}

// GameAdmin are the read use cases of the Game Operations Console (F078-AC-09, AC-16, AC-17, AC-18, AC-23): platform
// content read in a READ ONLY transaction under the PlatformScope of the principal; lists page in the query.
type GameAdmin struct {
	Read    read.Session
	Data    read.GameConfigReadModel
	Cursors *page.Codec
	URLs    MediaURLs
	Now     func() time.Time
}

func (h GameAdmin) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

func (h GameAdmin) renderURL(sha *string, kind gameconfig.MediaKind) *string {
	if sha == nil || h.URLs == nil {
		return nil
	}
	u := h.URLs.URL("ew-media-public", gameconfig.RenderKey(*sha, kind))
	if u == "" {
		return nil
	}
	return &u
}

func (h GameAdmin) view(t gameconfig.Type, r read.ConfigRow) gameadmin.ConfigVersionView {
	var payload map[string]any
	_ = json.Unmarshal(r.Payload, &payload)
	if payload == nil {
		payload = map[string]any{}
	}
	return gameadmin.ConfigVersionView{ConfigType: t.Name, ConfigID: r.ConfigID, ConfigVersionID: r.VersionID, Version: r.VersionNo, Status: r.Status,
		Title: r.Title, Payload: payload, AuthorID: r.AuthorID, ReviewerID: r.ReviewerID, ReviewNote: r.ReviewNote, ChangeNote: r.ChangeNote,
		BasedOnVersion: r.BasedOnNo, IsCurrent: r.IsCurrent, PublishedAt: r.PublishedAt, UpdatedAt: r.UpdatedAt.UTC(),
		ThumbnailURL: h.renderURL(r.ThumbnailSHA, gameconfig.MediaRenderThumbnail), TurntableURL: h.renderURL(r.TurntableSHA, gameconfig.MediaRenderTurntable),
		ETag: gameadmin.ETagOf(r.Revision)}
}

// reader authorizes a read of configType and opens the read transaction under the PlatformScope.
func (h GameAdmin) reader(ctx context.Context, configType string, fn func(ctx context.Context, t gameconfig.Type) error) error {
	t, err := gameadmin.Type(configType)
	if err != nil {
		return err
	}
	p, sc, err := gameadmin.Principal(ctx)
	if err != nil {
		return err
	}
	if err := gameadmin.RequireRead(ctx, p, configType); err != nil {
		return err
	}
	return h.Read.Read(tenancy.WithScope(ctx, sc), func(ctx context.Context) error { return fn(ctx, t) })
}

// ConfigPage is the listGameConfigs_game response.
type ConfigPage struct {
	Data []gameadmin.ConfigVersionView `json:"data"`
	Page page.Info                     `json:"page"`
}

// List is listGameConfigs_game: one entry per item (its newest version, or its newest version of status), keyset by
// config_id in the query (LIMIT n+1).
func (h GameAdmin) List(ctx context.Context, configType, status, cursor string, limit int) (out ConfigPage, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.ListGameConfigs")
	defer func() { done(err) }()
	if status != "" && !gameconfig.ValidStatus(status) {
		return out, problem.Invalid("status", problem.SubInvalid, "unknown status")
	}
	n := page.Limit(limit)
	cur, err := h.Cursors.Decode(cursor)
	if err != nil {
		return out, problem.New(problem.BadRequest, "invalid cursor")
	}
	err = h.reader(ctx, configType, func(ctx context.Context, t gameconfig.Type) error {
		f := read.ConfigFilter{ItemType: t.ItemType, Status: status, Limit: n + 1}
		if cur != nil {
			id := cur.ID
			f.After = &id
		}
		rows, err := h.Data.List(ctx, f)
		if err != nil {
			return err
		}
		out = ConfigPage{Data: []gameadmin.ConfigVersionView{}, Page: page.Info{Limit: n}}
		if len(rows) > n {
			rows = rows[:n]
			next := h.Cursors.Encode(page.Cursor{Key: "c", ID: rows[n-1].ConfigID})
			out.Page.HasMore, out.Page.NextCursor = true, &next
		}
		for _, r := range rows {
			out.Data = append(out.Data, h.view(t, r))
		}
		return nil
	})
	return out, err
}

// Get is getGameConfigVersion_game.
func (h GameAdmin) Get(ctx context.Context, configType string, configID uuid.UUID, no int) (out gameadmin.ConfigVersionView, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.GetGameConfigVersion")
	defer func() { done(err) }()
	err = h.reader(ctx, configType, func(ctx context.Context, t gameconfig.Type) error {
		r, err := h.Data.Version(ctx, t.ItemType, configID, no)
		if err != nil {
			return err
		}
		if r == nil {
			return problem.New(problem.NotFound, "")
		}
		out = h.view(t, *r)
		return nil
	})
	return out, err
}

// VersionSummary is ContentVersionSummary.
type VersionSummary struct {
	Version     int        `json:"version"`
	VersionID   uuid.UUID  `json:"content_version_id"`
	Status      string     `json:"status"`
	IsCurrent   bool       `json:"is_current"`
	ChangeNote  *string    `json:"change_note,omitempty"`
	AuthorID    *uuid.UUID `json:"author_id,omitempty"`
	ReviewerID  *uuid.UUID `json:"reviewer_id,omitempty"`
	PublishedBy *uuid.UUID `json:"published_by,omitempty"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
	PublishAt   *time.Time `json:"publish_at,omitempty"`
}

// VersionHistory is ContentVersionHistory.
type VersionHistory struct {
	ItemID         uuid.UUID        `json:"item_id"`
	CurrentVersion *int             `json:"current_version,omitempty"`
	Data           []VersionSummary `json:"data"`
	Page           page.Info        `json:"page"`
}

// Versions is listGameConfigVersions_game: newest first, keyset on the version number.
func (h GameAdmin) Versions(ctx context.Context, configType string, configID uuid.UUID, cursor string, limit int) (out VersionHistory, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.ListGameConfigVersions")
	defer func() { done(err) }()
	n := page.Limit(limit)
	cur, err := h.Cursors.Decode(cursor)
	if err != nil {
		return out, problem.New(problem.BadRequest, "invalid cursor")
	}
	var before *int
	if cur != nil {
		b, err := strconv.Atoi(cur.Key)
		if err != nil {
			return out, problem.New(problem.BadRequest, "invalid cursor")
		}
		before = &b
	}
	err = h.reader(ctx, configType, func(ctx context.Context, t gameconfig.Type) error {
		rows, current, found, err := h.Data.Versions(ctx, t.ItemType, configID, before, n+1)
		if err != nil {
			return err
		}
		if !found {
			return problem.New(problem.NotFound, "")
		}
		out = VersionHistory{ItemID: configID, CurrentVersion: current, Data: []VersionSummary{}, Page: page.Info{Limit: n}}
		if len(rows) > n {
			rows = rows[:n]
			next := h.Cursors.Encode(page.Cursor{Key: strconv.Itoa(rows[n-1].VersionNo), ID: rows[n-1].VersionID})
			out.Page.HasMore, out.Page.NextCursor = true, &next
		}
		for _, r := range rows {
			out.Data = append(out.Data, VersionSummary{Version: r.VersionNo, VersionID: r.VersionID, Status: r.Status, IsCurrent: r.IsCurrent,
				ChangeNote: r.ChangeNote, AuthorID: r.AuthorID, ReviewerID: r.ReviewerID, PublishedBy: r.PublishedBy, PublishedAt: r.PublishedAt,
				PublishAt: r.ScheduledAt})
		}
		return nil
	})
	return out, err
}

// VersionDiff is ContentVersionDiff.
type VersionDiff struct {
	From    int                 `json:"from_version"`
	To      int                 `json:"to_version"`
	Changes []gameconfig.Change `json:"changes"`
}

// Diff is diffGameConfigVersions_game: the changes from version `with` to version `no` by field path (F078-AC-09).
func (h GameAdmin) Diff(ctx context.Context, configType string, configID uuid.UUID, no, with int) (out VersionDiff, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.DiffGameConfigVersions")
	defer func() { done(err) }()
	err = h.reader(ctx, configType, func(ctx context.Context, t gameconfig.Type) error {
		a, err := h.Data.Version(ctx, t.ItemType, configID, with)
		if err != nil {
			return err
		}
		b, err := h.Data.Version(ctx, t.ItemType, configID, no)
		if err != nil {
			return err
		}
		if a == nil || b == nil {
			return problem.New(problem.NotFound, "")
		}
		var da, db map[string]any
		_ = json.Unmarshal(a.Payload, &da)
		_ = json.Unmarshal(b.Payload, &db)
		changes, _ := gameconfig.Diff(t, da, db)
		if changes == nil {
			changes = []gameconfig.Change{}
		}
		out = VersionDiff{From: with, To: no, Changes: changes}
		return nil
	})
	return out, err
}

// ReleasePage is the listGameConfigReleases_game response.
type ReleasePage struct {
	Data []gameadmin.ReleaseView `json:"data"`
	Page page.Info               `json:"page"`
}

// Releases is listGameConfigReleases_game: the release timeline (publish, rollback, unpublish, scheduled, cancelled,
// failed) newest first, keyset on (at, id) (F078-AC-16).
func (h GameAdmin) Releases(ctx context.Context, configType string, configID uuid.UUID, cursor string, limit int) (out ReleasePage, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.ListGameConfigReleases")
	defer func() { done(err) }()
	n := page.Limit(limit)
	cur, err := h.Cursors.Decode(cursor)
	if err != nil {
		return out, problem.New(problem.BadRequest, "invalid cursor")
	}
	var beforeAt *time.Time
	var beforeID uuid.UUID
	if cur != nil {
		at, err := time.Parse(time.RFC3339Nano, cur.Key)
		if err != nil {
			return out, problem.New(problem.BadRequest, "invalid cursor")
		}
		beforeAt, beforeID = &at, cur.ID
	}
	err = h.reader(ctx, configType, func(ctx context.Context, t gameconfig.Type) error {
		rows, found, err := h.Data.Releases(ctx, t.ItemType, configID, beforeAt, beforeID, n+1)
		if err != nil {
			return err
		}
		if !found {
			return problem.New(problem.NotFound, "")
		}
		out = ReleasePage{Data: []gameadmin.ReleaseView{}, Page: page.Info{Limit: n}}
		if len(rows) > n {
			rows = rows[:n]
			last := rows[n-1]
			next := h.Cursors.Encode(page.Cursor{Key: last.At.UTC().Format(time.RFC3339Nano), ID: last.ID})
			out.Page.HasMore, out.Page.NextCursor = true, &next
		}
		for _, r := range rows {
			out.Data = append(out.Data, gameadmin.ReleaseView{ReleaseID: r.ID, ItemType: t.ItemType, ItemID: configID, Action: r.Action,
				FromVersion: r.FromNo, ToVersion: r.ToNo, Reason: r.Reason, ScheduledFor: r.ScheduledFor, Status: r.Status,
				FailureCode: r.FailureCode, ReleasedBy: r.By, ReleasedAt: r.At.UTC()})
		}
		return nil
	})
	return out, err
}

// PreviewSnapshot is getGamePreviewSnapshot (F078-AC-17): the token reads only its version, for its creator, before it
// expires; anything else is 403 (audited authz.denied). Nothing is written: no game session, attempt, XP or event.
type PreviewSnapshot struct {
	ConfigType string         `json:"config_type"`
	ConfigID   uuid.UUID      `json:"config_id"`
	Version    int            `json:"version"`
	Status     string         `json:"status"`
	Title      *string        `json:"title,omitempty"`
	Payload    map[string]any `json:"payload"`
	ExpiresAt  time.Time      `json:"expires_at"`
}

// Preview reads the version a preview token grants.
func (h GameAdmin) Preview(ctx context.Context, token, configType string, configID uuid.UUID, no int) (out PreviewSnapshot, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.GetGamePreviewSnapshot")
	defer func() { done(err) }()
	err = h.reader(ctx, configType, func(ctx context.Context, t gameconfig.Type) error {
		p, _ := authz.PrincipalFrom(ctx)
		s, err := h.Data.Preview(ctx, gameconfig.HashToken(token))
		if err != nil {
			return err
		}
		if s == nil || !(gameconfig.PreviewSession{Type: s.ConfigType, ConfigID: s.ConfigID, Version: s.VersionNo, CreatedBy: s.CreatedBy,
			ExpiresAt: s.ExpiresAt}).Allows(p.UserID, configType, configID, no, h.now()) {
			authz.RecordDenied(ctx, "preview_token_scope", map[string]any{"user_id": p.UserID.String()})
			return problem.New(problem.PermissionDenied, "the preview token does not grant this version")
		}
		r, err := h.Data.Version(ctx, t.ItemType, configID, no)
		if err != nil {
			return err
		}
		if r == nil || r.VersionID != s.VersionID {
			return problem.New(problem.NotFound, "")
		}
		var payload map[string]any
		_ = json.Unmarshal(r.Payload, &payload)
		out = PreviewSnapshot{ConfigType: configType, ConfigID: configID, Version: no, Status: r.Status, Title: r.Title, Payload: payload,
			ExpiresAt: s.ExpiresAt.UTC()}
		return nil
	})
	return out, err
}

// MediaView is MediaAsset of the contract.
type MediaView struct {
	AssetID    uuid.UUID        `json:"asset_id"`
	OwnerScope string           `json:"owner_scope"`
	Kind       string           `json:"kind"`
	Filename   string           `json:"original_filename"`
	MIME       string           `json:"mime_type"`
	Size       int64            `json:"size_bytes"`
	Width      *int             `json:"width,omitempty"`
	Height     *int             `json:"height,omitempty"`
	SHA256     string           `json:"sha256"`
	Status     string           `json:"status"`
	Reject     *string          `json:"reject_reason,omitempty"`
	AltText    *string          `json:"alt_text,omitempty"`
	License    *string          `json:"license,omitempty"`
	Bucket     string           `json:"bucket,omitempty"`
	Variants   []map[string]any `json:"variants"`
	UsedIn     []map[string]any `json:"used_in"`
	CreatedAt  time.Time        `json:"created_at"`
}

// MediaAsset is getGameMediaAsset (F078-AC-18): status and variants (CDN URLs once ready).
func (h GameAdmin) MediaAsset(ctx context.Context, id uuid.UUID) (out MediaView, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.GetGameMediaAsset")
	defer func() { done(err) }()
	p, sc, err := gameadmin.Principal(ctx)
	if err != nil {
		return out, err
	}
	if err := gameadmin.RequireAnyRead(ctx, p); err != nil {
		return out, err
	}
	err = h.Read.Read(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		a, err := h.Data.MediaAsset(ctx, id)
		if err != nil {
			return err
		}
		if a == nil {
			return problem.New(problem.NotFound, "")
		}
		out = MediaView{AssetID: a.ID, OwnerScope: "platform", Kind: a.Kind, Filename: a.Filename, MIME: a.MIME, Size: a.Size, Width: a.Width,
			Height: a.Height, SHA256: a.SHA256, Status: a.Status, Reject: a.Reject, AltText: a.AltText, License: a.License,
			Variants: []map[string]any{}, UsedIn: []map[string]any{}, CreatedAt: a.CreatedAt.UTC()}
		if a.Status == string(gameconfig.MediaReady) {
			out.Bucket = a.Bucket
			for _, v := range a.Variants {
				key, _ := v["key"].(string)
				if h.URLs == nil || h.URLs.URL(a.Bucket, key) == "" {
					continue // no CDN base URL configured: no public URL to give
				}
				out.Variants = append(out.Variants, map[string]any{"variant": v["variant"], "key": key, "mime_type": v["mime_type"],
					"size_bytes": v["bytes"], "url": h.URLs.URL(a.Bucket, key)})
			}
		}
		for _, u := range a.UsedIn {
			out.UsedIn = append(out.UsedIn, map[string]any{"item_type": u.ItemType, "item_id": u.ConfigID, "version": u.Version})
		}
		return nil
	})
	return out, err
}

// AssignedPage is one page of listAssignedGameActivities (facade, module learning).
type AssignedPage struct {
	Data       []read.AssignedActivity
	NextCursor *string
	HasMore    bool
	Limit      int
}

// Assigned is listAssignedGameActivities (F078-AC-23): the published quests and mini-game configs assigned to the tenant
// (explicit tenant condition), keyset on (content_type, content_id). Module learning calls it under a platform scope
// to rebuild its catalog projection.
func (h GameAdmin) Assigned(ctx context.Context, tenant uuid.UUID, cursor string, limit int) (out AssignedPage, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.ListAssignedGameActivities")
	defer func() { done(err) }()
	if tenant == uuid.Nil {
		return out, problem.Invalid("tenant_id", problem.SubRequired, "a tenant is required")
	}
	n := page.Limit(limit)
	cur, err := h.Cursors.Decode(cursor)
	if err != nil {
		return out, problem.New(problem.BadRequest, "invalid cursor")
	}
	sc, err := authz.SystemPlatformScope("game-catalog")
	if err != nil {
		return out, err
	}
	err = h.Read.Read(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		afterType, afterID := "", (*uuid.UUID)(nil)
		if cur != nil {
			id := cur.ID
			afterType, afterID = cur.Key, &id
		}
		rows, err := h.Data.Assigned(ctx, tenant, afterType, afterID, n+1)
		if err != nil {
			return err
		}
		out = AssignedPage{Data: rows, Limit: n}
		if len(rows) > n {
			out.Data = rows[:n]
			last := out.Data[n-1]
			next := h.Cursors.Encode(page.Cursor{Key: last.ItemType, ID: last.ConfigID})
			out.HasMore, out.NextCursor = true, &next
		}
		return nil
	})
	return out, err
}

// ThumbnailURL is the CDN URL of a ready render thumbnail ("" when none).
func (h GameAdmin) ThumbnailURL(sha *string) string {
	if u := h.renderURL(sha, gameconfig.MediaRenderThumbnail); u != nil {
		return *u
	}
	return ""
}
