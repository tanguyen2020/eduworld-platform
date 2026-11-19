package contract

import (
	"strconv"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	identityhttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
)

// The F059-1/F059-2 operations (platform staff accounts, integration overview; owner F012) match the merged spec: method + path,
// X-Tenant-Id mode, Idempotency-Key, success status; every one is an Admin Portal operation with the
// portal_staff guard (x-portal-action).
func TestPlatformUserOperationsMatchSpec(t *testing.T) {
	doc := spec(t)
	found := map[string]bool{}
	for path, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			o, ok := identityhttp.PlatformUserOperations[op.OperationID]
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
	if len(found) != len(identityhttp.PlatformUserOperations) || len(found) != 3 {
		t.Fatalf("F059 operations found in the spec: %v", found)
	}
}

// F059-AC-14, F059-AC-23: the responses of listPlatformUsers, createPlatformUser and getIntegrationOverview declare no
// secret, credential, token or IP field (AGENTS R3, R4, R14; ADR-0031).
func TestPlatformUserResponsesHoldNoSecret(t *testing.T) {
	doc := spec(t)
	var walk func(path string, s *openapi3.Schema, depth int)
	walk = func(path string, s *openapi3.Schema, depth int) {
		if s == nil || depth > 6 {
			return
		}
		for name, p := range s.Properties {
			if forbiddenField(name) {
				t.Errorf("%s.%s is a forbidden field", path, name)
			}
			walk(path+"."+name, p.Value, depth+1)
		}
		if s.Items != nil {
			walk(path+"[]", s.Items.Value, depth+1)
		}
		if s.AdditionalProperties.Schema != nil {
			walk(path+"{}", s.AdditionalProperties.Schema.Value, depth+1)
		}
		for _, x := range s.AllOf {
			walk(path, x.Value, depth+1)
		}
	}
	for _, name := range []string{"IntegrationOverview", "User", "UserPage", "PlatformUserCreateRequest"} {
		ref, ok := doc.Components.Schemas[name]
		if !ok {
			t.Fatalf("schema %s missing", name)
		}
		walk(name, ref.Value, 0)
	}
}

func forbiddenField(name string) bool {
	for _, w := range []string{"secret", "password", "token", "credential"} {
		if strings.Contains(name, w) {
			return true
		}
	}
	return name == "ip" || strings.HasPrefix(name, "ip_") || strings.HasSuffix(name, "_ip")
}
