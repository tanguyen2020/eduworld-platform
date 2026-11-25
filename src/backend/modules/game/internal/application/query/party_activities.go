package query

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/partyrules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/partyquest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/quest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// MaxPartyCandidates is maxItems of PartyActivityCandidates.data (the database returns at most this many per type).
const MaxPartyCandidates = 50

// PartyActivities answers listPartyActivityCandidates and checkPartyActivityEligibility of the game facade (caller
// module competition, F103-AC-02): the quests and bosses a party may play and, per member, eligible + reason code —
// never the grade or school of a student (R16, ADR-0095). Read only.
type PartyActivities struct {
	Read           read.Session
	Bosses         port.BossContent
	BossReads      port.BossReads
	Party          port.PartyQuestContent
	Quests         port.QuestContent
	World          port.WorldContent
	AttemptsPerDay int
	Reset          quest.Reset
	Now            func() time.Time
}

func (h PartyActivities) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

// members reads the members in tenant; a student of another tenant (or unknown) is 422 VALIDATION_FAILED (F103-AC-26).
func (h PartyActivities) members(ctx context.Context, tenant uuid.UUID, ids []uuid.UUID) ([]partyrules.Member, error) {
	if len(ids) == 0 || len(ids) > 8 {
		return nil, problem.Invalid("/members", problem.SubInvalid, "1 to 8 members")
	}
	ms, err := partyrules.Members(ctx, h.World, h.Quests, tenant, ids)
	if err != nil {
		return nil, err
	}
	for _, m := range ms {
		if m.Student == nil {
			return nil, problem.Invalid("/members", problem.SubInvalid, "a member is not a student of the tenant of the call")
		}
	}
	return ms, nil
}

func (h PartyActivities) bossActivity(ctx context.Context, tenant uuid.UUID, row port.BossRow, ms []partyrules.Member) (partyrules.Activity, error) {
	v := row.Version
	a := partyrules.Activity{Type: "boss", Ref: v.Code, Title: map[string]string{"vi": v.Name.VI, "en": v.Name.EN}, AreaID: v.AreaID,
		MinMembers: v.SizeMin(), MaxMembers: v.SizeMax(), AllowContinue: true}
	period := partyrules.BossPeriod(h.Reset, h.now())
	for _, m := range ms {
		use, err := h.BossReads.Usage(ctx, tenant, m.ID, row.BossID, period)
		if err != nil {
			return a, err
		}
		a.Checks = append(a.Checks, partyrules.BossCheck(v, m, use, h.AttemptsPerDay))
	}
	return a, nil
}

func (h PartyActivities) questActivity(row port.QuestRow, ms []partyrules.Member) partyrules.Activity {
	v := row.Version
	a := partyrules.Activity{Type: "quest", Ref: v.Code, Title: map[string]string{"vi": v.Name.VI, "en": v.Name.EN}, AreaID: v.AreaID,
		MinMembers: 1, MaxMembers: 8, AllowContinue: v.PartyMode != partyquest.PartyRequired}
	if v.PartySize != nil {
		a.MinMembers, a.MaxMembers = v.PartySize.Min, v.PartySize.Max
	}
	at := h.now()
	for _, m := range ms {
		if hs := m.Hist[row.QuestID]; m.Valid() && hs != nil && hs.Latest != nil && hs.Latest.Active() {
			a.Checks = append(a.Checks, partyrules.Check{StudentID: m.ID, Eligible: true})
			continue
		}
		a.Checks = append(a.Checks, partyrules.QuestCheck(v, m, h.Reset, row.QuestID, at))
	}
	return a
}

// Candidates implements listPartyActivityCandidates: the party quests then the party bosses assigned to the tenant, of
// the area when given (at most MaxPartyCandidates, limited in the database), with the checks of every member.
func (h PartyActivities) Candidates(ctx context.Context, tenant uuid.UUID, areaID string, ids []uuid.UUID) (out []partyrules.Activity, err error) {
	ctx, done := observability.UseCase(ctx, "game.facade.ListPartyActivityCandidates")
	defer func() { done(err) }()
	if len(areaID) > 64 {
		return nil, problem.Invalid("/area_id", problem.SubMaxLength, "at most 64 characters")
	}
	err = h.Read.Read(ctx, func(ctx context.Context) error {
		ms, err := h.members(ctx, tenant, ids)
		if err != nil {
			return err
		}
		qs, err := h.Party.PartyQuests(ctx, tenant, areaID, MaxPartyCandidates)
		if err != nil {
			return err
		}
		for _, q := range qs {
			out = append(out, h.questActivity(q, ms))
		}
		left := MaxPartyCandidates - len(out)
		if left <= 0 {
			return nil
		}
		bs, err := h.Bosses.PartyBosses(ctx, tenant, areaID, left)
		if err != nil {
			return err
		}
		for _, b := range bs {
			a, err := h.bossActivity(ctx, tenant, b, ms)
			if err != nil {
				return err
			}
			out = append(out, a)
		}
		return nil
	})
	return out, err
}

// errUnknownActivity is an activity_ref unknown, unpublished or not assigned to the tenant (404).
var errUnknownActivity = errors.New("party activity unknown")

// Check implements checkPartyActivityEligibility (F103-AC-02): one quest or boss by code with min/max members,
// allow_continue_on_disconnect and the checks of every member (404 for an unknown activity).
func (h PartyActivities) Check(ctx context.Context, tenant uuid.UUID, typ, ref string, ids []uuid.UUID) (out partyrules.Activity, err error) {
	ctx, done := observability.UseCase(ctx, "game.facade.CheckPartyActivityEligibility")
	defer func() { done(err) }()
	if !slices.Contains([]string{"quest", "boss"}, typ) {
		return out, problem.Invalid("/activity_type", problem.SubInvalid, "module game serves quest and boss")
	}
	err = h.Read.Read(ctx, func(ctx context.Context) error {
		ms, err := h.members(ctx, tenant, ids)
		if err != nil {
			return err
		}
		if typ == "boss" {
			row, err := h.Bosses.BossByCode(ctx, ref)
			if err != nil {
				return err
			}
			if row == nil || !row.Version.Party() {
				return errUnknownActivity
			}
			if ok, err := h.Bosses.BossAssigned(ctx, tenant, row.BossID); err != nil || !ok {
				if err == nil {
					err = errUnknownActivity
				}
				return err
			}
			out, err = h.bossActivity(ctx, tenant, *row, ms)
			return err
		}
		row, err := h.Party.QuestByCode(ctx, ref)
		if err != nil {
			return err
		}
		if row == nil || row.Version.PartyMode == "" || row.Version.PartyMode == partyquest.PartySolo {
			return errUnknownActivity
		}
		if ok, err := h.Quests.Assigned(ctx, tenant, row.QuestID); err != nil || !ok {
			if err == nil {
				err = errUnknownActivity
			}
			return err
		}
		out = h.questActivity(*row, ms)
		return nil
	})
	if errors.Is(err, errUnknownActivity) {
		return out, problem.New(problem.NotFound, "party activity not found")
	}
	return out, err
}

// QuestClues answers getQuestStepClues (F103-AC-21, AC-24): the clue cards a member may read on an interdependent step —
// its roles and the shared cards in a party session, every role for a solo player — with their render from module
// learning (never a card not shared, never an answer).
type QuestClues struct {
	Read    read.Session
	Reads   port.PartyQuestReads
	Content port.QuestContent
	Renders port.ClueRenders
	Names   port.NicknameReader
}

// GetClues implements getQuestStepClues.
func (h QuestClues) GetClues(ctx context.Context, questID uuid.UUID, stepID string) (out partyrules.CluesView, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.GetQuestStepClues")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	if p.UserType != authz.UserTypeStudent || p.TenantID == nil {
		return out, problem.New(problem.GameAccessStudentOnly, "")
	}
	tenant, me := *p.TenantID, p.UserID
	var st quest.Step
	var vid uuid.UUID
	var visible []quest.Role
	var names map[uuid.UUID]string
	var session *partyquest.Session
	err = h.Read.Read(ctx, func(ctx context.Context) error {
		pr, err := h.Reads.ActiveProgress(ctx, tenant, me, questID)
		if err != nil {
			return err
		}
		if pr == nil {
			return problem.New(problem.NotFound, "no active progress of this quest")
		}
		row, err := h.Content.QuestVersion(ctx, pr.VersionID)
		if err != nil {
			return err
		}
		if row == nil {
			return problem.New(problem.NotFound, "quest version not found")
		}
		s, ok := row.Version.Step(stepID)
		if !ok || s.Type != quest.StepInterdep || pr.Steps[stepID].Status == quest.StepLocked {
			return problem.New(problem.NotFound, "no interdependent step open")
		}
		st, vid = s, pr.VersionID
		if pr.PartyID != nil {
			if session, err = h.Reads.ActiveSession(ctx, tenant, *pr.PartyID, questID, me); err != nil {
				return err
			}
		}
		allowed := map[string]bool{}
		if session != nil {
			for _, r := range session.Visible(st, me) {
				allowed[r] = true
			}
			vid = session.VersionID
			if h.Names != nil {
				if names, err = h.Names.Nicknames(ctx, tenant, session.Present()); err != nil {
					return err
				}
			}
		}
		for _, r := range st.Roles {
			if session == nil || allowed[r.Role] {
				visible = append(visible, r)
			}
		}
		return nil
	})
	if err != nil {
		return out, err
	}
	rs := map[string]port.ClueRender{}
	if len(visible) > 0 {
		if rs, err = h.Renders.Renders(ctx, vid, stepID, visible); err != nil {
			if errors.Is(err, port.ErrBossLearningUnavailable) {
				return out, problem.New(problem.Unavailable, "module learning unavailable").Wrap(err).Retry()
			}
			return out, err
		}
	}
	out = partyrules.Clues(session, st, me, rs, names)
	out.QuestID = questID
	return out, nil
}
