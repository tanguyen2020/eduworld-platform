package attemptview

import (
	"encoding/json"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/plugins/minigame"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

// SourceKindMinigameConfig is the source_kind of an attempt started by module game on a mini-game configuration
// (createAttempt, content_type minigame_config, F073): source_version_id is the pinned MiniGameConfigVersion.
const SourceKindMinigameConfig = "minigame_config"

// Pin pins a projected MiniGameConfigVersion on the source of an attempt and records the grade of its student.
func Pin(src *Source, row dto.MinigameConfigVersion, grade *int) {
	src.Config.Pinned = &activity.Pinned{VersionID: row.ID, Snapshot: row.Snapshot}
	src.Grade = grade
}

// MinigameSource is the source of an attempt on a mini-game configuration without learning activity (F073): the pinned
// version plays the role of the activity version — title = name, instructions, time limit and feedback policy of the
// configuration, activity_type minigame — so that the plugin, the render and the views are those of every attempt.
func MinigameSource(row dto.MinigameConfigVersion, p activity.Plugin, grade *int) (Source, error) {
	rules, err := minigame.Parse(row.Snapshot)
	if err != nil {
		return Source{}, err
	}
	name, _ := json.Marshal(rules.Name)
	var instructions json.RawMessage
	if len(rules.Instructions) > 0 {
		instructions, _ = json.Marshal(rules.Instructions)
	}
	fp := rules.FeedbackPolicy
	src := Source{Doc: dto.Doc{Type: contentmodel.TypeActivity, ID: row.ConfigID, CurrentVersionID: &row.ID, LatestVersionNo: row.Version},
		Version: dto.DocVersion{Type: contentmodel.TypeActivity, ID: row.ID, ItemID: row.ConfigID, VersionNo: row.Version,
			Status: contentmodel.StatusPublished, Title: name, GradeLevels: rules.GradeLevels, ContentLanguage: "vi", PublishedAt: &row.PublishedAt},
		Body: ActivityBody{ActivityType: string(activity.Minigame), Instructions: instructions, TimeLimitSeconds: rules.TimeLimitSeconds},
		Config: activity.Config{Evaluation: activity.Evaluation{Strategies: []string{"auto"}},
			Feedback: activity.Feedback{Timing: fp.Timing, ShowExplanation: fp.ShowExplanation, ShowCorrectAnswer: fp.ShowCorrectAnswer}},
		Plugin: p}
	Pin(&src, row, grade)
	return src, nil
}

// Minigame returns the rules of a source with a pinned configuration (false otherwise).
func Minigame(src *Source) (minigame.Rules, bool) {
	if src == nil {
		return minigame.Rules{}, false
	}
	return minigame.RulesOf(src.Config)
}
