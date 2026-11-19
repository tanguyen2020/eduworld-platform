package write

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ManifestRow is one version of a tenant content manifest (game.content_manifests, immutable rows).
type ManifestRow struct {
	TenantID       uuid.UUID
	Version        int64
	ETag           string
	Entries        json.RawMessage
	GeneratedAt    time.Time
	CauseReleaseID *uuid.UUID
}

// ManifestStore writes tenant manifests in the tenant transaction of ctx (F026 US6).
type ManifestStore interface {
	// Latest returns the newest manifest version and entries of tenant (0, nil when none).
	Latest(ctx context.Context, tenant uuid.UUID) (int64, json.RawMessage, error)
	// Insert adds the next manifest version: a concurrent writer of the same version violates the key (tenant_id,
	// manifest_version) and the event is redelivered (rows are immutable: no UPDATE privilege, so no row lock).
	Insert(ctx context.Context, m ManifestRow) error
}
