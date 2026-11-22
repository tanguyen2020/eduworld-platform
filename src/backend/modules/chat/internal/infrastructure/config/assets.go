package config

import (
	"errors"
	"strings"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// Defaults of CONFIG_DEFAULTS for the sticker/GIF library and the message features (F118 T002).
const (
	DefaultStickerGifPerMinute = 4               // chat_sticker_gif_rate: sticker + GIF per minute per channel
	DefaultStickerMaxBytes     = 512 * 1024      // chat_asset_file_max: sticker WebP/PNG ≤ 512 KB
	DefaultGifMaxBytes         = 2 * 1024 * 1024 // chat_asset_file_max: GIF ≤ 2 MB
	DefaultGifMaxPixels        = 480             // chat_asset_file_max: GIF ≤ 480 px
	DefaultAssetBucket         = "ew-chat-assets"
)

// Assets is the configuration of the sticker/GIF library: the CDN base of the public library (EW_CDN_BASE_URL, shared
// with the content media), the bucket of the uploads (EW_CHAT_ASSET_BUCKET) and the sticker/GIF ceiling.
type Assets struct {
	CDNBase             string
	Bucket              string
	StickerGifPerMinute int
	StickerMaxBytes     int
	GifMaxBytes         int
	GifMaxPixels        int
}

// AssetsFrom reads EW_CDN_BASE_URL, EW_CHAT_ASSET_BUCKET and EW_CHAT_STICKER_GIF_RATE.
func AssetsFrom(c *config.Config) (Assets, error) {
	a := Assets{CDNBase: strings.TrimRight(c.StringOr("EW_CDN_BASE_URL", ""), "/"), Bucket: c.StringOr("EW_CHAT_ASSET_BUCKET", DefaultAssetBucket),
		StickerGifPerMinute: c.Int("EW_CHAT_STICKER_GIF_RATE", DefaultStickerGifPerMinute), StickerMaxBytes: DefaultStickerMaxBytes,
		GifMaxBytes: DefaultGifMaxBytes, GifMaxPixels: DefaultGifMaxPixels}
	if a.StickerGifPerMinute < 1 {
		return a, errors.New("chat config: EW_CHAT_STICKER_GIF_RATE must be ≥ 1")
	}
	return a, nil
}

// URL is the public CDN URL of an immutable library key (media/<sha256>/…, R31).
func (a Assets) URL(key string) string { return a.CDNBase + "/" + strings.TrimLeft(key, "/") }
