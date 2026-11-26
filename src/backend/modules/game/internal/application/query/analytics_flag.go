package query

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
)

// TenantFlagReader reads the analytics flags of a tenant projection (ref.tenants).
type TenantFlagReader interface {
	TenantFlags(ctx context.Context, tenant uuid.UUID) (port.TenantFlags, error)
}

// ProductTelemetryFlag answers ClientConfig.product_telemetry_enabled (F080-AC-03): the platform value
// (analytics_product_telemetry_enabled) and, for a known tenant, its setting projected from module identity
// (settings_subset.analytics_product_telemetry_enabled, default on). A read error answers the platform value (the server
// still drops product events of a tenant that switched them off).
type ProductTelemetryFlag struct {
	Read     DashSession
	Flags    TenantFlagReader
	Platform bool
}

// ProductTelemetry implements the flag of getClientConfig.
func (f ProductTelemetryFlag) ProductTelemetry(ctx context.Context, tenant *uuid.UUID) bool {
	if !f.Platform || tenant == nil || f.Read == nil || f.Flags == nil {
		return f.Platform
	}
	on := true
	err := f.Read.Read(ctx, func(ctx context.Context) error {
		fl, err := f.Flags.TenantFlags(ctx, *tenant)
		if err != nil {
			return err
		}
		on = !fl.Known || fl.ProductTelemetry
		return nil
	})
	if err != nil {
		slog.WarnContext(ctx, "product telemetry flag unavailable", "error", err)
		return f.Platform
	}
	return on
}
