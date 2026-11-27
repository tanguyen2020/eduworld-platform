//go:build integration

package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/cmd/internal/wiring"
	"github.com/tanguyen2020/eduworld-platform/src/backend/cmd/internal/wiring/wiringtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/buildinfo"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/health"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/module"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/reconcile"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

func open(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func exec(t *testing.T, db *sql.DB, q string, args ...any) {
	t.Helper()
	if _, err := db.Exec(q, args...); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
}

func insertOutbox(t *testing.T, db *sql.DB, id uuid.UUID, typ string, tenant *uuid.UUID, at time.Time, published *time.Time) {
	t.Helper()
	exec(t, db, `INSERT INTO infra.outbox_events (event_id, event_type, subject, tenant_id, partition_key, envelope, created_at, published_at)
		VALUES ($1, $2, 'test', $3, 'test', '{"payload":{"student_name":"never returned"}}', $4, $5)`, id, typ, tenant, at, published)
}

func insertProcessed(t *testing.T, db *sql.DB, consumer string, id uuid.UUID, at time.Time) {
	t.Helper()
	exec(t, db, `INSERT INTO infra.processed_events (consumer, event_id, event_occurred_at, processed_at) VALUES ($1, $2, $3, $3)`,
		consumer, id, at)
}

// F134-AC-01, AC-02, AC-03, AC-05, AC-06 (TC-SVC-014) on the real module databases: learning "restored" to T (its
// outbox holds the events up to T; game, chat and competition consumers keep the events they processed after T),
// identity events after T for the learning consumers. The command reads only ids through the five facades in process
// on the <module>_platform pools, never the envelope, and records one audit ops.reconcile.completed.
func TestReconcileRestoreOnModuleDatabases(t *testing.T) {
	ctx := context.Background()
	cluster := testkit.StartCluster(ctx, t, true)
	_, vkAddr := testkit.StartValkey(ctx, t)
	env := wiringtest.Env(wiringtest.Options{RepoRoot: testkit.RepoRoot(t), DSN: cluster.ModuleDSN, Issuer: "http://127.0.0.1:1/realms/eduworld",
		ValkeyAddr: vkAddr, NATSURL: "nats://127.0.0.1:1", SecretsDir: t.TempDir()})
	getenv := func(k string) string { return env[k] }

	// Which consumers read activity.graded of learning and tenant.updated of identity (declared by the modules).
	reg := module.NewRegistry("worker", "reconcile", health.NewRegistry(buildinfo.Get("worker", "reconcile")), getenv)
	nc, err := connectLazyNATS(getenv)
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	reg.SetNATS(nc)
	if _, err := wiring.Compose(reg); err != nil {
		t.Fatal(err)
	}
	accepts := func(i module.ConsumerInfo, typ string) bool {
		return slices.Contains(i.EventTypes, typ) || slices.Contains(i.EventTypes, "*")
	}
	graded := map[string][]string{} // module → consumers of learning activity.graded
	var identityConsumers []string  // learning consumers of identity tenant.updated
	for _, i := range reg.ConsumerInfos("") {
		if i.Source == "learning" && i.Module != "learning" && accepts(i, "activity.graded") {
			graded[i.Module] = append(graded[i.Module], i.Name)
		}
		if i.Source == "identity" && i.Module == "learning" && accepts(i, "tenant.updated") {
			identityConsumers = append(identityConsumers, i.Name)
		}
	}
	reg.Close()
	if len(graded["game"]) == 0 || len(identityConsumers) == 0 {
		t.Fatalf("consumers: graded %v, identity→learning %v", graded, identityConsumers)
	}

	super := func(m string) *sql.DB { return open(t, cluster.DSN("postgres", "eduworld_"+m)) }
	learningDB, identityDB := super("learning"), super("identity")
	now := time.Now().UTC().Truncate(time.Millisecond)
	T := now.Add(-2 * time.Hour)
	tenant := ids.New()
	var kept, lost []uuid.UUID
	for i := 1; i <= 5; i++ { // published and processed before T
		at := T.Add(-time.Duration(i) * 10 * time.Minute)
		id := ids.Floor(at)
		kept = append(kept, id)
		insertOutbox(t, learningDB, id, "activity.graded", &tenant, at, &at)
	}
	for i := 1; i <= 3; i++ { // processed by game after T, lost by the restore of learning
		lost = append(lost, ids.Floor(T.Add(time.Duration(i)*10*time.Minute)))
	}
	for m, consumers := range graded {
		db := super(m)
		for _, c := range consumers {
			for _, id := range kept {
				at, _ := ids.Time(id)
				insertProcessed(t, db, c, id, at)
			}
		}
	}
	gameDB := super("game")
	for _, id := range lost {
		at, _ := ids.Time(id)
		insertProcessed(t, gameDB, graded["game"][0], id, at)
	}
	// identity published tenant.updated after T; learning reprocessed it (consumer reset to T).
	tu := T.Add(20 * time.Minute)
	tuID := ids.Floor(tu)
	insertOutbox(t, identityDB, tuID, "tenant.updated", nil, tu, &tu)
	for _, c := range identityConsumers {
		insertProcessed(t, learningDB, c, tuID, tu)
	}

	run := func(env map[string]string) (int, reconcile.Report, string) {
		t.Helper()
		ge := func(k string) string { return env[k] }
		var out, errb bytes.Buffer
		code := runReconcile(ctx, []string{"--module=learning", "--from=" + T.Add(-3*time.Hour).Format(time.RFC3339),
			"--restore-point=" + T.Format(time.RFC3339), "--settle=10m"}, reconcileDeps{Getenv: ge, Peers: composeReconcile(ge)}, &out, &errb)
		var r reconcile.Report
		if code != reconcile.ExitBadArgs {
			if err := json.Unmarshal(out.Bytes(), &r); err != nil {
				t.Fatalf("report: %v\n%s\n%s", err, out.String(), errb.String())
			}
		}
		if strings.Contains(out.String(), "never returned") {
			t.Fatal("the envelope reached the report (R14)")
		}
		return code, r, errb.String()
	}

	// 1. restored learning, consumer of identity reset: lost_after_restore = 3, unexplained = 0, exit 0 (AC-01, AC-03).
	code, r, logs := run(env)
	if code != reconcile.ExitOK || r.Counts.LostAfterRestore != 3 || r.Counts.Unexplained != 0 || r.Status != "complete" {
		t.Fatalf("run 1: exit %d counts %+v status %s\n%s", code, r.Counts, r.Status, logs)
	}
	for _, m := range r.Mismatches {
		if m.Classification == reconcile.LostAfterRestore && (m.Consumer != graded["game"][0] || !slices.Contains(lost, m.EventID)) {
			t.Fatalf("lost mismatch %+v", m)
		}
	}

	// 2. AC-02: the learning consumer was not reset — the identity event after T is unprocessed → unexplained, exit 2.
	tu2 := T.Add(30 * time.Minute)
	insertOutbox(t, identityDB, ids.Floor(tu2), "tenant.updated", nil, tu2, &tu2)
	code, r, logs = run(env)
	if code != reconcile.ExitUnexplained || r.Counts.Unexplained != len(identityConsumers) {
		t.Fatalf("run 2: exit %d counts %+v\n%s", code, r.Counts, logs)
	}
	for _, c := range identityConsumers {
		insertProcessed(t, learningDB, c, ids.Floor(tu2), tu2) // reset consumer reprocesses it → back to 0
	}
	if code, r, logs = run(env); code != reconcile.ExitOK || r.Counts.Unexplained != 0 {
		t.Fatalf("run 2b: exit %d counts %+v\n%s", code, r.Counts, logs)
	}

	// 3. AC-03: module game unreadable (its platform pool points nowhere) → status incomplete, exit 3.
	down := map[string]string{}
	for k, v := range env {
		down[k] = v
	}
	down["EW_DB_GAME_PLATFORM_DSN"] = "postgres://game_platform:x@127.0.0.1:1/eduworld_game?connect_timeout=1"
	code, r, logs = run(down)
	if code != reconcile.ExitIncomplete || r.Status != "incomplete" {
		t.Fatalf("run 3: exit %d status %s\n%s", code, r.Status, logs)
	}

	// AC-04: a window longer than outbox_retention never composes the modules.
	var out, errb bytes.Buffer
	if c := runReconcile(ctx, []string{"--module=learning", "--from=" + now.Add(-40*24*time.Hour).Format(time.RFC3339),
		"--restore-point=" + T.Format(time.RFC3339)}, reconcileDeps{Getenv: getenv, Peers: nil}, &out, &errb); c != reconcile.ExitBadArgs {
		t.Fatalf("long window: exit %d", c)
	}

	// AC-06: read only — one ops.reconcile.completed audit per run in the outbox of learning (4 runs), no envelope
	// readable by the platform role.
	var audits int
	if err := learningDB.QueryRow(`SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope->'payload'->>'action' = 'ops.reconcile.completed'`).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if audits != 4 {
		t.Fatalf("ops.reconcile.completed audits = %d, want 4", audits)
	}
	platform := open(t, cluster.ModuleDSN("learning", "platform"))
	if _, err := platform.Exec(`SELECT envelope FROM infra.outbox_events LIMIT 1`); err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("learning_platform reads the envelope: %v", err)
	}
	if _, err := platform.Exec(`UPDATE infra.processed_events SET processed_at = now()`); err == nil {
		t.Fatal("learning_platform may update processed_events")
	}
}
