//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/sis"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/contract"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/contractv1client"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/metrics"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/tests/fakesis"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// memSecrets is a test double of port.SecretStore (in-memory, tests only).
type memSecrets map[string]string

func (s memSecrets) Get(_ context.Context, ref string) ([]byte, error) {
	v, ok := s[ref]
	if !ok {
		return nil, port.ErrSisSecret
	}
	return []byte(v), nil
}

// memLock is a test double of port.RunLock (in-memory, tests only).
type memLock struct {
	mu   sync.Mutex
	held map[uuid.UUID]bool
}

func (l *memLock) Acquire(_ context.Context, id uuid.UUID) (func(), error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.held[id] {
		return nil, port.ErrRunLockBusy
	}
	l.held[id] = true
	return func() { l.mu.Lock(); delete(l.held, id); l.mu.Unlock() }, nil
}

// memJobs is a test double of port.SisSyncJobQueue: it records the jobs (tests only).
type memJobs struct {
	mu    sync.Mutex
	syncs []port.SyncJob
	ids   []string
	hooks []port.WebhookJob
}

func (q *memJobs) EnqueueSync(_ context.Context, j port.SyncJob, id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.syncs, q.ids = append(q.syncs, j), append(q.ids, id)
	return nil
}

func (q *memJobs) EnqueueWebhookBatch(_ context.Context, j port.WebhookJob) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.hooks = append(q.hooks, j)
	return nil
}

func (q *memJobs) last() port.SyncJob {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.syncs[len(q.syncs)-1]
}

type sisAPI struct {
	*env
	fake     *fakesis.Server
	a, b     fixture
	ta       uuid.UUID
	school   uuid.UUID
	jobs     *memJobs
	lock     *memLock
	contract command.SisContract
	sync     command.SisSync
	conns    command.SisConnections
	sa       context.Context
}

const schoolExt = "TH-DEMO-001"

func newSisAPI(ctx context.Context, t *testing.T, e *env) *sisAPI {
	t.Helper()
	s := &sisAPI{env: e, fake: fakesis.New(), jobs: &memJobs{}, lock: &memLock{held: map[uuid.UUID]bool{}}} // MOCK SIS
	t.Cleanup(s.fake.Close)
	s.a, s.b = seedTenant(ctx, t, e.platform, "th-sis-api"), seedTenant(ctx, t, e.platform, "pgd-sis-api")
	s.ta, s.school = uuid.MustParse(s.a.tenant), uuid.MustParse(s.a.school)
	s.fake.APIKey = "k-th-demo-a"
	s.fake.Lock()
	for _, c := range s.fake.Classes[schoolExt] { // the fixture school already has a class 3A
		c["name"] = "S" + c["name"].(string)
	}
	s.fake.Unlock()
	v, err := contract.New()
	if err != nil {
		t.Fatal(err)
	}
	providers := &contractv1client.Factory{Secrets: memSecrets{"sis/th-demo-a": "k-th-demo-a"}, Validator: v, Rate: 1000,
		Sleep: func(context.Context, time.Duration) error { return nil }}
	s.contract = command.SisContract{UoW: e.uow, Connections: write.SisConnectionRepo{}, Repo: write.SisContractRepo{}, Schools: write.SchoolRepo{},
		Providers: providers, Audit: e.deps.Audit, AllowHTTP: true}
	run := command.SisRun{UoW: e.uow, Savepoints: e.uow, Connections: write.SisConnectionRepo{}, Sync: write.SisSyncRepo{},
		Missing: write.SisSyncRepo{}, Rules: write.SisSyncRepo{}, Transfers: write.SisSyncRepo{},
		Students: write.StudentRepo{}, Events: e.deps.Events, Audit: e.deps.Audit, BatchSize: 2}
	s.sync = command.SisSync{UoW: e.uow, Connections: write.SisConnectionRepo{}, Repo: write.SisContractRepo{}, Run: run, Providers: providers,
		Lock: s.lock, Jobs: s.jobs, Metrics: metrics.Sync{}, Observer: metrics.Runs{}}
	s.conns = command.SisConnections{UoW: e.uow, Connections: write.SisConnectionRepo{}, Audit: e.deps.Audit, Contract: &s.contract}
	s.sa = authztest.Ctx(ctx, authztest.Platform("system_admin"))
	return s
}

func (s *sisAPI) create(t *testing.T, webhook bool) uuid.UUID {
	t.Helper()
	in := command.ContractCreateInput{ContractVersion: "1.2.0", SecretRef: "sis/th-demo-a", Reason: "pilot",
		Config: sis.ContractConfig{BaseURL: s.fake.URL + "/eduworld/v1", AuthType: sis.AuthAPIKey,
			Schools: []sis.ContractSchool{{ExternalID: schoolExt, SchoolID: s.school}}}}
	if webhook {
		in.WebhookSecretRef = "sis/th-demo-a-webhook"
	}
	out, err := s.conns.CreateContract(s.sa, s.ta, in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.conns.SetSchools(s.sa, out.ConnectionID, []uuid.UUID{s.school}, "pilot"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.contract.CheckHealth(s.sa, out.ConnectionID); err != nil {
		t.Fatal(err)
	}
	return out.ConnectionID
}

// trigger queues a manual run as System Admin and executes its job like the worker.
func (s *sisAPI) trigger(ctx context.Context, t *testing.T, conn uuid.UUID, kind string, dry bool) map[string]any {
	t.Helper()
	run, err := s.sync.Trigger(s.sa, conn, command.TriggerInput{Kind: kind, DryRun: dry})
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != "queued" {
		t.Fatalf("triggered run = %+v", run)
	}
	if err := s.sync.Execute(ctx, s.jobs.last()); err != nil {
		t.Fatal(err)
	}
	return s.runRow(ctx, t, run.SyncRunID.String())
}

func (s *sisAPI) runRow(ctx context.Context, t *testing.T, id string) map[string]any {
	t.Helper()
	var status, mode, trigger string
	var dry bool
	var failure *string
	var stats []byte
	if err := s.platform.QueryRowContext(ctx, `SELECT status, mode, "trigger", dry_run, failure_code, stats FROM sis.sync_runs WHERE sync_run_id = $1`, id).
		Scan(&status, &mode, &trigger, &dry, &failure, &stats); err != nil {
		t.Fatal(err)
	}
	out := map[string]any{"id": id, "status": status, "mode": mode, "trigger": trigger, "dry_run": dry, "failure_code": "", "stats": map[string]any{}}
	if failure != nil {
		out["failure_code"] = *failure
	}
	var st map[string]any
	_ = json.Unmarshal(stats, &st)
	out["stats"] = st
	return out
}

func (s *sisAPI) connRow(ctx context.Context, t *testing.T, id uuid.UUID) (status, reason string, failed int, wm string) {
	t.Helper()
	var r, w *string
	if err := s.platform.QueryRowContext(ctx, `SELECT status, status_reason, consecutive_failed_runs, delta_watermark FROM sis.sis_connections
		WHERE sis_connection_id = $1`, id).Scan(&status, &r, &failed, &w); err != nil {
		t.Fatal(err)
	}
	if r != nil {
		reason = *r
	}
	if w != nil {
		wm = *w
	}
	return status, reason, failed, wm
}

func (s *sisAPI) version(ctx context.Context, t *testing.T, id uuid.UUID) int64 {
	t.Helper()
	var v int64
	if err := s.platform.QueryRowContext(ctx, `SELECT version FROM sis.sis_connections WHERE sis_connection_id = $1`, id).Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func codeOfErr(err error) problem.Code {
	var pe *problem.Error
	if errors.As(err, &pe) {
		return pe.Code
	}
	return ""
}

// F017-AC-01, AC-02, AC-04, AC-13 (reactivation), AC-30 (domain rules through the use case).
func TestSisContractConnection(t *testing.T) {
	ctx := context.Background()
	s := newSisAPI(ctx, t, setup(ctx, t))
	bad := command.ContractCreateInput{ContractVersion: "1.2.0", SecretRef: "sis/th-demo-a",
		Config: sis.ContractConfig{BaseURL: "ftp://sis.example", AuthType: sis.AuthOAuth2, Schools: []sis.ContractSchool{{ExternalID: schoolExt, SchoolID: s.school}}}}
	if _, err := s.conns.CreateContract(s.sa, s.ta, bad); codeOfErr(err) != problem.ValidationFailed ||
		!strings.Contains(err.Error(), "base_url") && !strings.Contains(err.Error(), "VALIDATION") {
		t.Fatalf("invalid config: %v", err)
	}
	other := bad
	other.Config = sis.ContractConfig{BaseURL: "https://sis.example", AuthType: sis.AuthAPIKey,
		Schools: []sis.ContractSchool{{ExternalID: schoolExt, SchoolID: uuid.MustParse(s.b.school)}}}
	if _, err := s.conns.CreateContract(s.sa, s.ta, other); codeOfErr(err) != problem.ValidationFailed {
		t.Fatalf("school of another tenant: %v", err)
	}
	conn := s.create(t, true)
	// AC-04: no secret reference in the response, the row view, the audit or the events.
	out, err := s.contract.CheckHealth(s.sa, conn)
	if err != nil || !out.Compatible || len(out.Capabilities) != 4 {
		t.Fatalf("health = %+v %v", out, err)
	}
	if n := count(ctx, t, s.platform, `SELECT count(*) FROM sis.sis_connections WHERE sis_connection_id = $1 AND last_health_at IS NOT NULL
		AND capabilities @> '["student_lookup","webhook"]'`, conn); n != 1 {
		t.Fatal("capabilities and last_health_at not stored (AC-02)")
	}
	if n := count(ctx, t, s.relay, `SELECT count(*) FROM infra.outbox_events WHERE envelope::text LIKE '%sis/th-demo-a%'`); n != 0 {
		t.Fatalf("secret reference in %d outbox/audit events (AC-04)", n)
	}
	if n := count(ctx, t, s.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded' AND envelope::text LIKE '%sis_connection.created%'`); n != 1 {
		t.Fatalf("audit sis_connection.created = %d", n)
	}
	// AC-02: a facade on major 2 → contract_error + 422 SIS_CONTRACT_VERSION_UNSUPPORTED (TC-AUTH-011).
	s.fake.Lock()
	s.fake.Version = "2.0.0"
	s.fake.Unlock()
	if _, err := s.contract.CheckHealth(s.sa, conn); codeOfErr(err) != problem.SisContractVersionUnsupported {
		t.Fatalf("major 2: %v", err)
	}
	if st, reason, _, _ := s.connRow(ctx, t, conn); st != sis.StatusContractError || reason != sis.ReasonVersionMismatch {
		t.Fatalf("connection %s/%s", st, reason)
	}
	// AC-13: reactivation needs a passed health check and a reason.
	active := sis.StatusActive
	if _, err := s.contract.Update(s.sa, conn, s.version(ctx, t, conn), command.ContractPatch{Status: &active}); codeOfErr(err) != problem.ValidationFailed {
		t.Fatalf("no reason: %v", err)
	}
	if _, err := s.contract.Update(s.sa, conn, s.version(ctx, t, conn), command.ContractPatch{Status: &active, Reason: "facade fixed"}); codeOfErr(err) != problem.SisContractVersionUnsupported {
		t.Fatalf("still major 2: %v", err)
	}
	if _, err := s.contract.Update(s.sa, conn, 1, command.ContractPatch{Status: &active, Reason: "x"}); codeOfErr(err) != problem.VersionMismatch {
		t.Fatalf("stale If-Match: %v", err)
	}
	s.fake.Lock()
	s.fake.Version = "1.2.1"
	s.fake.Unlock()
	view, err := s.contract.Update(s.sa, conn, s.version(ctx, t, conn), command.ContractPatch{Status: &active, Reason: "facade fixed"})
	if err != nil || view.Status != sis.StatusActive || !view.HasSecret || !view.HasWebhookSecret || view.StatusReason != nil {
		t.Fatalf("reactivated = %+v %v", view, err)
	}
	raw, _ := json.Marshal(view)
	if strings.Contains(string(raw), "sis/th-demo-a") || strings.Contains(string(raw), "secret_ref") {
		t.Fatalf("secret reference in the view: %s", raw)
	}
	// School admins cannot manage connections (F017-AC-28).
	adm := authztest.Ctx(ctx, authztest.Staff(s.ta, "school_admin"))
	if _, err := s.contract.CheckHealth(adm, conn); codeOfErr(err) != problem.PermissionDenied {
		t.Fatalf("school admin health check: %v", err)
	}
}

// F017-AC-03, AC-05…AC-10, AC-12…AC-14: dry run, full sync, contract violations, lock, missing_in_source,
// unmapped school, retries, auth_error, circuit breaker.
func TestSisContractPull(t *testing.T) {
	ctx := context.Background()
	s := newSisAPI(ctx, t, setup(ctx, t))
	conn := s.create(t, false)
	users := func() int {
		return count(ctx, t, s.platform, `SELECT count(*) FROM identity.users WHERE username LIKE 'hs.%.thdemo001'`)
	}
	events := func(typ string) int {
		return count(ctx, t, s.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = $1`, typ)
	}

	// AC-03: dry run = pull + validate + staging + reconcile, no apply, no business event.
	dry := s.trigger(ctx, t, conn, sis.ModeFull, true)
	if dry["status"] != "succeeded" || dry["dry_run"] != true || users() != 0 || events("student.synced") != 0 {
		t.Fatalf("dry run = %v, users %d", dry, users())
	}
	if n := count(ctx, t, s.platform, `SELECT count(*) FROM sis.staging_records WHERE sync_run_id = $1`, dry["id"]); n != 4 {
		t.Fatalf("staging of the dry run = %d (1 school + 3 students; SIS contract 1.3.0 has no classes or enrollments)", n)
	}
	// ADR-0095 D6: EduWorld stops calling listClasses/listEnrollments, even on a facade that still serves them.
	if s.fake.CallsOf(fakesis.OpClasses) != 0 || s.fake.CallsOf(fakesis.OpEnrollments) != 0 {
		t.Fatalf("legacy operations called: classes %d enrollments %d", s.fake.CallsOf(fakesis.OpClasses), s.fake.CallsOf(fakesis.OpEnrollments))
	}
	if st := dry["stats"].(map[string]any); st["diff_student_create"] != float64(3) {
		t.Fatalf("dry run stats = %v", st)
	}

	// AC-05, AC-06, AC-09: one student breaks the enum, unknown fields, an unmapped school.
	s.fake.Lock()
	s.fake.Students[schoolExt][2]["status"] = "expelled"
	// SIS contract 1.3.0 has no classes or enrollments: eight more valid students keep the violation ratio under 10% (AC-06).
	for i := 1; i <= 8; i++ {
		n := strconv.Itoa(i)
		s.fake.Students[schoolExt] = append(s.fake.Students[schoolExt], fakesis.Student(schoolExt+"-X"+n, schoolExt, "X00"+n, "Hoc Sinh X", "xtra."+n, "active"))
	}
	s.fake.Extra[fakesis.OpStudents] = map[string]any{"phone": "0900000000"}
	s.fake.Schools = append(s.fake.Schools, map[string]any{"school_external_id": "TH-LA-999", "school_code": "LA", "name": "TH Lạ",
		"status": "active", "updated_at": fakesis.T})
	s.fake.Unlock()
	unknownBefore := testutil.ToFloat64(metrics.UnknownFieldsCounter().WithLabelValues(conn.String(), "student"))
	full := s.trigger(ctx, t, conn, sis.ModeFull, false)
	if full["status"] != "partially_failed" || full["mode"] != "full" || full["trigger"] != "manual" {
		t.Fatalf("full run = %v", full)
	}
	if users() != 2 {
		t.Fatalf("users = %d, want 2 (one record rejected)", users())
	}
	if n := count(ctx, t, s.platform, `SELECT count(*) FROM sis.sync_record_errors WHERE sync_run_id = $1 AND error_code = 'SIS_CONTRACT_VIOLATION'
		AND column_name = 'status'`, full["id"]); n != 1 {
		t.Fatalf("SIS_CONTRACT_VIOLATION rows = %d", n)
	}
	if n := count(ctx, t, s.platform, `SELECT count(*) FROM sis.staging_records WHERE payload ? 'phone'`); n != 0 {
		t.Fatal("a field outside the contract was stored (AC-06)")
	}
	if testutil.ToFloat64(metrics.UnknownFieldsCounter().WithLabelValues(conn.String(), "student")) <= unknownBefore {
		t.Fatal("eduworld_sis_contract_unknown_fields_total not increased")
	}
	if n := count(ctx, t, s.platform, `SELECT count(*) FROM sis.reconciliation_issues WHERE issue_type = 'unmapped_school'
		AND entity_refs->>'school_external_id' = 'TH-LA-999'`); n != 1 {
		t.Fatalf("unmapped_school issues = %d (AC-09)", n)
	}
	if n := count(ctx, t, s.platform, `SELECT count(*) FROM org.schools WHERE code = 'LA'`); n != 0 {
		t.Fatal("an unmapped school was created")
	}
	// AC-08: sis.sync_completed with mode, status, connection_status; last_success_at.
	if n := count(ctx, t, s.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'sis.sync_completed'
		AND envelope::text LIKE '%"mode": "full"%' AND envelope::text LIKE '%"connection_status": "active"%'`); n < 1 {
		var env string
		_ = s.relay.QueryRowContext(ctx, `SELECT envelope::text FROM infra.outbox_events WHERE event_type = 'sis.sync_completed' LIMIT 1`).Scan(&env)
		t.Fatalf("sis.sync_completed payload: %s", env)
	}
	if n := count(ctx, t, s.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope::text LIKE '%sis.sync_run.completed%' AND envelope::text LIKE '%system:sis-sync:`+conn.String()+`%'`); n < 2 {
		t.Fatalf("run audits with actor system:sis-sync = %d (AC-29)", n)
	}
	if n := count(ctx, t, s.platform, `SELECT count(*) FROM sis.sis_connections WHERE sis_connection_id = $1 AND last_success_at IS NOT NULL`, conn); n != 1 {
		t.Fatal("last_success_at")
	}
	// AC-08: a student missing from the next full sync is marked missing_in_source (missing_runs = 1).
	s.fake.Lock()
	s.fake.Students[schoolExt] = s.fake.Students[schoolExt][1:2]
	s.fake.Unlock()
	s.trigger(ctx, t, conn, sis.ModeFull, false)
	if n := count(ctx, t, s.platform, `SELECT count(*) FROM sis.synced_students WHERE student_external_id = $1 AND missing_runs = 1`,
		schoolExt+"-HS1"); n != 1 {
		t.Fatal("missing_in_source not marked")
	}

	// AC-07: a second trigger while a run is queued → 409; a scheduled run skips a busy lock (metric).
	if _, err := s.sync.Trigger(s.sa, conn, command.TriggerInput{Kind: sis.ModeFull}); err != nil {
		t.Fatal(err)
	}
	queued := s.jobs.last()
	if _, err := s.sync.Trigger(s.sa, conn, command.TriggerInput{Kind: sis.ModeFull}); codeOfErr(err) != problem.SisSyncAlreadyRunning {
		t.Fatalf("second trigger: %v", err)
	}
	release, _ := s.lock.Acquire(ctx, conn)
	skipped := testutil.ToFloat64(metrics.SkippedCounter().WithLabelValues(conn.String(), "locked"))
	if err := s.sync.Execute(ctx, port.SyncJob{TenantID: s.ta, ConnectionID: conn, Mode: sis.ModeFull, Trigger: sis.TriggerSchedule}); err != nil {
		t.Fatal(err)
	}
	if testutil.ToFloat64(metrics.SkippedCounter().WithLabelValues(conn.String(), "locked")) != skipped+1 {
		t.Fatal("scheduled run skipped without metric")
	}
	if err := s.sync.Execute(ctx, queued); !errors.Is(err, port.ErrRunLockBusy) {
		t.Fatalf("queued manual run under a busy lock: %v", err)
	}
	release()
	if err := s.sync.Execute(ctx, queued); err != nil {
		t.Fatal(err)
	}

	// AC-10: 503 twice then 200 → succeeded with retries.
	s.fake.Lock()
	s.fake.Fail[fakesis.OpSchools] = []int{503, 503}
	s.fake.Unlock()
	if r := s.trigger(ctx, t, conn, sis.ModeFull, false); r["status"] == "failed" || r["stats"].(map[string]any)["retries"].(float64) < 2 {
		t.Fatalf("retried run = %v", r)
	}
	// AC-12: 401 → failed, auth_error, no retry, event connection_status = auth_error.
	s.fake.Lock()
	s.fake.Fail[fakesis.OpHealth] = []int{401}
	s.fake.Unlock()
	healthCalls := s.fake.CallsOf(fakesis.OpHealth)
	if r := s.trigger(ctx, t, conn, sis.ModeFull, false); r["status"] != "failed" || r["failure_code"] != sis.FailAuth {
		t.Fatalf("401 run = %v", r)
	}
	if s.fake.CallsOf(fakesis.OpHealth) != healthCalls+1 {
		t.Fatal("401 was retried")
	}
	if st, reason, _, _ := s.connRow(ctx, t, conn); st != sis.StatusAuthError || reason != sis.ReasonCredentialRejected {
		t.Fatalf("connection %s/%s", st, reason)
	}
	if n := count(ctx, t, s.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'sis.sync_completed'
		AND envelope::text LIKE '%"connection_status": "auth_error"%'`); n != 1 {
		t.Fatal("sis.sync_completed with connection_status auth_error")
	}
	// AC-14: an inactive connection refuses triggers.
	if _, err := s.sync.Trigger(s.sa, conn, command.TriggerInput{Kind: sis.ModeFull}); codeOfErr(err) != problem.SisConnectionNotActive {
		t.Fatalf("trigger on auth_error: %v", err)
	}
	active := sis.StatusActive
	if _, err := s.contract.Update(s.sa, conn, s.version(ctx, t, conn), command.ContractPatch{Status: &active, Reason: "key rotated"}); err != nil {
		t.Fatal(err)
	}
	// AC-13: 5 consecutive failed runs pause the connection (circuit_open).
	for i := range sis.CircuitThreshold {
		s.fake.Lock()
		s.fake.Fail[fakesis.OpHealth] = []int{500, 500, 500, 500, 500, 500, 500}
		s.fake.Unlock()
		if err := s.sync.Execute(ctx, port.SyncJob{TenantID: s.ta, ConnectionID: conn, Mode: sis.ModeFull, Trigger: sis.TriggerSchedule}); err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
	}
	if st, reason, failed, _ := s.connRow(ctx, t, conn); st != sis.StatusPaused || reason != sis.ReasonCircuitOpen || failed != sis.CircuitThreshold {
		t.Fatalf("after %d failed runs: %s/%s/%d", sis.CircuitThreshold, st, reason, failed)
	}
	if _, err := s.sync.Trigger(s.sa, conn, command.TriggerInput{Kind: sis.ModeFull}); codeOfErr(err) != problem.SisConnectionNotActive {
		t.Fatalf("trigger on paused: %v", err)
	}
	if n := count(ctx, t, s.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope::text LIKE '%sis_connection.status_changed%' AND envelope::text LIKE '%circuit_open%'`); n != 1 {
		t.Fatal("audit of the circuit opening")
	}
	if _, err := s.contract.Update(s.sa, conn, s.version(ctx, t, conn), command.ContractPatch{Status: &active, Reason: "facade fixed"}); err != nil {
		t.Fatal(err)
	}
	if st, _, failed, _ := s.connRow(ctx, t, conn); st != sis.StatusActive || failed != 0 {
		t.Fatalf("reactivated %s/%d", st, failed)
	}
	// AC-06: more than 10% of the records violate the contract → stop before apply, contract_error.
	s.fake.Lock()
	for _, st := range s.fake.Students[schoolExt] {
		st["grade_level"] = 12
	}
	s.fake.Unlock()
	if r := s.trigger(ctx, t, conn, sis.ModeFull, false); r["status"] != "failed" || r["failure_code"] != sis.FailViolationThreshold {
		t.Fatalf("threshold run = %v", r)
	}
	if st, reason, _, _ := s.connRow(ctx, t, conn); st != sis.StatusContractError || reason != sis.ReasonViolationThreshold {
		t.Fatalf("connection %s/%s", st, reason)
	}
}

// F017-AC-15…AC-17: delta in sequence order, watermark after apply, 410 → full sync, delete/suspended
// effective in the same run (TC-AUTH-036).
func TestSisContractDelta(t *testing.T) {
	ctx := context.Background()
	s := newSisAPI(ctx, t, setup(ctx, t))
	conn := s.create(t, false)
	s.trigger(ctx, t, conn, sis.ModeFull, false)
	hs3 := schoolExt + "-HS3"
	rec := fakesis.Student(hs3, schoolExt, "A003", "Lê Minh Chi", "hs.chi.thdemo001", "suspended")
	s.fake.Lock()
	s.fake.Changes = []map[string]any{
		{"sequence": 3, "entity_kind": "student", "operation": "upsert", "external_id": hs3, "occurred_at": fakesis.T, "record": rec},
		{"sequence": 1, "entity_kind": "student", "operation": "upsert", "external_id": hs3, "occurred_at": fakesis.T,
			"record": fakesis.Student(hs3, schoolExt, "A003", "Lê Minh Chi", "hs.chi.thdemo001", "active")},
		{"sequence": 2, "entity_kind": "enrollment", "operation": "delete", "external_id": schoolExt + "-HS2|" + schoolExt + "-3A", "occurred_at": fakesis.T},
	}
	s.fake.Unlock()
	r := s.trigger(ctx, t, conn, sis.ModeDelta, false)
	if r["status"] != "succeeded" || r["mode"] != "delta" {
		t.Fatalf("delta run = %v", r)
	}
	if _, _, _, wm := s.connRow(ctx, t, conn); wm != "wm-3" {
		t.Fatalf("watermark = %q, want the next_cursor of the last page", wm)
	}
	if n := count(ctx, t, s.platform, `SELECT count(*) FROM identity.users WHERE username = 'hs.chi.thdemo001' AND status = 'locked'`); n != 1 {
		t.Fatal("suspended student not locked in the same run (AC-17)")
	}
	// ADR-0095 D6: a Change of kind enrollment (removed by contract 1.3.0) is ignored with a warning, never a violation.
	if n := count(ctx, t, s.platform, `SELECT count(*) FROM sis.sync_runs WHERE mode = 'delta' AND sis_connection_id = $1
		AND (stats->>'changes_ignored')::int = 1 AND (stats->>'errors')::int = 0`, conn); n != 1 {
		t.Fatal("the enrollment change is ignored and counted")
	}
	if n := count(ctx, t, s.platform, `SELECT count(*) FROM org.students st JOIN identity.users u ON u.user_id = st.student_id
		WHERE u.username = 'hs.binh.thdemo001' AND st.status = 'active'`); n != 1 {
		t.Fatal("an ignored enrollment change never touches the student")
	}
	q := s.fake.Queries[fakesis.OpChanges]
	if len(q) < 2 || !strings.Contains(q[0], "limit=500") || strings.Contains(q[0], "since=") || !strings.Contains(q[1], "since=wm-") {
		t.Fatalf("listChanges queries = %v", q)
	}
	s.trigger(ctx, t, conn, sis.ModeDelta, false)
	if q := s.fake.Queries[fakesis.OpChanges]; !strings.Contains(q[len(q)-1], "since=wm-3") {
		t.Fatalf("the next delta must send since = watermark: %v", q)
	}
	// AC-16: 410 → failed SIS_WATERMARK_EXPIRED, watermark unchanged, a full sync queued (fallback_full).
	s.fake.Lock()
	s.fake.ExpiredFrom["wm-3"] = true
	s.fake.Unlock()
	r = s.trigger(ctx, t, conn, sis.ModeDelta, false)
	if r["failure_code"] != sis.FailWatermarkExpired {
		t.Fatalf("410 run = %v", r)
	}
	if st, _, failed, wm := s.connRow(ctx, t, conn); wm != "wm-3" || st != sis.StatusActive || failed != 0 {
		t.Fatalf("after 410: %s/%d/%q", st, failed, wm)
	}
	if j := s.jobs.last(); j.Trigger != sis.TriggerFallbackFull || j.Mode != sis.ModeFull || j.RunID != nil {
		t.Fatalf("fallback job = %+v", j)
	}
	if err := s.sync.Execute(ctx, s.jobs.last()); err != nil {
		t.Fatal(err)
	}
	if n := count(ctx, t, s.platform, `SELECT count(*) FROM sis.sync_runs WHERE sis_connection_id = $1 AND "trigger" = 'fallback_full' AND status <> 'failed'`, conn); n != 1 {
		t.Fatal("fallback full run")
	}
	// The scheduler queues a delta when the facade declares it, deduplicated per minute.
	sched := command.ScheduleSisSync{UoW: s.uow, Repo: write.SisContractRepo{}, Jobs: s.jobs,
		Clock: func() time.Time { return time.Date(2026, 11, 19, 2, 30, 10, 0, time.UTC) }} // 09:30 ICT
	if n, err := sched.RunOnce(ctx); err != nil || n != 1 || s.jobs.last().Mode != sis.ModeDelta {
		t.Fatalf("scheduler = %d %v %+v", n, err, s.jobs.last())
	}
	sched.Clock = func() time.Time { return time.Date(2026, 11, 19, 18, 0, 0, 0, time.UTC) } // 01:00 ICT
	if n, err := sched.RunOnce(ctx); err != nil || n != 1 || s.jobs.last().Mode != sis.ModeFull ||
		!strings.HasPrefix(s.jobs.ids[len(s.jobs.ids)-1], "sched:"+conn.String()+":full:") {
		t.Fatalf("scheduler full = %d %v %+v", n, err, s.jobs.last())
	}
}
