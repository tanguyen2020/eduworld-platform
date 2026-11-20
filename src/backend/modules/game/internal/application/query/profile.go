package query

import (
	"context"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/catalog"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/profile"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Profile are the read use cases of the game profile (F070): getGameProfile, getCharacterCatalog,
// suggestNicknames, listAvatarPresets. They read in a READ ONLY session and never write the database;
// suggestNicknames keeps its options in the temporary Valkey store (ADR-0004), not in PostgreSQL.
type Profile struct {
	Read     read.Session
	Profiles read.ProfileReadModel
	Catalog  read.CatalogReadModel
	Options  port.NicknameOptionStore
	// TTL is nickname_suggestion_ttl (CONFIG_DEFAULTS, 10 min).
	TTL time.Duration
	// Rand draws the nickname words (nil: math/rand/v2 seeded per call).
	Rand profile.Rand
}

func studentOf(ctx context.Context) (uuid.UUID, uuid.UUID, error) {
	p, err := principal(ctx)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	if p.UserType != authz.UserTypeStudent || p.TenantID == nil {
		return uuid.Nil, uuid.Nil, problem.New(problem.GameAccessStudentOnly, "")
	}
	return *p.TenantID, p.UserID, nil
}

// GameProfile is the response of getGameProfile.
type GameProfile struct {
	read.Profile
	SetupRequired bool
}

// GetGameProfile implements getGameProfile (F070-AC-01): the own profile of the student; setup_required while the
// avatar or the nickname is missing. A student without a profile row yet (no game session started) gets the empty
// profile of level 1 with setup_required.
func (q Profile) GetGameProfile(ctx context.Context) (out GameProfile, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.GetGameProfile")
	defer func() { done(err) }()
	tenant, id, err := studentOf(ctx)
	if err != nil {
		return out, err
	}
	var p *read.Profile
	if err = q.Read.Read(ctx, func(ctx context.Context) error {
		p, err = q.Profiles.Profile(ctx, tenant, id)
		return err
	}); err != nil {
		return out, err
	}
	if p == nil {
		p = &read.Profile{StudentID: id, Level: 1}
	}
	out = GameProfile{Profile: *p, SetupRequired: p.AvatarCreatedAt == nil || len(p.AvatarConfig) == 0 || p.Nickname == nil}
	return out, nil
}

func (q Profile) current(ctx context.Context) (*catalog.Catalog, error) {
	var c *catalog.Catalog
	err := q.Read.Read(ctx, func(ctx context.Context) error {
		var err error
		c, err = q.Catalog.Current(ctx)
		return err
	})
	if err == nil && c == nil {
		err = problem.New(problem.Unavailable, "character catalog not published").Retry()
	}
	return c, err
}

// GetCharacterCatalog implements getCharacterCatalog (F070-AC-02): the published catalog; the nickname words stay on
// the server (suggestions are generated there).
func (q Profile) GetCharacterCatalog(ctx context.Context) (c *catalog.Catalog, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.GetCharacterCatalog")
	defer func() { done(err) }()
	if _, _, err = studentOf(ctx); err != nil {
		return nil, err
	}
	return q.current(ctx)
}

// Suggestion is one nickname option.
type Suggestion struct {
	OptionID uuid.UUID
	Nickname string
}

// Default and maximum count of suggestNicknames (contract: default 6, maximum 10).
const (
	DefaultSuggestions = 6
	MaxSuggestions     = 10
	suggestRounds      = 6
)

// SuggestNicknames implements suggestNicknames (F070-AC-07): count nicknames generated from the approved words of the
// catalog in the language of Accept-Language, at most 24 characters, not used by a live profile of the tenant; each
// option is valid nickname_suggestion_ttl for this student only.
func (q Profile) SuggestNicknames(ctx context.Context, count int, acceptLanguage string) (out []Suggestion, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.SuggestNicknames")
	defer func() { done(err) }()
	tenant, id, err := studentOf(ctx)
	if err != nil {
		return nil, err
	}
	if count <= 0 {
		count = DefaultSuggestions
	}
	if count > MaxSuggestions {
		count = MaxSuggestions
	}
	r := q.Rand
	if r == nil {
		r = rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())) //nolint:gosec // nickname words, not a secret
	}
	gen := profile.NicknameGenerator{Rand: r}
	lang := profile.Language(acceptLanguage)
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		c, err := q.Catalog.Current(ctx)
		if err != nil {
			return err
		}
		if c == nil {
			return problem.New(problem.Unavailable, "character catalog not published").Retry()
		}
		words := c.Words(lang)
		tried := map[string]bool{}
		// A few rounds of fresh candidates: a tenant where most combinations are taken still gets free names.
		for round := 0; round < suggestRounds && len(out) < count; round++ {
			names := gen.Candidates(lang, words, (count-len(out))*8, count*80, func(s string) bool { return !tried[strings.ToLower(s)] })
			if len(names) == 0 {
				break
			}
			for _, n := range names {
				tried[strings.ToLower(n)] = true
			}
			taken, err := q.Profiles.TakenNicknames(ctx, tenant, names)
			if err != nil {
				return err
			}
			for _, n := range names {
				if len(out) == count {
					break
				}
				if !taken[strings.ToLower(n)] {
					out = append(out, Suggestion{OptionID: ids.New(), Nickname: n})
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	options := make(map[uuid.UUID]string, len(out))
	for _, s := range out {
		options[s.OptionID] = s.Nickname
	}
	if err := q.Options.Save(ctx, id, options, q.TTL); err != nil {
		return nil, problem.New(problem.Unavailable, "").Retry().Wrap(err)
	}
	return out, nil
}

// ListAvatarPresets implements listAvatarPresets (F070-AC-16): the saved outfits of the student only.
func (q Profile) ListAvatarPresets(ctx context.Context) (out []read.Preset, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.ListAvatarPresets")
	defer func() { done(err) }()
	tenant, id, err := studentOf(ctx)
	if err != nil {
		return nil, err
	}
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		out, err = q.Profiles.Presets(ctx, tenant, id)
		return err
	})
	return out, err
}
