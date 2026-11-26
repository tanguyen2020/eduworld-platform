//go:build integration

package integration

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/metrics"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

// TestGradeMatrixSisRollover (F011, ADR-0095, F074-AC-25 producer side, TEST_STRATEGY §6.10): one SIS student of each
// grade 1–9 is placed in its grade (first placement: every field changed); the school-year rollover import moves grades
// 1–8 up one grade — student.placement_changed carries changed_fields = [grade_level], the new and the previous grade
// (what game.grade_transition consumes) — and grade 9 graduates (status inactive, grade kept). Nothing else changes.
func TestGradeMatrixSisRollover(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	a := seedTenant(ctx, t, e.platform, "pgd-grade-matrix")
	ta, school := uuid.MustParse(a.tenant), uuid.MustParse(a.school)
	sa := authztest.Ctx(ctx, authztest.Platform("system_admin"))
	conns := command.SisConnections{UoW: e.uow, Connections: write.SisConnectionRepo{}, Audit: e.deps.Audit}
	conn, err := conns.Create(sa, ta, "csv_import", "grade matrix")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conns.SetSchools(sa, conn.ID, []uuid.UUID{school}, "grade matrix"); err != nil {
		t.Fatal(err)
	}
	h := command.SisRun{UoW: e.uow, Savepoints: e.uow, Connections: write.SisConnectionRepo{}, Sync: write.SisSyncRepo{},
		Students: write.StudentRepo{}, Missing: write.SisSyncRepo{}, Rules: write.SisSyncRepo{}, Events: e.deps.Events, Audit: e.deps.Audit, BatchSize: 4,
		Observer: metrics.Runs{}}
	admin := authztest.Ctx(ctx, authztest.Staff(ta, "school_admin"))
	csv := func(grade func(g int) (int, string)) string {
		var b strings.Builder
		b.WriteString("student_external_id,school_external_id,student_code,full_name,grade_level,status,login_username\n")
		for g := 1; g <= 9; g++ {
			ng, status := grade(g)
			fmt.Fprintf(&b, "HS-G%d,TH-A,G%03d,Học Sinh Khối %d,%d,%s,hs.matrix.%d\n", g, g, g, ng, status, g)
		}
		return b.String()
	}
	run := func(body string) {
		t.Helper()
		res, err := h.Handle(admin, command.RunImportInput{ConnectionID: conn.ID, SchoolID: school, Files: sisFiles(t, map[string]string{"students.csv": body})})
		if err != nil || res.Status != "succeeded" {
			t.Fatalf("run = %+v %v", res, err)
		}
	}
	grade := func(username string) (int, string) {
		var g int
		var status string
		if err := e.platform.QueryRowContext(ctx, `SELECT s.grade_level, s.status FROM org.students s JOIN identity.users u ON u.user_id = s.student_id
			WHERE u.username = $1 AND s.tenant_id = $2 AND s.deleted_at IS NULL`, username, a.tenant).Scan(&g, &status); err != nil {
			t.Fatalf("%s: %v", username, err)
		}
		return g, status
	}
	run(csv(func(g int) (int, string) { return g, "active" }))
	first := syncedPayloads(ctx, t, e, "student.placement_changed", a.tenant)
	if len(first) != 9 {
		t.Fatalf("first placements = %d", len(first))
	}
	seen := map[float64]bool{}
	for _, p := range first {
		seen[p["grade_level"].(float64)] = true
		if len(p["changed_fields"].([]any)) != 4 || p["previous_grade_level"] != nil {
			t.Fatalf("first placement %v", p)
		}
	}
	for g := 1; g <= 9; g++ {
		if gl, st := grade(fmt.Sprintf("hs.matrix.%d", g)); gl != g || st != "active" || !seen[float64(g)] {
			t.Fatalf("grade %d placed at %d %s", g, gl, st)
		}
	}
	// School-year rollover: lên khối for grades 1–8, grade 9 graduates.
	run(csv(func(g int) (int, string) {
		if g == 9 {
			return 9, "graduated"
		}
		return g + 1, "active"
	}))
	all := syncedPayloads(ctx, t, e, "student.placement_changed", a.tenant)
	var moves []map[string]any // the outbox has no order: a move is a payload with a previous placement
	for _, p := range all {
		if p["previous_grade_level"] != nil {
			moves = append(moves, p)
		}
	}
	if len(moves) != 9 {
		t.Fatalf("rollover placements = %d", len(moves))
	}
	byPrev := map[float64]map[string]any{}
	for _, p := range moves {
		byPrev[p["previous_grade_level"].(float64)] = p
	}
	for g := 1; g <= 9; g++ {
		p := byPrev[float64(g)]
		if p == nil {
			t.Fatalf("no placement_changed from grade %d", g)
		}
		cf := p["changed_fields"].([]any)
		if g <= 8 {
			if p["grade_level"].(float64) != float64(g+1) || len(cf) != 1 || cf[0] != "grade_level" || p["status"] != "active" {
				t.Fatalf("grade %d → %d: %v", g, g+1, p)
			}
			if gl, st := grade(fmt.Sprintf("hs.matrix.%d", g)); gl != g+1 || st != "active" {
				t.Fatalf("grade %d after the rollover: %d %s", g, gl, st)
			}
			continue
		}
		if p["grade_level"].(float64) != 9 || len(cf) != 1 || cf[0] != "status" || p["status"] != "inactive" || p["previous_status"] != "active" {
			t.Fatalf("grade 9 graduates: %v", p)
		}
	}
	// Re-importing the rollover changes nothing.
	run(csv(func(g int) (int, string) {
		if g == 9 {
			return 9, "graduated"
		}
		return g + 1, "active"
	}))
	if n := len(syncedPayloads(ctx, t, e, "student.placement_changed", a.tenant)); n != len(all) {
		t.Fatalf("re-import emitted %d placements", n-len(all))
	}
}
