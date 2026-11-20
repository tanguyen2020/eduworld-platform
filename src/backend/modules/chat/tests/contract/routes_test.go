package contract

import (
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	chathttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/http"
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

// The F095 operations match the merged spec (x-service chat): method + path, never public, X-Tenant-Id mode from
// TenantIdHeader, Idempotency-Key from IdempotencyKey, success status; every /social/ route carries the game_player
// guard (ADR-0031, AGENTS R23, F095-AC-10).
func TestOperationsMatchSpec(t *testing.T) {
	doc := spec(t)
	found := map[string]bool{}
	for path, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			o, ok := chathttp.Operations[op.OperationID]
			if !ok {
				continue
			}
			id := op.OperationID
			found[id] = true
			if method != o.Method || path != o.Pattern || o.Module != "chat" {
				t.Errorf("%s: %s %s (%s), spec %s %s", id, o.Method, o.Pattern, o.Module, method, path)
			}
			if svc, _ := op.Extensions["x-service"].(string); svc != "chat" {
				t.Errorf("%s: x-service %v", id, op.Extensions["x-service"])
			}
			public := op.Security != nil && len(*op.Security) == 0
			if public != o.Public {
				t.Errorf("%s: public %v, spec %v", id, o.Public, public)
			}
			if strings.HasPrefix(path, "/social/") && !public && o.Guard&httpx.GuardGamePlayer == 0 {
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
	if len(found) != len(chathttp.Operations) || len(found) != 14 { // the 14 social operations of F095
		t.Fatalf("F095 operations found in the spec: %v", found)
	}
	for _, id := range chathttp.PlayerOperations {
		if chathttp.Operations[id].Guard&httpx.GuardGamePlayer == 0 {
			t.Errorf("%s: player operation without game_player", id)
		}
	}
}

// F095-AC-09 (AC-SOC-002): no operation of the public API searches players by name, username or a free identifier: no
// player/user listing takes a free-text query parameter, and the social routes take ids only from the caller's own
// relations or a short-lived friend code.
func TestNoPlayerSearch(t *testing.T) {
	doc := spec(t)
	free := map[string]bool{"q": true, "query": true, "search": true, "name": true, "nickname": true, "username": true,
		"display_name": true, "keyword": true}
	for path, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			if !strings.HasPrefix(path, "/social/") && !strings.HasPrefix(path, "/game/") && !strings.HasPrefix(path, "/multiplayer/") &&
				!strings.HasPrefix(path, "/competition/") {
				continue // player routes only (portal routes are staff, R23)
			}
			lower := strings.ToLower(op.OperationID + " " + path)
			if strings.Contains(lower, "search") || strings.Contains(lower, "findplayer") || strings.Contains(lower, "lookup") {
				t.Errorf("%s %s (%s): player search route", method, path, op.OperationID)
			}
			for _, p := range op.Parameters {
				if p.Value != nil && p.Value.In == "query" && free[p.Value.Name] {
					t.Errorf("%s %s (%s): free-text parameter %s", method, path, op.OperationID, p.Value.Name)
				}
			}
		}
	}
}
