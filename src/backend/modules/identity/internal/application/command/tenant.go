package command

import (
	"context"
	"errors"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/tenant"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// CreateTenantHandler creates a tenant (createTenant, F010-AC-01, AC-02, AC-04, AC-05).
type CreateTenantHandler interface {
	Handle(ctx context.Context, in dto.CreateTenant) (dto.Tenant, error)
}

// UpdateTenantHandler renames or enables/disables a tenant (updateTenant, F010-AC-03).
type UpdateTenantHandler interface {
	Handle(ctx context.Context, in dto.UpdateTenant) (dto.Tenant, error)
}

// TenantDeps are the ports of the tenant commands.
type TenantDeps struct {
	UoW     write.UnitOfWork
	Tenants write.TenantRepository
	Events  write.EventPublisher
	Audit   write.Auditor
}

// TenantPayload is the payload of tenant.created / tenant.updated (specs/events/payloads/identity).
type TenantPayload struct {
	TenantID      string   `json:"tenant_id"`
	Code          string   `json:"code"`
	Name          string   `json:"name"`
	TenantType    string   `json:"tenant_type"`
	Status        string   `json:"status"`
	Alias         string   `json:"keycloak_org_alias"`
	IsQA          bool     `json:"is_qa"`
	ChangedFields []string `json:"changed_fields"`
	SourceVersion int64    `json:"source_version"`
}

func tenantPayload(t *tenant.Tenant, changed []string) TenantPayload {
	if changed == nil {
		changed = []string{}
	}
	return TenantPayload{TenantID: t.ID.String(), Code: t.Code, Name: t.Name, TenantType: string(t.Type), Status: string(t.Status),
		Alias: t.OrgAlias, IsQA: t.IsQA, ChangedFields: changed, SourceVersion: t.Version}
}

// TenantView maps the aggregate to TenantDetail.
func TenantView(t *tenant.Tenant) dto.Tenant {
	return dto.Tenant{TenantID: t.ID, Code: t.Code, Name: t.Name, TenantType: string(t.Type), Status: string(t.Status),
		Alias: t.OrgAlias, IsQA: t.IsQA, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt, Version: t.Version}
}

// platformWrite authorizes perm for platform staff and returns ctx with the platform scope (G tables).
func platformWrite(ctx context.Context, perm authz.Permission) (context.Context, authz.Principal, error) {
	p, err := principal(ctx)
	if err != nil {
		return ctx, p, err
	}
	if err := authz.Require(ctx, p, perm, authz.Resource{}); err != nil {
		return ctx, p, err
	}
	if !p.IsPlatform() { // a tenant role can never hold platform.* (R20); defense in depth
		return ctx, p, problem.New(problem.PermissionDenied, "")
	}
	sc, err := authz.PlatformScope(p)
	if err != nil {
		return ctx, p, problem.New(problem.PermissionDenied, "").Wrap(err)
	}
	return tenancy.WithScope(ctx, sc), p, nil
}

// CreateTenant implements CreateTenantHandler.
type CreateTenant struct{ TenantDeps }

// Handle runs the command.
func (h CreateTenant) Handle(ctx context.Context, in dto.CreateTenant) (out dto.Tenant, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.CreateTenant")
	defer func() { done(err) }()
	ctx, _, err = platformWrite(ctx, authz.PermPlatformTenantManage)
	if err != nil {
		return out, err
	}
	typ, err := tenant.ParseType(in.TenantType)
	if err != nil {
		return out, invalid(err)
	}
	t, err := tenant.New(ids.New(), in.Code, in.Name, typ, in.Alias, in.DefaultLocale)
	if err != nil {
		return out, invalid(err)
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.Tenants.Create(ctx, t); err != nil {
			return err
		}
		id := t.ID
		if err := h.Events.Publish(ctx, write.Event{Type: "tenant.created", SubjectType: "tenant", SubjectID: id.String(),
			TenantID: &id, Payload: tenantPayload(t, nil)}); err != nil {
			return err
		}
		if err := h.Audit.Record(ctx, audit.Event{Action: "tenant.created", TargetType: "tenant", TargetID: id.String(),
			Result: audit.ResultSuccess, Reason: in.Reason,
			Metadata: map[string]any{"tenant_type": string(t.Type), "code": t.Code}}); err != nil {
			return err
		}
		out = TenantView(t)
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// UpdateTenant implements UpdateTenantHandler.
type UpdateTenant struct{ TenantDeps }

// Handle runs the command.
func (h UpdateTenant) Handle(ctx context.Context, in dto.UpdateTenant) (out dto.Tenant, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.UpdateTenant")
	defer func() { done(err) }()
	ctx, _, err = platformWrite(ctx, authz.PermPlatformTenantManage)
	if err != nil {
		return out, err
	}
	patch := tenant.Patch{Name: in.Name, IsQA: in.IsQA}
	if in.Status != nil {
		st, err := tenant.ParseStatus(*in.Status)
		if err != nil {
			return out, invalid(err)
		}
		patch.Status = &st
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		t, err := h.Tenants.Get(ctx, in.ID)
		if err != nil {
			return err
		}
		if in.IfMatch != t.Version {
			return problem.New(problem.VersionMismatch, "")
		}
		wasQA := t.IsQA
		ch, err := t.Apply(patch)
		if errors.Is(err, tenant.ErrNoChange) {
			out = TenantView(t)
			return nil
		}
		if err != nil {
			return invalid(err)
		}
		if err := h.Tenants.Save(ctx, t); err != nil {
			return err
		}
		var changed []string
		if ch.NameChanged {
			changed = append(changed, "name")
		}
		if ch.StatusChanged {
			changed = append(changed, "status")
		}
		if ch.QAChanged {
			changed = append(changed, "is_qa")
		}
		id := t.ID
		if err := h.Events.Publish(ctx, write.Event{Type: "tenant.updated", SubjectType: "tenant", SubjectID: id.String(),
			TenantID: &id, Payload: tenantPayload(t, changed)}); err != nil {
			return err
		}
		if err := h.Audit.Record(ctx, audit.Event{Action: "tenant.updated", TargetType: "tenant", TargetID: id.String(),
			Result: audit.ResultSuccess, Reason: in.Reason, Metadata: map[string]any{"changed_fields": changed, "status": string(t.Status)}}); err != nil {
			return err
		}
		if ch.QAChanged { // F134-AC-07: the QA flag has its own audit with the value before and after
			if err := h.Audit.Record(ctx, audit.Event{Action: "tenant.qa_flag_changed", TargetType: "tenant", TargetID: id.String(),
				Result: audit.ResultSuccess, Reason: in.Reason, Metadata: map[string]any{"is_qa_before": wasQA, "is_qa_after": t.IsQA}}); err != nil {
				return err
			}
		}
		out = TenantView(t)
		return idempotency.SetResult(ctx, out) // replay of the same Idempotency-Key (F134-AC-07)
	})
	return out, err
}
