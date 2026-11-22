package port

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/chatasset"
)

// ErrObjectNotFound: the object does not exist (not uploaded yet).
var ErrObjectNotFound = errors.New("object storage: not found")

// AssetStorage is the S3-compatible bucket of the sticker/GIF uploads (EW_CHAT_ASSET_BUCKET; ADR-0075). Keys are
// immutable media/<sha256>/<file> (R31); the public library is served from the EduWorld CDN.
type AssetStorage interface {
	// PresignPut returns a URL to upload exactly size bytes to key before ttl expires.
	PresignPut(ctx context.Context, key string, size int64, ttl time.Duration) (string, error)
	// Head returns the size of the object (ErrObjectNotFound when absent).
	Head(ctx context.Context, key string) (int64, error)
	// Open streams the object.
	Open(ctx context.Context, key string) (io.ReadCloser, error)
}

// AssetInspector reads an uploaded object and tells what it really is (sniffed format, size, dimensions, animation,
// sha256 against the key) — the client is never trusted (F118-AC-17).
type AssetInspector interface {
	Inspect(ctx context.Context, key string, maxBytes int) (chatasset.File, error)
}

// AssetVersion is one row of chat.chat_asset_versions.
type AssetVersion struct {
	VersionID   uuid.UUID
	Type        chatasset.Type
	ConfigID    uuid.UUID
	Version     int
	Status      chatasset.Status
	Title       string
	Payload     []byte
	AuthorID    uuid.UUID
	ReviewerID  *uuid.UUID
	ReviewedAt  *time.Time
	PublishedBy *uuid.UUID
	PublishedAt *time.Time
	UpdatedAt   time.Time
}

// AssetRelease is one row of chat.chat_asset_releases.
type AssetRelease struct {
	ReleaseID   uuid.UUID
	Type        chatasset.Type
	ConfigID    uuid.UUID
	Action      string
	FromVersion *int
	ToVersion   *int
	Reason      string
	ActorID     uuid.UUID
	At          time.Time
}

// AssetReader reads the versions of the library (platform G tables, read session).
type AssetReader interface {
	ListVersions(ctx context.Context, t chatasset.Type, id *uuid.UUID, status string, limit int) ([]AssetVersion, error)
	GetVersion(ctx context.Context, t chatasset.Type, id uuid.UUID, version int) (*AssetVersion, error)
}
