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
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/natsx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// F078 (F073 handoff): the current version of a mini-game config follows the release pointer of module game:
// content.published v1, v2 → v2; content.rolled_back to v1 → v1 (no new projected version); content.unpublished → no
// current version; an older event never moves the pointer back.
func TestMinigameConfigPointerFollowsRollback(t *testing.T) {
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
	send := func(typ string, payload map[string]any) {
		t.Helper()
		env, err := w.Envelope(ctx, outbox.Message{Type: typ, Subject: outbox.Subject{Type: "minigame_config", ID: uuid.NewString()}, Platform: true,
			Payload: payload})
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(env)
		if _, err := js.Publish(ctx, natsx.EventSubject("game", typ), raw); err != nil {
			t.Fatal(err)
		}
		waitFor(t, 15*time.Second, "processed "+typ, func() bool {
			return count(ctx, t, e.migrator, `SELECT count(*) FROM infra.processed_events WHERE consumer = 'learning.minigame_config_projection'
				AND event_id = $1`, env.EventID) == 1
		})
	}
	configID := uuid.New()
	v1 := mgSnapshot(configID, 1, "matching", selectionSource(uuid.NewString(), "matching"), 3, nil)
	v2 := mgSnapshot(configID, 2, "matching", selectionSource(uuid.NewString(), "matching"), 3, nil)
	t0 := time.Now().UTC()
	send("content.published", v1.contentPublished(t0))
	send("content.published", v2.contentPublished(t0.Add(time.Minute)))
	current := func() string {
		t.Helper()
		var out string
		ps, err := authz.SystemPlatformScope("test-minigame-pointer")
		if err != nil {
			t.Fatal(err)
		}
		err = e.uow.InTenantTx(tenancy.WithModule(ctx, "learning"), ps, func(ctx context.Context) error {
			row, err := write.MinigameConfigRepo{}.CurrentMinigameConfigVersion(ctx, configID)
			if err != nil || row == nil {
				return err
			}
			out = row.ID.String()
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	if current() != v2.versionID.String() {
		t.Fatal("current after publish v2")
	}
	rb := v1.contentPublished(t0.Add(2 * time.Minute))
	rb["reason"] = "Sửa lỗi"
	send("content.rolled_back", rb)
	if current() != v1.versionID.String() {
		t.Fatal("current after rollback to v1")
	}
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM ref.minigame_config_versions WHERE minigame_config_id = $1`, configID); n != 2 {
		t.Fatalf("projected versions: %d", n)
	}
	// An older event (v2 published at t0+1m) delivered again does not move the pointer back.
	send("content.published", v2.contentPublished(t0.Add(time.Minute)))
	if current() != v1.versionID.String() {
		t.Fatal("stale event moved the pointer")
	}
	send("content.unpublished", map[string]any{"item_type": "minigame_config", "item_id": configID.String(), "release_id": uuid.New(),
		"owner_scope": "platform", "tenant_id": nil, "from_version_id": v1.versionID, "to_version_id": nil, "reason": "Gỡ",
		"released_by": uuid.New(), "released_at": t0.Add(3 * time.Minute)})
	if current() != "" {
		t.Fatal("current after unpublish")
	}
}
