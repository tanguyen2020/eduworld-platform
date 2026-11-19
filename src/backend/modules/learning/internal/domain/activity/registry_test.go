package activity_test

import (
	"errors"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/plugins"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/plugins/quiz"
)

// F030-AC-20 (T036): the registry resolves only types of the release with a plugin.
func TestRegistryResolve(t *testing.T) {
	r := plugins.Registry()
	for _, ok := range []string{"quiz", "interactive_exercise"} {
		if p, err := r.Resolve(ok); err != nil || string(p.Info().Type) != ok {
			t.Fatalf("%s: %v", ok, err)
		}
	}
	for _, no := range []string{"video_lesson", "audio_lesson", "simulation", "minigame", "speaking", "written_response", "project", ""} {
		if _, err := r.Resolve(no); !errors.Is(err, activity.ErrNotSupported) {
			t.Fatalf("%s: %v", no, err)
		}
	}
	if got := r.Missing([]string{"quiz", "video_lesson", "video_lesson"}); len(got) != 1 || got[0] != "video_lesson" {
		t.Fatalf("missing: %v", got)
	}
	if r.MinClientVersion("quiz") == "" || r.MinClientVersion("simulation") != "" {
		t.Fatal("min client version")
	}
	if infos := r.Infos(); len(infos) != 2 || infos[0].Type != activity.InteractiveExercise || infos[1].Type != activity.Quiz {
		t.Fatalf("infos: %v", infos)
	}
}

func TestRegistryRefusesDuplicatesAndOutOfRelease(t *testing.T) {
	if _, err := activity.NewRegistry(quiz.New(), quiz.New()); err == nil {
		t.Fatal("duplicate plugin accepted")
	}
	speaking := activity.QuestionBased{Meta: activity.Info{Type: "speaking"}}
	if _, err := activity.NewRegistry(speaking); err == nil {
		t.Fatal("plugin outside the release accepted")
	}
	disabled := activity.QuestionBased{Meta: activity.Info{Type: activity.Minigame, Status: activity.StatusDisabled}}
	r, err := activity.NewRegistry(disabled)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Resolve("minigame"); !errors.Is(err, activity.ErrNotSupported) {
		t.Fatal("disabled plugin resolved")
	}
}
