package unit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// uowDouble is a TEST DOUBLE (R17) of write.UnitOfWork: runs fn directly, counts the transactions.
type uowDouble struct{ txs int }

func (u *uowDouble) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	u.txs++
	return fn(ctx)
}
func (u *uowDouble) InTenantTx(ctx context.Context, _ tenancy.Scope, fn func(ctx context.Context) error) error {
	u.txs++
	return fn(ctx)
}

// purgeStore is a TEST DOUBLE (R17) of port.PurgeStore recording what the use case asks.
type purgeStore struct {
	ready     bool
	readyErr  error
	expired   []port.ExpiredChannel
	holds     map[uuid.UUID]port.Holds
	failPurge map[uuid.UUID]bool
	purged    []uuid.UUID
	runs      []port.PurgeRun
	dropped   bool
	ephTenant []uuid.UUID
	ephFail   bool
	ephDrop   bool
	ephDelete []uuid.UUID
}

func (s *purgeStore) ModerationReady(context.Context) (bool, error) { return s.ready, s.readyErr }
func (s *purgeStore) Expired(context.Context, time.Time, int) ([]port.ExpiredChannel, error) {
	return s.expired, nil
}
func (s *purgeStore) Holds(_ context.Context, _, ch uuid.UUID) (port.Holds, error) {
	return s.holds[ch], nil
}
func (s *purgeStore) PurgeChannel(_ context.Context, c port.ExpiredChannel, _, _ int, _ time.Time) (port.ChannelPurge, error) {
	if s.failPurge[c.ID] {
		return port.ChannelPurge{}, errors.New("TEST DOUBLE: evidence copy failed")
	}
	s.purged = append(s.purged, c.ID)
	return port.ChannelPurge{Messages: 3, Evidence: 1, Stats: []port.DayStats{{Day: time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC),
		ChannelType: c.ChannelType, Channels: 1, Messages: 3}}}, nil
}
func (s *purgeStore) DropGameplayPartitions(context.Context) (int, error) {
	s.dropped = true
	return 0, nil
}
func (s *purgeStore) EphemeralTenants(context.Context, time.Time) ([]uuid.UUID, error) {
	return s.ephTenant, nil
}
func (s *purgeStore) CopyEphemeralEvidence(context.Context, uuid.UUID, time.Time, int, time.Time) (int, error) {
	if s.ephFail {
		return 0, errors.New("TEST DOUBLE: copy failed")
	}
	return 0, nil
}
func (s *purgeStore) DropEphemeralPartitions(context.Context, time.Time) (int, error) {
	s.ephDrop = true
	return 1, nil
}
func (s *purgeStore) DeleteEphemeral(_ context.Context, tenant uuid.UUID, _ time.Time, _ int) (int64, error) {
	s.ephDelete = append(s.ephDelete, tenant)
	return 2, nil
}
func (s *purgeStore) RecordRun(_ context.Context, r port.PurgeRun) error {
	s.runs = append(s.runs, r)
	return nil
}
func (s *purgeStore) EphemeralRetention(context.Context) (*time.Duration, error) { return nil, nil }
func (s *purgeStore) EnsurePartitions(context.Context, time.Time, int) (int, map[string]int, error) {
	return 0, map[string]int{}, nil
}

// purgeMetrics is a TEST DOUBLE (R17) of port.PurgeMetrics.
type purgeMetrics struct {
	blocked  map[string]bool
	deferred map[string]int
	failed   map[string]int
}

func newPurgeMetrics() *purgeMetrics {
	return &purgeMetrics{blocked: map[string]bool{}, deferred: map[string]int{}, failed: map[string]int{}}
}
func (m *purgeMetrics) Purged(string, string, int)     {}
func (m *purgeMetrics) Deferred(reason string)         { m.deferred[reason]++ }
func (m *purgeMetrics) Blocked(reason string, on bool) { m.blocked[reason] = on }
func (m *purgeMetrics) Backlog(int, time.Duration)     {}
func (m *purgeMetrics) Failed(stage string)            { m.failed[stage]++ }
func (m *purgeMetrics) PartitionsAhead(string, int)    {}
func (m *purgeMetrics) failures(stage string) int      { return m.failed[stage] }
func (m *purgeMetrics) isBlocked(reason string) bool   { return m.blocked[reason] }
func (m *purgeMetrics) deferrals(reason string) int    { return m.deferred[reason] }

// eventsDouble / auditDouble are TEST DOUBLEs (R17) of the outbox publisher and the auditor.
type eventsDouble struct{ events []write.Event }

func (e *eventsDouble) Publish(_ context.Context, ev write.Event) error {
	e.events = append(e.events, ev)
	return nil
}

type auditDouble struct{ events []audit.Event }

func (a *auditDouble) Record(_ context.Context, ev audit.Event) error {
	a.events = append(a.events, ev)
	return nil
}

func purgeHandler(s *purgeStore, m *purgeMetrics, ev *eventsDouble, au *auditDouble) command.GameplayPurge {
	return command.GameplayPurge{UoW: &uowDouble{}, Store: s, Events: ev, Audit: au, Metrics: m,
		Config: command.PurgeConfig{EvidenceContext: 20, Batch: 1000, BacklogAlert: 48 * time.Hour, EphemeralRetention: 24 * time.Hour},
		Now:    func() time.Time { return time.Date(2026, 11, 30, 3, 0, 0, 0, time.UTC) }}
}

// T029 / F117-AC-19 (TC-CHAT-022): without the moderation sources the purge deletes nothing, records a blocked run per
// kind, raises eduworld_chat_purge_blocked{moderation_unavailable} and audits the run; a failed evidence copy keeps the
// channel (the transaction rolls back) and the run is partial; an open case or report defers the channel (AC-18); a failed
// ephemeral copy keeps every partition and the rows of that tenant (AC-21).
func TestGameplayPurgeFailClosed(t *testing.T) {
	tenant := uuid.New()
	ch := port.ExpiredChannel{ID: uuid.New(), TenantID: tenant, ChannelType: string(channel.Match), PurgeAfter: time.Date(2026, 11, 29, 0, 0, 0, 0, time.UTC)}
	for _, c := range []struct {
		name string
		s    *purgeStore
	}{
		{"not ready", &purgeStore{ready: false, expired: []port.ExpiredChannel{ch}}},
		{"readiness error", &purgeStore{readyErr: errors.New("TEST DOUBLE: down"), expired: []port.ExpiredChannel{ch}}},
	} {
		m, ev, au := newPurgeMetrics(), &eventsDouble{}, &auditDouble{}
		res, err := purgeHandler(c.s, m, ev, au).RunOnce(context.Background())
		if err != nil || !res.Blocked || len(c.s.purged) != 0 || c.s.dropped || c.s.ephDrop || len(c.s.ephDelete) != 0 {
			t.Fatalf("%s: something was deleted: %+v %v %+v", c.name, res, err, c.s)
		}
		if !m.isBlocked(command.ReasonModerationUnavailable) || len(c.s.runs) != 2 || c.s.runs[0].Status != port.RunBlocked ||
			c.s.runs[0].TenantID != nil || len(au.events) != 2 || au.events[0].Result != audit.ResultFailure {
			t.Fatalf("%s: blocked run not recorded: runs=%+v audit=%+v", c.name, c.s.runs, au.events)
		}
	}

	held, failing, ok := ch, ch, ch
	held.ID, failing.ID, ok.ID = uuid.New(), uuid.New(), uuid.New()
	s := &purgeStore{ready: true, expired: []port.ExpiredChannel{held, failing, ok},
		holds:     map[uuid.UUID]port.Holds{held.ID: {OpenCases: 1}},
		failPurge: map[uuid.UUID]bool{failing.ID: true}, ephTenant: []uuid.UUID{tenant}, ephFail: true}
	m, ev, au := newPurgeMetrics(), &eventsDouble{}, &auditDouble{}
	res, err := purgeHandler(s, m, ev, au).RunOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(s.purged) != 1 || s.purged[0] != ok.ID || res.Deferred != 1 || res.Failed != 1 || res.ChannelsPurged != 1 {
		t.Fatalf("held/failing channels touched: %+v purged=%v", res, s.purged)
	}
	if m.deferrals(command.ReasonOpenCase) != 1 || m.failures("channel") != 1 || m.isBlocked(command.ReasonModerationUnavailable) {
		t.Fatalf("metrics: %+v", m)
	}
	if s.ephDrop || len(s.ephDelete) != 0 || m.failures("ephemeral_evidence") != 1 {
		t.Fatalf("ephemeral data dropped although the evidence copy failed: drop=%v delete=%v", s.ephDrop, s.ephDelete)
	}
	var gameplayRun *port.PurgeRun
	for i, r := range s.runs {
		if r.TenantID != nil && *r.TenantID == tenant && r.Kind == port.KindGameplay {
			gameplayRun = &s.runs[i]
		}
	}
	if gameplayRun == nil || gameplayRun.Status != port.RunPartial || gameplayRun.ChannelsPurged != 1 || gameplayRun.DeferredOpenCase != 1 ||
		gameplayRun.EvidenceCopied != 1 || gameplayRun.MessagesPurged != 3 {
		t.Fatalf("tenant run: %+v", gameplayRun)
	}
	types := map[string]int{}
	for _, e := range ev.events {
		types[e.Type]++
	}
	if types[command.EventGameplayPurged] < 3 || types[command.EventDailyStats] != 1 {
		t.Fatalf("events: %v", types)
	}
}

// lockDouble is a TEST DOUBLE (R17) of port.JobLock held by another worker.
type lockDouble struct{ free bool }

func (l lockDouble) Acquire(context.Context, string, time.Duration) (bool, error) { return l.free, nil }

// Edge case: two workers — only the holder of ew:lock:job:gameplay-chat-purge runs.
func TestGameplayPurgeSingleInstance(t *testing.T) {
	s := &purgeStore{ready: true, expired: []port.ExpiredChannel{{ID: uuid.New(), TenantID: uuid.New(), ChannelType: "party"}}}
	h := purgeHandler(s, newPurgeMetrics(), &eventsDouble{}, &auditDouble{})
	h.Lock = lockDouble{free: false}
	if n, err := h.Run(context.Background()); err != nil || n != 0 || len(s.purged) != 0 || len(s.runs) != 0 {
		t.Fatalf("ran without the lease: n=%d err=%v %+v", n, err, s)
	}
	h.Lock = lockDouble{free: true}
	if n, err := h.Run(context.Background()); err != nil || n != 1 {
		t.Fatalf("with the lease: n=%d err=%v", n, err)
	}
}

// T015 / F117-AC-15: purge_after = closed_at + effective retention; below the ADR-0033 minimum the minimum applies.
func TestGameplayRetention(t *testing.T) {
	closed := time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		days, want int
		floored    bool
	}{{7, 7, false}, {3, 3, false}, {2, 3, true}, {1, 3, true}, {30, 30, false}} {
		r := channel.ResolveRetention(c.days)
		if r.Effective != c.want || r.Floored != c.floored || !r.PurgeAfter(closed).Equal(closed.AddDate(0, 0, c.want)) {
			t.Errorf("%d days: %+v", c.days, r)
		}
	}
	ch, _ := channel.New(uuid.New(), uuid.New(), nil, channel.Match, map[string]string{"match_id": "m"}, closed)
	changed, err := ch.Close(closed, channel.ResolveRetention(7))
	if err != nil || !changed || ch.Open() || ch.PurgeAfter == nil || !ch.PurgeAfter.Equal(closed.AddDate(0, 0, 7)) {
		t.Fatalf("close: %+v %v", ch, err)
	}
	if again, _ := ch.Close(closed.Add(time.Hour), channel.ResolveRetention(30)); again || !ch.PurgeAfter.Equal(closed.AddDate(0, 0, 7)) {
		t.Fatal("closing twice must keep purge_after")
	}
	club, _ := channel.New(uuid.New(), uuid.New(), nil, channel.Team, map[string]string{"club_id": "c"}, closed)
	if _, err := club.Close(closed, channel.ResolveRetention(7)); !errors.Is(err, channel.ErrNotGameplay) {
		t.Fatalf("a club team channel is not gameplay: %v", err)
	}
}
