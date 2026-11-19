// Package quiz is the plugin of activity_type quiz (ACTIVITY_ENGINE §3: single/multiple choice, true/false, image-based
// and timed quizzes; strategy auto). Questions come from the question bank (quiz.schema.json).
package quiz

import "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"

// SchemaURL is the $id of specs/schemas/activities/quiz.schema.json.
const SchemaURL = "https://schemas.eduworld.vn/content/activities/quiz.schema.json"

// MinClientVersion is the oldest game client with the quiz renderer (F063 takes it over with the renderer registry).
const MinClientVersion = "0.1.0"

// New returns the quiz plugin.
func New() activity.Plugin {
	return activity.QuestionBased{
		Meta: activity.Info{Type: activity.Quiz, Label: "Quiz", SchemaURL: SchemaURL, SchemaVersion: 1, Strategies: []string{"auto"},
			MinClientVersion: MinClientVersion, Status: activity.StatusEnabled},
		Types: []string{"single_choice", "multiple_choice", "true_false", "image_based"},
	}
}
