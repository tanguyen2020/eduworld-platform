package reconcile

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// fakePeer is a TEST DOUBLE of a module facade (R17): its outbox and the processed rows of its consumers per source,
// served in pages of the requested limit.
type fakePeer struct {
	outbox    []OutboxEvent
	consumers map[string][]Consumer  // by source
	processed map[string][]Processed // by source, sorted (consumer, event_id)
	down      bool
	calls     int
}

func page[T any](xs []T, cursor string, limit int) ([]T, string) {
	start := 0
	if cursor != "" {
		start, _ = strconv.Atoi(cursor)
	}
	end := min(start+limit, len(xs))
	next := ""
	if end < len(xs) {
		next = strconv.Itoa(end)
	}
	return xs[start:end], next
}

func (p *fakePeer) ListOutboxEventIDs(_ context.Context, _, _ time.Time, types []string, cursor string, limit int) ([]OutboxEvent, string, error) {
	p.calls++
	if p.down {
		return nil, "", problem.New(problem.Unavailable, "database down")
	}
	var sel []OutboxEvent
	for _, e := range p.outbox {
		if types == nil || contains(types, e.EventType) {
			sel = append(sel, e)
		}
	}
	rows, next := page(sel, cursor, limit)
	return rows, next, nil
}

func (p *fakePeer) ListProcessedEventIDs(_ context.Context, source string, _, _ time.Time, cursor string, limit int) ([]Consumer, []Processed, string, error) {
	p.calls++
	if p.down {
		return nil, nil, "", errors.New("connection refused")
	}
	rows, next := page(p.processed[source], cursor, limit)
	return p.consumers[source], rows, next, nil
}

func at(base time.Time, minutes int) uuid.UUID {
	return ids.Floor(base.Add(time.Duration(minutes) * time.Minute))
}

type scenario struct {
	base, T, now      time.Time
	learning, game    *fakePeer
	identity, cmp, ch *fakePeer
}

// newScenario: learning restored to T (base+60). Game credited XP for 5 graded attempts ≤ T (still in the learning
// outbox) and 3 after T (lost by the restore). The learning outbox also holds one event published 1 minute ago (not yet
// delivered), one unpublished and one game moved to its DLQ. Inbound, identity published 2 tenant.updated after T that
// learning has processed (consumer reset).
func newScenario() *scenario {
	s := &scenario{base: time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)}
	s.T, s.now = s.base.Add(60*time.Minute), s.base.Add(180*time.Minute)
	pub := func(m int) *time.Time { v := s.base.Add(time.Duration(m) * time.Minute); return &v }
	tenant := ids.New()
	s.learning = &fakePeer{consumers: map[string][]Consumer{"identity": {{Name: "learning.projection_identity", EventTypes: []string{"tenant.updated"}}}},
		processed: map[string][]Processed{}}
	s.game = &fakePeer{consumers: map[string][]Consumer{"learning": {{Name: "game.quest_on_activity_graded", EventTypes: []string{"activity.graded"}}}},
		processed: map[string][]Processed{}}
	s.identity = &fakePeer{consumers: map[string][]Consumer{}, processed: map[string][]Processed{}}
	s.cmp = &fakePeer{consumers: map[string][]Consumer{}, processed: map[string][]Processed{}}
	s.ch = &fakePeer{consumers: map[string][]Consumer{}, processed: map[string][]Processed{}}
	var rows []Processed
	for m := 10; m <= 50; m += 10 {
		id := at(s.base, m)
		s.learning.outbox = append(s.learning.outbox, OutboxEvent{EventID: id, EventType: "activity.graded", TenantID: &tenant, PublishedAt: pub(m)})
		rows = append(rows, Processed{Consumer: "game.quest_on_activity_graded", EventID: id, At: s.base.Add(time.Duration(m) * time.Minute)})
	}
	dlq := at(s.base, 55)
	s.learning.outbox = append(s.learning.outbox,
		OutboxEvent{EventID: dlq, EventType: "activity.graded", TenantID: &tenant, PublishedAt: pub(55)},
		OutboxEvent{EventID: at(s.base, 170), EventType: "activity.graded", TenantID: &tenant, PublishedAt: pub(179)},
		OutboxEvent{EventID: at(s.base, 175), EventType: "activity.graded", TenantID: &tenant},
		OutboxEvent{EventID: at(s.base, 21), EventType: "attempt.started", PublishedAt: pub(21)}, // not consumed by game: ignored
	)
	rows = append(rows, Processed{Consumer: "game.quest_on_activity_graded", EventID: dlq, DeadLettered: true})
	for _, m := range []int{70, 80, 90} { // lost by the restore
		rows = append(rows, Processed{Consumer: "game.quest_on_activity_graded", EventID: at(s.base, m)})
	}
	sortRows(rows)
	s.game.processed["learning"] = rows
	for _, m := range []int{65, 95} {
		id := at(s.base, m)
		s.identity.outbox = append(s.identity.outbox, OutboxEvent{EventID: id, EventType: "tenant.updated", PublishedAt: pub(m)})
		s.learning.processed["identity"] = append(s.learning.processed["identity"], Processed{Consumer: "learning.projection_identity", EventID: id})
	}
	return s
}

func sortRows(rows []Processed) {
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0 && rowLess(rows[j], rows[j-1]); j-- {
			rows[j], rows[j-1] = rows[j-1], rows[j]
		}
	}
}

func rowLess(a, b Processed) bool {
	if a.Consumer != b.Consumer {
		return a.Consumer < b.Consumer
	}
	if a.EventID != b.EventID {
		return less(a.EventID, b.EventID)
	}
	return a.DeadLettered && !b.DeadLettered
}

func (s *scenario) run(t *testing.T, limit int) Report {
	t.Helper()
	r, err := Run(context.Background(), Options{Module: "learning", Window: Window{From: s.base, To: s.now, RestorePoint: s.T, Settle: 10 * time.Minute},
		Peers:           map[string]Peer{"learning": s.learning, "game": s.game, "identity": s.identity, "competition": s.cmp, "chat": s.ch},
		StreamRetention: 7 * 24 * time.Hour, PageLimit: limit, Now: func() time.Time { return s.now }})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// F134-AC-01, AC-02, AC-03 (TC-SVC-014) with stand-in facades, pages of 2 rows to cross page boundaries.
func TestRunScenarios(t *testing.T) {
	s := newScenario()
	r := s.run(t, 2)
	want := Counts{LostAfterRestore: 3, PendingPublish: 1, PendingDelivery: 1, DeadLettered: 1}
	if r.Counts != want || r.Status != "complete" || r.ExitCode() != ExitOK {
		t.Fatalf("restored learning: counts %+v status %s exit %d, want %+v complete 0", r.Counts, r.Status, r.ExitCode(), want)
	}
	for _, m := range r.Mismatches {
		if m.Classification == LostAfterRestore && (m.EventType != "activity.graded" || m.Consumer != "game.quest_on_activity_graded" || m.Direction != Outbound) {
			t.Fatalf("lost mismatch %+v", m)
		}
	}
	if r.CompensationHint != "reset_consumer" {
		t.Fatalf("hint %q", r.CompensationHint)
	}
	// AC-03: no payload nor personal data in the JSON report.
	b, _ := json.Marshal(r)
	for _, k := range []string{"payload", "envelope", "student", "name"} {
		if bytes.Contains(b, []byte(`"`+k)) {
			t.Fatalf("report carries %q: %s", k, b)
		}
	}
	var sum strings.Builder
	if err := r.Summary(&sum); err != nil || !strings.Contains(sum.String(), "lost_after_restore") {
		t.Fatalf("summary: %v %q", err, sum.String())
	}

	// AC-02: the learning consumer was not reset — the identity events after T are not processed → unexplained, exit 2.
	s = newScenario()
	s.learning.processed["identity"] = nil
	r = s.run(t, 5000)
	if r.Counts.Unexplained != 2 || r.ExitCode() != ExitUnexplained {
		t.Fatalf("not reset: %+v exit %d", r.Counts, r.ExitCode())
	}
	for _, m := range r.Mismatches {
		if m.Classification == Unexplained && m.Direction != Inbound {
			t.Fatalf("unexplained outbound %+v", m)
		}
	}

	// A consumer has an event of the restored module older than T that the outbox lost: unexplained.
	s = newScenario()
	s.game.processed["learning"] = append(s.game.processed["learning"], Processed{Consumer: "game.quest_on_activity_graded", EventID: at(s.base, 59)})
	sortRows(s.game.processed["learning"])
	if r = s.run(t, 3); r.Counts.Unexplained != 1 {
		t.Fatalf("loss before T: %+v", r.Counts)
	}

	// AC-03: module game cannot be read → incomplete, exit 3; the other modules are still compared.
	s = newScenario()
	s.game.down = true
	r = s.run(t, 5000)
	if r.Status != "incomplete" || r.ExitCode() != ExitIncomplete {
		t.Fatalf("game down: %s exit %d", r.Status, r.ExitCode())
	}
	failed := 0
	for _, p := range r.Peers {
		if p.Status == "failed" {
			failed++
			if p.Module != "game" || p.ErrorCode == "" {
				t.Fatalf("failed peer %+v", p)
			}
		}
	}
	if failed != 1 { // outbound only: learning has no consumer of game events, so game is not needed inbound
		t.Fatalf("failed peers %d: %+v", failed, r.Peers)
	}
	if r.Counts.LostAfterRestore != 0 {
		t.Fatalf("nothing read from game, counts %+v", r.Counts)
	}

	// A T older than event_stream_retention suggests relaying the outbox instead of resetting the consumer.
	s = newScenario()
	s.now = s.T.Add(8 * 24 * time.Hour)
	if r = s.run(t, 5000); r.CompensationHint != "relay_outbox" {
		t.Fatalf("hint %q", r.CompensationHint)
	}
}

func TestRunRefusesUnknownModule(t *testing.T) {
	if _, err := Run(context.Background(), Options{Module: "realtime"}); err == nil {
		t.Fatal("realtime has no database")
	}
	if _, err := Run(context.Background(), Options{Module: "learning", Peers: map[string]Peer{}}); err == nil {
		t.Fatal("no own facade")
	}
}

func TestMismatchCap(t *testing.T) {
	var r Report
	for i := 0; i < MaxMismatches+3; i++ {
		r.record(Mismatch{Classification: Unexplained})
	}
	if len(r.Mismatches) != MaxMismatches || !r.Truncated || r.Counts.Unexplained != MaxMismatches+3 {
		t.Fatalf("cap: %d truncated=%v count=%d", len(r.Mismatches), r.Truncated, r.Counts.Unexplained)
	}
}
