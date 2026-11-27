package contract

import (
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	learninghttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/http"
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

// F021 route table = merged spec: method + path, X-Tenant-Id mode, Idempotency-Key and success status, so the httpx
// chain enforces what the contract declares (contracts/README §1).
func TestCurriculumOperationsMatchSpec(t *testing.T) {
	type specOp struct {
		method, path string
		params       []string
		status       int
	}
	byID := map[string]specOp{}
	for path, item := range spec(t).Paths.Map() {
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
			byID[op.OperationID] = specOp{method, path, names, status}
		}
	}
	for id, o := range learninghttp.Operations {
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
		case slices.Contains(s.params, "TenantIdHeader"):
			want = httpx.TenantRequired
		case slices.Contains(s.params, "TenantIdHeaderOptional"):
			want = httpx.TenantOptional
		}
		if o.Tenant != want {
			t.Errorf("%s: tenant mode %v, spec %v", id, o.Tenant, want)
		}
		if (o.Idempotency == httpx.IdempotencyRequired) != slices.Contains(s.params, "IdempotencyKey") {
			t.Errorf("%s: idempotency %v, spec has IdempotencyKey=%v", id, o.Idempotency, slices.Contains(s.params, "IdempotencyKey"))
		}
		if o.SuccessStatus != s.status {
			t.Errorf("%s: success %d, spec %d", id, o.SuccessStatus, s.status)
		}
		if o.Module != "learning" {
			t.Errorf("%s: module %q", id, o.Module)
		}
	}
	// Every curriculum operation of the contract (contracts/README §1) is served.
	for _, id := range []string{"listCmsCurricula", "createCurriculum", "getCmsCurriculum", "updateCurriculum", "deleteCurriculum",
		"listCurriculumVersions", "createCurriculumVersion", "getCurriculumVersion", "updateCurriculumVersionDraft", "transitionCurriculumVersion",
		"publishCurriculumVersion", "exportCurriculumVersion", "getCurriculumVersionLoMappings", "putCurriculumVersionLoMappings",
		"listCmsSubjects", "createSubject", "updateSubject", "listCmsTopics", "createTopic", "updateTopic", "deleteTopic", "listCmsObjectives",
		"createObjective", "updateObjective", "deleteObjective", "listCmsCompetencies", "createCompetency", "updateCompetency",
		"listTextbookSeries", "createTextbookSeries", "listTextbooks", "createTextbook", "listTextbookSections", "createTextbookSection",
		"updateTextbookSection", "putTextbookSectionObjectives", "createContentImport", "getContentImport", "listCurricula", "listSubjects",
		"listTopics", "listLearningObjectives", "getLearningObjective", "listContentImportItems", "createDraftFromImportItem"} {
		if _, ok := learninghttp.Operations[id]; !ok {
			t.Errorf("%s is not served", id)
		}
	}
}
