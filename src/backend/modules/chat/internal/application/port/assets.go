package port

import (
	"context"
	"errors"
	"io"
	"strconv"
	"strings"
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
	// PresignGet signs a short GET of a private object (preview of an unpublished pack/GIF in Game Studio, F082-AC-07).
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
}

// AssetInspector reads an uploaded object and tells what it really is (sniffed format, size, dimensions, animation,
// sha256 against the key) — the client is never trusted (F118-AC-17).
type AssetInspector interface {
	Inspect(ctx context.Context, key string, maxBytes int) (chatasset.File, error)
}

// AssetVersion is one row of chat.chat_asset_versions. Revision is its optimistic version: every change of the row adds 1,
// ETag() is the entity tag of getGameConfigVersion and the If-Match of the writes (F082 Clarifications 2026-11-26).
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
	Revision    int64
}

// ETag is the entity tag of the version ("<revision>").
func (v AssetVersion) ETag() string { return `"` + strconv.FormatInt(v.Revision, 10) + `"` }

// MatchesETag reports whether an If-Match header names the current revision: "<revision>", W/"<revision>", the bare
// number or *.
func (v AssetVersion) MatchesETag(header string) bool {
	h := strings.TrimPrefix(strings.TrimSpace(header), "W/")
	return h == "*" || h == v.ETag() || h == strconv.FormatInt(v.Revision, 10)
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

// AssetFilter is listGameConfigs of sticker-packs/gifs (F082): one row per item — its newest version, or its newest
// version of Status — of the live items, filtered by the author of that version and by a GIF tag, keyset after
// (updated_at, config_id) descending; Limit rows are read (limit+1 detects has_more).
type AssetFilter struct {
	Type     chatasset.Type
	Status   string
	AuthorID *uuid.UUID
	Tag      string
	AfterAt  *time.Time
	AfterID  *uuid.UUID
	Limit    int
}

// AssetReader reads the versions of the library (platform G tables, read session).
type AssetReader interface {
	ListItems(ctx context.Context, f AssetFilter) ([]AssetVersion, error)
	// ListItemVersions lists the versions of one item newest first, keyset before version number (found = the item exists).
	ListItemVersions(ctx context.Context, t chatasset.Type, id uuid.UUID, before *int, limit int) (rows []AssetVersion, found bool, err error)
	GetVersion(ctx context.Context, t chatasset.Type, id uuid.UUID, version int) (*AssetVersion, error)
	// CurrentVersion is the version number of the release pointer of the item (nil: not published or unpublished).
	CurrentVersion(ctx context.Context, t chatasset.Type, id uuid.UUID) (*int, error)
	// Releases lists the release history of one item newest first, keyset before (created_at, release_id).
	Releases(ctx context.Context, t chatasset.Type, id uuid.UUID, beforeAt *time.Time, beforeID *uuid.UUID, limit int) ([]AssetRelease, error)
}
