package module_test

import (
	"strings"
	"testing"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// F134: every declared consumer records its source module and event types, in every binary (no JetStream needed), so
// listProcessedEventIds can answer which consumers receive the events of a source.
func TestConsumerInfos(t *testing.T) {
	r := newRegistry(t, "core-api", "", nil)
	r.Consumer("game.quest_on_activity_graded", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return outbox.ConsumerOptions{Module: "game", Name: "game.quest_on_activity_graded", Stream: "EVT_LEARNING", JS: js,
			Subjects: []string{"ew.evt.learning.activity.graded", "ew.evt.learning.activity.graded", "ew.evt.learning.attempt.submitted"}}
	})
	r.Consumer("game.projection_identity", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return outbox.ConsumerOptions{Module: "game", Name: "game.projection_identity", Stream: "EVT_IDENTITY", Subjects: []string{"ew.evt.identity.>"}}
	})
	r.Consumer("game.needs_js", func(js jetstream.JetStream) outbox.ConsumerOptions {
		_ = js.Conn() // a factory that dereferences JetStream: recorded without source, never a panic
		return outbox.ConsumerOptions{}
	})
	r.Consumer("chat.other", func(jetstream.JetStream) outbox.ConsumerOptions {
		return outbox.ConsumerOptions{Module: "chat", Name: "chat.other", Stream: "EVT_GAME", Subjects: []string{"ew.evt.game.xp.awarded"}}
	})
	got := r.ConsumerInfos("game")
	if len(got) != 3 {
		t.Fatalf("game consumers %+v", got)
	}
	want := map[string]string{"game.needs_js": "|", "game.projection_identity": "identity|*",
		"game.quest_on_activity_graded": "learning|activity.graded,attempt.submitted"}
	for _, c := range got {
		if s := c.Source + "|" + strings.Join(c.EventTypes, ","); s != want[c.Name] {
			t.Errorf("%s = %s, want %s", c.Name, s, want[c.Name])
		}
	}
	if all := r.ConsumerInfos(""); len(all) != 4 {
		t.Fatalf("all consumers %d", len(all))
	}
}
