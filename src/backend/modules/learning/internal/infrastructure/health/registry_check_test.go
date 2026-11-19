package health

import (
	"context"
	"errors"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/plugins"
)

// F030-AC-23 (T042): readiness fails while published content of a type has no plugin (quickstart 18).
func TestRegistryCheck(t *testing.T) {
	reg := plugins.Registry()
	published := func(types ...string) PublishedTypes {
		return func(context.Context) ([]string, error) { return types, nil }
	}
	if err := RegistryCheck(reg, published("quiz", "interactive_exercise"))(context.Background()); err != nil {
		t.Fatal(err)
	}
	err := RegistryCheck(reg, published("quiz", "video_lesson"))(context.Background())
	if !errors.Is(err, ErrPluginMissing) || err.Error() != "activity registry: published content has no plugin: video_lesson" {
		t.Fatalf("missing plugin: %v", err)
	}
	failing := func(context.Context) ([]string, error) { return nil, errors.New("db down") }
	if err := RegistryCheck(reg, failing)(context.Background()); err == nil {
		t.Fatal("db error swallowed")
	}
}
