package command

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/world"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// Progress event kinds of the consumer game.unlock_on_events.
const (
	ProgressQuestCompleted   = "quest_completed"
	ProgressChapterCompleted = "chapter_completed"
	ProgressLevelReached     = "level"
	ProgressAchievement      = "achievement_unlocked"
	ProgressEventJoined      = "event_participation"
	ProgressMastery          = "mastery"
)

// ProgressEvent is a fact about a student delivered by an event (never by the client, ADR-0016).
type ProgressEvent struct {
	Kind      string
	TenantID  uuid.UUID
	StudentID uuid.UUID
	Ref       string // quest_id, achievement_id, event id, objective_id
	Value     int    // level
	At        time.Time
}

// MasteryUpdate is learning_objective.progressed projected into ref.mastery_summary.
type MasteryUpdate struct {
	TenantID      uuid.UUID
	StudentID     uuid.UUID
	ObjectiveID   uuid.UUID
	Level         string
	Value         float64
	SourceVersion int64
}

// MasteryWriter upserts ref.mastery_summary by source_version (false when the event is older than the row).
type MasteryWriter interface {
	UpsertMastery(ctx context.Context, m MasteryUpdate) (bool, error)
}

// ApplyProgress records the fact of a progress event and re-evaluates the world of the student in the transaction of the
// consumer (TenantScope of the event): story chapters on quest.completed (F071-AC-21: first main quest → in_progress,
// last → completed with story.chapter_completed once) and the unlock rules of every gated area (F071-AC-14, AC-15: one
// unlock row, world.area_unlocked and the WS push the first time only). Replayed events change nothing.
func (h World) ApplyProgress(ctx context.Context, ev ProgressEvent) (err error) {
	ctx, done := observability.UseCase(ctx, "game.command.ApplyWorldProgress")
	defer func() { done(err) }()
	at := ev.At
	if at.IsZero() {
		at = h.now()
	}
	if err := h.Store.LockStudent(ctx, ev.StudentID); err != nil {
		return err
	}
	switch ev.Kind {
	case ProgressQuestCompleted, ProgressAchievement, ProgressEventJoined:
		if _, err := h.Store.RecordFact(ctx, ev.TenantID, ev.StudentID, ev.Kind, ev.Ref, 1, at); err != nil {
			return err
		}
	case ProgressLevelReached:
		if _, err := h.Store.RecordFact(ctx, ev.TenantID, ev.StudentID, ev.Kind, "level", ev.Value, at); err != nil {
			return err
		}
	}
	chapters, err := h.Content.CurrentChapters(ctx)
	if err != nil {
		return err
	}
	pr, err := h.Content.Progress(ctx, ev.TenantID, ev.StudentID, chapters)
	if err != nil {
		return err
	}
	if ev.Kind == ProgressQuestCompleted {
		changed, err := h.applyStory(ctx, ev, chapters, pr, at)
		if err != nil {
			return err
		}
		if changed {
			if pr, err = h.Content.Progress(ctx, ev.TenantID, ev.StudentID, chapters); err != nil {
				return err
			}
		}
	}
	return h.evaluateUnlocks(ctx, ev.TenantID, ev.StudentID, pr, at)
}

func (h World) applyStory(ctx context.Context, ev ProgressEvent, chapters []port.ChapterRow, pr *port.WorldProgress, at time.Time) (bool, error) {
	anyChanged := false
	for _, c := range chapters {
		next, changed, completed := c.Chapter.RecordQuest(pr.Story[c.Chapter.Code], ev.Ref)
		if !changed {
			continue
		}
		anyChanged = true
		var doneAt *time.Time
		if completed {
			doneAt = &at
		}
		if err := h.Store.SaveStory(ctx, ev.TenantID, ev.StudentID, c.Chapter.Code, c.VersionID, next, c.Chapter.MainQuestsDone(&next), doneAt, at); err != nil {
			return false, err
		}
		if !completed {
			continue
		}
		t := ev.TenantID
		if err := h.Events.Publish(ctx, write.Event{Type: EventChapterCompleted, SubjectType: "story_chapter", SubjectID: c.Chapter.Code,
			PartitionKey: ev.StudentID.String(), TenantID: &t, OccurredAt: at, Payload: map[string]any{"student_id": ev.StudentID,
				"chapter_code": c.Chapter.Code, "chapter_version_id": c.VersionID, "region_code": c.Chapter.RegionCode, "order": c.Chapter.Order,
				"completed_at": at}}); err != nil {
			return false, err
		}
	}
	return anyChanged, nil
}

// evaluateUnlocks records the unlock of every gated area whose rule the student now meets.
func (h World) evaluateUnlocks(ctx context.Context, tenant, student uuid.UUID, pr *port.WorldProgress, at time.Time) error {
	st, err := h.Content.Student(ctx, tenant, student)
	if err != nil || st == nil {
		return err // not projected yet: the facts are kept, the next event (or enterWorldArea) evaluates again
	}
	areas, err := h.Content.CurrentAreas(ctx)
	if err != nil {
		return err
	}
	for _, a := range areas {
		if a.Area.IsLearning() || a.Area.UnlockRule == nil {
			continue
		}
		if _, ok := pr.Unlocks[a.Area.AreaID]; ok {
			continue
		}
		d := world.Decide(a.Area, st.Domain(), false, pr.Facts)
		if !d.Visible || !d.RuleMet {
			continue
		}
		if err := h.unlock(ctx, tenant, student, a, world.UnlockedBy(d.Requirements), at); err != nil {
			return err
		}
	}
	return nil
}

// ApplyMastery projects learning_objective.progressed into ref.mastery_summary (F071 T011, read by objective_progress)
// and re-evaluates the unlock rules of the student when the row changed.
func (h World) ApplyMastery(ctx context.Context, w MasteryWriter, m MasteryUpdate) (err error) {
	ctx, done := observability.UseCase(ctx, "game.command.ProjectMastery")
	defer func() { done(err) }()
	changed, err := w.UpsertMastery(ctx, m)
	if err != nil || !changed {
		return err
	}
	return h.ApplyProgress(ctx, ProgressEvent{Kind: ProgressMastery, TenantID: m.TenantID, StudentID: m.StudentID, Ref: m.ObjectiveID.String()})
}
