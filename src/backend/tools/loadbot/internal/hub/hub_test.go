package hub

import (
	"context"
	"encoding/json"
	"math"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/gameclient/fakert"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/gameclient/wsclient"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/loadbot/internal/report"
)

// The walk stays within max_move_hz (10 Hz) and below max_speed_mps, with increasing client_seq.
func TestJoinAndWalkRespectRateAndSpeed(t *testing.T) {
	srv := fakert.New("tk") // test stand-in of the gateway
	srv.SetReply(func(in fakert.In) []fakert.Out {
		out := fakert.Ack(in)
		if in.Type == "hub.instance.join" {
			out = append(out, fakert.Out{Type: "hub.instance.assigned", Payload: map[string]any{"room_id": "hub:main_hub:1", "area_id": "main_hub",
				"self_eid": 1, "spawn": []float64{0, 0, 0}, "tick_rate_hz": 10, "max_speed_mps": 5}})
		}
		return out
	})
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := wsclient.Dial(ctx, srv.URL(), "tk")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	m := report.New("hub")
	in, err := Join(ctx, c, m, "main_hub", 2*time.Second)
	if err != nil || in.RoomID != "hub:main_hub:1" {
		t.Fatalf("join %+v %v", in, err)
	}
	wctx, wcancel := context.WithTimeout(ctx, 1200*time.Millisecond)
	defer wcancel()
	_ = Walk(wctx, c, m, in, Options{Hz: 10, WalkShare: 1, Phase: time.Second}, rand.New(rand.NewPCG(1, 1)))
	time.Sleep(100 * time.Millisecond)
	var moves []fakert.In
	for _, e := range srv.Received() {
		if e.Type == "hub.player.move" {
			moves = append(moves, e)
		}
	}
	if len(moves) < 8 || len(moves) > 13 {
		t.Fatalf("%d moves in 1.2 s, want about 10 Hz", len(moves))
	}
	prev := [3]float64{}
	var prevSeq int64
	for _, mv := range moves {
		var p struct {
			Room string     `json:"room_id"`
			Seq  int64      `json:"client_seq"`
			P    [3]float64 `json:"p"`
		}
		if err := json.Unmarshal(mv.Payload, &p); err != nil || p.Room != in.RoomID || p.Seq <= prevSeq {
			t.Fatalf("move %s", mv.Payload)
		}
		if d := math.Hypot(p.P[0]-prev[0], p.P[2]-prev[2]); d > 5.0/10+0.02 {
			t.Fatalf("step %.3f m above max_speed/Hz", d)
		}
		prev, prevSeq = p.P, p.Seq
	}
}
