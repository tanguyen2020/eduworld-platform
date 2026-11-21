//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	identityapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/facade"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// eventProperties returns the payload properties of an identity event schema.
func eventProperties(t *testing.T, event string) map[string]bool {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(testkit.RepoRoot(t), "specs", "events", "payloads", "identity", event+".v1.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var s struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for k := range s.Properties {
		out[k] = true
	}
	return out
}

// F010-AC-27: listProjectionSnapshot pages the current state with the event payload fields only,
// source_version, the deleted flag, one tenant or every tenant (platform job).
func TestListProjectionSnapshot(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	a := seedTenant(ctx, t, e.platform, "pgd-sa")
	b := seedTenant(ctx, t, e.platform, "pgd-sb")
	enrolled(ctx, t, e.platform, a, "hs.snap.a")
	if _, err := e.platform.ExecContext(ctx, `UPDATE org.classes SET deleted_at = now() WHERE class_id = $1 AND tenant_id = $2 AND deleted_at IS NULL`, a.otherClass, a.tenant); err != nil {
		t.Fatal(err)
	}
	codec, _ := page.NewCodec([]byte(strings.Repeat("k", 32)))
	f := facade.Facade{Snapshot: query.ProjectionSnapshot{Model: read.SnapshotModel{Session: e.read}, Cursors: codec}}
	call := func(entity string, tenant *uuid.UUID, cursor *string, limit int) identityapi.ListProjectionSnapshot200JSONResponse {
		t.Helper()
		p := identityapi.ListProjectionSnapshotParams{Cursor: cursor, Limit: &limit}
		if tenant != nil {
			tid := *tenant
			p.TenantId = &tid
		}
		resp, err := f.ListProjectionSnapshot(ctx, identityapi.ListProjectionSnapshotRequestObject{
			EntityType: identityapi.ListProjectionSnapshotParamsEntityType(entity), Params: p})
		if err != nil {
			t.Fatalf("%s: %v", entity, err)
		}
		return resp.(identityapi.ListProjectionSnapshot200JSONResponse)
	}
	ta := uuid.MustParse(a.tenant)

	// One tenant, stable paging by id: 2 classes of A in pages of 1, the deleted one flagged.
	p1 := call("class", &ta, nil, 1)
	if len(p1.Items) != 1 || !p1.Page.HasMore {
		t.Fatalf("page 1 = %+v", p1)
	}
	p2 := call("class", &ta, p1.Page.NextCursor, 1)
	if len(p2.Items) != 1 || p2.Page.HasMore || p2.Items[0].EntityId == p1.Items[0].EntityId {
		t.Fatalf("page 2 = %+v", p2)
	}
	deleted := 0
	for _, it := range append(p1.Items, p2.Items...) {
		if uuid.UUID(it.TenantId) != ta {
			t.Fatalf("item of another tenant: %+v", it)
		}
		if it.Deleted {
			deleted++
		}
	}
	if deleted != 1 {
		t.Fatalf("deleted classes = %d, want 1", deleted)
	}

	// Every entity type: data keys are fields of the projection event payload (no PII beyond them).
	events := map[string]string{"tenant": "tenant.updated", "school": "school.updated", "class": "class.synced",
		"student": "student.synced", "enrollment": "enrollment.changed"}
	for entity, event := range events {
		props := eventProperties(t, event)
		resp := call(entity, &ta, nil, 100)
		if len(resp.Items) == 0 {
			t.Fatalf("%s: no items", entity)
		}
		for _, it := range resp.Items {
			if it.SourceVersion < 1 {
				t.Fatalf("%s: source_version %d", entity, it.SourceVersion)
			}
			for k := range it.Data {
				if !props[k] {
					t.Errorf("%s: field %q is not in %s payload", entity, k, event)
				}
			}
		}
	}
	// F015 T044: the tenant snapshot carries settings_subset as tenant.updated publishes it (rebuilt projection).
	if _, err := e.platform.ExecContext(ctx, `UPDATE platform.tenants SET settings = '{"cross_school_social_enabled": true}' WHERE tenant_id = $1`, a.tenant); err != nil {
		t.Fatal(err)
	}
	if ts := call("tenant", &ta, nil, 1); len(ts.Items) != 1 || fmt.Sprint(ts.Items[0].Data["settings_subset"]) !=
		"map[common_social_enabled:false cross_school_social_enabled:true]" {
		t.Fatalf("tenant snapshot settings_subset = %+v", ts.Items)
	}
	// Platform scope (no tenant_id): tenants A, B and the seeded common tenant.
	all := call("tenant", nil, nil, 1000)
	seen := map[string]bool{}
	for _, it := range all.Items {
		seen[uuid.UUID(it.EntityId).String()] = true
	}
	if !seen[a.tenant] || !seen[b.tenant] || len(all.Items) < 3 {
		t.Fatalf("platform snapshot of tenants = %d items", len(all.Items))
	}
	// Limit above 1000 is rejected.
	big := 1001
	if _, err := f.ListProjectionSnapshot(ctx, identityapi.ListProjectionSnapshotRequestObject{EntityType: "school",
		Params: identityapi.ListProjectionSnapshotParams{Limit: &big}}); code(err) != "VALIDATION_FAILED" {
		t.Fatalf("limit 1001: %v", err)
	}
}
