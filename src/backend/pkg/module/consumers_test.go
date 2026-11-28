package module_test

import (
	"context"
	"strings"
	"testing"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
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

// F003 Clarifications 2026-11-28: `worker projection rebuild` gets the handler and unit of work of a declared consumer; a
// consumer without them, one whose factory needs JetStream or an unknown name gives ok = false (never a panic).
func TestConsumerOptions(t *testing.T) {
	r := newRegistry(t, "worker", "projection", nil)
	uow := &entx.UnitOfWork{}
	r.Consumer("learning.projection_identity", func(jetstream.JetStream) outbox.ConsumerOptions {
		return outbox.ConsumerOptions{Module: "learning", Name: "learning.projection_identity", Stream: "EVT_IDENTITY", UoW: uow,
			Subjects: []string{"ew.evt.identity.tenant.updated"}, Handler: func(context.Context, outbox.Envelope) error { return nil }}
	})
	r.Consumer("learning.no_handler", func(jetstream.JetStream) outbox.ConsumerOptions {
		return outbox.ConsumerOptions{Module: "learning", Name: "learning.no_handler", Stream: "EVT_IDENTITY"}
	})
	r.Consumer("learning.needs_js", func(js jetstream.JetStream) outbox.ConsumerOptions {
		_ = js.Conn()
		return outbox.ConsumerOptions{}
	})
	if o, ok := r.ConsumerOptions("learning.projection_identity"); !ok || o.UoW != uow || o.Handler == nil {
		t.Fatalf("declared consumer: %v %+v", ok, o)
	}
	for _, name := range []string{"learning.no_handler", "learning.needs_js", "learning.unknown"} {
		if _, ok := r.ConsumerOptions(name); ok {
			t.Errorf("%s: ok", name)
		}
	}
	if !r.Reconciles() {
		t.Fatal("profile projection is a one-off composition")
	}
}
