package query

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/view"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/party"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// AreasLimit bounds the explore candidates (PartyEligibleActivity list ≤ 100 with the quest/boss of module game).
const AreasLimit = 50

// ListEligibleActivities implements listPartyEligibleActivities (F101-AC-05): members only; explore from the
// projections of module competition, quest/boss from the facade game (outside the read transaction); each activity
// says whether every member is eligible and, per member, a reason code and the requirement of the activity — never the
// grade or class of another student (R16). Module game not answering leaves explore only.
func (q Parties) ListEligibleActivities(ctx context.Context, partyID uuid.UUID) (res []view.Eligible, err error) {
	ctx, done := observability.UseCase(ctx, "competition.query.ListPartyEligibleActivities")
	defer func() { done(err) }()
	tenant, me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	if q.LobbyReader == nil {
		return nil, problem.New(problem.Unavailable, "").Retry()
	}
	var p *party.Party
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		var err error
		if p, err = q.Reader.Party(ctx, tenant, partyID); err != nil {
			return err
		}
		if p == nil || p.RequireMember(me) != nil {
			return problem.New(problem.PartyNotFound, "")
		}
		subjects, err := q.LobbyReader.Subjects(ctx, tenant, p.MemberIDs(), q.now())
		if err != nil {
			return err
		}
		areas, err := q.LobbyReader.Areas(ctx, AreasLimit)
		if err != nil {
			return err
		}
		unlocked, err := q.LobbyReader.UnlockedAreas(ctx, tenant, p.MemberIDs())
		if err != nil {
			return err
		}
		for _, a := range areas {
			per := map[uuid.UUID]bool{}
			for id, set := range unlocked {
				per[id] = set[a.ID]
			}
			res = append(res, view.Eligible{Type: party.ActivityExplore, Ref: a.ID,
				Eligibility: party.ExploreEligibility(a, subjects, per, q.PrivateCapacity)})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if q.Game == nil {
		return res, nil
	}
	gctx := ctx
	if _, ok := tenancy.ScopeFrom(ctx); !ok {
		sc, err := authz.EventScope(tenant)
		if err != nil {
			return nil, err
		}
		gctx = tenancy.WithScope(ctx, sc)
	}
	cands, err := q.Game.Candidates(gctx, p.ID, p.MemberIDs())
	if err != nil {
		slog.WarnContext(ctx, "competition: party candidates of module game unavailable", "error", err) // explore only
		return res, nil
	}
	for _, c := range cands {
		if !party.FromGame(c.Type) {
			continue
		}
		res = append(res, view.Eligible{Type: c.Type, Ref: c.Ref, Title: c.Title, Eligibility: c.Eligibility})
	}
	return res, nil
}
