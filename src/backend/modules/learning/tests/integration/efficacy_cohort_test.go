//go:build integration

package integration

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// cohortBlock is the conversion of the classes of a study into cohorts of migration 20261229000007 (between its markers).
func cohortBlock(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(testkit.RepoRoot(t), "src", "backend", "modules", "learning", "migrations",
		"20261229000007_adr0095_efficacy_cohorts.sql"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	i, j := strings.Index(s, "-- BEGIN adr0095-cohort-backfill"), strings.Index(s, "-- END adr0095-cohort-backfill")
	if i < 0 || j < i {
		t.Fatal("cohort backfill markers not found")
	}
	return s[i:j]
}

// F035-AC-08, F035-AC-09 (ADR-0095 D3): the classes of a study become cohorts (school, grade of the study); a school
// whose classes sat in two arms is flagged conflict and never merged automatically; the activation refuses it until the
// cohort is removed and added again. The migration block is re-run as learning_migrator in a rolled back transaction;
// the API part runs on cohorts written by the block's conversion rules.
func TestEfficacyCohortBackfillConflict(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.effSetup(ctx, t)
	c.agreement(t, c.tenantA, "HD-COH", "efficacy_study")
	study := must(t, c.do(t, http.MethodPost, studies, saAdmin, idem(), k.studyBody("math-g3-coh")), http.StatusCreated, "study").str("study_id")
	by := c.users[saAdmin].UserID

	// 1. The migration block on legacy class rows (rolled back).
	tx, err := c.migrator.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := tx.ExecContext(ctx, q, args...); err != nil {
			t.Fatalf("%v: %s", err, q)
		}
	}
	preADR0095Schema(ctx, t, tx)
	exec(`SELECT set_config('app.tenant_id', $1, true)`, c.tenantA.String())
	legacy := func(school uuid.UUID, arm string, pair int) {
		exec(`INSERT INTO learning.efficacy_study_classes (study_id, class_id, tenant_id, school_id, grade_level, pair_no, arm, assignment_method,
			form_order, added_by) VALUES ($1, $2, $3, $4, 3, $5, $6, 'manual', 'a_then_b', $7)`, study, uuid.New(), c.tenantA, school, pair, arm, by)
	}
	legacy(k.school, "intervention", 1) // school A: 3A intervention, 3B comparison → conflict
	legacy(k.school, "comparison", 1)
	legacy(k.school2, "comparison", 1) // school 2: one class → a clean cohort
	exec(cohortBlock(t))
	got := map[uuid.UUID]string{}
	func() {
		rows, err := tx.QueryContext(ctx, `SELECT school_id, conflict, COALESCE(arm, ''), grade_level FROM learning.efficacy_study_cohorts
		WHERE study_id = $1 ORDER BY school_id`, study)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var school uuid.UUID
			var conflict bool
			var arm string
			var grade int
			if err := rows.Scan(&school, &conflict, &arm, &grade); err != nil {
				t.Fatal(err)
			}
			if grade != 3 {
				t.Fatalf("grade of the cohort %d", grade)
			}
			if conflict {
				got[school] = "conflict:" + arm
			} else {
				got[school] = arm
			}
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
	}()
	if got[k.school] != "conflict:" || got[k.school2] != "comparison" || len(got) != 2 {
		t.Fatalf("backfilled cohorts %v", got)
	}
	_ = tx.Rollback()

	// 2. A cohort flagged conflict blocks the activation (422 naming it) until it is removed and added again.
	if _, err := c.platform.ExecContext(ctx, `INSERT INTO learning.efficacy_study_cohorts (study_id, school_id, tenant_id, grade_level, pair_no,
			assignment_method, form_order, conflict, added_by) VALUES ($1, $2, $3, 3, 1, 'manual', 'a_then_b', true, $4),
			($1, $5, $3, 3, 1, 'manual', 'a_then_b', false, $4)`, study, k.school, c.tenantA, by, k.school2); err != nil {
		t.Fatal(err)
	}
	cohorts := studies + "/" + study + "/cohorts"
	list := must(t, c.do(t, http.MethodGet, cohorts, saAdmin, nil, nil), http.StatusOK, "cohorts")
	flags := map[string]any{}
	for _, it := range list.items("items") {
		flags[it["school_id"].(string)] = it["conflict"]
	}
	if flags[k.school.String()] != true || flags[k.school2.String()] != false {
		t.Fatalf("conflict flags %v", flags)
	}
	x := c.do(t, http.MethodPost, studies+"/"+study+":activate", saAdmin, idem(), nil)
	if x.status != http.StatusUnprocessableEntity || !slices.Contains(x.errorCodes(), "/cohorts/"+k.school.String()+"/conflict:INVALID") {
		t.Fatalf("activation with a conflict: %d %s", x.status, x.raw)
	}
	must(t, c.do(t, http.MethodDelete, cohorts+"/"+k.school.String(), saAdmin, tenantHdr(c.tenantA), nil), http.StatusNoContent, "remove")
	re := must(t, c.do(t, http.MethodPost, cohorts, saAdmin, merge(idem(), tenantHdr(c.tenantA)), map[string]any{"school_id": k.school,
		"pair_no": 1, "arm": "intervention"}), http.StatusCreated, "add again")
	if re.body["conflict"] != false {
		t.Fatalf("revived cohort keeps the conflict: %s", re.raw)
	}
	act := must(t, c.do(t, http.MethodPost, studies+"/"+study+":activate", saAdmin, idem(), nil), http.StatusOK, "activate")
	if act.num("cohort_count") != 2 {
		t.Fatalf("activated %s", act.raw)
	}
	// A class scope or a class body is refused by the contract (ADR-0095).
	if x := c.do(t, http.MethodPost, cohorts, saAdmin, merge(idem(), tenantHdr(c.tenantA)), map[string]any{"class_id": uuid.New()}); x.status !=
		http.StatusUnprocessableEntity {
		t.Fatalf("class body: %d %s", x.status, x.raw)
	}
}
