//go:build integration

package partitionmaint_test

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/clock"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx/partitionmaint"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/sample"
)

// F003-AC-36: partitions ≥ 7 days ahead; expired ones dropped; partitions with unpublished outbox rows
// or unresolved dead letters kept; no business table touched; partition_admin has only EXECUTE.
func TestPartitionMaintenance(t *testing.T) {
	ctx := context.Background()
	e := sample.Setup(ctx, t)
	admin := sample.Open(t, e.Cluster.ModuleDSN("learning", "partition_admin"))
	now := time.Now().UTC()
	job, err := partitionmaint.New(partitionmaint.Options{Module: "learning", DB: admin, Clock: clock.NewFake(now),
		DeadLetterRetention: 30 * 24 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	// partition_admin cannot touch tables directly.
	if _, err := admin.ExecContext(ctx, `SELECT count(*) FROM infra.outbox_events`); err == nil {
		t.Fatal("partition_admin can read infra tables")
	}

	// Old partitions (40 days ago) with one unpublished outbox row and one unresolved dead letter.
	old := now.AddDate(0, 0, -40)
	if _, err := e.Migrator.ExecContext(ctx, `SELECT infra.ensure_partitions($1::date, 2)`, old); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Migrator.ExecContext(ctx, `INSERT INTO infra.outbox_events (event_id, event_type, subject, partition_key, envelope, created_at)
		VALUES ($1, 'x.y', 'ew.evt.learning.x.y', 'k', '{}', $2)`, ids.New(), old); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Migrator.ExecContext(ctx, `INSERT INTO infra.dead_letters (dead_letter_id, stream, consumer, event_id, event_type, payload, error, delivery_count, created_at)
		VALUES ($1, 'EVT_LEARNING', 'learning.c', $2, 'x.y', '{}', 'e', 5, $3)`, ids.New(), uuid.New(), old); err != nil {
		t.Fatal(err)
	}

	rep, err := job.RunOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, tbl := range partitionmaint.Tables {
		if rep.DaysAhead[tbl] < partitionmaint.MinDaysAhead {
			t.Fatalf("%s: %d days ahead", tbl, rep.DaysAhead[tbl])
		}
	}
	oldDay := old.Format("20060102")
	has := func(list []string, name string) bool { return slices.Contains(list, name) }
	if !has(rep.Refused, "outbox_events_p"+oldDay) || !has(rep.Refused, "dead_letters_p"+oldDay) {
		t.Fatalf("refused = %v", rep.Refused)
	}
	if !has(rep.Dropped, "processed_events_p"+oldDay) || !has(rep.Dropped, "idempotency_records_p"+oldDay) {
		t.Fatalf("dropped = %v", rep.Dropped)
	}
	for _, d := range rep.Dropped {
		if !strings.HasPrefix(d, "outbox_events_p") && !strings.HasPrefix(d, "processed_events_p") &&
			!strings.HasPrefix(d, "dead_letters_p") && !strings.HasPrefix(d, "idempotency_records_p") {
			t.Fatalf("dropped a non-infra partition %s", d)
		}
	}

	// Once published/resolved, the next run drops them.
	if _, err := e.Migrator.ExecContext(ctx, `UPDATE infra.outbox_events SET published_at = now() WHERE created_at = $1`, old); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Migrator.ExecContext(ctx, `UPDATE infra.dead_letters SET resolved_at = now() WHERE created_at = $1`, old); err != nil {
		t.Fatal(err)
	}
	rep, err = job.RunOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !has(rep.Dropped, "outbox_events_p"+oldDay) || !has(rep.Dropped, "dead_letters_p"+oldDay) || len(rep.Refused) != 0 {
		t.Fatalf("second run: dropped %v refused %v", rep.Dropped, rep.Refused)
	}
	var widgets bool
	_ = e.Migrator.QueryRow(`SELECT to_regclass('sample.widgets') IS NOT NULL`).Scan(&widgets)
	if !widgets {
		t.Fatal("business table touched")
	}
}
