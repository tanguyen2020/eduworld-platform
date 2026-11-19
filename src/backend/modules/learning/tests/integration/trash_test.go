//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"

	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/facade"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

func listTrash(ctx context.Context, f facade.Facade, tenants ...uuid.UUID) ([]learningapi.CommonInternalCommonV1TrashItem, error) {
	res, err := asIdentity(ctx, "listTrash", "", func(ctx context.Context) (learningapi.ListTrashResponseObject, error) {
		p := learningapi.ListTrashParams{}
		if len(tenants) > 0 {
			p.TenantIds = &tenants
		}
		return f.ListTrash(ctx, learningapi.ListTrashRequestObject{Params: p})
	})
	if err != nil {
		return nil, err
	}
	return res.(learningapi.ListTrash200JSONResponse).Data, nil
}

func restore(ctx context.Context, f facade.Facade, key string, id uuid.UUID, reason string) (learningapi.RestoreTrashItemResponseObject, error) {
	return asIdentity(ctx, "restoreTrashItem", key, func(ctx context.Context) (learningapi.RestoreTrashItemResponseObject, error) {
		return f.RestoreTrashItem(ctx, learningapi.RestoreTrashItemRequestObject{ResourceType: "question", ResourceId: id,
			Body: &learningapi.RestoreTrashItemJSONRequestBody{Reason: reason}})
	})
}

func bypassCount(reason tenancy.BypassReason) float64 {
	return testutil.ToFloat64(tenancy.BypassCounter().WithLabelValues("learning", string(reason)))
}

// F020-AC-24 (TC-CMS-DEL-01, TC-TEN-023): listTrash returns the deleted items of the registered types (tenant
// filter, restorable flag) through tenancy.Bypass(trash) with audit cross_tenant.accessed and
// eduworld_tenant_filter_bypass_total. F020-AC-25: restoreTrashItem clears deleted_*, audits content.restored,
// writes <resource_type>.restored; the same Idempotency-Key replays. F020-AC-26 (TC-CMS-DEL-02): a restore
// that would break a partial unique index is 409 CONFLICT and changes nothing.
func TestTrashListAndRestore(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, true)
	f := e.facadeOf(t, nil)
	sa, saP := systemAdmin(ctx)
	editorA := authztest.Ctx(ctx, authztest.Staff(e.tenantA, "tenant_content_editor"))
	a1 := tenantItem(ctx, t, e, e.tenantA, "dup")
	b1 := tenantItem(ctx, t, e, e.tenantB, "b-1")
	for _, x := range []struct {
		ctx context.Context
		id  uuid.UUID
	}{{editorA, a1}, {authztest.Ctx(ctx, authztest.Staff(e.tenantB, "tenant_content_editor")), b1}} {
		if err := e.softDelete(x.ctx, t, x.id, "dọn dẹp"); err != nil {
			t.Fatal(err)
		}
	}
	// A new live item of A takes the unique code of the deleted one.
	a2 := tenantItem(ctx, t, e, e.tenantA, "dup")

	before := bypassCount(tenancy.ReasonTrash)
	audits := e.audits(ctx, t, "cross_tenant.accessed", nil)
	all, err := listTrash(sa, f)
	if err != nil || len(all) != 2 {
		t.Fatalf("trash (all tenants) = %+v %v", all, err)
	}
	if bypassCount(tenancy.ReasonTrash) <= before || e.audits(ctx, t, "cross_tenant.accessed", nil) <= audits {
		t.Fatal("trash read without bypass metric/audit")
	}
	onlyA, err := listTrash(sa, f, e.tenantA)
	if err != nil || len(onlyA) != 1 || onlyA[0].ResourceId != a1 || onlyA[0].Restorable == nil || *onlyA[0].Restorable ||
		onlyA[0].DeleteReason == nil || *onlyA[0].DeleteReason != "dọn dẹp" || onlyA[0].TenantId == nil || *onlyA[0].TenantId != e.tenantA {
		t.Fatalf("trash of A = %+v %v", onlyA, err)
	}
	// Tenant admin of B: tenant_ids is ignored, only B's trash.
	adminB := authztest.Ctx(ctx, authztest.Staff(e.tenantB, "tenant_admin"))
	mine, err := listTrash(adminB, f, e.tenantA)
	if err != nil || len(mine) != 1 || mine[0].ResourceId != b1 || !*mine[0].Restorable {
		t.Fatalf("trash of tenant admin B = %+v %v", mine, err)
	}
	// Ordinary reads still do not see deleted items (the bypass is scoped to the trash use case).
	if visibleTo(ctx, t, e, e.tenantA, a1) {
		t.Fatal("deleted item visible")
	}

	// F020-AC-26: restoring a1 conflicts with a2 → 409, nothing changes.
	if _, err := restore(sa, f, newKey(), a1, "khôi phục nhầm"); code(err) != problem.Conflict {
		t.Fatalf("restore with unique conflict: %v", err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM content.fixture_contents WHERE fixture_content_id = $1 AND deleted_at IS NOT NULL`, a1); n != 1 {
		t.Fatal("conflicting restore changed the row")
	}
	// Delete a2, then restore a1 (twice with the same key: the result is replayed).
	if err := e.softDelete(editorA, t, a2, "thay thế"); err != nil {
		t.Fatal(err)
	}
	key := newKey()
	r1, err := restore(sa, f, key, a1, "khôi phục bản gốc")
	if err != nil {
		t.Fatal(err)
	}
	r2, err := restore(sa, f, key, a1, "khôi phục bản gốc")
	if err != nil || r1.(learningapi.RestoreTrashItem200JSONResponse).RestoredAt != r2.(learningapi.RestoreTrashItem200JSONResponse).RestoredAt {
		t.Fatalf("replay = %+v %+v %v", r1, r2, err)
	}
	if !visibleTo(ctx, t, e, e.tenantA, a1) {
		t.Fatal("restored item not visible")
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM content.fixture_contents WHERE fixture_content_id = $1
		AND deleted_at IS NULL AND deleted_by IS NULL AND delete_reason IS NULL`, a1); n != 1 {
		t.Fatal("deleted_* not cleared")
	}
	if n := e.audits(ctx, t, "content.restored", &e.tenantA); n != 1 {
		t.Fatalf("audit content.restored = %d (replay must not audit again)", n)
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'question.restored' AND tenant_id = $1
		AND envelope->'payload'->>'restored_by' = $2`, e.tenantA, saP.UserID.String()); n != 1 {
		t.Fatalf("event question.restored = %d", n)
	}
	// Not in the trash → 404; tenant admin B cannot restore A's item.
	if _, err := restore(sa, f, newKey(), a1, "again!"); code(err) != problem.NotFound {
		t.Fatalf("restore of a live item: %v", err)
	}
	if _, err := restore(adminB, f, newKey(), a2, "not mine"); code(err) != problem.NotFound {
		t.Fatalf("tenant admin B restoring A's item: %v", err)
	}
	// Tenant admin B restores its own item.
	if _, err := restore(adminB, f, newKey(), b1, "của trường B"); err != nil {
		t.Fatalf("tenant admin B restoring its item: %v", err)
	}
	// Without trash permissions.
	if _, err := listTrash(authztest.Ctx(ctx, authztest.Platform("content_admin")), f); code(err) != problem.PermissionDenied {
		t.Fatalf("content_admin listing trash: %v", err)
	}
	if _, err := restore(editorA, f, newKey(), a2, "editor"); code(err) != problem.PermissionDenied {
		t.Fatalf("editor restoring: %v", err)
	}
}
