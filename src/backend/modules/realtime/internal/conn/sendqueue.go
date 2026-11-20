package conn

import (
	"context"
	"sync"
	"time"
)

// item is one frame waiting for the writer (data already carries its seq), or a close marker.
type item struct {
	data        []byte
	seq         int64
	typ         string
	ts          int64
	droppable   bool
	buffer      bool // non-droppable, non-transient: appended to the resume buffer before the write
	closeCode   int  // close marker: close the connection with this code once every earlier frame is written
	closeReason string
}

// sendQueue is the bounded send queue of a connection (ws_send_queue, REALTIME_ARCHITECTURE §8): when full the
// oldest droppable frame (snapshot, presence) is dropped first; non-droppable frames are never dropped — the queue
// overflows and the session closes with 4408 when it stays over capacity longer than the send stall (5 s) or
// reaches twice its capacity, so the client resumes instead of losing messages.
type sendQueue struct {
	mu        sync.Mutex
	items     []item
	max       int
	notify    chan struct{}
	overSince time.Time
}

func newSendQueue(max int) *sendQueue {
	return &sendQueue{max: max, notify: make(chan struct{}, 1)}
}

// push appends it; dropped is the droppable frame removed (or it itself), hardFull reports 2× capacity.
func (q *sendQueue) push(it item, now time.Time) (dropped *item, hardFull bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	defer q.signal()
	if it.closeCode != 0 || len(q.items) < q.max {
		q.items = append(q.items, it)
		return nil, false
	}
	for i := range q.items {
		if q.items[i].droppable {
			d := q.items[i]
			q.items = append(q.items[:i], q.items[i+1:]...)
			q.items = append(q.items, it)
			return &d, false
		}
	}
	if it.droppable {
		return &it, false
	}
	q.items = append(q.items, it)
	if q.overSince.IsZero() {
		q.overSince = now
	}
	return nil, len(q.items) >= 2*q.max
}

func (q *sendQueue) signal() {
	select {
	case q.notify <- struct{}{}:
	default:
	}
}

// pop waits for the next item until ctx ends.
func (q *sendQueue) pop(ctx context.Context) (item, bool) {
	for {
		q.mu.Lock()
		if len(q.items) > 0 {
			it := q.items[0]
			q.items[0] = item{}
			q.items = q.items[1:]
			if len(q.items) <= q.max {
				q.overSince = time.Time{}
			}
			q.mu.Unlock()
			return it, true
		}
		q.mu.Unlock()
		select {
		case <-ctx.Done():
			return item{}, false
		case <-q.notify:
		}
	}
}

// drain removes every queued item (session end: non-droppable frames go to the resume buffer).
func (q *sendQueue) drain() []item {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := q.items
	q.items = nil
	return out
}

// stalled reports a queue over capacity for longer than d.
func (q *sendQueue) stalled(now time.Time, d time.Duration) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return !q.overSince.IsZero() && now.Sub(q.overSince) >= d
}

func (q *sendQueue) len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}
