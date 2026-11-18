// Package page implements keyset pagination cursors (API_DESIGN §5: ?limit=20&cursor=<opaque>, limit
// default 20, at most 100; the cursor is base64url of (sort key, id) signed with HMAC so clients cannot
// forge positions [Proposed]). The key comes from configuration (EW_PAGE_CURSOR_KEY), never from code.
package page

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
)

// Limits of API_DESIGN §5.
const (
	DefaultLimit = 20
	MaxLimit     = 100
)

// ErrInvalidCursor: the cursor is malformed or its signature does not match (400 BAD_REQUEST).
var ErrInvalidCursor = errors.New("page: invalid cursor")

// Cursor is the position after the last returned row.
type Cursor struct {
	Key string    `json:"k"`
	ID  uuid.UUID `json:"i"`
}

// Codec signs and verifies cursors.
type Codec struct{ key []byte }

// NewCodec returns a codec; the key must have at least 32 bytes.
func NewCodec(key []byte) (*Codec, error) {
	if len(key) < 32 {
		return nil, errors.New("page: cursor key must have at least 32 bytes")
	}
	return &Codec{key: append([]byte(nil), key...)}, nil
}

func (c *Codec) mac(b []byte) []byte {
	m := hmac.New(sha256.New, c.key)
	m.Write(b)
	return m.Sum(nil)[:16]
}

// Encode returns the opaque cursor of cur.
func (c *Codec) Encode(cur Cursor) string {
	b, _ := json.Marshal(cur)
	return base64.RawURLEncoding.EncodeToString(append(b, c.mac(b)...))
}

// Decode verifies and decodes s; "" returns (nil, nil) for the first page.
func (c *Codec) Decode(s string) (*Cursor, error) {
	if s == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil || len(raw) <= 16 {
		return nil, ErrInvalidCursor
	}
	body, sig := raw[:len(raw)-16], raw[len(raw)-16:]
	if !hmac.Equal(sig, c.mac(body)) {
		return nil, ErrInvalidCursor
	}
	var cur Cursor
	if err := json.Unmarshal(body, &cur); err != nil || cur.ID == uuid.Nil {
		return nil, ErrInvalidCursor
	}
	return &cur, nil
}

// Limit clamps a requested limit (0 = default).
func Limit(n int) int {
	switch {
	case n <= 0:
		return DefaultLimit
	case n > MaxLimit:
		return MaxLimit
	}
	return n
}

// Info is PageInfo of the contract.
type Info struct {
	NextCursor *string `json:"next_cursor"`
	HasMore    bool    `json:"has_more"`
	Limit      int     `json:"limit"`
}
