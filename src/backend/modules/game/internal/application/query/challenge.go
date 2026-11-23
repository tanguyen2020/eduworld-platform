package query

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/challengerules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/challenge"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/quest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Challenges answers the challenge read routes of a student (F076): listGameChallenges, getGameChallenge and
// getGameChallengeAttempt. Read only: what the student may see is computed from the published challenges assigned to
// the tenant, its grade (never a class, ADR-0095), the server facts of the unlock rules and its attempts.
type Challenges struct {
	Read    read.Session
	Content port.ChallengeContent
	World   port.WorldContent
	Quests  port.QuestContent
	Cursors *page.Codec
	Rules   challenge.Rules
	Reset   quest.Reset
	Now     func() time.Time
}

func (h Challenges) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

// ChallengeRewardSummary is RewardGrantSummary.
type ChallengeRewardSummary struct {
	Kind         string `json:"kind"`
	Amount       *int   `json:"amount,omitempty"`
	CurrencyCode string `json:"currency_code,omitempty"`
	ItemID       string `json:"item_id,omitempty"`
}

// ChallengeSummary is GameChallengeSummary.
type ChallengeSummary struct {
	ChallengeID           uuid.UUID      `json:"challenge_id"`
	ChallengeType         string         `json:"challenge_type"`
	Title                 challenge.Text `json:"title"`
	AreaID                string         `json:"area_id,omitempty"`
	SubjectID             string         `json:"subject_id,omitempty"`
	AvailableDifficulties []string       `json:"available_difficulties"`
	PartyRequired         bool           `json:"party_required"`
	AvailableTo           *time.Time     `json:"available_to,omitempty"`
	AttemptsLeft          int            `json:"attempts_left"`
}

// ChallengeDetail is GameChallengeDetail.
type ChallengeDetail struct {
	ChallengeSummary
	Description       string                   `json:"description,omitempty"`
	Version           int                      `json:"version"`
	TimeLimitSeconds  *int                     `json:"time_limit_seconds,omitempty"`
	CooldownUntil     *time.Time               `json:"cooldown_until,omitempty"`
	FirstClearRewards []ChallengeRewardSummary `json:"first_clear_rewards"`
}

// ChallengeListInput is the filter of listGameChallenges.
type ChallengeListInput struct {
	Type       string
	AreaID     string
	Difficulty string
	Limit      int
	Cursor     string
}

// ChallengePage is the listGameChallenges response.
type ChallengePage struct {
	Data []ChallengeSummary `json:"data"`
	Page PageInfo           `json:"page"`
}

func (h Challenges) student(ctx context.Context) (uuid.UUID, uuid.UUID, *port.WorldStudent, error) {
	p, err := principal(ctx)
	if err != nil {
		return uuid.Nil, uuid.Nil, nil, err
	}
	if p.UserType != authz.UserTypeStudent || p.TenantID == nil {
		return uuid.Nil, uuid.Nil, nil, problem.New(problem.GameAccessStudentOnly, "")
	}
	st, err := h.World.Student(ctx, *p.TenantID, p.UserID)
	if err != nil {
		return uuid.Nil, uuid.Nil, nil, err
	}
	if st == nil || st.Deleted || st.Status != "active" {
		return uuid.Nil, uuid.Nil, nil, problem.New(problem.NotFound, "student not found")
	}
	return p.UserID, *p.TenantID, st, nil
}

// facts loads the server facts of the unlock rules once, when a challenge needs them.
type facts struct {
	h      Challenges
	tenant uuid.UUID
	id     uuid.UUID
	st     *port.WorldStudent
	loaded *challenge.Student
}

func (f *facts) of(ctx context.Context, v challenge.Version) (challenge.Student, error) {
	if v.Unlock == nil {
		return challenge.Student{Grade: f.st.Grade}, nil
	}
	if f.loaded == nil {
		s, err := challengerules.Student(ctx, f.h.World, f.h.Quests, f.tenant, f.id, f.st, true)
		if err != nil {
			return s, err
		}
		f.loaded = &s
	}
	return *f.loaded, nil
}

// summary returns the view of a row for the student (nil when not available: grade, unlock, window, no level).
func (h Challenges) summary(ctx context.Context, f *facts, row port.ChallengeRow, now time.Time) (*ChallengeSummary, *port.ChallengeUsage, error) {
	v := row.Version
	if !v.Solo() {
		return nil, nil, nil
	}
	cs, err := f.of(ctx, v)
	if err != nil {
		return nil, nil, err
	}
	if ok, _ := v.Visible(cs, now); !ok {
		return nil, nil, nil
	}
	levels := h.Rules.Difficulties(v, f.st.Grade)
	if len(levels) == 0 {
		return nil, nil, nil
	}
	use, err := h.Content.Usage(ctx, f.tenant, f.id, row.ChallengeID, challengerules.PeriodKey(v, h.Reset, now))
	if err != nil {
		return nil, nil, err
	}
	_, to := v.Window()
	s := &ChallengeSummary{ChallengeID: row.ChallengeID, ChallengeType: v.Type, Title: v.Name, AreaID: v.AreaID, SubjectID: v.SubjectID,
		AvailableDifficulties: levels, PartyRequired: v.Mode == challenge.ModePartyRequired, AvailableTo: to,
		AttemptsLeft: max(0, h.Rules.MaxAttempts(v)-use.Attempts)}
	return s, &use, nil
}

// ListChallenges implements listGameChallenges (F076-AC-01): the published challenges assigned to the tenant for the
// grade of the student, playable solo, open now, unlocked, with the levels of its grade and the attempts left. Keyset
// pages in the database (challenge_id, limit + 1 rows per read); a read whose rows are not all visible reads on from
// its last row until the page is full or the challenges end — never everything read then sliced.
func (h Challenges) ListChallenges(ctx context.Context, in ChallengeListInput) (out ChallengePage, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.ListGameChallenges")
	defer func() { done(err) }()
	if in.Type != "" && !slices.Contains(challenge.SoloTypes, in.Type) && !slices.Contains([]string{challenge.TypeBoss, challenge.TypeCoop,
		challenge.TypeParty, challenge.TypeTournament}, in.Type) {
		return out, problem.Invalid("type", problem.SubInvalid, "a challenge type")
	}
	if in.Difficulty != "" && challenge.Rank(in.Difficulty) < 0 {
		return out, problem.Invalid("difficulty", problem.SubInvalid, "a difficulty level")
	}
	if len(in.AreaID) > 64 {
		return out, problem.Invalid("area_id", problem.SubInvalid, "at most 64 characters")
	}
	limit, err := limitOf(in.Limit)
	if err != nil {
		return out, err
	}
	var after *uuid.UUID
	if in.Cursor != "" {
		if len(in.Cursor) > 512 || h.Cursors == nil {
			return out, problem.Invalid("cursor", problem.SubInvalid, "malformed cursor")
		}
		c, err := h.Cursors.Decode(in.Cursor)
		if err != nil || c == nil {
			return out, problem.Invalid("cursor", problem.SubInvalid, "malformed cursor")
		}
		after = &c.ID
	}
	now := h.now()
	out = ChallengePage{Data: []ChallengeSummary{}, Page: PageInfo{Limit: limit}}
	err = h.Read.Read(ctx, func(ctx context.Context) error {
		id, tenant, st, err := h.student(ctx)
		if err != nil {
			return err
		}
		f := &facts{h: h, tenant: tenant, id: id, st: st}
		for {
			rows, err := h.Content.AvailableChallenges(ctx, port.ChallengeFilter{Tenant: tenant, Grade: st.Grade, Type: in.Type,
				AreaID: in.AreaID, Now: now, After: after, Limit: limit + 1})
			if err != nil {
				return err
			}
			for _, row := range rows {
				s, _, err := h.summary(ctx, f, row, now)
				if err != nil {
					return err
				}
				if s == nil || (in.Difficulty != "" && !slices.Contains(s.AvailableDifficulties, in.Difficulty)) {
					continue
				}
				if len(out.Data) == limit {
					out.Page.HasMore = true
					if h.Cursors != nil {
						c := h.Cursors.Encode(page.Cursor{Key: "challenge", ID: out.Data[len(out.Data)-1].ChallengeID})
						out.Page.NextCursor = &c
					}
					return nil
				}
				out.Data = append(out.Data, *s)
			}
			if len(rows) <= limit {
				return nil
			}
			last := rows[len(rows)-1].ChallengeID
			after = &last
		}
	})
	return out, err
}

// GetChallenge implements getGameChallenge (F076-AC-02): 404 for a challenge not available to the student (another
// grade, locked, outside its window, not assigned, not playable solo); else the attempts left today/period and the end
// of the cooldown after a failure.
func (h Challenges) GetChallenge(ctx context.Context, challengeID uuid.UUID) (out ChallengeDetail, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.GetGameChallenge")
	defer func() { done(err) }()
	now := h.now()
	err = h.Read.Read(ctx, func(ctx context.Context) error {
		id, tenant, st, err := h.student(ctx)
		if err != nil {
			return err
		}
		notFound := problem.New(problem.NotFound, "challenge not found")
		if ok, err := h.Content.Assigned(ctx, tenant, challengeID); err != nil || !ok {
			if err != nil {
				return err
			}
			return notFound
		}
		row, err := h.Content.CurrentChallenge(ctx, challengeID)
		if err != nil {
			return err
		}
		if row == nil {
			return notFound
		}
		s, use, err := h.summary(ctx, &facts{h: h, tenant: tenant, id: id, st: st}, *row, now)
		if err != nil {
			return err
		}
		if s == nil {
			return notFound
		}
		v := row.Version
		out = ChallengeDetail{ChallengeSummary: *s, Version: row.VersionNo, FirstClearRewards: []ChallengeRewardSummary{}}
		if v.Description != nil {
			out.Description = v.Description.VI
			if lang, _ := ctx.Value(langKey{}).(string); lang == "en" {
				out.Description = v.Description.EN
			}
		}
		if d := h.Rules.Deadline(v, st.Grade, now); d != nil {
			secs := int(d.Sub(now) / time.Second)
			out.TimeLimitSeconds = &secs
		}
		if use.LastFailedAt != nil {
			if until := use.LastFailedAt.Add(h.Rules.Cooldown(v, use.LastFailedOn)); now.Before(until) {
				out.CooldownUntil = &until
			}
		}
		for _, r := range v.Rewards.FirstClear {
			rs := ChallengeRewardSummary{Kind: r.Type, CurrencyCode: r.CurrencyCode}
			if r.Amount > 0 {
				n := r.Amount
				rs.Amount = &n
			}
			for _, ref := range []string{r.ItemID, r.BadgeID, r.CollectionItemID, r.TitleCode} {
				if ref != "" {
					rs.ItemID = ref
				}
			}
			out.FirstClearRewards = append(out.FirstClearRewards, rs)
		}
		return nil
	})
	return out, err
}

type langKey struct{}

// WithLanguage carries the Accept-Language of the request (vi default) to the read use cases.
func WithLanguage(ctx context.Context, lang string) context.Context {
	return context.WithValue(ctx, langKey{}, lang)
}

// GetChallengeAttempt implements getGameChallengeAttempt (F076-AC-15): the state of the checkpoint row (score, lives,
// activities, deadline) from any device; an attempt of another student is 404 (F076-AC-03).
func (h Challenges) GetChallengeAttempt(ctx context.Context, attemptID uuid.UUID) (out challengerules.AttemptView, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.GetGameChallengeAttempt")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	if p.UserType != authz.UserTypeStudent || p.TenantID == nil {
		return out, problem.New(problem.GameAccessStudentOnly, "")
	}
	err = h.Read.Read(ctx, func(ctx context.Context) error {
		a, err := h.Content.Attempt(ctx, *p.TenantID, attemptID)
		if err != nil {
			return err
		}
		if a == nil || a.StudentID != p.UserID {
			return problem.New(problem.NotFound, "challenge attempt not found")
		}
		row, err := h.Content.ChallengeVersion(ctx, a.VersionID)
		if err != nil {
			return err
		}
		no := 1
		if row != nil {
			no = row.VersionNo
		}
		out = challengerules.ViewOf(a, no)
		return nil
	})
	return out, err
}
