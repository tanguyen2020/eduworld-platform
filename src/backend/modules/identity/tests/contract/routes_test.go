package contract

import (
	"net/http"
	"strconv"
	"strings"
	"testing"

	identityhttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
)

// The F010 route table matches the merged spec: method + path, X-Tenant-Id mode, Idempotency-Key and the
// success status (so the httpx chain enforces what the contract declares).
func TestOperationsMatchSpec(t *testing.T) {
	doc := spec(t)
	byID := map[string]struct {
		method, path string
		params       []string
		status       int
	}{}
	for path, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			var names []string
			for _, p := range append(item.Parameters, op.Parameters...) {
				if p.Ref != "" {
					names = append(names, p.Ref[strings.LastIndex(p.Ref, "/")+1:])
				} else if p.Value != nil {
					names = append(names, p.Value.Name)
				}
			}
			status := 0
			for code := range op.Responses.Map() {
				if n, err := strconv.Atoi(code); err == nil && n >= 200 && n < 300 && (status == 0 || n < status) {
					status = n
				}
			}
			byID[op.OperationID] = struct {
				method, path string
				params       []string
				status       int
			}{method, path, names, status}
		}
	}
	has := func(ps []string, n string) bool {
		for _, p := range ps {
			if p == n {
				return true
			}
		}
		return false
	}
	for id, o := range identityhttp.Operations {
		s, ok := byID[id]
		if !ok {
			t.Errorf("%s: not in the spec", id)
			continue
		}
		if s.method != o.Method || s.path != o.Pattern {
			t.Errorf("%s: %s %s, spec %s %s", id, o.Method, o.Pattern, s.method, s.path)
		}
		want := httpx.TenantNone
		switch {
		case has(s.params, "TenantIdHeader"):
			want = httpx.TenantRequired
		case has(s.params, "TenantIdHeaderOptional"):
			want = httpx.TenantOptional
		}
		if o.Tenant != want {
			t.Errorf("%s: tenant mode %v, spec %v", id, o.Tenant, want)
		}
		if (o.Idempotency == httpx.IdempotencyRequired) != has(s.params, "IdempotencyKey") {
			t.Errorf("%s: idempotency %v, spec has IdempotencyKey=%v", id, o.Idempotency, has(s.params, "IdempotencyKey"))
		}
		if o.SuccessStatus != s.status {
			t.Errorf("%s: success %d, spec %d", id, o.SuccessStatus, s.status)
		}
		if strings.HasPrefix(o.Pattern, "/students/me") && o.Guard&httpx.GuardGamePlayer == 0 {
			t.Errorf("%s: the student self route needs the game_player guard (ADR-0031)", id)
		}
	}
	if len(identityhttp.Operations) != 39 || identityhttp.Operations["listTenants"].Method != http.MethodGet {
		t.Fatalf("F010 + F011 operations = %d", len(identityhttp.Operations))
	}
}
