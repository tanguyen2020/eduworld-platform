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
			// F120: a /social/ route the Admin Portal also calls (x-portal-action: listClubs, getClub, approveClub, archiveClub)
			// serves staff of the portal; its use case checks the caller (students never reach staff paths and vice versa).
			_, portal := op.Extensions["x-portal-action"]
			if (strings.HasPrefix(path, "/social/") || strings.HasPrefix(path, "/chat/")) && !public && o.Guard&httpx.GuardGamePlayer == 0 && !portal {
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
	// 8 Game Studio sticker/GIF operations (F118: the shared /game-admin/{config_type} ones for sticker-packs/gifs + presign),
	// 7 chat policy, 2 persistent channel/search and 21 club operations (F120), getTenantModerationStatus (F122 banner),
	// rejectModerationSwitchChange, listModerationCaseTenants, listSchoolClubMembers and removeSchoolClubMember (F122 T060–T064),
	// 9 on-call and break-glass operations (F124), 3 in-game notification operations (F123) and updateChannelMembership
	// (delta F112-9).
	if len(found) != len(chathttp.Operations) || len(found) != 126 { // + 4 of F082 (diff, rollback, unpublish, releases of sticker-packs/gifs)
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
		if id != "listTenantSensitiveCategories" && id != "getTenantModerationStatus" && !strings.HasPrefix(o.Pattern, "/admin/chat/") {
			t.Errorf("%s: %s is not under /admin/chat/", id, o.Pattern)
		}
	}
	for path, item := range doc.Paths.Map() {
		for _, op := range item.Operations() {
			if !slices.Contains(admin, op.OperationID) || op.OperationID == "listTenantSensitiveCategories" || op.OperationID == "getTenantModerationStatus" {
				continue
			}
			if internal, _ := op.Extensions["x-internal"].(bool); !internal {
				t.Errorf("%s %s: catalogue write/read operations must be x-internal (no public gateway route)", op.OperationID, path)
			}
		}
	}
	// F120-AC-11 (TC-TEN-003): the platform chat policy and the common-tenant override are internal-gateway routes only
	// (x-internal, /admin/), never on the public gateway; the tenant/school levels are portal routes (/tenant/).
	for path, item := range doc.Paths.Map() {
		for _, op := range item.Operations() {
			switch op.OperationID {
			case "getPlatformChatPolicy", "putPlatformChatPolicy", "getCommonTenantSocialOverride", "putCommonTenantSocialOverride":
				if internal, _ := op.Extensions["x-internal"].(bool); !internal || !strings.HasPrefix(path, "/admin/chat/") {
					t.Errorf("%s %s: platform chat policy routes are x-internal under /admin/chat/", op.OperationID, path)
				}
			case "getChatPolicy", "putChatPolicy", "getEffectiveChatPolicy":
				if !strings.HasPrefix(path, "/tenant/chat-policies") {
					t.Errorf("%s %s: chat policy levels are portal routes", op.OperationID, path)
				}
			}
		}
	}
	for _, id := range append(append(append([]string{"listQuickMessages"}, chathttp.PlayerOperations...), chathttp.ChatOperations...),
		chathttp.PersistentOperations...) {
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
// to or the context of a moderation case — or a break-glass grant (F124, MODERATION §6.1: the only path outside a case):
// every chat operation exposing messages is in this closed list.
func TestNoChatBrowsing(t *testing.T) {
	allowed := map[string]bool{"listChannelMessages": true, "sendChannelMessage": true, "listQuickMessages": true, "pinMessage": true,
		"unpinMessage": true, "getMessageReceipts": true, "editMessage": true, "recallMessage": true, "forwardMessage": true,
		"toggleMessageReaction":      true, // F118: a reaction on a message of a channel the caller belongs to
		"searchMessages":             true, // member channels only (F120): tenant, membership, deleted_at, recalled_at
		"listModerationCaseMessages": true, // the case only, assignee or team lead, audited (F121)
		"listBreakGlassMessages":     true} // F124: the requester of an approved, unexpired two-person grant, its scope only, audited
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

// F124-AC-16, AC-21 (TC-CMS-WS-02): the on-call and break-glass operations are internal-gateway only (x-internal, under
// /admin/moderation/oncall-shifts or /admin/break-glass/: no route on the public gateway; httpx adds the portal staff and
// platform role guards), never player routes; break-glass writes name the target tenant (X-Tenant-Id required, R20) and
// every write takes an Idempotency-Key (R10).
func TestSafetyOpsOperations(t *testing.T) {
	doc := spec(t)
	writes := map[string]bool{"createModerationOncallShift": true, "deleteModerationOncallShift": true, "createBreakGlassRequest": true,
		"approveBreakGlassRequest": true, "rejectBreakGlassRequest": true, "revokeBreakGlassGrant": true}
	for _, id := range chathttp.SafetyOpsOperations {
		o := chathttp.Operations[id]
		if o.Guard&httpx.GuardGamePlayer != 0 {
			t.Errorf("%s: never a player route", id)
		}
		if !strings.HasPrefix(o.Pattern, "/admin/moderation/oncall-shifts") && !strings.HasPrefix(o.Pattern, "/admin/break-glass/") {
			t.Errorf("%s: %s outside the internal admin routes", id, o.Pattern)
		}
		if writes[id] && o.Idempotency != httpx.IdempotencyRequired {
			t.Errorf("%s: write without Idempotency-Key", id)
		}
		if strings.HasPrefix(o.Pattern, "/admin/break-glass/") && writes[id] && o.Tenant != httpx.TenantRequired {
			t.Errorf("%s: a break-glass write names its target tenant", id)
		}
	}
	n := 0
	for path, item := range doc.Paths.Map() {
		for _, op := range item.Operations() {
			if !slices.Contains(chathttp.SafetyOpsOperations, op.OperationID) {
				continue
			}
			n++
			if internal, _ := op.Extensions["x-internal"].(bool); !internal {
				t.Errorf("%s %s: must be x-internal", op.OperationID, path)
			}
		}
	}
	if n != len(chathttp.SafetyOpsOperations) {
		t.Fatalf("F124 operations in the spec: %d", n)
	}
}
