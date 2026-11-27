package csv

import (
	"bytes"
	stdcsv "encoding/csv"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/datagen/internal/org"
)

func tenants(t *testing.T) []org.Tenant {
	t.Helper()
	ts, err := org.Build(org.Options{Prefix: "perf", Tenants: 1, SchoolsPerTenant: 2, Students: 5, Seed: 1})
	if err != nil {
		t.Fatal(err)
	}
	return ts
}

// The header is the one of the seed students.csv read by the identity csv_import (plus updated_at, required by the
// SIS contract), so the generated file goes through the real import path.
func TestStudentsMatchesImportFormat(t *testing.T) {
	var b bytes.Buffer
	at := time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC)
	if err := Students(&b, tenants(t)[0], at); err != nil {
		t.Fatal(err)
	}
	rows, err := stdcsv.NewReader(strings.NewReader(b.String())).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rows[0], StudentHeader) || len(rows) != 6 {
		t.Fatalf("rows = %v", rows)
	}
	if rows[1][5] != "active" || rows[1][7] != "2026-11-27T00:00:00Z" {
		t.Fatalf("row = %v", rows[1])
	}
}

func TestSchoolsStaffAndBots(t *testing.T) {
	ts := tenants(t)
	var s, st, bt bytes.Buffer
	if err := Schools(&s, ts[0], time.Unix(0, 0)); err != nil {
		t.Fatal(err)
	}
	if err := Staff(&st, ts[0]); err != nil {
		t.Fatal(err)
	}
	if err := Bots(&bt, ts); err != nil {
		t.Fatal(err)
	}
	if strings.Count(s.String(), "\n") != 3 || strings.Count(st.String(), "\n") != 5 {
		t.Fatalf("schools=%q staff=%q", s.String(), st.String())
	}
	var bots []Bot
	if err := json.Unmarshal(bt.Bytes(), &bots); err != nil || len(bots) != 5 {
		t.Fatalf("bots = %v, %v", bots, err)
	}
	if strings.Contains(bt.String(), "password") {
		t.Fatal("bots.json must never carry a password")
	}
}
