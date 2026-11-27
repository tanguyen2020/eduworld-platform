package reconcile

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

func code(err error) problem.Code {
	var pe *problem.Error
	if errors.As(err, &pe) {
		return pe.Code
	}
	return ""
}

// F134-AC-05 (TC-TEN-024): the facade parameters are bounded — cursor ≤ 512, source_module enum, limit ≤ 5000,
// event types bounded — and a violation is VALIDATION_FAILED.
func TestFacadeParameters(t *testing.T) {
	from := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	to := from.Add(time.Hour)
	okOutbox := OutboxQuery{From: from, To: to, EventTypes: []string{"activity.graded"}}
	if _, _, err := ValidateOutbox(okOutbox); err != nil {
		t.Fatalf("valid outbox query: %v", err)
	}
	for name, q := range map[string]OutboxQuery{
		"cursor 513":       {From: from, To: to, Cursor: strings.Repeat("a", 513)},
		"cursor malformed": {From: from, To: to, Cursor: "not-a-uuid"},
		"limit 5001":       {From: from, To: to, Limit: 5001},
		"limit negative":   {From: from, To: to, Limit: -1},
		"type too long":    {From: from, To: to, EventTypes: []string{"a." + strings.Repeat("b", 64)}},
		"type pattern":     {From: from, To: to, EventTypes: []string{"DROP TABLE"}},
		"from after to":    {From: to, To: from},
		"window 40 days":   {From: from, To: from.Add(40 * 24 * time.Hour)},
	} {
		if _, _, err := ValidateOutbox(q); code(err) != problem.ValidationFailed {
			t.Errorf("outbox %s: %v, want VALIDATION_FAILED", name, err)
		}
	}
	okProc := ProcessedQuery{SourceModule: "learning", From: from, To: to}
	if _, _, err := ValidateProcessed(okProc); err != nil {
		t.Fatalf("valid processed query: %v", err)
	}
	for name, q := range map[string]ProcessedQuery{
		"unknown source module": {SourceModule: "realtime", From: from, To: to},
		"empty source module":   {From: from, To: to},
		"cursor 513":            {SourceModule: "game", From: from, To: to, Cursor: strings.Repeat("a", 513)},
		"cursor not base64":     {SourceModule: "game", From: from, To: to, Cursor: "%%%"},
		"limit 5001":            {SourceModule: "game", From: from, To: to, Limit: 5001},
	} {
		if _, _, err := ValidateProcessed(q); code(err) != problem.ValidationFailed {
			t.Errorf("processed %s: %v, want VALIDATION_FAILED", name, err)
		}
	}
	// a cursor round-trips
	c := encodeProcessed(outbox.ProcessedKey{Consumer: "game.quest_on_activity_graded", EventID: ids.New(), Status: "processed"})
	if len(c) > MaxCursorLen {
		t.Fatalf("cursor %d > %d", len(c), MaxCursorLen)
	}
	if _, _, err := ValidateProcessed(ProcessedQuery{SourceModule: "learning", From: from, To: to, Cursor: c}); err != nil {
		t.Fatalf("round-trip cursor: %v", err)
	}
}

type fakeStore struct {
	outbox    []outbox.OutboxRow
	processed []outbox.ProcessedRow
	names     []string
}

func (f *fakeStore) Outbox(_ context.Context, _, _ time.Time, fromID, toID uuid.UUID, types []string, after uuid.UUID, limit int) ([]outbox.OutboxRow, error) {
	var out []outbox.OutboxRow
	for _, r := range f.outbox {
		if less(r.EventID, fromID) || !less(r.EventID, toID) || !less(after, r.EventID) {
			continue
		}
		if types != nil && !contains(types, r.EventType) {
			continue
		}
		out = append(out, r)
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

func (f *fakeStore) Processed(_ context.Context, _ time.Time, consumers []string, _, _ uuid.UUID, _ outbox.ProcessedKey, limit int) ([]outbox.ProcessedRow, error) {
	f.names = consumers
	if len(f.processed) > limit {
		return f.processed[:limit], nil
	}
	return f.processed, nil
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func jobCtx(t *testing.T, caller string) context.Context {
	t.Helper()
	tenancy.Configure(tenancy.Hooks{Recorder: nopRecorder{}})
	ctx := modcallCtx(caller)
	ps, err := authz.SystemPlatformScope("reconcile")
	if err != nil {
		t.Fatal(err)
	}
	ctx = tenancy.WithScope(ctx, ps)
	ctx, err = tenancy.Bypass(ctx, tenancy.ReasonPlatformJob)
	if err != nil {
		t.Fatal(err)
	}
	return ctx
}

// The facade serves only the reconcile job: an in-process call (pkg/modcall caller), a platform scope and the audited
// platform_job bypass; ids only, keyset pages.
func TestSourceAuthorizationAndPages(t *testing.T) {
	base := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	st := &fakeStore{}
	for i := 0; i < 5; i++ {
		st.outbox = append(st.outbox, outbox.OutboxRow{EventID: ids.Floor(base.Add(time.Duration(i) * time.Minute)), EventType: "activity.graded"})
	}
	src := Source{Module: "learning", Store: st, Consumers: func() []ConsumerDecl {
		return []ConsumerDecl{{Name: "learning.projection_identity", Source: "identity", EventTypes: []string{"tenant.updated"}},
			{Name: "learning.shared", Source: "game", EventTypes: []string{"*"}}, {Name: "learning.shared", Source: "chat", EventTypes: []string{"*"}}}
	}}
	q := OutboxQuery{From: base, To: base.Add(time.Hour), Limit: 2}
	if _, _, err := src.ListOutbox(context.Background(), q); code(err) != problem.CallerNotAllowed {
		t.Fatalf("no modcall caller: %v", err)
	}
	if _, _, err := src.ListOutbox(modcallCtx("game"), q); code(err) != problem.PermissionDenied {
		t.Fatalf("no platform scope: %v", err)
	}
	ps, _ := authz.SystemPlatformScope("reconcile")
	if _, _, err := src.ListOutbox(tenancy.WithScope(modcallCtx("game"), ps), q); code(err) != problem.PermissionDenied {
		t.Fatalf("no bypass: %v", err)
	}
	ctx := jobCtx(t, "game")
	var all []OutboxEvent
	cursor := ""
	for i := 0; i < 5; i++ {
		q.Cursor = cursor
		page, next, err := src.ListOutbox(ctx, q)
		if err != nil {
			t.Fatal(err)
		}
		all = append(all, page...)
		if next == "" {
			break
		}
		cursor = next
	}
	if len(all) != 5 {
		t.Fatalf("pages returned %d events, want 5", len(all))
	}
	cons, _, _, err := src.ListProcessed(ctx, ProcessedQuery{SourceModule: "identity", From: base, To: base.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if len(cons) != 1 || cons[0].Name != "learning.projection_identity" || len(st.names) != 1 {
		t.Fatalf("consumers of identity = %+v (store asked %v)", cons, st.names)
	}
	// a consumer bound to two sources cannot be split by source and is left out
	if cons := src.ConsumersOf("game"); len(cons) != 0 {
		t.Fatalf("shared consumer listed: %+v", cons)
	}
}
