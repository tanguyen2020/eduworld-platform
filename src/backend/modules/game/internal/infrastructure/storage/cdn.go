package storage

import (
	"strings"
)

// PublicURL builds the CDN URL of an object of ew-media-public (ADR-0044 §2): <base>/<bucket>/<key>. Game media is
// platform content and never private (no signed URL); exam material never goes through module game.
type PublicURL struct {
	BaseURL string // EW_CDN_BASE_URL, e.g. http://localhost:8090
}

// URL returns the CDN URL of key in bucket ("" when no CDN base URL is configured).
func (p PublicURL) URL(bucket, key string) string {
	if p.BaseURL == "" {
		return ""
	}
	return strings.TrimRight(p.BaseURL, "/") + "/" + bucket + "/" + strings.TrimLeft(key, "/")
}
