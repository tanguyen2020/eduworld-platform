package facade

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/partyrules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// PartyActivityQueries are listPartyActivityCandidates and checkPartyActivityEligibility (query.PartyActivities, F103).
type PartyActivityQueries interface {
	Candidates(ctx context.Context, tenant uuid.UUID, areaID string, ids []uuid.UUID) ([]partyrules.Activity, error)
	Check(ctx context.Context, tenant uuid.UUID, typ, ref string, ids []uuid.UUID) (partyrules.Activity, error)
}

// PartyActivityStarter starts the encounter (boss) or the shared session (quest) of a party activity.
type PartyActivityStarter interface {
	StartParty(ctx context.Context, in command.PartyStart) (command.PartyStarted, error)
}

// PartyActivities serves the party-activity operations of the game facade (x-allowed-callers: competition, F103-AC-02,
// AC-03, AC-13, AC-26, AC-28): the tenant is the TenantScope of the call (pkg/modcall), never a request field; every
// member must be a student of that tenant (422 otherwise); startPartyActivity needs the Idempotency-Key (428).
type PartyActivities struct {
	Queries PartyActivityQueries
	Bosses  PartyActivityStarter
	Quests  PartyActivityStarter
}

// scopeTenant is the session tenant of the TenantScope of ctx.
func scopeTenant(ctx context.Context) (uuid.UUID, error) {
	s, ok := tenancy.ScopeFrom(ctx)
	if !ok {
		return uuid.Nil, problem.New(problem.Unauthenticated, "the tenant scope of the party is required")
	}
	id, single := s.SessionTenantID()
	if !single {
		return uuid.Nil, problem.New(problem.Unauthenticated, "one session tenant is required")
	}
	return id, nil
}

func checks(cs []partyrules.Check) []api.PartyActivityMemberCheck {
	out := make([]api.PartyActivityMemberCheck, 0, len(cs))
	for _, c := range cs {
		m := api.PartyActivityMemberCheck{StudentId: c.StudentID, Eligible: c.Eligible}
		if c.Reason != "" {
			r := api.PartyActivityMemberCheckReasonCode(c.Reason)
			m.ReasonCode = &r
		}
		if c.Requirement != "" {
			req := c.Requirement
			m.Requirement = &req
		}
		out = append(out, m)
	}
	return out
}

func (f PartyActivities) ready() error {
	if f.Queries == nil || f.Bosses == nil || f.Quests == nil {
		return problem.New(problem.Unavailable, "party activities are not wired").Retry()
	}
	return nil
}

// ListPartyActivityCandidates implements api.StrictServerInterface (x-allowed-callers: competition).
func (f PartyActivities) ListPartyActivityCandidates(ctx context.Context, req api.ListPartyActivityCandidatesRequestObject) (api.ListPartyActivityCandidatesResponseObject, error) {
	if err := f.ready(); err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, problem.New(problem.BadRequest, "body is required")
	}
	tenant, err := scopeTenant(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, len(req.Body.Members))
	for i, m := range req.Body.Members {
		ids[i] = m.StudentId
	}
	area := ""
	if req.Body.AreaId != nil {
		area = *req.Body.AreaId
	}
	as, err := f.Queries.Candidates(ctx, tenant, area, ids)
	if err != nil {
		return nil, err
	}
	out := api.ListPartyActivityCandidates200JSONResponse{Data: []api.PartyActivityCandidate{}}
	for _, a := range as {
		c := api.PartyActivityCandidate{ActivityType: api.PartyActivityType(a.Type), ActivityRef: a.Ref, MinMembers: a.MinMembers,
			MaxMembers: a.MaxMembers, AllEligible: a.AllEligible(), Members: checks(a.Checks)}
		if vi := a.Title["vi"]; vi != "" {
			c.Title = &struct {
				En *string `json:"en,omitempty"`
				Vi string  `json:"vi"`
			}{Vi: vi}
			if en := a.Title["en"]; en != "" {
				c.Title.En = &en
			}
		}
		out.Data = append(out.Data, c)
	}
	return out, nil
}

// CheckPartyActivityEligibility implements api.StrictServerInterface (x-allowed-callers: competition).
func (f PartyActivities) CheckPartyActivityEligibility(ctx context.Context, req api.CheckPartyActivityEligibilityRequestObject) (api.CheckPartyActivityEligibilityResponseObject, error) {
	if err := f.ready(); err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, problem.New(problem.BadRequest, "body is required")
	}
	tenant, err := scopeTenant(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, len(req.Body.Members))
	for i, m := range req.Body.Members {
		ids[i] = m.StudentId
	}
	a, err := f.Queries.Check(ctx, tenant, string(req.Body.ActivityType), req.Body.ActivityRef, ids)
	if err != nil {
		return nil, err
	}
	out := api.CheckPartyActivityEligibility200JSONResponse{Eligible: a.AllEligible(), MinMembers: a.MinMembers, MaxMembers: a.MaxMembers,
		AllowContinueOnDisconnect: a.AllowContinue, Members: checks(a.Checks)}
	if a.AreaID != "" {
		area := a.AreaID
		out.AreaId = &area
	}
	return out, nil
}

// StartPartyActivity implements api.StrictServerInterface (x-allowed-callers: competition; Idempotency-Key =
// party_activity_id, research R2): quest → shared session (private_instance), boss → encounter (boss).
func (f PartyActivities) StartPartyActivity(ctx context.Context, req api.StartPartyActivityRequestObject) (api.StartPartyActivityResponseObject, error) {
	if err := f.ready(); err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, problem.New(problem.BadRequest, "body is required")
	}
	if _, ok := modcall.IdempotencyKeyFrom(ctx); !ok {
		return nil, problem.New(problem.IdempotencyKeyRequired, "")
	}
	tenant, err := scopeTenant(ctx)
	if err != nil {
		return nil, err
	}
	b := *req.Body
	in := command.PartyStart{TenantID: tenant, PartyActivityID: b.PartyActivityId, PartyID: b.PartyId, LeaderID: b.LeaderId,
		Type: string(b.ActivityType), Ref: b.ActivityRef}
	for _, m := range b.Members {
		in.Members = append(in.Members, m.StudentId)
	}
	var started command.PartyStarted
	switch b.ActivityType {
	case api.PartyActivityTypeBoss:
		started, err = f.Bosses.StartParty(ctx, in)
	case api.PartyActivityTypeQuest:
		started, err = f.Quests.StartParty(ctx, in)
	default:
		return nil, problem.Invalid("/activity_type", problem.SubInvalid, "module game serves quest and boss")
	}
	if err != nil {
		return nil, err
	}
	out := api.StartPartyActivity201JSONResponse{ExternalRef: started.ExternalRef, RoomType: api.PartyActivityStartedRoomType(started.RoomType)}
	if started.AreaID != "" {
		area := started.AreaID
		out.AreaId = &area
	}
	if started.SceneRef != "" {
		scene := started.SceneRef
		out.SceneRef = &scene
	}
	return out, nil
}
