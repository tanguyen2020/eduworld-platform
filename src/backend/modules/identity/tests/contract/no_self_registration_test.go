// Package contract holds contract tests of module identity against the merged OpenAPI spec.
package contract

import (
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
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

// F010-AC-26 (TC-TEN-014): no endpoint lets a game client self-register or create a student account;
// accounts are created only by the school (SIS/portal) or the platform, under /admin or /tenant.
func TestNoSelfRegistration(t *testing.T) {
	doc := spec(t)
	selfReg := regexp.MustCompile(`(?i)(^|[^a-z])(register|sign-?up|self[-_]?enrol)`)
	createsAccount := regexp.MustCompile(`(?i)^(create|import|provision)\w*(student|user|account)`)
	n := 0
	for path, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			n++
			if selfReg.MatchString(path) || selfReg.MatchString(op.OperationID) {
				t.Errorf("%s %s (%s): self-registration route", method, path, op.OperationID)
			}
			if !createsAccount.MatchString(op.OperationID) {
				continue
			}
			if !strings.HasPrefix(path, "/admin/") && !strings.HasPrefix(path, "/tenant/") && !strings.HasPrefix(path, "/sis/") {
				t.Errorf("%s %s (%s): account creation outside /admin, /tenant or /sis", method, path, op.OperationID)
			}
			if op.Security != nil && len(*op.Security) == 0 {
				t.Errorf("%s %s (%s): account creation without authentication", method, path, op.OperationID)
			}
		}
	}
	if n < 100 {
		t.Fatalf("only %d operations read from the merged spec", n)
	}
	op := doc.Paths.Find("/admin/common-tenant/students")
	if op == nil || op.Post == nil || op.Post.OperationID != "createCommonTenantStudents" {
		t.Fatal("createCommonTenantStudents must exist under /admin (System Admin only)")
	}
}
