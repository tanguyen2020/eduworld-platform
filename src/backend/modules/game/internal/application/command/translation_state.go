package command

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/gameadmin"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/gameconfig"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/i18ntext"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// saveTranslationState computes the translation state of v (F082-AC-08, CONTENT_MODEL §8) in the transaction that saves
// it: prev is the payload v replaces (the same draft before the save, or the version it was copied from) and prevID the
// version holding the meta to inherit; the state is stored with the version and set on the view.
func (h GameAdmin) saveTranslationState(ctx context.Context, t gameconfig.Type, v write.ConfigVersion, prev []byte, prevID *uuid.UUID,
	now time.Time, out *ConfigVersionView) error {
	if h.Translations == nil {
		return nil
	}
	st, err := computeState(ctx, h.Translations, v, prev, prevID, h.Policy.RequiredLanguages, now)
	if err != nil {
		return err
	}
	if err := h.Translations.SaveState(ctx, write.TranslationState{VersionID: v.ID, ConfigID: v.ConfigID, ConfigType: t.Name, State: st,
		ComputedAt: now}); err != nil {
		return err
	}
	if out != nil {
		applyState(out, st)
	}
	return nil
}

func computeState(ctx context.Context, store write.TranslationStore, v write.ConfigVersion, prev []byte, prevID *uuid.UUID, required []string,
	now time.Time) (i18ntext.State, error) {
	var doc, before map[string]any
	if err := json.Unmarshal(v.Payload, &doc); err != nil {
		return i18ntext.State{}, fmt.Errorf("translation state: %w", err)
	}
	if len(prev) > 0 {
		if err := json.Unmarshal(prev, &before); err != nil {
			return i18ntext.State{}, fmt.Errorf("translation state: %w", err)
		}
	}
	var meta map[string]i18ntext.Meta
	if prevID != nil {
		old, err := store.State(ctx, *prevID)
		if err != nil {
			return i18ntext.State{}, err
		}
		if old != nil {
			meta = old.State.Meta
		}
	}
	return i18ntext.Compute(i18ntext.Input{Doc: doc, Prev: before, PrevMeta: meta, Required: required, Version: v.VersionNo,
		VersionID: v.ID.String(), Now: now}), nil
}

// applyState sets translation_status, missing_fields and outdated_fields of a version view.
func applyState(out *ConfigVersionView, st i18ntext.State) {
	s := st.Status
	out.TranslationStatus = &s
	out.MissingFields = append([]string{}, st.Missing...)
	out.OutdatedFields = make([]gameadmin.OutdatedField, 0, len(st.Outdated))
	for _, o := range st.Outdated {
		out.OutdatedFields = append(out.OutdatedFields, gameadmin.OutdatedField{Pointer: o.Pointer, TranslatedInVersion: o.TranslatedInVersion})
	}
}

// TranslationBackfill is the job game-translation-state-backfill (Clarifications 2026-11-26): console versions saved
// without a translation state (seed files, the release core, versions older than F082) get one, oldest first; the first
// computation takes existing translations as current. Idempotent; platform job scope.
type TranslationBackfill struct {
	UoW      write.UnitOfWork
	Store    write.TranslationStore
	Required []string
	Batch    int
	Now      func() time.Time
}

// Run computes one batch and returns the number of versions done.
func (h TranslationBackfill) Run(ctx context.Context) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.TranslationBackfill")
	defer func() { done(err) }()
	sc, err := authz.SystemPlatformScope("game-translation-state-backfill")
	if err != nil {
		return 0, err
	}
	batch := h.Batch
	if batch <= 0 {
		batch = 200
	}
	now := time.Now().UTC()
	if h.Now != nil {
		now = h.Now().UTC()
	}
	err = h.UoW.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		vs, err := h.Store.VersionsWithoutState(ctx, batch)
		if err != nil {
			return err
		}
		for _, v := range vs {
			t, ok := gameconfig.ByItemType(v.ItemType)
			if !ok {
				continue
			}
			// The meta of the source version is inherited when it has one; no previous payload: present texts are current.
			st, err := computeState(ctx, h.Store, v, nil, v.BasedOn, h.Required, now)
			if err != nil {
				return err
			}
			if err := h.Store.SaveState(ctx, write.TranslationState{VersionID: v.ID, ConfigID: v.ConfigID, ConfigType: t.Name, State: st,
				ComputedAt: now}); err != nil {
				return err
			}
			n++
		}
		return nil
	})
	return n, err
}
