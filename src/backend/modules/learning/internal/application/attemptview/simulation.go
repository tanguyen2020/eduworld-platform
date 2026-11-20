package attemptview

import (
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/attempt"
)

// WithSimulation adds the seed and runs_used of the attempt to the render of a simulation attempt (F066-AC-10): the
// seed is the one the server issued at start (seed_policy), never chosen by the client.
func WithSimulation(v dto.AttemptView, a attempt.Attempt, runsUsed int) dto.AttemptView {
	if v.Render == nil || v.Render.RenderPayload == nil || a.ActivityType != string(activity.Simulation) {
		return v
	}
	v.Render.RenderPayload["seed"] = max(a.Seed, 0)
	v.Render.RenderPayload["runs_used"] = runsUsed
	return v
}
