package cases

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/test/security/authz-matrix/internal/spec"
)

// F132-AC-17 (TC-TEN-024, T024): one oversized value per bounded string field, at the JSON path pkg/httpx names.
func TestLengthCases(t *testing.T) {
	cs, skipped := LengthCases(fixtureOps(t))
	got := map[string]LengthCase{}
	for _, c := range cs {
		got[c.OpID+" "+c.Field] = c
	}
	want := map[string]uint64{
		"gameLogin username": 128, "gameLogin password": 256, "gameLogin meta.note": 10, "gameLogin tags.0": 5,
		"updateThing title": 255,
	}
	if len(got) != len(want) {
		t.Fatalf("cases = %v", keys(got))
	}
	for k, max := range want {
		c, ok := got[k]
		if !ok || c.Max != max {
			t.Fatalf("case %s = %+v", k, c)
		}
		b, _ := json.Marshal(c.Body)
		if !strings.Contains(string(b), strings.Repeat("a", int(max)+1)+`"`) || strings.Contains(string(b), strings.Repeat("a", int(max)+2)) {
			t.Fatalf("%s body %s is not maxLength+1", k, b)
		}
	}
	if b, _ := json.Marshal(got["gameLogin tags.0"].Body); string(b) != `{"tags":["aaaaaa"]}` {
		t.Fatalf("array body %s", b)
	}
	if b, _ := json.Marshal(got["gameLogin meta.note"].Body); string(b) != `{"meta":{"note":"aaaaaaaaaaa"}}` {
		t.Fatalf("nested body %s", b)
	}
	// enum and uuid fields are not length cases; the oneOf string is counted as not covered
	if _, ok := got["updateThing status"]; ok {
		t.Fatal("enum field got a length case")
	}
	if skipped != 1 {
		t.Fatalf("skipped composite = %d, want 1", skipped)
	}
}

func TestHeaderCases(t *testing.T) {
	hs := HeaderCases(fixtureOps(t))
	ids := []string{}
	for _, h := range hs {
		ids = append(ids, h.OpID)
	}
	if strings.Join(ids, ",") != "getPlayerProfile,listCmsThings,updateThing" {
		t.Fatalf("header cases = %v (optional X-Tenant-Id and public operations are excluded)", ids)
	}
}

func TestMinimalBody(t *testing.T) {
	ops := fixtureOps(t)
	for _, op := range ops {
		if op.ID != "updateThing" {
			continue
		}
		b, _ := json.Marshal(MinimalBody(op.Body))
		if string(b) != `{"title":"x"}` {
			t.Fatalf("minimal body %s", b)
		}
	}
}

func TestLengthCasesRealContract(t *testing.T) {
	ops, err := spec.Load(context.Background(), filepath.Join(repoRoot, "specs/openapi/eduworld-api.v1.yaml"),
		filepath.Join(backendRoot, "test/authz/matrix_gen.json"))
	if err != nil {
		t.Fatal(err)
	}
	cs, skipped := LengthCases(ops)
	hs := HeaderCases(ops)
	if len(cs) == 0 || len(hs) == 0 {
		t.Fatalf("length=%d header=%d", len(cs), len(hs))
	}
	t.Logf("%d maxLength cases (%d strings under oneOf/anyOf/allOf not covered), %d missing X-Tenant-Id cases", len(cs), skipped, len(hs))
}

func keys(m map[string]LengthCase) []string {
	out := []string{}
	for k := range m {
		out = append(out, k)
	}
	return out
}
