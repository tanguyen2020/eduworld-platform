package contract

import (
	"testing"

	identityhttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
)

// F012-AC-05 (TC-AUTH-070): every identity operation of the Admin Portal (x-portal-action in the spec) has the
// portal_staff guard, so a student token of eduworld-portal gets PORTAL_ACCESS_STAFF_ONLY on all of them.
func TestPortalRoutesRequirePortalStaff(t *testing.T) {
	doc := spec(t)
	actions := map[string]bool{}
	for _, item := range doc.Paths.Map() {
		for _, op := range item.Operations() {
			if _, ok := op.Extensions["x-portal-action"]; ok {
				actions[op.OperationID] = true
			}
		}
	}
	n := 0
	for id, o := range identityhttp.Operations {
		if !actions[id] {
			continue
		}
		n++
		if o.Guard&httpx.GuardPortalStaff == 0 {
			t.Errorf("%s: portal operation without the portal_staff guard", id)
		}
	}
	if n < 50 {
		t.Fatalf("only %d portal operations found", n)
	}
}
