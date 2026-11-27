package spec

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Repository paths seen from this package (src/backend/test/security/authz-matrix/internal/spec).
const (
	backendRoot = "../../../../.."
	repoRoot    = "../../../../../../.."
)

func TestLoadFixture(t *testing.T) {
	ops, err := Load(context.Background(), "testdata/mini.yaml", "testdata/mini_matrix.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 8 {
		t.Fatalf("ops = %d, want 8", len(ops))
	}
	by := map[string]Operation{}
	for _, o := range ops {
		by[o.ID] = o
	}
	if o := by["updateThing"]; !o.TenantRequired || !o.NeedsIfMatch || !o.Idempotent || o.Body == nil || len(o.PathParams) != 1 {
		t.Fatalf("updateThing = %+v", o)
	}
	if o := by["getLeaderboard"]; !o.TenantScoped || o.TenantRequired {
		t.Fatalf("optional tenant header: %+v", o)
	}
	if o := by["getGameAdminThing"]; !o.Internal || !o.AdminBearer {
		t.Fatalf("x-internal/admin bearer: %+v", o)
	}
	if o := by["gameLogin"]; !o.Public || o.Body == nil {
		t.Fatalf("gameLogin = %+v", o)
	}
}

// A matrix that lost an operation (or still lists a removed one) is stale: the runner refuses to run.
func TestLoadRejectsStaleMatrix(t *testing.T) {
	raw, err := os.ReadFile("testdata/mini_matrix.json")
	if err != nil {
		t.Fatal(err)
	}
	stale := strings.Replace(string(raw), `"/me/tenants"`, `"/me/tenants-old"`, 1)
	p := filepath.Join(t.TempDir(), "matrix.json")
	if err := os.WriteFile(p, []byte(stale), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(context.Background(), "testdata/mini.yaml", p); err == nil || !strings.Contains(err.Error(), "make gen") {
		t.Fatalf("stale matrix accepted: %v", err)
	}
}

// The real contract and the committed routematrix output agree (the matrix covers every public operation).
func TestLoadRealContract(t *testing.T) {
	ops, err := Load(context.Background(), filepath.Join(repoRoot, "specs/openapi/eduworld-api.v1.yaml"),
		filepath.Join(backendRoot, "test/authz/matrix_gen.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) < 500 {
		t.Fatalf("only %d operations loaded", len(ops))
	}
	paths, err := InternalPaths(filepath.Join(repoRoot, "specs/openapi/modules"))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range paths {
		if !strings.HasPrefix(p, "/internal/") {
			t.Fatalf("facade path %s is not under /internal/", p)
		}
	}
	t.Logf("%d public operations, %d facade paths", len(ops), len(paths))
}
