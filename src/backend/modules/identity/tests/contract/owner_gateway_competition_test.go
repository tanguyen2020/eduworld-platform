package contract

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	competitionapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/modules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/participants"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// fakeCompetition is a TEST DOUBLE (R17) of the lifecycle operations of the competition facade
// (competitionapi.StrictServerInterface subset): it records the call context and answers like
// specs/openapi/modules/competition.internal.v1.yaml. The real facade is tested by module competition
// (tests/integration/party_lifecycle_test.go).
type fakeCompetition struct {
	fakeLearning
	status competitionapi.CommonInternalCommonV1UserLifecycleResultStatus
	merge  *competitionapi.MergeUserDataJSONRequestBody
}

func (f *fakeCompetition) CleanupStudentData(ctx context.Context, req competitionapi.CleanupStudentDataRequestObject) (competitionapi.CleanupStudentDataResponseObject, error) {
	if err := f.record(ctx, "cleanupStudentData"); err != nil {
		return nil, err
	}
	return competitionapi.CleanupStudentData200JSONResponse{StudentId: req.StudentId, Status: "done"}, nil
}

func (f *fakeCompetition) RestoreStudentData(ctx context.Context, req competitionapi.RestoreStudentDataRequestObject) (competitionapi.RestoreStudentDataResponseObject, error) {
	if err := f.record(ctx, "restoreStudentData"); err != nil {
		return nil, err
	}
	return competitionapi.RestoreStudentData200JSONResponse{StudentId: req.StudentId, Status: "nothing_to_do"}, nil
}

func (f *fakeCompetition) AnonymizeUserData(ctx context.Context, req competitionapi.AnonymizeUserDataRequestObject) (competitionapi.AnonymizeUserDataResponseObject, error) {
	if err := f.record(ctx, "anonymizeUserData"); err != nil {
		return nil, err
	}
	return competitionapi.AnonymizeUserData200JSONResponse{UserId: req.UserId, Status: "done"}, nil
}

func (f *fakeCompetition) MergeUserData(ctx context.Context, req competitionapi.MergeUserDataRequestObject) (competitionapi.MergeUserDataResponseObject, error) {
	if err := f.record(ctx, "mergeUserData"); err != nil {
		return nil, err
	}
	f.merge = req.Body
	return competitionapi.MergeUserData200JSONResponse{UserId: req.UserId, Status: f.status}, nil
}

// F100 T042 (caller side, OQ F016-2/F045-3): the merge step and the lifecycle steps of module competition go through
// pkg/modcall with the saga deadline, the Principal, the tenant scope and the Idempotency-Key of the step; not wired
// is 503.
func TestOwnerGatewayCompetition(t *testing.T) {
	sa := authztest.Platform("system_admin")
	tenant := uuid.New()
	sc, err := authz.EventScope(tenant)
	if err != nil {
		t.Fatal(err)
	}
	merge := uuid.New()
	ctx := modcall.WithIdempotencyKey(tenancy.WithScope(authztest.Ctx(context.Background(), sa), sc), merge.String())
	from, into := uuid.New(), uuid.New()
	fake := &fakeCompetition{status: "done"}
	res, err := (modules.Gateway{CompetitionMerge: fake}).MergeUserData(ctx, "competition", from,
		port.MergeUserDataRequest{MergeID: merge, Into: into, Reason: "same person"})
	if err != nil || res.Status != "done" || fake.merge == nil || fake.merge.TargetUserId != into || fake.merge.MergeId != merge {
		t.Fatalf("merge: %+v %v %+v", res, err, fake.merge)
	}
	c := fake.seen[0]
	if c.caller != "identity" || c.principal.UserID != sa.UserID || !c.scope || c.key != merge.String() || c.deadline <= 0 ||
		c.deadline > modcall.DefaultDeadlines[modcall.Saga] {
		t.Fatalf("call context %+v", c)
	}
	if _, err := (modules.Gateway{}).MergeUserData(ctx, "competition", from, port.MergeUserDataRequest{}); probCode(err) != problem.Unavailable {
		t.Fatalf("not wired: %v", err)
	}
	// Lifecycle participants (cleanup, restore, anonymize).
	part := participants.Competition{F: fake}
	if part.Name() != "competition" {
		t.Fatal(part.Name())
	}
	last := time.Now()
	if r, err := part.Cleanup(ctx, from, uuid.New(), &last); err != nil || r.Status != "done" {
		t.Fatalf("cleanup: %+v %v", r, err)
	}
	if r, err := part.Restore(ctx, from, true); err != nil || r.Status != "nothing_to_do" {
		t.Fatalf("restore: %+v %v", r, err)
	}
	if r, err := (participants.CompetitionAnonymize{F: fake}).Anonymize(ctx, from, uuid.New(), "data_subject_request"); err != nil || r.Status != "done" {
		t.Fatalf("anonymize: %+v %v", r, err)
	}
	for _, s := range fake.seen[1:] {
		if s.caller != "identity" || !s.scope {
			t.Fatalf("%s: %+v", s.op, s)
		}
	}
	if len(fake.seen) != 4 {
		t.Fatalf("calls: %+v", fake.seen)
	}
}
