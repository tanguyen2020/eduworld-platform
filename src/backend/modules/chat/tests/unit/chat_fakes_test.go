package unit

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/service"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/policy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Every type below is a TEST DOUBLE (R17) of a port of the send path; the integration tests use the real adapters.

type runner struct{}

func (runner) InTenantRead(ctx context.Context, _ tenancy.Scope, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

type reader struct {
	facts  *read.ChannelFacts
	levels policy.Levels
	last   int64
}

func (r *reader) ChannelFacts(context.Context, uuid.UUID, uuid.UUID) (*read.ChannelFacts, error) {
	return r.facts, nil
}
func (r *reader) PolicyLevels(context.Context, uuid.UUID, *uuid.UUID, *uuid.UUID) (policy.Levels, error) {
	return r.levels, nil
}
func (r *reader) CurfewWindows(context.Context, uuid.UUID, *uuid.UUID) (*read.CurfewWindow, *read.CurfewWindow, error) {
	return nil, nil, nil
}
func (r *reader) LastSeq(context.Context, uuid.UUID, uuid.UUID) (int64, error) { return r.last, nil }
func (r *reader) MyChannels(context.Context, uuid.UUID, uuid.UUID, int) ([]read.MyChannel, error) {
	return nil, nil
}
func (r *reader) Messages(context.Context, read.MessageQuery) ([]read.StoredMessage, error) {
	return nil, nil
}

type memCache struct {
	mu sync.Mutex
	m  map[string][]byte
}

func (c *memCache) Get(ctx context.Context, key string, _ time.Duration, load func(ctx context.Context) ([]byte, error)) ([]byte, error) {
	c.mu.Lock()
	v, ok := c.m[key]
	c.mu.Unlock()
	if ok {
		return v, nil
	}
	v, err := load(ctx)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	if c.m == nil {
		c.m = map[string][]byte{}
	}
	c.m[key] = v
	c.mu.Unlock()
	return v, nil
}

func (c *memCache) Invalidate(context.Context, ...string) error { return nil }

type spyValidator struct {
	mu       sync.Mutex
	calls    []port.ValidationInput
	decision message.Decision
	display  string
}

func (s *spyValidator) Validate(_ context.Context, in port.ValidationInput) (port.Verdict, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, in)
	d := s.decision
	if d == "" {
		d = message.Allow
	}
	return port.Verdict{Decision: d, Display: s.display}, nil
}

type seqs struct {
	mu sync.Mutex
	n  map[uuid.UUID]int64
}

func (s *seqs) Next(ctx context.Context, ch uuid.UUID, floor func(ctx context.Context) (int64, error)) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.n == nil {
		s.n = map[uuid.UUID]int64{}
	}
	if _, ok := s.n[ch]; !ok {
		f, err := floor(ctx)
		if err != nil {
			return 0, err
		}
		s.n[ch] = f
	}
	s.n[ch]++
	return s.n[ch], nil
}

func (s *seqs) allocated() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.n)
}

type stream struct {
	mu   sync.Mutex
	sent []message.Envelope
	fail error
}

func (s *stream) Publish(_ context.Context, e message.Envelope) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail != nil {
		return s.fail
	}
	s.sent = append(s.sent, e)
	return nil
}

type fanout struct {
	mu      sync.Mutex
	created []message.Envelope
	acked   []message.Envelope
	held    []message.Envelope
}

func (f *fanout) Created(_ context.Context, e message.Envelope, _ read.ChatCard, _ []uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.created = append(f.created, e)
	return nil
}
func (f *fanout) Acked(_ context.Context, _ uuid.UUID, _ string, e message.Envelope) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.acked = append(f.acked, e)
	return nil
}
func (f *fanout) Held(_ context.Context, _ uuid.UUID, _ string, e message.Envelope) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.held = append(f.held, e)
	return nil
}
func (f *fanout) History(context.Context, uuid.UUID, string, uuid.UUID, []port.HistoryItem, bool) error {
	return nil
}

type recent struct{}

func (recent) Push(context.Context, message.Envelope) error                { return nil }
func (recent) List(context.Context, uuid.UUID) ([]message.Envelope, error) { return nil, nil }

type rate struct {
	mu    sync.Mutex
	count map[uuid.UUID]int
	last  port.RateLimits
}

func (r *rate) Allow(_ context.Context, user, ch, _ uuid.UUID, l port.RateLimits, _ time.Time) (bool, time.Duration, error) {
	r.mu.Lock()
	r.last = l
	r.mu.Unlock()
	perChannel := l.PerChannel
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.count == nil {
		r.count = map[uuid.UUID]int{}
	}
	if r.count[ch] >= perChannel {
		return false, 5 * time.Second, nil
	}
	r.count[ch]++
	return true, 0, nil
}

type dedupe struct {
	mu sync.Mutex
	m  map[string]port.SendRecord
}

func (d *dedupe) key(s, c uuid.UUID) string { return s.String() + c.String() }

func (d *dedupe) Reserve(_ context.Context, s, c uuid.UUID) (port.SendRecord, bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.m == nil {
		d.m = map[string]port.SendRecord{}
	}
	if r, ok := d.m[d.key(s, c)]; ok {
		return r, false, nil
	}
	d.m[d.key(s, c)] = port.SendRecord{}
	return port.SendRecord{}, true, nil
}
func (d *dedupe) Save(_ context.Context, s, c uuid.UUID, r port.SendRecord) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.m[d.key(s, c)] = r
	return nil
}
func (d *dedupe) Release(_ context.Context, s, c uuid.UUID) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.m, d.key(s, c))
	return nil
}

// platformDoc is the platform default of CHAT_SYSTEM §3.1 for the channel types used by the unit tests.
func platformDoc() *policy.Document {
	var d policy.Document
	_ = json.Unmarshal([]byte(`{"scope_type":"platform","global_rate":{"messages_per_minute":30,"burst":5},"rules":[
		{"channel_type":"party","enabled":true,"allowed_kinds":["text","emoji","quick","sticker","gif"],"max_text_length":200,
		 "rate":{"messages_per_minute":12,"burst":3},"validation":{"strictness":"strict"}},
		{"channel_type":"class","enabled":true,"allowed_kinds":["text","emoji","quick","sticker","gif"],"max_text_length":300,
		 "rate":{"messages_per_minute":10,"burst":3},"validation":{"strictness":"strict"}}]}`), &d)
	return &d
}

type harness struct {
	send      command.Send
	validator *spyValidator
	seq       *seqs
	stream    *stream
	fanout    *fanout
	tenant    uuid.UUID
	sender    uuid.UUID
	other     uuid.UUID
	channel   uuid.UUID
}

func newHarness(t channel.Type) *harness {
	h := &harness{validator: &spyValidator{}, seq: &seqs{}, stream: &stream{}, fanout: &fanout{}, tenant: uuid.New(), sender: uuid.New(),
		other: uuid.New(), channel: uuid.New()}
	school := uuid.New()
	scope := map[string]string{"party_id": "p1"}
	if t == channel.Class {
		scope = map[string]string{"class_id": uuid.NewString()}
	}
	ch, _ := channel.New(h.channel, h.tenant, &school, t, scope, time.Now().Add(-time.Hour))
	facts := &read.ChannelFacts{Channel: ch, Cards: map[uuid.UUID]read.ChatCard{}, Members: channel.Members{
		{ChannelID: h.channel, UserID: h.sender, TenantID: h.tenant, SchoolID: &school, JoinedAt: time.Now().Add(-time.Hour)},
		{ChannelID: h.channel, UserID: h.other, TenantID: h.tenant, SchoolID: &school, JoinedAt: time.Now().Add(-time.Hour)}}}
	rd := &reader{facts: facts, levels: policy.Levels{Platform: platformDoc()}}
	// Noon ICT: outside the platform curfew.
	noon := time.Date(2026, 11, 21, 5, 0, 0, 0, time.UTC)
	ict := time.FixedZone("ICT", 7*3600)
	curfew, _ := policy.ParseWindow("21:30-06:30", ict)
	h.send = command.Send{Facts: service.Facts{Read: runner{}, Reader: rd, Cache: &memCache{}, Recent: recent{},
		Config: service.FactsConfig{FactsFresh: time.Minute, PolicyCache: time.Minute, CurfewFresh: time.Minute, PlatformCurfew: curfew,
			CurfewLocation: ict}},
		Validator: h.validator, Seq: h.seq, Stream: h.stream, Fanout: h.fanout, Recent: recent{}, Rate: &rate{}, Dedupe: &dedupe{},
		Config: command.SendConfig{RatePerUser: 30}, Now: func() time.Time { return noon }}
	return h
}

func (h *harness) req(kind message.Kind) message.Request {
	r := message.Request{ChannelID: h.channel, ClientMessageID: uuid.New(), Kind: kind}
	sticker := uuid.New()
	switch kind {
	case message.KindText:
		r.Body = "xin chao"
	case message.KindEmoji:
		r.Emoji = "smile"
	case message.KindQuick:
		r.QuickMessageID = "qm.good_luck"
	case message.KindSticker:
		r.StickerID = &sticker
	case message.KindGif:
		r.GifID = &sticker
	}
	return r
}

var errBoom = errors.New("boom")
