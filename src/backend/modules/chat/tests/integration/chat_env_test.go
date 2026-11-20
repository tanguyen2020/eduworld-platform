//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/testcontainers/testcontainers-go"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/service"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/policy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/cache"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/natsstream"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/persistence"
	chatrealtime "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/realtime"
	chathttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/natscmd"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/persister"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// switchValidator is a TEST DOUBLE (R17) of the ContentValidator port (F116 implements the real pipeline): it records
// every call and answers the configured decision; it never matches any word (no sensitive term in tests, R15).
type switchValidator struct {
	mu       sync.Mutex
	decision message.Decision
	display  string
	calls    []port.ValidationInput
}

func (v *switchValidator) set(d message.Decision, display string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.decision, v.display = d, display
}

func (v *switchValidator) Validate(_ context.Context, in port.ValidationInput) (port.Verdict, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.calls = append(v.calls, in)
	d := v.decision
	if d == "" {
		d = message.Allow
	}
	return port.Verdict{Decision: d, Display: v.display}, nil
}

func (v *switchValidator) count() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return len(v.calls)
}

// outage is a TEST DOUBLE switch simulating PostgreSQL down for the chat path: the read port of the send path, the
// message store and the health check of chat-persister fail while it is on (the database itself stays up for the
// assertions).
type outage struct {
	mu sync.Mutex
	on bool
}

var errPGDown = errors.New("TEST DOUBLE: PostgreSQL unavailable")

func (o *outage) set(on bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.on = on
}

func (o *outage) down() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.on
}

type outageReader struct {
	read.ChatReader
	o *outage
}

func (r outageReader) ChannelFacts(ctx context.Context, tenant, id uuid.UUID) (*read.ChannelFacts, error) {
	if r.o.down() {
		return nil, errPGDown
	}
	return r.ChatReader.ChannelFacts(ctx, tenant, id)
}

func (r outageReader) PolicyLevels(ctx context.Context, tenant uuid.UUID, school, class *uuid.UUID) (policy.Levels, error) {
	if r.o.down() {
		return policy.Levels{}, errPGDown
	}
	return r.ChatReader.PolicyLevels(ctx, tenant, school, class)
}

func (r outageReader) CurfewWindows(ctx context.Context, tenant uuid.UUID, school *uuid.UUID) (*read.CurfewWindow, *read.CurfewWindow, error) {
	if r.o.down() {
		return nil, nil, errPGDown
	}
	return r.ChatReader.CurfewWindows(ctx, tenant, school)
}

func (r outageReader) Messages(ctx context.Context, q read.MessageQuery) ([]read.StoredMessage, error) {
	if r.o.down() {
		return nil, errPGDown
	}
	return r.ChatReader.Messages(ctx, q)
}

type outageStore struct {
	write.MessageStore
	o *outage
}

func (s outageStore) Insert(ctx context.Context, tenant uuid.UUID, es []message.Envelope) (write.PersistResult, error) {
	if s.o.down() {
		return write.PersistResult{}, errPGDown
	}
	return s.MessageStore.Insert(ctx, tenant, es)
}

type chatOpts struct {
	noPersister bool
	maxDeliver  int
	factsFresh  time.Duration
	store       write.MessageStore // nil: persistence.Messages
}

// chatEnv is the chat core on the real adapters: PostgreSQL (eduworld_chat), Valkey and NATS JetStream
// (testcontainers), the send use case behind REST and the NATS commands of the gateway, chat-persister.
type chatEnv struct {
	*env
	vk        valkey.Client
	nc        *nats.Conn
	js        jetstream.JetStream
	natsC     testcontainers.Container
	validator *switchValidator
	outage    *outage
	send      command.Send
	reads     query.Chat
	channels  command.Channels
	persist   command.Persist
	facts     service.Facts
	worker    *persister.Worker
}

func setupChat(t *testing.T, o chatOpts) *chatEnv {
	t.Helper()
	ctx := context.Background()
	_, vkAddr := testkit.StartValkey(ctx, t)
	vk, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{vkAddr}, DisableCache: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(vk.Close)
	natsC, url := testkit.StartNATS(ctx, t)
	nc, err := nats.Connect(url, nats.MaxReconnects(-1), nats.ReconnectWait(100*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	if err := natsstream.EnsureStream(ctx, js, natsstream.StreamOptions{MaxAge: 72 * time.Hour}); err != nil {
		t.Fatal(err)
	}
	e := setup(t, envOpts{})
	c := &chatEnv{env: e, vk: vk, nc: nc, js: js, natsC: natsC, validator: &switchValidator{}, outage: &outage{}}
	fresh := o.factsFresh
	if fresh == 0 {
		fresh = time.Minute
	}
	curfew, _ := policy.ParseWindow("21:30-06:30", ict)
	recent := cache.Recent{C: vk, Size: 200, TTL: 10 * time.Minute}
	c.facts = service.Facts{Read: e.read, Reader: outageReader{ChatReader: persistence.ChatReader{}, o: c.outage}, Recent: recent,
		Cache:  cache.Facts{C: vk, StaleTTL: time.Hour, OnStale: observability.StaleFacts},
		Config: service.FactsConfig{FactsFresh: fresh, PolicyCache: fresh, CurfewFresh: fresh, PlatformCurfew: curfew, CurfewLocation: ict}}
	fanout := chatrealtime.ChatFanout{P: rt.Publisher{C: vk}, Now: e.clock.Now}
	c.send = command.Send{Facts: c.facts, Validator: c.validator, Seq: cache.Seq{C: vk}, Recent: recent, Rate: cache.Rate{C: vk},
		Stream: natsstream.Publisher{JS: js, Timeout: 2 * time.Second}, Fanout: fanout, Dedupe: cache.Dedupe{C: vk, TTL: 10 * time.Minute},
		Metrics: observability.Chat{}, UoW: e.uow, Audit: persistence.Auditor{R: e.rec}, Config: command.SendConfig{RatePerUser: 30},
		Now: e.clock.Now}
	c.reads = query.Chat{Read: e.read, Reader: outageReader{ChatReader: persistence.ChatReader{}, o: c.outage}, Facts: c.facts,
		Recent: recent, Fanout: fanout, Config: query.ChatConfig{RecentBuffer: 200}}
	c.channels = command.Channels{UoW: e.uow, Repo: persistence.Channels{}, Audit: persistence.Auditor{R: e.rec}, Facts: c.facts,
		AfterCommit: func(ctx context.Context, fn func(ctx context.Context)) error { fn(ctx); return nil }, Now: e.clock.Now}
	store := o.store
	if store == nil {
		store = persistence.Messages{}
	}
	c.persist = command.Persist{UoW: e.uow, Store: outageStore{MessageStore: store, o: c.outage},
		Events: persistence.Publisher{W: e.writer}}
	e.chatHandlers = &chathttp.ChatHandlers{Send: c.send, Queries: c.reads}
	e.server(ctx)
	runCtx, stop := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { defer close(done); _ = natscmd.Serve(runCtx, nc, c.send, c.reads) }()
	maxDeliver := o.maxDeliver
	if maxDeliver == 0 {
		maxDeliver = 5
	}
	c.worker, err = persister.New(ctx, persister.Options{JS: js, Persist: c.persist, DB: e.app, Batch: 500, Wait: 20 * time.Millisecond,
		MaxDeliver: maxDeliver, AckWait: 5 * time.Second, Backoff: 100 * time.Millisecond,
		Stream: natsstream.StreamOptions{MaxAge: 72 * time.Hour},
		Health: func(ctx context.Context) error {
			if c.outage.down() {
				return errPGDown
			}
			return e.app.PingContext(ctx)
		}})
	if err != nil {
		t.Fatal(err)
	}
	wdone := make(chan struct{})
	if o.noPersister {
		close(wdone)
	} else {
		go func() { defer close(wdone); _ = c.worker.Run(runCtx) }()
	}
	t.Cleanup(func() { stop(); <-done; <-wdone })
	time.Sleep(150 * time.Millisecond) // command subscriptions registered
	return c
}

// open opens a channel of type t in the tenant/school of the first member with the members.
func (c *chatEnv) open(t channel.Type, members ...student) uuid.UUID {
	c.t.Helper()
	id := uuid.Must(uuid.NewV7())
	school := members[0].school
	scope := map[string]string{"party_id": id.String()[:8]}
	switch t {
	case channel.Class:
		scope = map[string]string{"class_id": uuid.NewString()}
	case channel.Match:
		scope = map[string]string{"match_id": id.String()[:8]}
	case channel.Direct:
		scope = map[string]string{"pair": id.String()[:8]}
	}
	ids := make([]uuid.UUID, len(members))
	for i, m := range members {
		ids[i] = m.id
	}
	if _, err := c.channels.OpenChannel(context.Background(), command.OpenChannelInput{ID: id, TenantID: members[0].tenant, SchoolID: &school,
		Type: t, ScopeRef: scope, Members: ids}); err != nil {
		c.t.Fatal(err)
	}
	return id
}

// caller is the gateway identity of a student's connection (Ew-* headers).
func caller(s student, conn string) rt.Caller {
	return rt.Caller{TenantID: s.tenant, UserID: s.id, GameSessionID: uuid.New(), DeviceID: uuid.New(), ConnID: conn, NodeID: "gw-test"}
}

// ws sends a command as the gateway does (NATS request ew.cmd.chat.<type> with the Ew-* headers) and returns the reply.
func (c *chatEnv) ws(s student, conn, typ string, payload any) rt.Reply {
	c.t.Helper()
	b, _ := json.Marshal(payload)
	m := nats.NewMsg(rt.CommandSubject("chat", typ))
	m.Header = caller(s, conn).Header(context.Background())
	m.Data = b
	res, err := c.nc.RequestMsg(m, 5*time.Second)
	if err != nil {
		c.t.Fatalf("%s: %v", typ, err)
	}
	var r rt.Reply
	if err := json.Unmarshal(res.Data, &r); err != nil {
		c.t.Fatal(err)
	}
	return r
}

func replyCode(r rt.Reply) string {
	if r.Problem == nil {
		return ""
	}
	return r.Problem.Code
}

// sendWS sends a text over WS and returns the reply.
func (c *chatEnv) sendWS(s student, conn string, ch uuid.UUID, body string) rt.Reply {
	return c.ws(s, conn, "chat.message.send", map[string]any{"channel_id": ch, "client_message_id": uuid.New(), "kind": "text", "body": body})
}

// sendREST posts a text through sendChannelMessage with a new Idempotency-Key.
func (c *chatEnv) sendREST(s student, ch uuid.UUID, body map[string]any) resp {
	return c.call(s, http.MethodPost, "/chat/channels/"+ch.String()+"/messages", newKey(), body)
}

// channelInbox subscribes ew:chat:fanout:<channel_id> and returns the envelopes a gateway node would receive.
func channelInbox(t *testing.T, vk valkey.Client, ch uuid.UUID) <-chan rt.Envelope {
	t.Helper()
	out := make(chan rt.Envelope, 1024)
	dc, release := vk.Dedicate()
	wait := dc.SetPubSubHooks(valkey.PubSubHooks{OnMessage: func(m valkey.PubSubMessage) {
		var env rt.Envelope
		if json.Unmarshal([]byte(m.Message), &env) == nil {
			out <- env
		}
	}})
	if err := dc.Do(context.Background(), dc.B().Subscribe().Channel(rt.ChatFanoutChannel(ch.String())).Build()).Error(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { release(); <-wait })
	return out
}

// next returns the next envelope of type typ within d (other types are skipped), nil when none.
func next(ch <-chan rt.Envelope, typ string, d time.Duration) *rt.Envelope {
	deadline := time.After(d)
	for {
		select {
		case env := <-ch:
			if env.Kind == rt.KindMessage && env.Message.Type == typ {
				return &env
			}
		case <-deadline:
			return nil
		}
	}
}

func payloadOf(env *rt.Envelope) map[string]any {
	var p map[string]any
	_ = json.Unmarshal(env.Message.Payload, &p)
	return p
}

// waitCount polls q until it returns want or d elapses.
func (c *chatEnv) waitCount(want int, d time.Duration, q string, args ...any) int {
	c.t.Helper()
	deadline := time.Now().Add(d)
	n := 0
	for time.Now().Before(deadline) {
		if n = c.count(q, args...); n == want {
			return n
		}
		time.Sleep(50 * time.Millisecond)
	}
	return n
}
