//go:build integration

package integration

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/loginsync"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/metrics"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// lookupStub is a TEST DOUBLE (R17) of SisLookupPort: the contract_v1 adapter is F017.
type lookupStub struct {
	calls  atomic.Int32
	record map[string]any
}

func (l *lookupStub) Lookup(context.Context, uuid.UUID, string, string) (map[string]any, error) {
	l.calls.Add(1)
	if l.record == nil {
		return nil, port.ErrSisLookupNotFound
	}
	return l.record, nil
}

// F011-AC-18, AC-19, AC-20, AC-21 with the real state, pipeline and IDENTITY_SIS_JOBS stream.
func TestLoginSync(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	_, natsURL := testkit.StartNATS(ctx, t)
	nc, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	if err := messaging.EnsureSisJobsStream(ctx, js, 1); err != nil {
		t.Fatal(err)
	}
	if err := messaging.EnsureSisJobsStream(ctx, js, 1); err != nil { // idempotent at start-up
		t.Fatal(err)
	}

	a := seedTenant(ctx, t, e.platform, "pgd-login")
	ta, school := uuid.MustParse(a.tenant), uuid.MustParse(a.school)
	sa := authztest.Ctx(ctx, authztest.Platform("system_admin"))
	conns := command.SisConnections{UoW: e.uow, Connections: write.SisConnectionRepo{}, Audit: e.deps.Audit}
	csvConn, err := conns.Create(sa, ta, "csv_import", "pilot")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conns.SetSchools(sa, csvConn.ID, []uuid.UUID{school}, "pilot"); err != nil {
		t.Fatal(err)
	}
	// A contract_v1 connection declaring student_lookup (created by F017; inserted here as platform).
	apiConn := uuid.New()
	if _, err := e.platform.ExecContext(ctx, `INSERT INTO sis.sis_connections (sis_connection_id, tenant_id, created_at, updated_at, provider_kind,
		config, secret_ref, status) VALUES ($1, $2, now(), now(), 'contract_v1', '{"capabilities":["student_lookup"]}', 'vault:test/sis', 'active')`, apiConn, a.tenant); err != nil {
		t.Fatal(err)
	}
	if _, err := conns.SetSchools(sa, apiConn, []uuid.UUID{uuid.MustParse(a.otherSchool)}, "pilot"); err != nil {
		t.Fatal(err)
	}
	run := command.SisRun{UoW: e.uow, Savepoints: e.uow, Connections: write.SisConnectionRepo{}, Sync: write.SisSyncRepo{},
		Enrollments: write.EnrollmentRepo{}, Missing: write.SisSyncRepo{}, Rules: write.SisSyncRepo{}, Transfers: write.SisSyncRepo{}, Students: write.StudentRepo{},
		Events: e.deps.Events, Audit: e.deps.Audit}
	admin := authztest.Ctx(ctx, authztest.Staff(ta, "school_admin"))
	files := map[string]string{
		"classes.csv":     "class_external_id,school_external_id,academic_year,grade_level,name\nC-3B,TH-A,2026-2027,3,3B\n",
		"students.csv":    "student_external_id,school_external_id,student_code,full_name,grade_level,status,login_username\nHS-1,TH-A,001,An,3,active,hs.l.an\nHS-2,TH-A,002,Binh,3,active,hs.l.binh\n",
		"enrollments.csv": "student_external_id,class_external_id,start_date,status\nHS-1,C-3B,2026-09-05,active\n",
	}
	if _, err := run.Handle(admin, command.RunImportInput{ConnectionID: csvConn.ID, SchoolID: school, Files: sisFiles(t, files)}); err != nil {
		t.Fatal(err)
	}
	userOf := func(username string) uuid.UUID {
		t.Helper()
		var id uuid.UUID
		if err := e.platform.QueryRowContext(ctx, `SELECT user_id FROM identity.users WHERE username = $1`, username).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	lookup := &lookupStub{}
	ls := loginsync.LoginSync{UoW: e.uow, State: write.SisSyncRepo{}, Lookup: lookup, Apply: run, Jobs: messaging.JobQueue{JS: js},
		Metrics: metrics.LoginSync{}, Guard: loginsync.NewGuard(20, 5, 30*time.Second)}
	scope, _ := authz.EventScope(ta)
	inA := tenancy.WithScope(ctx, scope)

	// AC-18: complete and fresh → no SIS call.
	fresh := testutil.ToFloat64(metrics.LoginSyncCounter().WithLabelValues("fresh", "ok"))
	if res, err := ls.Ensure(inA, userOf("hs.l.an")); err != nil || res.Branch != loginsync.BranchFresh || lookup.calls.Load() != 0 {
		t.Fatalf("fresh = %+v %v", res, err)
	}
	if testutil.ToFloat64(metrics.LoginSyncCounter().WithLabelValues("fresh", "ok")) != fresh+1 {
		t.Fatal("metric fresh/ok")
	}
	// AC-20: csv_import connection, no class → no lookup, NO_ACTIVE_ENROLLMENT.
	if _, err := ls.Ensure(inA, userOf("hs.l.binh")); code(err) != problem.NoActiveEnrollment || lookup.calls.Load() != 0 {
		t.Fatalf("csv_import without class: %v calls=%d", err, lookup.calls.Load())
	}

	// AC-21: stale data → login continues, exactly one job in 10 minutes for repeated logins.
	if _, err := e.platform.ExecContext(ctx, `UPDATE sis.synced_students SET last_synced_at = now() - interval '25 hours' WHERE student_external_id = 'HS-1' AND tenant_id = $1`, a.tenant); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if res, err := ls.Ensure(inA, userOf("hs.l.an")); err != nil || res.Branch != loginsync.BranchBackground || res.Outcome != loginsync.ResultOK {
			t.Fatalf("background = %+v %v", res, err)
		}
	}
	info, err := js.Stream(ctx, messaging.SisJobsStream)
	if err != nil {
		t.Fatal(err)
	}
	if si, err := info.Info(ctx); err != nil || si.State.Msgs != 1 {
		t.Fatalf("queued jobs = %+v %v (want 1, deduplicated by Nats-Msg-Id)", si, err)
	}
	// The worker consumes the job (csv_import: no lookup, nothing to refresh).
	jctx, cancel := context.WithCancel(ctx)
	var handled atomic.Int32
	go func() {
		_ = messaging.ConsumeSisJobs(jctx, js, messaging.SisJobHandlers{LoginRefresh: func(ctx context.Context, job port.LoginRefreshJob) error {
			handled.Add(1)
			sc, _ := authz.EventScope(job.TenantID)
			return ls.Refresh(tenancy.WithScope(ctx, sc), job)
		}})
	}()
	deadline := time.Now().Add(20 * time.Second)
	for handled.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
	cancel()
	if handled.Load() != 1 || lookup.calls.Load() != 0 {
		t.Fatalf("job handled = %d lookups = %d", handled.Load(), lookup.calls.Load())
	}

	// AC-19: a student of a contract_v1 school without class → blocking lookup → pipeline → still no class
	// → NO_ACTIVE_ENROLLMENT and missing_class.
	other := uuid.MustParse(a.otherSchool)
	lookup.record = map[string]any{"student_external_id": "API-7", "school_external_id": "TH-API", "student_code": "007", "full_name": "Chi",
		"grade_level": 3, "status": "active", "login": map[string]any{"username": "hs.l.chi"}, "updated_at": "2026-11-20T07:00:00Z"}
	if _, err := run.ApplyLookup(ctx, ta, apiConn, other, lookup.record); err != nil { // first sync of the record
		t.Fatal(err)
	}
	chi := userOf("hs.l.chi")
	_, err = ls.Ensure(inA, chi)
	if code(err) != problem.NoActiveEnrollment || lookup.calls.Load() != 1 {
		t.Fatalf("blocking without class: %v calls=%d", err, lookup.calls.Load())
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM sis.reconciliation_issues WHERE issue_type = 'missing_class' AND entity_refs->>'user_id' = $1`, chi.String()); n < 1 {
		t.Fatalf("missing_class issues = %d", n)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM sis.sync_runs WHERE mode = 'delta' AND sis_connection_id = $1`, apiConn); n != 2 {
		t.Fatalf("lookup runs = %d", n)
	}
	// With a class the next login is complete (AC-19 ok path through branch A).
	classOther := addClass(ctx, t, e.platform, a.tenant, a.otherSchool, "3C")
	if _, err := e.platform.ExecContext(ctx, `INSERT INTO org.enrollments (enrollment_id, tenant_id, created_at, updated_at, school_id, student_id, class_id, status, start_date, source)
		VALUES ($1, $2, now(), now(), $3, $4, $5, 'active', current_date, 'sis')`, uuid.New(), a.tenant, other, chi, classOther); err != nil {
		t.Fatal(err)
	}
	if res, err := ls.Ensure(inA, chi); err != nil || res.Branch != loginsync.BranchFresh || lookup.calls.Load() != 1 {
		t.Fatalf("after enrollment = %+v %v", res, err)
	}
}
