package command

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/appscope"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/placement"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/sis"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/user"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// SisRecord is one valid record of an import file.
type SisRecord struct {
	Row  int
	Data map[string]any
}

// SisFile is one file of an import: its records, or its violations (then nothing of the file is applied).
type SisFile struct {
	Kind    string // sis.KindSchool…
	Records []SisRecord
	Errors  []write.RecordError
}

// RunImportInput is a csv_import run of one school of a connection.
type RunImportInput struct {
	ConnectionID uuid.UUID
	SchoolID     uuid.UUID
	Files        []SisFile
	DryRun       bool
	// AcademicYear is the current academic year of the school in the source (YYYY-YYYY, optional): the run makes sure it
	// exists (current when the school has none), so student.placement_changed carries academic_year_id (ADR-0095).
	AcademicYear string
}

// RunImportResult is the diff (dry run) or the outcome (apply) of a run.
type RunImportResult struct {
	RunID  uuid.UUID                 `json:"sync_run_id"`
	Status string                    `json:"status"`
	DryRun bool                      `json:"dry_run"`
	Diff   map[string]map[string]int `json:"diff"`
	Errors []write.RecordError       `json:"errors"`
	Issues int                       `json:"reconciliation_issue_count"`
}

// SisRun orchestrates a csv_import run: validate → staging → reconcile → apply (F011-AC-01…AC-05,
// AC-08, AC-10, AC-14, AC-23, AC-25). Records are applied in batches of BatchSize, each record in a
// savepoint so a failing record never aborts the batch. A student record places the student in its school and grade
// (org.students — ADR-0095 mục 2) and every change of school, grade or effective status publishes
// student.placement_changed; class and enrollment records are ignored with a warning (SIS contract 1.3.0, D6).
type SisRun struct {
	UoW         write.UnitOfWork
	Savepoints  write.Savepoints
	Connections write.SisConnectionRepository
	Sync        write.SisSyncRepository
	Missing     write.MissingTracker
	Transfers   write.TransferMatcher
	Students    write.StudentRepository
	Rules       write.ConflictRules
	Events      write.EventPublisher
	Audit       write.Auditor
	BatchSize   int                       // default 500 [Proposed]
	Observer    port.SyncRunObserver      // optional: last success metric
	IDHashKey   []byte                    // key of the national id hash (EW_SIS_ID_HASH_KEY [Proposed]); empty = no transfer matching
	History     write.LifecycleRepository // F016 US2: identity.user_status_history of SIS status changes (nil = not recorded)
}

// ErrNoLoginUsername: a new student without login.username cannot get an account.
const codeLoginRequired = "LOGIN_USERNAME_REQUIRED"

type runState struct {
	in       RunImportInput
	run      *write.SyncRun
	tenantID uuid.UUID
	issues   int
	stats    map[string]int
	students map[string]uuid.UUID // student external id → user id (this run)
	seen     map[string]bool      // students present in the file
	// F011-AC-16: national id hash → synced student of another connection; students moved in this run.
	candidates  map[string]write.TransferCandidate
	transferred map[string]bool
	full        bool // F017: the students of a pull run are the full list of the school (missing_in_source)
}

func newRunState(in RunImportInput, tenantID uuid.UUID) *runState {
	return &runState{in: in, tenantID: tenantID, stats: map[string]int{}, students: map[string]uuid.UUID{}, seen: map[string]bool{},
		candidates: map[string]write.TransferCandidate{}, transferred: map[string]bool{}}
}

// authorize: School Admin with tenant.sis.import on the school, or platform staff managing connections.
func (h SisRun) authorize(ctx context.Context, schoolID uuid.UUID, tenantID uuid.UUID) (context.Context, error) {
	p, err := principal(ctx)
	if err != nil {
		return ctx, err
	}
	if p.TenantID != nil && p.Has(authz.PermTenantSisImport) {
		if err := authz.Require(ctx, p, authz.PermTenantSisImport, authz.Resource{TenantID: p.TenantID, SchoolID: &schoolID}); err != nil {
			return ctx, err
		}
		if *p.TenantID != tenantID {
			return ctx, problem.New(problem.NotFound, "")
		}
		sc, err := authz.ScopeFor(p)
		if err != nil {
			return ctx, problem.New(problem.PermissionDenied, "").Wrap(err)
		}
		return tenancy.WithScope(ctx, sc), nil
	}
	if err := authz.Require(ctx, p, authz.PermPlatformSisConnectionManage, authz.Resource{}); err != nil {
		return ctx, err
	}
	sc, err := appscope.TargetTenant(p, tenantID)
	if err != nil {
		return ctx, err
	}
	return tenancy.WithScope(ctx, sc), nil
}

// Handle runs the import.
func (h SisRun) Handle(ctx context.Context, in RunImportInput) (out RunImportResult, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.SisRunImport")
	defer func() { done(err) }()
	// The connection's tenant is read under the System Admin bypass only to choose the scope.
	var tenantID uuid.UUID
	bctx, err := tenancy.Bypass(ctx, tenancy.ReasonSystemAdminCrossTenant)
	if err != nil {
		return out, err
	}
	if err = h.UoW.Do(bctx, func(ctx context.Context) error {
		var err error
		tenantID, err = h.Connections.TenantOf(ctx, in.ConnectionID)
		return err
	}); err != nil {
		return out, err
	}
	if ctx, err = h.authorize(ctx, in.SchoolID, tenantID); err != nil {
		return out, err
	}
	return h.execute(ctx, tenantID, in, "csv_import")
}

// ApplyLookup applies one Student record returned by a SIS lookup (login sync, SIS_INTEGRATION §4.5) as
// the system actor in the tenant of the connection: a one-record run of mode delta through the same
// validate → staging → reconcile → apply pipeline; it never marks other students missing.
func (h SisRun) ApplyLookup(ctx context.Context, tenantID, connectionID, schoolID uuid.UUID, record map[string]any) (out RunImportResult, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.SisApplyLookup")
	defer func() { done(err) }()
	sc, err := authz.EventScope(tenantID)
	if err != nil {
		return out, err
	}
	in := RunImportInput{ConnectionID: connectionID, SchoolID: schoolID,
		Files: []SisFile{{Kind: sis.KindStudent, Records: []SisRecord{{Row: 1, Data: record}}}}}
	return h.execute(tenancy.WithScope(ctx, sc), tenantID, in, "delta")
}

// RunImportSystem runs a csv_import of an uploaded file as the system actor (sis-jobs worker): the School
// Admin was authorized when the import was created.
func (h SisRun) RunImportSystem(ctx context.Context, tenantID uuid.UUID, in RunImportInput) (out RunImportResult, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.SisRunImportJob")
	defer func() { done(err) }()
	sc, err := authz.EventScope(tenantID)
	if err != nil {
		return out, err
	}
	return h.execute(tenancy.WithScope(ctx, sc), tenantID, in, "csv_import")
}

// execute runs the pipeline in the scope already set on ctx.
func (h SisRun) execute(ctx context.Context, tenantID uuid.UUID, in RunImportInput, mode string) (out RunImportResult, err error) {
	st := newRunState(in, tenantID)
	out = RunImportResult{DryRun: in.DryRun, Diff: map[string]map[string]int{}, Errors: []write.RecordError{}}
	school := in.SchoolID
	st.run = &write.SyncRun{ID: ids.New(), TenantID: tenantID, ConnectionID: in.ConnectionID, SchoolID: &school, Mode: mode,
		DryRun: in.DryRun, Status: "running", Stats: map[string]int{}}
	if err = h.UoW.Do(ctx, func(ctx context.Context) error {
		c, err := h.Connections.Get(ctx, in.ConnectionID)
		if err != nil {
			return err
		}
		if c.Status != "active" || (mode == "csv_import" && c.ProviderKind != "csv_import") {
			return problem.Invalid("connection_id", problem.SubInvalid, "an active csv_import connection is required")
		}
		schools, err := h.Connections.Schools(ctx, in.ConnectionID)
		if err != nil {
			return err
		}
		served := false
		for _, s := range schools {
			served = served || (s.SchoolID == in.SchoolID && s.Active)
		}
		if !served {
			return problem.Invalid("school_id", problem.SubInvalid, "the school is not served by this connection")
		}
		return h.Sync.CreateRun(ctx, st.run)
	}); err != nil {
		return out, err
	}
	out.RunID = st.run.ID
	if in.AcademicYear != "" && !in.DryRun {
		if err = h.UoW.Do(ctx, func(ctx context.Context) error {
			_, err := h.Sync.AcademicYear(ctx, in.SchoolID, in.AcademicYear)
			return err
		}); err != nil {
			return out, err
		}
	}

	files, err := h.process(ctx, st, in.Files, nil, nil, &out)
	if err != nil {
		return out, err
	}
	out.Issues = st.issues
	out.Status = "succeeded"
	switch {
	case len(out.Errors) > 0 && len(files) == 0:
		out.Status = "failed"
	case len(out.Errors) > 0:
		out.Status = "partially_failed"
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		now := time.Now().UTC()
		st.run.Status, st.run.FinishedAt = out.Status, &now
		for k, v := range st.stats {
			st.run.Stats[k] = v
		}
		st.run.Stats["errors"] = len(out.Errors)
		st.run.Stats["issues"] = st.issues
		if out.Status == "failed" {
			code := "SIS_CONTRACT_VIOLATION"
			st.run.FailureCode = &code
		}
		if err := h.Sync.FinishRun(ctx, st.run); err != nil {
			return err
		}
		t := st.tenantID
		return h.Events.Publish(ctx, write.Event{Type: "sis.sync_completed", SubjectType: "sync_run", SubjectID: st.run.ID.String(), TenantID: &t,
			Payload: map[string]any{"sync_run_id": st.run.ID.String(), "stats": completedStats(st.run.Stats), "issues_count": st.issues}})
	})
	if err == nil {
		h.observe(st.run) // after the commit
	}
	return out, err
}

// process records the errors, stages and (unless the run is a dry run) applies the files of a run whose
// row exists: a file with a violation is skipped entirely (F011-AC-02, csv_import), recordErrs are
// per-record violations of a pull run that never block the other records (F017-AC-06); deletes are the
// delete changes of a delta/webhook run (F017-AC-17). It returns the files without violations.
func (h SisRun) process(ctx context.Context, st *runState, in []SisFile, recordErrs []write.RecordError, deletes []SisDelete,
	out *RunImportResult) (files []SisFile, err error) {
	// All-or-nothing per file (F011-AC-02): a file with a violation is recorded and skipped entirely.
	for _, f := range in {
		if sis.IgnoredKind(f.Kind) { // SIS contract 1.3.0 (ADR-0095 D6): never applied, never stored
			st.stats[f.Kind+"_ignored"] += len(f.Records) + len(f.Errors)
			slog.WarnContext(ctx, "sis: records of a kind removed by the SIS contract 1.3.0 are ignored", "kind", f.Kind,
				"records", len(f.Records)+len(f.Errors), "sis_connection_id", st.in.ConnectionID.String())
			continue
		}
		if len(f.Errors) > 0 {
			out.Errors = append(out.Errors, f.Errors...)
			continue
		}
		files = append(files, f)
	}
	for _, e := range recordErrs { // F017: per-record violations never block the other records
		if sis.IgnoredKind(e.Kind) {
			continue
		}
		out.Errors = append(out.Errors, e)
	}
	kept := deletes[:0:0]
	for _, d := range deletes {
		if sis.IgnoredKind(d.Kind) {
			st.stats[d.Kind+"_ignored"]++
			continue
		}
		kept = append(kept, d)
	}
	deletes = kept
	if len(out.Errors) > 0 {
		if err = h.UoW.Do(ctx, func(ctx context.Context) error { return h.Sync.AddErrors(ctx, st.run.ID, out.Errors) }); err != nil {
			return nil, err
		}
	}
	// Staging + diff (no business change): F011-AC-01.
	staged := map[string][]write.StagingItem{}
	if err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if len(deletes) > 0 {
			items := stageDeletes(deletes)
			staged[deleteKind] = items
			for _, it := range items {
				out.Diff[it.Kind] = orNew(out.Diff[it.Kind])
				out.Diff[it.Kind][it.Action]++
			}
			if err := h.Sync.AddStaging(ctx, st.run.ID, items); err != nil {
				return err
			}
		}
		for _, f := range files {
			items, err := h.stage(ctx, st, f)
			if err != nil {
				return err
			}
			staged[f.Kind] = items
			out.Diff[f.Kind] = orNew(out.Diff[f.Kind])
			for _, it := range items {
				out.Diff[f.Kind][it.Action]++
			}
			if err := h.Sync.AddStaging(ctx, st.run.ID, items); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if !st.in.DryRun {
		if err = h.apply(ctx, st, files, staged); err != nil {
			return nil, err
		}
	}
	return files, nil
}

func externalID(kind string, d map[string]any) string {
	switch kind {
	case sis.KindSchool:
		return sis.Str(d, "school_external_id")
	default:
		return sis.Str(d, "student_external_id")
	}
}

// stage decides the action of each record against the stored hash of its source key.
func (h SisRun) stage(ctx context.Context, st *runState, f SisFile) ([]write.StagingItem, error) {
	items := make([]write.StagingItem, 0, len(f.Records))
	for _, r := range f.Records {
		ext, hash := externalID(f.Kind, r.Data), sis.Hash(r.Data)
		var found bool
		var stored string
		if f.Kind == sis.KindStudent {
			s, err := h.Sync.SyncedStudentOf(ctx, st.in.ConnectionID, ext)
			if err != nil {
				return nil, err
			}
			if s != nil {
				found, stored = true, s.Hash
			}
		} else {
			_, h2, ok, err := h.Sync.LinkOf(ctx, st.in.ConnectionID, f.Kind, ext)
			if err != nil {
				return nil, err
			}
			found, stored = ok, h2
		}
		items = append(items, write.StagingItem{Kind: f.Kind, ExternalID: truncate(ext, 64), Hash: hash, Action: sis.ActionFor(found, stored, hash),
			Status: "pending", Row: r.Row, Payload: r.Data})
	}
	return items, nil
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// apply applies the files in the order school → student, in batches.
func (h SisRun) apply(ctx context.Context, st *runState, files []SisFile, staged map[string][]write.StagingItem) error {
	size := h.BatchSize
	if size <= 0 {
		size = 500
	}
	for _, kind := range sis.ApplyOrder {
		items := staged[kind]
		if kind == sis.KindStudent {
			if err := h.findTransfers(ctx, st, items); err != nil {
				return err
			}
		}
		for start := 0; start < len(items); start += size {
			end := min(start+size, len(items))
			if err := h.UoW.Do(ctx, func(ctx context.Context) error {
				for _, it := range items[start:end] {
					if it.Action == sis.ActionNoop {
						st.stats[kind+"_unchanged"]++
						if kind == sis.KindStudent {
							st.seen[it.ExternalID] = true
						}
						continue
					}
					err := h.Savepoints.Savepoint(ctx, func(ctx context.Context) error { return h.applyOne(ctx, st, kind, it) })
					// A conflicting record is rolled back; its issue is raised after the rollback so it is kept.
					var ri *recordIssue
					if errors.As(err, &ri) {
						if err := h.issue(ctx, st, ri.typ, ri.refs); err != nil {
							return err
						}
						st.stats[kind+"_conflicts"]++
						continue
					}
					if err != nil {
						var pe *problem.Error
						if !errors.As(err, &pe) {
							return err
						}
						st.stats[kind+"_failed"]++
						continue
					}
					st.stats[kind+"_"+it.Action+"d"]++
				}
				return nil
			}); err != nil {
				return err
			}
		}
	}
	if err := h.applyDeletes(ctx, st, staged[deleteKind], size); err != nil {
		return err
	}
	return h.afterApply(ctx, st, files)
}

func (h SisRun) applyOne(ctx context.Context, st *runState, kind string, it write.StagingItem) error {
	switch kind {
	case sis.KindSchool:
		return h.applySchool(ctx, st, it)
	default:
		return h.applyStudent(ctx, st, it)
	}
}

func (h SisRun) issue(ctx context.Context, st *runState, typ string, refs map[string]any) error {
	school := st.in.SchoolID
	id, err := h.Sync.RaiseIssue(ctx, write.Issue{SchoolID: &school, ConnectionID: st.in.ConnectionID, RunID: st.run.ID, Type: typ, Refs: refs})
	if err != nil {
		return err
	}
	st.issues++
	t := st.tenantID
	return h.Events.Publish(ctx, write.Event{Type: "sis.reconciliation_issue_raised", SubjectType: "reconciliation_issue", SubjectID: id.String(),
		TenantID: &t, Payload: map[string]any{"issue_id": id.String(), "kind": typ, "school_id": school.String(), "sis_connection_id": st.in.ConnectionID.String()}})
}

// findTransfers looks up, across tenants, synced students of other connections with the national id hash
// of the new students of the run (platform_job bypass, read only).
func (h SisRun) findTransfers(ctx context.Context, st *runState, items []write.StagingItem) error {
	if len(h.IDHashKey) == 0 {
		return nil
	}
	var hashes []string
	for _, it := range items {
		if it.Action == sis.ActionCreate {
			if nid := sis.NationalIDHash(h.IDHashKey, sis.Str(it.Payload, "national_student_id")); nid != "" {
				hashes = append(hashes, nid)
			}
		}
	}
	if len(hashes) == 0 {
		return nil
	}
	bctx, err := tenancy.Bypass(ctx, tenancy.ReasonPlatformJob)
	if err != nil {
		return err
	}
	return h.UoW.Do(bctx, func(ctx context.Context) error {
		var err error
		st.candidates, err = h.Transfers.TransferCandidates(ctx, hashes, st.in.ConnectionID)
		return err
	})
}

// moveLink moves the SIS link of a student to the new source key in the same tenant (F011-AC-16): the old
// link is superseded, the User and its learning/game data are kept.
func (h SisRun) moveLink(ctx context.Context, st *runState, cand write.TransferCandidate, ext string) error {
	if err := h.Transfers.SupersedeLink(ctx, cand.ConnectionID, cand.ExternalID); err != nil {
		return err
	}
	if err := h.Transfers.LinkExisting(ctx, *cand.UserID, st.tenantID, st.in.SchoolID, st.in.ConnectionID, ext); err != nil {
		return err
	}
	st.transferred[ext] = true
	t, uid := st.tenantID, cand.UserID.String()
	if err := h.Events.Publish(ctx, write.Event{Type: "student.sis_link_superseded", SubjectType: "user", SubjectID: uid, TenantID: &t,
		Payload: map[string]any{"user_id": uid, "old_sis_connection_id": cand.ConnectionID.String(), "old_student_external_id": cand.ExternalID,
			"new_sis_connection_id": st.in.ConnectionID.String(), "new_student_external_id": ext, "cross_tenant": false}}); err != nil {
		return err
	}
	return h.Events.Publish(ctx, write.Event{Type: "student.sis_link_created", SubjectType: "user", SubjectID: uid, TenantID: &t,
		Payload: map[string]any{"user_id": uid, "tenant_id": t.String(), "school_id": st.in.SchoolID.String(),
			"sis_connection_id": st.in.ConnectionID.String(), "student_external_id": ext, "via": "transfer"}})
}

// markMissing applies F011-AC-24 after a full students file: students of the school missing from the file
// are marked missing_in_source (missing_runs+1), never deactivated; when the missing share is above
// MissingReviewRatio in two consecutive runs one deactivation_review issue is raised, otherwise a student
// missing twice in a row gets its own review issue.
func (h SisRun) markMissing(ctx context.Context, st *runState) error {
	synced, err := h.Sync.StudentsOfSchool(ctx, st.in.ConnectionID, st.in.SchoolID)
	if err != nil {
		return err
	}
	type miss struct {
		ext  string
		runs int
	}
	var missing []miss
	total := 0
	for _, s := range synced {
		if s.RecordStatus != "active" {
			continue
		}
		total++
		if st.seen[s.ExternalID] {
			if s.MissingRuns > 0 { // back in the source (an unchanged record is not saved by the apply)
				if err := h.Missing.ResetMissing(ctx, st.in.ConnectionID, s.ExternalID); err != nil {
					return err
				}
			}
			continue
		}
		n, err := h.Missing.MarkMissing(ctx, st.in.ConnectionID, s.ExternalID)
		if err != nil {
			return err
		}
		missing = append(missing, miss{s.ExternalID, n})
	}
	prev, _, err := h.Missing.PreviousRunStats(ctx, st.in.ConnectionID, st.in.SchoolID, st.run.ID)
	if err != nil {
		return err
	}
	above, review := sis.DeactivationReviewNeeded(len(missing), total, prev["missing_above"] == 1, sis.MissingReviewRatio)
	st.stats["missing"] = len(missing)
	if above {
		st.stats["missing_above"] = 1
	}
	if review {
		return h.issue(ctx, st, sis.IssueDeactivationReview, map[string]any{"missing_count": strconv.Itoa(len(missing)),
			"total_count": strconv.Itoa(total), "scope": "school"})
	}
	if above {
		return nil // first run above the ratio: mark only
	}
	for _, m := range missing {
		if m.runs == 2 {
			if err := h.issue(ctx, st, sis.IssueDeactivationReview, map[string]any{"student_external_id": m.ext, "missing_runs": "2"}); err != nil {
				return err
			}
		}
	}
	return nil
}

// observe reports the end of a committed run.
func (h SisRun) observe(r *write.SyncRun) {
	if h.Observer != nil && r.FinishedAt != nil {
		h.Observer.RunFinished(r.ConnectionID, r.Mode, r.Status, *r.FinishedAt)
	}
}

// completedStats folds the per-kind run stats into the totals of sis.sync_completed.
func completedStats(stats map[string]int) map[string]int {
	out := map[string]int{"fetched": 0, "created": 0, "updated": 0, "unchanged": 0, "errors": stats["errors"]}
	for k, v := range stats {
		_, suffix, ok := strings.Cut(k, "_")
		if !ok {
			continue
		}
		switch suffix {
		case "created", "updated", "unchanged":
			out[suffix] += v
		case "failed", "conflicts":
			out["errors"] += v
		default:
			continue
		}
		out["fetched"] += v
	}
	return out
}

// recordIssue is a record that is not applied and raises a reconciliation issue (SIS_INTEGRATION §6.3).
type recordIssue struct {
	typ  string
	refs map[string]any
}

func (r *recordIssue) Error() string { return "sis: reconciliation issue " + r.typ }

func conflict(typ string, refs map[string]any) error { return &recordIssue{typ: typ, refs: refs} }

// skip marks a record as not applied (a problem error makes the savepoint roll back).
func skip(code problem.Code, msg string) error { return problem.New(code, msg) }

// applySchool links the school record to the school of the import (a csv_import is scoped to one school).
func (h SisRun) applySchool(ctx context.Context, st *runState, it write.StagingItem) error {
	if p := sis.Str(it.Payload, "province_code"); p != "" {
		known, err := h.Rules.ProvinceKnown(ctx, p)
		if err != nil {
			return err
		}
		if !known { // SIS_INTEGRATION §4.6
			return conflict(sis.IssueSchoolProvinceMissing, map[string]any{"school_external_id": it.ExternalID, "province_code": p})
		}
	}
	id, _, found, err := h.Sync.LinkOf(ctx, st.in.ConnectionID, sis.KindSchool, it.ExternalID)
	if err != nil {
		return err
	}
	if found && id != st.in.SchoolID {
		return conflict(sis.IssueUnmappedSchool, map[string]any{"school_external_id": it.ExternalID})
	}
	school := st.in.SchoolID
	if err := h.Sync.SaveLink(ctx, st.in.ConnectionID, sis.KindSchool, it.ExternalID, school, &school, it.Hash); err != nil {
		return err
	}
	version, err := h.Sync.SchoolVersion(ctx, school)
	if err != nil {
		return err
	}
	t := st.tenantID
	return h.Events.Publish(ctx, write.Event{Type: "school.synced", SubjectType: "school", SubjectID: school.String(), TenantID: &t,
		Payload: map[string]any{"school_id": school.String(), "sync_run_id": st.run.ID.String(), "sis_connection_id": st.in.ConnectionID.String(),
			"name": sis.Str(it.Payload, "name"), "changed_fields": []string{}, "source_version": version}})
}

// schoolOf maps a school_external_id to the import school (unmapped → issue).
func (h SisRun) schoolOf(ctx context.Context, st *runState, ext string) (bool, error) {
	id, _, found, err := h.Sync.LinkOf(ctx, st.in.ConnectionID, sis.KindSchool, ext)
	if err != nil {
		return false, err
	}
	if !found {
		// The import school takes the first school external id it sees (schools.csv is optional).
		school := st.in.SchoolID
		if err := h.Sync.SaveLink(ctx, st.in.ConnectionID, sis.KindSchool, ext, school, &school, ""); err != nil {
			return false, err
		}
		return true, nil
	}
	return id == st.in.SchoolID, nil
}

func (h SisRun) applyStudent(ctx context.Context, st *runState, it write.StagingItem) error {
	d := it.Payload
	st.seen[it.ExternalID] = true
	ok, err := h.schoolOf(ctx, st, sis.Str(d, "school_external_id"))
	if err != nil {
		return err
	}
	if !ok {
		return conflict(sis.IssueUnmappedSchool, map[string]any{"student_external_id": it.ExternalID})
	}
	existing, err := h.Sync.SyncedStudentOf(ctx, st.in.ConnectionID, it.ExternalID)
	if err != nil {
		return err
	}
	grade, code, name, status := sis.Int(d, "grade_level"), sis.Str(d, "student_code"), sis.Str(d, "full_name"), sis.Str(d, "status")
	if !placement.ValidGrade(grade) { // the school and grade place the student (ADR-0095 mục 2)
		return skip(problem.ValidationFailed, "grade_level")
	}
	eff, err := sis.StatusEffect(status)
	if err != nil {
		return skip(problem.ValidationFailed, "status")
	}
	nid := sis.NationalIDHash(h.IDHashKey, sis.Str(d, "national_student_id"))
	var userID uuid.UUID
	var tr *placementTracker
	action := "update"
	cand, transfer := st.candidates[nid]
	transfer = transfer && nid != "" && cand.UserID != nil && (existing == nil || existing.UserID == nil)
	if transfer {
		if cand.TenantID != st.tenantID { // another tenant: confirmation by System Admin (ADR-0046 §6)
			return conflict(sis.IssueTransferConfirmation, map[string]any{"student_external_id": it.ExternalID, "match": "national_student_id"})
		}
		userID = *cand.UserID
		if tr, err = trackPlacement(ctx, h.Students, h.Events, userID); err != nil {
			return err
		}
		if err := h.moveLink(ctx, st, cand, it.ExternalID); err != nil {
			return err
		}
		if err := h.transferSchool(ctx, st, userID, tr.Before(), grade); err != nil {
			return err
		}
		if err := h.Sync.UpdateStudent(ctx, userID, grade, code, name); err != nil {
			return err
		}
	} else if existing == nil || existing.UserID == nil {
		if code != "" { // a new source key with the student_code of another student of the school [Proposed: no date of birth]
			other, err := h.Rules.StudentByCode(ctx, st.in.SchoolID, code)
			if err != nil {
				return err
			}
			if other != nil {
				return conflict(sis.IssuePossibleDuplicate, map[string]any{"student_external_id": it.ExternalID, "user_id": other.String()})
			}
		}
		raw := sis.LoginUsername(d)
		if raw == "" {
			return skip(problem.ValidationFailed, codeLoginRequired)
		}
		username, err := user.NormalizeUsername(raw)
		if err != nil {
			return skip(problem.ValidationFailed, "login.username")
		}
		owner, reason, err := h.Sync.UsernameOwner(ctx, username)
		if err != nil {
			return err
		}
		if owner != nil {
			typ := sis.IssueDuplicateIdentifier // never a second account, never overwrite (F011-AC-10)
			if reason != nil && *reason == "inactive_cleanup" {
				typ = sis.IssueCleanedUpStudentInSIS // no automatic restore (F011-AC-25)
			}
			return conflict(typ, map[string]any{"student_external_id": it.ExternalID, "user_id": owner.String()})
		}
		userID, action = ids.New(), "create"
		tr = &placementTracker{students: h.Students, events: h.Events, studentID: userID} // a new student: first placement
		if err := h.Sync.CreateSisStudent(ctx, write.NewSisStudent{UserID: userID, Username: username, DisplayName: name, TenantID: st.tenantID,
			SchoolID: st.in.SchoolID, Grade: grade, StudentCode: code, ConnectionID: st.in.ConnectionID, ExternalID: it.ExternalID}); err != nil {
			return err
		}
		if eff.StudentStatus == placement.StatusInactive { // the user stays pending; the placement follows the record status
			if err := h.Students.SetStatus(ctx, userID, placement.StatusInactive); err != nil {
				return err
			}
		}
		if err := h.Sync.SetLoginKey(ctx, st.in.ConnectionID, it.ExternalID, username); err != nil {
			return err
		}
		t := st.tenantID
		if err := h.Events.Publish(ctx, write.Event{Type: "student.sis_link_created", SubjectType: "user", SubjectID: userID.String(), TenantID: &t,
			Payload: map[string]any{"user_id": userID.String(), "tenant_id": t.String(), "school_id": st.in.SchoolID.String(),
				"sis_connection_id": st.in.ConnectionID.String(), "student_external_id": it.ExternalID, "via": "provisioning"}}); err != nil {
			return err
		}
	} else {
		userID = *existing.UserID
		deleted, reason, err := h.Rules.DeletedUser(ctx, userID)
		if err != nil {
			return err
		}
		if deleted { // never restored by a sync (ADR-0032, F011-AC-25)
			if reason != nil && *reason == "inactive_cleanup" {
				return conflict(sis.IssueCleanedUpStudentInSIS, map[string]any{"student_external_id": it.ExternalID, "user_id": userID.String()})
			}
			return skip(problem.NotFound, "the student account was deleted")
		}
		if tr, err = trackPlacement(ctx, h.Students, h.Events, userID); err != nil {
			return err
		}
		if err := h.transferSchool(ctx, st, userID, tr.Before(), grade); err != nil {
			return err
		}
		if err := h.Sync.UpdateStudent(ctx, userID, grade, code, name); err != nil {
			return err
		}
	}
	var nidPtr *string
	if nid != "" {
		nidPtr = &nid
	}
	if err := h.Sync.SaveSyncedStudent(ctx, write.SyncedStudent{ConnectionID: st.in.ConnectionID, ExternalID: it.ExternalID, SchoolID: st.in.SchoolID,
		StudentCode: code, RecordStatus: status, Hash: it.Hash, NationalIDHash: nidPtr}); err != nil {
		return err
	}
	t := st.tenantID
	if action == "create" {
		if err := h.Events.Publish(ctx, write.Event{Type: "student.provisioning_requested", SubjectType: "user", SubjectID: userID.String(), TenantID: &t,
			Payload: map[string]any{"student_id": userID.String(), "tenant_id": t.String(), "school_id": st.in.SchoolID.String(),
				"sis_connection_id": st.in.ConnectionID.String()}}); err != nil {
			return err
		}
	} else if err := h.applyStatus(ctx, st, userID, status); err != nil {
		return err
	}
	if err := tr.done(ctx, PlacementChange{Source: placement.SourceSIS, SyncRunID: &st.run.ID}); err != nil {
		return err
	}
	if err := h.Audit.Record(ctx, audit.Event{Action: "student.synced", TargetType: "student", TargetID: userID.String(), Result: audit.ResultSuccess,
		Metadata: map[string]any{"actor": "system:sis-sync:" + st.in.ConnectionID.String(), "action": action}}); err != nil {
		return err
	}
	st.students[it.ExternalID] = userID // only once the record is applied
	return h.Events.Publish(ctx, write.Event{Type: "student.synced", SubjectType: "student", SubjectID: userID.String(), TenantID: &t,
		Payload: map[string]any{"student_id": userID.String(), "school_id": st.in.SchoolID.String(), "action": action, "sync_run_id": st.run.ID.String(),
			"sis_connection_id": st.in.ConnectionID.String(), "grade_level": grade}})
}

// transferSchool moves a synced student of the tenant to the school of the run when its record now names this school
// (a SIS link moved by national id, F011-AC-16, or a school change in the same source): the student keeps its user, the
// move is published as student.transferred (and student.placement_changed by the caller). Another tenant never moves
// here (transfer_confirmation).
func (h SisRun) transferSchool(ctx context.Context, st *runState, userID uuid.UUID, before *placement.Record, grade int) error {
	if before == nil || before.SchoolID == st.in.SchoolID || before.TenantID != st.tenantID {
		return nil
	}
	to := placement.Placement{TenantID: st.tenantID, SchoolID: st.in.SchoolID, Grade: grade, Active: before.Active}
	if err := h.Students.Move(ctx, userID, to); err != nil {
		return err
	}
	t := st.tenantID
	return h.Events.Publish(ctx, write.Event{Type: "student.transferred", SubjectType: "student", SubjectID: userID.String(), TenantID: &t,
		Payload: map[string]string{"student_id": userID.String(), "from_tenant_id": t.String(), "to_tenant_id": t.String(),
			"from_school_id": before.SchoolID.String(), "to_school_id": st.in.SchoolID.String()}})
}

// applyStatus maps the SIS record status (F011-AC-23): user status, effective status of the student (org.students —
// ADR-0095), events in the same transaction. New students keep status pending until provisioning activates them. The
// caller publishes student.placement_changed.
func (h SisRun) applyStatus(ctx context.Context, st *runState, userID uuid.UUID, recordStatus string) error {
	eff, err := sis.StatusEffect(recordStatus)
	if err != nil {
		return skip(problem.ValidationFailed, "status")
	}
	t, uid := st.tenantID, userID.String()
	if eff.UserStatus != "" {
		from, changed, err := h.Sync.SetUserStatus(ctx, userID, eff.UserStatus)
		if err != nil {
			return err
		}
		if changed {
			reason := "sis:" + recordStatus
			if h.History != nil { // same history as a staff change (F016-AC-07)
				if err := h.History.RecordStatusChange(ctx, write.StatusChange{UserID: userID, TenantID: &t, From: from, To: eff.UserStatus,
					Reason: reason, Source: "sis"}); err != nil {
					return err
				}
			}
			if err := h.Events.Publish(ctx, write.Event{Type: "user.status_changed", SubjectType: "user", SubjectID: uid, TenantID: &t,
				Payload: map[string]any{"user_id": uid, "from": from, "to": eff.UserStatus, "reason": reason}}); err != nil {
				return err
			}
			if eff.UserStatus != "active" {
				revoke := "deactivated"
				if eff.UserStatus == "locked" {
					revoke = "locked"
				}
				if err := h.Events.Publish(ctx, write.Event{Type: "user.access_revoked", SubjectType: "user", SubjectID: uid, TenantID: &t,
					Payload: map[string]any{"user_id": uid, "reason": revoke}}); err != nil {
					return err
				}
			}
		}
	}
	if eff.StudentStatus != "" {
		return h.Students.SetStatus(ctx, userID, eff.StudentStatus)
	}
	return nil
}

// afterApply marks the students missing from a full students file (F011-AC-24). There is no missing_class any more:
// every student record carries its school and grade (ADR-0095 mục 2).
func (h SisRun) afterApply(ctx context.Context, st *runState, files []SisFile) error {
	full := false // a full students file of a csv_import (a lookup run has one record)
	for _, f := range files {
		full = full || (f.Kind == sis.KindStudent && (st.run.Mode == "csv_import" || st.full))
	}
	if !full {
		return nil
	}
	return h.UoW.Do(ctx, func(ctx context.Context) error { return h.markMissing(ctx, st) })
}
