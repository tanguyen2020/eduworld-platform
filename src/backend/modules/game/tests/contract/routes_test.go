package contract

import (
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	gamehttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
)

func spec(t *testing.T) *openapi3.T {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..", "..")
	doc, err := openapi3.NewLoader().LoadFromFile(filepath.Join(root, "specs", "openapi", "eduworld-api.v1.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

// The F014 operations match the merged spec (x-service game): method + path, public (security: []) only for
// client-config, X-Tenant-Id mode from TenantIdHeader, Idempotency-Key from IdempotencyKey, success status; every
// player route (/game/*, except client-config) carries the game_player guard (ADR-0031, AGENTS R23).
func TestOperationsMatchSpec(t *testing.T) {
	doc := spec(t)
	found := map[string]bool{}
	for path, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			o, ok := gamehttp.Operations[op.OperationID]
			if !ok {
				continue
			}
			id := op.OperationID
			found[id] = true
			if method != o.Method || path != o.Pattern || o.Module != "game" {
				t.Errorf("%s: %s %s (%s), spec %s %s", id, o.Method, o.Pattern, o.Module, method, path)
			}
			if svc, _ := op.Extensions["x-service"].(string); svc != "game" {
				t.Errorf("%s: x-service %v", id, op.Extensions["x-service"])
			}
			public := op.Security != nil && len(*op.Security) == 0
			if public != o.Public {
				t.Errorf("%s: public %v, spec %v", id, o.Public, public)
			}
			if strings.HasPrefix(path, "/game/") && !public && o.Guard&httpx.GuardGamePlayer == 0 {
				t.Errorf("%s: a player route needs the game_player guard", id)
			}
			var params []string
			for _, p := range op.Parameters {
				if p.Ref != "" {
					params = append(params, p.Ref[strings.LastIndex(p.Ref, "/")+1:])
				} else if p.Value != nil {
					params = append(params, p.Value.Name)
				}
			}
			joined := strings.Join(params, ",")
			want := httpx.TenantNone
			if strings.Contains(joined, "TenantIdHeader") {
				want = httpx.TenantRequired
			}
			if o.Tenant != want {
				t.Errorf("%s: tenant mode %v, spec %v", id, o.Tenant, want)
			}
			idem := httpx.IdempotencyNone
			if strings.Contains(joined, "IdempotencyKey") {
				idem = httpx.IdempotencyRequired
			}
			if o.Idempotency != idem {
				t.Errorf("%s: idempotency %v, spec %v", id, o.Idempotency, idem)
			}
			status := 0
			for code := range op.Responses.Map() {
				if n, err := strconv.Atoi(code); err == nil && n >= 200 && n < 300 && (status == 0 || n < status) {
					status = n
				}
			}
			if o.SuccessStatus != status {
				t.Errorf("%s: success %d, spec %d", id, o.SuccessStatus, status)
			}
		}
	}
	if len(found) != len(gamehttp.Operations) || len(found) != 71 { // 7 of F014 + createRealtimeTicket (F090) + 7 of F070 + 8 of F071 + 6 of F072 + 1 of F073 + 4 of F074 + 13 of F075 + 6 of F076 + 18 of F077
		t.Fatalf("F014/F090 operations found in the spec: %v", found)
	}
	for _, id := range gamehttp.PlayerOperations {
		if gamehttp.Operations[id].Guard&httpx.GuardGamePlayer == 0 {
			t.Errorf("%s: player operation without game_player", id)
		}
	}
}
