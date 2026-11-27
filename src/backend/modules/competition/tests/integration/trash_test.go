//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/persistence"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/facade"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/contract"
)

// trashTemplate inserts a template configuration of configType (soft-deleted at deletedAt when not nil) with one draft
// version titled title (seed through the superuser: the application never hard-deletes and has no delete path for
// templates yet).
func (e *penv) trashTemplate(configType, code, title string, deletedAt *time.Time) uuid.UUID {
	e.t.Helper()
	id := uuid.Must(uuid.NewV7())
	var by any
	if deletedAt != nil {
		by = uuid.New()
	}
	e.exec(`INSERT INTO competition.game_configs (config_id, config_type, code, latest_version_no, deleted_at, deleted_by)
		VALUES ($1, $2, $3, 1, $4, $5)`, id, configType, code, deletedAt, by)
	e.exec(`INSERT INTO competition.game_config_versions (config_version_id, config_id, version_no, status, title, payload)
		VALUES ($1, $2, 1, 'draft', $3, '{}'::jsonb)`, uuid.New(), id, title)
	return id
}

func trashErr(err error) problem.Code {
	var pe *problem.Error
	if errors.As(err, &pe) {
		return pe.Code
	}
	return ""
}

// F057 (callee side, real eduworld_competition, ADR-0028, R8/R10/R21): listTrash lists only soft-deleted tournament and
// challenge templates (never another config type), newest first, keyset-paged in SQL, restorable=false when a live
// configuration holds the code; restoreTrashItem clears deleted_* under tenancy.Bypass(trash) on competition_platform,
// audits content.restored + cross_tenant.accessed, writes <resource_type>.restored (platform event, schema-checked) and
// replays a repeated Idempotency-Key; a taken code is 409 and changes nothing; the tenant trash never sees templates.
func TestCompetitionTrashFacade(t *testing.T) {
	e := setupParty(t, popts{})
	f := facade.Facade{TrashList: &query.Trash{Read: e.read, Reader: persistence.TrashStore{}},
		TrashRestore: &command.RestoreTrash{UoW: e.uow, Store: persistence.TrashStore{}, Events: e.parties.Events, Audit: e.parties.Audit,
			Now: e.clock.Now},
		Idempotency: idempotency.NewStore(e.app, 0, nil)}
	t0 := time.Date(2026, 11, 20, 1, 0, 0, 0, time.UTC)
	at := func(h int) *time.Time { v := t0.Add(time.Duration(h) * time.Hour); return &v }
	older := e.trashTemplate("tournament-templates", "trash-cup", "Cúp mùa thu", at(0))
	taken := e.trashTemplate("challenge-templates", "trash-sprint", "Chạy nước rút", at(1))
	e.trashTemplate("challenge-templates", "trash-sprint", "Chạy nước rút mới", nil) // live configuration holding the code
	newest := e.trashTemplate("tournament-templates", "trash-winter", "Cúp mùa đông", at(2))
	e.trashTemplate("competition-rewards", "trash-rewards", "Phần thưởng", at(3)) // not a trash type of the contract

	sa := authztest.Ctx(context.Background(), authztest.Platform("system_admin"))
	limit := 2
	res, err := f.ListTrash(sa, api.ListTrashRequestObject{Params: api.ListTrashParams{Limit: &limit}})
	if err != nil {
		t.Fatal(err)
	}
	p1 := res.(api.ListTrash200JSONResponse)
	if len(p1.Data) != 2 || p1.Data[0].ResourceId != newest || p1.Data[0].ResourceType != "tournament_template" || *p1.Data[0].Title != "Cúp mùa đông" ||
		p1.Data[1].ResourceId != taken || p1.Data[1].ResourceType != "challenge_template" || *p1.Data[1].Restorable || !p1.Page.HasMore {
		t.Fatalf("page 1: %+v", p1)
	}
	res, err = f.ListTrash(sa, api.ListTrashRequestObject{Params: api.ListTrashParams{Limit: &limit, Cursor: p1.Page.NextCursor}})
	if err != nil {
		t.Fatal(err)
	}
	if p2 := res.(api.ListTrash200JSONResponse); len(p2.Data) != 1 || p2.Data[0].ResourceId != older || p2.Page.HasMore {
		t.Fatalf("page 2: %+v", p2)
	}
	rt := api.CommonInternalCommonV1ResourceType("challenge_template")
	if res, err := f.ListTrash(sa, api.ListTrashRequestObject{Params: api.ListTrashParams{ResourceType: &rt}}); err != nil ||
		len(res.(api.ListTrash200JSONResponse).Data) != 1 {
		t.Fatalf("challenge templates: %+v %v", res, err)
	}
	ta := authztest.Ctx(context.Background(), authztest.Staff(e.tenantA, "tenant_admin"))
	if res, err := f.ListTrash(ta, api.ListTrashRequestObject{}); err != nil || len(res.(api.ListTrash200JSONResponse).Data) != 0 {
		t.Fatalf("tenant trash: %+v %v", res, err)
	}

	newKey := func() string { return uuid.Must(uuid.NewV7()).String() }
	restore := func(c context.Context, key, rt string, id uuid.UUID, reason string) (api.RestoreTrashItemResponseObject, error) {
		return f.RestoreTrashItem(modcall.WithIdempotencyKey(c, key), api.RestoreTrashItemRequestObject{
			ResourceType: api.CommonInternalCommonV1ResourceTypePath(rt), ResourceId: id, Body: &api.RestoreTrashItemJSONRequestBody{Reason: reason}})
	}
	if _, err := restore(sa, newKey(), "challenge_template", taken, "khôi phục mẫu"); trashErr(err) != problem.Conflict {
		t.Fatalf("taken code: %v", err)
	}
	if _, err := restore(sa, newKey(), "challenge_template", newest, "khôi phục"); trashErr(err) != problem.NotFound { // a tournament template
		t.Fatalf("wrong type: %v", err)
	}
	if _, err := restore(ta, newKey(), "tournament_template", newest, "khôi phục"); trashErr(err) != problem.NotFound {
		t.Fatalf("tenant restore: %v", err)
	}
	if n := e.count(`SELECT count(*) FROM competition.game_configs WHERE config_id = ANY($1::uuid[]) AND deleted_at IS NOT NULL`,
		[]string{taken.String(), newest.String()}); n != 2 {
		t.Fatal("a refused restore changed a template")
	}
	restoredBefore, crossBefore := len(e.audits(command.ActionContentRestored)), len(e.audits("cross_tenant.accessed"))
	key := newKey()
	out, err := restore(sa, key, "tournament_template", newest, "mẫu bị xóa nhầm")
	if err != nil {
		t.Fatal(err)
	}
	if r := out.(api.RestoreTrashItem200JSONResponse); r.ResourceId != newest || r.ResourceType != "tournament_template" {
		t.Fatalf("restore: %+v", r)
	}
	if n := e.count(`SELECT count(*) FROM competition.game_configs WHERE config_id = $1 AND deleted_at IS NULL AND deleted_by IS NULL
		AND version = 2`, newest); n != 1 {
		t.Fatal("template not restored")
	}
	if len(e.audits(command.ActionContentRestored)) != restoredBefore+1 || len(e.audits("cross_tenant.accessed")) <= crossBefore {
		t.Fatal("audit content.restored / cross_tenant.accessed")
	}
	restored := func() int {
		rows, err := e.relay.QueryContext(context.Background(), `SELECT envelope FROM infra.outbox_events WHERE event_type = 'tournament_template.restored'`)
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
			if err := contract.CheckPayload("competition", "resource.restored", 1, env["payload"]); err != nil {
				t.Fatalf("payload: %v", err)
			}
			n++
		}
		return n
	}
	if restored() != 1 {
		t.Fatal("tournament_template.restored")
	}
	if again, err := restore(sa, key, "tournament_template", newest, "mẫu bị xóa nhầm"); err != nil ||
		again.(api.RestoreTrashItem200JSONResponse).ResourceId != newest {
		t.Fatalf("replay: %+v %v", again, err)
	}
	if _, err := restore(sa, key, "tournament_template", newest, "lý do khác"); trashErr(err) != problem.IdempotencyKeyReused {
		t.Fatalf("key reused: %v", err)
	}
	if restored() != 1 {
		t.Fatal("a replay wrote a second event")
	}
	if _, err := restore(sa, newKey(), "tournament_template", newest, "khôi phục"); trashErr(err) != problem.NotFound {
		t.Fatalf("not in trash: %v", err)
	}
}
