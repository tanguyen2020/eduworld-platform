package contract

import (
	"strconv"
	"strings"
	"testing"

	identityhttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
)

// The portal login operations (ADR-0091, F012 US7/US8) match the merged spec: method + path, public
// (security: []), no X-Tenant-Id, no Idempotency-Key, success status, and the CSRF parameters on refresh and
// logout.
func TestPortalAuthOperationsMatchSpec(t *testing.T) {
	doc := spec(t)
	found := map[string]bool{}
	for path, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			o, ok := identityhttp.PortalAuthOperations[op.OperationID]
			if !ok {
				continue
			}
			found[op.OperationID] = true
			id := op.OperationID
			if method != o.Method || path != o.Pattern {
				t.Errorf("%s: %s %s, spec %s %s", id, o.Method, o.Pattern, method, path)
			}
			if op.Security == nil || len(*op.Security) != 0 || !o.Public {
				t.Errorf("%s: must be public (security: []) and registered Public", id)
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
			if strings.Contains(joined, "TenantIdHeader") || o.Tenant != httpx.TenantNone {
				t.Errorf("%s: X-Tenant-Id is not read (tenant_header_required)", id)
			}
			if strings.Contains(joined, "IdempotencyKey") || o.Idempotency != httpx.IdempotencyNone {
				t.Errorf("%s: no Idempotency-Key (ADR-0091 §5)", id)
			}
			if id == "refreshPortalToken" || id == "portalLogout" {
				if !strings.Contains(joined, "PortalCsrfCookie") || !strings.Contains(joined, "PortalCsrfHeader") {
					t.Errorf("%s: CSRF cookie and header parameters expected, got %v", id, params)
				}
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
			if op.Extensions["x-portal-shell"] != true {
				t.Errorf("%s: x-portal-shell expected (never feature-gated)", id)
			}
		}
	}
	if len(found) != len(identityhttp.PortalAuthOperations) || len(found) != 6 {
		t.Fatalf("portal auth operations found in the spec: %v", found)
	}
}
