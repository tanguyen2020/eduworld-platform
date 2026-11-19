package command

import (
	"context"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// SisStagingRetention is sis_staging_retention (CONFIG_DEFAULTS, code scope, Confirmed 2026-11-16).
const SisStagingRetention = 30 * 24 * time.Hour

// AnonymizeSisStaging is the job sis-staging-anonymizer (F011 T036): staged payloads older than
// sis_staging_retention are cleared in batches; the rows (hash, action, status) stay for audit.
type AnonymizeSisStaging struct {
	UoW     write.UnitOfWork
	Staging write.StagingAnonymizer
	Clock   func() time.Time
	Batch   int
}

// RunOnce clears batches until none is left and returns the number of payloads cleared.
func (h AnonymizeSisStaging) RunOnce(ctx context.Context) (total int, err error) {
	ctx, done := observability.UseCase(ctx, "identity.job.SisStagingAnonymizer")
	defer func() { done(err) }()
	bctx, err := tenancy.Bypass(ctx, tenancy.ReasonPlatformJob)
	if err != nil {
		return 0, err
	}
	now := time.Now().UTC()
	if h.Clock != nil {
		now = h.Clock().UTC()
	}
	batch := h.Batch
	if batch <= 0 {
		batch = 1000
	}
	for {
		var n int
		if err := h.UoW.Do(bctx, func(ctx context.Context) error {
			var err error
			n, err = h.Staging.AnonymizeStagingBefore(ctx, now.Add(-SisStagingRetention), batch)
			return err
		}); err != nil {
			return total, err
		}
		total += n
		if n < batch {
			return total, nil
		}
	}
}
