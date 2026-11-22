package port

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// CursorBuffer coalesces the read receipts of one member of one channel (ew:chat:cursor_buf, CHAT_SYSTEM §8.2): at most one
// durable write per window, the newest seq of the window written by a trailing flush (F117-AC-13).
type CursorBuffer interface {
	// Gate opens the window of (channel, user); first false while a window is open (the receipt is buffered instead).
	Gate(ctx context.Context, channel, user uuid.UUID, window time.Duration) (first bool, err error)
	// Hold keeps the highest seq buffered in the window; schedule is true for the first held seq of the window (the
	// caller schedules one trailing flush).
	Hold(ctx context.Context, channel, user uuid.UUID, seq int64, window time.Duration) (schedule bool, err error)
	// Take removes and returns the buffered seq (ok false: nothing buffered).
	Take(ctx context.Context, channel, user uuid.UUID) (seq int64, ok bool, err error)
}

// UnreadPush pushes chat.unread.updated {counts: {channel_id: n}} to every connection of a user (all devices).
type UnreadPush interface {
	Unread(ctx context.Context, user uuid.UUID, counts map[uuid.UUID]int64) error
}
