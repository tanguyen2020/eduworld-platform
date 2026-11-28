//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/facade"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/contract"
)

// trashItem inserts a console item of itemType (soft-deleted at deletedAt when not nil) and, with title, one draft version
// carrying it (test seed through game_migrator: the application never hard-deletes and has no delete path for console
// items yet).
func (e *env) trashItem(ctx context.Context, t *testing.T, itemType, key, title string, deletedAt *time.Time) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	var by any
	if deletedAt != nil {
		by = uuid.New()
	}
	if _, err := e.migrator.ExecContext(ctx, `INSERT INTO game.config_items (config_id, created_at, updated_at, item_type, item_key, latest_version_no,
		deleted_at, deleted_by, delete_reason) VALUES ($1, now(), now(), $2, $3, 1, $4, $5, CASE WHEN $4::timestamptz IS NULL THEN NULL ELSE 'nhầm' END)`,
		id, itemType, key, deletedAt, by); err != nil {
		t.Fatal(err)
	}
	if title != "" {
		if _, err := e.migrator.ExecContext(ctx, `INSERT INTO game.config_versions (config_version_id, created_at, updated_at, config_id, item_type,
			version_no, status, title, payload, payload_sha256) VALUES ($1, now(), now(), $2, $3, 1, 'draft', $4, '{}'::jsonb, $5)`,
			uuid.New(), id, itemType, title, strings.Repeat("0", 64)); err != nil {
			t.Fatal(err)
		}
	}
	return id
}

// F057 (callee side, real eduworld_game, ADR-0028, R8/R10/R21): listTrash lists only soft-deleted console items of the
// requested game resource types, newest first, keyset-paged in SQL (title = latest version, else the key; restorable=false
// when a live item holds the key); restoreTrashItem clears deleted_* under tenancy.Bypass(trash) on game_platform, audits
// content.restored + cross_tenant.accessed, writes <resource_type>.restored (platform event, schema-checked) and replays a
// repeated Idempotency-Key; a taken key is 409 and changes nothing; the tenant trash never lists nor restores them.
func TestGameTrashFacade(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	codec, err := page.NewCodec([]byte(strings.Repeat("g", 32)))
	if err != nil {
		t.Fatal(err)
	}
	f := facade.Facade{Trash: facade.Trash{TrashList: query.Trash{Read: e.read, Model: read.Trash{}, Cursors: codec},
		TrashRestore: command.RestoreTrash{UoW: e.uow, Store: write.TrashRepo{}, Events: write.Publisher{W: e.writer},
			Audit: write.Auditor{R: e.rec}, Now: e.clock.Now}, Idempotency: idempotency.NewStore(e.app, 0, nil)}}
	t0 := time.Date(2026, 11, 20, 1, 0, 0, 0, time.UTC)
	at := func(h int) *time.Time { v := t0.Add(time.Duration(h) * time.Hour); return &v }
	q1 := e.trashItem(ctx, t, "quest", "trash.q1", "Bài toán phiên chợ", at(0))
	q2 := e.trashItem(ctx, t, "quest", "trash.q2", "", at(1))
	shop := e.trashItem(ctx, t, "shop_item", "trash.shop", "Mũ len", at(2))
	tmpl := e.trashItem(ctx, t, "event_template", "trash.tpl", "", at(3))
	e.trashItem(ctx, t, "event_template", "trash.tpl", "", nil) // live item holding the key of tmpl
	liveQuest := e.trashItem(ctx, t, "quest", "trash.live", "", nil)
	e.trashItem(ctx, t, "npc_dialogue", "trash.npc", "", at(4)) // not a trash type of the contract

	sa := authztest.Ctx(ctx, authztest.Platform("system_admin"))
	limit := 3
	res, err := f.ListTrash(sa, api.ListTrashRequestObject{Params: api.ListTrashParams{Limit: &limit}})
	if err != nil {
		t.Fatal(err)
	}
	p1 := res.(api.ListTrash200JSONResponse)
	if len(p1.Data) != 3 || p1.Data[0].ResourceId != tmpl || *p1.Data[0].Restorable || p1.Data[1].ResourceId != shop || *p1.Data[1].Title != "Mũ len" ||
		p1.Data[2].ResourceId != q2 || *p1.Data[2].Title != "trash.q2" || !p1.Page.HasMore || *p1.Data[1].DeleteReason != "nhầm" || p1.Data[1].DeletedBy == nil {
		t.Fatalf("page 1: %+v", p1)
	}
	res, err = f.ListTrash(sa, api.ListTrashRequestObject{Params: api.ListTrashParams{Limit: &limit, Cursor: p1.Page.NextCursor}})
	if err != nil {
		t.Fatal(err)
	}
	if p2 := res.(api.ListTrash200JSONResponse); len(p2.Data) != 1 || p2.Data[0].ResourceId != q1 || *p2.Data[0].Title != "Bài toán phiên chợ" || p2.Page.HasMore {
		t.Fatalf("page 2: %+v", p2)
	}
	rt := api.CommonInternalCommonV1ResourceType("quest")
	if res, err := f.ListTrash(sa, api.ListTrashRequestObject{Params: api.ListTrashParams{ResourceType: &rt}}); err != nil ||
		len(res.(api.ListTrash200JSONResponse).Data) != 2 {
		t.Fatalf("quests: %+v %v", res, err)
	}
	ta := authztest.Ctx(ctx, authztest.Staff(e.tenantA, "tenant_admin"))
	if res, err := f.ListTrash(ta, api.ListTrashRequestObject{}); err != nil || len(res.(api.ListTrash200JSONResponse).Data) != 0 {
		t.Fatalf("tenant trash: %+v %v", res, err)
	}

	newKey := func() string { return uuid.Must(uuid.NewV7()).String() }
	restore := func(c context.Context, key, rt string, id uuid.UUID, reason string) (api.RestoreTrashItemResponseObject, error) {
		return f.RestoreTrashItem(modcall.WithIdempotencyKey(c, key), api.RestoreTrashItemRequestObject{ResourceType: api.CommonInternalCommonV1ResourceTypePath(rt),
			ResourceId: id, Body: &api.RestoreTrashItemJSONRequestBody{Reason: reason}})
	}
	if _, err := restore(sa, newKey(), "event_template", tmpl, "khôi phục mẫu"); codeOf(err) != problem.Conflict {
		t.Fatalf("taken key: %v", err)
	}
	if _, err := restore(sa, newKey(), "quest", shop, "khôi phục"); codeOf(err) != problem.NotFound { // the item is not a quest
		t.Fatalf("wrong type: %v", err)
	}
	if _, err := restore(ta, newKey(), "shop_item", shop, "khôi phục"); codeOf(err) != problem.NotFound {
		t.Fatalf("tenant restore: %v", err)
	}
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM game.config_items WHERE config_id = ANY($1::uuid[]) AND deleted_at IS NOT NULL`,
		[]string{tmpl.String(), shop.String()}); n != 2 {
		t.Fatal("a refused restore changed an item")
	}
	restoredBefore, crossBefore := e.audits(ctx, t, command.ActionContentRestored), e.audits(ctx, t, "cross_tenant.accessed")
	key := newKey()
	out, err := restore(sa, key, "shop_item", shop, "mũ bị xóa nhầm")
	if err != nil {
		t.Fatal(err)
	}
	if r := out.(api.RestoreTrashItem200JSONResponse); r.ResourceId != shop || r.ResourceType != "shop_item" || !r.RestoredAt.Equal(e.clock.Now()) {
		t.Fatalf("restore: %+v", r)
	}
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM game.config_items WHERE config_id = $1 AND deleted_at IS NULL AND deleted_by IS NULL
		AND delete_reason IS NULL AND version = 2`, shop); n != 1 {
		t.Fatal("item not restored")
	}
	if e.audits(ctx, t, command.ActionContentRestored) != restoredBefore+1 || e.audits(ctx, t, "cross_tenant.accessed") <= crossBefore {
		t.Fatal("audit content.restored / cross_tenant.accessed")
	}
	checkRestored := func() int {
		rows, err := e.relay.QueryContext(ctx, `SELECT envelope FROM infra.outbox_events WHERE event_type = 'shop_item.restored'`)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = rows.Close() }()
		n := 0
		for rows.Next() {
			var raw []byte
			if err := rows.Scan(&raw); err != nil {
				t.Fatal(err)
			}
			var env map[string]any
			_ = json.Unmarshal(raw, &env)
			if err := contract.CheckEnvelope(env); err != nil || env["tenant_id"] != nil {
				t.Fatalf("envelope: %v %v", err, env["tenant_id"])
			}
			if err := contract.CheckPayload("game", "resource.restored", 1, env["payload"]); err != nil {
				t.Fatalf("payload: %v", err)
			}
			n++
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if checkRestored() != 1 {
		t.Fatal("shop_item.restored")
	}
	if again, err := restore(sa, key, "shop_item", shop, "mũ bị xóa nhầm"); err != nil || again.(api.RestoreTrashItem200JSONResponse).ResourceId != shop {
		t.Fatalf("replay: %+v %v", again, err)
	}
	if _, err := restore(sa, key, "shop_item", shop, "lý do khác"); codeOf(err) != problem.IdempotencyKeyReused {
		t.Fatalf("key reused: %v", err)
	}
	if checkRestored() != 1 {
		t.Fatal("a replay wrote a second event")
	}
	for _, c := range []struct {
		rt string
		id uuid.UUID
	}{{"shop_item", shop}, {"quest", liveQuest}, {"quest", uuid.New()}} {
		if _, err := restore(sa, newKey(), c.rt, c.id, "khôi phục"); codeOf(err) != problem.NotFound {
			t.Fatalf("not in trash %s: %v", c.id, err)
		}
	}
}
