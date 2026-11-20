package cache

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/preview"
)

// PreviewPrefix is the key prefix of the preview sessions: ew:learning:preview:<preview_session_id>.
const PreviewPrefix = "ew:learning:preview:"

// PreviewStore implements port.PreviewStore on Valkey (TTL preview_token_ttl; not business state, R9).
type PreviewStore struct{ C valkey.Client }

var _ port.PreviewStore = PreviewStore{}

// Put implements port.PreviewStore.
func (s PreviewStore) Put(ctx context.Context, p preview.Session, ttl time.Duration) error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, PoolTimeout)
	defer cancel()
	// Bounded by PoolTimeout; a failed write is logged and the session is simply not found later (cache only, OQ F031
	// owner 2026-11-20): the preview is rendered again from PostgreSQL by a new request.
	if err := s.C.Do(ctx, s.C.B().Set().Key(PreviewPrefix+p.ID.String()).Value(string(b)).Ex(ttl).Build()).Error(); err != nil {
		slog.WarnContext(ctx, "preview cache write failed", "error", err)
	}
	return nil
}

// Get implements port.PreviewStore.
func (s PreviewStore) Get(ctx context.Context, id uuid.UUID) (preview.Session, error) {
	ctx, cancel := context.WithTimeout(ctx, PoolTimeout)
	defer cancel()
	b, err := s.C.Do(ctx, s.C.B().Get().Key(PreviewPrefix+id.String()).Build()).AsBytes()
	if err != nil {
		if !valkey.IsValkeyNil(err) {
			slog.WarnContext(ctx, "preview cache read failed", "error", err)
		}
		return preview.Session{}, port.ErrPreviewNotFound
	}
	var p preview.Session
	if err := json.Unmarshal(b, &p); err != nil {
		return preview.Session{}, port.ErrPreviewNotFound
	}
	return p, nil
}
