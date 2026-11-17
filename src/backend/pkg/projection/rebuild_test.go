package projection_test

import (
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/projection"
)

func TestUpsertSQL(t *testing.T) {
	q, err := projection.UpsertSQL("ref.schools", "school_id", []string{"tenant_id", "name"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"INSERT INTO ref.schools (school_id, source_version, tenant_id, name)", "ON CONFLICT (school_id)",
		"WHERE schools.source_version < EXCLUDED.source_version", "name = EXCLUDED.name"} {
		if !strings.Contains(q, want) {
			t.Fatalf("missing %q in %s", want, q)
		}
	}
	if strings.Contains(strings.ToUpper(q), "DELETE") || strings.Contains(strings.ToUpper(q), "TRUNCATE") {
		t.Fatal("rebuild must never delete")
	}
	if _, err := projection.UpsertSQL("ref.x; DROP TABLE y", "id", nil); err == nil {
		t.Fatal("bad identifier accepted")
	}
	var reg projection.Registry
	if err := reg.Register(projection.Table{Module: "learning", Source: "identity", Name: "schools"}); err == nil {
		t.Fatal("table outside ref.* accepted")
	}
}
