package contract

import (
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	competitionhttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
)

func spec(t *testing.T) *openapi3.T {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..", "..")
	doc, err := openapi3.NewLoader().LoadFromFile(filepath.Join(root, "specs", "openapi", "eduworld-api.v1.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

// T005 (F100), T005 (F101), T003 (F104) — the F100, F101 and F104 operations match the merged spec (x-service competition): method + path, never public, X-Tenant-Id
// mode from TenantIdHeader, Idempotency-Key from IdempotencyKey (createParty, inviteToParty, acceptPartyInvitation,
// joinParty, transferPartyLeader — F100-AC-03), success status; every route carries the game_player guard (F100-AC-23).
func TestPartyOperationsMatchSpec(t *testing.T) {
	doc := spec(t)
	found := map[string]bool{}
	for path, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			o, ok := competitionhttp.Operations[op.OperationID]
			if !ok {
				continue
			}
			id := op.OperationID
			found[id] = true
			if method != o.Method || path != o.Pattern || o.Module != "competition" {
				t.Errorf("%s: %s %s (%s), spec %s %s", id, o.Method, o.Pattern, o.Module, method, path)
			}
			if svc, _ := op.Extensions["x-service"].(string); svc != "competition" {
				t.Errorf("%s: x-service %v", id, op.Extensions["x-service"])
			}
			if public := op.Security != nil && len(*op.Security) == 0; public || o.Public {
				t.Errorf("%s: public route", id)
			}
			// getMatch (F105-AC-25), seasons and leaderboards (F106) also serve school staff (portal): the use case checks the channel.
			if o.Guard&httpx.GuardGamePlayer == 0 && !slices.Contains(competitionhttp.StaffOperations, id) {
				t.Errorf("%s: a player route needs the game_player guard", id)
			}
			var params []string
			for _, p := range op.Parameters {
				if p.Ref != "" {
					params = append(params, p.Ref[strings.LastIndex(p.Ref, "/")+1:])
				} else if p.Value != nil {
					params = append(params, p.Value.Name)
				}
			}
			joined := strings.Join(params, ",")
			if want := strings.Contains(joined, "TenantIdHeader"); want != (o.Tenant == httpx.TenantRequired) {
				t.Errorf("%s: tenant mode %v", id, o.Tenant)
			}
			if want := strings.Contains(joined, "IdempotencyKey"); want != (o.Idempotency == httpx.IdempotencyRequired) {
				t.Errorf("%s: idempotency %v", id, o.Idempotency)
			}
			status := 0
			for code := range op.Responses.Map() {
				if n, err := strconv.Atoi(code); err == nil && n >= 200 && n < 300 && (status == 0 || n < status) {
					status = n
				}
			}
			if o.SuccessStatus != status {
				t.Errorf("%s: success %d, spec %d", id, o.SuccessStatus, status)
			}
			if op.Responses.Value("403") == nil || op.Responses.Value("401") == nil {
				t.Errorf("%s: 401/403 (GAME_ACCESS_STUDENT_ONLY) not declared", id)
			}
		}
	}
	if len(found) != len(competitionhttp.Operations) || len(found) != 37 {
		t.Fatalf("F100/F101/F104/F105/F106 operations found in the spec: %v", found)
	}
	for _, id := range []string{"createParty", "inviteToParty", "acceptPartyInvitation", "joinParty", "transferPartyLeader",
		"startPartySession", "createMatchmakingTicket", "createDuel", "acceptDuel", "startBotMatch"} {
		if competitionhttp.Operations[id].Idempotency != httpx.IdempotencyRequired {
			t.Errorf("%s: Idempotency-Key required (F100-AC-03)", id)
		}
	}
	// The Party and PartyInvitation schemas carry the F100 delta (state_version, created_at, name ≤ 128).
	party := doc.Components.Schemas["Party"].Value
	if party.Properties["state_version"] == nil || party.Properties["name"].Value.MaxLength == nil || *party.Properties["name"].Value.MaxLength != 128 {
		t.Error("Party: state_version / name maxLength 128")
	}
	if doc.Components.Schemas["PartyInvitation"].Value.Properties["created_at"] == nil {
		t.Error("PartyInvitation: created_at")
	}
}
