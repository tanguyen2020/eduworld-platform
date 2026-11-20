package command

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/catalog"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/profile"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
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
	NicknameOptionID  *uuid.UUID
	Settings          map[string]any // nil: absent
	SetLanguage       bool
	PreferredLanguage *string
}

// UpdateProfile implements updateGameProfile (F070 US2): sets the nickname of a suggestion option the student
// received (409 NICKNAME_UNAVAILABLE when expired, of another student or taken meanwhile; 409 NICKNAME_CHANGE_LIMIT
// beyond nickname_change_monthly_limit), records the history and publishes game_profile.updated (F070-AC-08…11).
func (h Profiles) UpdateProfile(ctx context.Context, in ProfileUpdate) (out *profile.Profile, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.UpdateGameProfile")
	defer func() { done(err) }()
	pr, tenant, id, err := student(ctx)
	if err != nil {
		return nil, err
	}
	if in.SetLanguage && in.PreferredLanguage != nil && !h.supported(*in.PreferredLanguage) {
		return nil, problem.Invalid("preferred_language", problem.SubInvalid, "unsupported locale")
	}
	var nickname string
	if in.NicknameOptionID != nil {
		n, ok, err := h.Options.Get(ctx, id, *in.NicknameOptionID)
		if err != nil {
			return nil, problem.New(problem.Unavailable, "").Retry().Wrap(err)
		}
		if !ok {
			h.conflict()
			return nil, problem.New(problem.NicknameUnavailable, "")
		}
		nickname = n
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
		h.afterCommit(ctx, func(ctx context.Context) { _ = h.Options.Clear(ctx, id) })
		return nil
	})
	return out, err
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
