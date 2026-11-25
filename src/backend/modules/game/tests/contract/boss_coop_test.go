package contract

import (
	"context"
	"errors"
	"regexp"
	"slices"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"

	gameapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/api"
	gamehttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// bossBanned are the request fields a client could use to claim a boss result, a contribution or a reward (R5, R6).
var bossBanned = regexp.MustCompile(`^(score|damage|energy.*|phase_cleared|phases_cleared|status|result|outcome|eligible|contribution.*|` +
	`correct.*|is_correct|reward.*|defeated|first_defeat|deadline_at|.*_completed|completed)$`)

// F103-AC-05, AC-25 (R5, R23): no boss or clue route takes a damage, an energy, a phase result, a contribution, a
// correct flag or a reward from the client (body or query); every one is a player route (game_player); the only body
// field of createBossEncounter is mode (party_id kept, ignored).
func TestNoClientBossResult(t *testing.T) {
	doc := spec(t)
	checked := 0
	for _, item := range doc.Paths.Map() {
		for _, op := range item.Operations() {
			if !slices.Contains(gamehttp.BossOperations, op.OperationID) {
				continue
			}
			checked++
			if !slices.Contains(gamehttp.PlayerOperations, op.OperationID) {
				t.Errorf("%s is not a player route", op.OperationID)
			}
			fields := map[string]bool{}
			for _, p := range op.Parameters {
				if p.Value != nil && p.Value.In != "header" {
					fields[p.Value.Name] = true
				}
			}
			if op.RequestBody != nil && op.RequestBody.Value != nil {
				for _, mt := range op.RequestBody.Value.Content {
					requestFields(mt.Schema, map[*openapi3.Schema]bool{}, fields)
				}
			}
			for f := range fields {
				if bossBanned.MatchString(f) {
					t.Errorf("%s: request field %q lets the client claim a result", op.OperationID, f)
				}
			}
		}
	}
	if checked != len(gamehttp.BossOperations) {
		t.Fatalf("checked %d of %d boss operations", checked, len(gamehttp.BossOperations))
	}
}

// F103-AC-26 (TC-SVC-005, AGENTS R29): the party-activity operations of the game facade allow module competition only —
// a call from another module (chat) is refused by pkg/modcall with CALLER_NOT_ALLOWED before the facade runs; the call
// of competition reaches it with the TenantScope and the Idempotency-Key.
func TestPartyActivityCallers(t *testing.T) {
	for _, op := range []string{"listPartyActivityCandidates", "checkPartyActivityEligibility", "startPartyActivity"} {
		if got := gameapi.AllowedCallers[op]; !slices.Equal(got, []string{"competition"}) {
			t.Fatalf("%s: x-allowed-callers %v", op, got)
		}
	}
	tenant := uuid.New()
	p := authz.NewPrincipal(authz.PrincipalData{UserID: uuid.New(), UserType: authz.UserTypeStudent, TenantID: &tenant})
	sc, err := authz.ScopeFor(p)
	if err != nil {
		t.Fatal(err)
	}
	ctx := modcall.WithIdempotencyKey(tenancy.WithScope(authz.WithPrincipal(context.Background(), p), sc), uuid.NewString())
	call := func(caller string) (bool, error) {
		reached := false
		_, err := modcall.Call(ctx, modcall.Op{Caller: caller, Callee: "game", Operation: "startPartyActivity", Kind: modcall.Write,
			AllowedCallers: gameapi.AllowedCallers["startPartyActivity"], ContextRequired: gameapi.ContextRequired["startPartyActivity"]},
			func(ctx context.Context) (struct{}, error) {
				reached = true
				if _, ok := modcall.IdempotencyKeyFrom(ctx); !ok {
					t.Error("the Idempotency-Key reaches the facade")
				}
				if _, ok := tenancy.ScopeFrom(ctx); !ok {
					t.Error("the TenantScope reaches the facade")
				}
				return struct{}{}, nil
			})
		return reached, err
	}
	var pe *problem.Error
	if reached, err := call("chat"); reached || err == nil || !errors.As(err, &pe) || pe.Code != problem.CallerNotAllowed {
		t.Fatalf("module chat: reached=%v err=%v", reached, err)
	}
	if reached, err := call("competition"); !reached || err != nil {
		t.Fatalf("module competition: reached=%v err=%v", reached, err)
	}
}
