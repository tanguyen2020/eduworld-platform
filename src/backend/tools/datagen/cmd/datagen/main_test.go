package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/envguard"
)

// F132-AC-07 (quickstart step 6): `make datagen ENV=production` refuses to run and writes nothing.
func TestRunRefusesProduction(t *testing.T) {
	dir := t.TempDir()
	err := run([]string{"-out", dir}, "production", &bytes.Buffer{})
	if !errors.Is(err, envguard.ErrRefused) {
		t.Fatalf("err = %v, want ErrRefused", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("files written: %v", entries)
	}
}

func TestRunWritesEveryFile(t *testing.T) {
	dir := t.TempDir()
	if err := run([]string{"-out", dir, "-tenants", "2", "-schools-per-tenant", "1", "-students", "10"}, "staging", &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"bots.json", "perf-001/schools.csv", "perf-001/students.csv", "perf-002/staff.csv"} {
		if _, err := os.Stat(filepath.Join(dir, p)); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
	}
}
