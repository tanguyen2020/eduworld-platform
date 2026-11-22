package contract

import (
	"path/filepath"
	"runtime"
	"slices"
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

// The F095 and F115 operations match the merged spec (x-service chat): method + path, never public, X-Tenant-Id mode
// from TenantIdHeader, Idempotency-Key from IdempotencyKey, success status; every /social/ and /chat/ route carries the
// game_player guard (ADR-0031, AGENTS R23, F095-AC-10, F115-AC-13).
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
			if !servedByChat(op.Extensions["x-service"]) {
				t.Errorf("%s: x-service %v", id, op.Extensions["x-service"])
			}
			public := op.Security != nil && len(*op.Security) == 0
			if public != o.Public {
				t.Errorf("%s: public %v, spec %v", id, o.Public, public)
			}
			if (strings.HasPrefix(path, "/social/") || strings.HasPrefix(path, "/chat/")) && !public && o.Guard&httpx.GuardGamePlayer == 0 {
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
			switch {
			case strings.Contains(joined, "TenantIdHeaderOptional"):
				want = httpx.TenantOptional
			case strings.Contains(joined, "TenantIdHeader"):
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
	// 14 social operations (F095), 3 chat (F115), listQuickMessages (F116 US4), 21 catalogue operations (F116 US2),
	// 4 moderation switch operations (F116 US3), 13 Safety Team operations (F121), 10 message feature operations (F118) and
	// 8 Game Studio sticker/GIF operations (F118: the shared /game-admin/{config_type} ones for sticker-packs/gifs + presign).
	if len(found) != len(chathttp.Operations) || len(found) != 74 {
		t.Fatalf("chat operations found in the spec: %d of %d: %v", len(found), len(chathttp.Operations), found)
	}
	// F116-AC-18: the catalogue and switch administration is internal-gateway only (x-internal, /admin/), except the
	// read-only category view of a tenant (/tenant/chat/categories); none is a player route.
	admin := append(append([]string{}, chathttp.CatalogOperations...), chathttp.SwitchOperations...)
	for _, id := range admin {
		o := chathttp.Operations[id]
		if o.Guard&httpx.GuardGamePlayer != 0 {
			t.Errorf("%s: a catalogue operation is never a player route", id)
		}
		if id != "listTenantSensitiveCategories" && !strings.HasPrefix(o.Pattern, "/admin/chat/") {
			t.Errorf("%s: %s is not under /admin/chat/", id, o.Pattern)
		}
	}
	for path, item := range doc.Paths.Map() {
		for _, op := range item.Operations() {
			if !slices.Contains(admin, op.OperationID) || op.OperationID == "listTenantSensitiveCategories" {
				continue
			}
			if internal, _ := op.Extensions["x-internal"].(bool); !internal {
				t.Errorf("%s %s: catalogue write/read operations must be x-internal (no public gateway route)", op.OperationID, path)
			}
		}
	}
	for _, id := range append(append([]string{"listQuickMessages"}, chathttp.PlayerOperations...), chathttp.ChatOperations...) {
		if chathttp.Operations[id].Guard&httpx.GuardGamePlayer == 0 {
			t.Errorf("%s: player operation without game_player", id)
		}
	}
}

// F121-AC-09, AC-25: the Safety Team operations are internal-gateway only (x-internal, /admin/moderation/ or
// /admin/safety/), never player routes; the school routes are /tenant/safety/ (portal staff); createReport is the only
// player route (game_player).
func TestSafetyOperations(t *testing.T) {
	doc := spec(t)
	for _, id := range chathttp.SafetyOperations {
		o := chathttp.Operations[id]
		switch {
		case id == "createReport":
			if o.Guard&httpx.GuardGamePlayer == 0 || o.Pattern != "/chat/reports" {
				t.Errorf("%s: player route with game_player", id)
			}
		case strings.HasPrefix(o.Pattern, "/tenant/safety/"):
			if o.Guard&httpx.GuardGamePlayer != 0 {
				t.Errorf("%s: a school route is never a player route", id)
			}
		default:
			if o.Guard&httpx.GuardGamePlayer != 0 || (!strings.HasPrefix(o.Pattern, "/admin/moderation/") && !strings.HasPrefix(o.Pattern, "/admin/safety/")) {
				t.Errorf("%s: %s must be under /admin/moderation/ or /admin/safety/", id, o.Pattern)
			}
			pi := doc.Paths.Find(o.Pattern)
			if pi == nil || pi.GetOperation(o.Method) == nil {
				t.Errorf("%s: not in the spec", id)
				continue
			}
			if internal, _ := pi.GetOperation(o.Method).Extensions["x-internal"].(bool); !internal {
				t.Errorf("%s: must be x-internal (internal gateway only, AGENTS R20)", id)
			}
		}
	}
}

// F121-AC-25 (TC-CHAT-009, AC-MOD-007): no operation reads or searches chat messages outside a channel the caller belongs
// to or the context of a moderation case: every chat operation exposing messages is in this closed list.
func TestNoChatBrowsing(t *testing.T) {
	allowed := map[string]bool{"listChannelMessages": true, "sendChannelMessage": true, "listQuickMessages": true, "pinMessage": true,
		"unpinMessage": true, "getMessageReceipts": true, "editMessage": true, "recallMessage": true, "forwardMessage": true,
		"toggleMessageReaction":      true, // F118: a reaction on a message of a channel the caller belongs to
		"searchMessages":             true, // member channels only (F120): tenant, membership, deleted_at, recalled_at
		"listModerationCaseMessages": true} // the case only, assignee or team lead, audited (F121)
	doc := spec(t)
	for path, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			if svc, _ := op.Extensions["x-service"].(string); svc != "chat" {
				continue
			}
			lower := strings.ToLower(op.OperationID + " " + path)
			if (strings.Contains(lower, "message") || strings.Contains(lower, "search")) && !allowed[op.OperationID] &&
				!strings.Contains(lower, "quick") {
				t.Errorf("%s %s (%s): message read/search outside the closed list", method, path, op.OperationID)
			}
		}
	}
	if o := chathttp.Operations["listModerationCaseMessages"]; !strings.HasPrefix(o.Pattern, "/admin/moderation/cases/{case_id}/") {
		t.Errorf("case messages must be scoped to one case: %s", o.Pattern)
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

// servedByChat reports an x-service of module chat: "chat", or a list containing chat (x-service-routing by parameter,
// e.g. /game-admin/{config_type} for sticker-packs and gifs, F118).
func servedByChat(v any) bool {
	switch s := v.(type) {
	case string:
		return s == "chat"
	case []any:
		for _, x := range s {
			if x == "chat" {
				return true
			}
		}
	}
	return false
}
