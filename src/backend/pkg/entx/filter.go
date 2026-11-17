package entx

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

var (
	// ErrMissingTenantScope: a tenant-scoped entity was queried or written without a scope or Bypass.
	ErrMissingTenantScope = errors.New("entx: missing tenant scope")
	// ErrHardDeleteForbidden: delete on an entity without soft-delete fields (ADR-0028).
	ErrHardDeleteForbidden = errors.New("entx: hard delete forbidden")
	// ErrTenantOutOfScope: a create sets a tenant_id outside the scope, or a NULL tenant outside PlatformScope.
	ErrTenantOutOfScope = errors.New("entx: tenant_id outside the scope")
)

type hybrid struct{ resourceType string }

type wherer interface{ WhereP(...func(*sql.Selector)) }

// filter returns the global query filter of a mixin (MULTITENANCY §4.4).
func filter(k Kind, h *hybrid) ent.Interceptor {
	return ent.TraverseFunc(func(ctx context.Context, q ent.Query) error {
		ps, err := predicates(ctx, k, h)
		if err != nil {
			return err
		}
		return where(q, ps)
	})
}

// predicates computes the conditions of kind k for the scope of ctx.
func predicates(ctx context.Context, k Kind, h *hybrid) ([]func(*sql.Selector), error) {
	var ps []func(*sql.Selector)
	reason, bypassed := tenancy.Bypassed(ctx)
	trash := bypassed && reason == tenancy.ReasonTrash
	if k.softDelete() && !trash {
		ps = append(ps, func(s *sql.Selector) { s.Where(sql.IsNull(s.C("deleted_at"))) })
	}
	if !k.tenant() || bypassed {
		return ps, nil
	}
	scope, ok := tenancy.ScopeFrom(ctx)
	if !ok {
		return nil, ErrMissingTenantScope
	}
	if scope.PlatformOnly() {
		if !k.nullable() {
			return append(ps, func(s *sql.Selector) { s.Where(sql.False()) }), nil
		}
		return append(ps, func(s *sql.Selector) { s.Where(sql.IsNull(s.C("tenant_id"))) }), nil
	}
	tenants := anyIDs(scope.TenantIDs())
	if k != KindH {
		return append(ps, func(s *sql.Selector) { s.Where(sql.In(s.C("tenant_id"), tenants...)) }), nil
	}
	return append(ps, func(s *sql.Selector) {
		a := sql.Table("tenant_resource_assignments").Schema("infra").As("ra")
		assigned := sql.Select("1").From(a).Where(sql.And(
			sql.EQ(a.C("resource_type"), h.resourceType),
			sql.ColumnsEQ(a.C("resource_id"), s.C("id")),
			sql.In(a.C("tenant_id"), tenants...),
			sql.IsNull(a.C("deleted_at")),
		))
		s.Where(sql.Or(
			sql.In(s.C("tenant_id"), tenants...),
			sql.And(sql.IsNull(s.C("tenant_id")), sql.Exists(assigned)),
		))
	}), nil
}

// where applies storage-level predicates: WhereP when generated, otherwise the typed Where(predicate.X...)
// whose element type has the underlying type func(*sql.Selector).
func where(q any, ps []func(*sql.Selector)) error {
	if len(ps) == 0 {
		return nil
	}
	if w, ok := q.(wherer); ok {
		w.WhereP(ps...)
		return nil
	}
	m := reflect.ValueOf(q).MethodByName("Where")
	if !m.IsValid() || m.Type().NumIn() != 1 || !m.Type().IsVariadic() {
		return fmt.Errorf("entx: query %T has no Where(predicate...)", q)
	}
	elem := m.Type().In(0).Elem()
	args := make([]reflect.Value, len(ps))
	for i, p := range ps {
		v := reflect.ValueOf(p)
		if !v.Type().ConvertibleTo(elem) {
			return fmt.Errorf("entx: %T.Where takes %s", q, elem)
		}
		args[i] = v.Convert(elem)
	}
	m.Call(args)
	return nil
}

func anyIDs(ids []uuid.UUID) []any {
	out := make([]any, len(ids))
	for i, id := range ids {
		out[i] = id
	}
	return out
}

// mutationHook scopes writes: create gets/validates tenant_id, update/delete get the same predicates as
// queries (ent interceptors do not cover mutations), delete becomes a soft-delete (or is refused).
func mutationHook(k Kind, h *hybrid) ent.Hook {
	return func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
			switch {
			case m.Op().Is(ent.OpCreate):
				if err := scopeCreate(ctx, k, m); err != nil {
					return nil, err
				}
				stampActor(ctx, m, "created_by")
				return next.Mutate(ctx, m)
			case m.Op().Is(ent.OpDelete) || m.Op().Is(ent.OpDeleteOne):
				if !k.softDelete() {
					return nil, ErrHardDeleteForbidden
				}
				if err := scopeWhere(ctx, k, h, m); err != nil {
					return nil, err
				}
				return softDelete(ctx, m)
			default: // update
				if err := scopeWhere(ctx, k, h, m); err != nil {
					return nil, err
				}
				if k.softDelete() {
					stampActor(ctx, m, "updated_by")
				}
				return next.Mutate(ctx, m)
			}
		})
	}
}

func scopeWhere(ctx context.Context, k Kind, h *hybrid, m ent.Mutation) error {
	ps, err := predicates(ctx, k, h)
	if err != nil {
		return err
	}
	return where(m, ps)
}

func scopeCreate(ctx context.Context, k Kind, m ent.Mutation) error {
	if !k.tenant() {
		return nil
	}
	if _, bypassed := tenancy.Bypassed(ctx); bypassed {
		return nil
	}
	scope, ok := tenancy.ScopeFrom(ctx)
	if !ok {
		return ErrMissingTenantScope
	}
	v, set := m.Field("tenant_id")
	if scope.PlatformOnly() {
		if set || !k.nullable() {
			return ErrTenantOutOfScope // platform scope writes only tenant_id NULL rows of T*/TA*/H
		}
		return nil
	}
	if !set {
		id, single := scope.SessionTenantID()
		if !single {
			return fmt.Errorf("%w: cross-tenant create needs an explicit tenant_id", ErrTenantOutOfScope)
		}
		return m.SetField("tenant_id", id)
	}
	id, _ := v.(uuid.UUID)
	for _, t := range scope.TenantIDs() {
		if t == id {
			return nil
		}
	}
	return ErrTenantOutOfScope
}

// actor is the user id of the principal, or nil for jobs and consumers.
func actor(ctx context.Context) *uuid.UUID {
	if p, ok := authz.PrincipalFrom(ctx); ok {
		id := p.UserID
		return &id
	}
	return nil
}

func stampActor(ctx context.Context, m ent.Mutation, name string) {
	if a := actor(ctx); a != nil {
		_ = m.SetField(name, *a)
	}
}

type deleteReasonKey struct{}

// WithDeleteReason sets delete_reason (≤ 64 chars) for soft-deletes run with ctx.
func WithDeleteReason(ctx context.Context, reason string) context.Context {
	if len(reason) > 64 {
		reason = reason[:64]
	}
	return context.WithValue(ctx, deleteReasonKey{}, reason)
}

// softDelete turns the delete mutation into an update of deleted_at/deleted_by/delete_reason and runs it
// through the generated client (ent's soft-delete pattern; reflection keeps entx free of generated code).
func softDelete(ctx context.Context, m ent.Mutation) (ent.Value, error) {
	op, ok := m.(interface{ SetOp(ent.Op) })
	if !ok {
		return nil, ErrHardDeleteForbidden
	}
	op.SetOp(ent.OpUpdate)
	if err := m.SetField("deleted_at", time.Now().UTC()); err != nil {
		return nil, ErrHardDeleteForbidden
	}
	if a := actor(ctx); a != nil {
		_ = m.SetField("deleted_by", *a)
	}
	if r, ok := ctx.Value(deleteReasonKey{}).(string); ok && r != "" {
		_ = m.SetField("delete_reason", r)
	}
	client := reflect.ValueOf(m).MethodByName("Client")
	if !client.IsValid() {
		return nil, fmt.Errorf("entx: mutation %T has no Client()", m)
	}
	c := client.Call(nil)[0]
	mutate := c.MethodByName("Mutate")
	if !mutate.IsValid() {
		return nil, fmt.Errorf("entx: client of %T has no Mutate", m)
	}
	out := mutate.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(m)})
	if err, _ := out[1].Interface().(error); err != nil {
		return nil, err
	}
	return out[0].Interface(), nil
}
