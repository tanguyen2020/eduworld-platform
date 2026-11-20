//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/catalog"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// memOption is one nickname option of memOptions.
type memOption struct {
	nickname string
	expires  time.Time
}

// memOptions is an in-memory TEST DOUBLE of port.NicknameOptionStore driven by the test clock (R17); the Valkey
// adapter has its own test (profile_options_valkey_test.go).
type memOptions struct {
	mu   sync.Mutex
	now  func() time.Time
	sets map[uuid.UUID]map[uuid.UUID]memOption
}

func (m *memOptions) Save(_ context.Context, student uuid.UUID, options map[uuid.UUID]string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	set := m.sets[student]
	if set == nil {
		set = map[uuid.UUID]memOption{}
		m.sets[student] = set
	}
	exp := m.now().Add(ttl)
	for id := range set { // EXPIRE of the whole hash
		o := set[id]
		o.expires = exp
		set[id] = o
	}
	for id, n := range options {
		set[id] = memOption{nickname: n, expires: exp}
	}
	return nil
}

func (m *memOptions) Get(_ context.Context, student, option uuid.UUID) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.sets[student][option]
	if !ok || !m.now().Before(o.expires) {
		return "", false, nil
	}
	return o.nickname, true, nil
}

func (m *memOptions) Clear(_ context.Context, student uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sets, student)
	return nil
}

// put stores one option directly (a suggestion "received" by student).
func (m *memOptions) put(student uuid.UUID, nickname string) uuid.UUID {
	id := uuid.New()
	_ = m.Save(context.Background(), student, map[uuid.UUID]string{id: nickname}, 10*time.Minute)
	return id
}

func seedFiles(t *testing.T) ([]byte, []catalog.ItemDefinition) {
	t.Helper()
	dir := filepath.Join(testkit.RepoRoot(t), "src", "backend", "modules", "game", "seed")
	payload, err := os.ReadFile(filepath.Join(dir, "character_catalog.v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "items.default.v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var items []catalog.ItemDefinition
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatal(err)
	}
	return payload, items
}

// extraItems are non-default cosmetics of the tests (F075 grants them; here inserted into the inventory by hand).
var extraItems = []catalog.ItemDefinition{
	{ItemID: "outfit_knight_01", Kind: "outfit", EquipSlot: "outfit", Rarity: "rare", Name: catalog.Name{VI: "Bộ hiệp sĩ", EN: "Knight"},
		Sources: []string{"shop"}},
	{ItemID: "emote_bow", Kind: "emote", EquipSlot: "emote", Rarity: "rare", Name: catalog.Name{VI: "Cúi chào", EN: "Bow"},
		Sources: []string{"quest"}},
}

// publishCatalog publishes the seed catalog v1 (and extra items) through the publish use case under a PlatformScope,
// as cmd/seed does (T008); payload overrides the seed catalog when given.
func (e *env) publishCatalog(ctx context.Context, t *testing.T, payload ...[]byte) {
	t.Helper()
	seed, items := seedFiles(t)
	if len(payload) > 0 {
		seed = payload[0]
	}
	sc, err := authz.SystemPlatformScope("game-seed")
	if err != nil {
		t.Fatal(err)
	}
	h := command.CatalogPublish{UoW: e.uow, Store: write.CatalogRepo{}, Audit: write.Auditor{R: e.rec}, Now: e.clock.Now}
	if _, err := h.Publish(tenancy.WithScope(ctx, sc), seed, append(items, extraItems...), "test"); err != nil {
		t.Fatal(err)
	}
}

// own inserts an inventory item of s (what F075 does on a reward grant).
func (e *env) own(ctx context.Context, t *testing.T, s student, item string) {
	t.Helper()
	if _, err := e.platform.ExecContext(ctx, `INSERT INTO game.inventory_items (inventory_item_id, tenant_id, created_at, updated_at,
		student_id, item_id, school_id, acquired_at) VALUES ($1, $2, now(), now(), $3, $4, $5, now())`, uuid.New(), s.tenant, s.id, item,
		s.school); err != nil {
		t.Fatal(err)
	}
}

// gtoken is a game token of s (new Keycloak session).
func (e *env) gtoken(s student) string { return e.gameToken(s, uuid.NewString()) }

func th(s student) map[string]string { return map[string]string{"X-Tenant-Id": s.tenant.String()} }

func (e *env) profile(t *testing.T, s student) resp {
	t.Helper()
	return e.do(t, http.MethodGet, "/game/profile", e.gtoken(s), th(s), nil)
}

func (e *env) putAvatar(t *testing.T, s student, cfg any) resp {
	t.Helper()
	return e.do(t, http.MethodPut, "/game/profile/avatar", e.gtoken(s), th(s), cfg)
}

func (e *env) patchProfile(t *testing.T, s student, body any) resp {
	t.Helper()
	h := th(s)
	h["Content-Type"] = "application/merge-patch+json"
	return e.do(t, http.MethodPatch, "/game/profile", e.gtoken(s), h, body)
}

func (e *env) suggest(t *testing.T, s student, count int, lang string) resp {
	t.Helper()
	h := th(s)
	if lang != "" {
		h["Accept-Language"] = lang
	}
	path := "/game/profile/nickname-suggestions"
	if count > 0 {
		path += "?count=" + strconv.Itoa(count)
	}
	return e.do(t, http.MethodGet, path, e.gtoken(s), h, nil)
}

// avatar is a valid AvatarConfig of the seed catalog (preset_explorer without emotes: the server fills them).
func avatar() map[string]any {
	return map[string]any{"schema_version": 1, "body_type": "kid_a", "skin_tone": "tone_02",
		"hair":        map[string]any{"item_id": "hair_short_01", "color": "hair_col_01"},
		"outfit":      map[string]any{"item_id": "outfit_explorer_01"},
		"accessories": []any{map[string]any{"slot": "back", "item_id": "acc_backpack_owl"}}}
}

func with(m map[string]any, k string, v any) map[string]any {
	out := map[string]any{}
	for kk, vv := range m {
		out[kk] = vv
	}
	out[k] = v
	return out
}

// ready gives s an avatar and a nickname (setup done).
func (e *env) ready(t *testing.T, s student, nickname string) {
	t.Helper()
	if r := e.putAvatar(t, s, avatar()); r.status != http.StatusOK {
		t.Fatalf("avatar: %d %s", r.status, r.raw)
	}
	if r := e.patchProfile(t, s, map[string]any{"nickname_option_id": e.options.put(s.id, nickname)}); r.status != http.StatusOK {
		t.Fatalf("nickname: %d %s", r.status, r.raw)
	}
}

// catalogPayload returns the seed payload with nickname words replaced (tests on taken nicknames).
func catalogPayload(t *testing.T, words map[string]any) []byte {
	t.Helper()
	seed, _ := seedFiles(t)
	var p map[string]any
	if err := json.Unmarshal(seed, &p); err != nil {
		t.Fatal(err)
	}
	p["nickname_parts"] = words
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// doRaw sends a raw JSON body.
func (e *env) doRaw(t *testing.T, method, path, token string, headers map[string]string, body string) resp {
	t.Helper()
	req, err := http.NewRequest(method, e.srv.URL+"/api/v1"+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := e.srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	b, _ := io.ReadAll(res.Body)
	out := resp{status: res.StatusCode, header: res.Header, raw: string(b)}
	_ = json.Unmarshal(b, &out.body)
	return out
}
