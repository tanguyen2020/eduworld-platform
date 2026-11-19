// Package plugins assembles the activity plugins of the release (F030: quiz, interactive_exercise; video, audio,
// simulation and minigame plugins come with F065, F066, F073).
package plugins

import (
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/plugins/interactive"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/plugins/quiz"
)

// All returns the plugins of the release.
func All() []activity.Plugin { return []activity.Plugin{quiz.New(), interactive.New()} }

// Registry returns the registry of the release; it never fails for the plugins above.
func Registry() *activity.Registry {
	r, err := activity.NewRegistry(All()...)
	if err != nil {
		panic(err)
	}
	return r
}
