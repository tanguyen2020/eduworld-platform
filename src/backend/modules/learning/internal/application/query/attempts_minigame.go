package query

import (
	"context"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/attemptview"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// minigame pins the configuration version of a mini-game attempt on its source (F073-AC-17): the projected version
// alone for a play of module game, on the activity version for a minigame learning activity; with the grade of the
// student for the display configuration (render_payload {minigame_type, minigame_config_version_id, config}).
func (q Attempts) minigame(ctx context.Context, out *loaded) error {
	a := out.a
	if a.MinigameConfigVersionID == nil && a.SourceKind != attemptview.SourceKindMinigameConfig {
		return nil
	}
	if q.Minigames == nil {
		return problem.New(problem.Internal, "the mini-game projection is not wired")
	}
	pinned := a.SourceVersionID
	if a.MinigameConfigVersionID != nil {
		pinned = *a.MinigameConfigVersionID
	}
	row, err := q.Minigames.MinigameConfigVersion(ctx, pinned)
	if err != nil {
		return err
	}
	if row == nil {
		return problem.New(problem.Conflict, "the mini-game configuration version is not projected yet; retry shortly").Retry()
	}
	var grade *int
	if q.Students != nil {
		if _, grade, err = q.Students.StudentSchool(ctx, a.StudentID); err != nil {
			return err
		}
	}
	if out.src != nil {
		attemptview.Pin(out.src, *row, grade)
		return nil
	}
	p, err := q.Registry.Resolve(string(activity.Minigame))
	if err != nil {
		return problem.New(problem.ActivityTypeNotSupported, "")
	}
	src, err := attemptview.MinigameSource(*row, p, grade)
	if err != nil {
		return problem.New(problem.ActivityConfigInvalid, "the pinned mini-game configuration cannot be read")
	}
	out.src = &src
	return nil
}
