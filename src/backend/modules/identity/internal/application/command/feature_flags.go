package command

import (
	"context"
	"log/slog"
	"maps"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/settings"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// AuditFeatureFlagUpdated is the audit action of putFeatureFlag (F015-AC-25).
const AuditFeatureFlagUpdated = "feature_flag.updated"

// FeatureFlagWrite is the body of putFeatureFlag.
type FeatureFlagWrite struct {
	Scope       string // platform | tenant
	State       string // enabled | disabled | inherit
	Description string
	Reason      string
	TenantID    *uuid.UUID // X-Tenant-Id: the target tenant when Scope = tenant
}

// FeatureFlags implements putFeatureFlag (F015 US5, ADR-0090): System Admin (platform.feature_flag.write,
// MFA through the /admin guard) sets the platform default or a tenant override of a feature flag or of a
// menu/action of the catalog; kind and parent come from the catalog; protected items cannot be disabled.
type FeatureFlags struct {
	UoW    write.UnitOfWork
	Repo   write.FeatureFlagRepository
	Events write.EventPublisher
	Audit  write.Auditor
	Cache  port.FeatureToggleCache // nil: entries expire after feature_toggle_cache_ttl
}

// Put implements putFeatureFlag; expected is the If-Match version (0 = no row yet).
func (h FeatureFlags) Put(ctx context.Context, key string, in FeatureFlagWrite, expected int64) (out settings.Flag, canDisable bool, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.PutFeatureFlag")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, false, err
	}
	// A System Admin writing a tenant override carries the target tenant in X-Tenant-Id (R20), so the
	// principal may have that tenant; any other tenant principal is refused.
	if p.TenantID != nil && !p.IsSystemAdmin {
		return out, false, problem.New(problem.PermissionDenied, "")
	}
	if err := authz.Require(ctx, p, authz.PermPlatformFeatureFlagWrite, authz.Resource{}); err != nil {
		return out, false, err
	}
	if err := checkReason(in.Reason); err != nil {
		return out, false, err
	}
	switch {
	case in.Scope != "platform" && in.Scope != "tenant":
		return out, false, problem.Invalid("scope", problem.SubInvalid, "platform or tenant")
	case in.State != "enabled" && in.State != "disabled" && in.State != "inherit":
		return out, false, problem.Invalid("state", problem.SubInvalid, "enabled, disabled or inherit")
	case in.State == "inherit" && in.Scope == "platform":
		return out, false, problem.Invalid("state", problem.SubInvalid, "inherit only with scope tenant")
	case in.Scope == "tenant" && in.TenantID == nil:
		return out, false, problem.Invalid("X-Tenant-Id", problem.SubInvalid, "the target tenant is required with scope tenant")
	}
	sc, err := authz.SystemPlatformScope("feature-flags")
	if err != nil {
		return out, false, err
	}
	canDisable = true
	err = h.UoW.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		kind := settings.KindOfKey(key)
		parent := ""
		if kind != "feature" {
			item, err := h.Repo.CatalogItem(ctx, key)
			if err != nil {
				return err
			}
			if item == nil || item.Kind == "workspace" {
				return problem.New(problem.NotFound, "")
			}
			kind, parent, canDisable = item.Kind, item.Parent, item.CanDisable
			if kind == "menu" {
				parent = ""
			}
		}
		if !canDisable && in.State == "disabled" {
			return problem.New(problem.FeatureNotDisableable, "")
		}
		cur, err := h.Repo.Flag(ctx, key)
		if err != nil {
			return err
		}
		version := int64(0)
		f := settings.Flag{Key: key, Kind: kind, Parent: parent, Default: true, Overrides: map[string]bool{}}
		if cur != nil {
			version, f = cur.Version, *cur
			f.Overrides = maps.Clone(cur.Overrides)
			if f.Overrides == nil {
				f.Overrides = map[string]bool{}
			}
		}
		if version != expected {
			return problem.New(problem.VersionMismatch, "")
		}
		before := f
		if kind == "feature" && in.Description != "" {
			f.Description = in.Description
		}
		switch {
		case in.Scope == "platform":
			f.Default = in.State == "enabled"
		case in.State == "inherit":
			delete(f.Overrides, in.TenantID.String())
		default:
			f.Overrides[in.TenantID.String()] = in.State == "enabled"
		}
		v, err := h.Repo.Save(ctx, f, expected, p.UserID)
		if err != nil {
			return err
		}
		f.Version = v
		out = f
		var tenant any
		ev := write.Event{Type: "platform_settings.changed", SubjectType: "feature_flag", SubjectID: key, Platform: true}
		if in.Scope == "tenant" {
			tenant = *in.TenantID
		}
		ev.Payload = map[string]any{"key": key, "scope": "feature_flag", "tenant_id": tenant, "school_id": nil, "flag_key": key,
			"before": flagState(before, in.TenantID), "after": flagState(f, in.TenantID), "curfew_window": nil, "reason": in.Reason,
			"actor_id": p.UserID}
		if err := h.Events.Publish(ctx, ev); err != nil {
			return err
		}
		meta := map[string]any{"flag_kind": kind, "scope": in.Scope, "state": in.State,
			"before": auditValue(flagState(before, in.TenantID)), "after": auditValue(flagState(f, in.TenantID))}
		if in.TenantID != nil && in.Scope == "tenant" {
			meta["tenant_id"] = in.TenantID.String()
		}
		return h.Audit.Record(ctx, audit.Event{Action: AuditFeatureFlagUpdated, TargetType: "feature_flag", TargetID: key,
			Result: audit.ResultSuccess, Reason: in.Reason, Metadata: meta})
	})
	if err != nil {
		return out, canDisable, err
	}
	if h.Cache != nil {
		var scope *uuid.UUID
		if in.Scope == "tenant" {
			scope = in.TenantID
		}
		if err := h.Cache.Invalidate(ctx, scope); err != nil {
			slog.WarnContext(ctx, "feature toggle cache invalidation failed; entries expire with their TTL", "error", err)
		}
	}
	return out, canDisable, nil
}

func flagState(f settings.Flag, tenant *uuid.UUID) map[string]any {
	m := map[string]any{"default_enabled": f.Default}
	if o := f.Override(tenant); o != nil {
		m["tenant_override"] = *o
	}
	return m
}
