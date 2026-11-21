package command

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/sis"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// DefaultRunStaleAfter: a queued/running pull run older than this no longer blocks a trigger (the worker
// died; the Valkey lock has expired long before) [Proposed].
const DefaultRunStaleAfter = time.Hour

// SisSync orchestrates the pull runs of contract_v1 connections (F017 US2–US5): triggerSisSync, the
// scheduled_sync and webhook_batch jobs of the worker. Records go through the F011 pipeline (SisRun:
// staging → reconcile → apply); the lock, version check, violation threshold, retries and circuit breaker
// are applied here and in the client.
type SisSync struct {
	UoW         write.UnitOfWork
	Connections write.SisConnectionRepository
	Repo        write.SisContractRepository
	Run         SisRun
	Providers   port.SisProviderFactory
	Lock        port.RunLock
	Jobs        port.SisSyncJobQueue
	Metrics     port.SisSyncMetrics  // optional
	Observer    port.SyncRunObserver // optional
	Clock       func() time.Time
	StaleAfter  time.Duration
}

func (h SisSync) now() time.Time {
	if h.Clock != nil {
		return h.Clock().UTC()
	}
	return time.Now().UTC()
}

// TriggerInput is the body of triggerSisSync.
type TriggerInput struct {
	Kind     string // full | delta
	SchoolID *uuid.UUID
	DryRun   bool
}

func runView(r *write.SyncRun) dto.SyncRun {
	stats := r.Stats
	if stats == nil {
		stats = map[string]int{}
	}
	return dto.SyncRun{SyncRunID: r.ID, ConnectionID: r.ConnectionID, TenantID: r.TenantID, Kind: r.Mode, Status: r.Status, DryRun: r.DryRun,
		Trigger: r.Trigger, FailureCode: r.FailureCode, StartedAt: r.StartedAt, FinishedAt: r.FinishedAt, Stats: stats}
}

// authorizeTrigger: platform.sis_connection.manage for every school of the connection, or tenant.sis.import
// for a school in the caller's scope (school_id required, F017-AC-28). The tenant of the connection is read
// under the System Admin bypass only to choose the scope; another tenant is 404.
func (h SisSync) authorizeTrigger(ctx context.Context, id uuid.UUID, school *uuid.UUID) (context.Context, authz.Principal, uuid.UUID, error) {
	p, err := principal(ctx)
	if err != nil {
		return ctx, p, uuid.Nil, err
	}
	bctx, err := tenancy.Bypass(ctx, tenancy.ReasonSystemAdminCrossTenant)
	if err != nil {
		return ctx, p, uuid.Nil, err
	}
	var tenantID uuid.UUID
	if err := h.UoW.Do(bctx, func(ctx context.Context) error {
		var err error
		tenantID, err = h.Connections.TenantOf(ctx, id)
		return err
	}); err != nil {
		return ctx, p, uuid.Nil, err
	}
	if p.Has(authz.PermPlatformSisConnectionManage) {
		ctx, _, err = platformTarget(ctx, tenantID)
		return ctx, p, tenantID, err
	}
	if school == nil {
		if err := authz.Require(ctx, p, authz.PermTenantSisImport, authz.Resource{TenantID: p.TenantID}); err != nil {
			return ctx, p, uuid.Nil, err
		}
		return ctx, p, uuid.Nil, problem.Invalid("school_id", problem.SubRequired, "required with tenant.sis.import")
	}
	if err := authz.Require(ctx, p, authz.PermTenantSisImport, authz.Resource{TenantID: p.TenantID, SchoolID: school}); err != nil {
		return ctx, p, uuid.Nil, err
	}
	if p.TenantID == nil || *p.TenantID != tenantID {
		return ctx, p, uuid.Nil, problem.New(problem.NotFound, "")
	}
	sc, err := authz.ScopeFor(p)
	if err != nil {
		return ctx, p, uuid.Nil, problem.New(problem.PermissionDenied, "").Wrap(err)
	}
	return tenancy.WithScope(ctx, sc), p, tenantID, nil
}

// Trigger queues a manual run (triggerSisSync, F017-AC-05, AC-07, AC-14, AC-28, AC-29).
func (h SisSync) Trigger(ctx context.Context, id uuid.UUID, in TriggerInput) (out dto.SyncRun, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.TriggerSisSync")
	defer func() { done(err) }()
	if in.Kind != sis.ModeFull && in.Kind != sis.ModeDelta {
		return out, problem.Invalid("kind", problem.SubInvalid, "must be full or delta")
	}
	ctx, p, tenantID, err := h.authorizeTrigger(ctx, id, in.SchoolID)
	if err != nil {
		return out, err
	}
	stale := h.StaleAfter
	if stale <= 0 {
		stale = DefaultRunStaleAfter
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		c, err := h.Repo.GetContract(ctx, id)
		if err != nil {
			return err
		}
		if c.ProviderKind != sis.ProviderContractV1 {
			return problem.Invalid("connection_id", problem.SubInvalid, "triggerSisSync applies to contract_v1 connections (csv_import: sis-imports)")
		}
		if !c.State.Active() {
			return problem.New(problem.SisConnectionNotActive, "").With("status", c.State.Status)
		}
		if in.Kind == sis.ModeDelta && !sis.HasCapability(c.Capabilities, sis.CapDelta) {
			return problem.Invalid("kind", problem.SubInvalid, "the connection does not declare the delta capability")
		}
		if in.SchoolID != nil {
			active, err := h.Repo.ActiveSchoolIDs(ctx, id)
			if err != nil {
				return err
			}
			if !active[*in.SchoolID] || mappedExternal(c.Config, *in.SchoolID) == "" {
				return problem.New(problem.NotFound, "the school is not served by this connection")
			}
		}
		pending, err := h.Repo.PendingPullRun(ctx, id, h.now().Add(-stale))
		if err != nil {
			return err
		}
		if pending {
			return problem.New(problem.SisSyncAlreadyRunning, "")
		}
		by := p.UserID
		run := &write.SyncRun{ID: ids.New(), TenantID: tenantID, ConnectionID: id, SchoolID: in.SchoolID, Mode: in.Kind, DryRun: in.DryRun,
			Status: "queued", Trigger: sis.TriggerManual, RequestedBy: &by, Stats: map[string]int{}}
		if err := h.Run.Sync.CreateRun(ctx, run); err != nil {
			return err
		}
		if err := h.Repo.SaveContract(ctx, c); err != nil { // version bump: concurrent triggers serialize here
			var pe *problem.Error
			if errors.As(err, &pe) && pe.Code == problem.VersionMismatch {
				return problem.New(problem.SisSyncAlreadyRunning, "")
			}
			return err
		}
		if err := h.Run.Audit.Record(ctx, audit.Event{Action: "sis.sync_run.requested", TargetType: "sync_run", TargetID: run.ID.String(),
			Result: audit.ResultSuccess, Metadata: map[string]any{"sis_connection_id": id.String(), "mode": in.Kind, "dry_run": in.DryRun}}); err != nil {
			return err
		}
		out = runView(run)
		if err := idempotency.SetResult(ctx, out); err != nil {
			return err
		}
		rid := run.ID
		// Last step of the transaction: a job without its committed run is dropped by the worker.
		return h.Jobs.EnqueueSync(ctx, port.SyncJob{TenantID: tenantID, ConnectionID: id, RunID: &rid, Mode: in.Kind, Trigger: sis.TriggerManual,
			DryRun: in.DryRun}, "run:"+rid.String())
	})
	return out, err
}

func mappedExternal(cfg sis.ContractConfig, school uuid.UUID) string {
	for _, s := range cfg.Schools {
		if s.SchoolID == school {
			return s.ExternalID
		}
	}
	return ""
}

// outcome is the result of a pull run before it is recorded.
type outcome struct {
	status        string
	failure       string
	stats         map[string]int
	issues        int
	authReason    string // auth_error with this reason
	contractError string // contract_error with this reason
	health        *port.SisHealth
	watermark     *string
	fallbackFull  bool
	noCount       bool // not counted by the circuit breaker (expired watermark → full sync)
}

func runFailed(code string) outcome {
	return outcome{status: "failed", failure: code, stats: map[string]int{}}
}

// classify maps a provider error to the failed outcome (F017-AC-10, AC-12, edge cases).
func classify(err error) outcome {
	switch {
	case errors.Is(err, port.ErrSisSecret):
		o := runFailed(sis.FailAuth)
		o.authReason = sis.ReasonSecretUnavailable
		return o
	case errors.Is(err, port.ErrSisAuth):
		o := runFailed(sis.FailAuth)
		o.authReason = sis.ReasonCredentialRejected
		return o
	case errors.Is(err, port.ErrSisPagination):
		return runFailed(sis.FailPaginationInvalid)
	case errors.Is(err, port.ErrSisBadResponse): // a page outside the contract (not one record)
		return runFailed(sis.CodeContractViolation)
	case errors.Is(err, port.ErrSisWatermarkExpired):
		o := runFailed(sis.FailWatermarkExpired)
		o.fallbackFull, o.noCount = true, true
		return o
	default:
		return runFailed(sis.FailUnavailable)
	}
}

// Execute runs a scheduled_sync job in the worker (scope: the tenant of the job). A busy lock skips a
// scheduled run (metric) and returns port.ErrRunLockBusy for a queued manual run (redelivered later).
func (h SisSync) Execute(ctx context.Context, job port.SyncJob) (err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.RunPullSync")
	defer func() { done(err) }()
	return h.execute(ctx, job, nil)
}

// ExecuteWebhook processes a webhook_batch job: one run of mode webhook per batch (F017-AC-21).
func (h SisSync) ExecuteWebhook(ctx context.Context, job port.WebhookJob) (err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.ProcessSisWebhookBatch")
	defer func() { done(err) }()
	return h.execute(ctx, port.SyncJob{TenantID: job.TenantID, ConnectionID: job.ConnectionID, Mode: sis.ModeWebhook, Trigger: sis.TriggerWebhook},
		&job)
}

func (h SisSync) execute(ctx context.Context, job port.SyncJob, wh *port.WebhookJob) error {
	sc, err := authz.EventScope(job.TenantID)
	if err != nil {
		return err
	}
	ctx = tenancy.WithScope(ctx, sc)
	var c *write.ContractConnectionRow
	var run *write.SyncRun
	if err := h.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		if c, err = h.Repo.GetContract(ctx, job.ConnectionID); err != nil {
			return err
		}
		if job.RunID != nil {
			run, err = h.Repo.GetRun(ctx, *job.RunID)
		}
		return err
	}); err != nil {
		var pe *problem.Error
		if errors.As(err, &pe) && pe.Code == problem.NotFound {
			return nil // the connection is gone: nothing to do
		}
		return err
	}
	if c.ProviderKind != sis.ProviderContractV1 || (job.RunID != nil && (run == nil || run.Status != "queued")) {
		return nil // not a pull connection, or a redelivery of a run already processed
	}
	if !c.State.Active() {
		if run != nil {
			return h.finish(ctx, run, runFailed(sis.FailConnectionNotActive), time.Now())
		}
		if h.Metrics != nil {
			h.Metrics.RunSkipped(c.ID, "not_active")
		}
		return nil
	}
	release, err := h.Lock.Acquire(ctx, c.ID)
	if errors.Is(err, port.ErrRunLockBusy) {
		if run == nil && wh == nil { // scheduled or fallback run: skipped, the next tick runs (F017-AC-07)
			if h.Metrics != nil {
				h.Metrics.RunSkipped(c.ID, "locked")
			}
			return nil
		}
		return err // manual run or webhook: queued until the lock is released
	}
	if err != nil {
		return err
	}
	defer release()
	start := time.Now()
	if run == nil {
		run = &write.SyncRun{ID: ids.New(), TenantID: c.TenantID, ConnectionID: c.ID, Mode: job.Mode, DryRun: job.DryRun, Status: "running",
			Trigger: job.Trigger, Stats: map[string]int{}}
		if wh != nil {
			d := wh.DeliveryID
			run.DeliveryID = &d
		}
		if err := h.UoW.Do(ctx, func(ctx context.Context) error { return h.Run.Sync.CreateRun(ctx, run) }); err != nil {
			return err
		}
	} else if err := h.UoW.Do(ctx, func(ctx context.Context) error { return h.Repo.StartRun(ctx, run.ID) }); err != nil {
		return err
	}
	run.Status = "running"
	var o outcome
	if wh != nil {
		o, err = h.changes(ctx, c, run, wh.Changes, nil, wh.Violations)
	} else {
		o, err = h.pull(ctx, c, run)
	}
	if err != nil { // database failure: the run is recorded failed, the job retried
		_ = h.finish(ctx, run, runFailed(sis.FailUnavailable), start)
		return err
	}
	return h.finish(ctx, run, o, start)
}

// pull runs the health check then the full or delta pull.
func (h SisSync) pull(ctx context.Context, c *write.ContractConnectionRow, run *write.SyncRun) (outcome, error) {
	p, err := h.Providers.For(ctx, contractConn(c))
	if err != nil {
		return classify(err), nil
	}
	hl, err := p.Health(ctx)
	if err != nil {
		return classify(err), nil
	}
	if !sis.CompatibleMajor(c.ContractVersion, hl.ContractVersion) { // F017-AC-02 at the start of every run
		o := runFailed(sis.FailVersionUnsupported)
		o.contractError, o.health = sis.ReasonVersionMismatch, &hl
		return o, nil
	}
	var o outcome
	if run.Mode == sis.ModeDelta {
		o, err = h.delta(ctx, p, c, run)
	} else {
		o, err = h.full(ctx, p, c, run)
	}
	o.health = &hl
	if hl.Status == "degraded" { // edge case: run anyway, warn in stats
		o.stats["health_degraded"] = 1
	}
	st := p.Stats()
	o.stats["http_requests"], o.stats["retries"] = st.Requests, st.Retries
	return o, err
}

// schoolMap is the mapped schools of the connection that are active on it (optionally one school).
func (h SisSync) schoolMap(ctx context.Context, c *write.ContractConnectionRow, only *uuid.UUID) (map[string]uuid.UUID, error) {
	var active map[uuid.UUID]bool
	if err := h.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		active, err = h.Repo.ActiveSchoolIDs(ctx, c.ID)
		return err
	}); err != nil {
		return nil, err
	}
	out := map[string]uuid.UUID{}
	for _, s := range c.Config.Schools {
		if active[s.SchoolID] && (only == nil || *only == s.SchoolID) {
			out[s.ExternalID] = s.SchoolID
		}
	}
	return out, nil
}

func recordErrors(vs []port.SisViolation) []write.RecordError {
	out := make([]write.RecordError, 0, len(vs))
	for _, v := range vs {
		msg := v.Message
		if len(msg) > 1000 {
			msg = msg[:1000]
		}
		out = append(out, write.RecordError{Kind: kindOrChange(v.Kind), ExternalID: truncate(v.ExternalID, 64), Column: truncate(v.Field, 64),
			Code: sis.CodeContractViolation, Message: msg})
	}
	return out
}

func kindOrChange(k string) string {
	switch k {
	case sis.KindSchool, sis.KindStudent:
		return k
	}
	return truncate(k, 64)
}

type fetched struct {
	schoolID   uuid.UUID
	ext        string
	files      []SisFile
	violations []port.SisViolation
	records    int
}

func asFile(kind string, recs []map[string]any) SisFile {
	f := SisFile{Kind: kind}
	for i, r := range recs {
		f.Records = append(f.Records, SisRecord{Row: i + 1, Data: r})
	}
	return f
}

// all pages of one list operation.
func allPages(ctx context.Context, f func(ctx context.Context, cursor string) (port.SisPage, error)) (recs []map[string]any, vs []port.SisViolation,
	err error) {
	cursor := ""
	for {
		p, err := f(ctx, cursor)
		if err != nil {
			return nil, nil, err
		}
		recs, vs = append(recs, p.Records...), append(vs, p.Violations...)
		if !p.HasMore {
			return recs, vs, nil
		}
		cursor = p.NextCursor
	}
}

// full pulls every mapped school (schools → academic year → students; SIS contract 1.3.0 has no classes and no
// enrollments — ADR-0095 D6), checks the violation threshold, then hands each school to the F011 pipeline (F017-AC-05,
// AC-06, AC-08, AC-09).
func (h SisSync) full(ctx context.Context, p port.SisProvider, c *write.ContractConnectionRow, run *write.SyncRun) (outcome, error) {
	o := outcome{status: "succeeded", stats: map[string]int{}}
	mapped, err := h.schoolMap(ctx, c, run.SchoolID)
	if err != nil {
		return o, err
	}
	schools, schoolViolations, err := allPages(ctx, p.ListSchools)
	if err != nil {
		return classify(err), nil
	}
	byExt := map[string]map[string]any{}
	var unmapped []string
	for _, s := range schools {
		ext := sis.Str(s, "school_external_id")
		if _, ok := mapped[ext]; ok {
			byExt[ext] = s
		} else if run.SchoolID == nil {
			unmapped = append(unmapped, ext)
		}
	}
	exts := make([]string, 0, len(mapped))
	for ext := range mapped {
		exts = append(exts, ext)
	}
	sort.Strings(exts)
	total, violations := len(schools)+len(schoolViolations), len(schoolViolations)
	var data []fetched
	for _, ext := range exts {
		d := fetched{schoolID: mapped[ext], ext: ext}
		if s, ok := byExt[ext]; ok {
			d.files = append(d.files, asFile(sis.KindSchool, []map[string]any{s}))
		}
		years, err := p.ListAcademicYears(ctx, ext)
		if err != nil {
			return classify(err), nil
		}
		d.violations = append(d.violations, years.Violations...)
		year := ""
		for _, y := range years.Records {
			if b, _ := y["is_current"].(bool); b {
				year = sis.Str(y, "academic_year")
			}
		}
		if year == "" {
			o.stats["schools_without_current_year"]++
		} else {
			if !run.DryRun { // the current year of the SIS is the year of the placements of this school (ADR-0095)
				school := d.schoolID
				if err := h.UoW.Do(ctx, func(ctx context.Context) error { _, err := h.Run.Sync.AcademicYear(ctx, school, year); return err }); err != nil {
					return o, err
				}
			}
			students, sv, err := allPages(ctx, func(ctx context.Context, cur string) (port.SisPage, error) { return p.ListStudents(ctx, ext, cur) })
			if err != nil {
				return classify(err), nil
			}
			d.files = append(d.files, asFile(sis.KindStudent, students))
			d.violations = append(d.violations, sv...)
			d.records = len(students)
		}
		total += d.records + len(d.violations)
		violations += len(d.violations)
		data = append(data, d)
	}
	o.stats["fetched_violations"] = violations
	if sis.ViolationRatioExceeded(violations, total, sis.ViolationThreshold) { // F017-AC-06: stop before any apply
		f := runFailed(sis.FailViolationThreshold)
		f.contractError = sis.ReasonViolationThreshold
		f.stats["fetched_violations"] = violations
		errs := recordErrors(schoolViolations)
		for _, d := range data {
			errs = append(errs, recordErrors(d.violations)...)
		}
		f.stats["errors"] = len(errs)
		return f, h.UoW.Do(ctx, func(ctx context.Context) error { return h.Run.Sync.AddErrors(ctx, run.ID, errs) })
	}
	if len(schoolViolations) > 0 {
		errs := recordErrors(schoolViolations)
		if err := h.UoW.Do(ctx, func(ctx context.Context) error { return h.Run.Sync.AddErrors(ctx, run.ID, errs) }); err != nil {
			return o, err
		}
		o.stats["errors"] += len(errs)
	}
	if err := h.unmapped(ctx, run, unmapped, &o); err != nil {
		return o, err
	}
	for _, d := range data {
		res, stats, err := h.Run.ProcessPulled(ctx, run, PulledSchool{SchoolID: d.schoolID, Files: d.files, RecordErrors: recordErrors(d.violations),
			FullStudents: true})
		if err != nil {
			return o, err
		}
		h.merge(&o, res, stats)
	}
	if o.stats["errors"] > 0 {
		o.status = "partially_failed"
	}
	return o, nil
}

// unmapped raises one unmapped_school issue per school external id outside the mapping (F017-AC-09); a dry
// run only counts them.
func (h SisSync) unmapped(ctx context.Context, run *write.SyncRun, exts []string, o *outcome) error {
	if len(exts) == 0 {
		return nil
	}
	o.stats["unmapped_schools"] += len(exts)
	if run.DryRun {
		return nil
	}
	return h.UoW.Do(ctx, func(ctx context.Context) error {
		for _, ext := range exts {
			if err := h.Run.RaiseRunIssue(ctx, run, nil, sis.IssueUnmappedSchool, map[string]any{"school_external_id": truncate(ext, 64)}); err != nil {
				return err
			}
			o.issues++
		}
		return nil
	})
}

func (h SisSync) merge(o *outcome, res RunImportResult, stats map[string]int) {
	for k, v := range stats {
		o.stats[k] += v
	}
	for kind, diff := range res.Diff {
		for action, n := range diff {
			o.stats["diff_"+kind+"_"+action] += n
		}
	}
	o.stats["errors"] += len(res.Errors)
	o.issues += res.Issues
}

// delta pulls listChanges since the watermark (F017-AC-15, AC-16): the new watermark is kept only when every
// batch applied; 410 queues a full sync without touching the watermark.
func (h SisSync) delta(ctx context.Context, p port.SisProvider, c *write.ContractConnectionRow, run *write.SyncRun) (outcome, error) {
	cursor := c.DeltaWatermark
	var all []port.SisChange
	var vs []port.SisViolation
	next := ""
	for {
		page, err := p.ListChanges(ctx, cursor)
		if err != nil {
			return classify(err), nil
		}
		all, vs = append(all, page.Changes...), append(vs, page.Violations...)
		if !page.HasMore {
			next = page.NextCursor
			break
		}
		cursor = page.NextCursor
	}
	o, err := h.changes(ctx, c, run, all, vs, 0)
	if err == nil && o.status != "failed" && !run.DryRun && next != "" && next != c.DeltaWatermark {
		o.watermark = &next
	}
	return o, err
}

// changes applies delta/webhook changes in sequence order, grouped by school (F017-AC-15, AC-17, AC-21).
func (h SisSync) changes(ctx context.Context, c *write.ContractConnectionRow, run *write.SyncRun, changes []port.SisChange, vs []port.SisViolation,
	extraViolations int) (outcome, error) {
	o := outcome{status: "succeeded", stats: map[string]int{}}
	violations := len(vs) + extraViolations
	if sis.ViolationRatioExceeded(violations, len(changes)+violations, sis.ViolationThreshold) {
		f := runFailed(sis.FailViolationThreshold)
		f.contractError = sis.ReasonViolationThreshold
		errs := recordErrors(vs)
		f.stats["errors"] = violations
		return f, h.UoW.Do(ctx, func(ctx context.Context) error { return h.Run.Sync.AddErrors(ctx, run.ID, errs) })
	}
	if len(vs) > 0 {
		errs := recordErrors(vs)
		if err := h.UoW.Do(ctx, func(ctx context.Context) error { return h.Run.Sync.AddErrors(ctx, run.ID, errs) }); err != nil {
			return o, err
		}
	}
	o.stats["errors"] = violations
	mapped, err := h.schoolMap(ctx, c, nil)
	if err != nil {
		return o, err
	}
	// The last change of a source key wins (changes are sorted by sequence).
	sort.SliceStable(changes, func(i, j int) bool { return changes[i].Sequence < changes[j].Sequence })
	last := map[string]int{}
	for i, ch := range changes {
		last[ch.Kind+"\x00"+ch.ExternalID] = i
	}
	groups := map[uuid.UUID]*PulledSchool{}
	group := func(id uuid.UUID) *PulledSchool {
		if groups[id] == nil {
			groups[id] = &PulledSchool{SchoolID: id}
		}
		return groups[id]
	}
	files := map[uuid.UUID]map[string][]map[string]any{}
	var unmapped []string
	seenUnmapped := map[string]bool{}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		for i, ch := range changes {
			if sis.IgnoredKind(ch.Kind) { // SIS contract 1.3.0 (ADR-0095 D6): ignored with a warning, never rejected
				o.stats["changes_ignored"]++
				slog.WarnContext(ctx, "sis: change of a kind removed by the SIS contract 1.3.0 is ignored", "kind", ch.Kind,
					"sis_connection_id", c.ID.String())
				continue
			}
			if last[ch.Kind+"\x00"+ch.ExternalID] != i {
				o.stats["changes_superseded"]++
				continue
			}
			schoolExt, err := h.schoolOfChange(ctx, c.ID, ch, mapped)
			if err != nil {
				return err
			}
			school, ok := mapped[schoolExt]
			if !ok {
				if schoolExt != "" && !seenUnmapped[schoolExt] {
					seenUnmapped[schoolExt] = true
					unmapped = append(unmapped, schoolExt)
				}
				o.stats["changes_unresolved"]++
				continue
			}
			g := group(school)
			if ch.Operation == "delete" {
				g.Deletes = append(g.Deletes, SisDelete{Kind: ch.Kind, ExternalID: ch.ExternalID})
				continue
			}
			if files[school] == nil {
				files[school] = map[string][]map[string]any{}
			}
			files[school][ch.Kind] = append(files[school][ch.Kind], ch.Record)
		}
		return nil
	})
	if err != nil {
		return o, err
	}
	if err := h.unmapped(ctx, run, unmapped, &o); err != nil {
		return o, err
	}
	schoolIDs := make([]uuid.UUID, 0, len(groups))
	for id := range groups {
		schoolIDs = append(schoolIDs, id)
	}
	for id := range files {
		if groups[id] == nil {
			group(id)
			schoolIDs = append(schoolIDs, id)
		}
	}
	sort.Slice(schoolIDs, func(i, j int) bool { return schoolIDs[i].String() < schoolIDs[j].String() })
	for _, id := range schoolIDs {
		g := groups[id]
		for _, kind := range sis.ApplyOrder {
			if recs := files[id][kind]; len(recs) > 0 {
				g.Files = append(g.Files, asFile(kind, recs))
			}
		}
		res, stats, err := h.Run.ProcessPulled(ctx, run, *g)
		if err != nil {
			return o, err
		}
		h.merge(&o, res, stats)
	}
	if o.stats["errors"] > 0 {
		o.status = "partially_failed"
	}
	return o, nil
}

// schoolOfChange returns the school external id of a change: from its record, or from the synced student of a delete.
func (h SisSync) schoolOfChange(ctx context.Context, conn uuid.UUID, ch port.SisChange, mapped map[string]uuid.UUID) (string, error) {
	if ch.Operation == "upsert" {
		return sis.Str(ch.Record, "school_external_id"), nil
	}
	if ch.Kind == sis.KindSchool { // school delete: only counted by the pipeline of a mapped school
		return ch.ExternalID, nil
	}
	s, err := h.Run.Sync.SyncedStudentOf(ctx, conn, ch.ExternalID)
	if err != nil || s == nil {
		return "", err
	}
	for ext, id := range mapped {
		if id == s.SchoolID {
			return ext, nil
		}
	}
	return "", nil
}

// finish records the run, the connection state (circuit breaker, auth/contract errors, watermark, last
// success, capabilities), the audit of the run (actor system:sis-sync:<connection>) and sis.sync_completed.
func (h SisSync) finish(ctx context.Context, run *write.SyncRun, o outcome, start time.Time) error {
	now := h.now()
	var opened bool
	var connStatus string
	err := h.UoW.Do(ctx, func(ctx context.Context) error {
		c, err := h.Repo.GetContract(ctx, run.ConnectionID)
		if err != nil {
			return err
		}
		before := c.State
		if o.health != nil {
			c.Capabilities, c.LastHealthAt = o.health.Capabilities, &now
		}
		switch {
		case o.authReason != "":
			c.State.MarkAuthError(o.authReason)
		case o.contractError != "":
			c.State.MarkContractError(o.contractError)
		}
		if o.status != "failed" {
			c.LastSuccessAt = &now
		}
		if !o.noCount && !run.DryRun {
			opened = c.State.RecordRun(o.status == "failed", sis.CircuitThreshold)
		}
		if o.watermark != nil {
			c.DeltaWatermark = *o.watermark
		}
		if err := h.Repo.SaveContract(ctx, c); err != nil {
			return err
		}
		connStatus = c.State.Status
		run.Status, run.FinishedAt = o.status, &now
		if o.failure != "" {
			code := o.failure
			run.FailureCode = &code
		}
		run.Stats = o.stats
		run.Stats["issues"] = o.issues
		if err := h.Run.Sync.FinishRun(ctx, run); err != nil {
			return err
		}
		actor := "system:sis-sync:" + run.ConnectionID.String()
		result := audit.ResultSuccess
		if o.status == "failed" {
			result = audit.ResultFailure
		}
		if err := h.Run.Audit.Record(ctx, audit.Event{Action: "sis.sync_run.completed", TargetType: "sync_run", TargetID: run.ID.String(), Result: result,
			Metadata: map[string]any{"actor": actor, "mode": run.Mode, "status": o.status, "dry_run": run.DryRun, "failure_code": o.failure,
				"stats": completedStats(run.Stats)}}); err != nil {
			return err
		}
		if before.Status != c.State.Status {
			if err := h.Run.Audit.Record(ctx, audit.Event{Action: "sis_connection.status_changed", TargetType: "sis_connection",
				TargetID: run.ConnectionID.String(), Result: audit.ResultSuccess, Metadata: map[string]any{"actor": actor, "from": before.Status,
					"to": c.State.Status, "status_reason": c.State.Reason}}); err != nil {
				return err
			}
		}
		t := run.TenantID
		payload := map[string]any{"sync_run_id": run.ID.String(), "sis_connection_id": run.ConnectionID.String(), "mode": run.Mode, "status": o.status,
			"dry_run": run.DryRun, "connection_status": c.State.Status, "failure_code": run.FailureCode, "stats": completedStats(run.Stats),
			"issues_count": o.issues}
		return h.Run.Events.Publish(ctx, write.Event{Type: "sis.sync_completed", SubjectType: "sync_run", SubjectID: run.ID.String(), TenantID: &t,
			Payload: payload})
	})
	if err != nil {
		return err
	}
	if h.Observer != nil {
		h.Observer.RunFinished(run.ConnectionID, run.Mode, o.status, now)
	}
	if h.Metrics != nil {
		h.Metrics.RunDuration(run.ConnectionID, run.Mode, time.Since(start))
		if opened {
			h.Metrics.RunSkipped(run.ConnectionID, "circuit_open")
		}
	}
	if o.fallbackFull && connStatus == sis.StatusActive { // F017-AC-16: 410 → a full sync, watermark unchanged
		return h.Jobs.EnqueueSync(ctx, port.SyncJob{TenantID: run.TenantID, ConnectionID: run.ConnectionID, Mode: sis.ModeFull,
			Trigger: sis.TriggerFallbackFull}, "fallback:"+run.ID.String())
	}
	return nil
}
