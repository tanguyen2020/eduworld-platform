package cases

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/test/security/authz-matrix/internal/spec"
)

// F132-AC-16 (TC-SVC-005, T023): every facade path is probed through both gateways, every x-internal operation
// through the public gateway. The x-allowed-callers half (an import of modules/<callee>/api by a module not in
// x-allowed-callers fails `make arch-check`) is the fixture "AC-22" of scripts/ci/test-arch-fixtures.sh, run by
// `make ci-local` — not duplicated here.
func TestInternalProbes(t *testing.T) {
	paths, err := spec.InternalPaths("../spec/testdata/modules")
	if err != nil {
		t.Fatal(err)
	}
	probes := InternalProbes(fixtureOps(t), paths)
	var facade, xinternal int
	for _, p := range probes {
		if strings.Contains(p.Path, "{") {
			t.Fatalf("unfilled path parameter: %s", p.Path)
		}
		switch p.Reason {
		case "facade_path_not_http":
			facade++
			if !p.FromOrigin || !strings.HasPrefix(p.Path, "/internal/v1/") {
				t.Fatalf("facade probe %+v", p)
			}
		case "x_internal_not_public":
			xinternal++
			if p.Gateway != PublicGateway || p.Path != "/game-admin/things/00000000-0000-4000-8000-000000000000" {
				t.Fatalf("x-internal probe %+v", p)
			}
		}
	}
	if facade != 2*2*2 || xinternal != 1 { // 2 paths × 2 gateways × GET/POST; 1 x-internal operation
		t.Fatalf("facade=%d xinternal=%d", facade, xinternal)
	}
}

// No facade path of the real specs is published in the public contract.
func TestNoPublicMirrorOfFacades(t *testing.T) {
	ops, err := spec.Load(context.Background(), filepath.Join(repoRoot, "specs/openapi/eduworld-api.v1.yaml"),
		filepath.Join(backendRoot, "test/authz/matrix_gen.json"))
	if err != nil {
		t.Fatal(err)
	}
	paths, err := spec.InternalPaths(filepath.Join(repoRoot, "specs/openapi/modules"))
	if err != nil {
		t.Fatal(err)
	}
	if m := PublicMirrorsInternal(ops, paths); len(m) > 0 {
		t.Fatalf("facade paths published as public routes: %v", m)
	}
	// and the detector itself works
	fake := []spec.Operation{{ID: "x", Path: "/internal/v1/learning/scores"}}
	if m := PublicMirrorsInternal(fake, []string{"/internal/v1/learning/scores"}); len(m) != 1 {
		t.Fatal("mirror not detected")
	}
	t.Logf("%d facade paths, none public; %d probes", len(paths), len(InternalProbes(ops, paths)))
}
