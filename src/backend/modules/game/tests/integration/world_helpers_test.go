//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/worldsql"
	writerepo "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/seedfiles"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// Seed ids of the world seed v1 (src/backend/modules/game/seed/world, generated with fixed UUIDs).
var (
	questCh0Last = seedID(0x71100, 0*16+2)
	questCh1A    = seedID(0x71100, 1*16+1)
	questCh1B    = seedID(0x71100, 1*16+2)
	questCh2A    = seedID(0x71100, 2*16+1)
	questCh2B    = seedID(0x71100, 2*16+2)
	questCh0A    = seedID(0x71100, 0*16+1)
	levelMainHub = seedID(0x71200, 1)
)

func seedID(base, n int) string {
	return uuid.MustParse("0193a5f4-6c1e-7b2a-9d1f-" + hex12(base+n)).String()
}

func hex12(v int) string {
	const digits = "0123456789abcdef"
	b := make([]byte, 12)
	for i := 11; i >= 0; i-- {
		b[i] = digits[v&0xf]
		v >>= 4
	}
	return string(b)
}

// publisher is the release core of game content on the test databases.
func (e *env) publisher() command.PublishGameContent {
	return command.PublishGameContent{UoW: e.uow, Store: writerepo.GameContentRepo{}, Content: worldsql.Content{},
		Events: writerepo.Publisher{W: e.writer}, Audit: writerepo.Auditor{R: e.rec}, Now: e.clock.Now}
}

// publish runs PublishGameContent under the system platform scope of the seed.
func (e *env) publish(ctx context.Context, t *testing.T, items []command.ContentItem, note string) (command.GameContentResult, error) {
	t.Helper()
	sc, err := authz.SystemPlatformScope("game-seed")
	if err != nil {
		t.Fatal(err)
	}
	return e.publisher().Publish(tenancy.WithScope(ctx, sc), items, note, "")
}

// worldSeed returns the authored items of the world seed v1.
func worldSeed(t *testing.T) []command.ContentItem {
	t.Helper()
	items, err := seedfiles.World(filepath.Join(testkit.RepoRoot(t), "src", "backend", "modules", "game", "seed"))
	if err != nil {
		t.Fatal(err)
	}
	return items
}

// publishWorld publishes the world seed v1 and rebuilds the manifests (as the consumer would).
func (e *env) publishWorld(ctx context.Context, t *testing.T) command.GameContentResult {
	t.Helper()
	res, err := e.publish(ctx, t, worldSeed(t), "seed v1")
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// gameManifest is the manifest builder of game content.
func (e *env) gameManifest() command.GameManifest {
	return command.GameManifest{UoW: e.uow, Store: writerepo.ManifestRepo{}, Tenants: writerepo.ManifestRepo{}, Content: worldsql.Content{},
		Events: writerepo.Publisher{W: e.writer}, Metrics: observability.ManifestMetrics{}, Now: e.clock.Now}
}

// rebuildManifests delivers one game content.published to the consumer game.manifest_on_game_content (platform scope).
func (e *env) rebuildManifests(ctx context.Context, t *testing.T) {
	t.Helper()
	sc, err := authz.SystemPlatformScope("consumer-game-manifest-on-game-content")
	if err != nil {
		t.Fatal(err)
	}
	c := consumer.ManifestOnGameContent{Manifests: e.gameManifest()}
	payload, _ := json.Marshal(map[string]any{"item_type": write.ItemWorldArea, "release_id": uuid.New(), "released_at": e.clock.Now()})
	err = e.uow.InTenantTx(ctx, sc, func(ctx context.Context) error {
		return c.Handle(ctx, outbox.Envelope{EventID: uuid.New(), EventType: "content.published", SchemaVersion: 1, OccurredAt: time.Now().UTC(),
			Producer: "game", Payload: payload})
	})
	if err != nil {
		t.Fatal(err)
	}
}

// item returns a copy of the seed item of type whose key field equals id, modified by edit.
func seedItem(t *testing.T, typ, key, id string, edit func(m map[string]any)) command.ContentItem {
	t.Helper()
	for _, it := range worldSeed(t) {
		if it.Type != typ {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(it.Payload, &m); err != nil {
			t.Fatal(err)
		}
		if m[key] != id {
			continue
		}
		if edit != nil {
			edit(m)
		}
		b, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		return command.ContentItem{Type: typ, Payload: b}
	}
	t.Fatalf("no seed %s %s", typ, id)
	return command.ContentItem{}
}

// ctxOf is the request context of student s (principal + TenantScope), as the httpx chain builds it.
func ctxOf(t *testing.T, _ *env, s student) context.Context {
	t.Helper()
	p := authz.NewPrincipal(authztestStudent(s.id, s.tenant))
	sc, err := authz.ScopeFor(p)
	if err != nil {
		t.Fatal(err)
	}
	return tenancy.WithScope(authz.WithPrincipal(context.Background(), p), sc)
}
