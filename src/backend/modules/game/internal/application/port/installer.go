package port

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/installer"
)

// SchoolKitIndex loads the signed school-kit index of a channel from the release CDN (F134-AC-13, AC-14): the adapter
// returns only an index whose RSA-SHA256 signature and schema were verified, otherwise an error (→ 503 UNAVAILABLE).
type SchoolKitIndex interface {
	Load(ctx context.Context, channel string) (installer.Index, error)
}

// InstallerReads are the projections the installer page reads in the tenant of the session (ref.schools, ref.students).
type InstallerReads interface {
	// SchoolExists reports whether the school is a school of the tenant.
	SchoolExists(ctx context.Context, tenant, school uuid.UUID) (bool, error)
	// SchoolGradeLevels returns the distinct grade levels of the active students of the school (ADR-0095: no class).
	SchoolGradeLevels(ctx context.Context, tenant, school uuid.UUID) ([]int, error)
}

// InstallerMetrics count the index loads (eduworld_school_kit_index_fetch_total{result}).
type InstallerMetrics interface {
	IndexFetch(result string)
}
