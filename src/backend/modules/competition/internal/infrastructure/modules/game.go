package modules

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/party"
	gameapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// GameActivities is the part of the game facade for party activities (x-allowed-callers: competition; implemented by
// F103): listPartyActivityCandidates, checkPartyActivityEligibility, startPartyActivity.
type GameActivities interface {
	ListPartyActivityCandidates(ctx context.Context, req gameapi.ListPartyActivityCandidatesRequestObject) (gameapi.ListPartyActivityCandidatesResponseObject, error)
	CheckPartyActivityEligibility(ctx context.Context, req gameapi.CheckPartyActivityEligibilityRequestObject) (gameapi.CheckPartyActivityEligibilityResponseObject, error)
	StartPartyActivity(ctx context.Context, req gameapi.StartPartyActivityRequestObject) (gameapi.StartPartyActivityResponseObject, error)
}

// Game implements port.GameActivities on the game facade through pkg/modcall (span, deadline, TenantScope of the
// party, Principal of the caller; Idempotency-Key = party_activity_id for startPartyActivity, research R2). F nil
// (module game offers no party activities yet), an error, a 5xx or the deadline → port.ErrGameUnavailable; a 4xx →
// port.ErrGameRefused.
type Game struct{ F GameActivities }

var _ port.GameActivities = Game{}

func op(name string, kind modcall.Kind) modcall.Op {
	return modcall.Op{Caller: "competition", Callee: "game", Operation: name, Kind: kind, AllowedCallers: gameapi.AllowedCallers[name],
		ContextRequired: gameapi.ContextRequired[name]}
}

func members(ids []uuid.UUID) []gameapi.PartyActivityMember {
	out := make([]gameapi.PartyActivityMember, len(ids))
	for i, id := range ids {
		out[i] = gameapi.PartyActivityMember{StudentId: id}
	}
	return out
}

func checks(in []gameapi.PartyActivityMemberCheck) []party.Check {
	out := make([]party.Check, len(in))
	for i, c := range in {
		out[i] = party.Check{StudentID: c.StudentId, Eligible: c.Eligible}
		if c.ReasonCode != nil {
			out[i].Reason = string(*c.ReasonCode)
		}
		if c.Requirement != nil {
			out[i].Requirement = *c.Requirement
		}
	}
	return out
}

// classify maps a failed call: a problem below 500 is a refusal, anything else is unavailable.
func classify(err error) error {
	var pe *problem.Error
	if errors.As(err, &pe) && problem.Status(pe.Code) < 500 {
		return fmt.Errorf("%w: %w", port.ErrGameRefused, err)
	}
	return fmt.Errorf("%w: %w", port.ErrGameUnavailable, err)
}

// Candidates implements port.GameActivities.
func (g Game) Candidates(ctx context.Context, partyID uuid.UUID, ids []uuid.UUID) ([]port.GameCandidate, error) {
	if g.F == nil {
		return nil, port.ErrGameUnavailable
	}
	body := gameapi.ListPartyActivityCandidatesJSONRequestBody{PartyId: partyID, Members: members(ids)}
	resp, err := modcall.Call(ctx, op("listPartyActivityCandidates", modcall.Read),
		func(ctx context.Context) (gameapi.ListPartyActivityCandidatesResponseObject, error) {
			return g.F.ListPartyActivityCandidates(ctx, gameapi.ListPartyActivityCandidatesRequestObject{Body: &body})
		})
	if err != nil {
		return nil, classify(err)
	}
	v, ok := resp.(gameapi.ListPartyActivityCandidates200JSONResponse)
	if !ok {
		return nil, fmt.Errorf("%w: response %T", port.ErrGameUnavailable, resp)
	}
	out := make([]port.GameCandidate, 0, len(v.Data))
	for _, c := range v.Data {
		gc := port.GameCandidate{Type: string(c.ActivityType), Ref: c.ActivityRef, Eligibility: party.Eligibility{MinMembers: c.MinMembers,
			MaxMembers: c.MaxMembers, Members: checks(c.Members)}}
		if c.Title != nil {
			gc.Title = map[string]string{"vi": c.Title.Vi}
			if c.Title.En != nil {
				gc.Title["en"] = *c.Title.En
			}
		}
		out = append(out, gc)
	}
	return out, nil
}

// Check implements port.GameActivities.
func (g Game) Check(ctx context.Context, partyID uuid.UUID, typ, ref string, ids []uuid.UUID) (party.Eligibility, error) {
	if g.F == nil {
		return party.Eligibility{}, port.ErrGameUnavailable
	}
	body := gameapi.CheckPartyActivityEligibilityJSONRequestBody{PartyId: partyID, ActivityType: gameapi.PartyActivityType(typ),
		ActivityRef: ref, Members: members(ids)}
	resp, err := modcall.Call(ctx, op("checkPartyActivityEligibility", modcall.Read),
		func(ctx context.Context) (gameapi.CheckPartyActivityEligibilityResponseObject, error) {
			return g.F.CheckPartyActivityEligibility(ctx, gameapi.CheckPartyActivityEligibilityRequestObject{Body: &body})
		})
	if err != nil {
		return party.Eligibility{}, classify(err)
	}
	v, ok := resp.(gameapi.CheckPartyActivityEligibility200JSONResponse)
	if !ok {
		if _, nf := resp.(gameapi.CheckPartyActivityEligibility404ApplicationProblemPlusJSONResponse); nf {
			return party.Eligibility{}, port.ErrGameRefused
		}
		return party.Eligibility{}, fmt.Errorf("%w: response %T", port.ErrGameUnavailable, resp)
	}
	e := party.Eligibility{MinMembers: v.MinMembers, MaxMembers: v.MaxMembers, AllowContinue: v.AllowContinueOnDisconnect,
		Members: checks(v.Members)}
	if v.AreaId != nil {
		e.AreaID = *v.AreaId
	}
	return e, nil
}

// Start implements port.GameActivities (Idempotency-Key = party_activity_id).
func (g Game) Start(ctx context.Context, s port.GameStart) (port.GameStarted, error) {
	if g.F == nil {
		return port.GameStarted{}, port.ErrGameUnavailable
	}
	ms := make([]gameapi.PartyActivityStartMember, len(s.Members))
	for i, id := range s.Members {
		ms[i] = gameapi.PartyActivityStartMember{StudentId: id}
		if gs, ok := s.Sessions[id]; ok {
			g := gs
			ms[i].GameSessionId = &g
		}
	}
	body := gameapi.StartPartyActivityJSONRequestBody{PartyActivityId: s.ActivityID, PartyId: s.PartyID, LeaderId: s.LeaderID,
		ActivityType: gameapi.PartyActivityType(s.Type), ActivityRef: s.Ref, Members: ms}
	if s.GameMode != "" {
		mode := s.GameMode
		body.GameMode = &mode
	}
	ctx = modcall.WithIdempotencyKey(ctx, s.ActivityID.String())
	resp, err := modcall.Call(ctx, op("startPartyActivity", modcall.Write),
		func(ctx context.Context) (gameapi.StartPartyActivityResponseObject, error) {
			return g.F.StartPartyActivity(ctx, gameapi.StartPartyActivityRequestObject{Body: &body})
		})
	if err != nil {
		return port.GameStarted{}, classify(err)
	}
	v, ok := resp.(gameapi.StartPartyActivity201JSONResponse)
	if !ok {
		return port.GameStarted{}, fmt.Errorf("%w: response %T", port.ErrGameRefused, resp)
	}
	out := port.GameStarted{ExternalRef: v.ExternalRef, RoomType: string(v.RoomType)}
	if v.AreaId != nil {
		out.AreaID = *v.AreaId
	}
	if v.SceneRef != nil {
		out.SceneRef = *v.SceneRef
	}
	return out, nil
}

// LateGame is the GameActivities of the commands while module game is wired after Register (Module.Wire): until a
// facade is set quest and boss answer 503 UNAVAILABLE and are not offered.
type LateGame struct {
	mu sync.RWMutex
	f  GameActivities
}

var _ port.GameActivities = (*LateGame)(nil)

// Set wires the game facade (nil: none).
func (l *LateGame) Set(f GameActivities) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.f = f
}

func (l *LateGame) get() Game {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return Game{F: l.f}
}

// Candidates implements port.GameActivities.
func (l *LateGame) Candidates(ctx context.Context, partyID uuid.UUID, ids []uuid.UUID) ([]port.GameCandidate, error) {
	return l.get().Candidates(ctx, partyID, ids)
}

// Check implements port.GameActivities.
func (l *LateGame) Check(ctx context.Context, partyID uuid.UUID, typ, ref string, ids []uuid.UUID) (party.Eligibility, error) {
	return l.get().Check(ctx, partyID, typ, ref, ids)
}

// Start implements port.GameActivities.
func (l *LateGame) Start(ctx context.Context, s port.GameStart) (port.GameStarted, error) {
	return l.get().Start(ctx, s)
}
