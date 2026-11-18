package command

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/appscope"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/enrollment"
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
// AC-08, AC-10, AC-14, AC-15, AC-23, AC-25). Records are applied in batches of BatchSize, each record in a
// savepoint so a failing record never aborts the batch.
type SisRun struct {
	UoW         write.UnitOfWork
	Savepoints  write.Savepoints
	Connections write.SisConnectionRepository
	Sync        write.SisSyncRepository
	Enrollments write.EnrollmentRepository
	Events      write.EventPublisher
	Audit       write.Auditor
	BatchSize   int // default 500 [Proposed]
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
	inactive map[string]bool      // students of the run whose record status is not active (no missing_class)
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
	st := &runState{in: in, tenantID: tenantID, stats: map[string]int{}, students: map[string]uuid.UUID{}, seen: map[string]bool{}, inactive: map[string]bool{}}
	out = RunImportResult{DryRun: in.DryRun, Diff: map[string]map[string]int{}, Errors: []write.RecordError{}}
	school := in.SchoolID
	st.run = &write.SyncRun{ID: ids.New(), TenantID: tenantID, ConnectionID: in.ConnectionID, SchoolID: &school, Mode: "csv_import",
		DryRun: in.DryRun, Status: "running", Stats: map[string]int{}}
	if err = h.UoW.Do(ctx, func(ctx context.Context) error {
		c, err := h.Connections.Get(ctx, in.ConnectionID)
		if err != nil {
			return err
		}
		if c.Status != "active" || c.ProviderKind != "csv_import" {
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

	// All-or-nothing per file (F011-AC-02): a file with a violation is recorded and skipped entirely.
	var files []SisFile
	for _, f := range in.Files {
		if len(f.Errors) > 0 {
			out.Errors = append(out.Errors, f.Errors...)
			continue
		}
		files = append(files, f)
	}
	if len(out.Errors) > 0 {
		if err = h.UoW.Do(ctx, func(ctx context.Context) error { return h.Sync.AddErrors(ctx, st.run.ID, out.Errors) }); err != nil {
			return out, err
		}
	}
	// Staging + diff (no business change): F011-AC-01.
	staged := map[string][]write.StagingItem{}
	if err = h.UoW.Do(ctx, func(ctx context.Context) error {
		for _, f := range files {
			items, err := h.stage(ctx, st, f)
			if err != nil {
				return err
			}
			staged[f.Kind] = items
			out.Diff[f.Kind] = map[string]int{}
			for _, it := range items {
				out.Diff[f.Kind][it.Action]++
			}
			if err := h.Sync.AddStaging(ctx, st.run.ID, items); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return out, err
	}
	if !in.DryRun {
		if err = h.apply(ctx, st, files, staged); err != nil {
			return out, err
		}
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
	return out, err
}

func externalID(kind string, d map[string]any) string {
	switch kind {
	case sis.KindSchool:
		return sis.Str(d, "school_external_id")
	case sis.KindClass:
		return sis.Str(d, "class_external_id")
	case sis.KindStudent:
		return sis.Str(d, "student_external_id")
	default: // enrollment: student + class
		return sis.Str(d, "student_external_id") + "|" + sis.Str(d, "class_external_id")
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

// apply applies the files in the order school → class → student → enrollment, in batches.
func (h SisRun) apply(ctx context.Context, st *runState, files []SisFile, staged map[string][]write.StagingItem) error {
	size := h.BatchSize
	if size <= 0 {
		size = 500
	}
	for _, kind := range sis.ApplyOrder {
		items := staged[kind]
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
	return h.afterApply(ctx, st, files)
}

func (h SisRun) applyOne(ctx context.Context, st *runState, kind string, it write.StagingItem) error {
	switch kind {
	case sis.KindSchool:
		return h.applySchool(ctx, st, it)
	case sis.KindClass:
		return h.applyClass(ctx, st, it)
	case sis.KindStudent:
		return h.applyStudent(ctx, st, it)
	default:
		return h.applyEnrollment(ctx, st, it)
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
		}
		if suffix != "fetched" {
			out["fetched"] += v
		}
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
	t := st.tenantID
	return h.Events.Publish(ctx, write.Event{Type: "school.synced", SubjectType: "school", SubjectID: school.String(), TenantID: &t,
		Payload: map[string]any{"school_id": school.String(), "sync_run_id": st.run.ID.String(), "sis_connection_id": st.in.ConnectionID.String(),
			"name": sis.Str(it.Payload, "name"), "changed_fields": []string{}}})
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

func (h SisRun) applyClass(ctx context.Context, st *runState, it write.StagingItem) error {
	d := it.Payload
	ok, err := h.schoolOf(ctx, st, sis.Str(d, "school_external_id"))
	if err != nil {
		return err
	}
	if !ok {
		return conflict(sis.IssueUnmappedSchool, map[string]any{"class_external_id": it.ExternalID})
	}
	year, err := h.Sync.AcademicYear(ctx, st.in.SchoolID, sis.Str(d, "academic_year"))
	if err != nil {
		return err
	}
	grade, name := sis.Int(d, "grade_level"), sis.Str(d, "name")
	id, _, found, err := h.Sync.LinkOf(ctx, st.in.ConnectionID, sis.KindClass, it.ExternalID)
	if err != nil {
		return err
	}
	op := "updated"
	if found {
		err = h.Sync.UpdateClass(ctx, id, grade, name)
	} else {
		id, op = ids.New(), "created"
		err = h.Sync.CreateClass(ctx, id, st.in.SchoolID, year, grade, name)
	}
	if err != nil {
		return err
	}
	school := st.in.SchoolID
	if err := h.Sync.SaveLink(ctx, st.in.ConnectionID, sis.KindClass, it.ExternalID, id, &school, it.Hash); err != nil {
		return err
	}
	t := st.tenantID
	return h.Events.Publish(ctx, write.Event{Type: "class.synced", SubjectType: "class", SubjectID: id.String(), TenantID: &t,
		Payload: map[string]any{"class_id": id.String(), "school_id": school.String(), "grade_level": grade, "name": name,
			"academic_year": sis.Str(d, "academic_year"), "status": "active", "op": op, "trigger": "manual_import",
			"sis_connection_id": st.in.ConnectionID.String(), "sync_run_id": st.run.ID.String()}})
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
	var userID uuid.UUID
	action := "update"
	if existing == nil || existing.UserID == nil {
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
		if err := h.Sync.CreateSisStudent(ctx, write.NewSisStudent{UserID: userID, Username: username, DisplayName: name, TenantID: st.tenantID,
			SchoolID: st.in.SchoolID, Grade: grade, StudentCode: code, ConnectionID: st.in.ConnectionID, ExternalID: it.ExternalID}); err != nil {
			return err
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
		if err := h.Sync.UpdateStudent(ctx, userID, grade, code, name); err != nil {
			return err
		}
	}
	if err := h.Sync.SaveSyncedStudent(ctx, write.SyncedStudent{ConnectionID: st.in.ConnectionID, ExternalID: it.ExternalID, SchoolID: st.in.SchoolID,
		StudentCode: code, RecordStatus: status, Hash: it.Hash}); err != nil {
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
	if err := h.Audit.Record(ctx, audit.Event{Action: "student.synced", TargetType: "student", TargetID: userID.String(), Result: audit.ResultSuccess,
		Metadata: map[string]any{"actor": "system:sis-sync:" + st.in.ConnectionID.String(), "action": action}}); err != nil {
		return err
	}
	st.students[it.ExternalID] = userID // only once the record is applied
	st.inactive[it.ExternalID] = status != "active"
	return h.Events.Publish(ctx, write.Event{Type: "student.synced", SubjectType: "student", SubjectID: userID.String(), TenantID: &t,
		Payload: map[string]any{"student_id": userID.String(), "school_id": st.in.SchoolID.String(), "action": action, "sync_run_id": st.run.ID.String(),
			"sis_connection_id": st.in.ConnectionID.String(), "grade_level": grade}})
}

// applyStatus maps the SIS record status (F011-AC-23): user status, enrollment closed, events in the same
// transaction. New students keep status pending until provisioning activates them.
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
	if eff.CloseEnrollment != "" {
		cur, err := h.Enrollments.LockActive(ctx, userID)
		var pe *problem.Error
		if errors.As(err, &pe) && pe.Code == problem.NotFound {
			return nil
		}
		if err != nil {
			return err
		}
		if err := cur.Close(enrollment.Status(eff.CloseEnrollment), time.Now()); err != nil {
			return err
		}
		if err := h.Enrollments.Close(ctx, cur); err != nil {
			return err
		}
		return h.Events.Publish(ctx, write.Event{Type: "enrollment.changed", SubjectType: "enrollment", SubjectID: cur.ID.String(), TenantID: &t,
			Payload: enrollmentPayload(cur, "ended", &st.run.ID)})
	}
	return nil
}

func (h SisRun) applyEnrollment(ctx context.Context, st *runState, it write.StagingItem) error {
	d := it.Payload
	studentExt, classExt := sis.Str(d, "student_external_id"), sis.Str(d, "class_external_id")
	userID, ok := st.students[studentExt]
	if !ok {
		s, err := h.Sync.SyncedStudentOf(ctx, st.in.ConnectionID, studentExt)
		if err != nil {
			return err
		}
		if s == nil || s.UserID == nil {
			return conflict(sis.IssueOrphanEnrollment, map[string]any{"student_external_id": studentExt, "class_external_id": classExt})
		}
		userID = *s.UserID
	}
	classID, _, found, err := h.Sync.LinkOf(ctx, st.in.ConnectionID, sis.KindClass, classExt)
	if err != nil {
		return err
	}
	if !found {
		return conflict(sis.IssueOrphanEnrollment, map[string]any{"student_external_id": studentExt, "class_external_id": classExt})
	}
	classSchool, _, err := h.Sync.ClassSchool(ctx, classID)
	if err != nil {
		return err
	}
	if classSchool != st.in.SchoolID {
		return conflict(sis.IssueClassSchoolMismatch, map[string]any{"student_external_id": studentExt, "class_external_id": classExt})
	}
	if sis.Str(d, "status") != "active" {
		return h.Sync.SaveLink(ctx, st.in.ConnectionID, sis.KindEnrollment, it.ExternalID, classID, &classSchool, it.Hash)
	}
	start := time.Now()
	if s := sis.Date(d, "start_date"); s != nil {
		start = *s
	}
	target := enrollment.Placement{TenantID: st.tenantID, SchoolID: st.in.SchoolID, ClassID: classID}
	cur, err := h.Enrollments.LockActive(ctx, userID)
	var pe *problem.Error
	t := st.tenantID
	switch {
	case errors.As(err, &pe) && pe.Code == problem.NotFound:
		opened, err := enrollment.Open(ids.New(), userID, target, start, enrollment.SourceSIS)
		if err != nil {
			return err
		}
		if err := h.Enrollments.Open(ctx, opened); err != nil {
			return err
		}
		if err := h.Events.Publish(ctx, write.Event{Type: "enrollment.changed", SubjectType: "enrollment", SubjectID: opened.ID.String(), TenantID: &t,
			Payload: enrollmentPayload(opened, "added", &st.run.ID)}); err != nil {
			return err
		}
	case err != nil:
		return err
	case cur.Placement == target:
		// already enrolled in this class
	case cur.TenantID != st.tenantID || cur.SchoolID != st.in.SchoolID:
		return conflict(sis.IssueMultiSchoolEnrollment, map[string]any{"student_external_id": studentExt})
	default:
		tr, err := enrollment.TransferTo(cur, ids.New(), target, start, enrollment.SourceSIS)
		if err != nil {
			return skip(problem.ValidationFailed, fmt.Sprintf("enrollment: %v", err))
		}
		if err := h.Enrollments.Close(ctx, tr.Closed); err != nil {
			return err
		}
		if err := h.Enrollments.Open(ctx, tr.Opened); err != nil {
			return err
		}
		for _, x := range []struct {
			e  *enrollment.Enrollment
			op string
		}{{tr.Closed, "transferred"}, {tr.Opened, "added"}} {
			if err := h.Events.Publish(ctx, write.Event{Type: "enrollment.changed", SubjectType: "enrollment", SubjectID: x.e.ID.String(), TenantID: &t,
				Payload: enrollmentPayload(x.e, x.op, &st.run.ID)}); err != nil {
				return err
			}
		}
	}
	return h.Sync.SaveLink(ctx, st.in.ConnectionID, sis.KindEnrollment, it.ExternalID, classID, &classSchool, it.Hash)
}

// afterApply raises missing_class for students of the run without an active enrollment (F011-AC-15).
func (h SisRun) afterApply(ctx context.Context, st *runState, _ []SisFile) error {
	return h.UoW.Do(ctx, func(ctx context.Context) error {
		for ext, userID := range st.students {
			if st.inactive[ext] {
				continue
			}
			_, err := h.Enrollments.LockActive(ctx, userID)
			var pe *problem.Error
			if errors.As(err, &pe) && pe.Code == problem.NotFound {
				if err := h.issue(ctx, st, sis.IssueMissingClass, map[string]any{"student_external_id": ext, "user_id": userID.String()}); err != nil {
					return err
				}
				continue
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
}
