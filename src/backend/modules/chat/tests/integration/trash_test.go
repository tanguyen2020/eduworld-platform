//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/persistence"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/facade"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/contract"
)

// trashPack inserts a sticker pack, soft-deleted at deletedAt when it is not nil (test seed through the superuser: the
// application never hard-deletes and has no delete path for packs yet).
func (e *env) trashPack(code string, deletedAt *time.Time) uuid.UUID {
	e.t.Helper()
	id := uuid.New()
	var by any
	if deletedAt != nil {
		by = uuid.New()
	}
	e.exec(`INSERT INTO chat.sticker_packs (pack_id, code, name_vi, status, created_by, deleted_at, deleted_by)
		VALUES ($1, $2, $3, 'published', $4, $5, $6)`, id, code, "Gói "+code, uuid.New(), deletedAt, by)
	return id
}

// restoredEnvelopes validates and returns the envelopes of the outbox events of type (envelope + payload schema of chat).
func (e *env) restoredEnvelopes(typ string) []map[string]any {
	e.t.Helper()
	rows, err := e.relay.QueryContext(context.Background(), `SELECT envelope FROM infra.outbox_events WHERE event_type = $1`, typ)
	if err != nil {
		e.t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var out []map[string]any
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			e.t.Fatal(err)
		}
		var env map[string]any
		if err := json.Unmarshal(raw, &env); err != nil {
			e.t.Fatal(err)
		}
		if err := contract.CheckEnvelope(env); err != nil {
			e.t.Fatalf("%s envelope: %v", typ, err)
		}
		if err := contract.CheckPayload("chat", "resource.restored", 1, env["payload"]); err != nil {
			e.t.Fatalf("%s payload: %v", typ, err)
		}
		out = append(out, env)
	}
	if err := rows.Err(); err != nil {
		e.t.Fatal(err)
	}
	return out
}

// F057 (callee side, real eduworld_chat, ADR-0028, R8/R10/R21): listTrash lists the soft-deleted sticker packs only,
// newest first, keyset-paged in SQL, with restorable=false when a live pack holds the code; restoreTrashItem clears
// deleted_* under tenancy.Bypass(trash) on chat_platform, audits content.restored + cross_tenant.accessed, writes
// sticker_pack.restored (platform event, schema-checked) and replays a repeated Idempotency-Key; a taken code is 409 and
// changes nothing; the tenant trash (tenant_admin) never lists nor restores a platform pack.
func TestChatTrashFacade(t *testing.T) {
	e := setup(t, envOpts{})
	codec, err := page.NewCodec([]byte(strings.Repeat("t", 32)))
	if err != nil {
		t.Fatal(err)
	}
	f := facade.Facade{Trash: facade.Trash{TrashList: query.Trash{Read: e.read, Reader: persistence.Trash{}, Cursors: codec},
		TrashRestore: command.RestoreTrash{UoW: e.uow, Store: persistence.Trash{}, Events: persistence.Publisher{W: e.writer},
			Audit: persistence.Auditor{R: e.rec}, Now: e.clock.Now}, Idempotency: idempotency.NewStore(e.app, 0, nil)}}
	t1 := time.Date(2026, 11, 20, 1, 0, 0, 0, time.UTC)
	t2, t3 := t1.Add(time.Hour), t1.Add(2*time.Hour)
	older := e.trashPack("mua-he", &t1)
	taken := e.trashPack("trung-thu", &t2)
	e.trashPack("trung-thu", nil) // live pack holding the code of taken
	newest := e.trashPack("tet", &t3)
	live := e.trashPack("live", nil)

	sa := authztest.Ctx(context.Background(), authztest.Platform("system_admin"))
	limit := 2
	res, err := f.ListTrash(sa, api.ListTrashRequestObject{Params: api.ListTrashParams{Limit: &limit}})
	if err != nil {
		t.Fatal(err)
	}
	p1 := res.(api.ListTrash200JSONResponse)
	if len(p1.Data) != 2 || p1.Data[0].ResourceId != newest || p1.Data[1].ResourceId != taken || *p1.Data[1].Restorable || !*p1.Data[0].Restorable ||
		!p1.Page.HasMore || p1.Data[0].DeletedBy == nil || *p1.Data[0].Title != "Gói tet" || p1.Data[0].TenantId != nil {
		t.Fatalf("page 1: %+v", p1)
	}
	res, err = f.ListTrash(sa, api.ListTrashRequestObject{Params: api.ListTrashParams{Limit: &limit, Cursor: p1.Page.NextCursor}})
	if err != nil {
		t.Fatal(err)
	}
	if p2 := res.(api.ListTrash200JSONResponse); len(p2.Data) != 1 || p2.Data[0].ResourceId != older || p2.Page.HasMore {
		t.Fatalf("page 2: %+v", p2)
	}
	// R8: the tenant trash of tenant A never lists the platform library.
	ta := authztest.Ctx(context.Background(), authztest.Staff(e.tenantA, "tenant_admin"))
	if res, err := f.ListTrash(ta, api.ListTrashRequestObject{}); err != nil || len(res.(api.ListTrash200JSONResponse).Data) != 0 {
		t.Fatalf("tenant trash: %+v %v", res, err)
	}

	restore := func(ctx context.Context, key string, id uuid.UUID, reason string) (api.RestoreTrashItemResponseObject, error) {
		return f.RestoreTrashItem(modcall.WithIdempotencyKey(ctx, key), api.RestoreTrashItemRequestObject{ResourceType: "sticker_pack",
			ResourceId: id, Body: &api.RestoreTrashItemJSONRequestBody{Reason: reason}})
	}
	newKey := func() string { return uuid.Must(uuid.NewV7()).String() }
	// 409: a live pack holds the code; nothing changes.
	if _, err := restore(sa, newKey(), taken, "khôi phục gói trung thu"); trashErrCode(err) != problem.Conflict {
		t.Fatalf("taken code: %v", err)
	}
	if n := e.count(`SELECT count(*) FROM chat.sticker_packs WHERE pack_id = $1 AND deleted_at IS NOT NULL`, taken); n != 1 {
		t.Fatal("a refused restore changed the pack")
	}
	// Tenant user: 404, the pack stays in the trash.
	if _, err := restore(ta, newKey(), newest, "khôi phục"); trashErrCode(err) != problem.NotFound {
		t.Fatalf("tenant restore: %v", err)
	}
	key := newKey()
	before := len(e.audits())
	out, err := restore(sa, key, newest, "gói bị xóa nhầm")
	if err != nil {
		t.Fatal(err)
	}
	if r := out.(api.RestoreTrashItem200JSONResponse); r.ResourceId != newest || r.ResourceType != "sticker_pack" {
		t.Fatalf("restore: %+v", r)
	}
	if n := e.count(`SELECT count(*) FROM chat.sticker_packs WHERE pack_id = $1 AND deleted_at IS NULL AND deleted_by IS NULL`, newest); n != 1 {
		t.Fatal("pack not restored")
	}
	acts := e.audits()[before:]
	if !slices.Contains(acts, command.AuditContentRestored) || !slices.Contains(acts, "cross_tenant.accessed") {
		t.Fatalf("audits: %v", acts)
	}
	evs := e.restoredEnvelopes("sticker_pack.restored")
	if len(evs) != 1 || evs[0]["tenant_id"] != nil || evs[0]["payload"].(map[string]any)["resource_id"] != newest.String() {
		t.Fatalf("events: %+v", evs)
	}
	// Same Idempotency-Key: the stored result, no second event; another body with the key: 409 IDEMPOTENCY_KEY_REUSED.
	if again, err := restore(sa, key, newest, "gói bị xóa nhầm"); err != nil || again.(api.RestoreTrashItem200JSONResponse).ResourceId != newest {
		t.Fatalf("replay: %+v %v", again, err)
	}
	if _, err := restore(sa, key, newest, "lý do khác"); trashErrCode(err) != problem.IdempotencyKeyReused {
		t.Fatalf("key reused: %v", err)
	}
	if len(e.restoredEnvelopes("sticker_pack.restored")) != 1 {
		t.Fatal("a replay wrote a second event")
	}
	// A new key on a live pack: not in the trash.
	for _, id := range []uuid.UUID{newest, live, uuid.New()} {
		if _, err := restore(sa, newKey(), id, "khôi phục"); trashErrCode(err) != problem.NotFound {
			t.Fatalf("not in trash %s: %v", id, err)
		}
	}
	// The restored pack left the trash.
	res, err = f.ListTrash(sa, api.ListTrashRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range res.(api.ListTrash200JSONResponse).Data {
		if it.ResourceId == newest || it.ResourceId == live {
			t.Fatalf("live pack listed: %+v", it)
		}
	}
}

func trashErrCode(err error) problem.Code {
	var pe *problem.Error
	if errors.As(err, &pe) {
		return pe.Code
	}
	return ""
}
