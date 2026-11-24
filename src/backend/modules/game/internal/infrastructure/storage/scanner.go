package storage

import (
	"context"
	"io"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/media"
)

// Scanner adapts the clamd scanner of the platform kit to port.MediaScanner.
type Scanner struct{ Clamd media.Clamd }

var _ port.MediaScanner = Scanner{}

// Scan implements port.MediaScanner (an unavailable scanner is an error: the asset stays processing, never ready unscanned).
func (s Scanner) Scan(ctx context.Context, r io.Reader) (bool, error) {
	v, err := s.Clamd.Scan(ctx, r)
	if err != nil {
		return false, err
	}
	return v.Clean, nil
}
