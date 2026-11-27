// Package ids generates the identifiers a game client sends: UUIDv7 for Idempotency-Key (must be recent and
// time-ordered, API_DESIGN) and WebSocket command ids, UUIDv4 for client_message_id.
package ids

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"time"
)

// V7 returns a UUIDv7 for time t.
func V7(t time.Time) string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	ms := uint64(t.UnixMilli())
	var ts [8]byte
	binary.BigEndian.PutUint64(ts[:], ms)
	copy(b[0:6], ts[2:8])
	b[6] = b[6]&0x0f | 0x70
	b[8] = b[8]&0x3f | 0x80
	return format(b)
}

// V4 returns a random UUIDv4.
func V4() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return format(b)
}

func format(b [16]byte) string {
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
