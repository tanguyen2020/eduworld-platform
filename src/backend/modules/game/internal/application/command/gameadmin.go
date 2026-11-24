package command

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/gameadmin"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/gameconfig"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Events of the console workflow (specs/events/payloads/game, owner F078).
const (
	EventVersionSubmitted = "content.version_submitted"
	EventVersionApproved  = "content.version_approved"
	EventVersionRejected  = "content.version_rejected"
	EventVersionArchived  = "content.version_archived"
	EventRolledBack       = "content.rolled_back"
	EventUnpublished      = "content.unpublished"
	EventPublishScheduled = "content.publish_scheduled"
)

// Audit actions of the console (ADMIN_PORTAL §7, F078-AC-16).
const (
	ActionVersionCreated   = "content.version_created"
	ActionSubmitted        = "content.submitted"
	ActionApproved         = "content.approved"
	ActionRejected         = "content.rejected"
	ActionPublished        = "content.published"
	ActionPublishScheduled = "content.publish_scheduled"
	ActionScheduleCanceled = "content.publish_schedule_cancelled"
	ActionRolledBack       = "content.rolled_back"
	ActionUnpublished      = "content.unpublished"
	ActionArchived         = "content.archived"
	ActionConfigChanged    = "game_config.changed"
)

// GameAdminPolicy are the parameters of the console (CONFIG_DEFAULTS): scheduled_publish_min_lead,
// game_text_required_languages, assets_per_version_max, preview_token_ttl.
type GameAdminPolicy struct {
	MinLead             time.Duration
	RequiredLanguages   []string
	AssetsPerVersionMax int
	PreviewTTL          time.Duration
	// ScheduleBatch bounds the due publications the job applies per run.
	ScheduleBatch int
}

// GameAdminMetrics observes eduworld_game_config_transition_total{config_type,action}; nil = not observed.
type GameAdminMetrics interface {
	Transition(configType, action string)
}

// GameAdmin are the write use cases of the Game Operations Console (F078 US2–US4): drafts, review workflow, publish now
// or at a time, cancellation of a schedule, rollback, unpublish and preview sessions of every game config type, on top of
// the release core (PublishGameContent, F071) which keeps the type checks and the type hooks of F072–F077.
type GameAdmin struct {
	UoW    write.UnitOfWork
	Store  write.GameConfigStore
	Kernel *PublishGameContent
	// Learning checks that the referenced platform learning content is released (submit_review); nil = 503 when a
	// version references learning content.
	Learning port.LearningContentCheck
	Events   write.EventPublisher
	Audit    write.Auditor
	Metrics  GameAdminMetrics
	Policy   GameAdminPolicy
	Now      func() time.Time
}

// ConfigVersionView, ReleaseView and UsageView are the contract views (application/gameadmin).
type (
	ConfigVersionView = gameadmin.ConfigVersionView
	ReleaseView       = gameadmin.ReleaseView
	UsageView         = gameadmin.UsageView
)

// ETagOf is gameadmin.ETagOf.
func ETagOf(revision int64) string { return gameadmin.ETagOf(revision) }

// ifMatch compares an If-Match header with the revision (428 when missing, 412 when stale).
func ifMatch(header string, revision int64) error {
	h := strings.TrimSpace(header)
	if h == "" {
		return problem.New(problem.PreconditionRequired, "")
	}
	h = strings.TrimPrefix(h, "W/")
	if h != "*" && h != ETagOf(revision) && h != strconv.FormatInt(revision, 10) {
		return problem.New(problem.VersionMismatch, "")
	}
	return nil
}

func (h GameAdmin) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

func (h GameAdmin) metric(configType, action string) {
	if h.Metrics != nil {
		h.Metrics.Transition(configType, action)
	}
}

// Draft is the input of createGameConfig and updateGameConfigDraft.
type Draft struct {
	Title   *string
	Payload json.RawMessage
}

func checkTitle(title *string) error {
	if title != nil && utf8.RuneCountInString(*title) > gameconfig.MaxTitle {
		return problem.Invalid("title", problem.SubMaxLength, "at most 200 characters")
	}
	return nil
}

func decodePayload(raw json.RawMessage) (map[string]any, error) {
	if len(raw) > gameconfig.MaxPayloadBytes {
		return nil, problem.Invalid("payload", problem.SubMaxLength, "at most 65536 bytes")
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil || doc == nil {
		return nil, problem.Invalid("payload", problem.SubInvalid, "an object is required")
	}
	return doc, nil
}

// prepare normalizes the server fields of a payload and runs the type checks (F078-AC-04, AC-10).
func (h GameAdmin) prepare(ctx context.Context, t gameconfig.Type, doc map[string]any, key string, versionID uuid.UUID, no int,
	status gameconfig.Status, now time.Time) ([]byte, string, string, error) {
	t.Normalize(doc, key, versionID.String(), no, status)
	b, err := json.Marshal(doc)
	if err != nil {
		return nil, "", "", err
	}
	if len(b) > gameconfig.MaxPayloadBytes {
		return nil, "", "", problem.Invalid("payload", problem.SubMaxLength, "at most 65536 bytes")
	}
	parsed, err := h.Kernel.Validate(ctx, ContentItem{Type: t.ItemType, Payload: b}, now)
	if err != nil {
		return nil, "", "", err
	}
	sum := sha256.Sum256(b)
	return b, hex.EncodeToString(sum[:]), parsed.Key, nil
}

func (h GameAdmin) record(ctx context.Context, action string, t gameconfig.Type, configID uuid.UUID, reason string, meta map[string]any) error {
	if meta == nil {
		meta = map[string]any{}
	}
	meta["config_type"], meta["item_type"] = t.Name, t.ItemType
	return h.Audit.Record(ctx, audit.Event{Action: action, TargetType: "game_config", TargetID: configID.String(), Result: audit.ResultSuccess,
		Reason: reason, Metadata: meta})
}

// Create is createGameConfig_game: a new item with version 1 draft (author = principal) after the type checks.
func (h GameAdmin) Create(ctx context.Context, configType string, in Draft) (out ConfigVersionView, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.CreateGameConfig")
	defer func() { done(err) }()
	t, err := gameadmin.Type(configType)
	if err != nil {
		return out, err
	}
	p, sc, err := gameadmin.Principal(ctx)
	if err != nil {
		return out, err
	}
	if err := gameadmin.RequireWrite(ctx, p, configType); err != nil {
		return out, err
	}
	if err := checkTitle(in.Title); err != nil {
		return out, err
	}
	doc, err := decodePayload(in.Payload)
	if err != nil {
		return out, err
	}
	now := h.now()
	err = h.UoW.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		configID, vid := ids.New(), ids.New()
		if s, ok := doc[t.IDField].(string); ok && t.UUIDKey {
			// A UUID the author chose (stable ids of the dev seed files) is kept when no item has it yet; else new.
			if id, err := uuid.Parse(s); err == nil && id != uuid.Nil {
				configID = id
			}
		}
		key := configID.String()
		if !t.UUIDKey {
			if key = t.Key(doc); key == "" {
				return problem.Invalid("payload/"+t.IDField, problem.SubRequired, "the item code is required")
			}
		}
		b, sha, parsedKey, err := h.prepare(ctx, t, doc, key, vid, 1, gameconfig.StatusDraft, now)
		if err != nil {
			return err
		}
		if parsedKey != "" {
			key = parsedKey
		}
		if existing, err := h.Store.ItemByKey(ctx, t.ItemType, key); err != nil {
			return err
		} else if existing != nil {
			return problem.New(problem.Conflict, "the item already exists: create a new version of it")
		}
		it := write.ConfigItem{ConfigID: configID, ItemType: t.ItemType, ItemKey: key}
		if err := h.Store.CreateItem(ctx, it, p.UserID, now); err != nil {
			if errors.Is(err, write.ErrConflict) {
				return problem.New(problem.Conflict, "the item already exists")
			}
			return err
		}
		no, err := h.Store.NextVersionNo(ctx, it)
		if err != nil {
			return err
		}
		if no != 1 { // a code-keyed item already published by the seed: the draft continues its numbering
			if b, sha, _, err = h.prepare(ctx, t, doc, key, vid, no, gameconfig.StatusDraft, now); err != nil {
				return err
			}
		}
		author := p.UserID
		v := write.ConfigVersion{ID: vid, ConfigID: configID, ItemType: t.ItemType, VersionNo: no, Status: string(gameconfig.StatusDraft),
			Title: in.Title, Payload: b, SHA256: sha, AuthorID: &author, UpdatedAt: now, Revision: 1}
		if err := h.Store.InsertVersion(ctx, v, p.UserID, now); err != nil {
			return err
		}
		if err := h.record(ctx, ActionVersionCreated, t, configID, "", map[string]any{"version": no}); err != nil {
			return err
		}
		out = configView(t, v, false)
		return idempotency.SetResult(ctx, out)
	})
	if err == nil {
		h.metric(configType, "create")
	}
	return out, err
}

// CreateVersion is createGameConfigVersion: a new draft copied from basedOn (default the current release, else the
// newest version); the source version is not changed (F078-AC-06).
func (h GameAdmin) CreateVersion(ctx context.Context, configType string, configID uuid.UUID, basedOn *int, title, changeNote *string) (out ConfigVersionView, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.CreateGameConfigVersion")
	defer func() { done(err) }()
	t, err := gameadmin.Type(configType)
	if err != nil {
		return out, err
	}
	p, sc, err := gameadmin.Principal(ctx)
	if err != nil {
		return out, err
	}
	if err := gameadmin.RequireWrite(ctx, p, configType); err != nil {
		return out, err
	}
	if err := checkTitle(title); err != nil {
		return out, err
	}
	if changeNote != nil && utf8.RuneCountInString(*changeNote) > gameconfig.MaxChangeNote {
		return out, problem.Invalid("change_note", problem.SubMaxLength, "at most 500 characters")
	}
	now := h.now()
	err = h.UoW.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		it, err := h.Store.ItemByID(ctx, t.ItemType, configID)
		if err != nil {
			return err
		}
		if it == nil {
			return problem.New(problem.NotFound, "")
		}
		src, err := h.sourceVersion(ctx, t, *it, basedOn)
		if err != nil {
			return err
		}
		var doc map[string]any
		if err := json.Unmarshal(src.Payload, &doc); err != nil {
			return err
		}
		no, err := h.Store.NextVersionNo(ctx, *it)
		if err != nil {
			return err
		}
		vid := ids.New()
		b, sha, _, err := h.prepare(ctx, t, doc, it.ItemKey, vid, no, gameconfig.StatusDraft, now)
		if err != nil {
			return err
		}
		if title == nil {
			title = src.Title
		}
		author, based := p.UserID, src.ID
		v := write.ConfigVersion{ID: vid, ConfigID: configID, ItemType: t.ItemType, VersionNo: no, Status: string(gameconfig.StatusDraft),
			Title: title, Payload: b, SHA256: sha, AuthorID: &author, ChangeNote: changeNote, BasedOn: &based, UpdatedAt: now, Revision: 1}
		if err := h.Store.InsertVersion(ctx, v, p.UserID, now); err != nil {
			return err
		}
		if err := h.record(ctx, ActionVersionCreated, t, configID, "", map[string]any{"version": no, "based_on_version": src.VersionNo}); err != nil {
			return err
		}
		out = configView(t, v, false)
		bn := src.VersionNo
		out.BasedOnVersion = &bn
		return idempotency.SetResult(ctx, out)
	})
	if err == nil {
		h.metric(configType, "create_version")
	}
	return out, err
}

func (h GameAdmin) sourceVersion(ctx context.Context, t gameconfig.Type, it write.ConfigItem, basedOn *int) (*write.ConfigVersion, error) {
	no := 0
	switch {
	case basedOn != nil:
		no = *basedOn
	default:
		ptr, err := h.Store.Pointer(ctx, t.ItemType, it.ItemKey)
		if err != nil {
			return nil, err
		}
		for n := it.LatestVersionNo; n >= 1 && no == 0; n-- {
			v, err := h.Store.Version(ctx, it.ConfigID, n)
			if err != nil {
				return nil, err
			}
			if v != nil && (ptr.Current == nil || v.ID == *ptr.Current) {
				no = n
			}
		}
	}
	v, err := h.Store.Version(ctx, it.ConfigID, no)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, problem.Invalid("based_on_version", problem.SubInvalid, "no such version of the item")
	}
	return v, nil
}

// UpdateDraft is updateGameConfigDraft_game: If-Match required (428), stale ETag 412, only a draft (or a rejected
// version, which returns to draft) is editable — any other status is 409 CONTENT_NOT_EDITABLE (F078-AC-05, R11).
func (h GameAdmin) UpdateDraft(ctx context.Context, configType string, configID uuid.UUID, no int, match string, in Draft) (out ConfigVersionView, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.UpdateGameConfigDraft")
	defer func() { done(err) }()
	t, err := gameadmin.Type(configType)
	if err != nil {
		return out, err
	}
	p, sc, err := gameadmin.Principal(ctx)
	if err != nil {
		return out, err
	}
	if err := gameadmin.RequireWrite(ctx, p, configType); err != nil {
		return out, err
	}
	if err := checkTitle(in.Title); err != nil {
		return out, err
	}
	doc, err := decodePayload(in.Payload)
	if err != nil {
		return out, err
	}
	now := h.now()
	err = h.UoW.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		it, v, err := h.load(ctx, t, configID, no)
		if err != nil {
			return err
		}
		if err := ifMatch(match, v.Revision); err != nil {
			return err
		}
		if !gameconfig.Editable(gameconfig.Status(v.Status)) {
			return problem.New(problem.ContentNotEditable, "")
		}
		var before map[string]any
		_ = json.Unmarshal(v.Payload, &before)
		b, sha, _, err := h.prepare(ctx, t, doc, it.ItemKey, v.ID, v.VersionNo, gameconfig.StatusDraft, now)
		if err != nil {
			return err
		}
		next := *v
		next.Payload, next.SHA256, next.Status = b, sha, string(gameconfig.StatusDraft)
		if in.Title != nil {
			next.Title = in.Title
		}
		ok, err := h.Store.UpdateDraft(ctx, next, v.Revision, p.UserID, now)
		if err != nil {
			return err
		}
		if !ok {
			return problem.New(problem.VersionMismatch, "")
		}
		var after map[string]any
		_ = json.Unmarshal(b, &after)
		if err := h.record(ctx, ActionConfigChanged, t, configID, "", map[string]any{"version": v.VersionNo,
			"diff_summary": diffSummary(t, before, after)}); err != nil {
			return err
		}
		next.Revision, next.UpdatedAt = v.Revision+1, now
		cur, err := h.isCurrent(ctx, t, *it, v.ID)
		if err != nil {
			return err
		}
		out = configView(t, next, cur)
		return nil
	})
	if err == nil {
		h.metric(configType, "update")
	}
	return out, err
}

// diffSummary is the audit summary of a change (count and the first field paths).
func diffSummary(t gameconfig.Type, a, b map[string]any) map[string]any {
	changes, truncated := gameconfig.Diff(t, a, b)
	paths := make([]string, 0, 20)
	for i, c := range changes {
		if i == 20 {
			break
		}
		paths = append(paths, c.Op+" "+c.Path)
	}
	return map[string]any{"changes": len(changes), "truncated": truncated, "paths": paths}
}

func (h GameAdmin) load(ctx context.Context, t gameconfig.Type, configID uuid.UUID, no int) (*write.ConfigItem, *write.ConfigVersion, error) {
	it, err := h.Store.ItemByID(ctx, t.ItemType, configID)
	if err != nil {
		return nil, nil, err
	}
	if it == nil {
		return nil, nil, problem.New(problem.NotFound, "")
	}
	v, err := h.Store.Version(ctx, configID, no)
	if err != nil {
		return nil, nil, err
	}
	if v == nil {
		return nil, nil, problem.New(problem.NotFound, "")
	}
	return it, v, nil
}

func (h GameAdmin) isCurrent(ctx context.Context, t gameconfig.Type, it write.ConfigItem, versionID uuid.UUID) (bool, error) {
	ptr, err := h.Store.Pointer(ctx, t.ItemType, it.ItemKey)
	if err != nil {
		return false, err
	}
	return ptr.Current != nil && *ptr.Current == versionID, nil
}

// Transition is transitionGameConfigVersion_game (F078-AC-07, AC-08): submit_review runs the checks of a submission;
// approve/reject need game.content.review and a reviewer other than the author; archive never touches the current
// release. Each transition writes its event and its audit.
func (h GameAdmin) Transition(ctx context.Context, configType string, configID uuid.UUID, no int, match string, action, note string) (out ConfigVersionView, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.TransitionGameConfigVersion")
	defer func() { done(err) }()
	t, err := gameadmin.Type(configType)
	if err != nil {
		return out, err
	}
	p, sc, err := gameadmin.Principal(ctx)
	if err != nil {
		return out, err
	}
	a := gameconfig.Action(action)
	switch a {
	case gameconfig.ActionApprove, gameconfig.ActionReject:
		err = gameadmin.RequireReview(ctx, p)
	case gameconfig.ActionSubmitReview, gameconfig.ActionArchive:
		err = gameadmin.RequireWrite(ctx, p, configType)
	default:
		return out, problem.Invalid("action", problem.SubInvalid, "unknown action")
	}
	if err != nil {
		return out, err
	}
	now := h.now()
	err = h.UoW.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		it, v, err := h.load(ctx, t, configID, no)
		if err != nil {
			return err
		}
		if err := ifMatch(match, v.Revision); err != nil {
			return err
		}
		cur, err := h.isCurrent(ctx, t, *it, v.ID)
		if err != nil {
			return err
		}
		to, err := gameconfig.Transition(gameconfig.Version{Status: gameconfig.Status(v.Status), Author: v.AuthorID, IsCurrent: cur},
			gameconfig.Request{Action: a, Actor: p.UserID, Note: note})
		if err != nil {
			return transitionProblem(ctx, p, err)
		}
		if a == gameconfig.ActionSubmitReview {
			if err := h.checkSubmission(ctx, t, *v, now); err != nil {
				return err
			}
		}
		next := *v
		next.Status = string(to)
		if note != "" {
			next.ReviewNote = &note
		}
		switch a {
		case gameconfig.ActionSubmitReview:
			next.SubmittedAt, next.ReviewerID, next.ReviewedAt = &now, nil, nil
		case gameconfig.ActionApprove, gameconfig.ActionReject:
			reviewer := p.UserID
			next.ReviewerID, next.ReviewedAt = &reviewer, &now
		}
		ok, err := h.Store.SetWorkflow(ctx, next, v.Revision, p.UserID, now)
		if err != nil {
			return err
		}
		if !ok {
			return problem.New(problem.VersionMismatch, "")
		}
		if a == gameconfig.ActionArchive && v.Status == string(gameconfig.StatusPublished) {
			if err := h.Store.SetTypeStatus(ctx, t.ItemType, v.ID, string(gameconfig.StatusArchived), now); err != nil {
				return err
			}
		}
		if err := h.transitionEvent(ctx, t, *it, next, a, p.UserID, note, now); err != nil {
			return err
		}
		next.Revision, next.UpdatedAt = v.Revision+1, now
		out = configView(t, next, cur)
		return idempotency.SetResult(ctx, out)
	})
	if err == nil {
		h.metric(configType, action)
	}
	return out, err
}

func transitionProblem(ctx context.Context, p authz.Principal, err error) error {
	switch {
	case errors.Is(err, gameconfig.ErrSelfReview):
		authz.RecordDenied(ctx, "reviewer_is_author", map[string]any{"user_id": p.UserID.String()})
		return problem.New(problem.PermissionDenied, "the reviewer must not be the author")
	case errors.Is(err, gameconfig.ErrNoteRequired):
		return problem.Invalid("note", problem.SubRequired, "a reason is required to reject")
	case errors.Is(err, gameconfig.ErrNoteTooLong):
		return problem.Invalid("note", problem.SubMaxLength, "at most 2000 characters")
	case errors.Is(err, gameconfig.ErrArchiveCurrent):
		return problem.New(problem.InvalidStateTransition, "the current release cannot be archived")
	case errors.Is(err, gameconfig.ErrInvalidTransition):
		return problem.New(problem.InvalidStateTransition, "")
	}
	return problem.Invalid("action", problem.SubInvalid, err.Error())
}

// checkSubmission runs the checks of submit_review (F078-AC-07, CONTENT_MODEL §6): type checks and references against
// the published content (ITEM_REF_INVALID), area/NPC/scene/interactable ids against the newest client manifest
// (ITEM_REF_INVALID), referenced media ready (MEDIA_NOT_READY) and at most assets_per_version_max, required languages
// (TRANSLATION_INCOMPLETE); the media of the version are linked (media_usages).
func (h GameAdmin) checkSubmission(ctx context.Context, t gameconfig.Type, v write.ConfigVersion, now time.Time) error {
	var doc map[string]any
	if err := json.Unmarshal(v.Payload, &doc); err != nil {
		return err
	}
	if missing := gameconfig.MissingTranslations(doc, h.Policy.RequiredLanguages); len(missing) > 0 {
		pe := problem.New(problem.TranslationIncomplete, "")
		for i, m := range missing {
			if i == 20 {
				break
			}
			pe.AddField("payload"+m, problem.SubRequired, "a required language is missing")
		}
		return pe
	}
	if err := h.Kernel.CheckReferences(ctx, ContentItem{Type: t.ItemType, Payload: v.Payload}, now); err != nil {
		return err
	}
	if err := h.checkLearningRefs(ctx, doc); err != nil {
		return err
	}
	refs := gameconfig.CollectRefs(doc)
	manifests, err := h.Store.LatestClientManifests(ctx)
	if err != nil {
		return err
	}
	for _, raw := range manifests {
		var m gameconfig.ClientManifest
		if err := json.Unmarshal(raw, &m); err != nil {
			return fmt.Errorf("client manifest: %w", err)
		}
		if missing := m.Missing(refs); len(missing) > 0 {
			return problem.New(problem.ItemRefInvalid, "not in the client manifest: "+strings.Join(first(missing, 10), ", "))
		}
	}
	if max := h.Policy.AssetsPerVersionMax; max > 0 && len(refs.Assets) > max {
		return problem.Invalid("payload", problem.SubInvalid, fmt.Sprintf("at most %d media assets per version", max))
	}
	assets := make([]uuid.UUID, 0, len(refs.Assets))
	for _, a := range refs.Assets {
		id, err := uuid.Parse(a)
		if err != nil {
			continue
		}
		assets = append(assets, id)
	}
	states, err := h.Store.MediaStatuses(ctx, assets)
	if err != nil {
		return err
	}
	for _, a := range assets {
		s, ok := states[a]
		if !ok {
			return problem.New(problem.ItemRefInvalid, "unknown media asset "+a.String())
		}
		if s.Status != string(gameconfig.MediaReady) {
			return problem.New(problem.MediaNotReady, "media asset "+a.String()+" is "+s.Status)
		}
	}
	return h.Store.LinkMedia(ctx, assets, t.ItemType, v.ConfigID, v.ID, uuid.Nil, now)
}

// checkLearningRefs asks module learning whether the learning content of the version (quest steps, challenge
// activity_refs, mini-game question set) is released (F078-AC-07, F076 handoff): any unreleased one is 422 ITEM_REF_INVALID.
func (h GameAdmin) checkLearningRefs(ctx context.Context, doc map[string]any) error {
	raw := contentRefs(doc)
	if len(raw) == 0 {
		return nil
	}
	if h.Learning == nil {
		return problem.New(problem.Unavailable, "module learning is not wired").Retry()
	}
	refs := make([]port.LearningRef, 0, len(raw))
	for _, r := range raw {
		id, err := uuid.Parse(r["content_id"].(string))
		if err != nil {
			continue
		}
		ref := port.LearningRef{ContentType: r["content_type"].(string), ContentID: id}
		if v, ok := r["version"].(int); ok {
			ref.Version = &v
		}
		refs = append(refs, ref)
	}
	ok, err := h.Learning.PublishedContent(ctx, refs)
	if errors.Is(err, port.ErrLearningUnavailable) {
		return problem.New(problem.Unavailable, "module learning is unavailable").Retry().Wrap(err)
	}
	if err != nil {
		return err
	}
	var missing []string
	for i, published := range ok {
		if !published && i < len(refs) {
			missing = append(missing, refs[i].ContentType+":"+refs[i].ContentID.String())
		}
	}
	if len(missing) > 0 {
		return problem.New(problem.ItemRefInvalid, "learning content not published: "+strings.Join(first(missing, 10), ", "))
	}
	return nil
}

func first(list []string, n int) []string {
	if len(list) > n {
		return list[:n]
	}
	return list
}

func (h GameAdmin) transitionEvent(ctx context.Context, t gameconfig.Type, it write.ConfigItem, v write.ConfigVersion, a gameconfig.Action,
	actor uuid.UUID, note string, now time.Time) error {
	payload := map[string]any{"config_type": t.Name, "item_type": t.ItemType, "config_id": it.ConfigID, "item_id": it.ItemKey,
		"config_version_id": v.ID, "version": v.VersionNo, "owner_scope": "platform", "tenant_id": nil, "author_id": authorOf(v)}
	var evt, action string
	switch a {
	case gameconfig.ActionSubmitReview:
		evt, action = EventVersionSubmitted, ActionSubmitted
	case gameconfig.ActionApprove:
		evt, action = EventVersionApproved, ActionApproved
		payload["reviewer_id"] = actor
		if note != "" {
			payload["note"] = note
		}
	case gameconfig.ActionReject:
		evt, action = EventVersionRejected, ActionRejected
		payload["reviewer_id"], payload["note"] = actor, note
	case gameconfig.ActionArchive:
		evt, action = EventVersionArchived, ActionArchived
		payload["archived_by"] = actor
		if note != "" {
			payload["note"] = note
		}
	}
	if err := h.Events.Publish(ctx, write.Event{Type: evt, SubjectType: t.ItemType, SubjectID: it.ItemKey, Platform: true, OccurredAt: now,
		Payload: payload}); err != nil {
		return err
	}
	return h.record(ctx, action, t, it.ConfigID, note, map[string]any{"version": v.VersionNo})
}

// authorOf is the author of a version for the events (the system actor uuid.Nil for a seed version).
func authorOf(v write.ConfigVersion) uuid.UUID {
	if v.AuthorID != nil {
		return *v.AuthorID
	}
	return uuid.Nil
}

// Publish is publishGameConfigVersion_game (F078-AC-11, AC-12): an approved version, If-Match required. Without
// publish_at it is published now through the release core (snapshot, pointer, release row, type hook, content.published;
// the manifest builder rebuilds the tenant manifests from the event). With publish_at ≥ now + scheduled_publish_min_lead
// it is scheduled (202, content.publish_scheduled) and the job scheduled-publish applies it once at its time.
func (h GameAdmin) Publish(ctx context.Context, configType string, configID uuid.UUID, no int, match string, publishAt *time.Time,
	changeNote string) (out ReleaseView, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.PublishGameConfig")
	defer func() { done(err) }()
	t, err := gameadmin.Type(configType)
	if err != nil {
		return out, err
	}
	p, sc, err := gameadmin.Principal(ctx)
	if err != nil {
		return out, err
	}
	if err := gameadmin.RequirePublish(ctx, p); err != nil {
		return out, err
	}
	if utf8.RuneCountInString(changeNote) > gameconfig.MaxChangeNote {
		return out, problem.Invalid("change_note", problem.SubMaxLength, "at most 500 characters")
	}
	now := h.now()
	if publishAt != nil {
		if err := gameconfig.CheckSchedule(publishAt.UTC(), now, h.Policy.MinLead); err != nil {
			return out, problem.Invalid("publish_at", problem.SubInvalid,
				fmt.Sprintf("must be at least %s after now (scheduled_publish_min_lead)", h.Policy.MinLead))
		}
	}
	key := idempotencyKey(ctx)
	err = h.UoW.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		it, v, err := h.load(ctx, t, configID, no)
		if err != nil {
			return err
		}
		if err := ifMatch(match, v.Revision); err != nil {
			return err
		}
		if err := gameconfig.CheckPublish(gameconfig.Status(v.Status)); err != nil {
			return problem.New(problem.InvalidStateTransition, "only an approved version can be published")
		}
		if publishAt != nil {
			out, err = h.schedule(ctx, t, *it, *v, publishAt.UTC(), changeNote, p.UserID, now)
		} else {
			out, err = h.publishNow(ctx, t, *it, *v, changeNote, key, uuid.Nil, false, p.UserID, now)
		}
		if err != nil {
			return err
		}
		return idempotency.SetResult(ctx, out)
	})
	if err == nil {
		h.metric(configType, "publish")
	}
	return out, err
}

func idempotencyKey(ctx context.Context) string {
	if pnd, ok := idempotency.PendingFrom(ctx); ok {
		return pnd.Key.Key.String()
	}
	return ""
}

func (h GameAdmin) schedule(ctx context.Context, t gameconfig.Type, it write.ConfigItem, v write.ConfigVersion, at time.Time, note string,
	by uuid.UUID, now time.Time) (ReleaseView, error) {
	active, err := h.Store.ActiveSchedule(ctx, it.ConfigID)
	if err != nil {
		return ReleaseView{}, err
	}
	if active != nil {
		return ReleaseView{}, problem.New(problem.Conflict, "the item already has a scheduled publication: cancel it first")
	}
	s := write.Schedule{ID: ids.New(), ConfigID: it.ConfigID, ItemType: t.ItemType, VersionID: v.ID, VersionNo: v.VersionNo, PublishAt: at,
		Status: string(gameconfig.ScheduleScheduled), RequestedBy: by, CreatedAt: now}
	if note != "" {
		s.ChangeNote = &note
	}
	if err := h.Store.InsertSchedule(ctx, s, now); err != nil {
		if errors.Is(err, write.ErrConflict) {
			return ReleaseView{}, problem.New(problem.Conflict, "the item already has a scheduled publication")
		}
		return ReleaseView{}, err
	}
	payload := map[string]any{"item_type": t.ItemType, "item_id": it.ItemKey, "config_id": it.ConfigID, "version_id": v.ID, "version": v.VersionNo,
		"schedule_id": s.ID, "publish_at": at, "owner_scope": "platform", "tenant_id": nil, "requested_by": by}
	if note != "" {
		payload["change_note"] = note
	}
	if err := h.Events.Publish(ctx, write.Event{Type: EventPublishScheduled, SubjectType: t.ItemType, SubjectID: it.ItemKey, Platform: true,
		OccurredAt: now, Payload: payload}); err != nil {
		return ReleaseView{}, err
	}
	if err := h.record(ctx, ActionPublishScheduled, t, it.ConfigID, note, map[string]any{"version": v.VersionNo, "publish_at": at,
		"schedule_id": s.ID}); err != nil {
		return ReleaseView{}, err
	}
	return scheduleView(s, it.ConfigID, nil), nil
}

func scheduleView(s write.Schedule, configID uuid.UUID, from *int) ReleaseView {
	to, at := s.VersionNo, s.PublishAt
	r := ReleaseView{ReleaseID: s.ID, ItemType: s.ItemType, ItemID: configID, Action: string(gameconfig.ReleasePublish), FromVersion: from,
		ToVersion: &to, ScheduledFor: &at, Status: s.Status, FailureCode: s.FailureCode, ReleasedBy: s.RequestedBy, ReleasedAt: s.CreatedAt,
		Reason: s.Reason, Pending: s.Status == string(gameconfig.ScheduleScheduled)}
	if s.CancelledBy != nil {
		r.ReleasedBy = *s.CancelledBy
	}
	if s.FinishedAt != nil {
		r.ReleasedAt = *s.FinishedAt
	}
	return r
}

// publishNow publishes an approved version through the release core in the caller transaction and records it.
func (h GameAdmin) publishNow(ctx context.Context, t gameconfig.Type, it write.ConfigItem, v write.ConfigVersion, note, key string,
	releaseID uuid.UUID, scheduled bool, by uuid.UUID, now time.Time) (ReleaseView, error) {
	ptr, err := h.Store.Pointer(ctx, t.ItemType, it.ItemKey)
	if err != nil {
		return ReleaseView{}, err
	}
	var from *int
	var before map[string]any
	if ptr.Current != nil {
		if raw, err := h.Store.TypeVersion(ctx, t.ItemType, *ptr.Current); err == nil && raw != nil {
			_ = json.Unmarshal(raw, &before)
			if n, ok := before["version"].(float64); ok {
				fn := int(n)
				from = &fn
			}
		}
	}
	if key == "" {
		key = releaseIDKey(releaseID)
	}
	rel, err := h.Kernel.PublishPinned(ctx, ContentItem{Type: t.ItemType, Payload: v.Payload},
		Pin{ConfigID: it.ConfigID, VersionID: v.ID, VersionNo: v.VersionNo, ReleaseID: releaseID, Scheduled: scheduled}, note, key, by, now)
	if err != nil {
		return ReleaseView{}, err
	}
	var after map[string]any
	_ = json.Unmarshal(v.Payload, &after)
	if err := h.record(ctx, ActionPublished, t, it.ConfigID, note, map[string]any{"version": v.VersionNo, "release_id": rel.ReleaseID,
		"scheduled": scheduled}); err != nil {
		return ReleaseView{}, err
	}
	if err := h.record(ctx, ActionConfigChanged, t, it.ConfigID, note, map[string]any{"version": v.VersionNo,
		"diff_summary": diffSummary(t, before, after)}); err != nil {
		return ReleaseView{}, err
	}
	to := v.VersionNo
	r := ReleaseView{ReleaseID: rel.ReleaseID, ItemType: t.ItemType, ItemID: it.ConfigID, Action: string(gameconfig.ReleasePublish),
		FromVersion: from, ToVersion: &to, Status: string(gameconfig.ScheduleApplied), ReleasedBy: by, ReleasedAt: now}
	if note != "" {
		r.Reason = &note
	}
	return r, nil
}

func releaseIDKey(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}
	return id.String()
}

// CancelSchedule is cancelScheduledGameConfigPublish (F078-AC-12): a scheduled publication is cancelled before its time,
// with an audit; any other status is 422 INVALID_STATE_TRANSITION.
func (h GameAdmin) CancelSchedule(ctx context.Context, configType string, configID, releaseID uuid.UUID, reason string) (out ReleaseView, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.CancelScheduledGameConfigPublish")
	defer func() { done(err) }()
	t, err := gameadmin.Type(configType)
	if err != nil {
		return out, err
	}
	p, sc, err := gameadmin.Principal(ctx)
	if err != nil {
		return out, err
	}
	if err := gameadmin.RequirePublish(ctx, p); err != nil {
		return out, err
	}
	if utf8.RuneCountInString(reason) > gameconfig.MaxReason {
		return out, problem.Invalid("reason", problem.SubMaxLength, "at most 500 characters")
	}
	now := h.now()
	err = h.UoW.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		it, err := h.Store.ItemByID(ctx, t.ItemType, configID)
		if err != nil {
			return err
		}
		if it == nil {
			return problem.New(problem.NotFound, "")
		}
		s, err := h.Store.ScheduleByID(ctx, configID, releaseID)
		if err != nil {
			return err
		}
		if s == nil {
			return problem.New(problem.NotFound, "")
		}
		st, err := gameconfig.Cancel(gameconfig.ScheduleStatus(s.Status))
		if err != nil {
			return problem.New(problem.InvalidStateTransition, "the publication is no longer scheduled")
		}
		by := p.UserID
		s.Status, s.CancelledBy, s.FinishedAt = string(st), &by, &now
		if reason != "" {
			s.Reason = &reason
		}
		if err := h.Store.FinishSchedule(ctx, *s, now); err != nil {
			return err
		}
		if err := h.record(ctx, ActionScheduleCanceled, t, configID, reason, map[string]any{"schedule_id": s.ID, "version": s.VersionNo,
			"publish_at": s.PublishAt}); err != nil {
			return err
		}
		out = scheduleView(*s, configID, nil)
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// ApplyDueSchedules is the job scheduled-publish (F078-AC-12): each due publication is applied once (SKIP LOCKED); a
// version no longer approved (archived meanwhile) is not published: the schedule fails with its guard code and an
// audit content.publish_scheduled cancelled.
func (h GameAdmin) ApplyDueSchedules(ctx context.Context) (applied int, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.ApplyScheduledPublications")
	defer func() { done(err) }()
	sc, err := authz.SystemPlatformScope("scheduled-publish")
	if err != nil {
		return 0, err
	}
	now := h.now()
	limit := h.Policy.ScheduleBatch
	if limit <= 0 {
		limit = 50
	}
	var due []write.Schedule
	if err := h.UoW.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		var err error
		due, err = h.Store.DueSchedules(ctx, now, limit)
		return err
	}); err != nil {
		return 0, err
	}
	for _, s := range due {
		ok, err := h.applyOne(tenancy.WithScope(ctx, sc), s.ConfigID, s.ID, now)
		if err != nil {
			return applied, err
		}
		if ok {
			applied++
		}
	}
	return applied, nil
}

func (h GameAdmin) applyOne(ctx context.Context, configID, scheduleID uuid.UUID, now time.Time) (bool, error) {
	published := false
	err := h.UoW.Do(ctx, func(ctx context.Context) error {
		s, err := h.Store.ScheduleByID(ctx, configID, scheduleID)
		if err != nil || s == nil || !gameconfig.Due(gameconfig.ScheduleStatus(s.Status), s.PublishAt, now) {
			return err
		}
		t, ok := gameconfig.ByItemType(s.ItemType)
		if !ok {
			return fmt.Errorf("schedule %s: unknown item type %s", s.ID, s.ItemType)
		}
		it, v, err := h.load(ctx, t, configID, s.VersionNo)
		if err != nil {
			return err
		}
		if gameconfig.CheckPublish(gameconfig.Status(v.Status)) != nil {
			code := "VERSION_NOT_APPROVED"
			s.Status, s.FailureCode, s.FinishedAt = string(gameconfig.ScheduleFailed), &code, &now
			if err := h.Store.FinishSchedule(ctx, *s, now); err != nil {
				return err
			}
			return h.record(ctx, ActionPublishScheduled, t, configID, "version "+v.Status+" before its time",
				map[string]any{"schedule_id": s.ID, "version": s.VersionNo, "outcome": "cancelled"})
		}
		note := ""
		if s.ChangeNote != nil {
			note = *s.ChangeNote
		}
		if _, err := h.publishNow(ctx, t, *it, *v, note, s.ID.String(), s.ID, true, s.RequestedBy, now); err != nil {
			return err
		}
		s.Status, s.FinishedAt = string(gameconfig.ScheduleApplied), &now
		if err := h.Store.FinishSchedule(ctx, *s, now); err != nil {
			return err
		}
		published = true
		h.metric(t.Name, "publish_scheduled")
		return nil
	})
	return published, err
}

// Rollback is rollbackGameConfig_game (F078-AC-13, R31): the pointer moves to another published version of the item;
// no version is created; content.rolled_back carries the type blocks of the target (manifest, mini-game projection,
// catalog). An invalid target is 422 ROLLBACK_TARGET_INVALID.
func (h GameAdmin) Rollback(ctx context.Context, configType string, configID uuid.UUID, toVersion int, reason string) (out ReleaseView, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.RollbackGameConfig")
	defer func() { done(err) }()
	t, err := gameadmin.Type(configType)
	if err != nil {
		return out, err
	}
	p, sc, err := gameadmin.Principal(ctx)
	if err != nil {
		return out, err
	}
	if err := gameadmin.RequirePublish(ctx, p); err != nil {
		return out, err
	}
	now := h.now()
	key := idempotencyKey(ctx)
	err = h.UoW.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		it, err := h.Store.ItemByID(ctx, t.ItemType, configID)
		if err != nil {
			return err
		}
		if it == nil {
			return problem.New(problem.NotFound, "")
		}
		ptr, err := h.Store.Pointer(ctx, t.ItemType, it.ItemKey)
		if err != nil {
			return err
		}
		target, err := h.Store.Version(ctx, configID, toVersion)
		if err != nil {
			return err
		}
		tg := gameconfig.Target{Status: gameconfig.StatusDraft}
		if target != nil {
			tg = gameconfig.Target{ID: target.ID, Status: gameconfig.Status(target.Status)}
		}
		if err := gameconfig.CheckRollback(t, ptr.Current, tg, reason); err != nil {
			return releaseProblem(err)
		}
		stored, err := h.Store.TypeVersion(ctx, t.ItemType, target.ID)
		if err != nil {
			return err
		}
		if stored == nil {
			return problem.New(problem.RollbackTargetInvalid, "the target is not published")
		}
		if err := h.Store.SetTypeStatus(ctx, t.ItemType, target.ID, string(gameconfig.StatusPublished), now); err != nil {
			return err
		}
		if err := h.Store.MovePointer(ctx, t.ItemType, it.ItemKey, &target.ID, p.UserID, now); err != nil {
			return err
		}
		from := h.versionNoOf(ctx, t, ptr.Current)
		rid := ids.New()
		if key == "" {
			key = rid.String()
		}
		if err := h.Kernel.Store.Release(ctx, write.ContentRelease{ReleaseID: rid, ItemType: t.ItemType, ItemID: it.ItemKey,
			Action: string(gameconfig.ReleaseRollback), From: ptr.Current, To: &target.ID, Reason: reason, By: p.UserID, At: now,
			IdempotencyKey: key}); err != nil {
			return err
		}
		blocks, err := h.Kernel.ReleaseBlocks(ctx, ContentItem{Type: t.ItemType, Payload: stored}, target.ID, target.VersionNo, stored, now)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(stored)
		payload := map[string]any{"item_type": t.ItemType, "item_id": it.ItemKey, "config_id": configID, "release_id": rid,
			"owner_scope": "platform", "tenant_id": nil, "from_version_id": ptr.Current, "to_version_id": target.ID,
			"to_version": target.VersionNo, "reason": reason, "released_by": p.UserID, "released_at": now,
			"snapshot_sha256": hex.EncodeToString(sum[:])}
		for k, b := range blocks {
			payload[k] = b
		}
		if err := h.Events.Publish(ctx, write.Event{Type: EventRolledBack, SubjectType: t.ItemType, SubjectID: it.ItemKey, Platform: true,
			OccurredAt: now, Payload: payload}); err != nil {
			return err
		}
		if err := h.record(ctx, ActionRolledBack, t, configID, reason, map[string]any{"from_version": from, "to_version": target.VersionNo,
			"release_id": rid}); err != nil {
			return err
		}
		to := target.VersionNo
		out = ReleaseView{ReleaseID: rid, ItemType: t.ItemType, ItemID: configID, Action: string(gameconfig.ReleaseRollback), FromVersion: from,
			ToVersion: &to, Reason: &reason, Status: string(gameconfig.ScheduleApplied), ReleasedBy: p.UserID, ReleasedAt: now}
		if h.Kernel.AfterCommit != nil && h.Kernel.Cache != nil {
			tid := target.ID
			_ = h.Kernel.AfterCommit(ctx, func(ctx context.Context) { _ = h.Kernel.Cache.SetCurrent(ctx, t.ItemType, it.ItemKey, tid) })
		}
		return idempotency.SetResult(ctx, out)
	})
	if err == nil {
		h.metric(configType, "rollback")
	}
	return out, err
}

func (h GameAdmin) versionNoOf(ctx context.Context, t gameconfig.Type, id *uuid.UUID) *int {
	if id == nil {
		return nil
	}
	raw, err := h.Store.TypeVersion(ctx, t.ItemType, *id)
	if err != nil || raw == nil {
		return nil
	}
	var doc map[string]any
	if json.Unmarshal(raw, &doc) != nil {
		return nil
	}
	if n, ok := doc["version"].(float64); ok {
		v := int(n)
		return &v
	}
	return nil
}

func releaseProblem(err error) error {
	switch {
	case errors.Is(err, gameconfig.ErrReasonRequired):
		return problem.Invalid("reason", problem.SubRequired, "a reason is required")
	case errors.Is(err, gameconfig.ErrReasonTooLong):
		return problem.Invalid("reason", problem.SubMaxLength, "at most 500 characters")
	case errors.Is(err, gameconfig.ErrRollbackNotAllowed):
		return problem.New(problem.RollbackTargetInvalid, "this config type does not follow the release pointer: publish a new version")
	case errors.Is(err, gameconfig.ErrNotPublished):
		return problem.New(problem.ContentNotPublished, "")
	}
	return problem.New(problem.RollbackTargetInvalid, "")
}

// Unpublish is unpublishGameConfig_game (F078-AC-14): the pointer becomes empty (the manifest builder removes the item
// from every tenant manifest on content.unpublished); pinned plays keep their version; the response lists the places
// that still reference the item.
func (h GameAdmin) Unpublish(ctx context.Context, configType string, configID uuid.UUID, reason string) (out ReleaseView, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.UnpublishGameConfig")
	defer func() { done(err) }()
	t, err := gameadmin.Type(configType)
	if err != nil {
		return out, err
	}
	p, sc, err := gameadmin.Principal(ctx)
	if err != nil {
		return out, err
	}
	if err := gameadmin.RequirePublish(ctx, p); err != nil {
		return out, err
	}
	now := h.now()
	key := idempotencyKey(ctx)
	err = h.UoW.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		it, err := h.Store.ItemByID(ctx, t.ItemType, configID)
		if err != nil {
			return err
		}
		if it == nil {
			return problem.New(problem.NotFound, "")
		}
		ptr, err := h.Store.Pointer(ctx, t.ItemType, it.ItemKey)
		if err != nil {
			return err
		}
		if err := gameconfig.CheckUnpublish(t, ptr.Current, reason); err != nil {
			return releaseProblem(err)
		}
		from := h.versionNoOf(ctx, t, ptr.Current)
		if err := h.Store.MovePointer(ctx, t.ItemType, it.ItemKey, nil, p.UserID, now); err != nil {
			return err
		}
		rid := ids.New()
		if key == "" {
			key = rid.String()
		}
		if err := h.Kernel.Store.Release(ctx, write.ContentRelease{ReleaseID: rid, ItemType: t.ItemType, ItemID: it.ItemKey,
			Action: string(gameconfig.ReleaseUnpublish), From: ptr.Current, Reason: reason, By: p.UserID, At: now, IdempotencyKey: key}); err != nil {
			return err
		}
		usages, err := h.Store.Usages(ctx, t.ItemType, it.ItemKey, configID, 500)
		if err != nil {
			return err
		}
		if err := h.Events.Publish(ctx, write.Event{Type: EventUnpublished, SubjectType: t.ItemType, SubjectID: it.ItemKey, Platform: true,
			OccurredAt: now, Payload: map[string]any{"item_type": t.ItemType, "item_id": it.ItemKey, "config_id": configID, "release_id": rid,
				"owner_scope": "platform", "tenant_id": nil, "from_version_id": ptr.Current, "to_version_id": nil, "reason": reason,
				"released_by": p.UserID, "released_at": now}}); err != nil {
			return err
		}
		if err := h.record(ctx, ActionUnpublished, t, configID, reason, map[string]any{"from_version": from, "release_id": rid,
			"usages": len(usages)}); err != nil {
			return err
		}
		out = ReleaseView{ReleaseID: rid, ItemType: t.ItemType, ItemID: configID, Action: string(gameconfig.ReleaseUnpublish), FromVersion: from,
			Reason: &reason, Status: string(gameconfig.ScheduleApplied), ReleasedBy: p.UserID, ReleasedAt: now, Usages: []UsageView{}}
		for _, u := range usages {
			out.Usages = append(out.Usages, UsageView(u))
		}
		if h.Kernel.AfterCommit != nil && h.Kernel.Cache != nil {
			_ = h.Kernel.AfterCommit(ctx, func(ctx context.Context) { _ = h.Kernel.Cache.ClearCurrent(ctx, t.ItemType, it.ItemKey) })
		}
		return idempotency.SetResult(ctx, out)
	})
	if err == nil {
		h.metric(configType, "unpublish")
	}
	return out, err
}

// PreviewToken is the createGamePreviewSession response.
type PreviewToken struct {
	Token     string    `json:"preview_token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// CreatePreview is createGamePreviewSession (F078-AC-17): a random token valid preview_token_ttl that reads only the
// given version, for its creator; only the SHA-256 of the token is stored. No game session, attempt, XP or learning
// event is created.
func (h GameAdmin) CreatePreview(ctx context.Context, configType string, configID uuid.UUID, no int) (out PreviewToken, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.CreateGamePreviewSession")
	defer func() { done(err) }()
	t, err := gameadmin.Type(configType)
	if err != nil {
		return out, err
	}
	p, sc, err := gameadmin.Principal(ctx)
	if err != nil {
		return out, err
	}
	if err := gameadmin.RequireRead(ctx, p, configType); err != nil {
		return out, err
	}
	ttl := h.Policy.PreviewTTL
	if ttl <= 0 {
		ttl = gameconfig.PreviewTTL
	}
	now := h.now()
	err = h.UoW.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		_, v, err := h.load(ctx, t, configID, no)
		if err != nil {
			return err
		}
		raw := make([]byte, 32)
		if _, err := rand.Read(raw); err != nil {
			return err
		}
		token := base64.RawURLEncoding.EncodeToString(raw)
		out = PreviewToken{Token: token, ExpiresAt: now.Add(ttl)}
		if err := h.Store.InsertPreview(ctx, ids.New(), gameconfig.HashToken(token), t.Name, configID, v.ID, v.VersionNo, p.UserID, now,
			out.ExpiresAt); err != nil {
			return err
		}
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// configView maps a version row to the contract.
func configView(t gameconfig.Type, v write.ConfigVersion, current bool) ConfigVersionView {
	var payload map[string]any
	_ = json.Unmarshal(v.Payload, &payload)
	if payload == nil {
		payload = map[string]any{}
	}
	return ConfigVersionView{ConfigType: t.Name, ConfigID: v.ConfigID, ConfigVersionID: v.ID, Version: v.VersionNo, Status: v.Status, Title: v.Title,
		Payload: payload, AuthorID: v.AuthorID, ReviewerID: v.ReviewerID, ReviewNote: v.ReviewNote, ChangeNote: v.ChangeNote, IsCurrent: current,
		PublishedAt: v.PublishedAt, UpdatedAt: v.UpdatedAt.UTC(), ETag: ETagOf(v.Revision)}
}
