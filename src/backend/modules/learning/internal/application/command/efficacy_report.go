package command

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"log/slog"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/efficacy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

const (
	efficacyReportsPerRun   = 2
	efficacyReportStale     = 30 * time.Minute // a running report without progress is retried after this [Proposed]
	efficacyReportErrorRead = "report_failed"
)

// EfficacyReportView is the view of a report row.
func EfficacyReportView(r write.EfficacyReportRow) dto.EfficacyReportView {
	v := dto.EfficacyReportView{ReportID: r.ID, StudyID: r.StudyID, Status: r.Status, DescriptiveOnly: r.DescriptiveOnly, Excluded: r.Excluded,
		Results: r.Results, Error: r.Error, RequestedBy: r.RequestedBy, CreatedAt: r.CreatedAt, StartedAt: r.StartedAt, FinishedAt: r.FinishedAt}
	if v.Excluded == nil {
		v.Excluded = []uuid.UUID{}
	}
	return v
}

// CreateEfficacyStudyReport queues the report of an active or completed study (F035-AC-29): Idempotency-Key, at most 100
// excluded question versions; the job efficacy-reports computes it in the background (202).
func (h Efficacy) CreateEfficacyStudyReport(ctx context.Context, studyID uuid.UUID, excluded []uuid.UUID, key string) (out dto.EfficacyReportView,
	err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.CreateEfficacyStudyReport")
	defer func() { done(err) }()
	p, err := efficacyAdmin(ctx, authz.PermPlatformEfficacyStudyManage)
	if err != nil {
		return out, err
	}
	if len(excluded) > 100 {
		return out, problem.Invalid("/excluded_question_version_ids", problem.SubInvalid, "lists at most 100 question versions")
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if key != "" {
			if prev, err := h.Store.ReportByKey(ctx, p.UserID, key); err != nil || prev != nil {
				if err == nil {
					out = EfficacyReportView(*prev)
				}
				return err
			}
		}
		s, err := h.Store.Study(ctx, studyID, false)
		if err != nil {
			return err
		}
		if s == nil {
			return problem.New(problem.NotFound, "")
		}
		if s.Status != efficacy.StudyActive && s.Status != efficacy.StudyCompleted {
			return problem.New(problem.InvalidStateTransition, "a report needs an active or completed study")
		}
		r := write.EfficacyReportRow{ID: ids.New(), StudyID: studyID, Status: "queued", Excluded: excluded, RequestedBy: p.UserID,
			IdempotencyKey: truncateRunes(key, 64), CreatedAt: h.now()}
		if r.IdempotencyKey == "" {
			r.IdempotencyKey = r.ID.String()
		}
		if err := h.Store.InsertReport(ctx, r); err != nil {
			return err
		}
		out = EfficacyReportView(r)
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// RunEfficacyReports is the job efficacy-reports: claims the queued reports (FOR UPDATE SKIP LOCKED) and computes them on
// the read session under the audited bypass efficacy_study; audit efficacy_report.generated. Returns how many ran.
func (h Efficacy) RunEfficacyReports(ctx context.Context) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "learning.job.EfficacyReports")
	defer func() { done(err) }()
	bctx, err := studyScope(ctx, tenancy.ModeRead)
	if err != nil {
		return 0, err
	}
	var claimed []write.EfficacyReportRow
	if err := h.UoW.Do(bctx, func(ctx context.Context) error {
		var err error
		claimed, err = h.Store.ClaimReports(ctx, h.now().Add(-efficacyReportStale), efficacyReportsPerRun)
		return err
	}); err != nil {
		return 0, err
	}
	for _, r := range claimed {
		start := time.Now()
		res, hash, err := h.computeReport(bctx, r)
		at := h.now()
		r.FinishedAt = &at
		if err != nil {
			slog.WarnContext(ctx, "efficacy report failed", "report_id", r.ID, "error", err)
			code := efficacyReportErrorRead
			r.Status, r.Error = "failed", &code
		} else {
			b, _ := json.Marshal(res.Results)
			r.Status, r.Results, r.InputsHash, r.DescriptiveOnly = "succeeded", b, hash, res.DescriptiveOnly
		}
		if err := h.UoW.Do(bctx, func(ctx context.Context) error {
			if err := h.Store.FinishReport(ctx, r); err != nil {
				return err
			}
			if r.Status != "succeeded" {
				return nil
			}
			return h.auditPlatform(ctx, ActionEfficacyReport, "efficacy_report", r.ID, "", map[string]any{"study_id": r.StudyID,
				"descriptive_only": r.DescriptiveOnly, "excluded_count": len(r.Excluded)})
		}); err != nil {
			return n, err
		}
		if h.Metrics != nil {
			h.Metrics.ReportDuration(time.Since(start).Seconds())
		}
		n++
	}
	return n, nil
}

// reportData gathers the inputs of a report (read session, bypass efficacy_study).
func (h Efficacy) reportData(ctx context.Context, studyID uuid.UUID) (efficacy.ReportInput, error) {
	var in efficacy.ReportInput
	if h.Data == nil || h.Read == nil {
		return in, problem.New(problem.Unavailable, "the efficacy data reader is not wired")
	}
	err := h.Read.Read(ctx, func(ctx context.Context) error {
		s, err := h.Data.StudyOf(ctx, studyID)
		if err != nil || s == nil {
			if err == nil {
				err = problem.New(problem.NotFound, "")
			}
			return err
		}
		in.Objectives = s.ObjectiveIDs
		classes, err := h.Data.ClassesOf(ctx, studyID)
		if err != nil {
			return err
		}
		in.Classes = map[string]int{}
		for _, c := range classes {
			if c.Arm != nil {
				in.Classes[*c.Arm]++
			}
		}
		parts, err := h.Data.ParticipantsOf(ctx, studyID)
		if err != nil {
			return err
		}
		byTenant := map[uuid.UUID][]uuid.UUID{}
		for _, p := range parts {
			in.Participants = append(in.Participants, efficacy.Participant{StudentID: p.StudentID, ClassID: p.ClassID, Arm: p.Arm, Status: p.Status})
			if p.Status == efficacy.ParticipantActive {
				byTenant[p.TenantID] = append(byTenant[p.TenantID], p.StudentID)
			}
		}
		waves, err := h.Data.WavesOf(ctx, studyID)
		if err != nil {
			return err
		}
		var from, to time.Time
		for _, w := range waves {
			if w.Status == efficacy.WaveCancelled {
				continue
			}
			answers, err := h.Data.Answers(ctx, w.ID)
			if err != nil {
				return err
			}
			for _, a := range answers {
				in.Answers = append(in.Answers, efficacy.Answer{StudentID: a.StudentID, WaveKind: a.WaveKind, Form: a.Form,
					QuestionVersionID: a.QuestionVersionID, ObjectiveIDs: a.ObjectiveIDs, Correct: a.Correct, Score: a.Score, Max: a.Max})
			}
			switch w.Kind {
			case efficacy.WavePre:
				from = w.WindowEnd
			case efficacy.WavePost:
				to = w.WindowStart
			}
		}
		snaps, err := h.Data.SnapshotsOf(ctx, studyID)
		if err != nil {
			return err
		}
		for _, sn := range snaps {
			in.Snapshots = append(in.Snapshots, efficacy.Snapshot{StudentID: sn.StudentID, ObjectiveID: sn.ObjectiveID, MEff: sn.MEff, Level: sn.Level})
		}
		if from.IsZero() || to.IsZero() || !to.After(from) {
			return nil // usage needs the intervention window between the pre and the post waves
		}
		tenants := make([]uuid.UUID, 0, len(byTenant))
		for t := range byTenant {
			tenants = append(tenants, t)
		}
		sort.Slice(tenants, func(i, j int) bool { return tenants[i].String() < tenants[j].String() })
		for _, t := range tenants {
			us, err := h.Data.Usage(ctx, t, byTenant[t], from, to)
			if err != nil {
				return err
			}
			for _, u := range us {
				in.Usage = append(in.Usage, efficacy.Usage{StudentID: u.StudentID, ActiveSeconds: u.ActiveSeconds, Days: u.Days, Answers: u.Answers,
					RapidGuesses: u.RapidGuesses, PracticeAttempts: u.PracticeAttempts})
			}
			recs, err := h.Data.Recommendations(ctx, t, byTenant[t], from, to)
			if err != nil {
				return err
			}
			for _, r := range recs {
				in.Recommendations = append(in.Recommendations, efficacy.Recommendation{StudentID: r.StudentID, ObjectiveIDs: r.ObjectiveIDs,
					Followed: r.Followed})
			}
		}
		return nil
	})
	return in, err
}

// computeReport builds the report of a claimed row; the inputs hash makes a recomputation comparable (same input, same
// output, F035-AC-29).
func (h Efficacy) computeReport(ctx context.Context, r write.EfficacyReportRow) (efficacy.ReportResult, []byte, error) {
	in, err := h.reportData(ctx, r.StudyID)
	if err != nil {
		return efficacy.ReportResult{}, nil, err
	}
	in.Excluded = r.Excluded
	raw, err := json.Marshal(in)
	if err != nil {
		return efficacy.ReportResult{}, nil, err
	}
	sum := sha256.Sum256(raw)
	return efficacy.BuildReport(in, h.config()), sum[:], nil
}
