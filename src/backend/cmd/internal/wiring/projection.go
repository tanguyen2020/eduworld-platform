package wiring

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	identityapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/module"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/projection"
)

// ProjectionCallDeadline is the deadline of one listProjectionSnapshot page of the rebuild: like the reconciliation, a job
// page uses the saga-step value of module_call_deadline (5 s, CONFIG_DEFAULTS), no retry (ADR-0074).
var ProjectionCallDeadline = 5 * time.Second

// ProjectionPageLimit is the page size the rebuild asks listProjectionSnapshot for (the facade default, ≤ 1000).
const ProjectionPageLimit = 500

// projectionProducer is the envelope producer of the replayed snapshot rows (never published: no outbox row).
const projectionProducer = "identity/projection-rebuild"

// IdentityProjectionFacade is the part of the identity facade the rebuild reads.
type IdentityProjectionFacade interface {
	ListProjectionSnapshot(ctx context.Context, req identityapi.ListProjectionSnapshotRequestObject) (identityapi.ListProjectionSnapshotResponseObject, error)
}

// identityRefTables are the ref.* projections kept from module identity that listProjectionSnapshot can rebuild (entity
// type → table), in the order a full rebuild needs them: ref.schools and ref.students reference ref.tenants.
var identityRefTables = []struct{ entity, table string }{
	{"tenant", "ref.tenants"}, {"school", "ref.schools"}, {"student", "ref.students"},
}

// IdentityProjectionModules are the modules that keep ref.* projections of module identity (DATABASE_DESIGN §2.17).
var IdentityProjectionModules = []string{"learning", "game", "competition", "chat"}

// RegisterIdentityProjections registers in reg the rebuild of ref.tenants, ref.schools and ref.students of every module of
// IdentityProjectionModules that declares the consumer <module>.projection_identity in r (F003-AC-39, F003 Clarifications
// 2026-11-28). Each table pages listProjectionSnapshot of the identity facade of c in process (pkg/modcall, caller = the
// receiving module, checked against x-allowed-callers) and applies every item as the projection event it stands for
// (tenant.updated, school.updated, student.placement_changed or student.synced, then student.cleaned_up/restored) through
// the handler of that consumer, one transaction per tenant of a page under the EventScope of that tenant: the validation,
// the tenant guard and the source_version rule of the event path apply unchanged, nothing is deleted or truncated.
func RegisterIdentityProjections(reg *projection.Registry, r *module.Registry, c *Composition) (int, error) {
	if c == nil || c.Identity == nil || c.Identity.Facade() == nil {
		return 0, fmt.Errorf("projection: the identity facade is not composed")
	}
	f := c.Identity.Facade()
	n := 0
	for _, m := range IdentityProjectionModules {
		name := m + ".projection_identity"
		o, ok := r.ConsumerOptions(name)
		if !ok {
			continue
		}
		types := map[string]bool{}
		for _, i := range r.ConsumerInfos(m) {
			if i.Name == name && i.Source == "identity" {
				for _, t := range i.EventTypes {
					types[t] = true
				}
			}
		}
		for _, rt := range identityRefTables {
			if !identityTableSupported(rt.entity, types) {
				continue
			}
			src := identitySnapshot{caller: m, entity: rt.entity, f: f}
			if err := reg.Register(projection.Table{Module: m, Source: "identity", Name: rt.table, Fetch: src.fetch,
				Upsert: replayer{o: o, types: types}.upsert}); err != nil {
				return n, err
			}
			n++
		}
	}
	return n, nil
}

// identityTableSupported reports whether the consumer understands the events of entity (its subscribed types).
func identityTableSupported(entity string, types map[string]bool) bool {
	switch entity {
	case "tenant":
		return types["tenant.updated"] || types["*"]
	case "school":
		return types["school.updated"] || types["*"]
	case "student":
		return types["student.placement_changed"] || types["student.synced"] || types["*"]
	}
	return false
}

// identitySnapshot pages listProjectionSnapshot of one entity type for the rebuild of caller.
type identitySnapshot struct {
	caller, entity string
	f              IdentityProjectionFacade
}

func (s identitySnapshot) op() modcall.Op {
	const name = "listProjectionSnapshot"
	return modcall.Op{Caller: s.caller, Callee: "identity", Operation: name, Kind: modcall.Read, Deadline: ProjectionCallDeadline,
		AllowedCallers: identityapi.AllowedCallers[name], ContextRequired: identityapi.ContextRequired[name]}
}

// snapshotRow is the value of a projection.Row of the rebuild: the item and the time its page was read.
type snapshotRow struct {
	item   identityapi.ProjectionSnapshotItem
	readAt time.Time
}

const rowValue = "snapshot"

func (s identitySnapshot) fetch(ctx context.Context, cursor string) ([]projection.Row, string, error) {
	limit := ProjectionPageLimit
	params := identityapi.ListProjectionSnapshotParams{Limit: &limit} // no tenant_id: platform scope of the job (every tenant)
	if cursor != "" {
		params.Cursor = &cursor
	}
	readAt := time.Now().UTC()
	resp, err := modcall.Call(ctx, s.op(), func(ctx context.Context) (identityapi.ListProjectionSnapshotResponseObject, error) {
		return s.f.ListProjectionSnapshot(ctx, identityapi.ListProjectionSnapshotRequestObject{
			EntityType: identityapi.ListProjectionSnapshotParamsEntityType(s.entity), Params: params})
	})
	if err != nil {
		return nil, "", err
	}
	page, ok := resp.(identityapi.ListProjectionSnapshot200JSONResponse)
	if !ok {
		return nil, "", fmt.Errorf("wiring: identity listProjectionSnapshot: unexpected %T", resp)
	}
	rows := make([]projection.Row, 0, len(page.Items))
	for _, it := range page.Items {
		if string(it.EntityType) != s.entity {
			return nil, "", fmt.Errorf("wiring: listProjectionSnapshot %s answered a %s item", s.entity, it.EntityType)
		}
		rows = append(rows, projection.Row{Key: it.EntityId.String(), SourceVersion: int64(it.SourceVersion),
			Values: map[string]any{rowValue: snapshotRow{item: it, readAt: readAt}}})
	}
	next := ""
	if page.Page.HasMore {
		next = deref(page.Page.NextCursor)
	}
	return rows, next, nil
}

// replayer applies the rows of a page through the projection consumer of the receiving module.
type replayer struct {
	o     outbox.ConsumerOptions
	types map[string]bool
}

func (p replayer) upsert(ctx context.Context, rows []projection.Row) error {
	var order []uuid.UUID
	byTenant := map[uuid.UUID][]outbox.Envelope{}
	for _, r := range rows {
		v, ok := r.Values[rowValue].(snapshotRow)
		if !ok {
			return fmt.Errorf("wiring: projection row %s without snapshot", r.Key)
		}
		envs, err := SnapshotEnvelopes(v.item, p.types, v.readAt)
		if err != nil {
			return err
		}
		t := v.item.TenantId
		if _, seen := byTenant[t]; !seen {
			order = append(order, t)
		}
		byTenant[t] = append(byTenant[t], envs...)
	}
	for _, t := range order {
		tenant := t
		if err := outbox.Replay(ctx, p.o, &tenant, byTenant[t]); err != nil {
			return err
		}
	}
	return nil
}

// SnapshotEnvelopes turns one listProjectionSnapshot item into the envelopes of the projection events it stands for, for a
// consumer subscribed to types (F003 Clarifications 2026-11-28):
//   - tenant → tenant.updated; school → school.updated (data + source_version, changed_fields []); a soft-deleted source
//     row is projected with status disabled (never active);
//   - student → student.placement_changed when the consumer has it and the grade is known, otherwise student.synced
//     (action update, grade, school; status deactivated when the placement is inactive, otherwise the stored status is
//     kept and a new row is active), then student.cleaned_up at deleted_at of the source (deleted) or student.restored at
//     the page read time (live), when the consumer has them.
//
// The envelopes carry the tenant of the item, occurred_at = read time (or deleted_at) and never reach NATS nor an outbox.
func SnapshotEnvelopes(it identityapi.ProjectionSnapshotItem, types map[string]bool, readAt time.Time) ([]outbox.Envelope, error) {
	has := func(t string) bool { return types[t] || types["*"] }
	tenant := it.TenantId
	env := func(typ string, at time.Time, payload map[string]any) (outbox.Envelope, error) {
		b, err := json.Marshal(payload)
		if err != nil {
			return outbox.Envelope{}, fmt.Errorf("wiring: %s payload of %s: %w", typ, it.EntityId, err)
		}
		return outbox.Envelope{EventID: ids.New(), EventType: typ, SchemaVersion: 1, OccurredAt: at.UTC(), RecordedAt: readAt.UTC(),
			Producer: projectionProducer, TenantID: &tenant, Actor: outbox.Actor{Type: "system"},
			Subject: outbox.Subject{Type: string(it.EntityType), ID: it.EntityId.String()}, Payload: b}, nil
	}
	data := func() map[string]any {
		out := make(map[string]any, len(it.Data)+3)
		for k, v := range it.Data {
			out[k] = v
		}
		out["source_version"] = it.SourceVersion
		return out
	}
	var out []outbox.Envelope
	add := func(typ string, at time.Time, payload map[string]any) error {
		e, err := env(typ, at, payload)
		if err == nil {
			out = append(out, e)
		}
		return err
	}
	switch it.EntityType {
	case "tenant", "school":
		typ := "tenant.updated"
		if it.EntityType == "school" {
			typ = "school.updated"
		}
		if !has(typ) {
			return nil, fmt.Errorf("wiring: the consumer does not handle %s", typ)
		}
		d := data()
		d["changed_fields"] = []string{}
		if it.EntityType == "tenant" {
			d["tenant_id"] = it.EntityId.String()
		} else {
			d["school_id"], d["tenant_id"] = it.EntityId.String(), tenant.String()
		}
		if it.Deleted {
			d["status"] = "disabled"
		}
		if err := add(typ, readAt, d); err != nil {
			return nil, err
		}
		return out, nil
	case "student":
		d := data()
		d["student_id"], d["tenant_id"] = it.EntityId.String(), tenant.String()
		grade, hasGrade := d["grade_level"]
		switch {
		case has("student.placement_changed") && hasGrade && grade != nil:
			if err := add("student.placement_changed", readAt, d); err != nil {
				return nil, err
			}
		case has("student.synced"):
			synced := map[string]any{"student_id": d["student_id"], "school_id": d["school_id"], "action": "update",
				"source_version": it.SourceVersion}
			if hasGrade && grade != nil {
				synced["grade_level"] = grade
			}
			if d["status"] != "active" {
				synced["status"] = "deactivated"
			}
			if err := add("student.synced", readAt, synced); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("wiring: the consumer cannot project student %s", it.EntityId)
		}
		life := map[string]any{"student_id": d["student_id"], "tenant_id": d["tenant_id"]}
		switch {
		case it.Deleted && has("student.cleaned_up"):
			at := readAt
			if it.DeletedAt != nil {
				at = *it.DeletedAt
			}
			if err := add("student.cleaned_up", at, life); err != nil {
				return nil, err
			}
		case !it.Deleted && has("student.restored"):
			if err := add("student.restored", readAt, life); err != nil {
				return nil, err
			}
		}
		return out, nil
	}
	return nil, fmt.Errorf("wiring: unknown snapshot entity %q", it.EntityType)
}
