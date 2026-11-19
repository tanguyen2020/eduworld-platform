package cache

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
)

// LoginTickets implements port.LoginTicketStore (F013 T015, research R1): the ticket state, including the
// Keycloak token set, is sealed with AES-256-GCM under a key from the secret store (EW_GAME_LOGIN_TICKET_KEY)
// before it reaches Valkey; the key of the entry is the SHA-256 of the ticket; GETDEL makes it single use.
type LoginTickets struct {
	C    valkey.Client
	aead cipher.AEAD
}

var _ port.LoginTicketStore = (*LoginTickets)(nil)

// NewLoginTickets builds the store; key must be 32 bytes (AES-256).
func NewLoginTickets(c valkey.Client, key []byte) (*LoginTickets, error) {
	if len(key) != 32 {
		return nil, errors.New("cache: login ticket key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &LoginTickets{C: c, aead: aead}, nil
}

func loginTicketKey(hash string) string { return prefix + "login_ticket:" + hash }

// Save implements port.LoginTicketStore (SET … PXAT expires_at). The ticket hash is the associated data, so
// a sealed value cannot be moved under another key.
func (s *LoginTickets) Save(ctx context.Context, ticketHash string, t port.LoginTicket) error {
	if !t.ExpiresAt.After(time.Now()) {
		return errors.New("cache: login ticket already expired")
	}
	raw, err := json.Marshal(t)
	if err != nil {
		return err
	}
	defer clear(raw)
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	sealed := s.aead.Seal(nonce, nonce, raw, []byte(ticketHash))
	return s.C.Do(ctx, s.C.B().Set().Key(loginTicketKey(ticketHash)).Value(valkey.BinaryString(sealed)).
		Pxat(t.ExpiresAt).Build()).Error()
}

// Take implements port.LoginTicketStore (GETDEL then open).
func (s *LoginTickets) Take(ctx context.Context, ticketHash string) (port.LoginTicket, error) {
	sealed, err := s.C.Do(ctx, s.C.B().Getdel().Key(loginTicketKey(ticketHash)).Build()).AsBytes()
	if valkey.IsValkeyNil(err) {
		return port.LoginTicket{}, port.ErrLoginTicketInvalid
	}
	if err != nil {
		return port.LoginTicket{}, fmt.Errorf("cache: login ticket: %w", err)
	}
	n := s.aead.NonceSize()
	if len(sealed) < n {
		return port.LoginTicket{}, port.ErrLoginTicketInvalid
	}
	raw, err := s.aead.Open(nil, sealed[:n], sealed[n:], []byte(ticketHash))
	if err != nil {
		return port.LoginTicket{}, port.ErrLoginTicketInvalid
	}
	defer clear(raw)
	var t port.LoginTicket
	if err := json.Unmarshal(raw, &t); err != nil || !t.ExpiresAt.After(time.Now()) {
		return port.LoginTicket{}, port.ErrLoginTicketInvalid
	}
	return t, nil
}

// FallbackLimiter implements port.RateLimiter (F013 T024): Valkey first; when Valkey fails, a fixed-window
// counter in the memory of this replica keeps limiting (spec edge case "Valkey lỗi"), so a cache outage
// neither opens brute force nor blocks logins.
type FallbackLimiter struct {
	Primary port.RateLimiter
	mu      sync.Mutex
	local   map[string]*window
	now     func() time.Time
}

type window struct {
	start time.Time
	n     int
}

var _ port.RateLimiter = (*FallbackLimiter)(nil)

// maxLocalKeys bounds the memory of the fallback; older windows are dropped first when it is full.
const maxLocalKeys = 100_000

// NewFallbackLimiter wraps primary (nil: memory only).
func NewFallbackLimiter(primary port.RateLimiter) *FallbackLimiter {
	return &FallbackLimiter{Primary: primary, local: map[string]*window{}, now: time.Now}
}

// Allow implements port.RateLimiter.
func (l *FallbackLimiter) Allow(ctx context.Context, key string, limit int, win time.Duration) (bool, time.Duration, error) {
	if l.Primary != nil {
		ok, after, err := l.Primary.Allow(ctx, key, limit, win)
		if err == nil {
			return ok, after, nil
		}
	}
	return l.allowLocal(key, limit, win), win, nil
}

func (l *FallbackLimiter) allowLocal(key string, limit int, win time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	w, ok := l.local[key]
	if !ok || now.Sub(w.start) >= win {
		if !ok && len(l.local) >= maxLocalKeys {
			for k, v := range l.local {
				if now.Sub(v.start) >= win {
					delete(l.local, k)
				}
			}
			if len(l.local) >= maxLocalKeys {
				return true // memory bound reached: APISIX and Keycloak brute-force detection still apply
			}
		}
		w = &window{start: now}
		l.local[key] = w
	}
	w.n++
	return w.n <= limit
}
