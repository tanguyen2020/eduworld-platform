package command

import (
	"context"
	"errors"
	"slices"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/partyrules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/partyquest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/quest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// stepOf returns the interdependent step of a version (404 otherwise).
func stepOf(v quest.Version, stepID string) (quest.Step, error) {
	st, ok := v.Step(stepID)
	if !ok || st.Type != quest.StepInterdep {
		return st, problem.New(problem.NotFound, "no interdependent step")
	}
	return st, nil
}

// sessionOfPlayer locks the active session of the caller on a quest (404 without one, never revealing others).
func (h PartyQuests) sessionOfPlayer(ctx context.Context, tenant, student, questID uuid.UUID) (*partyquest.Session, quest.Version, error) {
	p, err := h.Store.ProgressRefActive(ctx, tenant, student, questID)
	if err != nil {
		return nil, quest.Version{}, err
	}
	if p == nil || p.PartyID == nil {
		return nil, quest.Version{}, problem.New(problem.NotFound, "no party quest session")
	}
	s, err := h.Store.ActiveFor(ctx, tenant, *p.PartyID, questID, student)
	if err != nil {
		return nil, quest.Version{}, err
	}
	if s == nil {
		return nil, quest.Version{}, problem.New(problem.NotFound, "no party quest session")
	}
	row, err := h.Content.QuestVersion(ctx, s.VersionID)
	if err != nil {
		return nil, quest.Version{}, err
	}
	if row == nil {
		return nil, quest.Version{}, problem.New(problem.NotFound, "quest version not found")
	}
	return s, row.Version, nil
}

// AssignRoles implements assignQuestStepRoles (F103-AC-20): only the leader of the session (403 PARTY_NOT_LEADER);
// every role of the step exactly once to a present member, several roles to one member only when allow_multi_role
// (422); every member receives quest.roles.assigned. Idempotency-Key is the httpx middleware.
func (h PartyQuests) AssignRoles(ctx context.Context, questID uuid.UUID, stepID string, as []partyquest.Assignment) (out partyrules.RolesView, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.AssignQuestStepRoles")
	defer func() { done(err) }()
	student, tenant, err := player(ctx)
	if err != nil {
		return out, err
	}
	at := h.now()
	var s *partyquest.Session
	var v quest.Version
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if s, v, err = h.sessionOfPlayer(ctx, tenant, student, questID); err != nil {
			return err
		}
		st, err := stepOf(v, stepID)
		if err != nil {
			return err
		}
		if err := s.AssignByLeader(st, student, as); err != nil {
			return partyQuestProblem(err)
		}
		if err := h.Store.Save(ctx, s); err != nil {
			return err
		}
		names := map[uuid.UUID]string{}
		if h.Names != nil {
			if names, err = h.Names.Nicknames(ctx, tenant, s.Present()); err != nil {
				return err
			}
		}
		out = partyrules.RolesView{QuestID: questID, StepID: stepID, Assignments: []partyrules.RoleView{}}
		for _, r := range st.Roles {
			if id, ok := s.Roles[stepID].Assignments[r.Role]; ok {
				out.Assignments = append(out.Assignments, partyrules.RoleView{Role: r.Role, StudentID: id, Nickname: names[id]})
			}
		}
		return setResult(ctx, out)
	})
	if err != nil {
		return out, err
	}
	h.pushRoles(ctx, s, v, at)
	return out, nil
}

// ShareClue implements shareQuestStepClue (F103-AC-22): the holder of the role puts its clue card on the shared board
// and every member receives quest.clue.shared; another member gets 403 PERMISSION_DENIED; a repeated call never
// duplicates the card. The response is the board as the caller sees it (getQuestStepClues).
func (h PartyQuests) ShareClue(ctx context.Context, questID uuid.UUID, stepID, role string) (out partyrules.CluesView, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.ShareQuestStepClue")
	defer func() { done(err) }()
	student, tenant, err := player(ctx)
	if err != nil {
		return out, err
	}
	at := h.now()
	// The renders the caller will see after sharing (its roles and the shared cards) are read before the transaction:
	// module learning is never called with a row locked.
	var visible []quest.Role
	var vid uuid.UUID
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		s, v, err := h.sessionOfPlayer(ctx, tenant, student, questID)
		if err != nil {
			return err
		}
		st, err := stepOf(v, stepID)
		if err != nil {
			return err
		}
		if !slices.Contains(s.RolesOf(st, student), role) {
			return partyQuestProblem(partyquest.ErrNotRoleOwner)
		}
		for _, r := range st.Roles {
			if slices.Contains(s.Visible(st, student), r.Role) || r.Role == role {
				visible = append(visible, r)
			}
		}
		vid = s.VersionID
		return nil
	})
	if err != nil {
		return out, err
	}
	if h.Renders == nil {
		return out, RetryAfter{Err: problem.New(problem.Unavailable, "module learning is not wired"), After: h.Quests.Config.LearningRetry}
	}
	rs, err := h.Renders.Renders(ctx, vid, stepID, visible)
	if errors.Is(err, port.ErrBossLearningUnavailable) {
		return out, RetryAfter{Err: problem.New(problem.Unavailable, "module learning unavailable").Wrap(err), After: h.Quests.Config.LearningRetry}
	}
	if err != nil {
		return out, err
	}
	var s *partyquest.Session
	added := false
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		var v quest.Version
		if s, v, err = h.sessionOfPlayer(ctx, tenant, student, questID); err != nil {
			return err
		}
		st, err := stepOf(v, stepID)
		if err != nil {
			return err
		}
		if added, err = s.ShareClue(st, role, student, at); err != nil {
			return partyQuestProblem(err)
		}
		if err := h.Store.Save(ctx, s); err != nil {
			return err
		}
		names := map[uuid.UUID]string{}
		if h.Names != nil {
			if names, err = h.Names.Nicknames(ctx, tenant, s.Present()); err != nil {
				return err
			}
		}
		out = partyrules.Clues(s, st, student, rs, names)
		return setResult(ctx, out)
	})
	if err != nil {
		return out, err
	}
	if added && h.Messages != nil {
		names := map[uuid.UUID]string{}
		if h.Names != nil {
			names, _ = h.Names.Nicknames(ctx, tenant, []uuid.UUID{student})
		}
		for _, id := range s.Present() {
			_ = h.Messages.QuestMessage(ctx, id, MsgQuestClueShared, map[string]any{"quest_id": questID, "step_id": stepID, "role": role,
				"shared_by_nickname": names[student]}, at)
		}
	}
	return out, nil
}
