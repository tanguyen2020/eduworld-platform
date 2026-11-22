//go:build integration

package integration

import (
	"context"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/service"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/cache"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/emoji"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/natsstream"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/persistence"
	chatrealtime "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/realtime"
	chathttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/natscmd"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/persister"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// featureEnv is the chat core plus the F118 message features on the real adapters: the Messaging use cases behind REST
// and the NATS commands, the receipts (read + delivered), the feature queries and the change consumer persister-evt.
type featureEnv struct {
	*chatEnv
	messaging command.Messaging
	receipts  command.Receipts
	features  query.Features
	apply     command.ApplyChange
	changes   *persister.ChangesWorker
}

// setupFeatures wires the F118 use cases on top of setupChat.
func setupFeatures(t *testing.T, o chatOpts) *featureEnv {
	t.Helper()
	f := &featureEnv{}
	hook := o.hook
	o.hook = func(c *chatEnv) {
		f.chatEnv = c
		wireFeatures(t, f)
		if hook != nil {
			hook(c)
		}
	}
	c := setupChat(t, o)
	f.chatEnv = c
	return f
}

// wireFeatures builds the F118 use cases (called from the hook of setupChat, before the HTTP server starts).
func wireFeatures(t *testing.T, f *featureEnv) {
	t.Helper()
	c := f.chatEnv
	set, err := emoji.Load()
	if err != nil {
		t.Fatal(err)
	}
	state := cache.FeatureState{C: c.vk}
	fanout := chatrealtime.ChatFanout{P: rt.Publisher{C: c.vk}, Now: c.clock.Now}
	lookup := service.Lookup{Read: c.read, Reader: persistence.Features{}, Recent: c.send.Recent, Index: state}
	c.send.Lookup, c.send.Assets, c.send.Emoji = &lookup, service.Assets{Read: c.read, Reader: persistence.Features{}}, set
	c.send.AssetRate, c.send.Index, c.send.Notify = state, state, fanout
	c.send.Config.StickerGifPerMinute = 4
	f.messaging = command.Messaging{Send: c.send, Lookup: lookup, Features: persistence.Features{},
		Changes: natsstream.Publisher{JS: c.js, Timeout: 2 * time.Second}, Gate: state, Reactions: state, Pins: state, Typing: state,
		Fanout: fanout, Emoji: set, Now: c.clock.Now}
	f.receipts = command.Receipts{UoW: c.uow, Facts: c.facts, Repo: persistence.Gameplay{}, Buffer: cache.Cursor{C: c.vk},
		Recent: c.send.Recent, Push: fanout, Cursors: persistence.Gameplay{}, Delivered: cache.Cursor{C: c.vk, Kind: "delivered:"},
		Mentions: fanout, Fanout: fanout, Now: c.clock.Now}
	f.features = query.Features{Read: c.read, Reader: persistence.Features{}, Facts: c.facts, Lookup: lookup, Recent: c.send.Recent,
		Now: c.clock.Now}
	f.apply = command.ApplyChange{UoW: c.uow, Store: persistence.Changes{}, Events: persistence.Publisher{W: c.writer}}
}

// startFeatures mounts the F118 routes and commands and starts persister-evt (the GREEN step of the F118 tests).
func startFeatures(t *testing.T, f *featureEnv) {
	t.Helper()
	c := f.chatEnv
	c.reads.Decor = &f.features
	c.chatHandlers.Send, c.chatHandlers.Queries = c.send, c.reads
	c.featureHandlers = &chathttp.FeatureHandlers{Commands: f.messaging, Queries: f.features,
		AssetURL: func(k string) string { return "https://cdn.test.invalid/" + k }, Recorder: c.send.RecordResult}
	c.server(context.Background())
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan struct{}, 3)
	go func() { _ = natscmd.ServeFeatures(ctx, c.nc, f.messaging, f.receipts); done <- struct{}{} }()
	go func() { _ = natscmd.ServeReceipts(ctx, c.nc, f.receipts); done <- struct{}{} }()
	w, err := persister.NewChanges(ctx, persister.Options{JS: c.js, DB: c.app, Batch: 100, Wait: 20 * time.Millisecond, MaxDeliver: 20,
		AckWait: 5 * time.Second, Backoff: 100 * time.Millisecond}, f.apply)
	if err != nil {
		t.Fatal(err)
	}
	f.changes = w
	go func() { _ = w.Run(ctx); done <- struct{}{} }()
	t.Cleanup(func() { stop(); <-done; <-done; <-done })
	time.Sleep(150 * time.Millisecond)
}

// sendText posts a text through REST and returns the message id.
func (f *featureEnv) sendText(s student, ch uuid.UUID, body string, extra map[string]any) uuid.UUID {
	f.t.Helper()
	b := map[string]any{"client_message_id": uuid.New(), "kind": "text", "body": body}
	for k, v := range extra {
		b[k] = v
	}
	r := f.sendREST(s, ch, b)
	if r.status != http.StatusCreated {
		f.t.Fatalf("send: %d %s", r.status, r.raw)
	}
	id, err := uuid.Parse(r.body["message_id"].(string))
	if err != nil {
		f.t.Fatal(err)
	}
	return id
}

// edit is editMessage with If-Match revision.
func (f *featureEnv) edit(s student, id uuid.UUID, rev int, body string) resp {
	f.t.Helper()
	return f.do(http.MethodPatch, "/chat/messages/"+id.String(), s.token, map[string]string{"X-Tenant-Id": s.tenant.String(),
		"If-Match": `"` + strconv.Itoa(rev) + `"`}, map[string]any{"body": body})
}

func (f *featureEnv) recall(s student, id uuid.UUID) resp {
	f.t.Helper()
	return f.call(s, http.MethodPost, "/chat/messages/"+id.String()+":recall", "", nil)
}

// persisted waits until the message has been written by chat-persister.
func (f *featureEnv) persisted(id uuid.UUID) {
	f.t.Helper()
	if n := f.waitCount(1, 5*time.Second, `SELECT count(*) FROM (SELECT message_id FROM chat.messages WHERE message_id = $1
		UNION ALL SELECT message_id FROM chat.gameplay_messages WHERE message_id = $1) x`, id); n != 1 {
		f.t.Fatalf("message %s not persisted", id)
	}
}

// party opens a party channel of the students with the first as leader.
func (f *featureEnv) party(members ...student) uuid.UUID {
	f.t.Helper()
	ch := f.open(channel.Party, members...)
	f.exec(`UPDATE chat.channel_members SET role = 'leader' WHERE channel_id = $1 AND user_id = $2`, ch, members[0].id)
	_ = f.facts.InvalidateChannel(context.Background(), members[0].tenant, ch)
	return ch
}
