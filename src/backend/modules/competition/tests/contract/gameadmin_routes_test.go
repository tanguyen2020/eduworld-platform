package contract

import (
	"strconv"
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/gameconfig"
	competitionhttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
)

// F108 T001/T016 — F108-AC-01, AC-03: the eleven shared /game-admin/{config_type} operations module competition serves
// match the merged spec: method + path, x-service-routing competition for its seven types, internal only (x-internal),
// portal staff with a platform role (route guards), no X-Tenant-Id, Idempotency-Key where the spec requires it, success
// status.
func TestGameAdminOperationsMatchSpec(t *testing.T) {
	doc := spec(t)
	found := 0
	for path, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			o, ok := competitionhttp.GameAdminOperations[op.OperationID]
			if !ok || !strings.HasPrefix(path, "/game-admin/{config_type}") {
				continue
			}
			found++
			id := op.OperationID
			if method != o.Method || path != o.Pattern || o.Module != "competition" {
				t.Errorf("%s: %s %s, spec %s %s", id, o.Method, o.Pattern, method, path)
			}
			if internal, _ := op.Extensions["x-internal"].(bool); !internal {
				t.Errorf("%s: not internal", id)
			}
			if o.Guard&httpx.GuardPlatformRole == 0 || o.Guard&httpx.GuardPortalStaff == 0 || o.Tenant != httpx.TenantNone {
				t.Errorf("%s: guard %v tenant %v", id, o.Guard, o.Tenant)
			}
			r, _ := op.Extensions["x-service-routing"].(map[string]any)
			m, _ := r["map"].(map[string]any)
			for _, typ := range gameconfig.Names() {
				if m[typ] != "competition" {
					t.Errorf("%s: %s not routed to competition", id, typ)
				}
			}
			idem := false
			for _, p := range op.Parameters {
				if strings.HasSuffix(p.Ref, "/IdempotencyKey") {
					idem = true
				}
			}
			if idem != (o.Idempotency == httpx.IdempotencyRequired) {
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
		}
	}
	if found != len(competitionhttp.GameAdminOperations) || found != 11 {
		t.Fatalf("game-admin operations found: %d of %d", found, len(competitionhttp.GameAdminOperations))
	}
}
