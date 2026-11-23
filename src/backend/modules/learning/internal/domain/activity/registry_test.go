package activity_test

import (
	"errors"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/plugins"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/plugins/quiz"
)

// F030-AC-20 (T036): the registry resolves only types of the release with a plugin (video_lesson since F065, audio_lesson
// and simulation since F066, minigame since F073-AC-14).
func TestRegistryResolve(t *testing.T) {
	r := plugins.Registry()
	for _, ok := range []string{"quiz", "interactive_exercise", "video_lesson", "audio_lesson", "simulation", "minigame"} {
		if p, err := r.Resolve(ok); err != nil || string(p.Info().Type) != ok {
			t.Fatalf("%s: %v", ok, err)
		}
	}
	for _, no := range []string{"speaking", "written_response", "project", ""} {
		if _, err := r.Resolve(no); !errors.Is(err, activity.ErrNotSupported) {
			t.Fatalf("%s: %v", no, err)
		}
	}
	if got := r.Missing([]string{"quiz", "minigame", "speaking", "speaking"}); len(got) != 1 || got[0] != "speaking" {
		t.Fatalf("missing: %v", got)
	}
	if r.MinClientVersion("quiz") == "" || r.MinClientVersion("minigame") == "" || r.MinClientVersion("speaking") != "" {
		t.Fatal("min client version")
	}
	if infos := r.Infos(); len(infos) != 6 || infos[0].Type != activity.AudioLesson || infos[1].Type != activity.InteractiveExercise ||
		infos[2].Type != activity.Minigame || infos[3].Type != activity.Quiz || infos[4].Type != activity.Simulation ||
		infos[5].Type != activity.VideoLesson {
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
