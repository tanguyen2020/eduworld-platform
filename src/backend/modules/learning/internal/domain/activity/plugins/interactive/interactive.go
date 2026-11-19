// Package interactive is the plugin of activity_type interactive_exercise (ACTIVITY_ENGINE §3: fill in the blank, short
// answer from a closed set, drag and drop, matching, ordering, numeric input, interactive widgets; strategies auto,
// accepted_answers, rule, numeric_tolerance). Questions come from the question bank (interactive-exercise.schema.json).
package interactive

import "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"

// SchemaURL is the $id of specs/schemas/activities/interactive-exercise.schema.json.
const SchemaURL = "https://schemas.eduworld.vn/content/activities/interactive-exercise.schema.json"

// MinClientVersion is the oldest game client with the interactive renderers (F063 takes it over).
const MinClientVersion = "0.1.0"

// New returns the interactive_exercise plugin.
func New() activity.Plugin {
	return activity.QuestionBased{
		Meta: activity.Info{Type: activity.InteractiveExercise, Label: "Interactive exercise", SchemaURL: SchemaURL, SchemaVersion: 1,
			Strategies: []string{"auto", "accepted_answers", "rule", "numeric_tolerance"}, MinClientVersion: MinClientVersion,
			Status: activity.StatusEnabled},
		Types: []string{"fill_in_blank", "short_answer", "numeric", "matching", "ordering", "drag_drop", "image_based", "interactive"},
	}
}
