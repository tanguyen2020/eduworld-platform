package platformadmin_test

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/platformadmin"
)

// Routing table: every type has one owner; identity types are not assignable; the clone allowlist never holds
// student data (F045-AC-15, R22).
func TestRouting(t *testing.T) {
	cases := map[string]string{"question": "learning", "question_set": "learning", "quest": "game", "tournament_template": "competition",
		"tenant_settings": "identity", "user": "identity", "student": "identity"}
	for rt, want := range cases {
		if m, err := platformadmin.Owner(rt); err != nil || m != want {
			t.Fatalf("%s: %s %v", rt, m, err)
		}
	}
	if _, err := platformadmin.Owner("attempt"); !errors.Is(err, platformadmin.ErrUnknownResourceType) {
		t.Fatal("unknown type")
	}
	if !platformadmin.Assignable("question_set") || platformadmin.Assignable("user") || platformadmin.Assignable("tenant_settings") {
		t.Fatal("assignable")
	}
	for _, rt := range []string{"student", "user", "challenge", "quest", "item", "course", "attempt", "enrollment"} {
		if platformadmin.Clonable(rt) {
			t.Fatalf("%s must not be clonable", rt)
		}
	}
	if platformadmin.RestorableThroughTrash("user") || !platformadmin.RestorableThroughTrash("student") {
		t.Fatal("restorable")
	}
	if platformadmin.DeleteReasonOf("student") != "inactive_cleanup" || platformadmin.DeleteReasonOf("user") != "merged" {
		t.Fatal("delete reason")
	}
}

// learning_material (F024) is learning content like lesson: owned by learning, assignable, clonable and
// restorable from the trash (OQ F057 follow-up).
func TestRoutingLearningMaterial(t *testing.T) {
	if m, err := platformadmin.Owner("learning_material"); err != nil || m != platformadmin.ModuleLearning {
		t.Fatalf("owner = %q %v", m, err)
	}
	for _, rt := range []string{"learning_material", "lesson"} {
		if !platformadmin.Assignable(rt) || !platformadmin.Clonable(rt) || !platformadmin.RestorableThroughTrash(rt) {
			t.Fatalf("%s: assignable/clonable/restorable", rt)
		}
	}
	steps, err := platformadmin.PlanClone(uuid.New(), uuid.New(), "learning_material", []uuid.UUID{uuid.New()})
	if err != nil || len(steps) != 1 || steps[0].Module != platformadmin.ModuleLearning {
		t.Fatalf("plan: %+v %v", steps, err)
	}
}

// Every value of the ResourceType enum of the merged public spec (source of truth) has an owner, so filtering or
// restoring the trash by any type is routed.
func TestRoutingCoversResourceTypeEnum(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "..", "..", "specs", "openapi", "eduworld-api.v1.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^    ResourceType:\r?\n(?:      .*\r?\n)*?      enum: \[([^\]]+)\]`).FindSubmatch(raw)
	if m == nil {
		t.Fatal("ResourceType enum not found in the merged spec")
	}
	for _, rt := range strings.Split(string(m[1]), ",") {
		if _, err := platformadmin.Owner(strings.TrimSpace(rt)); err != nil {
			t.Errorf("ResourceType %q has no owner module: %v", strings.TrimSpace(rt), err)
		}
	}
}

// F045-AC-15, AC-16: same tenant, type outside the allowlist, empty/too many/duplicate ids are refused.
func TestPlanClone(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	ids := []uuid.UUID{uuid.New(), uuid.New()}
	steps, err := platformadmin.PlanClone(a, b, "question_set", ids)
	if err != nil || len(steps) != 1 || steps[0].Module != "learning" || !steps[0].Root {
		t.Fatalf("plan: %+v %v", steps, err)
	}
	if _, err := platformadmin.PlanClone(a, a, "question", ids); !errors.Is(err, platformadmin.ErrCloneSameTenant) {
		t.Fatal("same tenant")
	}
	if _, err := platformadmin.PlanClone(a, b, "quest", ids); !errors.Is(err, platformadmin.ErrCloneNotAllowed) {
		t.Fatal("allowlist")
	}
	if _, err := platformadmin.PlanClone(a, b, "question", nil); !errors.Is(err, platformadmin.ErrCloneNoResources) {
		t.Fatal("empty")
	}
	many := make([]uuid.UUID, 501)
	for i := range many {
		many[i] = uuid.New()
	}
	if _, err := platformadmin.PlanClone(a, b, "question", many); !errors.Is(err, platformadmin.ErrCloneTooManyIDs) {
		t.Fatal("too many")
	}
	if _, err := platformadmin.PlanClone(a, b, "question", []uuid.UUID{ids[0], ids[0]}); !errors.Is(err, platformadmin.ErrCloneDuplicateIDs) {
		t.Fatal("duplicates")
	}
}

// F045-AC-17: the job status folds the step outcomes.
func TestSummarizeClone(t *testing.T) {
	ok := platformadmin.CloneStepOutcome{Module: "learning", ResourceType: "question", Status: "succeeded", Cloned: 3}
	if r := platformadmin.SummarizeClone([]platformadmin.CloneStepOutcome{ok}, false, ""); r.Status != "succeeded" || r.Count != 3 || len(r.Errors) != 0 {
		t.Fatalf("%+v", r)
	}
	part := ok
	part.Status, part.Skipped = "partially_failed", 1
	if r := platformadmin.SummarizeClone([]platformadmin.CloneStepOutcome{part}, false, ""); r.Status != "partially_failed" || r.Stats["question_skipped"] != 1 {
		t.Fatalf("%+v", r)
	}
	failed := platformadmin.CloneStepOutcome{StepNo: 0, Module: "learning", ResourceType: "question", Status: "failed", Error: strings.Repeat("x", 400)}
	r := platformadmin.SummarizeClone([]platformadmin.CloneStepOutcome{failed}, true, "boom")
	if r.Status != "failed" || len(r.Errors) != 1 || len(r.Errors[0]) > 255 || !strings.HasPrefix(r.Errors[0], "learning step 0: ") {
		t.Fatalf("%+v", r)
	}
}

// F045-AC-31: merge invariants.
func TestCheckMerge(t *testing.T) {
	from := platformadmin.MergeAccount{UserID: uuid.New(), UserType: "student", Status: "active"}
	into := platformadmin.MergeAccount{UserID: uuid.New(), UserType: "student", Status: "active"}
	issue := platformadmin.MergeIssue{Type: "duplicate_identifier", Status: "open", Resolution: "merge_requested"}
	if err := platformadmin.CheckMerge(from, into, issue); err != nil {
		t.Fatal(err)
	}
	staff := into
	staff.UserType = "staff"
	merged := from
	merged.MergedAway = true
	anon := into
	anon.Status = "anonymized"
	closed := issue
	closed.Resolution = "same_person"
	for name, c := range map[string]struct {
		f, i platformadmin.MergeAccount
		is   platformadmin.MergeIssue
		want error
	}{
		"same":   {from, from, issue, platformadmin.ErrMergeSameUser},
		"type":   {from, staff, issue, platformadmin.ErrMergeUserType},
		"merged": {merged, into, issue, platformadmin.ErrMergeAlreadyMerged},
		"anon":   {from, anon, issue, platformadmin.ErrMergeAnonymized},
		"issue":  {from, into, closed, platformadmin.ErrMergeIssueNotPending},
	} {
		if err := platformadmin.CheckMerge(c.f, c.i, c.is); !errors.Is(err, c.want) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if !platformadmin.ValidReason("abc") || platformadmin.ValidReason("ab") || platformadmin.ValidReason(strings.Repeat("a", 1001)) {
		t.Fatal("reason")
	}
	if platformadmin.MergeStatus("blocked") != "failed" || platformadmin.MergeStatus("compensating") != "running" ||
		platformadmin.MergeStepStatus("pending") != "pending" {
		t.Fatal("status mapping")
	}
}
