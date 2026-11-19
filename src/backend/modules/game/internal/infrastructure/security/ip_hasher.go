// Package security holds the HMAC IP hasher of module game (F014 T023, ADR-0031): login events keep only the
// HMAC-SHA-256 of the client address under a key from the secret store (rotated by redeploying a new key); the raw
// IP is never stored, logged or returned (AGENTS R23, NFR-PRIV-002).
package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"net"
	"strings"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
)

// IPHasher implements port.IPHasher.
type IPHasher struct{ key []byte }

var _ port.IPHasher = IPHasher{}

// NewIPHasher returns a hasher; an empty key disables hashing (Hash returns nil).
func NewIPHasher(key []byte) IPHasher { return IPHasher{key: append([]byte(nil), key...)} }

// Hash returns HMAC-SHA-256(key, normalized ip), nil without a key or an address.
func (h IPHasher) Hash(ip string) []byte {
	ip = strings.TrimSpace(ip)
	if len(h.key) == 0 || ip == "" {
		return nil
	}
	if host, _, err := net.SplitHostPort(ip); err == nil {
		ip = host
	}
	if parsed := net.ParseIP(ip); parsed != nil {
		ip = parsed.String()
	}
	m := hmac.New(sha256.New, h.key)
	_, _ = m.Write([]byte(ip))
	return m.Sum(nil)
}
