package contract

import (
	"strconv"
	"strings"
	"testing"

	identityhttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
)

// The F015 operations (audit, settings, feature flags, menu catalog) match the merged spec: method + path,
// X-Tenant-Id mode, Idempotency-Key, success status; every one is an Admin Portal operation with the
// portal_staff guard (x-portal-action).
func TestSettingsOperationsMatchSpec(t *testing.T) {
	doc := spec(t)
	found := map[string]bool{}
	for path, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			o, ok := identityhttp.SettingsOperations[op.OperationID]
			if !ok {
				continue
			}
			id := op.OperationID
			found[id] = true
			if method != o.Method || path != o.Pattern {
				t.Errorf("%s: %s %s, spec %s %s", id, o.Method, o.Pattern, method, path)
			}
			var params []string
			for _, p := range op.Parameters {
				if p.Ref != "" {
					params = append(params, p.Ref[strings.LastIndex(p.Ref, "/")+1:])
				}
			}
			joined := "," + strings.Join(params, ",") + ","
			want := httpx.TenantNone
			switch {
			case strings.Contains(joined, ",TenantIdHeader,"):
				want = httpx.TenantRequired
			case strings.Contains(joined, ",TenantIdHeaderOptional,"):
				want = httpx.TenantOptional
			}
			if o.Tenant != want {
				t.Errorf("%s: tenant mode %v, spec %v", id, o.Tenant, want)
			}
			if (o.Idempotency == httpx.IdempotencyRequired) != strings.Contains(joined, ",IdempotencyKey,") {
				t.Errorf("%s: idempotency mismatch", id)
			}
			if _, portal := op.Extensions["x-portal-action"]; !portal || o.Guard&httpx.GuardPortalStaff == 0 {
				t.Errorf("%s: portal operation without portal_staff guard", id)
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
	if len(found) != len(identityhttp.SettingsOperations) {
		t.Fatalf("F015 operations found in the spec: %v", found)
	}
}
