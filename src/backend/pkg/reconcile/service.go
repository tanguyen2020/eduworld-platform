package reconcile

import (
	"context"
	"encoding/base64"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Facade parameter limits (specs/openapi/modules/common/internal-common.v1.yaml, F134-AC-05).
const (
	MaxPageLimit = 5000
	MaxCursorLen = 512
	MaxTypes     = 100
	MaxTypeLen   = 64
	// MaxWindow is the longest window a facade accepts: outbox_retention (30 days) plus one day of slack for the
	// partition kept on the boundary day; the command checks the configured retention (F134-AC-04).
	MaxWindow = 31 * 24 * time.Hour
)

var eventTypeRe = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)

// Store is the read model of the infra tables of one module database (pkg/outbox.Reader).
type Store interface {
	Outbox(ctx context.Context, from, to time.Time, fromID, toID uuid.UUID, types []string, after uuid.UUID, limit int) ([]outbox.OutboxRow, error)
	Processed(ctx context.Context, from time.Time, consumers []string, fromID, toID uuid.UUID, after outbox.ProcessedKey, limit int) ([]outbox.ProcessedRow, error)
}

// Source is the query side of the two reconciliation facade operations of one module (F134 T017): shared by the five
// modules so the rules (authorization, limits, keyset pagination, no payload) cannot drift. Read only: no write, no
// event, no UnitOfWork (R26).
type Source struct {
	Module string
	Store  Store
	// Consumers returns the declared consumers of the module (pkg/module.Registry.ConsumerInfos), read at call time.
	Consumers func() []ConsumerDecl
}

// ConsumerDecl is one declared consumer of the module and its source.
type ConsumerDecl struct {
	Name       string
	Source     string
	EventTypes []string
}

// OutboxQuery are the parameters of listOutboxEventIds.
type OutboxQuery struct {
	From, To   time.Time
	EventTypes []string
	Cursor     string
	Limit      int
}

// ProcessedQuery are the parameters of listProcessedEventIds.
type ProcessedQuery struct {
	SourceModule string
	From, To     time.Time
	Cursor       string
	Limit        int
}

// authorize: only an in-process facade call (pkg/modcall sets the caller, checked against x-allowed-callers) of the
// reconcile job, with a platform scope and the audited bypass platform_job (ADR-0089), reaches the infra tables.
func authorize(ctx context.Context) error {
	if modcall.CallerFrom(ctx) == "" {
		return problem.New(problem.CallerNotAllowed, "")
	}
	s, ok := tenancy.ScopeFrom(ctx)
	if !ok || !s.PlatformOnly() {
		return problem.New(problem.PermissionDenied, "reconciliation needs the platform scope of the reconcile job")
	}
	if r, ok := tenancy.Bypassed(ctx); !ok || r != tenancy.ReasonPlatformJob {
		return problem.New(problem.PermissionDenied, "reconciliation needs the audited platform_job bypass")
	}
	return nil
}

func limitOf(n int) (int, error) {
	switch {
	case n == 0:
		return MaxPageLimit, nil
	case n < 1 || n > MaxPageLimit:
		return 0, problem.Invalid("limit", problem.SubInvalid, fmt.Sprintf("must be within 1..%d", MaxPageLimit))
	}
	return n, nil
}

func windowOf(from, to time.Time) error {
	switch {
	case from.IsZero() || to.IsZero():
		return problem.Invalid("from", problem.SubRequired, "from and to are required")
	case !from.Before(to):
		return problem.Invalid("to", problem.SubInvalid, "to must be after from")
	case to.Sub(from) > MaxWindow:
		return problem.Invalid("to", problem.SubInvalid, "window longer than outbox_retention")
	}
	return nil
}

func cursorOf(c string) error {
	if len(c) > MaxCursorLen {
		return problem.Invalid("cursor", problem.SubInvalid, fmt.Sprintf("longer than %d", MaxCursorLen))
	}
	return nil
}

// ValidateOutbox checks the parameters of listOutboxEventIds (VALIDATION_FAILED).
func ValidateOutbox(q OutboxQuery) (int, uuid.UUID, error) {
	if err := windowOf(q.From, q.To); err != nil {
		return 0, uuid.Nil, err
	}
	if len(q.EventTypes) > MaxTypes {
		return 0, uuid.Nil, problem.Invalid("event_types", problem.SubInvalid, fmt.Sprintf("at most %d", MaxTypes))
	}
	for i, t := range q.EventTypes {
		if len(t) > MaxTypeLen || !eventTypeRe.MatchString(t) {
			return 0, uuid.Nil, problem.Invalid(fmt.Sprintf("event_types/%d", i), problem.SubInvalid, "not an event type")
		}
	}
	if err := cursorOf(q.Cursor); err != nil {
		return 0, uuid.Nil, err
	}
	after := uuid.Nil
	if q.Cursor != "" {
		id, err := uuid.Parse(q.Cursor)
		if err != nil {
			return 0, uuid.Nil, problem.Invalid("cursor", problem.SubInvalid, "malformed cursor")
		}
		after = id
	}
	n, err := limitOf(q.Limit)
	return n, after, err
}

// ValidateProcessed checks the parameters of listProcessedEventIds (VALIDATION_FAILED).
func ValidateProcessed(q ProcessedQuery) (int, outbox.ProcessedKey, error) {
	var k outbox.ProcessedKey
	if !slices.Contains(Modules, q.SourceModule) {
		return 0, k, problem.Invalid("source_module", problem.SubInvalid, "must be identity, learning, game, competition or chat")
	}
	if err := windowOf(q.From, q.To); err != nil {
		return 0, k, err
	}
	if err := cursorOf(q.Cursor); err != nil {
		return 0, k, err
	}
	if q.Cursor != "" {
		var err error
		if k, err = decodeProcessed(q.Cursor); err != nil {
			return 0, k, problem.Invalid("cursor", problem.SubInvalid, "malformed cursor")
		}
	}
	n, err := limitOf(q.Limit)
	return n, k, err
}

// ListOutbox is listOutboxEventIds: a page of the outbox of the module in [From, To) on the event-id time.
func (s Source) ListOutbox(ctx context.Context, q OutboxQuery) (out []OutboxEvent, next string, err error) {
	ctx, done := observability.UseCase(ctx, s.Module+".query.ListOutboxEventIds")
	defer func() { done(err) }()
	if err := authorize(ctx); err != nil {
		return nil, "", err
	}
	limit, after, err := ValidateOutbox(q)
	if err != nil {
		return nil, "", err
	}
	rows, err := s.Store.Outbox(ctx, q.From, q.To, ids.Floor(q.From), ids.Floor(q.To), q.EventTypes, after, limit+1)
	if err != nil {
		return nil, "", problem.New(problem.Unavailable, "").Retry().Wrap(err)
	}
	if len(rows) > limit {
		rows = rows[:limit]
		next = rows[limit-1].EventID.String()
	}
	out = make([]OutboxEvent, 0, len(rows))
	for _, r := range rows {
		out = append(out, OutboxEvent{EventID: r.EventID, EventType: r.EventType, TenantID: r.TenantID, CreatedAt: r.CreatedAt, PublishedAt: r.PublishedAt})
	}
	return out, next, nil
}

// ConsumersOf returns the consumers of the module that receive the events of source. A consumer name declared for
// several sources cannot be split by source in processed_events and is left out (none today).
func (s Source) ConsumersOf(source string) []Consumer {
	if s.Consumers == nil {
		return nil
	}
	decls := s.Consumers()
	sources := map[string]map[string]bool{}
	for _, d := range decls {
		if sources[d.Name] == nil {
			sources[d.Name] = map[string]bool{}
		}
		sources[d.Name][d.Source] = true
	}
	var out []Consumer
	for _, d := range decls {
		if d.Source != source || len(sources[d.Name]) != 1 || len(d.EventTypes) == 0 {
			continue
		}
		out = append(out, Consumer{Name: d.Name, EventTypes: append([]string(nil), d.EventTypes...)})
	}
	return out
}

// ListProcessed is listProcessedEventIds: the consumers of the module for source and a page of their processed and
// dead-lettered event ids, ordered by (consumer, event_id, status).
func (s Source) ListProcessed(ctx context.Context, q ProcessedQuery) (consumers []Consumer, out []Processed, next string, err error) {
	ctx, done := observability.UseCase(ctx, s.Module+".query.ListProcessedEventIds")
	defer func() { done(err) }()
	if err := authorize(ctx); err != nil {
		return nil, nil, "", err
	}
	limit, after, err := ValidateProcessed(q)
	if err != nil {
		return nil, nil, "", err
	}
	consumers = s.ConsumersOf(q.SourceModule)
	if consumers == nil {
		consumers = []Consumer{}
	}
	names := make([]string, 0, len(consumers))
	for _, c := range consumers {
		names = append(names, c.Name)
	}
	rows, err := s.Store.Processed(ctx, q.From, names, ids.Floor(q.From), ids.Floor(q.To), after, limit+1)
	if err != nil {
		return nil, nil, "", problem.New(problem.Unavailable, "").Retry().Wrap(err)
	}
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[limit-1]
		next = encodeProcessed(outbox.ProcessedKey{Consumer: last.Consumer, EventID: last.EventID, Status: last.Status})
	}
	out = make([]Processed, 0, len(rows))
	for _, r := range rows {
		out = append(out, Processed{Consumer: r.Consumer, EventID: r.EventID, At: r.At, DeadLettered: r.Status == "dead_lettered"})
	}
	return consumers, out, next, nil
}

func encodeProcessed(k outbox.ProcessedKey) string {
	return base64.RawURLEncoding.EncodeToString([]byte(k.Consumer + "\x00" + k.EventID.String() + "\x00" + k.Status))
}

func decodeProcessed(c string) (outbox.ProcessedKey, error) {
	var k outbox.ProcessedKey
	b, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil {
		return k, err
	}
	parts := strings.Split(string(b), "\x00")
	if len(parts) != 3 || parts[0] == "" || len(parts[0]) > 128 || (parts[2] != "processed" && parts[2] != "dead_lettered") {
		return k, fmt.Errorf("reconcile: cursor")
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return k, err
	}
	return outbox.ProcessedKey{Consumer: parts[0], EventID: id, Status: parts[2]}, nil
}
