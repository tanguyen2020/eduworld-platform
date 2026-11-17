package tenancy_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

func TestZeroScopesAreInvalid(t *testing.T) {
	for _, s := range []tenancy.Scope{tenancy.TenantScope{}, tenancy.CrossTenantScope{}, tenancy.PlatformScope{}} {
		if s.Valid() {
			t.Fatalf("zero %T must be invalid", s)
		}
	}
}

func TestScopeContext(t *testing.T) {
	a := uuid.New()
	ctx := tenancy.WithScope(context.Background(), tenancy.NewTenantScopeForAuthz(a))
	s, ok := tenancy.ScopeFrom(ctx)
	if !ok || !s.Valid() {
		t.Fatal("scope not found")
	}
	ids := s.TenantIDs()
	if len(ids) != 1 || ids[0] != a {
		t.Fatalf("TenantIDs = %v", ids)
	}
	if id, ok := s.SessionTenantID(); !ok || id != a {
		t.Fatal("SessionTenantID")
	}
	if _, ok := tenancy.ScopeFrom(context.Background()); ok {
		t.Fatal("empty context has a scope")
	}
}

func TestPlatformScopeSeesOnlyNullTenant(t *testing.T) {
	s := tenancy.NewPlatformScopeForAuthz("job:rollup")
	if !s.Valid() || !s.PlatformOnly() || len(s.TenantIDs()) != 0 {
		t.Fatalf("platform scope: %+v", s)
	}
	if _, ok := s.SessionTenantID(); ok {
		t.Fatal("platform scope must not set a session tenant")
	}
}

type recorded struct{ events []audit.Event }

func (r *recorded) Record(_ context.Context, e audit.Event) error {
	r.events = append(r.events, e)
	return nil
}

// F002-AC-12: closed reason list, audit cross_tenant.accessed and metric per reason.
func TestBypass(t *testing.T) {
	rec := &recorded{}
	reg := prometheus.NewRegistry()
	tenancy.Configure(tenancy.Hooks{Recorder: rec, Registerer: reg})
	t.Cleanup(func() { tenancy.Configure(tenancy.Hooks{}) })

	ctx := tenancy.WithModule(context.Background(), "chat")
	if _, err := tenancy.Bypass(ctx, ""); !errors.Is(err, tenancy.ErrBypassReason) {
		t.Fatalf("empty reason: %v", err)
	}
	if _, err := tenancy.Bypass(ctx, tenancy.BypassReason("debug")); !errors.Is(err, tenancy.ErrBypassReason) {
		t.Fatalf("unknown reason: %v", err)
	}
	if len(rec.events) != 0 {
		t.Fatal("rejected bypass must not be audited as access")
	}

	tid := uuid.New()
	bctx, err := tenancy.Bypass(ctx, tenancy.ReasonModerationCase, tenancy.WithTenantIDs(tid), tenancy.WithMode(tenancy.ModeRead))
	if err != nil {
		t.Fatal(err)
	}
	if r, ok := tenancy.Bypassed(bctx); !ok || r != tenancy.ReasonModerationCase {
		t.Fatal("Bypassed not set")
	}
	if len(rec.events) != 1 {
		t.Fatalf("events = %d", len(rec.events))
	}
	e := rec.events[0]
	if e.Action != audit.ActionCrossTenantAccessed || e.Metadata["reason"] != "moderation_case" || e.Metadata["mode"] != "read" {
		t.Fatalf("audit = %+v", e)
	}
	if ids, _ := e.Metadata["tenant_ids"].([]string); len(ids) != 1 || ids[0] != tid.String() {
		t.Fatalf("tenant_ids = %v", e.Metadata["tenant_ids"])
	}
	n := testutil.ToFloat64(tenancy.BypassCounter().WithLabelValues("chat", "moderation_case"))
	if n != 1 {
		t.Fatalf("metric = %v", n)
	}
}

func TestBypassFailsClosedWhenAuditFails(t *testing.T) {
	tenancy.Configure(tenancy.Hooks{Recorder: audit.RecorderFunc(func(context.Context, audit.Event) error {
		return errors.New("outbox down")
	})})
	t.Cleanup(func() { tenancy.Configure(tenancy.Hooks{}) })
	if _, err := tenancy.Bypass(context.Background(), tenancy.ReasonTrash); err == nil {
		t.Fatal("bypass allowed although audit failed")
	}
}

func TestAllReasonsListed(t *testing.T) {
	want := []string{"system_admin_cross_tenant", "moderation_case", "break_glass", "trash", "auth_login", "tenant_clone",
		"inactive_cleanup", "re_evaluation", "efficacy_study", "platform_job", "platform_analytics"}
	got := map[string]bool{}
	for _, r := range tenancy.Reasons() {
		got[string(r)] = true
	}
	for _, w := range want {
		if !got[w] {
			t.Fatalf("reason %s missing (ADR-0089 §3)", w)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("unexpected reasons: %v", tenancy.Reasons())
	}
}
