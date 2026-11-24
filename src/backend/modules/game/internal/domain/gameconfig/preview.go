package gameconfig

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
)

// PreviewTTL is preview_token_ttl (CONFIG_DEFAULTS, owner ADMIN_PORTAL §5): 30 minutes.
const PreviewTTL = 30 * time.Minute

// HashToken is the stored form of a preview token (game.preview_sessions.token_hash): SHA-256 hex, never the token.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// PreviewSession is the read grant of one version (F078-AC-17).
type PreviewSession struct {
	ID        uuid.UUID
	Type      string // config_type
	ConfigID  uuid.UUID
	VersionID uuid.UUID
	Version   int
	CreatedBy uuid.UUID
	ExpiresAt time.Time
}

// Allows reports whether the session grants principal the read of (configType, configID, version) at now: only that
// version, only its creator, only before it expires.
func (s PreviewSession) Allows(principal uuid.UUID, configType string, configID uuid.UUID, version int, now time.Time) bool {
	return s.CreatedBy == principal && s.Type == configType && s.ConfigID == configID && s.Version == version && now.Before(s.ExpiresAt)
}
