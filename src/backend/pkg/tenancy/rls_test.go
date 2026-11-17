package tenancy_test

import (
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

func TestRLSPoliciesNeverFilterDeletedAt(t *testing.T) {
	for _, k := range []tenancy.TableKind{tenancy.KindT, tenancy.KindTA, tenancy.KindTStar, tenancy.KindTAStar} {
		sql, err := tenancy.RLSPolicies("assessment.attempts", k)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(sql, "deleted_at") {
			t.Fatalf("%s policy filters deleted_at (ADR-0089 §1):\n%s", k, sql)
		}
		for _, want := range []string{"ENABLE ROW LEVEL SECURITY", "FORCE ROW LEVEL SECURITY", "tenant_id = infra.current_tenant_id()", "FOR SELECT", "FOR INSERT"} {
			if !strings.Contains(sql, want) {
				t.Fatalf("%s policy misses %q:\n%s", k, want, sql)
			}
		}
		if strings.Contains(sql, "FOR DELETE") {
			t.Fatalf("%s has a DELETE policy", k)
		}
		hasUpdate := strings.Contains(sql, "FOR UPDATE")
		appendOnly := k == tenancy.KindTA || k == tenancy.KindTAStar
		if hasUpdate == appendOnly {
			t.Fatalf("%s: update policy present=%v, append-only=%v", k, hasUpdate, appendOnly)
		}
	}
}

func TestHybridPolicyNeedsResource(t *testing.T) {
	if _, err := tenancy.RLSPolicies("content.question_versions", tenancy.KindH); err == nil {
		t.Fatal("H without HybridResource accepted")
	}
	sql, err := tenancy.RLSPolicies("content.question_versions", tenancy.KindH, tenancy.HybridResource("question", "question_id"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sql, "deleted_at IS NULL AND (") || !strings.Contains(sql, "infra.tenant_resource_assignments") ||
		!strings.Contains(sql, "a.resource_type = 'question'") || !strings.Contains(sql, "question_versions.question_id") {
		t.Fatalf("unexpected H policy:\n%s", sql)
	}
}

func TestIdentifiersAreValidated(t *testing.T) {
	for _, bad := range []string{"attempts", "a.b; DROP TABLE x", "A.b", "a.b.c"} {
		if _, err := tenancy.RLSPolicies(bad, tenancy.KindT); err == nil {
			t.Fatalf("bad table %q accepted", bad)
		}
	}
	if _, err := tenancy.RLSPolicies("content.q", tenancy.KindH, tenancy.HybridResource("x'; --", "id")); err == nil {
		t.Fatal("bad resource type accepted")
	}
	if _, err := tenancy.LiveView("assessment.attempts", "Learning"); err == nil {
		t.Fatal("bad module accepted")
	}
}

func TestLiveView(t *testing.T) {
	sql, err := tenancy.LiveView("assessment.attempts", "learning")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"CREATE OR REPLACE VIEW assessment.attempts_live", "WHERE deleted_at IS NULL", "GRANT SELECT ON assessment.attempts_live TO learning_readonly", "GRANT USAGE ON SCHEMA assessment TO learning_readonly"} {
		if !strings.Contains(sql, want) {
			t.Fatalf("missing %q:\n%s", want, sql)
		}
	}
	if strings.Contains(sql, "security_invoker") {
		t.Fatal("security_invoker would require SELECT on the base table for _readonly (F002-AC-29)")
	}
}
