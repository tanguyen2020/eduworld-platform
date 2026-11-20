package command

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/blueprint"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/efficacy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/selection"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// WaveInput is EfficacyWaveCreate.
type WaveInput struct {
	efficacy.WaveRequest
	IdempotencyKey string
}

// WaveViewOf is the view of a wave with its sessions (counts are added by the query).
func WaveViewOf(w write.WaveRow, sessions []write.WaveSessionRow) dto.EfficacyWaveView {
	v := dto.EfficacyWaveView{WaveID: w.ID, StudyID: w.StudyID, Kind: w.Kind, Form: w.Form, WindowStart: w.WindowStart, WindowEnd: w.WindowEnd,
		DurationSeconds: w.DurationSec, Status: w.Status, OpenedAt: w.OpenedAt, ClosedAt: w.ClosedAt, Classes: []dto.EfficacyWaveClassView{}}
	for _, s := range sessions {
		v.Classes = append(v.Classes, dto.EfficacyWaveClassView{ClassID: s.ClassID, TenantID: s.TenantID, ExamSessionID: s.ExamSessionID,
			Form: s.Form, SessionStatus: s.Status})
	}
	return v
}

// instrument is a form of the study with its pool of fixed questions.
type instrument struct {
	form write.FormVersion
	pool []selection.Candidate
}

func (h Efficacy) instrument(ctx context.Context, versionID uuid.UUID) (instrument, error) {
	f, err := h.Content.FormVersion(ctx, versionID)
	if err != nil {
		return instrument{}, err
	}
	if f == nil || f.Status != "published" {
		return instrument{}, problem.Invalid("/form", problem.SubInvalid, "a form of the study is not a published blueprint version")
	}
	var body blueprintBody
	_ = json.Unmarshal(f.Body, &body)
	def, err := blueprint.Parse(body.Definition)
	if err != nil {
		return instrument{}, problem.Invalid("/form", problem.SubInvalid, "a form of the study cannot be read")
	}
	pool, err := h.Content.InstrumentPool(ctx, def.FixedQuestions())
	if err != nil {
		return instrument{}, err
	}
	return instrument{form: *f, pool: pool}, nil
}

// ScheduleEfficacyWave schedules a validation, pre or post wave (F035-AC-12, AC-13): the checks of efficacy.CheckWave;
// one exam session per target class in the tenant of the class with the fixed efficacy settings and the form of the
// class (pre the first form of its order, post the second, validation the requested one); the audience is the class
// (validation, pre) or its active participants (post); idempotent by Idempotency-Key; audit efficacy_wave.scheduled.
func (h Efficacy) ScheduleEfficacyWave(ctx context.Context, studyID uuid.UUID, in WaveInput) (out dto.EfficacyWaveView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.ScheduleEfficacyWave")
	defer func() { done(err) }()
	p, err := efficacyAdmin(ctx, authz.PermPlatformEfficacyStudyManage)
	if err != nil {
		return out, err
	}
	if h.Exams == nil {
		return out, problem.New(problem.Unavailable, "the exam engine is not wired").Retry()
	}
	bctx, err := studyScope(ctx, tenancy.ModeWrite)
	if err != nil {
		return out, err
	}
	created := 0
	err = h.UoW.Do(bctx, func(ctx context.Context) error {
		if in.IdempotencyKey != "" {
			if prev, err := h.Store.WaveByKey(ctx, p.UserID, in.IdempotencyKey); err != nil || prev != nil {
				if err == nil {
					sessions, err := h.Store.WaveSessions(ctx, prev.ID)
					out = WaveViewOf(*prev, sessions)
					return err
				}
				return err
			}
		}
		s, err := h.Store.Study(ctx, studyID, true)
		if err != nil {
			return err
		}
		if s == nil {
			return problem.New(problem.NotFound, "")
		}
		rows, err := h.Store.Waves(ctx, studyID)
		if err != nil {
			return err
		}
		existing := make([]efficacy.Wave, len(rows))
		for i, w := range rows {
			existing[i] = efficacy.Wave{ID: w.ID, Kind: w.Kind, WindowStart: w.WindowStart, WindowEnd: w.WindowEnd, Status: w.Status}
		}
		vs, err := efficacy.CheckWave(in.WaveRequest, s.Status, s.MinWeeks, existing, h.now())
		if err != nil {
			return stateErr(err)
		}
		if err := efficacyViolations(vs); err != nil {
			return err
		}
		classes, err := h.Store.StudyClasses(ctx, studyID)
		if err != nil {
			return err
		}
		var targets []write.StudyClassRow
		for _, c := range classes {
			if c.Status == "enrolled" && c.Arm != nil && efficacy.TargetsWave(in.Kind, *c.Arm) {
				targets = append(targets, c)
			}
		}
		if len(targets) == 0 {
			return problem.Invalid("/kind", problem.SubInvalid, "the study has no enrolled class for this wave")
		}
		forms := map[string]instrument{}
		for form, id := range map[string]uuid.UUID{efficacy.FormA: s.FormA, efficacy.FormB: s.FormB} {
			if forms[form], err = h.instrument(ctx, id); err != nil {
				return err
			}
		}
		w := write.WaveRow{ID: ids.New(), StudyID: studyID, Kind: in.Kind, Form: optString(in.Form), WindowStart: in.WindowStart.UTC(),
			WindowEnd: in.WindowEnd.UTC(), DurationSec: in.DurationSec, Status: efficacy.WaveScheduled, CreatedBy: p.UserID,
			IdempotencyKey: truncateRunes(in.IdempotencyKey, 64), Version: 1, CreatedAt: h.now()}
		if w.IdempotencyKey == "" {
			w.IdempotencyKey = w.ID.String()
		}
		if w.DurationSec == 0 {
			w.DurationSec = 1 // replaced by the duration of the blueprint below
		}
		var sessions []write.WaveSessionRow
		for _, c := range targets {
			order := efficacy.OrderAThenB
			if c.FormOrder != nil {
				order = *c.FormOrder
			}
			form := efficacy.FormOf(in.Kind, order, in.Form)
			students, err := h.waveAudience(ctx, studyID, in.Kind, c)
			if err != nil {
				return err
			}
			if err := h.Store.SessionTenant(ctx, c.TenantID); err != nil {
				return err
			}
			row, err := h.Exams.CreateEfficacySession(ctx, EfficacySessionInput{WaveID: w.ID, TenantID: c.TenantID, SchoolID: c.SchoolID,
				ClassID: c.ClassID, Title: s.Title, Form: forms[form].form, Pool: forms[form].pool, WindowStart: w.WindowStart, WindowEnd: w.WindowEnd,
				DurationSec: in.DurationSec, Students: students, CreatedBy: p.UserID})
			if errors.Is(err, ErrEfficacyPool) {
				return problem.Invalid("/form", problem.SubInvalid, "a question of the form is no longer published")
			}
			if err != nil {
				return err
			}
			if in.DurationSec == 0 {
				w.DurationSec = row.DurationSec
			}
			sessions = append(sessions, write.WaveSessionRow{WaveID: w.ID, ClassID: c.ClassID, TenantID: c.TenantID, SchoolID: c.SchoolID,
				ExamSessionID: row.ID, Form: form, Status: "active"})
		}
		if err := h.Store.InsertWave(ctx, w); err != nil {
			return err
		}
		for _, ws := range sessions {
			if err := h.Store.InsertWaveSession(ctx, ws); err != nil {
				return err
			}
		}
		created = len(sessions)
		if err := h.auditPlatform(ctx, ActionWaveScheduled, "efficacy_wave", w.ID, "", map[string]any{"study_id": studyID, "kind": w.Kind,
			"exam_session_count": len(sessions), "tenant_ids": studyTenants(targets)}); err != nil {
			return err
		}
		out = WaveViewOf(w, sessions)
		return idempotency.SetResult(ctx, out)
	})
	if err == nil && h.Metrics != nil {
		h.Metrics.WaveSessions(created)
	}
	return out, err
}

// waveAudience is the audience of the session of a class: the active students of the class for validation and pre,
// the active participants snapshotted in that class for post (F035-AC-14).
func (h Efficacy) waveAudience(ctx context.Context, studyID uuid.UUID, kind string, c write.StudyClassRow) ([]uuid.UUID, error) {
	if kind != efficacy.WavePost {
		return h.Store.ClassStudents(ctx, c.TenantID, c.ClassID)
	}
	ps, err := h.Store.Participants(ctx, studyID, &c.ClassID, efficacy.ParticipantActive)
	if err != nil {
		return nil, err
	}
	out := make([]uuid.UUID, len(ps))
	for i, p := range ps {
		out[i] = p.StudentID
	}
	return out, nil
}

// WaveMonitorReport counts what the job efficacy-wave-monitor did.
type WaveMonitorReport struct {
	Opened, Closed, Participants, Snapshots int
}

// RunWaveMonitor is the job efficacy-wave-monitor (every minute, plan "Bảo mật"): scheduled waves whose window started
// open — the pre wave snapshots the participants of each class (F035-AC-14), the post wave the M_eff of the intervention
// participants (F035-AC-28) — and open waves whose window ended close. Every tenant under the audited bypass
// efficacy_study; one transaction per wave.
func (h Efficacy) RunWaveMonitor(ctx context.Context, limit int) (rep WaveMonitorReport, err error) {
	ctx, done := observability.UseCase(ctx, "learning.job.EfficacyWaveMonitor")
	defer func() { done(err) }()
	if limit <= 0 {
		limit = 50
	}
	bctx, err := studyScope(ctx, tenancy.ModeWrite)
	if err != nil {
		return rep, err
	}
	var due []write.WaveRow
	if err := h.UoW.Do(bctx, func(ctx context.Context) error {
		var err error
		due, err = h.Store.DueWaves(ctx, h.now(), limit)
		return err
	}); err != nil {
		return rep, err
	}
	for _, w := range due {
		if err := h.UoW.Do(bctx, func(ctx context.Context) error { return h.advanceWave(ctx, w.ID, &rep) }); err != nil {
			slog.ErrorContext(ctx, "efficacy wave", "wave_id", w.ID, "error", err)
			return rep, err
		}
	}
	return rep, nil
}

func (h Efficacy) advanceWave(ctx context.Context, id uuid.UUID, rep *WaveMonitorReport) error {
	w, err := h.Store.Wave(ctx, id, true)
	if err != nil || w == nil {
		return err
	}
	now := h.now()
	prev := w.Version
	switch {
	case w.Status == efficacy.WaveScheduled && !w.WindowStart.After(now):
		s, err := h.Store.Study(ctx, w.StudyID, false)
		if err != nil || s == nil {
			return err
		}
		switch w.Kind {
		case efficacy.WavePre:
			n, err := h.snapshotParticipants(ctx, *s, w.ID, now)
			if err != nil {
				return err
			}
			rep.Participants += n
		case efficacy.WavePost:
			n, err := h.snapshotMastery(ctx, *s, now)
			if err != nil {
				return err
			}
			rep.Snapshots += n
		}
		w.Status, w.OpenedAt = efficacy.WaveOpen, &now
		if _, err := h.Store.UpdateWave(ctx, *w, prev); err != nil {
			return err
		}
		rep.Opened++
		return h.auditPlatform(ctx, ActionWaveOpened, "efficacy_wave", w.ID, "", map[string]any{"study_id": w.StudyID, "kind": w.Kind})
	case w.Status == efficacy.WaveOpen && !w.WindowEnd.After(now):
		w.Status, w.ClosedAt = efficacy.WaveClosed, &now
		if _, err := h.Store.UpdateWave(ctx, *w, prev); err != nil {
			return err
		}
		rep.Closed++
		return h.auditPlatform(ctx, ActionWaveClosed, "efficacy_wave", w.ID, "", map[string]any{"study_id": w.StudyID, "kind": w.Kind})
	}
	return nil
}

// snapshotParticipants records the active students of every class of the pre wave as participants with the arm and
// form order of their class (F035-AC-14); accounts that are not active (anonymized, processing restricted) are left out.
func (h Efficacy) snapshotParticipants(ctx context.Context, s write.StudyRow, waveID uuid.UUID, now time.Time) (int, error) {
	sessions, err := h.Store.WaveSessions(ctx, waveID)
	if err != nil {
		return 0, err
	}
	classes, err := h.Store.StudyClasses(ctx, s.ID)
	if err != nil {
		return 0, err
	}
	byClass := map[uuid.UUID]write.StudyClassRow{}
	for _, c := range classes {
		byClass[c.ClassID] = c
	}
	total := 0
	for _, ws := range sessions {
		c, ok := byClass[ws.ClassID]
		if !ok || ws.Status != "active" || c.Status != "enrolled" || c.Arm == nil || c.FormOrder == nil {
			continue
		}
		students, err := h.Store.ClassStudents(ctx, c.TenantID, c.ClassID)
		if err != nil {
			return total, err
		}
		// Students withdrawn by name before the opening never become participants (F035-AC-18).
		gone, err := h.Store.WithdrawnStudents(ctx, c.TenantID, s.ID)
		if err != nil {
			return total, err
		}
		students = slices.DeleteFunc(students, func(id uuid.UUID) bool { return slices.Contains(gone, id) })
		ps := make([]write.ParticipantRow, len(students))
		for i, st := range students {
			ps[i] = write.ParticipantRow{StudyID: s.ID, StudentID: st, TenantID: c.TenantID, SchoolID: c.SchoolID, ClassID: c.ClassID, Arm: *c.Arm,
				FormOrder: *c.FormOrder, Status: efficacy.ParticipantActive, EnrolledAt: now}
		}
		n, err := h.Store.InsertParticipants(ctx, ps)
		if err != nil {
			return total, err
		}
		total += n
	}
	if h.Metrics != nil {
		h.Metrics.Participants(efficacy.ParticipantActive, total)
	}
	return total, nil
}

// snapshotMastery records M_eff and the level (active model, after decay) of every active intervention participant on
// every objective of the study when the post wave opens (F035-AC-28); a participant without evidence is not_started.
func (h Efficacy) snapshotMastery(ctx context.Context, s write.StudyRow, now time.Time) (int, error) {
	ps, err := h.Store.Participants(ctx, s.ID, nil, efficacy.ParticipantActive)
	if err != nil {
		return 0, err
	}
	cfg, err := h.Store.ActiveModel(ctx)
	if err != nil {
		return 0, err
	}
	m, err := ModelOf(cfg)
	if err != nil {
		return 0, err
	}
	byTenant := map[uuid.UUID][]uuid.UUID{}
	for _, p := range ps {
		if p.Arm == efficacy.ArmIntervention {
			byTenant[p.TenantID] = append(byTenant[p.TenantID], p.StudentID)
		}
	}
	total := 0
	for tenant, students := range byTenant {
		rows, err := h.Store.StudentMasteries(ctx, tenant, students, s.ObjectiveIDs)
		if err != nil {
			return total, err
		}
		have := map[[2]uuid.UUID]write.MasteryRow{}
		for _, r := range rows {
			have[[2]uuid.UUID{r.StudentID, r.ObjectiveID}] = r
		}
		var snaps []write.SnapshotRow
		for _, st := range students {
			for _, o := range s.ObjectiveIDs {
				r, ok := have[[2]uuid.UUID{st, o}]
				snap := write.SnapshotRow{StudyID: s.ID, StudentID: st, ObjectiveID: o, TenantID: tenant, ModelVersion: m.ModelVersion, TakenAt: now,
					Level: "not_started"}
				if ok {
					v, level := m.Effective(r.State, now)
					snap.MEff, snap.Level, snap.EvidenceCount = round3(v), string(level), r.State.EvidenceCount
				}
				snaps = append(snaps, snap)
			}
		}
		if err := h.Store.InsertSnapshots(ctx, snaps); err != nil {
			return total, err
		}
		total += len(snaps)
	}
	return total, nil
}
