package command

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/placement"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/sis"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// SisDelete is a Change with operation = delete of a delta or webhook run (F017-AC-17).
type SisDelete struct {
	Kind       string // sis.KindSchool…
	ExternalID string
}

// deleteKind keys the staged deletes of a run (never an entity kind).
const deleteKind = "delete"

func orNew(m map[string]int) map[string]int {
	if m == nil {
		return map[string]int{}
	}
	return m
}

// stageDeletes records the deletes as staging items with action deactivate (no payload, no hash).
func stageDeletes(ds []SisDelete) []write.StagingItem {
	out := make([]write.StagingItem, 0, len(ds))
	for _, d := range ds {
		out = append(out, write.StagingItem{Kind: d.Kind, ExternalID: truncate(d.ExternalID, 64), Hash: "-", Action: sis.ActionDeactivate,
			Status: "pending", Payload: map[string]any{"external_id": truncate(d.ExternalID, 200), "operation": "delete"}})
	}
	return out
}

// PulledSchool is the data of one school fetched by a contract_v1 run (F017): valid records only, the
// per-record contract violations, the deletes of delta/webhook changes.
type PulledSchool struct {
	SchoolID     uuid.UUID
	Files        []SisFile
	RecordErrors []write.RecordError
	Deletes      []SisDelete
	FullStudents bool // the students file is the full list of the school (missing_in_source, F017-AC-08)
}

// ProcessPulled stages and, unless the run is a dry run, applies the data of one school inside an existing
// run through the F011 pipeline (staging → reconcile → apply). The caller sets the tenant scope of the
// connection, creates and finishes the run; the returned stats are those of this school.
func (h SisRun) ProcessPulled(ctx context.Context, run *write.SyncRun, in PulledSchool) (RunImportResult, map[string]int, error) {
	st := newRunState(RunImportInput{ConnectionID: run.ConnectionID, SchoolID: in.SchoolID, DryRun: run.DryRun}, run.TenantID)
	st.run, st.full = run, in.FullStudents
	out := RunImportResult{RunID: run.ID, DryRun: run.DryRun, Diff: map[string]map[string]int{}, Errors: []write.RecordError{}}
	if _, err := h.process(ctx, st, in.Files, in.RecordErrors, in.Deletes, &out); err != nil {
		return out, st.stats, err
	}
	out.Issues = st.issues
	return out, st.stats, nil
}

// RaiseRunIssue raises an issue of a pull run outside one school (unmapped_school, orphan change,
// F017-AC-09) and publishes sis.reconciliation_issue_raised; the caller holds the transaction.
func (h SisRun) RaiseRunIssue(ctx context.Context, run *write.SyncRun, schoolID *uuid.UUID, typ string, refs map[string]any) error {
	id, err := h.Sync.RaiseIssue(ctx, write.Issue{SchoolID: schoolID, ConnectionID: run.ConnectionID, RunID: run.ID, Type: typ, Refs: refs})
	if err != nil {
		return err
	}
	t := run.TenantID
	payload := map[string]any{"issue_id": id.String(), "kind": typ, "sis_connection_id": run.ConnectionID.String()}
	if schoolID != nil {
		payload["school_id"] = schoolID.String()
	}
	return h.Events.Publish(ctx, write.Event{Type: "sis.reconciliation_issue_raised", SubjectType: "reconciliation_issue", SubjectID: id.String(),
		TenantID: &t, Payload: payload})
}

// applyDeletes applies the deletes after the upserts of the run, each in a savepoint: a deleted student is
// deactivated and loses its effective placement (the SIS record status inactive, F011-AC-23, ADR-0095) so the state is
// effective in the same run; school deletes need a person (soft-delete only, AGENTS R21) and are only counted. Class
// and enrollment deletes are dropped before staging (SIS contract 1.3.0, ADR-0095 D6).
func (h SisRun) applyDeletes(ctx context.Context, st *runState, items []write.StagingItem, size int) error {
	for start := 0; start < len(items); start += size {
		end := min(start+size, len(items))
		if err := h.UoW.Do(ctx, func(ctx context.Context) error {
			for _, it := range items[start:end] {
				ext, _ := it.Payload["external_id"].(string)
				err := h.Savepoints.Savepoint(ctx, func(ctx context.Context) error { return h.applyDelete(ctx, st, it.Kind, ext) })
				var pe *problem.Error
				switch {
				case err == nil:
				case errors.As(err, &pe):
					st.stats[it.Kind+"_failed"]++
				default:
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func (h SisRun) applyDelete(ctx context.Context, st *runState, kind, ext string) error {
	switch kind {
	case sis.KindStudent:
		s, err := h.Sync.SyncedStudentOf(ctx, st.in.ConnectionID, ext)
		if err != nil {
			return err
		}
		if s == nil || s.UserID == nil {
			st.stats["student_delete_unknown"]++
			return nil
		}
		tr, err := trackPlacement(ctx, h.Students, h.Events, *s.UserID)
		if err != nil {
			return err
		}
		if err := h.applyStatus(ctx, st, *s.UserID, "inactive"); err != nil {
			return err
		}
		if err := tr.done(ctx, PlacementChange{Source: placement.SourceSIS, SyncRunID: &st.run.ID}); err != nil {
			return err
		}
		s.RecordStatus, s.Hash = "inactive", "" // the next upsert of the source key is applied again
		if err := h.Sync.SaveSyncedStudent(ctx, *s); err != nil {
			return err
		}
		st.stats["student_deactivated"]++
		return h.Audit.Record(ctx, audit.Event{Action: "student.synced", TargetType: "student", TargetID: s.UserID.String(), Result: audit.ResultSuccess,
			Metadata: map[string]any{"actor": "system:sis-sync:" + st.in.ConnectionID.String(), "action": "deactivate"}})
	default:
		st.stats[kind+"_delete_skipped"]++
		return nil
	}
}
