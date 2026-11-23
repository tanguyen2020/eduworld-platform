//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/natsx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// F073-AC-04 (T013, TC-SVC-003): the consumer learning.minigame_config_projection stores the minigame_config block of a
// content.published of module game (platform event on the game stream) once, immutable: the same event delivered again,
// or another event about the same version with another snapshot, neither adds a row nor changes it; other item types are
// ignored; a block the plugin cannot play or that contradicts the envelope is dead-lettered; the application role cannot
// write the projection.
func TestMinigameConfigProjection(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, true)
	js := jetStream(ctx, t)
	if err := natsx.EnsureStreams(ctx, js, "game", natsx.StreamOptions{}); err != nil {
		t.Fatal(err)
	}
	o := consumer.MinigameConfigProjectionOptions(consumer.MinigameConfigProjection{Upsert: command.UpsertMiniGameConfigSnapshot{Store: write.RefRepo{}}},
		e.uow, e.app, js)
	o.BackOff, o.MaxDeliver = []time.Duration{200 * time.Millisecond}, 3
	cons, err := outbox.NewConsumer(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { defer close(done); _ = cons.Run(runCtx) }()
	t.Cleanup(func() { cancel(); <-done })
	w, err := outbox.NewWriter("game", "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	publish := func(env outbox.Envelope) {
		t.Helper()
		raw, _ := json.Marshal(env)
		if _, err := js.Publish(ctx, natsx.EventSubject("game", "content.published"), raw); err != nil {
			t.Fatal(err)
		}
	}
	envelope := func(payload map[string]any) outbox.Envelope {
		t.Helper()
		env, err := w.Envelope(ctx, outbox.Message{Type: "content.published", Subject: outbox.Subject{Type: "minigame_config", ID: uuid.NewString()},
			Platform: true, Payload: payload})
		if err != nil {
			t.Fatal(err)
		}
		return env
	}
	processed := func(env outbox.Envelope) {
		t.Helper()
		waitFor(t, 15*time.Second, "processed", func() bool {
			return count(ctx, t, e.migrator, `SELECT count(*) FROM infra.processed_events WHERE consumer = 'learning.minigame_config_projection'
				AND event_id = $1`, env.EventID) == 1
		})
	}
	deadLettered := func(env outbox.Envelope) {
		t.Helper()
		waitFor(t, 15*time.Second, "dead letter", func() bool {
			return count(ctx, t, e.migrator, `SELECT count(*) FROM infra.dead_letters WHERE consumer = 'learning.minigame_config_projection'
				AND event_id = $1`, env.EventID) == 1
		})
	}
	cfg := mgSnapshot(uuid.New(), 1, "matching", selectionSource(uuid.NewString(), "matching"), 3, nil)
	first := envelope(cfg.contentPublished(time.Now()))
	publish(first)
	processed(first)
	snapshotOf := func() string {
		var s string
		if err := e.migrator.QueryRowContext(ctx, `SELECT snapshot::text FROM ref.minigame_config_versions WHERE minigame_config_version_id = $1`,
			cfg.versionID).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	stored := snapshotOf()
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM ref.minigame_config_versions WHERE minigame_config_version_id = $1 AND minigame_config_id = $2
		AND version = 1 AND minigame_type = 'matching' AND grade_levels = '{3,7}'`, cfg.versionID, cfg.configID); n != 1 {
		t.Fatal("projected row")
	}
	// The same event again (redelivery), then another event about the same version with a changed snapshot.
	publish(first)
	changed := cfg
	changed.body = map[string]any{}
	for k, v := range cfg.body {
		changed.body[k] = v
	}
	changed.body["question_count"] = 9
	second := envelope(changed.contentPublished(time.Now().Add(time.Minute)))
	publish(second)
	processed(second)
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM ref.minigame_config_versions WHERE minigame_config_id = $1`, cfg.configID); n != 1 {
		t.Fatalf("rows after replay: %d", n)
	}
	if got := snapshotOf(); got != stored {
		t.Fatalf("the projected version changed: %s", got)
	}
	// Another item type of the game stream is ignored.
	area := envelope(map[string]any{"item_type": "world_area", "item_id": "area_vuon_ong_so", "to_version_id": uuid.New(), "to_version": 1,
		"release_id": uuid.New(), "owner_scope": "platform", "tenant_id": nil, "scheduled": false, "published_by": uuid.New(), "released_at": time.Now().UTC()})
	publish(area)
	processed(area)
	// A snapshot the plugin cannot play, a block that contradicts the envelope: dead letters, nothing stored.
	bad := mgSnapshot(uuid.New(), 1, "matching", selectionSource(uuid.NewString(), "matching"), 3, func(m map[string]any) { m["question_count"] = 0 })
	badEnv := envelope(bad.contentPublished(time.Now()))
	publish(badEnv)
	deadLettered(badEnv)
	other := mgSnapshot(uuid.New(), 1, "matching", selectionSource(uuid.NewString(), "matching"), 3, nil)
	p := other.contentPublished(time.Now())
	p["to_version"] = 2
	mismatch := envelope(p)
	publish(mismatch)
	deadLettered(mismatch)
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM ref.minigame_config_versions WHERE minigame_config_id IN ($1, $2)`, bad.configID, other.configID); n != 0 {
		t.Fatal("a refused block was stored")
	}
	// The application role reads the projection but never writes it (only the consumer, learning_platform).
	if _, err := e.app.ExecContext(ctx, `UPDATE ref.minigame_config_versions SET version = 5 WHERE minigame_config_version_id = $1`, cfg.versionID); err == nil {
		t.Fatal("learning_app updated the projection")
	}
	if _, err := e.platform.ExecContext(ctx, `UPDATE ref.minigame_config_versions SET version = 5 WHERE minigame_config_version_id = $1`, cfg.versionID); err == nil {
		t.Fatal("learning_platform updated the projection")
	}
	if n := count(ctx, t, e.app, `SELECT count(*) FROM ref.minigame_config_versions WHERE minigame_config_version_id = $1`, cfg.versionID); n != 1 {
		t.Fatal("learning_app reads the projection")
	}
}
