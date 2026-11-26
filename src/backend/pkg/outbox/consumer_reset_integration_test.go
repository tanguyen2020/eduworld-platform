//go:build integration

package outbox_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver "pgx"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// F130-AC-17 (BACKUP_RECOVERY §3.5): after scripts/ops/reset-consumers.sh recreated a durable with
// deliver_policy=by_start_time, the binary starting again keeps that position instead of failing on an update of the
// deliver policy or replaying the whole stream.
func TestNewConsumerKeepsResetStartPosition(t *testing.T) {
	ctx := context.Background()
	_, url := testkit.StartNATS(ctx, t)
	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := js.CreateStream(ctx, jetstream.StreamConfig{Name: "EVT_LEARNING", Subjects: []string{"ew.evt.learning.>"}}); err != nil {
		t.Fatal(err)
	}
	t0 := time.Now().Add(-time.Hour).UTC().Truncate(time.Second)
	// What reset-consumers.sh leaves behind: same durable/filter/ack, start at T.
	if _, err := js.CreateConsumer(ctx, "EVT_LEARNING", jetstream.ConsumerConfig{Durable: "game_progress",
		FilterSubjects: []string{"ew.evt.learning.>"}, AckPolicy: jetstream.AckExplicitPolicy, MaxDeliver: -1,
		DeliverPolicy: jetstream.DeliverByStartTimePolicy, OptStartTime: &t0}); err != nil {
		t.Fatal(err)
	}
	db, _ := sql.Open("pgx", "postgres://unused@127.0.0.1:1/unused") // lazy: never used by NewConsumer
	t.Cleanup(func() { _ = db.Close() })
	_, err = outbox.NewConsumer(ctx, outbox.ConsumerOptions{Module: "game", Name: "game.progress", Stream: "EVT_LEARNING",
		Subjects: []string{"ew.evt.learning.>"}, UoW: &entx.UnitOfWork{}, DLQDB: db, JS: js,
		Handler: func(context.Context, outbox.Envelope) error { return nil }})
	if err != nil {
		t.Fatalf("binary restart over a reset durable failed: %v", err)
	}
	c, err := js.Consumer(ctx, "EVT_LEARNING", "game_progress")
	if err != nil {
		t.Fatal(err)
	}
	got := c.CachedInfo().Config
	if got.DeliverPolicy != jetstream.DeliverByStartTimePolicy || got.OptStartTime == nil || !got.OptStartTime.Equal(t0) {
		t.Fatalf("start position lost: policy %v start %v", got.DeliverPolicy, got.OptStartTime)
	}
}
