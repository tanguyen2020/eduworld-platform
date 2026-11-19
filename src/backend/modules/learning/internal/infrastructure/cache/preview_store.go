package cache

import (
	"context"
	"encoding/json"
	"fmt"
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
	if err := s.C.Do(ctx, s.C.B().Set().Key(PreviewPrefix+p.ID.String()).Value(string(b)).Ex(ttl).Build()).Error(); err != nil {
		return fmt.Errorf("cache: put preview: %w", err)
	}
	return nil
}

// Get implements port.PreviewStore.
func (s PreviewStore) Get(ctx context.Context, id uuid.UUID) (preview.Session, error) {
	b, err := s.C.Do(ctx, s.C.B().Get().Key(PreviewPrefix+id.String()).Build()).AsBytes()
	if valkey.IsValkeyNil(err) {
		return preview.Session{}, port.ErrPreviewNotFound
	}
	if err != nil {
		return preview.Session{}, fmt.Errorf("cache: get preview: %w", err)
	}
	var p preview.Session
	if err := json.Unmarshal(b, &p); err != nil {
		return preview.Session{}, port.ErrPreviewNotFound
	}
	return p, nil
}
