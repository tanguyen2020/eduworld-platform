//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// F020 T044 (OQ F020-1, owner 2026-11-20): ref.schools / ref.classes order events by source_version when the payload
// carries it (the envelope time is only the fallback): a source_version replaces a row ordered by time, a time-ordered
// event never replaces a row ordered by source_version, an older source_version is ignored; class.synced also projects
// academic_year_id and is_virtual.
func TestProjectionSourceVersionOrdering(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, false)
	p := startProjection(ctx, t, e)
	tenant, school, class, year := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	p.processed(ctx, t, p.publish(ctx, t, "tenant.created", &tenant, tenantPayload(tenant, "th-demo-sv", "active", 1)))
	schoolEvt := func(name string, sv any) {
		pl := map[string]any{"school_id": school.String(), "status": "active", "name": name, "school_level": "primary", "op": "updated",
			"trigger": "scheduled_sync"}
		if sv != nil {
			pl["source_version"] = sv
		}
		p.processed(ctx, t, p.publish(ctx, t, "school.synced", &tenant, pl))
	}
	classEvt := func(name string, sv any, extra map[string]any) {
		pl := map[string]any{"class_id": class.String(), "school_id": school.String(), "grade_level": 3, "name": name, "academic_year": "2026-2027",
			"status": "active", "op": "updated", "trigger": "scheduled_sync"}
		if sv != nil {
			pl["source_version"] = sv
		}
		for k, v := range extra {
			pl[k] = v
		}
		p.processed(ctx, t, p.publish(ctx, t, "class.synced", &tenant, pl))
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

	classEvt("3A", nil, nil)
	classEvt("3A1", 3, map[string]any{"academic_year_id": year.String(), "is_virtual": true})
	var gotYear uuid.NullUUID
	var virtual bool
	var sv int64
	// tenancyguard:ok TEST read of the projection under the migrator role
	if err := e.migrator.QueryRowContext(ctx, `SELECT academic_year_id, is_virtual, source_version FROM ref.classes WHERE class_id = $1`, class).
		Scan(&gotYear, &virtual, &sv); err != nil || gotYear.UUID != year || !virtual || sv != 3 {
		t.Fatalf("class with source_version: %v %v %d %v", gotYear, virtual, sv, err)
	}
	classEvt("3A cũ", 2, nil)
	classEvt("3A thời gian", nil, nil)
	if got := name("classes", "class_id", class); got != "3A1" {
		t.Fatalf("older or time-ordered class event applied: %q", got)
	}
	classEvt("3A2", 4, map[string]any{"is_virtual": false})
	// tenancyguard:ok TEST read of the projection under the migrator role
	if err := e.migrator.QueryRowContext(ctx, `SELECT academic_year_id, is_virtual FROM ref.classes WHERE class_id = $1`, class).
		Scan(&gotYear, &virtual); err != nil || gotYear.UUID != year || virtual || name("classes", "class_id", class) != "3A2" {
		t.Fatalf("class v4: %v %v %v", gotYear, virtual, err)
	}
}
