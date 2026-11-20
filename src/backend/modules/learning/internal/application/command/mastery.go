package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/mastery"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// Events of the mastery model (LEARNING_ANALYTICS §3.4; payloads learning_objective.*.v1, owner F031).
const (
	EventObjectiveProgressed = "learning_objective.progressed"
	EventObjectiveMastered   = "learning_objective.mastered"
)

// Kinds of mastery changes (learning.mastery_history.source_kind).
const (
	MasterySourceEvidence     = "evidence"
	MasterySourceReEvaluation = "re_evaluation"
	MasterySourceRecompute    = "recompute"
)

// masteryRetries bounds the optimistic retries of one (student, objective) inside the consumer transaction
// (F031-AC-14); past it the event is redelivered by the consumer.
const masteryRetries = 5

// ErrMasteryContended is returned when the row of (student, objective) kept changing under the update.
var ErrMasteryContended = errors.New("mastery: concurrent updates of the same objective")

// MasteryMetrics counts updates (eduworld_mastery_updates_total{result}) and the lag between the graded event and the
// update (eduworld_mastery_update_lag_seconds); nil = not counted.
type MasteryMetrics interface {
	Updated(result string)
	Lag(seconds float64)
}

// Mastery applies evidence to the mastery of (student, objective) pairs (F031 US2): consumer learning.mastery_on_graded.
type Mastery struct {
	Store    write.MasteryStore
	Evidence write.EvidenceReader
	Learning write.LearningEventWriter
	Events   write.EventPublisher
	Clock    func() time.Time
	Metrics  MasteryMetrics
}

// GradedAttempt is the activity.graded event the consumer received.
type GradedAttempt struct {
	EventID    uuid.UUID
	TenantID   uuid.UUID
	AttemptID  uuid.UUID
	OccurredAt time.Time
}

// Model returns the active model version with its parameters (defaults of LEARNING_ANALYTICS §3.1 under the stored ones).
func (h Mastery) Model(ctx context.Context) (mastery.WeightedEvidence, error) {
	c, err := h.Store.ActiveModel(ctx)
	if err != nil {
		return mastery.WeightedEvidence{}, err
	}
	return ModelOf(c)
}

// ModelOf builds the model of a configuration row.
func ModelOf(c write.ModelConfig) (mastery.WeightedEvidence, error) {
	p := mastery.DefaultParams()
	if len(c.Params) > 0 {
		if err := json.Unmarshal(c.Params, &p); err != nil {
			return mastery.WeightedEvidence{}, fmt.Errorf("mastery model %s: %w", c.ModelVersion, err)
		}
	}
	if err := p.Validate(); err != nil {
		return mastery.WeightedEvidence{}, fmt.Errorf("mastery model %s: %w", c.ModelVersion, err)
	}
	return mastery.WeightedEvidence{ModelVersion: c.ModelVersion, P: p}, nil
}

// ApplyGraded updates every objective with evidence in a graded attempt (F031-AC-08): the evidence of the pair is
// replayed in occurred_at order with the active model, so a late or redelivered event gives the same result as
// sequential processing (F031-AC-14). Runs in the transaction and TenantScope of the consumer.
func (h Mastery) ApplyGraded(ctx context.Context, in GradedAttempt) (err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.ApplyMasteryEvidence")
	defer func() { done(err) }()
	facts, err := h.Evidence.Attempt(ctx, in.AttemptID)
	if err != nil {
		return err
	}
	objectives, err := h.Evidence.AttemptObjectives(ctx, in.AttemptID)
	if err != nil || len(objectives) == 0 {
		return err
	}
	m, err := h.Model(ctx)
	if err != nil {
		return err
	}
	src := Source{EventID: in.EventID, TenantID: in.TenantID, StudentID: facts.StudentID, SchoolID: facts.SchoolID, Mode: facts.Mode,
		Kind: MasterySourceEvidence, OccurredAt: in.OccurredAt}
	for _, o := range objectives {
		if err := h.Recompute(ctx, m, src, o); err != nil {
			return err
		}
	}
	if h.Metrics != nil && !in.OccurredAt.IsZero() {
		h.Metrics.Lag(h.now().Sub(in.OccurredAt).Seconds())
	}
	return nil
}

// Source is what caused a mastery change (a graded attempt, a re-evaluation or a recompute).
type Source struct {
	EventID    uuid.UUID
	TenantID   uuid.UUID
	StudentID  uuid.UUID
	SchoolID   uuid.UUID
	Mode       string
	Kind       string
	OccurredAt time.Time
}

func (h Mastery) now() time.Time { return now(h.Clock).UTC() }

func round3(v float64) float64 { return math.Round(v*1000) / 1000 }

// EvidenceOf converts the stored evidence rows of a pair to model evidence.
func EvidenceOf(rows []write.EvidenceRow) []mastery.Evidence {
	out := make([]mastery.Evidence, 0, len(rows))
	for _, r := range rows {
		o := 0.0
		if r.Max > 0 {
			o = r.Raw / r.Max
		}
		out = append(out, mastery.Evidence{EventID: r.EventID, OccurredAt: r.OccurredAt, Context: r.Mode, Difficulty: r.Difficulty,
			Outcome: o, QuestionType: r.QuestionType, OptionCount: r.OptionCount, HintsUsed: r.HintsUsed, Repeat: r.Repeat})
	}
	return out
}

// Recompute rebuilds (student, objective) from its evidence with model m and stores it when it changed: the history row,
// learning_objective.progressed for a new value and learning_objective.mastered the first time only (F031-AC-13).
func (h Mastery) Recompute(ctx context.Context, m mastery.WeightedEvidence, src Source, objective uuid.UUID) error {
	for range masteryRetries {
		row, err := h.Store.Mastery(ctx, src.StudentID, objective)
		if err != nil {
			return err
		}
		rows, err := h.Evidence.ObjectiveEvidence(ctx, src.StudentID, objective)
		if err != nil {
			return err
		}
		prior := mastery.State{}
		if row != nil {
			prior = row.State
		}
		after := mastery.Replay(m, prior, EvidenceOf(rows))
		at := h.now()
		if after.LastEvidenceAt != nil {
			at = *after.LastEvidenceAt
		}
		_, level := m.Effective(after, at)
		next := write.MasteryRow{ID: ids.New(), TenantID: src.TenantID, SchoolID: src.SchoolID, StudentID: src.StudentID, ObjectiveID: objective,
			State: after, Level: level, ModelVersion: m.Version()}
		var ok bool
		if row == nil {
			if after.EvidenceCount == 0 {
				return nil
			}
			ok, err = h.Store.InsertMastery(ctx, next)
		} else {
			if unchanged(*row, next) {
				h.count("unchanged")
				return nil
			}
			next.ID, next.Version = row.ID, row.Version
			ok, err = h.Store.UpdateMastery(ctx, next)
		}
		if err != nil {
			return err
		}
		if !ok {
			h.count("conflict")
			continue
		}
		h.count("updated")
		return h.changed(ctx, m, src, row, next)
	}
	return ErrMasteryContended
}

func (h Mastery) count(result string) {
	if h.Metrics != nil {
		h.Metrics.Updated(result)
	}
}

func unchanged(old, next write.MasteryRow) bool {
	a, b := old.State, next.State
	return round3(a.Value) == round3(b.Value) && old.Level == next.Level && a.EvidenceCount == b.EvidenceCount &&
		a.DistinctDays == b.DistinctDays && a.LastEvidenceDate == b.LastEvidenceDate && old.ModelVersion == next.ModelVersion &&
		(a.FirstMasteredAt == nil) == (b.FirstMasteredAt == nil) && sameTime(a.LastEvidenceAt, b.LastEvidenceAt)
}

func sameTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Truncate(time.Microsecond).Equal(b.Truncate(time.Microsecond))
}

// changed records the history and the events of a stored change.
func (h Mastery) changed(ctx context.Context, m mastery.WeightedEvidence, src Source, old *write.MasteryRow, next write.MasteryRow) error {
	from, prevLevel := m.P.M0, mastery.NotStarted
	var valueBefore *float64
	var levelBefore *mastery.Level
	if old != nil {
		v, l := round3(old.State.Value), old.Level
		from, prevLevel, valueBefore, levelBefore = v, l, &v, &l
	}
	to := round3(next.State.Value)
	occurred := src.OccurredAt
	if occurred.IsZero() {
		occurred = h.now()
	}
	var srcID *uuid.UUID
	if src.EventID != uuid.Nil {
		id := src.EventID
		srcID = &id
	}
	if err := h.Store.AppendHistory(ctx, write.HistoryRow{ID: ids.New(), TenantID: src.TenantID, StudentID: src.StudentID,
		ObjectiveID: next.ObjectiveID, ValueBefore: valueBefore, ValueAfter: to, LevelBefore: levelBefore, LevelAfter: next.Level,
		EvidenceCount: next.State.EvidenceCount, SourceEventID: srcID, SourceKind: src.Kind, ModelVersion: m.Version(), OccurredAt: occurred}); err != nil {
		return err
	}
	if to != from || old == nil {
		if err := h.progressed(ctx, src, next, from, to, prevLevel, occurred); err != nil {
			return err
		}
	}
	if next.State.FirstMasteredAt != nil && (old == nil || old.State.FirstMasteredAt == nil) {
		return h.mastered(ctx, src, next, prevLevel)
	}
	return nil
}

// masteryKeys namespaces the deterministic idempotency keys of the mastery learning events (UUIDv5).
var masteryKeys = uuid.MustParse("0193a1f0-31f0-7031-8031-000000000031")

func (h Mastery) learningEvent(ctx context.Context, src Source, typ, key string, at time.Time, objective uuid.UUID, payload any) error {
	if h.Learning == nil {
		return nil
	}
	mode := src.Mode
	if mode == "" {
		mode = "practice"
	}
	return h.Learning.Write(ctx, write.LearningEvent{ID: ids.New(), OccurredAt: at, Type: typ, StudentID: src.StudentID, TenantID: src.TenantID,
		SchoolID: src.SchoolID, ObjectiveIDs: []uuid.UUID{objective}, Context: mode, ActorType: "system", Payload: payload, IdempotencyKey: key})
}

func (h Mastery) publish(ctx context.Context, src Source, typ string, objective uuid.UUID, payload any) error {
	if h.Events == nil {
		return nil
	}
	t := src.TenantID
	return h.Events.Publish(ctx, write.Event{Type: typ, SubjectType: "learning_objective", SubjectID: objective.String(), TenantID: &t,
		Payload: payload})
}

func (h Mastery) progressed(ctx context.Context, src Source, next write.MasteryRow, from, to float64, prev mastery.Level, at time.Time) error {
	base := map[string]any{"objective_id": next.ObjectiveID, "from": from, "to": to, "level": next.Level, "previous_level": prev,
		"evidence_count": next.State.EvidenceCount, "model_version": next.ModelVersion, "source_event_id": src.EventID}
	key := "lop:" + uuid.NewSHA1(masteryKeys, []byte(src.EventID.String()+":"+next.ObjectiveID.String()+":"+src.Kind)).String()
	if err := h.learningEvent(ctx, src, EventObjectiveProgressed, key, at, next.ObjectiveID, base); err != nil {
		return err
	}
	domain := map[string]any{"student_id": src.StudentID}
	for k, v := range base {
		domain[k] = v
	}
	return h.publish(ctx, src, EventObjectiveProgressed, next.ObjectiveID, domain)
}

func (h Mastery) mastered(ctx context.Context, src Source, next write.MasteryRow, prev mastery.Level) error {
	s := next.State
	base := map[string]any{"objective_id": next.ObjectiveID, "previous_level": prev, "new_level": mastery.Mastered, "value": round3(s.Value),
		"evidence_count": s.EvidenceCount, "distinct_days": s.DistinctDays, "model_version": next.ModelVersion}
	key := "mst:" + uuid.NewSHA1(masteryKeys, []byte(src.StudentID.String()+":"+next.ObjectiveID.String())).String()
	if err := h.learningEvent(ctx, src, EventObjectiveMastered, key, *s.FirstMasteredAt, next.ObjectiveID, base); err != nil {
		return err
	}
	domain := map[string]any{"student_id": src.StudentID, "mastered_at": s.FirstMasteredAt.UTC()}
	for k, v := range base {
		domain[k] = v
	}
	return h.publish(ctx, src, EventObjectiveMastered, next.ObjectiveID, domain)
}
