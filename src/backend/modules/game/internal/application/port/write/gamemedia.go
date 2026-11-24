package write

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// MediaAsset is one row of game.media_assets (F078 US4/US5).
type MediaAsset struct {
	ID              uuid.UUID
	Kind            string
	MIME            string
	Filename        string
	Size            int64
	SHA256          string
	Bucket          string
	QuarantineKey   *string
	Status          string
	RejectReason    *string
	Variants        []map[string]any
	Width, Height   *int
	License         *string
	AltText         *string
	UploadedBy      *uuid.UUID
	UploadExpiresAt *time.Time
	CreatedAt       time.Time
}

// RenderJob is one row of game.render_jobs.
type RenderJob struct {
	ID            uuid.UUID
	ConfigType    string
	ItemType      string
	ConfigID      uuid.UUID
	VersionID     uuid.UUID
	SourceAssetID uuid.UUID
	Kind          string
	Status        string
	Attempts      int
	LastError     *string
	ResultAssetID *uuid.UUID
	NextAttemptAt *time.Time
}

// GameMediaStore writes game media, renders and their usages under PlatformScope (role game_platform).
type GameMediaStore interface {
	AssetBySHA(ctx context.Context, sha string) (*MediaAsset, error)
	InsertAsset(ctx context.Context, a MediaAsset, at time.Time) error
	// Pending returns quarantined/processing assets created before `before`, oldest first, locked (SKIP LOCKED).
	Pending(ctx context.Context, before time.Time, limit int) ([]MediaAsset, error)
	// SaveAsset writes status, reject reason, bucket, variants, dimensions of an asset.
	SaveAsset(ctx context.Context, a MediaAsset, at time.Time) error
	Asset(ctx context.Context, id uuid.UUID) (*MediaAsset, error)
	// Content3D returns the ready model3d assets among ids.
	Content3D(ctx context.Context, ids []uuid.UUID) ([]MediaAsset, error)
	// VersionPayload returns the payload, config id and version number of a console version (nil when unknown).
	VersionPayload(ctx context.Context, versionID uuid.UUID) (json.RawMessage, uuid.UUID, int, error)
	// InsertRenderJob creates the job unless (version, source, kind) exists; false = it existed.
	InsertRenderJob(ctx context.Context, j RenderJob, at time.Time) (bool, error)
	RenderJob(ctx context.Context, id uuid.UUID) (*RenderJob, error)
	SaveRenderJob(ctx context.Context, j RenderJob, at time.Time) error
	// DueRenderRetries returns the failed jobs whose retry time has come (locked, SKIP LOCKED).
	DueRenderRetries(ctx context.Context, now time.Time, limit int) ([]RenderJob, error)
	// LinkRender records the render of a version (media_usages usage render_thumbnail/render_turntable; idempotent).
	LinkRender(ctx context.Context, assetID uuid.UUID, j RenderJob, at time.Time) error
	// DeadLetter records an exhausted render job in infra.dead_letters (consumer game.render_results).
	DeadLetter(ctx context.Context, j RenderJob, at time.Time) error
}

// Assignment is one active row of infra.tenant_resource_assignments of a game resource.
type Assignment struct {
	ID           uuid.UUID
	ResourceType string
	ResourceID   uuid.UUID
	TenantID     uuid.UUID
	AssignedBy   uuid.UUID
	AssignedAt   time.Time
	RevokedAt    *time.Time
}

// TenantAssignmentStore writes the assignments of the game resources to tenants of the facade (ADR-0029, F078).
type TenantAssignmentStore interface {
	// Active returns the active assignments of a resource for tenants (all tenants when tenants is empty).
	Active(ctx context.Context, resourceType string, resourceID uuid.UUID, tenants []uuid.UUID) ([]Assignment, error)
	Create(ctx context.Context, a Assignment) error
	Revoke(ctx context.Context, a Assignment, by uuid.UUID, reason string, at time.Time) (Assignment, error)
	// Page lists the active assignments of a resource after tenant (keyset), limit rows.
	Page(ctx context.Context, resourceType string, resourceID uuid.UUID, after *uuid.UUID, limit int) ([]Assignment, error)
	// ActiveTenants returns the tenants among ids that are active in ref.tenants.
	ActiveTenants(ctx context.Context, ids []uuid.UUID) ([]uuid.UUID, error)
	// Published reports whether the game resource (config id of its type) has a current release.
	Published(ctx context.Context, itemType string, configID uuid.UUID) (exists, published bool, err error)
}
