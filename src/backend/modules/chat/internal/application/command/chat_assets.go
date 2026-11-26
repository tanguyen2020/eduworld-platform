package command

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/chatasset"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Audit actions and events of the Game Studio library (ids only).
const (
	AuditChatAssetCreated     = "chat_asset.created"
	AuditChatAssetUpdated     = "chat_asset.updated"
	AuditChatAssetTransition  = "chat_asset.transitioned"
	AuditChatAssetPublished   = "chat_asset.published"
	AuditChatAssetPresigned   = "chat_asset.upload_presigned"
	AuditChatAssetRolledBack  = "chat_asset.rolled_back"
	AuditChatAssetUnpublished = "chat_asset.unpublished"
	EventContentRolledBack    = "content.rolled_back"
	EventContentUnpublished   = "content.unpublished"
	// MaxReasonLength bounds the reason of a rollback/unpublish (RollbackRequest/ReasonRequest maxLength).
	MaxReasonLength           = 500
	EventContentPublished     = "content.published"
	PresignTTL                = 15 * time.Minute
	assetPresignMaxFileLength = 200
)

// AssetMetrics count refused uploads (reason label only).
type AssetMetrics interface {
	AssetRejected(reason string)
}

// ChatAssets are the Game Studio use cases of the sticker/GIF library (F118-AC-17; /game-admin/{sticker-packs|gifs},
// internal gateway, platform role with game.chat_assets.write — AGENTS R20): create, edit a draft, workflow, publish
// (versioned, immutable once published — R31), presigned upload; every action audited. A tenant role is refused with
// 403 PERMISSION_DENIED.
type ChatAssets struct {
	UoW       write.UnitOfWork
	Repo      write.AssetRepository
	Storage   port.AssetStorage
	Inspector port.AssetInspector
	Events    write.EventPublisher
	Audit     write.Auditor
	Metrics   AssetMetrics // nil: none
	Limits    chatasset.Limits
	// RequiredLanguages is game_text_required_languages (vi, en): the languages a sticker pack needs before submit_review
	// (F082-AC-02, NFR-I18N-002); nil = vi and en.
	RequiredLanguages []string
	Now               func() time.Time
}

func (h ChatAssets) required() []string {
	if len(h.RequiredLanguages) == 0 {
		return []string{"vi", "en"}
	}
	return h.RequiredLanguages
}

func (h ChatAssets) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

// assetWrite authorizes a Game Studio action and returns the actor and the platform scope.
func assetWrite(ctx context.Context) (authz.Principal, tenancy.Scope, error) {
	return platformWrite(ctx, authz.PermGameChatAssetsWrite)
}

// ifMatch compares the If-Match header of a version write with the revision of the locked row (API_DESIGN §7): missing
// → 428 PRECONDITION_REQUIRED, another revision → 412 VERSION_MISMATCH (F082 Clarifications 2026-11-26). The httpx contract
// validator already answers 428 for a missing header; the check is repeated here so the use case never trusts its caller.
func ifMatch(header string, cur *port.AssetVersion) error {
	if strings.TrimSpace(header) == "" {
		return problem.New(problem.PreconditionRequired, "")
	}
	if !cur.MatchesETag(header) {
		return problem.New(problem.VersionMismatch, "")
	}
	return nil
}

func assetType(t string) (chatasset.Type, error) {
	ct := chatasset.Type(t)
	if !ct.Valid() {
		return "", problem.New(problem.NotFound, "")
	}
	return ct, nil
}

func fieldProblem(err error) error {
	var fe chatasset.FieldError
	if errors.As(err, &fe) {
		return problem.Invalid(fe.Field, problem.SubInvalid, fe.Reason)
	}
	var fl chatasset.FileError
	if errors.As(err, &fl) {
		return problem.Invalid("payload."+fl.Field, problem.SubInvalid, "file refused: "+fl.Reason)
	}
	return err
}

// normalize decodes and validates a payload of t; new stickers get their id here (server side).
func normalize(t chatasset.Type, raw json.RawMessage) ([]byte, *chatasset.Pack, error) {
	switch t {
	case chatasset.TypeStickerPacks:
		var p chatasset.Pack
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, nil, problem.Invalid("payload", problem.SubInvalid, "sticker pack payload")
		}
		if err := p.Check(); err != nil {
			return nil, nil, fieldProblem(err)
		}
		for i := range p.Stickers {
			if p.Stickers[i].StickerID == uuid.Nil {
				p.Stickers[i].StickerID = ids.New()
			}
		}
		b, err := json.Marshal(p)
		return b, &p, err
	default:
		var g chatasset.Gif
		if err := json.Unmarshal(raw, &g); err != nil {
			return nil, nil, problem.Invalid("payload", problem.SubInvalid, "gif payload")
		}
		if err := g.Check(); err != nil {
			return nil, nil, fieldProblem(err)
		}
		b, err := json.Marshal(g)
		return b, nil, err
	}
}

func (h ChatAssets) record(ctx context.Context, action string, t chatasset.Type, id uuid.UUID, meta map[string]any) error {
	if meta == nil {
		meta = map[string]any{}
	}
	meta["config_type"] = string(t)
	return h.Audit.Record(ctx, audit.Event{Action: action, TargetType: t.ItemType(), TargetID: id.String(), Result: audit.ResultSuccess,
		Metadata: meta})
}

// Create is createGameConfig for sticker-packs/gifs: a new item at version 1 (draft).
func (h ChatAssets) Create(ctx context.Context, typ, title string, payload json.RawMessage) (v port.AssetVersion, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.CreateChatAsset")
	defer func() { done(err) }()
	t, err := assetType(typ)
	if err != nil {
		return v, err
	}
	p, sc, err := assetWrite(ctx)
	if err != nil {
		return v, err
	}
	b, pack, err := normalize(t, payload)
	if err != nil {
		return v, err
	}
	now := h.now()
	v = port.AssetVersion{VersionID: ids.New(), Type: t, ConfigID: ids.New(), Version: 1, Status: chatasset.StatusDraft, Title: title,
		Payload: b, AuthorID: p.UserID, UpdatedAt: now, Revision: 1}
	err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		if pack != nil {
			taken, err := h.Repo.CodeTaken(ctx, pack.Code, v.ConfigID)
			if err != nil {
				return err
			}
			if taken {
				return problem.New(problem.Conflict, "a sticker pack with this code exists")
			}
		}
		if err := h.Repo.CreateItem(ctx, v, pack); err != nil {
			return err
		}
		return h.record(ctx, AuditChatAssetCreated, t, v.ConfigID, map[string]any{"version": 1})
	})
	return v, err
}

// UpdateDraft is updateGameConfigDraft: changes a draft; on the newest published/archived version it starts the next
// version as a draft (a published version is never changed, R31). match is the If-Match header: the revision of the
// version read by the editor (428 when missing, 412 VERSION_MISMATCH when someone saved meanwhile).
func (h ChatAssets) UpdateDraft(ctx context.Context, typ string, id uuid.UUID, version int, match, title string, payload json.RawMessage) (v port.AssetVersion, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.UpdateChatAssetDraft")
	defer func() { done(err) }()
	t, err := assetType(typ)
	if err != nil {
		return v, err
	}
	p, sc, err := assetWrite(ctx)
	if err != nil {
		return v, err
	}
	b, pack, err := normalize(t, payload)
	if err != nil {
		return v, err
	}
	err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		cur, err := h.Repo.Version(ctx, t, id, version)
		if err != nil {
			return err
		}
		if cur == nil {
			return problem.New(problem.NotFound, "")
		}
		if err := ifMatch(match, cur); err != nil {
			return err
		}
		if pack != nil {
			taken, err := h.Repo.CodeTaken(ctx, pack.Code, id)
			if err != nil {
				return err
			}
			if taken {
				return problem.New(problem.Conflict, "a sticker pack with this code exists")
			}
		}
		switch cur.Status {
		case chatasset.StatusDraft, chatasset.StatusRejected:
			cur.Payload, cur.Title, cur.Status, cur.UpdatedAt, cur.ReviewerID, cur.ReviewedAt = b, title, chatasset.StatusDraft, h.now(), nil, nil
			cur.AuthorID = p.UserID // the last editor answers for the content (four-eyes of the GIF review)
			if err := h.Repo.SaveVersion(ctx, *cur); err != nil {
				return err
			}
			cur.Revision++ // SaveVersion added 1 to the locked row
			v = *cur
		case chatasset.StatusPublished, chatasset.StatusArchived:
			latest, err := h.Repo.Latest(ctx, t, id)
			if err != nil {
				return err
			}
			if latest != version {
				return problem.New(problem.VersionMismatch, "")
			}
			v = port.AssetVersion{VersionID: ids.New(), Type: t, ConfigID: id, Version: latest + 1, Status: chatasset.StatusDraft, Title: title,
				Payload: b, AuthorID: p.UserID, UpdatedAt: h.now(), Revision: 1}
			if err := h.Repo.InsertVersion(ctx, v); err != nil {
				return err
			}
		default:
			return problem.New(problem.InvalidStateTransition, "").Wrap(chatasset.ErrImmutable)
		}
		return h.record(ctx, AuditChatAssetUpdated, t, id, map[string]any{"version": v.Version})
	})
	return v, err
}

// inspect applies the file rules of chat_asset_file_max to the files of a version (F118-AC-17): read from the bucket,
// never trusted from the client.
func (h ChatAssets) inspect(ctx context.Context, t chatasset.Type, payload []byte) error {
	_, err := h.inspectStamp(ctx, t, payload)
	return err
}

// inspectStamp is inspect returning the payload whose file facts (size, format, dimensions) are those read from the
// bucket, not the numbers the client declared (F082 Clarifications 2026-11-26).
func (h ChatAssets) inspectStamp(ctx context.Context, t chatasset.Type, payload []byte) ([]byte, error) {
	reject := func(err error) error {
		var fl chatasset.FileError
		if errors.As(err, &fl) && h.Metrics != nil {
			h.Metrics.AssetRejected(fl.Reason)
		}
		return fieldProblem(err)
	}
	open := func(key string, maxBytes int) (chatasset.File, error) {
		f, err := h.Inspector.Inspect(ctx, key, maxBytes)
		if errors.Is(err, port.ErrObjectNotFound) {
			return f, reject(chatasset.FileError{Field: "asset_key", Reason: chatasset.RejectMissing})
		}
		if err != nil {
			return f, problem.New(problem.Unavailable, "object storage unavailable").Retry()
		}
		return f, nil
	}
	switch t {
	case chatasset.TypeStickerPacks:
		var p chatasset.Pack
		if err := json.Unmarshal(payload, &p); err != nil {
			return nil, err
		}
		if len(p.Stickers) == 0 {
			return nil, problem.Invalid("payload.stickers", problem.SubInvalid, "at least one sticker")
		}
		for i, s := range p.Stickers {
			if s.AssetKey == "" {
				return nil, problem.Invalid("payload.stickers.asset_key", problem.SubInvalid, "required")
			}
			f, err := open(s.AssetKey, h.Limits.StickerMaxBytes)
			if err != nil {
				return nil, err
			}
			if err := chatasset.CheckSticker(f, h.Limits); err != nil {
				return nil, reject(err)
			}
			p.Stickers[i].SizeBytes, p.Stickers[i].ContentType = f.SizeBytes, f.ContentType
		}
		return json.Marshal(p)
	default:
		var g chatasset.Gif
		if err := json.Unmarshal(payload, &g); err != nil {
			return nil, err
		}
		if g.AssetKey == "" || g.PreviewKey == "" {
			return nil, problem.Invalid("payload.asset_key", problem.SubInvalid, "asset_key and preview_key are required")
		}
		a, err := open(g.AssetKey, h.Limits.GifMaxBytes)
		if err != nil {
			return nil, err
		}
		pv, err := open(g.PreviewKey, h.Limits.GifMaxBytes)
		if err != nil {
			return nil, err
		}
		if err := chatasset.CheckGif(a, pv, h.Limits); err != nil {
			return nil, reject(err)
		}
		g.SizeBytes, g.Width, g.Height = a.SizeBytes, a.Width, a.Height
		return json.Marshal(g)
	}
}

// Transition is transitionGameConfigVersion: submit_review (files checked), approve (a GIF by someone else than its
// author), reject, archive. match is the If-Match header (revision of the version, 428/412 as UpdateDraft).
func (h ChatAssets) Transition(ctx context.Context, typ string, id uuid.UUID, version int, match, action string) (v port.AssetVersion, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.TransitionChatAsset")
	defer func() { done(err) }()
	t, err := assetType(typ)
	if err != nil {
		return v, err
	}
	perm := authz.PermGameChatAssetsWrite
	if action == chatasset.ActionApprove || action == chatasset.ActionReject {
		perm = authz.PermGameContentReview // review is a separate right (menu.game_ops.chat_assets)
	}
	p, sc, err := platformWrite(ctx, perm)
	if err != nil {
		return v, err
	}
	err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		cur, err := h.Repo.Version(ctx, t, id, version)
		if err != nil {
			return err
		}
		if cur == nil {
			return problem.New(problem.NotFound, "")
		}
		if err := ifMatch(match, cur); err != nil {
			return err
		}
		next, err := chatasset.Next(t, cur.Status, action, p.UserID, cur.AuthorID)
		switch {
		case errors.Is(err, chatasset.ErrSelfReview):
			return problem.New(problem.PermissionDenied, "a GIF is approved by another reviewer than its author")
		case err != nil:
			return problem.New(problem.InvalidStateTransition, "")
		}
		if action == chatasset.ActionSubmit {
			if t == chatasset.TypeStickerPacks {
				var pack chatasset.Pack
				if err := json.Unmarshal(cur.Payload, &pack); err != nil {
					return err
				}
				if missing := pack.MissingTranslations(h.required()); len(missing) > 0 {
					pe := problem.New(problem.TranslationIncomplete, "")
					for i, m := range missing {
						if i == 20 {
							break
						}
						pe.AddField("payload"+m, problem.SubRequired, "a required language is missing")
					}
					return pe
				}
			}
			stamped, err := h.inspectStamp(ctx, t, cur.Payload)
			if err != nil {
				return err
			}
			cur.Payload = stamped
		}
		if action == chatasset.ActionApprove {
			at, by := h.now(), p.UserID
			cur.ReviewerID, cur.ReviewedAt = &by, &at
		}
		cur.Status, cur.UpdatedAt = next, h.now()
		if err := h.Repo.SaveVersion(ctx, *cur); err != nil {
			return err
		}
		cur.Revision++
		v = *cur
		return h.record(ctx, AuditChatAssetTransition, t, id, map[string]any{"version": version, "action": action, "to": string(next)})
	})
	return v, err
}

// Publish is publishGameConfigVersion: an approved version becomes the library state (immutable from now on), the version
// published before is archived, content.published is written (item_type sticker_pack|gif). match is the If-Match header
// (revision of the approved version, 428/412 as UpdateDraft).
func (h ChatAssets) Publish(ctx context.Context, typ string, id uuid.UUID, version int, match, note string) (r port.AssetRelease, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.PublishChatAsset")
	defer func() { done(err) }()
	t, err := assetType(typ)
	if err != nil {
		return r, err
	}
	p, sc, err := platformWrite(ctx, authz.PermGameContentPublish)
	if err != nil {
		return r, err
	}
	err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		cur, err := h.Repo.Version(ctx, t, id, version)
		if err != nil {
			return err
		}
		if cur == nil {
			return problem.New(problem.NotFound, "")
		}
		if err := ifMatch(match, cur); err != nil {
			return err
		}
		if cur.Status != chatasset.StatusApproved {
			return problem.New(problem.InvalidStateTransition, "").Wrap(chatasset.ErrNotPublished)
		}
		if t == chatasset.TypeGifs && (cur.ReviewerID == nil || *cur.ReviewerID == cur.AuthorID) {
			return problem.New(problem.PermissionDenied, "a GIF is approved by another reviewer than its author")
		}
		if err := h.inspect(ctx, t, cur.Payload); err != nil { // the files are still there and unchanged
			return err
		}
		prev, err := h.Repo.Published(ctx, t, id)
		if err != nil {
			return err
		}
		now := h.now()
		by := p.UserID
		cur.Status, cur.PublishedBy, cur.PublishedAt, cur.UpdatedAt = chatasset.StatusPublished, &by, &now, now
		to := cur.Version
		r = port.AssetRelease{ReleaseID: ids.New(), Type: t, ConfigID: id, Action: "publish", ToVersion: &to, Reason: note, ActorID: by, At: now}
		if prev != nil {
			from := prev.Version
			r.FromVersion = &from
		}
		if err := h.Repo.Publish(ctx, *cur, prev, r); err != nil {
			return err
		}
		if err := h.Events.Publish(ctx, write.Event{Type: EventContentPublished, SubjectType: t.ItemType(), SubjectID: id.String(),
			PartitionKey: id.String(), Platform: true, OccurredAt: now, Payload: map[string]any{"item_type": t.ItemType(), "item_id": id,
				"version_id": cur.VersionID, "version": cur.Version, "published_by": by, "published_at": now}}); err != nil {
			return err
		}
		return h.record(ctx, AuditChatAssetPublished, t, id, map[string]any{"version": version})
	})
	if err != nil {
		var pe *problem.Error
		if !errors.As(err, &pe) {
			slog.ErrorContext(ctx, "chat assets: publish failed", "error", err)
		}
	}
	return r, err
}

var fileNameRe = regexp.MustCompile(`^[A-Za-z0-9._-]{1,200}$`)

// Presign is the presigned upload of one library file (F118 contract delta): the key media/<sha256>/<file> is derived
// from the declared sha256 and checked again against the bytes at submit (the client is never trusted); the size and
// format limits of the type are applied before signing.
func (h ChatAssets) Presign(ctx context.Context, typ string, id uuid.UUID, file, contentType, sha string, size int) (key, url string, expires time.Time, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.PresignChatAsset")
	defer func() { done(err) }()
	t, err := assetType(typ)
	if err != nil {
		return "", "", expires, err
	}
	_, sc, err := assetWrite(ctx)
	if err != nil {
		return "", "", expires, err
	}
	file = path.Base(strings.TrimSpace(file))
	maxBytes := h.Limits.StickerMaxBytes
	allowed := map[string]bool{"image/webp": true, "image/png": true}
	if t == chatasset.TypeGifs {
		maxBytes, allowed = h.Limits.GifMaxBytes, map[string]bool{"image/gif": true, "image/webp": true, "image/png": true}
	}
	switch {
	case !fileNameRe.MatchString(file) || len(file) > assetPresignMaxFileLength:
		return "", "", expires, problem.Invalid("file_name", problem.SubInvalid, "^[A-Za-z0-9._-]{1,200}$")
	case !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(sha):
		return "", "", expires, problem.Invalid("sha256", problem.SubInvalid, "64 lowercase hex characters")
	case !allowed[contentType]:
		return "", "", expires, problem.Invalid("content_type", problem.SubInvalid, "format not allowed for this type")
	case size <= 0 || size > maxBytes:
		return "", "", expires, problem.Invalid("size_bytes", problem.SubInvalid, "over the file limit of the type")
	}
	key = "media/" + sha + "/" + file
	err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		latest, err := h.Repo.Latest(ctx, t, id)
		if err != nil {
			return err
		}
		if latest == 0 {
			return problem.New(problem.NotFound, "")
		}
		return h.record(ctx, AuditChatAssetPresigned, t, id, map[string]any{"key_sha256": sha, "size_bytes": size})
	})
	if err != nil {
		return "", "", expires, err
	}
	if h.Storage == nil {
		return "", "", expires, problem.New(problem.Unavailable, "object storage not configured").Retry()
	}
	url, err = h.Storage.PresignPut(ctx, key, int64(size), PresignTTL)
	if err != nil {
		return "", "", expires, problem.New(problem.Unavailable, "object storage unavailable").Retry()
	}
	return key, url, h.now().Add(PresignTTL), nil
}

// releaseInput checks the reason of a rollback/unpublish (required, ≤ 500).
func releaseReason(reason string) error {
	switch {
	case strings.TrimSpace(reason) == "":
		return problem.Invalid("reason", problem.SubRequired, "a reason is required")
	case utf8.RuneCountInString(reason) > MaxReasonLength:
		return problem.Invalid("reason", problem.SubMaxLength, "at most 500 characters")
	}
	return nil
}

// Rollback is rollbackGameConfig of sticker-packs/gifs (F082-AC-04, ADR-0043): the library takes the state of a version
// published before (it has published_at) again and the pointer moves to it; the version that was current is archived;
// no version content changes (R31). Reason required, game.content.publish, content.rolled_back + audit.
func (h ChatAssets) Rollback(ctx context.Context, typ string, id uuid.UUID, to int, reason string) (r port.AssetRelease, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.RollbackChatAsset")
	defer func() { done(err) }()
	t, err := assetType(typ)
	if err != nil {
		return r, err
	}
	p, sc, err := platformWrite(ctx, authz.PermGameContentPublish)
	if err != nil {
		return r, err
	}
	if err := releaseReason(reason); err != nil {
		return r, err
	}
	err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		target, err := h.Repo.Version(ctx, t, id, to)
		if err != nil {
			return err
		}
		if target == nil {
			return problem.New(problem.NotFound, "")
		}
		if target.PublishedAt == nil || (target.Status != chatasset.StatusPublished && target.Status != chatasset.StatusArchived) {
			return problem.Invalid("to_version", problem.SubInvalid, "a version published before")
		}
		cur, err := h.Repo.Current(ctx, t, id)
		if err != nil {
			return err
		}
		if cur != nil && cur.VersionID == target.VersionID {
			return problem.New(problem.InvalidStateTransition, "this version is already current")
		}
		if err := h.inspect(ctx, t, target.Payload); err != nil { // the files are still there and unchanged
			return err
		}
		now := h.now()
		tv := target.Version
		r = port.AssetRelease{ReleaseID: ids.New(), Type: t, ConfigID: id, Action: "rollback", ToVersion: &tv, Reason: reason, ActorID: p.UserID, At: now}
		if cur != nil {
			from := cur.Version
			r.FromVersion = &from
		}
		if err := h.Repo.Rollback(ctx, *target, cur, r); err != nil {
			return err
		}
		if err := h.Events.Publish(ctx, write.Event{Type: EventContentRolledBack, SubjectType: t.ItemType(), SubjectID: id.String(),
			PartitionKey: id.String(), Platform: true, OccurredAt: now, Payload: map[string]any{"item_type": t.ItemType(), "item_id": id,
				"from_version": r.FromVersion, "to_version": tv, "to_version_id": target.VersionID, "reason": reason, "rolled_back_by": p.UserID,
				"rolled_back_at": now}}); err != nil {
			return err
		}
		return h.record(ctx, AuditChatAssetRolledBack, t, id, map[string]any{"to_version": tv, "reason_length": utf8.RuneCountInString(reason)})
	})
	return r, err
}

// Unpublish is unpublishGameConfig of sticker-packs/gifs (F082-AC-04): students stop seeing the pack/GIF (status
// archived, pointer NULL); messages already sent keep showing it (assets are immutable). Reason required,
// game.content.publish, content.unpublished + audit.
func (h ChatAssets) Unpublish(ctx context.Context, typ string, id uuid.UUID, reason string) (r port.AssetRelease, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.UnpublishChatAsset")
	defer func() { done(err) }()
	t, err := assetType(typ)
	if err != nil {
		return r, err
	}
	p, sc, err := platformWrite(ctx, authz.PermGameContentPublish)
	if err != nil {
		return r, err
	}
	if err := releaseReason(reason); err != nil {
		return r, err
	}
	err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		latest, err := h.Repo.Latest(ctx, t, id)
		if err != nil {
			return err
		}
		if latest == 0 {
			return problem.New(problem.NotFound, "")
		}
		cur, err := h.Repo.Current(ctx, t, id)
		if err != nil {
			return err
		}
		if cur == nil {
			return problem.New(problem.InvalidStateTransition, "not published")
		}
		now := h.now()
		from := cur.Version
		r = port.AssetRelease{ReleaseID: ids.New(), Type: t, ConfigID: id, Action: "unpublish", FromVersion: &from, Reason: reason, ActorID: p.UserID, At: now}
		if err := h.Repo.Unpublish(ctx, t, id, cur, r); err != nil {
			return err
		}
		if err := h.Events.Publish(ctx, write.Event{Type: EventContentUnpublished, SubjectType: t.ItemType(), SubjectID: id.String(),
			PartitionKey: id.String(), Platform: true, OccurredAt: now, Payload: map[string]any{"item_type": t.ItemType(), "item_id": id,
				"from_version": from, "reason": reason, "unpublished_by": p.UserID, "unpublished_at": now}}); err != nil {
			return err
		}
		return h.record(ctx, AuditChatAssetUnpublished, t, id, map[string]any{"from_version": from, "reason_length": utf8.RuneCountInString(reason)})
	})
	return r, err
}
