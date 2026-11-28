package command

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/catalog"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/profile"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// ProfileConfig is the configuration of the profile use cases (CONFIG_DEFAULTS emote_wheel_slots,
// saved_outfits_max, nickname_change_monthly_limit, nickname_suggestion_ttl); read by infrastructure/config.
type ProfileConfig struct {
	EmoteWheelSlots int
	SavedOutfitsMax int
	Nickname        profile.NicknamePolicy
	SuggestionTTL   time.Duration
	// SupportedLocales are the values accepted for preferred_language (supported_locales).
	SupportedLocales []string
	// NicknameRules are nickname_min_length and nickname_max_digits of a typed nickname (F136, ADR-0105).
	NicknameRules profile.NicknameRules
	// ScreenDeadline is nickname_moderation_deadline: the deadline of each facade call of a typed nickname.
	ScreenDeadline time.Duration
	// AttemptLimit and AttemptWindow are nickname_attempt_rate (typed submissions per window and student).
	AttemptLimit  int
	AttemptWindow time.Duration
}

// MaxSettingsBytes is the size limit of GameProfile.settings (CHECK octet_length <= 8192).
const MaxSettingsBytes = 8192

// Profiles are the write use cases of the game profile (F070): updateAvatar, updateGameProfile (nickname from a
// suggestion, settings, language), saveAvatarPreset. Each runs in one UnitOfWork with its outbox events.
type Profiles struct {
	UoW         write.UnitOfWork
	AfterCommit write.AfterCommit
	Repo        write.ProfileRepository
	Catalog     write.CatalogReader
	Owned       write.OwnershipReader
	Presets     write.PresetRepository
	Directory   write.Directory
	Schools     write.SchoolPolicy
	Options     port.NicknameOptionStore
	Events      write.EventPublisher
	Metrics     port.ProfileMetrics // nil: no metrics
	Config      ProfileConfig
	Now         func() time.Time
	// Audit records the nickname changes, resets and refusals (F136-AC-21, AC-30); nil = audit.Discard is refused at
	// wiring (the module always sets it).
	Audit write.Auditor
	// Moderation, Personal and Attempts check a typed nickname (F136 US3); nil = every typed nickname is 503 UNAVAILABLE
	// (fail-closed), the suggestion path keeps working.
	Moderation port.NicknameModeration
	Personal   port.NicknamePersonalDataScreen
	Attempts   port.NicknameAttemptCounter
	// NicknameMetrics records eduworld_nickname_checks_total (nil: no metrics).
	NicknameMetrics port.NicknameMetrics
}

func (h Profiles) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

// student returns the tenant and id of the calling student (the game_player guard already refused anyone else; the
// check is repeated so a use case is never reachable without it, R23).
func student(ctx context.Context) (authz.Principal, uuid.UUID, uuid.UUID, error) {
	p, err := principal(ctx)
	if err != nil {
		return p, uuid.Nil, uuid.Nil, err
	}
	if p.UserType != authz.UserTypeStudent || p.TenantID == nil {
		return p, uuid.Nil, uuid.Nil, problem.New(problem.GameAccessStudentOnly, "")
	}
	return p, *p.TenantID, p.UserID, nil
}

// load returns the live profile of the student, creating it first when the student never started a session
// (EnsureGameProfile without login time).
func (h Profiles) load(ctx context.Context, tenant, id uuid.UUID) (*profile.Profile, error) {
	p, err := h.Repo.Get(ctx, tenant, id)
	if err != nil || p != nil {
		return p, err
	}
	st, err := h.Directory.Student(ctx, tenant, id)
	if err != nil {
		return nil, err
	}
	if st == nil {
		return nil, problem.New(problem.Unavailable, "student not projected yet").Retry()
	}
	if !st.Playable() {
		return nil, problem.New(problem.GameAccessStudentOnly, "")
	}
	if err := h.Repo.Ensure(ctx, profile.New(tenant, id, st.SchoolID, h.now()), nil); err != nil {
		return nil, err
	}
	p, err = h.Repo.Get(ctx, tenant, id)
	if err == nil && p == nil {
		err = problem.New(problem.Conflict, "game profile was removed").Retry()
	}
	return p, err
}

func (h Profiles) reject(reason string) {
	if h.Metrics != nil {
		h.Metrics.AvatarRejected(reason)
	}
}

// avatarProblem maps the domain errors of an AvatarConfig to the contract (F070-AC-04, AC-13, AC-14).
func (h Profiles) avatarProblem(err error) error {
	var invalid *profile.InvalidError
	var notOwned *profile.NotOwnedError
	var slot *profile.SlotError
	switch {
	case errors.As(err, &invalid):
		h.reject(port.RejectInvalid)
		return problem.Invalid(invalid.Field, problem.SubInvalid, invalid.Reason)
	case errors.As(err, &notOwned):
		h.reject(port.RejectNotOwned)
		return problem.New(problem.ItemNotOwned, "").With("field", notOwned.Field)
	case errors.As(err, &slot):
		h.reject(port.RejectSlot)
		return problem.New(problem.EquipSlotInvalid, "").With("field", slot.Field)
	}
	return err
}

// validated checks cfg against the current catalog, the item definitions and the inventory of the student and
// returns the normalized config, its canonical JSON and hash (inside the transaction of ctx).
func (h Profiles) validated(ctx context.Context, tenant, id uuid.UUID, cfg profile.AvatarConfig, defaultEmotes bool) (*catalog.Catalog, profile.AvatarConfig, []byte, string, error) {
	cat, err := h.Catalog.Current(ctx)
	if err != nil {
		return nil, cfg, nil, "", err
	}
	if cat == nil {
		return nil, cfg, nil, "", problem.New(problem.Unavailable, "character catalog not published").Retry()
	}
	var defaults []string
	if defaultEmotes {
		defaults = cat.DefaultEmoteIDs
	}
	n := cfg.Normalized(defaults)
	rules := profile.Rules{EmoteWheelSlots: h.Config.EmoteWheelSlots}
	if err := n.CheckShape(rules); err != nil {
		return nil, n, nil, "", h.avatarProblem(err)
	}
	if err := n.CheckPalette(cat); err != nil {
		return nil, n, nil, "", h.avatarProblem(err)
	}
	codes := n.ItemIDs()
	items, err := h.Catalog.Items(ctx, codes)
	if err != nil {
		return nil, n, nil, "", err
	}
	var notDefault []string
	for _, c := range codes {
		if !cat.IsDefault(c) {
			notDefault = append(notDefault, c)
		}
	}
	inventory := map[string]bool{}
	if len(notDefault) > 0 {
		if inventory, err = h.Owned.Owned(ctx, tenant, id, notDefault); err != nil {
			return nil, n, nil, "", err
		}
	}
	owned := func(c string) bool { return cat.IsDefault(c) || inventory[c] }
	if err := n.CheckEquipment(items, owned); err != nil {
		return nil, n, nil, "", h.avatarProblem(err)
	}
	raw, err := n.Canonical()
	if err != nil {
		return nil, n, nil, "", err
	}
	if len(raw) > profile.MaxAvatarBytes {
		h.reject(port.RejectInvalid)
		return nil, n, nil, "", problem.Invalid("avatar_config", problem.SubMaxLength, "at most 16 KB")
	}
	hash, err := n.Hash()
	return cat, n, raw, hash, err
}

// presetOf returns the code of the catalog preset whose appearance equals cfg ("" when none).
func presetOf(cat *catalog.Catalog, hash string) string {
	for _, pr := range cat.Presets {
		if h, err := pr.AvatarConfig.Normalized(cat.DefaultEmoteIDs).Hash(); err == nil && h == hash {
			return pr.PresetCode
		}
	}
	return ""
}

// UpdateAvatar implements updateAvatar (F070 US1, US3): validates the config, saves it with its hash; the first avatar
// sets avatar_created_at once and publishes avatar.created, a different config publishes avatar.updated; both publish
// game_profile.updated in the same transaction (F070-AC-03, AC-15, AC-19). Absent emote_slots get the default emotes.
func (h Profiles) UpdateAvatar(ctx context.Context, cfg profile.AvatarConfig) (out profile.AvatarConfig, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.UpdateAvatar")
	defer func() { done(err) }()
	_, tenant, id, err := student(ctx)
	if err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		p, err := h.load(ctx, tenant, id)
		if err != nil {
			return err
		}
		if cfg.EmoteSlots == nil && p.Avatar != nil { // absent: keep the wheel of the current avatar
			cfg.EmoteSlots = append([]string{}, p.Avatar.EmoteSlots...)
		}
		cat, n, _, hash, err := h.validated(ctx, tenant, id, cfg, true)
		if err != nil {
			return err
		}
		now := h.now()
		change := p.SetAvatar(n, hash, now)
		out = n
		if change == profile.AvatarUnchanged {
			return nil
		}
		if err := h.Repo.Save(ctx, p); err != nil {
			return err
		}
		sid := id.String()
		if change == profile.AvatarCreated {
			payload := map[string]any{"student_id": sid, "avatar_config_hash": hash, "created_at": now}
			if code := presetOf(cat, hash); code != "" {
				payload["preset_code"] = code
			}
			if err := h.publish(ctx, tenant, "avatar.created", sid, now, payload); err != nil {
				return err
			}
			h.afterCommit(ctx, func(context.Context) {
				if h.Metrics != nil {
					h.Metrics.AvatarCreated()
				}
			})
		} else if err := h.publish(ctx, tenant, "avatar.updated", sid, now, map[string]any{"student_id": sid,
			"avatar_config_hash": hash, "updated_at": now}); err != nil {
			return err
		}
		return h.profileUpdated(ctx, p, now)
	})
	return out, err
}

func (h Profiles) publish(ctx context.Context, tenant uuid.UUID, typ, student string, at time.Time, payload map[string]any) error {
	return h.Events.Publish(ctx, write.Event{Type: typ, SubjectType: "student", SubjectID: student, PartitionKey: student,
		TenantID: &tenant, OccurredAt: at, Payload: payload})
}

// maxEventAvatarBytes is the size limit of avatar_config in game_profile.updated (≤ 4 KB for the hub, F091).
const maxEventAvatarBytes = 4096

// profileUpdated publishes game_profile.updated: only what other players may see — student_id, nickname, avatar,
// level — never a real name, username, school or class (F070-AC-12, R16); source_version = profile version.
func (h Profiles) profileUpdated(ctx context.Context, p *profile.Profile, at time.Time) error {
	payload := map[string]any{"student_id": p.StudentID.String(), "tenant_id": p.TenantID.String(), "nickname": p.Nickname,
		"avatar_config_hash": p.AvatarHash, "avatar_config": nil, "level": p.Level, "source_version": p.Version}
	if p.Avatar != nil {
		if raw, err := p.Avatar.Canonical(); err == nil && len(raw) <= maxEventAvatarBytes {
			payload["avatar_config"] = json.RawMessage(raw)
		}
	}
	return h.publish(ctx, p.TenantID, "game_profile.updated", p.StudentID.String(), at, payload)
}

func (h Profiles) afterCommit(ctx context.Context, fn func(ctx context.Context)) {
	if h.AfterCommit != nil {
		_ = h.AfterCommit(ctx, fn)
	}
}

// ProfileUpdate is the body of updateGameProfile (merge patch): the nickname only through a suggestion option
// (F070-AC-10), settings merged (null removes a key), preferred_language (null clears it).
type ProfileUpdate struct {
	// Nickname is a typed nickname (F136); exclusive with NicknameOptionID. SetNickname reports a present field (a JSON
	// null is refused).
	Nickname          *string
	SetNickname       bool
	NicknameOptionID  *uuid.UUID
	Settings          map[string]any // nil: absent
	SetLanguage       bool
	PreferredLanguage *string
}

// UpdateProfile implements updateGameProfile (F070 US2, F136 US1–US3): the nickname comes from exactly one of a
// suggestion option the student received (409 NICKNAME_UNAVAILABLE when expired, of another student or taken meanwhile)
// or a typed nickname checked by typedNickname (format, monthly limit, attempt rate, chat moderation, identity screen —
// fail-closed); 409 NICKNAME_CHANGE_LIMIT beyond nickname_change_monthly_limit; the history, game_profile.updated and the
// audit are written in the same transaction (F070-AC-08…11, F136-AC-03…AC-16).
func (h Profiles) UpdateProfile(ctx context.Context, in ProfileUpdate) (out *profile.Profile, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.UpdateGameProfile")
	defer func() { done(err) }()
	pr, tenant, id, err := student(ctx)
	if err != nil {
		return nil, err
	}
	if in.SetNickname && (in.Nickname == nil || in.NicknameOptionID != nil) {
		// nickname null, or both ways at once (F136-AC-03)
		return nil, problem.Invalid("nickname", problem.SubInvalid, "exactly one of nickname or nickname_option_id")
	}
	if in.SetLanguage && in.PreferredLanguage != nil && !h.supported(*in.PreferredLanguage) {
		return nil, problem.Invalid("preferred_language", problem.SubInvalid, "unsupported locale")
	}
	var nickname string
	typedText := false
	switch {
	case in.NicknameOptionID != nil:
		n, ok, err := h.Options.Get(ctx, id, *in.NicknameOptionID)
		if err != nil {
			return nil, problem.New(problem.Unavailable, "").Retry().Wrap(err)
		}
		if !ok {
			h.conflict()
			return nil, problem.New(problem.NicknameUnavailable, "")
		}
		nickname = n
	case in.Nickname != nil:
		n, unchanged, err := h.typedNickname(ctx, tenant, id, *in.Nickname)
		if err != nil {
			return nil, err
		}
		if !unchanged {
			nickname, typedText = n.String(), true
		}
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		p, err := h.load(ctx, tenant, id)
		if err != nil {
			return err
		}
		now := h.now()
		changed := false
		var change profile.NicknameChange
		if nickname != "" {
			change = p.SetNickname(nickname)
			if change.Changed && change.Reason == profile.ReasonPlayerChange {
				if err := h.checkLimit(ctx, p, now); err != nil {
					return err
				}
			}
			changed = change.Changed
		}
		if in.Settings != nil {
			merged := profile.MergeSettings(p.Settings, in.Settings)
			raw, err := json.Marshal(merged)
			if err != nil {
				return err
			}
			if len(raw) > MaxSettingsBytes {
				return problem.Invalid("settings", problem.SubMaxLength, "at most 8 KB")
			}
			p.Settings, changed = merged, true
		}
		if in.SetLanguage {
			p.PreferredLanguage, changed = in.PreferredLanguage, true
		}
		out = p
		if !changed {
			return nil
		}
		if err := h.Repo.Save(ctx, p); err != nil {
			if errors.Is(err, write.ErrNicknameTaken) {
				h.conflict()
				return problem.New(problem.NicknameUnavailable, "")
			}
			return err
		}
		if !change.Changed {
			return nil // settings or language only: nothing other players see
		}
		if err := h.Repo.AppendNicknameChange(ctx, write.NicknameChange{ID: ids.New(), TenantID: tenant, StudentID: id,
			Old: change.Old, New: change.New, Reason: change.Reason, ChangedBy: pr.UserID, ChangedAt: now}); err != nil {
			return err
		}
		if err := h.profileUpdated(ctx, p, now); err != nil {
			return err
		}
		if err := h.audit(ctx, audit.Event{Action: AuditNicknameChanged, TargetType: "student", TargetID: id.String(),
			Result: audit.ResultSuccess, SchoolID: p.SchoolID.String(), Metadata: map[string]any{"old_nickname": change.Old,
				"new_nickname": change.New, "reason": change.Reason, "source": nicknameSource(typedText)}}); err != nil {
			return err
		}
		h.afterCommit(ctx, func(ctx context.Context) { _ = h.Options.Clear(ctx, id) })
		return nil
	})
	if err != nil && typedText {
		var pe *problem.Error
		if errors.As(err, &pe) && pe.Code == problem.NicknameUnavailable {
			h.refused(ctx, id, port.NicknameTaken)
		}
		return out, err
	}
	if err == nil && typedText {
		h.check(port.NicknameAllowed)
	}
	return out, err
}

func nicknameSource(typed bool) string {
	if typed {
		return "typed"
	}
	return "suggestion"
}

func (h Profiles) audit(ctx context.Context, e audit.Event) error {
	if h.Audit == nil {
		return nil
	}
	return h.Audit.Record(ctx, e)
}

func (h Profiles) check(result string) {
	if h.NicknameMetrics != nil {
		h.NicknameMetrics.NicknameCheck(result)
	}
}

// refused counts and audits a refused typed nickname with its result code only — never the text (F136-AC-30, R14). The
// audit is best effort: the refusal is returned to the student whatever happens to it.
func (h Profiles) refused(ctx context.Context, student uuid.UUID, result string) {
	h.check(result)
	_ = h.UoW.Do(ctx, func(ctx context.Context) error {
		return h.audit(ctx, audit.Event{Action: AuditNicknameRefused, TargetType: "student", TargetID: student.String(),
			Result: audit.ResultDenied, Metadata: map[string]any{"result": result}})
	})
}

// notAllowed is 422 NICKNAME_NOT_ALLOWED: one code for every content reason, no field, no position, no echo (R15).
func (h Profiles) notAllowed(ctx context.Context, student uuid.UUID, result string) error {
	h.refused(ctx, student, result)
	return problem.New(problem.NicknameNotAllowed, "")
}

func (h Profiles) unavailable(cause error) error {
	h.check(port.NicknameUnavailable)
	return problem.New(problem.Unavailable, "").Retry().Wrap(cause)
}

// typedNickname runs the checks of a typed nickname outside the write transaction (no facade call inside a
// UnitOfWork, constitution II), in the order of F136 Clarifications: attempt counter (every typed submission, 429 before
// any facade call — AC-15), format (AC-10), monthly limit of a player change (409 before moderation — AC-08), chat
// moderation (AC-11), identity screen (AC-12). A facade or counter failure is 503 and nothing is stored (fail-closed,
// AC-14). unchanged reports the current nickname typed again (nothing to check or store, F070 behaviour). Uniqueness is
// the unique index on save (AC-13).
func (h Profiles) typedNickname(ctx context.Context, tenant, student uuid.UUID, raw string) (n profile.Nickname, unchanged bool, err error) {
	if h.Attempts == nil {
		return n, false, h.unavailable(errors.New("game: nickname attempt counter not wired"))
	}
	allowed, wait, err := h.Attempts.Hit(ctx, tenant, student, h.now())
	if err != nil {
		return n, false, h.unavailable(err)
	}
	if !allowed {
		h.check(port.NicknameRateLimited)
		return n, false, RetryAfter{Err: problem.New(problem.RateLimited, "").Retry(), After: wait}
	}
	n, err = profile.ParseNickname(raw, h.Config.NicknameRules)
	if err != nil {
		return n, false, h.notAllowed(ctx, student, port.NicknameFormat)
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		p, err := h.load(ctx, tenant, student)
		if err != nil {
			return err
		}
		if p.Nickname != nil && *p.Nickname == n.String() {
			unchanged = true
			return nil
		}
		if p.Nickname != nil { // a player change (the first nickname and the one after a reset are not counted)
			return h.checkLimit(ctx, p, h.now())
		}
		return nil
	})
	if err != nil || unchanged {
		return n, unchanged, err
	}
	if ok, err := h.screen(ctx, "chat", func(ctx context.Context) (bool, error) {
		if h.Moderation == nil {
			return false, port.ErrNicknameScreenUnavailable
		}
		return h.Moderation.CheckNickname(ctx, tenant, student, n.String())
	}); err != nil {
		return n, false, h.unavailable(err)
	} else if !ok {
		return n, false, h.notAllowed(ctx, student, port.NicknameModerated)
	}
	if ok, err := h.screen(ctx, "identity", func(ctx context.Context) (bool, error) {
		if h.Personal == nil {
			return false, port.ErrNicknameScreenUnavailable
		}
		return h.Personal.ScreenNickname(ctx, tenant, student, n.String())
	}); err != nil {
		return n, false, h.unavailable(err)
	} else if !ok {
		return n, false, h.notAllowed(ctx, student, port.NicknamePersonal)
	}
	return n, false, nil
}

// screen runs one facade check under nickname_moderation_deadline and records its latency.
func (h Profiles) screen(ctx context.Context, facade string, fn func(ctx context.Context) (bool, error)) (bool, error) {
	if d := h.Config.ScreenDeadline; d > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, d)
		defer cancel()
	}
	start := time.Now()
	ok, err := fn(ctx)
	if h.NicknameMetrics != nil {
		h.NicknameMetrics.NicknameScreenLatency(facade, time.Since(start))
	}
	if err == nil && ctx.Err() != nil {
		err = ctx.Err() // a late answer is no answer (fail-closed)
	}
	return ok, err
}

func (h Profiles) conflict() {
	if h.Metrics != nil {
		h.Metrics.NicknameConflict()
	}
}

func (h Profiles) supported(locale string) bool {
	for _, l := range h.Config.SupportedLocales {
		if l == locale {
			return true
		}
	}
	return false
}

// checkLimit refuses a player change beyond nickname_change_monthly_limit (the stricter of the platform value and
// the school policy) with the time a change is allowed again (F070-AC-11).
func (h Profiles) checkLimit(ctx context.Context, p *profile.Profile, now time.Time) error {
	school, err := h.Schools.NicknameChangeLimit(ctx, p.TenantID, p.SchoolID)
	if err != nil {
		return err
	}
	policy := h.Config.Nickname.Effective(school)
	n, err := h.Repo.PlayerChangesSince(ctx, p.TenantID, p.StudentID, policy.MonthStart(now))
	if err != nil {
		return err
	}
	if !policy.Allows(n) {
		return problem.New(problem.NicknameChangeLimit, "").With("retry_after", policy.NextMonth(now).UTC())
	}
	return nil
}

// PresetResult is the saved outfit returned by saveAvatarPreset.
type PresetResult struct {
	Slot         int
	Name         *string
	AvatarConfig profile.AvatarConfig
	UpdatedAt    time.Time
}

// SavePreset implements saveAvatarPreset (F070-AC-16): slot 1..saved_outfits_max, the config checked like
// updateAvatar (ownership, slots); the slot is overwritten.
func (h Profiles) SavePreset(ctx context.Context, slot int, name *string, cfg profile.AvatarConfig) (out PresetResult, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.SaveAvatarPreset")
	defer func() { done(err) }()
	_, tenant, id, err := student(ctx)
	if err != nil {
		return out, err
	}
	if slot < 1 || slot > h.Config.SavedOutfitsMax {
		return out, problem.Invalid("slot", problem.SubInvalid, "outside 1..saved_outfits_max")
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if _, err := h.load(ctx, tenant, id); err != nil {
			return err
		}
		_, n, raw, _, err := h.validated(ctx, tenant, id, cfg, true)
		if err != nil {
			return err
		}
		now := h.now()
		if err := h.Presets.Upsert(ctx, tenant, id, slot, name, raw, now); err != nil {
			return err
		}
		out = PresetResult{Slot: slot, Name: name, AvatarConfig: n, UpdatedAt: now}
		return nil
	})
	return out, err
}

// Audit actions of the nickname (F136-AC-21, AC-30).
const (
	AuditNicknameChanged = "game_profile.nickname_changed"
	AuditNicknameRefused = "game_profile.nickname_refused"
	AuditNicknameReset   = "game_profile.nickname_reset"
	// AuditNicknameHistoryAnonymized: the history of a student anonymized (anonymizeUserData, F136-AC-31).
	AuditNicknameHistoryAnonymized = "game_profile.nickname_history_anonymized"
)

// SanctionReset is a nickname_reset sanction of module chat (sanction.applied, F136-AC-20).
type SanctionReset struct {
	TenantID   uuid.UUID
	StudentID  uuid.UUID
	SanctionID uuid.UUID
	IssuedBy   *uuid.UUID
	At         time.Time
}

// ResetResult is StudentNicknameResetResult of resetStudentNickname.
type ResetResult struct {
	StudentID uuid.UUID `json:"student_id"`
	Nickname  *string   `json:"nickname"`
	ResetAt   time.Time `json:"reset_at"`
}

// Sources of a nickname reset (audit metadata).
const (
	ResetBySchool     = "school_admin"
	ResetByModeration = "moderation"
)

// nicknameResetCovered: a grant holding lms.student.nickname_reset without a school limit, or limited to school.
func nicknameResetCovered(p authz.Principal, school uuid.UUID) bool {
	for _, g := range p.Grants {
		for _, perm := range g.Permissions {
			if perm == authz.PermLmsStudentNicknameReset && (g.SchoolID == nil || *g.SchoolID == school) {
				return true
			}
		}
	}
	return false
}

// ResetStudentNickname implements resetStudentNickname (F136-AC-21): lms.student.nickname_reset (authz.Require, 403 for
// a role without it, a student or a platform role outside its tenant permissions — R20) in the tenant of the session; a
// student of another tenant or of a school outside the caller's grants is 404 NOT_FOUND. The nickname becomes NULL
// (setup_required), the history gets "reset", game_profile.updated and the audit (actor, reason, old nickname) are written
// in the same transaction; the result is stored for the Idempotency-Key replay. A student without nickname is 200
// without change.
func (h Profiles) ResetStudentNickname(ctx context.Context, student uuid.UUID, reason string) (out ResetResult, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.ResetStudentNickname")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	if p.TenantID == nil || p.UserType == authz.UserTypeStudent {
		return out, problem.New(problem.PermissionDenied, "")
	}
	tenant := *p.TenantID
	if !p.Has(authz.PermLmsStudentNicknameReset) {
		// no grant holds the permission: 403 PERMISSION_DENIED, audited by Require
		return out, authz.Require(ctx, p, authz.PermLmsStudentNicknameReset, authz.Resource{TenantID: &tenant})
	}
	if n := utf8.RuneCountInString(strings.TrimSpace(reason)); n < 3 || n > 500 {
		return out, problem.Invalid("reason", problem.SubInvalid, "3 to 500 characters")
	}
	now := h.now()
	out = ResetResult{StudentID: student, ResetAt: now}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		st, err := h.Directory.Student(ctx, tenant, student)
		if err != nil {
			return err
		}
		if st == nil || st.Deleted || !nicknameResetCovered(p, st.SchoolID) {
			return problem.New(problem.NotFound, "")
		}
		prof, err := h.Repo.Get(ctx, tenant, student)
		if err != nil {
			return err
		}
		if prof != nil {
			if _, err := h.reset(ctx, prof, p.UserID, ResetBySchool, reason, nil, now); err != nil {
				return err
			}
		}
		return idempotency.SetResult(ctx, out)
	})
	if err != nil {
		return ResetResult{}, err
	}
	return out, nil
}

// ResetNicknameBySanction applies sanction.applied of type nickname_reset (consumer game.sanction_nickname_reset,
// F136-AC-20) in the transaction of the consumer: once per sanction_id (the history row keeps the sanction as
// source_ref_id, so a redelivery after the student chose a new nickname changes nothing), changed_by = the moderator
// (uuid.Nil for a system sanction). A student without a profile has nothing to reset.
func (h Profiles) ResetNicknameBySanction(ctx context.Context, in SanctionReset) (err error) {
	ctx, done := observability.UseCase(ctx, "game.command.ResetNicknameBySanction")
	defer func() { done(err) }()
	if in.TenantID == uuid.Nil || in.StudentID == uuid.Nil || in.SanctionID == uuid.Nil {
		return problem.New(problem.Internal, "nickname reset needs tenant, student and sanction")
	}
	at := in.At
	if at.IsZero() {
		at = h.now()
	}
	actor := uuid.Nil
	if in.IssuedBy != nil {
		actor = *in.IssuedBy
	}
	return h.UoW.Do(ctx, func(ctx context.Context) error {
		done, err := h.Repo.ResetBySource(ctx, in.TenantID, in.StudentID, in.SanctionID)
		if err != nil || done {
			return err
		}
		prof, err := h.Repo.Get(ctx, in.TenantID, in.StudentID)
		if err != nil || prof == nil {
			return err
		}
		ref := in.SanctionID
		changed, err := h.reset(ctx, prof, actor, ResetByModeration, "", &ref, at)
		if err != nil || changed {
			return err
		}
		// No nickname to reset: the sanction is still recorded, so it is never applied to a nickname chosen later.
		return h.Repo.AppendNicknameChange(ctx, write.NicknameChange{ID: ids.New(), TenantID: in.TenantID, StudentID: in.StudentID,
			New: "", Reason: profile.ReasonReset, ChangedBy: actor, ChangedAt: at, SourceRefID: &ref})
	})
}

// reset clears the nickname of p inside the transaction of ctx: history "reset" (new_nickname empty — the F070 column is
// NOT NULL), game_profile.updated with nickname null (other players see the neutral ui.player.unnamed) and the audit.
func (h Profiles) reset(ctx context.Context, p *profile.Profile, actor uuid.UUID, source, reason string, ref *uuid.UUID,
	at time.Time) (bool, error) {
	old, changed := p.ResetNickname()
	if !changed {
		return false, nil
	}
	if err := h.Repo.Save(ctx, p); err != nil {
		return false, err
	}
	if err := h.Repo.AppendNicknameChange(ctx, write.NicknameChange{ID: ids.New(), TenantID: p.TenantID, StudentID: p.StudentID,
		Old: old, New: "", Reason: profile.ReasonReset, ChangedBy: actor, ChangedAt: at, SourceRefID: ref}); err != nil {
		return false, err
	}
	if err := h.profileUpdated(ctx, p, at); err != nil {
		return false, err
	}
	meta := map[string]any{"old_nickname": *old, "source": source}
	if reason != "" {
		meta["reason"] = reason
	}
	if ref != nil {
		meta["sanction_id"] = ref.String()
	}
	if actor != uuid.Nil {
		meta["changed_by"] = actor.String()
	}
	if err := h.audit(ctx, audit.Event{Action: AuditNicknameReset, TargetType: "student", TargetID: p.StudentID.String(),
		Result: audit.ResultSuccess, SchoolID: p.SchoolID.String(), Metadata: meta}); err != nil {
		return false, err
	}
	sid := p.StudentID
	h.afterCommit(ctx, func(ctx context.Context) { _ = h.Options.Clear(ctx, sid) })
	return true, nil
}
