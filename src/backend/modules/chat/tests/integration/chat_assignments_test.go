//go:build integration

package integration

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/persistence"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/facade"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// F082 T040 / F082-AC-04 (callee side): the chat facade assigns a published sticker pack to active tenants (students of
// those tenants see it), revokes (soft delete) and lists the assignments with keyset pages; a draft pack is 409
// CONTENT_NOT_PUBLISHED, a GIF is not assignable, a tenant principal or a principal without platform.tenant_data.assign is
// refused; each change is audited.
func TestChatAssignmentsFacade(t *testing.T) {
	f := setupFeatures(t, chatOpts{})
	startFeatures(t, f)
	codec, err := page.NewCodec([]byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatal(err)
	}
	fc := facade.Facade{Assignments: command.ChatAssignments{UoW: f.uow, Store: persistence.Assignments{}, Audit: persistence.Auditor{R: f.rec},
		Cursors: codec, Now: f.clock.Now}}
	ctx := authztest.Ctx(context.Background(), authztest.Platform("system_admin"))
	pub, _ := f.seedPack("published", false)
	draft, _ := f.seedPack("draft", false)
	apply := func(ctx context.Context, rt string, id uuid.UUID, tenants ...uuid.UUID) (api.ApplyTenantAssignmentsResponseObject, error) {
		return fc.ApplyTenantAssignments(ctx, api.ApplyTenantAssignmentsRequestObject{Body: &api.ApplyTenantAssignmentsJSONRequestBody{
			ResourceType: api.CommonInternalCommonV1ResourceType(rt), ResourceId: id, TenantIds: tenants}})
	}
	res, err := apply(ctx, "sticker_pack", pub, f.tenantA, f.tenantB)
	if err != nil {
		t.Fatal(err)
	}
	l := res.(api.ApplyTenantAssignments200JSONResponse)
	if len(l.Data) != 2 || l.Data[0].Status != "active" || l.Data[0].Created == nil || !*l.Data[0].Created {
		t.Fatalf("apply: %+v", l)
	}
	// idempotent: applying again creates nothing new (created = false, F045 Clarifications 2026-11-28).
	if res, err := apply(ctx, "sticker_pack", pub, f.tenantA); err != nil {
		t.Fatal(err)
	} else if d := res.(api.ApplyTenantAssignments200JSONResponse).Data; len(d) != 1 || d[0].Created == nil || *d[0].Created {
		t.Fatalf("apply again: %+v", d)
	}
	if n := f.count(`SELECT count(*) FROM infra.tenant_resource_assignments WHERE resource_id = $1 AND deleted_at IS NULL`, pub); n != 2 {
		t.Fatalf("assignments: %d", n)
	}
	// the students of tenant A see the pack.
	s := f.student(f.tenantA, f.schoolA)
	if r := f.call(s, http.MethodGet, "/chat/stickers", "", nil); r.status != http.StatusOK || !strings.Contains(r.raw, pub.String()) {
		t.Fatalf("student packs: %d %s", r.status, r.raw)
	}
	for name, c := range map[string]struct {
		ctx  context.Context
		rt   string
		id   uuid.UUID
		code problem.Code
	}{
		"draft":         {ctx, "sticker_pack", draft, problem.ContentNotPublished},
		"gif":           {ctx, "gif", pub, problem.ValidationFailed},
		"tenant admin":  {authztest.Ctx(context.Background(), authztest.Staff(f.tenantA, "tenant_admin")), "sticker_pack", pub, problem.PermissionDenied},
		"game designer": {authztest.Ctx(context.Background(), authztest.Platform("game_designer")), "sticker_pack", pub, problem.PermissionDenied},
	} {
		_, err := apply(c.ctx, c.rt, c.id, f.tenantA)
		var pe *problem.Error
		if !errors.As(err, &pe) || pe.Code != c.code {
			t.Errorf("%s: %v", name, err)
		}
	}
	// list: keyset pages of one tenant each.
	one := 1
	lp, err := fc.ListResourceAssignments(ctx, api.ListResourceAssignmentsRequestObject{Params: api.ListResourceAssignmentsParams{ResourceType: "sticker_pack",
		ResourceId: pub, Limit: &one}})
	if err != nil {
		t.Fatal(err)
	}
	p1 := lp.(api.ListResourceAssignments200JSONResponse)
	if len(p1.Data) != 1 || !p1.Page.HasMore || p1.Page.NextCursor == nil {
		t.Fatalf("page 1: %+v", p1)
	}
	lp2, _ := fc.ListResourceAssignments(ctx, api.ListResourceAssignmentsRequestObject{Params: api.ListResourceAssignmentsParams{ResourceType: "sticker_pack",
		ResourceId: pub, Limit: &one, Cursor: p1.Page.NextCursor}})
	if p2 := lp2.(api.ListResourceAssignments200JSONResponse); len(p2.Data) != 1 || p2.Page.HasMore || p2.Data[0].TenantId == p1.Data[0].TenantId {
		t.Fatalf("page 2: %+v", p2)
	}
	// revoke tenant A (soft delete): the student no longer sees the pack.
	rv, err := fc.RevokeTenantAssignments(ctx, api.RevokeTenantAssignmentsRequestObject{Body: &api.RevokeTenantAssignmentsJSONRequestBody{
		ResourceType: "sticker_pack", ResourceId: pub, TenantIds: []uuid.UUID{f.tenantA}, Reason: "Hết mùa lễ hội"}})
	if err != nil || len(rv.(api.RevokeTenantAssignments200JSONResponse).Data) != 1 {
		t.Fatalf("revoke: %v", err)
	}
	if n := f.count(`SELECT count(*) FROM infra.tenant_resource_assignments WHERE resource_id = $1 AND tenant_id = $2 AND deleted_at IS NOT NULL`,
		pub, f.tenantA); n != 1 {
		t.Fatal("revoke is not a soft delete")
	}
	if r := f.call(s, http.MethodGet, "/chat/stickers", "", nil); strings.Contains(r.raw, pub.String()) {
		t.Fatalf("revoked pack still listed: %s", r.raw)
	}
	if a := f.audits(); !containsAudit(a, command.AuditTenantDataAssigned) || !containsAudit(a, command.AuditTenantDataUnassigned) {
		t.Fatalf("audits: %v", a)
	}
	// not wired: 503.
	if _, err := (facade.Facade{}).ApplyTenantAssignments(ctx, api.ApplyTenantAssignmentsRequestObject{}); err == nil {
		t.Fatal("not wired must fail")
	}
}
