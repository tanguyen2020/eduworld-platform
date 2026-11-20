package command

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/attemptview"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/videolesson"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/attempt"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/media"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Learning events of the media activities (EVENT_DESIGN §5.2, learning-event.schema.json).
const (
	EventVideoProgressed   = "video.progressed"
	EventVideoCueAnswered  = "video.cue_answered"
	EventActivityCompleted = "activity.completed"
)

// Response kinds of ActivityResponseSubmission handled by the engine.
const (
	KindQuestion     = "question"
	KindCueAnswer    = "cue_answer"
	KindBranchChoice = "branch_choice"
)

// VideoMetrics counts the media activities (eduworld_video_progress_trimmed_total, eduworld_video_seek_rejected_total,
// eduworld_video_completion_total{completed}); nil = not counted.
type VideoMetrics interface {
	ProgressTrimmed()
	SeekRejected()
	Completion(completed bool)
}

// timeline returns the video config of a version whose plugin is a Timeline (video_lesson); ok is false for the
// question-based types.
func timeline(p activity.Plugin, v activity.Version) (videolesson.Config, bool, error) {
	if _, ok := p.(activity.Timeline); !ok || v.Type != activity.VideoLesson {
		return videolesson.Config{}, false, nil
	}
	vc, err := videolesson.Parse(v.Config.Raw)
	if err != nil {
		return vc, true, problem.New(problem.ActivityConfigInvalid, "the published video config cannot be read")
	}
	return vc, true, nil
}

func (h Attempts) mediaAssets(ctx context.Context, vc videolesson.Config) (map[uuid.UUID]media.Asset, error) {
	if h.Media == nil {
		return map[uuid.UUID]media.Asset{}, nil
	}
	return h.Media.Assets(ctx, vc.AssetIDs())
}

// timelineStart returns the cue questions of a media version and checks its video is playable (F065-AC-02, AC-04:
// 422 VIDEO_NOT_READY without a ready Theora variant, no attempt created).
func (h Attempts) timelineStart(ctx context.Context, src ActivitySource) ([]uuid.UUID, bool, error) {
	vc, ok, err := timeline(src.Plugin, src.DomainVersion())
	if !ok || err != nil {
		return nil, ok, err
	}
	assets, err := h.mediaAssets(ctx, vc)
	if err != nil {
		return nil, true, err
	}
	if !attemptview.VideoReady(vc, assets) {
		return nil, true, problem.New(problem.VideoNotReady, "the video has no playable variant yet")
	}
	qids, err := src.Plugin.(activity.Timeline).CueQuestions(src.DomainVersion())
	return qids, true, err
}

// withTimeline adds the signed media, the progress and the resume position to the view of a media attempt.
func (h Attempts) withTimeline(ctx context.Context, a attempt.Attempt, src *ActivitySource, v dto.AttemptView) (dto.AttemptView, error) {
	if src == nil {
		return v, nil
	}
	vc, ok, err := timeline(src.Plugin, src.DomainVersion())
	if !ok || err != nil {
		return v, err
	}
	assets, err := h.mediaAssets(ctx, vc)
	if err != nil {
		return v, err
	}
	var p *videolesson.Progress
	if h.Progress != nil {
		if p, err = h.Progress.Progress(ctx, a.ID); err != nil {
			return v, err
		}
	}
	return attemptview.WithTimeline(v, vc, assets, h.Signer, p, h.now()), nil
}

// progressOf returns the locked progress row of an attempt, or a new one.
func (h Attempts) progressOf(ctx context.Context, a attempt.Attempt) (videolesson.Progress, error) {
	if h.Progress == nil {
		return videolesson.Progress{}, errors.New("command: no media progress store")
	}
	p, err := h.Progress.Progress(ctx, a.ID)
	if err != nil || p != nil {
		if p == nil {
			return videolesson.Progress{}, err
		}
		return *p, err
	}
	return videolesson.NewProgress(a.ID, a.TenantID, a.SchoolID, a.StartedAt), nil
}

// ProgressInput is ActivityProgressReport.
type ProgressInput struct {
	AttemptID     uuid.UUID
	ClientSeq     int64
	PositionMS    int64
	WatchedRanges []dto.WatchedRangeView
	PlaybackRate  *float64
}

func (in ProgressInput) ranges() []videolesson.Range {
	out := make([]videolesson.Range, len(in.WatchedRanges))
	for i, r := range in.WatchedRanges {
		out[i] = videolesson.Range{StartMS: r.StartMS, EndMS: r.EndMS}
	}
	return out
}

// Validate checks the bounds of the report (F065-AC-14: ≤ 64 ranges, playback_rate in [0.5, 2]).
func (in ProgressInput) Validate() error {
	if in.ClientSeq < 0 {
		return problem.Invalid("/client_seq", problem.SubInvalid, "must be ≥ 0")
	}
	if in.PositionMS < 0 {
		return problem.Invalid("/position_ms", problem.SubInvalid, "must be ≥ 0")
	}
	if len(in.WatchedRanges) > videolesson.MaxRanges {
		return problem.Invalid("/watched_ranges", problem.SubInvalid, "has more than 64 ranges")
	}
	for i, r := range in.WatchedRanges {
		if r.StartMS < 0 || r.EndMS < r.StartMS {
			return problem.Invalid("/watched_ranges/"+strconv.Itoa(i), problem.SubInvalid, "needs 0 ≤ start_ms ≤ end_ms")
		}
	}
	if r := in.PlaybackRate; r != nil && (*r < 0.5 || *r > 2) {
		return problem.Invalid("/playback_rate", problem.SubInvalid, "must be between 0.5 and 2")
	}
	return nil
}

// ReportProgress records a heartbeat of a media attempt (F065-AC-05…AC-07, research R2): an old client_seq returns the
// stored state; the new watched time is bounded by real time × the fastest playback rate (the excess is trimmed and
// recorded as an integrity signal); allow_seek refuses a position past the accepted frontier with 422
// VIDEO_SEEK_NOT_ALLOWED and nothing changes; video.progressed is written once per milestone. A graded attempt keeps
// receiving heartbeats (watching again) without changing its evaluation.
func (h Attempts) ReportProgress(ctx context.Context, in ProgressInput) (out dto.ProgressStateView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.ReportActivityProgress")
	defer func() { done(err) }()
	p, err := student(ctx)
	if err != nil {
		return out, err
	}
	if err := in.Validate(); err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		a, err := h.loadOwn(ctx, p, in.AttemptID)
		if err != nil {
			return err
		}
		plugin, v, _, err := h.versionOf(ctx, a)
		if err != nil {
			return err
		}
		vc, ok, err := timeline(plugin, v)
		if err != nil {
			return err
		}
		if !ok {
			return problem.Invalid("/", problem.SubInvalid, "progress is only reported for media activities")
		}
		now := h.now()
		finished := a.Status == attempt.StatusGraded || a.Status == attempt.StatusSubmitted
		if !finished {
			row := a.RowVersion
			if err := a.Resume(now, h.policy()); err != nil {
				return attemptProblem(err)
			}
			if err := h.Store.UpdateAttempt(ctx, a, row); err != nil {
				return err
			}
		}
		prog, err := h.progressOf(ctx, a)
		if err != nil {
			return err
		}
		if in.ClientSeq <= prog.ClientSeq && prog.Version > 0 {
			out = attemptview.ProgressState(vc, prog)
			return nil
		}
		assets, err := h.mediaAssets(ctx, vc)
		if err != nil {
			return err
		}
		duration := attemptview.DurationMS(vc, assets)
		acc := videolesson.AcceptRanges(prog.Ranges, in.ranges(), now.Sub(prog.LastHeartbeatAt).Milliseconds(), vc.MaxRate(), duration)
		if err := videolesson.CheckSeek(vc.Seek(), in.PositionMS, videolesson.Frontier(acc.Ranges), finished); err != nil {
			if h.MediaMetrics != nil {
				h.MediaMetrics.SeekRejected()
			}
			return problem.New(problem.VideoSeekNotAllowed, "the position is past the watched part")
		}
		if acc.TrimmedMS > 0 {
			if h.MediaMetrics != nil {
				h.MediaMetrics.ProgressTrimmed()
			}
			ev, _ := json.Marshal(map[string]any{"trimmed_ms": acc.TrimmedMS, "client_seq": in.ClientSeq,
				"elapsed_ms": now.Sub(prog.LastHeartbeatAt).Milliseconds(), "max_rate": vc.MaxRate()})
			if err := h.Progress.InsertIntegritySignal(ctx, write.IntegritySignal{ID: ids.New(), TenantID: a.TenantID, SchoolID: a.SchoolID,
				StudentID: a.StudentID, AttemptID: a.ID, SignalType: write.SignalVideoProgressImplausible, Score: float64(acc.TrimmedMS) / 1000,
				Evidence: ev, CreatedAt: now}); err != nil {
				return err
			}
		}
		prog.Ranges, prog.Ratio = acc.Ranges, videolesson.Ratio(acc.Ranges, duration)
		prog.PositionMS, prog.ClientSeq, prog.LastHeartbeatAt = in.PositionMS, in.ClientSeq, now
		if duration > 0 {
			prog.PositionMS = min(prog.PositionMS, duration)
		}
		var reached []float64
		reached, prog.Milestones = videolesson.NewMilestones(prog.Ratio, prog.Milestones)
		for _, m := range reached {
			pct := strconv.Itoa(int(m * 100))
			if err := h.learningEvent(ctx, a, EventVideoProgressed, "vp:"+a.ID.String()+":"+pct, now, a.ObjectiveIDs, "student",
				map[string]any{"attempt_id": a.ID, "watched_ratio": m}); err != nil {
				return err
			}
		}
		if err := h.Progress.SaveProgress(ctx, prog); err != nil {
			return err
		}
		out = attemptview.ProgressState(vc, prog)
		return nil
	})
	return out, err
}

// cueAnswered closes a cue after a graded answer (F065-AC-08, AC-09): processed when right, or after
// attempts_per_question answers when allow_skip_after_wrong; writes video.cue_answered.
func (h Attempts) cueAnswered(ctx context.Context, a attempt.Attempt, vc videolesson.Config, it attempt.Item, ans attempt.Answer, isCorrect,
	rapid bool, limit int) error {
	cue, ok := vc.CueAt(it.Position)
	if !ok {
		return nil
	}
	prog, err := h.progressOf(ctx, a)
	if err != nil {
		return err
	}
	if videolesson.CueProcessed(cue, isCorrect, it.AnswerCount, limit) && !contains(prog.ProcessedCues, cue.CueID) {
		prog.ProcessedCues = append(prog.ProcessedCues, cue.CueID)
		if err := h.Progress.SaveProgress(ctx, prog); err != nil {
			return err
		}
	}
	return h.learningEvent(ctx, a, EventVideoCueAnswered, "vc:"+a.ID.String()+":"+itoa(it.Position)+":"+itoa(ans.AttemptNo), h.now(), it.ObjectiveIDs,
		"student", map[string]any{"attempt_id": a.ID, "cue_id": cue.CueID, "question_version_id": it.QuestionVersionID, "is_correct": isCorrect,
			"hints_used": ans.HintsUsed, "rapid_guess": rapid})
}

// BranchInput is an ActivityResponseSubmission of kind branch_choice.
type BranchInput struct {
	AttemptID     uuid.UUID
	BranchPointID string
	OptionID      string
}

// Branch records the choice at a branch point of an interactive video (F065-AC-24, AC-25): one choice per branch
// point (the same option again is accepted, another one is 409 CONFLICT), never graded; the response names the next
// segment and reaching an ending segment makes the attempt completable.
func (h Attempts) Branch(ctx context.Context, in BranchInput) (out dto.ResponseResultView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.ChooseVideoBranch")
	defer func() { done(err) }()
	p, err := student(ctx)
	if err != nil {
		return out, err
	}
	if in.BranchPointID == "" || in.OptionID == "" {
		return out, problem.Invalid("/branch_point_id", problem.SubRequired, "branch_point_id and option_id are required")
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		a, err := h.loadOwn(ctx, p, in.AttemptID)
		if err != nil {
			return err
		}
		if err := a.CheckActive(h.now(), h.policy()); err != nil {
			return attemptProblem(err)
		}
		plugin, v, _, err := h.versionOf(ctx, a)
		if err != nil {
			return err
		}
		vc, ok, err := timeline(plugin, v)
		if err != nil {
			return err
		}
		b, found := vc.Branch(in.BranchPointID)
		if !ok || !found {
			return problem.Invalid("/branch_point_id", problem.SubInvalid, "is not a branch point of the video")
		}
		var opt *videolesson.Option
		for i := range b.Options {
			if b.Options[i].OptionID == in.OptionID {
				opt = &b.Options[i]
			}
		}
		if opt == nil {
			return problem.Invalid("/option_id", problem.SubInvalid, "is not an option of the branch point")
		}
		choices, err := h.Progress.BranchChoices(ctx, a.ID)
		if err != nil {
			return err
		}
		for _, c := range choices {
			if c.BranchPointID == in.BranchPointID {
				if c.OptionID != in.OptionID {
					return problem.New(problem.Conflict, "this branch point already has a choice")
				}
				out = dto.ResponseResultView{Accepted: true, NextSegmentID: c.TargetSegmentID}
				return idempotency.SetResult(ctx, out)
			}
		}
		now := h.now()
		if err := h.Progress.InsertBranchChoice(ctx, write.BranchChoice{ID: ids.New(), TenantID: a.TenantID, SchoolID: a.SchoolID, AttemptID: a.ID,
			BranchPointID: b.BranchPointID, OptionID: opt.OptionID, TargetSegmentID: opt.TargetSegmentID, ChosenAt: now}); err != nil {
			return err
		}
		prog, err := h.progressOf(ctx, a)
		if err != nil {
			return err
		}
		prog.SegmentID = opt.TargetSegmentID
		if s, ok := vc.Segment(opt.TargetSegmentID); ok && s.IsEnding {
			prog.ReachedEnding = true
		}
		if err := h.Progress.SaveProgress(ctx, prog); err != nil {
			return err
		}
		out = dto.ResponseResultView{Accepted: true, NextSegmentID: opt.TargetSegmentID}
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// timelineOutcome is the completion of a media attempt now (nil outcome for the question-based types).
func (h Attempts) timelineOutcome(ctx context.Context, a attempt.Attempt) (*videolesson.Outcome, error) {
	plugin, v, _, err := h.versionOf(ctx, a)
	if err != nil {
		return nil, err
	}
	vc, ok, err := timeline(plugin, v)
	if !ok || err != nil {
		return nil, err
	}
	prog, err := h.progressOf(ctx, a)
	if err != nil {
		return nil, err
	}
	o := videolesson.Evaluate(vc, prog.Ratio, prog.ProcessedCues, prog.ReachedEnding)
	return &o, nil
}

// completed writes activity.completed once for a completed media attempt (F065-AC-10).
func (h Attempts) completed(ctx context.Context, a attempt.Attempt, o *videolesson.Outcome) error {
	if o == nil {
		return nil
	}
	if h.MediaMetrics != nil {
		h.MediaMetrics.Completion(o.Completed)
	}
	if !o.Completed || a.GradedAt == nil {
		return nil
	}
	payload := map[string]any{"source_kind": sourceKindOf(a), "activity_type": a.ActivityType, "completion": true, "duration_ms": int64(0)}
	if a.DurationMS != nil {
		payload["duration_ms"] = *a.DurationMS
	}
	if a.Score != nil && a.MaxScore != nil {
		payload["score"] = learningScore(*a.Score, *a.MaxScore)
	}
	return h.learningEvent(ctx, a, EventActivityCompleted, "cmp:"+a.ID.String(), *a.GradedAt, a.ObjectiveIDs, "student", payload)
}
