//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func refTenant(ctx context.Context, t *testing.T, e *env, id uuid.UUID) (code, status string, version int64) {
	t.Helper()
	if err := e.migrator.QueryRowContext(ctx, `SELECT coalesce(code, ''), status, source_version FROM ref.tenants WHERE tenant_id = $1`, id).
		Scan(&code, &status, &version); err != nil {
		t.Fatalf("ref.tenants %s: %v", id, err)
	}
	return code, status, version
}

// F020-AC-01 (TC-SVC-003): tenant.created/updated upsert ref.tenants by source_version; an older or equal
// version is ignored. SC-002: the projection catches up within 5 s.
func TestProjectionTenantSourceVersion(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, false)
	p := startProjection(ctx, t, e)
	id := uuid.New()

	start := time.Now()
	created := p.publish(ctx, t, "tenant.created", &id, tenantPayload(id, "th-demo-a", "active", 2))
	p.processed(ctx, t, created)
	if lag := time.Since(start); lag > 5*time.Second {
		t.Fatalf("projection lag %v > 5 s (SC-002)", lag)
	}
	if c, s, v := refTenant(ctx, t, e, id); c != "th-demo-a" || s != "active" || v != 2 {
		t.Fatalf("after created: %s %s %d", c, s, v)
	}

	// Older (1) and equal (2) versions are ignored.
	for _, v := range []int{1, 2} {
		old := p.publish(ctx, t, "tenant.updated", &id, tenantPayload(id, "renamed", "disabled", v))
		p.processed(ctx, t, old)
		if c, s, ver := refTenant(ctx, t, e, id); c != "th-demo-a" || s != "active" || ver != 2 {
			t.Fatalf("version %d applied: %s %s %d", v, c, s, ver)
		}
	}
	// A newer version is applied; a field absent from the payload keeps its value.
	newer := p.publish(ctx, t, "tenant.updated", &id, map[string]any{"tenant_id": id.String(), "tenant_type": "school",
		"status": "disabled", "changed_fields": []string{"status"}, "source_version": 3})
	p.processed(ctx, t, newer)
	if c, s, v := refTenant(ctx, t, e, id); c != "th-demo-a" || s != "disabled" || v != 3 {
		t.Fatalf("after newer: %s %s %d", c, s, v)
	}
}
