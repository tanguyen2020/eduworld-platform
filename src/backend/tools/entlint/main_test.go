package main

import (
	"strings"
	"testing"
)

// F003-AC-07: String without MaxLen, Text without justification, schema without exactly one mixin.
func TestCheckDir(t *testing.T) {
	got, err := CheckDir("testdata/bad")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(got, "\n")
	for _, want := range []string{"field.String without MaxLen", "field.Text without", "schema NoMixin declares 0", "schema TwoMixins declares 2"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in:\n%s", want, joined)
		}
	}
	if len(got) != 4 || strings.Contains(joined, "Good") || !strings.Contains(joined, "schema.go:") {
		t.Fatalf("unexpected findings:\n%s", joined)
	}
	clean, err := CheckDir("../../pkg/testkit/sample/ent/schema")
	if err != nil || len(clean) != 0 {
		t.Fatalf("sample schemas: %v %v", clean, err)
	}
}
