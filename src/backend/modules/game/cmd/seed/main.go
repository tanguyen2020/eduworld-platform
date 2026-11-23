// Command seed loads the dev/pilot seed of module game (F070 T008, F071 T001/T010, LOCAL_DEVELOPMENT §5): the character
// catalog v1 (seed/character_catalog.v1.json: presets, palette, default items and emotes, nickname words vi/en), the
// default item definitions (seed/items.default.v1.json) and the world v1 (seed/world: 12 areas of 8 regions, 7 story
// chapters, 14 dialogues of 13 NPCs, 12 greybox level packages), the quests v1 of F072 (seed/quests, assigned to the
// demo tenants of seed/quests/assignments.v1.json), the progression curve and the reward rules of F074 (seed/progression), the
// items, shop items (one sold to th-demo-a only), knowledge card collection and achievements of F075 (seed/economy) through
// the publish use cases under a PlatformScope —
// never SQL by hand. Running it again publishes nothing (identical versions are skipped).
//
//	go -C src/backend run ./modules/game/cmd/seed [-dir modules/game/seed]
//
// Connections: EW_DB_GAME_DSN (game_app) and EW_DB_GAME_PLATFORM_DSN (game_platform, platform content), or their
// _URL_FILE variants (LOCAL_DEVELOPMENT §3).
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver "pgx"

	_ "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/ent/runtime" // mixin hooks, interceptors
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	writeport "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/catalog"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/quest"
	gameconfig "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/config"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/economyschema"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/economysql"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/progressionsql"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/questsql"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/worldsql"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/progressionschema"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/questschema"
	rewardinfra "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/reward"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/seedfiles"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

func main() {
	dir := flag.String("dir", filepath.Join("modules", "game", "seed"), "directory of the game seed files")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, *dir); err != nil {
		slog.Error("game seed failed", "error", err)
		os.Exit(1)
	}
}

func dsn(key string) (string, error) {
	if v := strings.TrimSpace(os.Getenv(key + "_DSN")); v != "" {
		return v, nil
	}
	if f := strings.TrimSpace(os.Getenv(key + "_URL_FILE")); f != "" {
		b, err := os.ReadFile(f) //nolint:gosec // path from the operator's environment
		if err != nil {
			return "", fmt.Errorf("read %s_URL_FILE: %w", key, err)
		}
		return strings.TrimSpace(string(b)), nil
	}
	return "", fmt.Errorf("set %s_DSN or %s_URL_FILE", key, key)
}

func open(key string) (*sql.DB, error) {
	d, err := dsn(key)
	if err != nil {
		return nil, err
	}
	return sql.Open("pgx", d)
}

// Load reads the catalog payload and the item definitions of dir.
func Load(dir string) ([]byte, []catalog.ItemDefinition, error) {
	payload, err := os.ReadFile(filepath.Join(dir, "character_catalog.v1.json")) //nolint:gosec // repository seed path
	if err != nil {
		return nil, nil, err
	}
	raw, err := os.ReadFile(filepath.Join(dir, "items.default.v1.json")) //nolint:gosec // repository seed path
	if err != nil {
		return nil, nil, err
	}
	var items []catalog.ItemDefinition
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, nil, fmt.Errorf("items.default.v1.json: %w", err)
	}
	return payload, items, nil
}

func run(ctx context.Context, dir string) error {
	payload, items, err := Load(dir)
	if err != nil {
		return err
	}
	app, err := open("EW_DB_GAME")
	if err != nil {
		return err
	}
	defer func() { _ = app.Close() }()
	platform, err := open("EW_DB_GAME_PLATFORM")
	if err != nil {
		return err
	}
	defer func() { _ = platform.Close() }()
	w, err := outbox.NewWriter("game", "core", nil)
	if err != nil {
		return err
	}
	rec := outbox.NewAuditRecorder(w, app)
	tenancy.Configure(tenancy.Hooks{Recorder: rec})
	uow, err := entx.NewUnitOfWork(entx.Options{Module: "game", DBs: entx.DBs{App: app, Platform: platform}, Events: w, Audit: rec})
	if err != nil {
		return err
	}
	sc, err := authz.SystemPlatformScope("game-seed")
	if err != nil {
		return err
	}
	h := command.CatalogPublish{UoW: uow, Store: write.CatalogRepo{}, Audit: write.Auditor{R: rec}}
	res, err := h.Publish(tenancy.WithScope(ctx, sc), payload, items, "seed v1")
	if err != nil {
		return err
	}
	worldItems, err := seedfiles.World(dir)
	if err != nil {
		return err
	}
	questItems, err := seedfiles.Quests(dir)
	if err != nil {
		return err
	}
	progressionItems, err := seedfiles.Progression(dir)
	if err != nil {
		return err
	}
	economyItems, err := seedfiles.Economy(dir)
	if err != nil {
		return err
	}
	qcfg, err := gameconfig.QuestFrom(config.Load())
	if err != nil {
		return err
	}
	wh := command.PublishGameContent{UoW: uow, Store: write.GameContentRepo{}, Content: worldsql.Content{}, Events: write.Publisher{W: w},
		Audit: write.Auditor{R: rec}, QuestContent: questsql.Content{}, Quests: &command.QuestPublishRules{Schema: questschema.Validator{},
			Validator: quest.ValidatorConfig{PassRule: qcfg.PassRule, BlockMaxByBand: qcfg.BlockMax}},
		Progression: &command.ProgressionPublishRules{CurveSchema: progressionschema.Curve{}, RuleSchema: progressionschema.Rule{},
			Appliers: rewardinfra.Types(), Levels: write.GameContentRepo{}, Curves: progressionsql.Content{}},
		Economy: &command.EconomyPublishRules{ItemSchema: economyschema.Item{}, ShopSchema: economyschema.ShopItem{},
			CollectionSchema: economyschema.Collection{}, AchievementSchema: economyschema.Achievement{}, Appliers: rewardinfra.Types(),
			Items: economysql.Content{}, CollectionItems: write.GameContentRepo{}}}
	all := append(append(append(worldItems, questItems...), progressionItems...), economyItems...)
	wres, err := wh.Publish(tenancy.WithScope(ctx, sc), all, "seed world v1", "")
	if err != nil {
		return err
	}
	seedAssign, err := seedfiles.QuestAssignments(dir)
	if err != nil {
		return err
	}
	var assign []command.SeedAssignment
	for _, a := range seedAssign {
		id, err := uuid.Parse(a.QuestID)
		if err != nil {
			return fmt.Errorf("assignments.v1.json: %w", err)
		}
		assign = append(assign, command.SeedAssignment{QuestID: id, Tenants: a.Tenants})
	}
	assigned, skipped, err := command.AssignSeedQuests{UoW: uow, Assignments: write.QuestRepo{}, Audit: write.Auditor{R: rec}}.
		Assign(tenancy.WithScope(ctx, sc), assign)
	if err != nil {
		return err
	}
	if len(skipped) > 0 {
		slog.WarnContext(ctx, "quest assignments skipped: tenants not projected yet, run the seed again after make seed-check", "tenants", skipped)
	}
	shopAssign, err := seedfiles.ShopAssignments(dir)
	if err != nil {
		return err
	}
	var shops []command.SeedAssignment
	for _, a := range shopAssign {
		id, err := uuid.Parse(a.QuestID)
		if err != nil {
			return fmt.Errorf("economy assignments.v1.json: %w", err)
		}
		shops = append(shops, command.SeedAssignment{QuestID: id, Tenants: a.Tenants})
	}
	shopAssigned, shopSkipped, err := command.AssignSeedQuests{UoW: uow, Assignments: write.QuestRepo{}, Audit: write.Auditor{R: rec},
		ItemType: writeport.ItemShopItem}.Assign(tenancy.WithScope(ctx, sc), shops)
	if err != nil {
		return err
	}
	if len(shopSkipped) > 0 {
		slog.WarnContext(ctx, "shop assignments skipped: tenants not projected yet, run the seed again after make seed-check", "tenants", shopSkipped)
	}
	assigned += shopAssigned
	slog.InfoContext(ctx, "game seed done", "catalog_published", res.CatalogPublished, "items_published", res.ItemsPublished,
		"world_published", wres.Published, "world_unchanged", wres.Unchanged, "quest_assignments", assigned)
	return nil
}
