//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	competitionhttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/natscmd"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// lobbyOperations are the four operations of F101.
var lobbyOperations = []string{"setPartyReady", "selectPartyActivity", "startPartySession", "listPartyEligibleActivities"}

// T035 — F101-AC-19, AC-20 (TC-AUTH-014, TC-TEN-012, AGENTS R8, R23, R29): the four lobby operations × staff and platform tokens
// → 403 GAME_ACCESS_STUDENT_ONLY, nothing written; a student of another tenant, or of the same tenant outside the party,
// gets 404 PARTY_NOT_FOUND over REST and PERMISSION_DENIED over the realtime commands, without learning the party exists;
// the facade game is called with the TenantScope of the party.
func TestLobbyAuthzTenant(t *testing.T) {
	store, _ := valkeyLobby(t)
	game := &fakeGame{}
	e := setupParty(t, popts{lobby: store, game: game})
	e.area("meadow", []string{"g6_7"}, false)
	an, binh := e.kidA(), e.kidA()
	id := e.party(an, binh)
	e.lobbyReady(id, "explore", "meadow", an, binh)
	before := e.count(`SELECT count(*) FROM infra.outbox_events WHERE event_type <> 'audit.recorded'`)

	tokens := map[string]string{
		"school_admin":     e.staffToken(e.tenantA, "school_admin"),
		"academic_manager": e.staffToken(e.tenantA, "academic_manager"),
		"tenant_admin":     e.staffToken(e.tenantA, "tenant_admin"),
		"safety_moderator": e.platformToken("safety_moderator"),
		"system_admin":     e.platformToken("system_admin"),
	}
	for _, op := range lobbyOperations {
		o := competitionhttp.Operations[op]
		for role, tok := range tokens {
			r := e.do(o.Method, e.opPath(op, id, "", an.id), tok, map[string]string{"X-Tenant-Id": e.tenantA.String(), "Idempotency-Key": key()},
				partyBody(op, an.id), "")
			want := "GAME_ACCESS_STUDENT_ONLY"
			if role == "safety_moderator" && r.code() == "TENANT_MISMATCH" {
				want = r.code()
			}
			if r.status != http.StatusForbidden || r.code() != want {
				t.Fatalf("%s by %s: %d %s", op, role, r.status, r.raw)
			}
		}
	}
	coverLobbyRoleDenied(t)
	if n := e.count(`SELECT count(*) FROM infra.outbox_events WHERE event_type <> 'audit.recorded'`); n != before {
		t.Fatalf("staff wrote %d events", n-before)
	}

	// Another tenant (its own X-Tenant-Id) and a stranger of the same tenant: 404, same as an unknown party.
	dung := e.kid(e.tenantB, e.schoolB, uuid.Nil, 6)
	stranger := e.kidA()
	for _, k := range []kid{dung, stranger} {
		for _, op := range lobbyOperations {
			o := competitionhttp.Operations[op]
			idem := ""
			if o.Idempotency != 0 {
				idem = key()
			}
			r := e.call(k, o.Method, e.opPath(op, id, "", k.id), idem, partyBody(op, k.id))
			if r.status != http.StatusNotFound || r.code() != "PARTY_NOT_FOUND" {
				t.Fatalf("%s by %s: %d %s", op, k.nickname, r.status, r.raw)
			}
			unknown := e.call(k, o.Method, e.opPath(op, uuid.NewString(), "", k.id), idem, partyBody(op, k.id))
			if unknown.status != r.status || unknown.code() != r.code() {
				t.Fatalf("%s: a party of another tenant differs from an unknown party", op)
			}
		}
	}
	coverLobbyCrossTenant(t)

	time.Sleep(1100 * time.Millisecond) // a new window of party_lobby_cmd_rate
	// Realtime commands with the Ew-* headers of another tenant / a stranger: PERMISSION_DENIED.
	handlers := map[string]rt.CommandHandler{
		"ready":  natscmd.ReadySet(e.parties),
		"select": natscmd.ActivitySelect(e.parties),
		"start":  natscmd.SessionStart(e.parties),
	}
	payloads := map[string]any{
		"ready":  map[string]any{"party_id": id, "ready": true},
		"select": map[string]any{"party_id": id, "expected_version": 0, "activity": map[string]any{"kind": "explore", "id": "meadow"}},
		"start":  map[string]any{"party_id": id, "expected_version": 0},
	}
	for _, k := range []kid{dung, stranger} {
		for name, h := range handlers {
			if rep := h(context.Background(), k.caller(), wsPayload(t, payloads[name])); rep.OK || rep.Problem.Code != "PERMISSION_DENIED" {
				t.Fatalf("ws %s by %s: %+v", name, k.nickname, rep.Problem)
			}
		}
	}
	// A malformed command never reaches the use case.
	if rep := handlers["ready"](context.Background(), an.caller(), wsPayload(t, map[string]any{"party_id": "x"})); rep.OK ||
		rep.Problem.Code != "VALIDATION_FAILED" {
		t.Fatalf("malformed: %+v", rep)
	}
	if n := e.count(`SELECT count(*) FROM infra.outbox_events WHERE event_type <> 'audit.recorded'`); n != before {
		t.Fatalf("refused commands wrote %d events", n-before)
	}

	// Facade game: TenantScope of the party on the REST and the realtime paths.
	if r := e.pick(an, id, "quest", "q-bridge", -1); r.status != http.StatusOK {
		t.Fatalf("select quest: %d %s", r.status, r.raw)
	}
	rep := handlers["select"](context.Background(), an.caller(), wsPayload(t, map[string]any{"party_id": id,
		"expected_version": e.version(an, id), "activity": map[string]any{"kind": "boss", "id": "b-slime"}}))
	if !rep.OK {
		t.Fatalf("ws select boss: %+v", rep.Problem)
	}
	if len(game.scopes) < 2 {
		t.Fatalf("facade calls: %v", game.scopes)
	}
	for _, s := range game.scopes {
		if s != e.tenantA {
			t.Fatalf("facade game called with tenant %s", s)
		}
	}
}

// coverLobbyRoleDenied declares the role_denied case of the four lobby operations, verified by TestLobbyAuthzTenant.
func coverLobbyRoleDenied(t *testing.T) {
	authztest.Cover(t, "setPartyReady", authztest.RoleDenied)
	authztest.Cover(t, "selectPartyActivity", authztest.RoleDenied)
	authztest.Cover(t, "startPartySession", authztest.RoleDenied)
	authztest.Cover(t, "listPartyEligibleActivities", authztest.RoleDenied)
}

// coverLobbyCrossTenant declares the cross_tenant case of the four lobby operations, verified by TestLobbyAuthzTenant.
func coverLobbyCrossTenant(t *testing.T) {
	authztest.Cover(t, "setPartyReady", authztest.CrossTenant)
	authztest.Cover(t, "selectPartyActivity", authztest.CrossTenant)
	authztest.Cover(t, "startPartySession", authztest.CrossTenant)
	authztest.Cover(t, "listPartyEligibleActivities", authztest.CrossTenant)
}
