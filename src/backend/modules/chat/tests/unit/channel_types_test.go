package unit

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
)

// ADR-0095 (F115-AC-08, CHAT_SYSTEM §2): no class or study_group channel and no grade channel — only world, area,
// match, party, team and direct; opening a class/study_group channel is refused.
func TestChannelTypesWithoutClass(t *testing.T) {
	want := []channel.Type{channel.World, channel.Area, channel.Match, channel.Party, channel.Team, channel.Direct}
	if len(channel.Types) != len(want) {
		t.Fatalf("channel types %v", channel.Types)
	}
	for i, ty := range want {
		if channel.Types[i] != ty {
			t.Fatalf("channel types %v", channel.Types)
		}
	}
	school := uuid.New()
	for _, ty := range []channel.Type{"class", "study_group", "grade"} {
		if ty.Valid() {
			t.Fatalf("%s still a channel type", ty)
		}
		_, err := channel.New(uuid.New(), uuid.New(), &school, ty, map[string]string{"class_id": uuid.NewString()}, time.Now())
		if !errors.Is(err, channel.ErrType) {
			t.Fatalf("%s channel opened: %v", ty, err)
		}
	}
}
