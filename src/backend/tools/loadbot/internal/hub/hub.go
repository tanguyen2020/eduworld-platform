// Package hub is the hub behaviour of a bot (TEST_STRATEGY §8.1, TC-PERF-003): join an instance of an area, then
// alternate walking (~40% of the time, hub.player.move at ≤ 10 Hz — max_move_hz of the welcome) and standing idle.
// The walk stays below max_speed_mps of hub.instance.assigned so a legitimate bot is never snapped back; every
// hub.player.corrected is counted as an error of the run (it would mean the server rejects normal movement).
package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/gameclient/wsclient"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/loadbot/internal/report"
)

// Instance is the assignment of hub.instance.assigned.
type Instance struct {
	RoomID   string     `json:"room_id"`
	AreaID   string     `json:"area_id"`
	Spawn    [3]float64 `json:"spawn"`
	TickHz   int        `json:"tick_rate_hz"`
	MaxSpeed float64    `json:"max_speed_mps"`
}

// Options of the walk.
type Options struct {
	Hz         int           // moves per second while walking, ≤ 10
	WalkShare  float64       // share of time walking (0.4)
	Phase      time.Duration // length of one walk/idle phase
	SpeedRatio float64       // fraction of max speed (0.6)
}

// Join sends hub.instance.join and waits for hub.instance.assigned.
func Join(ctx context.Context, c *wsclient.Conn, m *report.Metrics, area string, timeout time.Duration) (Instance, error) {
	got := make(chan Instance, 1)
	c.On("hub.instance.assigned", func(e wsclient.Envelope) {
		var in Instance
		if json.Unmarshal(e.Payload, &in) == nil {
			select {
			case got <- in:
			default:
			}
		}
	})
	_, rtt, err := c.Request(ctx, "hub.instance.join", map[string]any{"area_id": area}, timeout)
	m.WS("hub_join", rtt, err)
	if err != nil {
		return Instance{}, err
	}
	select {
	case in := <-got:
		if in.MaxSpeed <= 0 {
			in.MaxSpeed = 5
		}
		return in, nil
	case <-time.After(timeout):
		return Instance{}, fmt.Errorf("hub: no hub.instance.assigned within %s", timeout)
	case <-ctx.Done():
		return Instance{}, ctx.Err()
	}
}

// Walk runs until ctx ends.
func Walk(ctx context.Context, c *wsclient.Conn, m *report.Metrics, in Instance, o Options, r *rand.Rand) error {
	if o.Hz <= 0 || o.Hz > 10 {
		o.Hz = 10
	}
	if o.WalkShare <= 0 {
		o.WalkShare = 0.4
	}
	if o.Phase <= 0 {
		o.Phase = 5 * time.Second
	}
	if o.SpeedRatio <= 0 || o.SpeedRatio > 0.9 {
		o.SpeedRatio = 0.6
	}
	c.On("hub.player.corrected", func(wsclient.Envelope) { m.Error("hub_corrected") })
	pos := in.Spawn
	seq := int64(0)
	step := in.MaxSpeed * o.SpeedRatio / float64(o.Hz)
	tick := time.NewTicker(time.Second / time.Duration(o.Hz))
	defer tick.Stop()
	for {
		walking := r.Float64() < o.WalkShare
		heading := r.Float64() * 2 * math.Pi
		end := time.Now().Add(o.Phase)
		for time.Now().Before(end) {
			select {
			case <-ctx.Done():
				return nil
			case <-c.Done():
				return fmt.Errorf("hub: connection closed")
			case <-tick.C:
			}
			if !walking {
				continue
			}
			pos[0] += step * math.Cos(heading)
			pos[2] += step * math.Sin(heading)
			seq++
			if _, err := c.Send(ctx, "hub.player.move", map[string]any{"room_id": in.RoomID, "client_seq": seq,
				"p": []float64{round(pos[0]), round(pos[1]), round(pos[2])}, "r": round(heading), "a": "walk"}); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				m.Error("hub_send")
				return err
			}
			m.Sent("hub_move")
		}
	}
}

// round quantises to 1 cm like the client (Vec3 of the AsyncAPI).
func round(v float64) float64 { return math.Round(v*100) / 100 }
