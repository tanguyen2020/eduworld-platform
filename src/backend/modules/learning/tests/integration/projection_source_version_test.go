//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// F020 T044 (OQ F020-1, owner 2026-11-20): ref.schools orders events by source_version when the payload carries it (the
// envelope time is only the fallback): a source_version replaces a row ordered by time, a time-ordered event never
// replaces a row ordered by source_version, an older source_version is ignored. ADR-0095: student.placement_changed
// orders ref.students by the version of the student row and keeps academic_year_id when a later event has none.
func TestProjectionSourceVersionOrdering(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, false)
	p := startProjection(ctx, t, e)
	tenant, school, student, year := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	p.processed(ctx, t, p.publish(ctx, t, "tenant.created", &tenant, tenantPayload(tenant, "th-demo-sv", "active", 1)))
	schoolEvt := func(name string, sv any) {
		pl := map[string]any{"school_id": school.String(), "status": "active", "name": name, "school_level": "primary", "op": "updated",
			"trigger": "scheduled_sync"}
		if sv != nil {
			pl["source_version"] = sv
		}
		p.processed(ctx, t, p.publish(ctx, t, "school.synced", &tenant, pl))
	}
	placementEvt := func(grade int, sv int64, extra map[string]any) {
		pl := map[string]any{"student_id": student.String(), "tenant_id": tenant.String(), "school_id": school.String(), "grade_level": grade,
			"status": "active", "source": "sis", "source_version": sv}
		for k, v := range extra {
			pl[k] = v
		}
		p.processed(ctx, t, p.publish(ctx, t, "student.placement_changed", &tenant, pl))
	}
	name := func(table, idCol string, id uuid.UUID) string {
		var s string
		// tenancyguard:ok TEST read of the projection under the migrator role
		if err := e.migrator.QueryRowContext(ctx, `SELECT name FROM ref.`+table+` WHERE `+idCol+` = $1`, id).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}

	schoolEvt("Trường theo thời gian", nil)
	schoolEvt("Trường v2", 2)
	if got := name("schools", "school_id", school); got != "Trường v2" {
		t.Fatalf("source_version after time ordering: %q", got)
	}
	schoolEvt("Trường v1", 1)
	schoolEvt("Trường thời gian muộn", nil)
	if got := name("schools", "school_id", school); got != "Trường v2" {
		t.Fatalf("older or time-ordered event applied: %q", got)
	}

	placementEvt(3, 3, map[string]any{"academic_year_id": year.String()})
	placementEvt(2, 2, nil) // older: ignored
	placementEvt(4, 4, nil) // newer without year: the year is kept
	var gotYear uuid.NullUUID
	var grade, sv int64
	// tenancyguard:ok TEST read of the projection under the migrator role
	if err := e.migrator.QueryRowContext(ctx, `SELECT academic_year_id, grade_level, source_version FROM ref.students WHERE student_id = $1`, student).
		Scan(&gotYear, &grade, &sv); err != nil || gotYear.UUID != year || grade != 4 || sv != 4 {
		t.Fatalf("placement ordering: %v %d %d %v", gotYear, grade, sv, err)
	}
}
