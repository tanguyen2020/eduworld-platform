package contract

import (
	"strconv"
	"strings"
	"testing"

	identityhttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
)

// The game login operations (F013) match the merged spec: method + path, public (security: []) except
// gameLogout (student token, game_player guard), X-Tenant-Id exactly on refresh and logout (exempt on login
// and tenant select — tenant_header_required, F013-AC-24), no Idempotency-Key, success status.
func TestGameAuthOperationsMatchSpec(t *testing.T) {
	doc := spec(t)
	found := map[string]bool{}
	for path, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			o, ok := identityhttp.GameAuthOperations[op.OperationID]
			if !ok {
				continue
			}
			id := op.OperationID
			found[id] = true
			if method != o.Method || path != o.Pattern {
				t.Errorf("%s: %s %s, spec %s %s", id, o.Method, o.Pattern, method, path)
			}
			public := op.Security != nil && len(*op.Security) == 0
			if public != o.Public {
				t.Errorf("%s: public %v, spec %v", id, o.Public, public)
			}
			if !o.Public && o.Guard&httpx.GuardGamePlayer == 0 {
				t.Errorf("%s: a player route needs the game_player guard (ADR-0031)", id)
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
			want := httpx.TenantNone
			if strings.Contains(joined, "TenantIdHeader") {
				want = httpx.TenantRequired
			}
			if o.Tenant != want {
				t.Errorf("%s: tenant mode %v, spec %v", id, o.Tenant, want)
			}
			if (id == "gameLogin" || id == "selectGameLoginTenant") && want != httpx.TenantNone {
				t.Errorf("%s: must be exempt from X-Tenant-Id", id)
			}
			if strings.Contains(joined, "IdempotencyKey") || o.Idempotency != httpx.IdempotencyNone {
				t.Errorf("%s: no Idempotency-Key", id)
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
		}
	}
	if len(found) != len(identityhttp.GameAuthOperations) || len(found) != 4 {
		t.Fatalf("game auth operations found in the spec: %v", found)
	}
}
