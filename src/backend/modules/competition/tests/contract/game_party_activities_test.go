package contract

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/party"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/modules"
	gameapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/contract"
)

func strp(s string) *string { return &s }

// T007 — caller-side facade test of module competition → game for party activities (listPartyActivityCandidates,
// checkPartyActivityEligibility, startPartyActivity; x-allowed-callers competition; provider F103) against the generated
// fake (TEST DOUBLE gameapi.Fake, R17): request mapping, TenantScope required, Idempotency-Key = party_activity_id on
// startPartyActivity, caller identity, responses valid against game.internal.v1.yaml, failures mapped to unavailable
// (5xx, deadline, no facade) or refused (4xx).
func TestGamePartyActivitiesFacade(t *testing.T) {
	spec, err := contract.LoadFacade("game")
	if err != nil {
		t.Fatal(err)
	}
	tenant, partyID, a, b, act := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	sc, err := authz.EventScope(tenant)
	if err != nil {
		t.Fatal(err)
	}
	ctx := tenancy.WithScope(context.Background(), sc)
	reason := gameapi.PartyActivityMemberCheckReasonCode("grade_not_allowed")
	area := "forest"
	candidates := gameapi.ListPartyActivityCandidates200JSONResponse{Data: []gameapi.PartyActivityCandidate{{ActivityType: "boss",
		ActivityRef: "b-golem", MinMembers: 2, MaxMembers: 4, Members: []gameapi.PartyActivityMemberCheck{{StudentId: a, Eligible: true},
			{StudentId: b, Eligible: false, ReasonCode: &reason, Requirement: strp("grade_levels:4")}}}}}
	eligibility := gameapi.CheckPartyActivityEligibility200JSONResponse{Eligible: true, MinMembers: 2, MaxMembers: 4,
		AllowContinueOnDisconnect: true, AreaId: &area, Members: []gameapi.PartyActivityMemberCheck{{StudentId: a, Eligible: true}}}
	ext := uuid.New()
	started := gameapi.StartPartyActivity201JSONResponse{ExternalRef: ext, RoomType: "private_instance", AreaId: &area, SceneRef: strp("quest/bridge")}
	for op, v := range map[string]struct {
		status int
		body   any
	}{"listPartyActivityCandidates": {200, candidates}, "checkPartyActivityEligibility": {200, eligibility}, "startPartyActivity": {201, started}} {
		if err := spec.CheckResponse(op, v.status, v.body); err != nil {
			t.Fatalf("%s fake response outside the spec: %v", op, err)
		}
	}
	var idem string
	fake := &gameapi.Fake{
		ListPartyActivityCandidatesFunc: func(ctx context.Context, r gameapi.ListPartyActivityCandidatesRequestObject) (gameapi.ListPartyActivityCandidatesResponseObject, error) {
			if modcall.CallerFrom(ctx) != "competition" || r.Body.PartyId != partyID || len(r.Body.Members) != 2 {
				t.Fatalf("candidates request: %+v", r.Body)
			}
			return candidates, nil
		},
		CheckPartyActivityEligibilityFunc: func(ctx context.Context, r gameapi.CheckPartyActivityEligibilityRequestObject) (gameapi.CheckPartyActivityEligibilityResponseObject, error) {
			if r.Body.ActivityType != "quest" || r.Body.ActivityRef != "q-bridge" {
				t.Fatalf("check request: %+v", r.Body)
			}
			if s, ok := tenancy.ScopeFrom(ctx); !ok {
				t.Fatal("no TenantScope")
			} else if id, _ := s.SessionTenantID(); id != tenant {
				t.Fatalf("scope %s", id)
			}
			return eligibility, nil
		},
		StartPartyActivityFunc: func(ctx context.Context, r gameapi.StartPartyActivityRequestObject) (gameapi.StartPartyActivityResponseObject, error) {
			idem, _ = modcall.IdempotencyKeyFrom(ctx)
			if r.Body.PartyActivityId != act || r.Body.LeaderId != a || len(r.Body.Members) != 2 || r.Body.Members[0].GameSessionId == nil {
				t.Fatalf("start request: %+v", r.Body)
			}
			return started, nil
		},
	}
	g := modules.Game{F: fake}
	cs, err := g.Candidates(ctx, partyID, []uuid.UUID{a, b})
	if err != nil || len(cs) != 1 || cs[0].Eligibility.Members[1].Reason != party.ReasonGradeNotAllowed ||
		cs[0].Eligibility.Members[1].Requirement != "grade_levels:4" {
		t.Fatalf("candidates: %+v %v", cs, err)
	}
	e, err := g.Check(ctx, partyID, "quest", "q-bridge", []uuid.UUID{a})
	if err != nil || e.MinMembers != 2 || !e.AllowContinue || e.AreaID != "forest" {
		t.Fatalf("check: %+v %v", e, err)
	}
	session := uuid.New()
	s, err := g.Start(ctx, port.GameStart{ActivityID: act, PartyID: partyID, LeaderID: a, Type: "quest", Ref: "q-bridge",
		Members: []uuid.UUID{a, b}, Sessions: map[uuid.UUID]uuid.UUID{a: session}})
	if err != nil || s.ExternalRef != ext || s.RoomType != party.RoomPrivateInstance || s.AreaID != "forest" || idem != act.String() {
		t.Fatalf("start: %+v %v idem=%s", s, err, idem)
	}

	// x-context-required tenant_scope: without the scope the call never reaches module game.
	if _, err := g.Check(context.Background(), partyID, "quest", "q-bridge", []uuid.UUID{a}); !errors.Is(err, port.ErrGameUnavailable) {
		t.Fatalf("missing scope: %v", err)
	}
	// No facade (F103 not wired), a 5xx or the deadline → unavailable (503 + Retry-After upstream); a 4xx → refused.
	if _, err := (modules.Game{}).Check(ctx, partyID, "quest", "q-bridge", nil); !errors.Is(err, port.ErrGameUnavailable) {
		t.Fatalf("no facade: %v", err)
	}
	down := &gameapi.Fake{CheckPartyActivityEligibilityFunc: func(context.Context, gameapi.CheckPartyActivityEligibilityRequestObject) (gameapi.CheckPartyActivityEligibilityResponseObject, error) {
		return nil, problem.New(problem.Unavailable, "")
	}}
	if _, err := (modules.Game{F: down}).Check(ctx, partyID, "quest", "q-bridge", nil); !errors.Is(err, port.ErrGameUnavailable) {
		t.Fatalf("5xx: %v", err)
	}
	slow := &gameapi.Fake{CheckPartyActivityEligibilityFunc: func(ctx context.Context, _ gameapi.CheckPartyActivityEligibilityRequestObject) (gameapi.CheckPartyActivityEligibilityResponseObject, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	dctx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if _, err := (modules.Game{F: slow}).Check(dctx, partyID, "quest", "q-bridge", nil); !errors.Is(err, port.ErrGameUnavailable) {
		t.Fatalf("deadline: %v", err)
	}
	refused := &gameapi.Fake{StartPartyActivityFunc: func(context.Context, gameapi.StartPartyActivityRequestObject) (gameapi.StartPartyActivityResponseObject, error) {
		return nil, problem.New(problem.Conflict, "")
	}}
	if _, err := (modules.Game{F: refused}).Start(ctx, port.GameStart{ActivityID: act, Type: "boss", Ref: "b"}); !errors.Is(err, port.ErrGameRefused) {
		t.Fatalf("4xx: %v", err)
	}
	// Only competition may call these operations.
	for _, op := range []string{"listPartyActivityCandidates", "checkPartyActivityEligibility", "startPartyActivity"} {
		if callers := gameapi.AllowedCallers[op]; len(callers) != 1 || callers[0] != "competition" {
			t.Fatalf("%s x-allowed-callers: %v", op, callers)
		}
	}
	if got := gameapi.ContextRequired["startPartyActivity"]; len(got) != 2 {
		t.Fatalf("startPartyActivity x-context-required: %v", got)
	}
}
