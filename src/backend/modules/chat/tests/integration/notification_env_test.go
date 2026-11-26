//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/cache"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/modules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/persistence"
	chatrealtime "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/realtime"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/consumer"
	chathttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/natscmd"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/natsx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// learningFake is the learning facade seen by module chat in these tests: the generated learningapi.Fake (TEST DOUBLE —
// R17) answering listAssignmentRecipients from a map, or failing while down. The receiving side runs against the real
// learning database in modules/learning/tests/integration (TestAssignmentRecipientsFacade).
type learningFake struct {
	mu       sync.Mutex
	down     bool
	calls    int
	title    string
	students map[uuid.UUID][]uuid.UUID // assignment → recipients (already without the excused students)
}

func (l *learningFake) set(assignment uuid.UUID, students ...uuid.UUID) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.students[assignment] = students
}

func (l *learningFake) callCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.calls
}

func (l *learningFake) setDown(v bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.down = v
}

func (l *learningFake) fake() *learningapi.Fake {
	return &learningapi.Fake{ListAssignmentRecipientsFunc: func(ctx context.Context, r learningapi.ListAssignmentRecipientsRequestObject) (learningapi.ListAssignmentRecipientsResponseObject, error) {
		l.mu.Lock()
		defer l.mu.Unlock()
		l.calls++
		if l.down {
			return nil, context.DeadlineExceeded // TEST DOUBLE: module learning does not answer
		}
		ids, ok := l.students[r.AssignmentId]
		if !ok {
			return learningapi.ListAssignmentRecipients404ApplicationProblemPlusJSONResponse{}, nil
		}
		// One student per page: the adapter must follow next_cursor.
		start := 0
		if r.Params.Cursor != nil {
			for i, id := range ids {
				if id.String() == *r.Params.Cursor {
					start = i + 1
				}
			}
		}
		out := learningapi.ListAssignmentRecipients200JSONResponse{Title: l.title, StudentIds: []uuid.UUID{}}
		if start < len(ids) {
			out.StudentIds = ids[start : start+1]
			if start+1 < len(ids) {
				c := ids[start].String()
				out.Page = learningapi.CommonInternalCommonV1PageInfo{HasMore: true, NextCursor: &c}
			}
		}
		return out, nil
	}}
}

// notifyEnv is the notification context (F123) on the real adapters: PostgreSQL, Valkey, NATS JetStream with the event
// streams of chat, competition, learning and game, the chat.notify_* consumers, the inbox routes and the WS mark_read.
type notifyEnv struct {
	*chatEnv
	cmd      command.Notifications
	learning *learningFake
	unread   cache.Unread
}

type notifyOpts struct {
	maxDeliver int
	backoff    time.Duration
}

func setupNotify(t *testing.T, o notifyOpts) *notifyEnv {
	t.Helper()
	n := &notifyEnv{learning: &learningFake{title: "Ôn tập bảng nhân", students: map[uuid.UUID][]uuid.UUID{}}}
	n.chatEnv = setupChat(t, chatOpts{hook: func(c *chatEnv) {
		codec, err := page.NewCodec([]byte(strings.Repeat("n", 32)))
		if err != nil {
			t.Fatal(err)
		}
		n.unread = cache.Unread{C: c.vk, TTL: 7 * 24 * time.Hour}
		n.cmd = command.Notifications{UoW: c.uow, AfterCommit: entx.AfterCommit, Store: persistence.Notifications{},
			Push: chatrealtime.Notifications{P: rt.Publisher{C: c.vk}}, Unread: n.unread,
			Recipients: modules.Learning{F: n.learning.fake()}, Metrics: observability.Notifications{}, Now: c.clock.Now}
		c.notificationHandlers = &chathttp.NotificationHandlers{Commands: n.cmd,
			Queries: query.Notifications{Read: c.read, Reader: persistence.NotificationReader{}, Unread: n.unread, Cursors: codec}}
	}})
	ctx := context.Background()
	for _, m := range []string{"chat", "competition", "learning", "game"} {
		if err := natsx.EnsureStreams(ctx, n.js, m, natsx.StreamOptions{Replicas: 1}); err != nil {
			t.Fatal(err)
		}
	}
	cfg := consumer.NotifierConfig{MaxDeliver: o.maxDeliver}
	if o.backoff > 0 {
		cfg.BackOff = []time.Duration{o.backoff}
	}
	runCtx, stop := context.WithCancel(ctx)
	var wg sync.WaitGroup
	for _, name := range consumer.NotifierNames {
		c, err := outbox.NewConsumer(ctx, consumer.NotifierOptions(name, consumer.Notifier{H: n.cmd}, cfg, n.uow, n.app, n.js))
		if err != nil {
			t.Fatal(err)
		}
		wg.Add(1)
		go func() { defer wg.Done(); _ = c.Run(runCtx) }()
	}
	wg.Add(1)
	go func() { defer wg.Done(); _ = natscmd.ServeNotifications(runCtx, n.nc, n.cmd) }()
	t.Cleanup(func() { stop(); wg.Wait() })
	time.Sleep(150 * time.Millisecond) // subscriptions registered
	return n
}

// publish sends an event envelope of producer on its stream as the outbox relay does (a new Nats-Msg-Id each time: a
// redelivery after a relay crash is a new message carrying the same event_id).
func (n *notifyEnv) publish(producer, typ string, tenant uuid.UUID, eventID uuid.UUID, payload any) {
	n.t.Helper()
	b, err := json.Marshal(payload)
	if err != nil {
		n.t.Fatal(err)
	}
	at := n.clock.Now()
	tt := tenant
	env := outbox.Envelope{EventID: eventID, EventType: typ, SchemaVersion: 1, OccurredAt: at, RecordedAt: at, Producer: producer,
		TenantID: &tt, Payload: b}
	raw, _ := json.Marshal(env)
	if _, err := n.js.Publish(context.Background(), natsx.EventSubject(producer, typ), raw, jetstream.WithMsgID(uuid.NewString())); err != nil {
		n.t.Fatal(err)
	}
}

// notifications counts the notifications of recipient (superuser, across RLS).
func (n *notifyEnv) notifications(recipient uuid.UUID) int {
	return n.count(`SELECT count(*) FROM social.notifications WHERE recipient_id = $1`, recipient)
}

// waitNotifications waits until recipient has want notifications.
func (n *notifyEnv) waitNotifications(recipient uuid.UUID, want int) int {
	n.t.Helper()
	return n.waitCount(want, 10*time.Second, `SELECT count(*) FROM social.notifications WHERE recipient_id = $1`, recipient)
}

// processed reports whether consumer handled event (infra.processed_events).
func (n *notifyEnv) processed(name string, event uuid.UUID) bool {
	return n.count(`SELECT count(*) FROM infra.processed_events WHERE consumer = $1 AND event_id = $2`, name, event) == 1
}

// waitProcessed waits until consumer recorded event.
func (n *notifyEnv) waitProcessed(name string, event uuid.UUID) {
	n.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !n.processed(name, event) {
		if time.Now().After(deadline) {
			n.t.Fatalf("%s did not process %s", name, event)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
