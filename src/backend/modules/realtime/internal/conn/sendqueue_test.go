package conn

import (
	"context"
	"testing"
	"time"
)

// T025 (F090-AC-23): a full queue drops the oldest droppable frame first; non-droppable frames are never dropped,
// overflow is reported as stalled after the send stall and as hard full at twice the capacity.
func TestSendQueue(t *testing.T) {
	now := time.Unix(1000, 0)
	q := newSendQueue(3)
	q.push(item{seq: 1, typ: "a"}, now)
	q.push(item{seq: 2, typ: "snap", droppable: true}, now)
	q.push(item{seq: 3, typ: "b"}, now)
	if d, _ := q.push(item{seq: 4, typ: "c"}, now); d == nil || d.seq != 2 {
		t.Fatalf("oldest droppable not dropped: %+v", d)
	}
	if d, _ := q.push(item{seq: 5, typ: "snap", droppable: true}, now); d == nil || d.seq != 5 {
		t.Fatalf("new droppable on a full queue of important frames must be dropped: %+v", d)
	}
	if d, full := q.push(item{seq: 6, typ: "d"}, now); d != nil || full {
		t.Fatalf("important frame dropped or hard full: %+v %v", d, full)
	}
	if q.stalled(now.Add(4*time.Second), 5*time.Second) || !q.stalled(now.Add(5*time.Second), 5*time.Second) {
		t.Fatal("stall detection")
	}
	q.push(item{seq: 7}, now)
	if _, full := q.push(item{seq: 8}, now); !full {
		t.Fatal("2x capacity must be hard full")
	}
	var seqs []int64
	for q.len() > 0 {
		it, _ := q.pop(context.Background())
		seqs = append(seqs, it.seq)
	}
	want := []int64{1, 3, 4, 6, 7, 8}
	for i := range want {
		if seqs[i] != want[i] {
			t.Fatalf("order %v", seqs)
		}
	}
	if q.stalled(now.Add(time.Hour), 5*time.Second) {
		t.Fatal("drained queue still stalled")
	}
}
