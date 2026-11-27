package reconcile

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Modules with a database (realtime has none, ADR-0074).
var Modules = []string{"identity", "learning", "game", "competition", "chat"}

// Wildcard in Consumer.EventTypes: the consumer subscribes to every event type of the source.
const Wildcard = "*"

// OutboxEvent is one row of listOutboxEventIds.
type OutboxEvent struct {
	EventID     uuid.UUID
	EventType   string
	TenantID    *uuid.UUID
	CreatedAt   time.Time
	PublishedAt *time.Time
}

// Consumer is a durable consumer of a source module and the event types it subscribes to.
type Consumer struct {
	Name       string
	EventTypes []string
}

// Accepts reports whether the consumer subscribes to typ.
func (c Consumer) Accepts(typ string) bool {
	return slices.Contains(c.EventTypes, Wildcard) || slices.Contains(c.EventTypes, typ)
}

// Processed is one row of listProcessedEventIds (processed_events or dead_letters).
type Processed struct {
	Consumer     string
	EventID      uuid.UUID
	At           time.Time
	DeadLettered bool
}

// Peer reads one module through its facade (in process, pkg/modcall); the reconciled module is a Peer of itself.
type Peer interface {
	// ListOutboxEventIDs is listOutboxEventIds: a page of the module outbox in [from, to), types nil = every type.
	ListOutboxEventIDs(ctx context.Context, from, to time.Time, types []string, cursor string, limit int) ([]OutboxEvent, string, error)
	// ListProcessedEventIDs is listProcessedEventIds: the module consumers of source and a page of their rows.
	ListProcessedEventIDs(ctx context.Context, source string, from, to time.Time, cursor string, limit int) ([]Consumer, []Processed, string, error)
}

// Options of one run.
type Options struct {
	Module          string          // the restored module
	Peers           map[string]Peer // every module with a database, including Module itself
	Window          Window
	StreamRetention time.Duration // event_stream_retention: chooses the compensation hint
	PageLimit       int           // default 5000 (facade ceiling)
	Now             func() time.Time
	RunID           uuid.UUID
}

// Run reconciles the module with every other module in both directions (F134-AC-01, AC-02). A module that cannot be
// read marks its peer failed and the report incomplete; the run goes on with the others. The returned error is only
// for a misuse of the options (exit 1).
func Run(ctx context.Context, o Options) (Report, error) {
	if !slices.Contains(Modules, o.Module) {
		return Report{}, fmt.Errorf("%w: unknown module %q", ErrWindow, o.Module)
	}
	if o.Peers[o.Module] == nil {
		return Report{}, fmt.Errorf("reconcile: no facade for module %s", o.Module)
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.PageLimit <= 0 || o.PageLimit > 5000 {
		o.PageLimit = 5000
	}
	if o.RunID == uuid.Nil {
		o.RunID = ids.New()
	}
	now := o.Now().UTC()
	w := o.Window
	r := Report{RunID: o.RunID, Module: o.Module, RestorePoint: w.RestorePoint.UTC(), WindowFrom: w.From.UTC(), WindowTo: w.To.UTC(),
		SettleSeconds: int64(w.Settle / time.Second), Status: "complete", Peers: []PeerState{}, Mismatches: []Mismatch{}}
	if o.StreamRetention > 0 {
		r.CompensationHint = "reset_consumer"
		if now.Sub(w.RestorePoint) > o.StreamRetention {
			r.CompensationHint = "relay_outbox"
		}
	}
	run := runner{o: o, now: now, r: &r}
	self := o.Peers[o.Module]
	for _, m := range Modules {
		if m == o.Module {
			continue
		}
		p := o.Peers[m]
		// Outbound: events of the restored module consumed by m.
		run.direction(ctx, m, Outbound, p, o.Module, self, p)
		// Inbound: events of m consumed by the restored module.
		run.direction(ctx, m, Inbound, self, m, p, p)
	}
	r.GeneratedAt = o.Now().UTC()
	sort.SliceStable(r.Mismatches, func(i, j int) bool {
		a, b := r.Mismatches[i], r.Mismatches[j]
		if a.Direction != b.Direction {
			return a.Direction > b.Direction
		}
		if a.Consumer != b.Consumer {
			return a.Consumer < b.Consumer
		}
		return a.EventID.String() < b.EventID.String()
	})
	return r, nil
}

type runner struct {
	o   Options
	now time.Time
	r   *Report
}

// direction compares the outbox of source (read through sourcePeer) with the rows of the consumers of
// consumerSide for that source; peerName/peer identify the other module of the pair (failed when nil or erroring).
func (run runner) direction(ctx context.Context, peerName string, dir Direction, consumerSide Peer, source string, sourcePeer, peer Peer) {
	st := PeerState{Module: peerName, Direction: dir, Status: "ok"}
	defer func() { run.r.Peers = append(run.r.Peers, st) }()
	if peer == nil {
		run.fail(&st, errors.New("module not loaded"))
		return
	}
	w := run.o.Window
	consumers, first, next, err := consumerSide.ListProcessedEventIDs(ctx, source, w.From, w.To, "", run.o.PageLimit)
	if err != nil {
		run.fail(&st, err)
		return
	}
	if len(consumers) == 0 {
		return // nobody consumes these events: nothing to compare
	}
	for _, c := range consumers {
		st.Consumers = append(st.Consumers, c.Name)
	}
	var types []string
	for _, c := range consumers {
		if slices.Contains(c.EventTypes, Wildcard) {
			types = nil
			break
		}
		for _, t := range c.EventTypes {
			if !slices.Contains(types, t) {
				types = append(types, t)
			}
		}
	}
	events, err := run.outbox(ctx, sourcePeer, types)
	if err != nil {
		run.fail(&st, err)
		return
	}
	byName := map[string]Consumer{}
	for _, c := range consumers {
		byName[c.Name] = c
	}
	seen := map[string]bool{}
	// Rows arrive sorted by (consumer, event_id, status): merge one consumer at a time.
	var group []Processed
	flush := func() {
		if len(group) == 0 {
			return
		}
		name := group[0].Consumer
		if c, ok := byName[name]; ok && !seen[name] {
			seen[name] = true
			run.merge(dir, c, events, group)
		}
		group = group[:0]
	}
	page := first
	for {
		for _, it := range page {
			if len(group) > 0 && group[0].Consumer != it.Consumer {
				flush()
			}
			group = append(group, it)
		}
		if next == "" {
			break
		}
		cur := next
		_, page, next, err = consumerSide.ListProcessedEventIDs(ctx, source, w.From, w.To, cur, run.o.PageLimit)
		if err != nil {
			run.fail(&st, err)
			return
		}
		if next == cur {
			run.fail(&st, errors.New("cursor did not advance"))
			return
		}
	}
	flush()
	for _, c := range consumers { // consumers without any row: every source event of their types is missing
		if !seen[c.Name] {
			run.merge(dir, c, events, nil)
		}
	}
}

func (run runner) outbox(ctx context.Context, p Peer, types []string) ([]OutboxEvent, error) {
	w := run.o.Window
	var out []OutboxEvent
	cursor := ""
	for {
		page, next, err := p.ListOutboxEventIDs(ctx, w.From, w.To, types, cursor, run.o.PageLimit)
		if err != nil {
			return nil, err
		}
		out = append(out, page...)
		if next == "" {
			break
		}
		if next == cursor {
			return nil, errors.New("cursor did not advance")
		}
		cursor = next
	}
	sort.Slice(out, func(i, j int) bool { return less(out[i].EventID, out[j].EventID) })
	return out, nil
}

func less(a, b uuid.UUID) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

// merge compares the source events accepted by c with its rows (both sorted by event id).
func (run runner) merge(dir Direction, c Consumer, events []OutboxEvent, rows []Processed) {
	w := run.o.Window
	type state struct{ processed, dead bool }
	got := make(map[uuid.UUID]state, len(rows))
	for _, p := range rows {
		s := got[p.EventID]
		if p.DeadLettered {
			s.dead = true
		} else {
			s.processed = true
		}
		got[p.EventID] = s
	}
	inSource := make(map[uuid.UUID]bool, len(got))
	for _, e := range events {
		if !c.Accepts(e.EventType) {
			continue
		}
		s, ok := got[e.EventID]
		if ok {
			inSource[e.EventID] = true
		}
		cl := Classify(Observation{Direction: dir, InSource: true, PublishedAt: e.PublishedAt, Processed: s.processed, DeadLettered: s.dead,
			EventTime: eventTime(e.EventID, e.CreatedAt)}, w.RestorePoint, w.Settle, run.now)
		if cl != "" {
			run.r.record(Mismatch{EventID: e.EventID, EventType: e.EventType, Consumer: c.Name, TenantID: e.TenantID,
				EventTime: eventTime(e.EventID, e.CreatedAt), Classification: cl, Direction: dir})
		}
	}
	single := ""
	if len(c.EventTypes) == 1 && c.EventTypes[0] != Wildcard {
		single = c.EventTypes[0]
	}
	orphans := make([]uuid.UUID, 0)
	for id := range got {
		if !inSource[id] {
			orphans = append(orphans, id)
		}
	}
	sort.Slice(orphans, func(i, j int) bool { return less(orphans[i], orphans[j]) })
	for _, id := range orphans {
		s := got[id]
		t := eventTime(id, time.Time{})
		cl := Classify(Observation{Direction: dir, Processed: s.processed, DeadLettered: s.dead, EventTime: t}, w.RestorePoint, w.Settle, run.now)
		if cl != "" {
			run.r.record(Mismatch{EventID: id, EventType: single, Consumer: c.Name, EventTime: t, Classification: cl, Direction: dir})
		}
	}
}

func eventTime(id uuid.UUID, fallback time.Time) time.Time {
	if t, ok := ids.Time(id); ok {
		return t.UTC()
	}
	return fallback.UTC()
}

func (run runner) fail(st *PeerState, err error) {
	st.Status = "failed"
	st.ErrorCode = "UNAVAILABLE"
	var pe *problem.Error
	if errors.As(err, &pe) {
		st.ErrorCode = string(pe.Code)
	}
	if len(st.ErrorCode) > 64 {
		st.ErrorCode = st.ErrorCode[:64]
	}
	run.r.Status = "incomplete"
}
