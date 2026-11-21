package port

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation/catalog"
)

// CatalogNotifier announces configuration changes to every instance (Valkey Pub/Sub ew:chat:sensitive:version and
// ew:chat:switches:changed; best effort — the instances also reconcile every moderation_config_propagation_max / 2).
type CatalogNotifier interface {
	CatalogPublished(ctx context.Context, tenant *uuid.UUID, version int64) error
	SwitchesChanged(ctx context.Context, tenant *uuid.UUID) error
}

// RegressionInput is the candidate catalogue evaluated before a publish (F116-AC-14): the platform part (the candidate
// itself for a platform publish, the version in force for a tenant publish), the tenant candidate (tenant publish) and
// the category settings of that tenant.
type RegressionInput struct {
	Platform catalog.Snapshot
	Tenant   *catalog.Snapshot
	Settings map[uuid.UUID]bool
}

// RegressionResult is the outcome of the regression suite: recall on the high/critical/PII samples, false-positive rate
// on the clean samples, and the sample counts (no sample text ever leaves the suite).
type RegressionResult struct {
	Recall    float64
	FPRate    float64
	Positives int
	Clean     int
}

// RegressionSuite runs the labelled corpus against a candidate catalogue with the real engine.
type RegressionSuite interface {
	Run(ctx context.Context, in RegressionInput) (RegressionResult, error)
}
