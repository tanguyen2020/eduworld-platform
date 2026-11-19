package contract

import (
	"strconv"
	"strings"
	"testing"

	identityhttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
)

// The F017 operations (triggerSisSync, updateSisConnection, checkSisConnectionHealth) match the merged spec:
// method + path, X-Tenant-Id mode, Idempotency-Key, success status, x-internal (internal gateway only,
// F017-AC-30) and the portal_staff guard of Admin Portal operations (F012-AC-05).
func TestSisContractOperationsMatchSpec(t *testing.T) {
	doc := spec(t)
	found := map[string]bool{}
	for path, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			o, ok := identityhttp.SisContractOperations[op.OperationID]
			if !ok {
				continue
			}
			id := op.OperationID
			found[id] = true
			if method != o.Method || path != o.Pattern {
				t.Errorf("%s: %s %s, spec %s %s", id, o.Method, o.Pattern, method, path)
			}
			if !strings.HasPrefix(path, "/sis/") || op.Extensions["x-internal"] != true {
				t.Errorf("%s: SIS administration must be x-internal under /sis/ (internal gateway only)", id)
			}
			if _, ok := op.Extensions["x-portal-action"]; ok && o.Guard&httpx.GuardPortalStaff == 0 {
				t.Errorf("%s: portal operation without the portal_staff guard", id)
			}
			var params []string
			for _, p := range append(item.Parameters, op.Parameters...) {
				if p.Ref != "" {
					params = append(params, p.Ref[strings.LastIndex(p.Ref, "/")+1:])
				} else if p.Value != nil {
					params = append(params, p.Value.Name)
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
				t.Errorf("%s: Idempotency-Key mismatch", id)
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
	if len(found) != 3 || len(identityhttp.SisContractOperations) != 3 {
		t.Fatalf("F017 operations found in the spec: %v", found)
	}
	for id := range identityhttp.SisContractOperations {
		if _, dup := identityhttp.Operations[id]; dup {
			t.Errorf("%s registered twice", id)
		}
	}
}

// F017-AC-04, AC-30: no response schema of the SIS administration exposes a secret reference; the
// contract_v1 create body bounds every string (base_url ≤ 2048 https, secret_ref ≤ 512, codes ≤ 64).
func TestSisConnectionSchemas(t *testing.T) {
	doc := spec(t)
	conn := doc.Components.Schemas["SisConnection"].Value
	for _, f := range []string{"secret_ref", "webhook_secret_ref", "credential"} {
		if _, ok := conn.Properties[f]; ok {
			t.Errorf("SisConnection exposes %s", f)
		}
	}
	for _, f := range []string{"has_secret", "has_webhook_secret", "capabilities", "status_reason", "last_health_at", "last_success_at"} {
		if _, ok := conn.Properties[f]; !ok {
			t.Errorf("SisConnection lacks %s", f)
		}
	}
	cfg := doc.Components.Schemas["SisContractConfig"].Value
	if u := cfg.Properties["base_url"].Value; u.MaxLength == nil || *u.MaxLength != 2048 || u.Pattern != "^https?://" {
		t.Errorf("base_url bounds: %+v", u)
	}
	ref := doc.Components.Schemas["SisSecretRef"].Value
	if ref.MaxLength == nil || *ref.MaxLength != 512 {
		t.Error("secret_ref ≤ 512")
	}
	create := doc.Components.Schemas["SisConnectionCreateContract"].Value
	if v := create.Properties["contract_version"].Value; v.MaxLength == nil || *v.MaxLength != 64 || !strings.HasPrefix(v.Pattern, "^1") {
		t.Error("contract_version ≤ 64, major 1")
	}
}
