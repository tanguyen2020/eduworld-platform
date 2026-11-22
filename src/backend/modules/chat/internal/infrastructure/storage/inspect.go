package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/gif"
	_ "image/png" // PNG decoder for DecodeConfig
	"io"

	_ "golang.org/x/image/webp" // WebP decoder for DecodeConfig

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/chatasset"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/media"
)

// Inspector implements port.AssetInspector: it reads the object (at most maxBytes + 1 bytes), checks its sha256 against
// the key, sniffs the format from the bytes (never the declared type) and reads the dimensions and animation.
type Inspector struct {
	Storage port.AssetStorage
}

var _ port.AssetInspector = Inspector{}

// Inspect implements port.AssetInspector.
func (i Inspector) Inspect(ctx context.Context, key string, maxBytes int) (chatasset.File, error) {
	f := chatasset.File{Key: key}
	if i.Storage == nil {
		return f, errors.New("chat assets: object storage not configured")
	}
	r, err := i.Storage.Open(ctx, key)
	if err != nil {
		return f, err
	}
	defer func() { _ = r.Close() }()
	data, err := io.ReadAll(io.LimitReader(r, int64(maxBytes)+1))
	if err != nil {
		return f, fmt.Errorf("chat assets: read %w", err)
	}
	return InspectBytes(key, data), nil
}

// InspectBytes inspects the bytes of an object stored under key.
func InspectBytes(key string, data []byte) chatasset.File {
	f := chatasset.File{Key: key, SizeBytes: len(data)}
	sum := sha256.Sum256(data)
	f.HashOK = hex.EncodeToString(sum[:]) == chatasset.KeyHash(key)
	f.ContentType = media.Sniff(data)
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
		f.Width, f.Height = cfg.Width, cfg.Height
	}
	switch f.ContentType {
	case "image/gif":
		if g, err := gif.DecodeAll(bytes.NewReader(data)); err == nil {
			f.Animated = len(g.Image) > 1
		}
	case "image/webp":
		f.Animated = bytes.Contains(data[:min(len(data), 4096)], []byte("ANIM"))
	}
	return f
}

// ErrNotFound reports a missing object as port.ErrObjectNotFound.
func ErrNotFound(err error) bool { return errors.Is(err, port.ErrObjectNotFound) }
