//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/facade"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// sagaCtx is the context of an identity saga step: System Admin principal and the tenant scope of the step.
func (e *penv) sagaCtx() context.Context {
	e.t.Helper()
	sc, err := authz.EventScope(e.tenantA)
	if err != nil {
		e.t.Fatal(err)
	}
	return tenancy.WithModule(tenancy.WithScope(authztest.Ctx(context.Background(), authztest.Platform("system_admin")), sc), "competition")
}

// T042 — OQ F016-2, F045-3 (callee side): cleanupStudentData takes the student out of its party (policy), restore has
// nothing to bring back, anonymizeUserData also resets the names of the parties the user led or created, and
// mergeUserData moves the active membership of the source to the target (role kept, the leader follows) or takes the
// source out when the target is in a party; every operation is idempotent.
func TestPartyLifecycleFacade(t *testing.T) {
	e := setupParty(t, popts{})
	f := facade.Facade{Lifecycle: command.PartyLifecycle{Parties: e.parties}}
	ctx := e.sagaCtx()
	status := func(r any, err error) string {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		switch v := r.(type) {
		case api.CleanupStudentData200JSONResponse:
			return string(v.Status)
		case api.RestoreStudentData200JSONResponse:
			return string(v.Status)
		case api.AnonymizeUserData200JSONResponse:
			return string(v.Status)
		case api.MergeUserData200JSONResponse:
			return string(v.Status)
		}
		t.Fatalf("response %T", r)
		return ""
	}
	a, b, c := e.kidA(), e.kidA(), e.kidA()
	id := e.party(a, b, c)
	// cleanup
	cleanup := api.CleanupStudentDataRequestObject{StudentId: c.id, Body: &api.CleanupStudentDataJSONRequestBody{CleanupRunId: uuid.New(),
		Reason: "inactive_cleanup"}}
	if s := status(f.CleanupStudentData(ctx, cleanup)); s != "done" || e.memberOf(c) != "" {
		t.Fatalf("cleanup: %s", s)
	}
	if s := status(f.CleanupStudentData(ctx, cleanup)); s != "nothing_to_do" {
		t.Fatalf("cleanup twice: %s", s)
	}
	if s := status(f.RestoreStudentData(ctx, api.RestoreStudentDataRequestObject{StudentId: c.id,
		Body: &api.RestoreStudentDataJSONRequestBody{Reason: "student_logged_in_again"}})); s != "nothing_to_do" {
		t.Fatalf("restore: %s", s)
	}
	// merge: a (leader) → x (no party): x leads with a's joined_at, a is out.
	x := e.kidA()
	mergeReq := api.MergeUserDataRequestObject{UserId: a.id, Body: &api.MergeUserDataJSONRequestBody{MergeId: uuid.New(), TargetUserId: x.id,
		Reason: "same person"}}
	if s := status(f.MergeUserData(ctx, mergeReq)); s != "done" {
		t.Fatalf("merge: %s", s)
	}
	r := e.call(b, http.MethodGet, "/multiplayer/parties/me", "", nil)
	if r.str("leader_id") != x.id.String() || len(r.members()) != 2 || e.memberOf(a) != "" {
		t.Fatalf("after merge: %s", r.raw)
	}
	if ev := e.events("party.member_joined"); ev[len(ev)-1]["via"] != "merge" || ev[len(ev)-1]["role"] != "leader" {
		t.Fatalf("member_joined via merge: %v", ev[len(ev)-1])
	}
	if s := status(f.MergeUserData(ctx, mergeReq)); s != "nothing_to_do" {
		t.Fatalf("merge twice: %s", s)
	}
	// merge when the target is in a party already: the source just leaves.
	y := e.kidA()
	e.create(y, http.StatusCreated, nil)
	if s := status(f.MergeUserData(ctx, api.MergeUserDataRequestObject{UserId: b.id, Body: &api.MergeUserDataJSONRequestBody{MergeId: uuid.New(),
		TargetUserId: y.id}})); s != "done" || e.memberOf(b) != "" || e.memberOf(y) == id {
		t.Fatalf("merge into a member: %s", s)
	}
	// anonymize: x leaves, the parties x led or created get the generated name without nickname.
	anon := api.AnonymizeUserDataRequestObject{UserId: x.id, Body: &api.AnonymizeUserDataJSONRequestBody{ErasureRequestId: uuid.New(),
		Reason: "data_subject_request"}}
	if s := status(f.AnonymizeUserData(ctx, anon)); s != "done" || e.memberOf(x) != "" {
		t.Fatalf("anonymize: %s", s)
	}
	if n := e.count(`SELECT count(*) FROM multiplayer.parties WHERE party_id = $1 AND name = 'Nhóm mới'`, id); n != 1 {
		t.Fatalf("name not reset: %d", n)
	}
	if s := status(f.AnonymizeUserData(ctx, anon)); s != "nothing_to_do" {
		t.Fatalf("anonymize twice: %s", s)
	}
	e.checkEventSchemas()
}
