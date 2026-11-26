package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/gameadmin"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/gameconfig"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// EventConfigPublished is the release event of a competition configuration (owner F108, schema
// specs/events/payloads/competition/competition_config.published.v1).
const EventConfigPublished = "competition_config.published"

// Audit actions of the competition configurations (F108-AC-07, plan "Bảo mật").
const (
	AuditConfigCreated      = "competition_config.created"
	AuditConfigUpdated      = "competition_config.updated"
	AuditConfigTransitioned = "competition_config.transitioned"
	AuditConfigPublished    = "competition_config.published"
	AuditConfigRolledBack   = "competition_config.rolled_back"
	AuditConfigUnpublished  = "competition_config.unpublished"
)

// GameConfigMetrics observes eduworld_competition_config_publish_total{config_type} and
// eduworld_competition_config_validation_failed_total{config_type}; nil = not observed.
type GameConfigMetrics interface {
	Published(configType string)
	ValidationFailed(configType string)
}

// GameConfigs are the write use cases of the competition configurations (F108 US1): drafts, review workflow, publish
// with effective_from, rollback and unpublish of the seven competition config types, platform only (R20), versioned and
// immutable once published (R11, ADR-0043).
type GameConfigs struct {
	UoW     *entx.UnitOfWork
	Store   write.GameConfigStore
	Schemas port.ConfigSchemas
	Events  write.EventPublisher
	Audit   write.Auditor
	Metrics GameConfigMetrics
	Now     func() time.Time
}

// ConfigDraft is the input of createGameConfig and updateGameConfigDraft (GameConfigInput).
type ConfigDraft struct {
	Title   *string
	Payload json.RawMessage
	// ConfigID + BasedOn (createGameConfig only): a new draft version of an existing configuration.
	ConfigID *uuid.UUID
	BasedOn  *uuid.UUID
}

func (h GameConfigs) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

// ifMatchRev compares an If-Match header with the revision (428 when missing, 412 VERSION_MISMATCH when stale —
// F108-AC-06).
func ifMatchRev(header string, revision int64) error {
	v := strings.TrimSpace(header)
	if v == "" {
		return problem.New(problem.PreconditionRequired, "")
	}
	v = strings.TrimPrefix(v, "W/")
	if v != "*" && v != gameadmin.ETagOf(revision) && v != strconv.FormatInt(revision, 10) {
		return problem.New(problem.VersionMismatch, "")
	}
	return nil
}

func decodeConfigPayload(raw json.RawMessage) (map[string]any, error) {
	if len(raw) > gameconfig.MaxPayloadBytes {
		return nil, problem.Invalid("payload", problem.SubMaxLength, "at most 65536 bytes")
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil || doc == nil {
		return nil, problem.Invalid("payload", problem.SubInvalid, "an object is required")
	}
	return doc, nil
}

func checkConfigTitle(title *string) error {
	if title != nil && utf8.RuneCountInString(*title) > gameconfig.MaxTitle {
		return problem.Invalid("title", problem.SubMaxLength, "at most 200 characters")
	}
	return nil
}

// violations turns broken rules into 422 VALIDATION_FAILED naming every field (at most 50).
func violations(vs []gameconfig.Violation) error {
	pe := problem.New(problem.ValidationFailed, "the configuration breaks its schema or a competition rule")
	for i, v := range vs {
		if i == 50 {
			break
		}
		pe.AddField(v.Field, v.Code, v.Message)
	}
	return pe
}

// validate normalizes the payload and checks it against the JSON Schema of the type, then the business invariants
// (F108-AC-04). It returns the JSON to store.
func (h GameConfigs) validate(t gameconfig.Type, doc map[string]any, code string) ([]byte, error) {
	t.Normalize(doc, code)
	b, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	if len(b) > gameconfig.MaxPayloadBytes {
		return nil, problem.Invalid("payload", problem.SubMaxLength, "at most 65536 bytes")
	}
	if h.Schemas == nil {
		return nil, problem.New(problem.Unavailable, "the config schemas are not loaded").Retry()
	}
	vs, err := h.Schemas.Validate(t.Name, b)
	if err != nil {
		return nil, err
	}
	if len(vs) == 0 {
		vs = gameconfig.Check(t, b)
	}
	if len(vs) > 0 {
		if h.Metrics != nil {
			h.Metrics.ValidationFailed(t.Name)
		}
		return nil, violations(vs)
	}
	return b, nil
}

// checkSeasons checks a seasons payload against the seasons in place (edge case: overlap with the active season, no
// retroactive season — F108).
func (h GameConfigs) checkSeasons(ctx context.Context, t gameconfig.Type, payload []byte, effective time.Time) error {
	if t.Name != gameconfig.TypeSeasons {
		return nil
	}
	existing, err := h.Store.SeasonStates(ctx)
	if err != nil {
		return err
	}
	if vs := gameconfig.CheckSeasonChanges(payload, existing, effective); len(vs) > 0 {
		if h.Metrics != nil {
			h.Metrics.ValidationFailed(t.Name)
		}
		return violations(vs)
	}
	return nil
}

func (h GameConfigs) record(ctx context.Context, action string, t gameconfig.Type, it port.ConfigItem, reason string, meta map[string]any) error {
	if meta == nil {
		meta = map[string]any{}
	}
	meta["config_type"], meta["code"] = t.Name, it.Code
	return h.Audit.Record(ctx, audit.Event{Action: action, TargetType: "competition_config", TargetID: it.ConfigID.String(),
		Result: audit.ResultSuccess, Reason: reason, Metadata: meta})
}

func (h GameConfigs) load(ctx context.Context, t gameconfig.Type, configID uuid.UUID, no int) (*port.ConfigItem, *port.ConfigVersion, error) {
	it, err := h.Store.ItemByID(ctx, t.Name, configID)
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

// codeOf is the code of a new configuration: default for a singleton, the payload code of a template or reward set
// (else one derived from the config id).
func codeOf(t gameconfig.Type, doc map[string]any, configID uuid.UUID) string {
	if t.Singleton {
		return gameconfig.DefaultCode
	}
	if s, ok := doc["code"].(string); ok && s != "" {
		return s
	}
	return "cfg_" + strings.ReplaceAll(configID.String(), "-", "")[20:]
}

// Create is createGameConfig_competition (F108-AC-01, AC-04): without config_id a new configuration with version 1
// draft; with config_id (+ based_on_version_id) a new draft version of an existing configuration — the source version
// never changes (R11, Clarifications 2026-11-16).
func (h GameConfigs) Create(ctx context.Context, configType string, in ConfigDraft) (out gameadmin.ConfigVersionView, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.CreateGameConfig")
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
	if err := checkConfigTitle(in.Title); err != nil {
		return out, err
	}
	if in.BasedOn != nil && in.ConfigID == nil {
		return out, problem.Invalid("config_id", problem.SubRequired, "based_on_version_id needs config_id")
	}
	doc, err := decodeConfigPayload(in.Payload)
	if err != nil {
		return out, err
	}
	now := h.now()
	err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		var it port.ConfigItem
		var basedOn *port.ConfigVersion
		action := AuditConfigCreated
		if in.ConfigID != nil {
			found, err := h.Store.ItemByID(ctx, t.Name, *in.ConfigID)
			if err != nil {
				return err
			}
			if found == nil {
				return problem.New(problem.NotFound, "")
			}
			it = *found
			if in.BasedOn != nil {
				if basedOn, err = h.Store.VersionByID(ctx, it.ConfigID, *in.BasedOn); err != nil {
					return err
				}
				if basedOn == nil {
					return problem.Invalid("based_on_version_id", problem.SubInvalid, "not a version of this configuration")
				}
			}
			action = AuditConfigUpdated
		} else {
			it = port.ConfigItem{ConfigID: ids.New(), Type: t.Name}
			it.Code = codeOf(t, doc, it.ConfigID)
			if existing, err := h.Store.ItemByCode(ctx, t.Name, it.Code); err != nil {
				return err
			} else if existing != nil {
				return problem.New(problem.Conflict, "the configuration exists: create a new version of it (config_id)")
			}
			if err := h.Store.CreateItem(ctx, it, p.UserID, now); err != nil {
				if errors.Is(err, write.ErrConfigConflict) {
					return problem.New(problem.Conflict, "the configuration exists")
				}
				return err
			}
		}
		b, err := h.validate(t, doc, it.Code)
		if err != nil {
			return err
		}
		no, err := h.Store.NextVersionNo(ctx, it.ConfigID, p.UserID, now)
		if err != nil {
			return err
		}
		author := p.UserID
		v := port.ConfigVersion{ID: ids.New(), ConfigID: it.ConfigID, No: no, Status: string(gameconfig.StatusDraft), Payload: b, AuthorID: &author,
			UpdatedAt: now, Revision: 1}
		switch {
		case in.Title != nil:
			v.Title = *in.Title
		case basedOn != nil:
			v.Title = basedOn.Title
		}
		var basedNo *int
		if basedOn != nil {
			v.BasedOn = &basedOn.ID
			n := basedOn.No
			basedNo = &n
		}
		if err := h.Store.InsertVersion(ctx, v, p.UserID, now); err != nil {
			return err
		}
		meta := map[string]any{"version": no}
		if basedNo != nil {
			meta["based_on_version"] = *basedNo
		}
		if err := h.record(ctx, action, t, it, "", meta); err != nil {
			return err
		}
		out = gameadmin.VersionView(t, it, v, basedNo)
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// UpdateDraft is updateGameConfigDraft_competition: If-Match required (428), stale ETag 412, only a draft or rejected
// version is editable — anything else (published, in review…) is 409 CONTENT_NOT_EDITABLE (F108-AC-05, AC-06).
func (h GameConfigs) UpdateDraft(ctx context.Context, configType string, configID uuid.UUID, no int, match string, in ConfigDraft) (out gameadmin.ConfigVersionView, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.UpdateGameConfigDraft")
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
	if err := checkConfigTitle(in.Title); err != nil {
		return out, err
	}
	doc, err := decodeConfigPayload(in.Payload)
	if err != nil {
		return out, err
	}
	now := h.now()
	err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		it, v, err := h.load(ctx, t, configID, no)
		if err != nil {
			return err
		}
		if err := ifMatchRev(match, v.Revision); err != nil {
			return err
		}
		if !gameconfig.Editable(gameconfig.Status(v.Status)) {
			return problem.New(problem.ContentNotEditable, "")
		}
		b, err := h.validate(t, doc, it.Code)
		if err != nil {
			return err
		}
		var before, after map[string]any
		_ = json.Unmarshal(v.Payload, &before)
		_ = json.Unmarshal(b, &after)
		next := *v
		next.Payload, next.Status = b, string(gameconfig.StatusDraft)
		if in.Title != nil {
			next.Title = *in.Title
		}
		ok, err := h.Store.UpdateDraft(ctx, next, v.Revision, p.UserID, now)
		if err != nil {
			return err
		}
		if !ok {
			return problem.New(problem.VersionMismatch, "")
		}
		changes, truncated := gameconfig.Diff(before, after)
		if err := h.record(ctx, AuditConfigUpdated, t, *it, "", map[string]any{"version": v.No, "changes": len(changes), "truncated": truncated}); err != nil {
			return err
		}
		next.Revision, next.UpdatedAt = v.Revision+1, now
		out = gameadmin.VersionView(t, *it, next, nil)
		return nil
	})
	return out, err
}

// Transition is transitionGameConfigVersion_competition (F108-AC-02, AC-06, AC-07): submit_review/archive need the write
// permission of the type, approve/reject game.content.review and a reviewer other than the author; If-Match required;
// each transition is audited.
func (h GameConfigs) Transition(ctx context.Context, configType string, configID uuid.UUID, no int, match, action, note string) (out gameadmin.ConfigVersionView, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.TransitionGameConfigVersion")
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
		return out, problem.Invalid("action", problem.SubInvalid, "submit_review, approve, reject or archive (publish has its own operation)")
	}
	if err != nil {
		return out, err
	}
	now := h.now()
	err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		it, v, err := h.load(ctx, t, configID, no)
		if err != nil {
			return err
		}
		if err := ifMatchRev(match, v.Revision); err != nil {
			return err
		}
		cur := it.Current != nil && *it.Current == v.ID
		to, err := gameconfig.Transition(gameconfig.Version{Status: gameconfig.Status(v.Status), Author: v.AuthorID, IsCurrent: cur},
			gameconfig.Request{Action: a, Actor: p.UserID, Note: note})
		if err != nil {
			return configTransitionProblem(ctx, p, err)
		}
		if a == gameconfig.ActionSubmitReview {
			// The stored payload is checked again: a schema or rule may have changed since it was saved.
			var doc map[string]any
			if err := json.Unmarshal(v.Payload, &doc); err != nil {
				return err
			}
			if _, err := h.validate(t, doc, it.Code); err != nil {
				return err
			}
		}
		next := *v
		next.Status = string(to)
		if note != "" {
			next.ReviewNote = &note
		}
		if a == gameconfig.ActionApprove || a == gameconfig.ActionReject {
			reviewer := p.UserID
			next.ReviewerID = &reviewer
		}
		ok, err := h.Store.SetWorkflow(ctx, next, v.Revision, p.UserID, now)
		if err != nil {
			return err
		}
		if !ok {
			return problem.New(problem.VersionMismatch, "")
		}
		if err := h.record(ctx, AuditConfigTransitioned, t, *it, note, map[string]any{"version": v.No, "action": action, "from": v.Status,
			"to": string(to)}); err != nil {
			return err
		}
		next.Revision, next.UpdatedAt = v.Revision+1, now
		out = gameadmin.VersionView(t, *it, next, nil)
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

func configTransitionProblem(ctx context.Context, p authz.Principal, err error) error {
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

// PublishInput is the body of publishGameConfigVersion (PublishRequest).
type PublishInput struct {
	PublishAt     *time.Time
	EffectiveFrom *time.Time
	ChangeNote    string
}

// Publish is publishGameConfigVersion_competition (F108-AC-05, AC-07, AC-08): an approved version, If-Match and
// Idempotency-Key required, game.content.publish. effective_from ≥ now (absent = now; never retroactive); the payload is
// checked again; the version is published, the pointer moves, a content_releases row and competition_config.published
// are written in the same transaction; the runtime applies the version from effective_from.
func (h GameConfigs) Publish(ctx context.Context, configType string, configID uuid.UUID, no int, match string, in PublishInput) (out gameadmin.ReleaseView, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.PublishGameConfig")
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
	if in.PublishAt != nil {
		return out, problem.Invalid("publish_at", problem.SubInvalid, "competition configurations take effect at effective_from (no scheduled publish)")
	}
	if utf8.RuneCountInString(in.ChangeNote) > gameconfig.MaxChangeNote {
		return out, problem.Invalid("change_note", problem.SubMaxLength, "at most 500 characters")
	}
	now := h.now()
	effective, err := gameconfig.EffectiveFrom(in.EffectiveFrom, now)
	if err != nil {
		return out, problem.Invalid("effective_from", problem.SubInvalid, "must not be in the past (no retroactive change)")
	}
	err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		it, v, err := h.load(ctx, t, configID, no)
		if err != nil {
			return err
		}
		if err := ifMatchRev(match, v.Revision); err != nil {
			return err
		}
		if err := gameconfig.CheckPublish(gameconfig.Status(v.Status)); err != nil {
			return problem.New(problem.InvalidStateTransition, "only an approved version can be published")
		}
		var doc map[string]any
		if err := json.Unmarshal(v.Payload, &doc); err != nil {
			return err
		}
		if _, err := h.validate(t, doc, it.Code); err != nil {
			return err
		}
		if err := h.checkSeasons(ctx, t, v.Payload, effective); err != nil {
			return err
		}
		next := *v
		by := p.UserID
		next.Status, next.EffectiveFrom, next.PublishedAt, next.PublishedBy = string(gameconfig.StatusPublished), &effective, &now, &by
		if in.ChangeNote != "" {
			note := in.ChangeNote
			next.ChangeNote = &note
		}
		ok, err := h.Store.Publish(ctx, next, v.Revision, p.UserID, now)
		if err != nil {
			return err
		}
		if !ok {
			return problem.New(problem.VersionMismatch, "")
		}
		out, err = h.release(ctx, t, *it, gameconfig.ReleasePublish, &next, in.ChangeNote, effective, p.UserID, now)
		if err != nil {
			return err
		}
		if h.Metrics != nil {
			h.Metrics.Published(t.Name)
		}
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// release moves the pointer, appends the release row, publishes competition_config.published and records the audit, in
// the caller transaction.
func (h GameConfigs) release(ctx context.Context, t gameconfig.Type, it port.ConfigItem, action gameconfig.ReleaseAction, to *port.ConfigVersion,
	reason string, effective time.Time, by uuid.UUID, now time.Time) (gameadmin.ReleaseView, error) {
	var toID *uuid.UUID
	if to != nil {
		toID = &to.ID
	}
	if err := h.Store.MovePointer(ctx, it.ConfigID, toID, by, now); err != nil {
		return gameadmin.ReleaseView{}, err
	}
	r := port.ConfigRelease{ID: ids.New(), ConfigID: it.ConfigID, Action: string(action), From: it.Current, To: toID, By: by, At: now, EffectiveFrom: effective}
	if reason != "" {
		r.Reason = &reason
	}
	if err := h.Store.InsertRelease(ctx, r); err != nil {
		return gameadmin.ReleaseView{}, err
	}
	nos := map[uuid.UUID]int{}
	var fromNo *int
	if it.Current != nil {
		if prev, err := h.Store.VersionByID(ctx, it.ConfigID, *it.Current); err != nil {
			return gameadmin.ReleaseView{}, err
		} else if prev != nil {
			nos[prev.ID] = prev.No
			n := prev.No
			fromNo = &n
		}
	}
	payload := map[string]any{"config_type": t.Name, "config_id": it.ConfigID, "code": it.Code, "config_version_id": nil, "version": nil,
		"previous_version_id": it.Current, "action": string(action), "effective_from": effective, "release_id": r.ID, "released_by": by,
		"reason": r.Reason, "content_refs": []any{}}
	if to != nil {
		nos[to.ID] = to.No
		payload["config_version_id"], payload["version"] = to.ID, to.No
	}
	if err := h.Events.Publish(ctx, write.Event{Type: EventConfigPublished, SubjectType: "competition_config", SubjectID: it.ConfigID.String(),
		PartitionKey: it.ConfigID.String(), OccurredAt: now, Platform: true, Payload: payload}); err != nil {
		return gameadmin.ReleaseView{}, err
	}
	auditAction := map[gameconfig.ReleaseAction]string{gameconfig.ReleasePublish: AuditConfigPublished, gameconfig.ReleaseRollback: AuditConfigRolledBack,
		gameconfig.ReleaseUnpublish: AuditConfigUnpublished}[action]
	meta := map[string]any{"release_id": r.ID, "effective_from": effective, "from_version": fromNo}
	if to != nil {
		meta["version"] = to.No
	}
	if err := h.record(ctx, auditAction, t, it, reason, meta); err != nil {
		return gameadmin.ReleaseView{}, err
	}
	return gameadmin.ReleaseOf(t.Name, r, nos), nil
}

// Rollback is rollbackGameConfig_competition (F108-AC-05, AC-07): a reason is required; the pointer moves to a version of
// the configuration published before (else 422 ROLLBACK_TARGET_INVALID), effective at once; no version is created or
// changed (ADR-0043).
func (h GameConfigs) Rollback(ctx context.Context, configType string, configID uuid.UUID, toVersion int, reason string) (out gameadmin.ReleaseView, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.RollbackGameConfig")
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
	err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		it, err := h.Store.ItemByID(ctx, t.Name, configID)
		if err != nil {
			return err
		}
		if it == nil {
			return problem.New(problem.NotFound, "")
		}
		v, err := h.Store.Version(ctx, configID, toVersion)
		if err != nil {
			return err
		}
		var tg *gameconfig.Target
		if v != nil {
			tg = &gameconfig.Target{ID: v.ID, Status: gameconfig.Status(v.Status), Published: v.PublishedAt != nil}
		}
		if err := gameconfig.CheckRollback(it.Current, tg, reason); err != nil {
			return configReleaseProblem(err)
		}
		if err := h.checkSeasons(ctx, t, v.Payload, now); err != nil {
			return err
		}
		out, err = h.release(ctx, t, *it, gameconfig.ReleaseRollback, v, reason, now, p.UserID, now)
		if err != nil {
			return err
		}
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// Unpublish is unpublishGameConfig_competition (F108-AC-05, AC-07): a reason is required; only the templates and the
// reward sets can be withdrawn (the runtime catalogs cannot — Clarifications 2026-11-25); the pointer is cleared,
// tournaments already created keep their pinned version.
func (h GameConfigs) Unpublish(ctx context.Context, configType string, configID uuid.UUID, reason string) (out gameadmin.ReleaseView, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.UnpublishGameConfig")
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
	err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		it, err := h.Store.ItemByID(ctx, t.Name, configID)
		if err != nil {
			return err
		}
		if it == nil {
			return problem.New(problem.NotFound, "")
		}
		if err := gameconfig.CheckUnpublish(t, it.Current, reason); err != nil {
			return configReleaseProblem(err)
		}
		out, err = h.release(ctx, t, *it, gameconfig.ReleaseUnpublish, nil, reason, now, p.UserID, now)
		if err != nil {
			return err
		}
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

func configReleaseProblem(err error) error {
	switch {
	case errors.Is(err, gameconfig.ErrReasonRequired):
		return problem.Invalid("reason", problem.SubRequired, "a reason is required")
	case errors.Is(err, gameconfig.ErrReasonTooLong):
		return problem.Invalid("reason", problem.SubMaxLength, "at most 500 characters")
	case errors.Is(err, gameconfig.ErrRollbackTarget):
		return problem.New(problem.RollbackTargetInvalid, "the target must be another version published before")
	case errors.Is(err, gameconfig.ErrUnpublishNotAllowed):
		return problem.New(problem.InvalidStateTransition, "this catalog cannot be withdrawn: roll back or publish a new version")
	case errors.Is(err, gameconfig.ErrNotPublished):
		return problem.New(problem.ContentNotPublished, "")
	}
	return fmt.Errorf("release: %w", err)
}
