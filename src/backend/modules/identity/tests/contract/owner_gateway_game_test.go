package contract

import (
	"context"
	"testing"

	"github.com/google/uuid"

	gameapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/modules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// fakeGameMerge is a TEST DOUBLE (R17) of mergeUserData of the game facade (gameapi.StrictServerInterface subset):
// it records the call context and answers like specs/openapi/modules/game.internal.v1.yaml. The real facade is
// tested by module game (tests/integration/profile_lifecycle_test.go).
type fakeGameMerge struct {
	fakeLearning
	status  gameapi.CommonInternalCommonV1UserLifecycleResultStatus
	blocked string
	body    *gameapi.MergeUserDataJSONRequestBody
}

func (f *fakeGameMerge) MergeUserData(ctx context.Context, req gameapi.MergeUserDataRequestObject) (gameapi.MergeUserDataResponseObject, error) {
	if err := f.record(ctx, "mergeUserData"); err != nil {
		return nil, err
	}
	f.body = req.Body
	out := gameapi.MergeUserData200JSONResponse{UserId: req.UserId, Status: f.status}
	if f.blocked != "" {
		out.BlockedReason = &f.blocked
	}
	return out, nil
}

// F070 T039 (caller side, OQ F016-2/F045-3): the merge step of module game goes through pkg/modcall with the saga
// deadline, the Principal, the tenant scope and Idempotency-Key = merge_id; blocked passes through; not wired is 503.
func TestOwnerGatewayGameMerge(t *testing.T) {
	sa := authztest.Platform("system_admin")
	tenant := uuid.New()
	sc, err := authz.EventScope(tenant)
	if err != nil {
		t.Fatal(err)
	}
	merge := uuid.New()
	ctx := modcall.WithIdempotencyKey(tenancy.WithScope(authztest.Ctx(context.Background(), sa), sc), merge.String())
	from, into := uuid.New(), uuid.New()
	fake := &fakeGameMerge{status: "done"}
	g := modules.Gateway{GameMerge: fake}
	res, err := g.MergeUserData(ctx, "game", from, port.MergeUserDataRequest{MergeID: merge, Into: into, Reason: "same person"})
	if err != nil || res.Status != "done" || fake.body == nil || fake.body.TargetUserId != into || fake.body.MergeId != merge {
		t.Fatalf("merge: %+v %v %+v", res, err, fake.body)
	}
	c := fake.seen[0]
	if c.caller != "identity" || c.principal.UserID != sa.UserID || !c.scope || c.key != merge.String() || c.deadline <= 0 ||
		c.deadline > modcall.DefaultDeadlines[modcall.Saga] {
		t.Fatalf("call context %+v", c)
	}
	fake.status, fake.blocked = "blocked", "active_game_session"
	if res, err := g.MergeUserData(ctx, "game", from, port.MergeUserDataRequest{MergeID: merge, Into: into}); err != nil ||
		res.Status != "blocked" || res.BlockedReason != "active_game_session" {
		t.Fatalf("blocked: %+v %v", res, err)
	}
	if _, err := (modules.Gateway{}).MergeUserData(ctx, "game", from, port.MergeUserDataRequest{}); probCode(err) != problem.Unavailable {
		t.Fatalf("not wired: %v", err)
	}
}
