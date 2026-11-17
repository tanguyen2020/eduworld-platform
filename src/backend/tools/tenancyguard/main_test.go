package main

import (
	"strings"
	"testing"
)

// F003-AC-07 (TC-TEN-017): each rule reports file and line; allowed cases stay silent.
func TestScan(t *testing.T) {
	got, err := Scan("testdata/fixture")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(got, "\n")
	for _, want := range []string{
		"0001.sql:3: RLS policy references deleted_at",
		"table content.orphans has tenant_id but no ENABLE ROW LEVEL SECURITY",
		"0001.sql:7: DELETE/TRUNCATE in a migration",
		"repo.go:4:", "must filter tenant_id and deleted_at",
		"repo.go:5:", "DELETE/TRUNCATE in application code",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in:\n%s", want, joined)
		}
	}
	for _, unwanted := range []string{"content.items has tenant_id", "infra.outbox_like", "repo.go:3:", "repo.go:6:", "repo.go:9:", "purge.go", "0001.sql:6:"} {
		if strings.Contains(joined, unwanted) {
			t.Errorf("unexpected %q in:\n%s", unwanted, joined)
		}
	}
}
