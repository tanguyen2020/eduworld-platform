package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

// Subprotocol of the realtime gateway (AsyncAPI eduworld-realtime, REALTIME_ARCHITECTURE §10).
const Subprotocol = "eduworld.v1"

// Close codes the gateway may legitimately answer a hostile frame with (REALTIME_ARCHITECTURE §3.1 + RFC 6455).
var acceptableClose = map[websocket.StatusCode]bool{
	websocket.StatusNormalClosure:           true,
	websocket.StatusPolicyViolation:         true, // 1008
	websocket.StatusMessageTooBig:           true, // 1009 (frame above the read limit)
	websocket.StatusUnsupportedData:         true, // 1003 (binary frame)
	websocket.StatusInvalidFramePayloadData: true, // 1007 (invalid UTF-8)
	4400:                                    true, // protocol error
	4409:                                    true, // replaced by a newer connection of the same student
	4429:                                    true, // rate limited
}

// Frame is one hostile client message.
type Frame struct {
	Category string // malformed | unknown_type | oversize | flood
	Name     string
	Binary   bool
	Data     []byte
	Repeat   int // flood: number of sends back to back
}

func envelope(typ string, payload any) []byte {
	b, _ := json.Marshal(map[string]any{"v": 1, "type": typ, "id": uuid.NewString(), "ts": time.Now().UnixMilli(), "payload": payload})
	return b
}

// Corpus returns the frames of a run. maxFrame is ws_max_frame (CONFIG_DEFAULTS, REALTIME_ARCHITECTURE); flood is
// the number of frames of the flood case (above ws_frame_rate burst).
func Corpus(maxFrame, flood int) []Frame {
	big := strings.Repeat("x", maxFrame+1)
	huge := strings.Repeat("y", 5*maxFrame)
	deep := strings.Repeat(`{"a":`, 2000) + "1" + strings.Repeat("}", 2000)
	return []Frame{
		{Category: "malformed", Name: "not_json", Data: []byte("hello gateway")},
		{Category: "malformed", Name: "truncated_json", Data: []byte(`{"v":1,"type":"system.heartbeat.ping","id":"`)},
		{Category: "malformed", Name: "json_array", Data: []byte(`[1,2,3]`)},
		{Category: "malformed", Name: "missing_type", Data: []byte(`{"v":1,"id":"` + uuid.NewString() + `","payload":{}}`)},
		{Category: "malformed", Name: "wrong_version", Data: []byte(`{"v":2,"type":"system.heartbeat.ping","id":"` + uuid.NewString() + `","payload":{}}`)},
		{Category: "malformed", Name: "type_not_string", Data: []byte(`{"v":1,"type":42,"id":"` + uuid.NewString() + `","payload":{}}`)},
		{Category: "malformed", Name: "payload_not_object", Data: []byte(`{"v":1,"type":"chat.message.send","id":"` + uuid.NewString() + `","payload":"x"}`)},
		{Category: "malformed", Name: "id_not_uuid", Data: envelopeID("not-a-uuid")},
		{Category: "malformed", Name: "deep_nesting", Data: []byte(`{"v":1,"type":"chat.message.send","id":"` + uuid.NewString() + `","payload":` + deep + `}`)},
		{Category: "malformed", Name: "invalid_utf8", Data: []byte{'{', '"', 'v', '"', ':', '1', ',', '"', 't', 'y', 'p', 'e', '"', ':', '"', 0xff, 0xfe, '"', '}'}},
		{Category: "malformed", Name: "null_bytes", Data: []byte("{\"v\":1,\"type\":\"chat.message.send\u0000\",\"id\":\"x\",\"payload\":{}}")},
		{Category: "malformed", Name: "binary_frame", Binary: true, Data: []byte{0x00, 0x01, 0x02, 0xff}},
		{Category: "unknown_type", Name: "unknown_command", Data: envelope("zz.unknown.command", map[string]any{})},
		{Category: "unknown_type", Name: "server_only_type", Data: envelope("system.session.welcome", map[string]any{})},
		{Category: "unknown_type", Name: "path_like_type", Data: envelope("../../internal/v1/learning", map[string]any{})},
		{Category: "unknown_type", Name: "long_type", Data: envelope(strings.Repeat("t", 2048), map[string]any{})},
		{Category: "unknown_type", Name: "extra_fields_score", Data: envelope("battle.answer.submit", map[string]any{"score": 999999, "is_correct": true})},
		{Category: "oversize", Name: "above_max_frame", Data: envelope("chat.message.send", map[string]any{"text": big})},
		{Category: "oversize", Name: "above_read_limit", Data: envelope("chat.message.send", map[string]any{"text": huge})},
		{Category: "flood", Name: "heartbeat_flood", Data: envelope("system.heartbeat.ping", map[string]any{}), Repeat: flood},
	}
}

func envelopeID(id string) []byte {
	b, _ := json.Marshal(map[string]any{"v": 1, "type": "system.heartbeat.ping", "id": id, "payload": map[string]any{}})
	return b
}

// Outcome of one frame.
type Outcome struct {
	Category string `json:"category"`
	Name     string `json:"name"`
	// Answer: failed (system.command.failed), closed (close code), silent (no answer within the window), crash.
	Answer    string `json:"answer"`
	CloseCode int    `json:"close_code,omitempty"`
	Code      string `json:"code,omitempty"` // system.command.failed code
	OK        bool   `json:"ok"`
	Note      string `json:"note,omitempty"`
}

// Dialer opens a connection to the gateway with a fresh single-use ticket.
type Dialer func(ctx context.Context) (*websocket.Conn, error)

// TicketDialer dials wsURL?ticket=<ticket> with the eduworld.v1 subprotocol and an Origin header.
func TicketDialer(wsURL, origin string, ticket func(ctx context.Context) (string, error)) Dialer {
	return func(ctx context.Context) (*websocket.Conn, error) {
		t, err := ticket(ctx)
		if err != nil {
			return nil, err
		}
		sep := "?"
		if strings.Contains(wsURL, "?") {
			sep = "&"
		}
		h := http.Header{}
		if origin != "" {
			h.Set("Origin", origin)
		}
		c, resp, err := websocket.Dial(ctx, wsURL+sep+"ticket="+t, &websocket.DialOptions{Subprotocols: []string{Subprotocol}, HTTPHeader: h})
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		if err != nil {
			return nil, err
		}
		c.SetReadLimit(1 << 20)
		return c, nil
	}
}

// Alive reports whether a fresh connection gets system.session.welcome (the gateway still accepts and serves).
func Alive(ctx context.Context, dial Dialer, timeout time.Duration) error {
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	c, err := dial(cctx)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer func() { _ = c.Close(websocket.StatusNormalClosure, "wsfuzz liveness") }()
	for {
		_, b, err := c.Read(cctx)
		if err != nil {
			return fmt.Errorf("no welcome: %w", err)
		}
		var env struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(b, &env) == nil && env.Type == "system.session.welcome" {
			return nil
		}
	}
}

// Send sends f on a fresh connection and classifies the answer within window. A frame is OK when the gateway
// answers system.command.failed, closes with an acceptable code, or stays silent (dropped) — and is still alive.
func Send(ctx context.Context, dial Dialer, f Frame, window time.Duration) Outcome {
	o := Outcome{Category: f.Category, Name: f.Name}
	cctx, cancel := context.WithTimeout(ctx, window+5*time.Second)
	defer cancel()
	c, err := dial(cctx)
	if err != nil {
		o.Answer, o.Note = "crash", "dial: "+err.Error()
		return o
	}
	defer func() { _ = c.CloseNow() }()
	typ := websocket.MessageText
	if f.Binary {
		typ = websocket.MessageBinary
	}
	n := max(f.Repeat, 1)
	for i := 0; i < n; i++ {
		if err := c.Write(cctx, typ, f.Data); err != nil {
			break // the gateway closed while we were flooding: read the close code below
		}
	}
	rctx, rcancel := context.WithTimeout(cctx, window)
	defer rcancel()
	for {
		_, b, err := c.Read(rctx)
		if err != nil {
			code := websocket.CloseStatus(err)
			switch {
			case errors.Is(rctx.Err(), context.DeadlineExceeded) && code == -1:
				o.Answer, o.OK = "silent", true
			case code != -1:
				o.Answer, o.CloseCode, o.OK = "closed", int(code), acceptableClose[code]
				if !o.OK {
					o.Note = "unexpected close code"
				}
			default:
				o.Answer, o.Note = "closed", "connection dropped without a close frame: "+err.Error()
			}
			return o
		}
		var env struct {
			Type    string `json:"type"`
			Payload struct {
				Code string `json:"code"`
			} `json:"payload"`
		}
		if json.Unmarshal(b, &env) != nil {
			continue
		}
		if env.Type == "system.command.failed" {
			o.Answer, o.Code, o.OK = "failed", env.Payload.Code, env.Payload.Code != "INTERNAL"
			if !o.OK {
				o.Note = "INTERNAL answer to a hostile frame"
			}
			if f.Repeat <= 1 {
				return o
			}
		}
	}
}

// Report of a run.
type Report struct {
	Env       string    `json:"env"`
	Target    string    `json:"target"`
	StartedAt time.Time `json:"started_at"`
	Outcomes  []Outcome `json:"outcomes"`
	// Liveness after every category; LeakDelta is eduworld_ws_connections after the run minus before (when a metrics
	// URL is given; > LeakTolerance = leak).
	Liveness  []string `json:"liveness"`
	LeakDelta *float64 `json:"leak_delta,omitempty"`
	Crashed   bool     `json:"crashed"`
	Failures  int      `json:"failures"`
}

// Run sends the corpus, checks liveness after each category and stops at the first crash.
func Run(ctx context.Context, dial Dialer, corpus []Frame, window time.Duration) *Report {
	rep := &Report{StartedAt: time.Now()}
	if err := Alive(ctx, dial, 10*time.Second); err != nil {
		rep.Crashed, rep.Failures = true, 1
		rep.Liveness = append(rep.Liveness, "before: "+err.Error())
		return rep
	}
	for i, f := range corpus {
		o := Send(ctx, dial, f, window)
		rep.Outcomes = append(rep.Outcomes, o)
		if !o.OK {
			rep.Failures++
		}
		if i == len(corpus)-1 || corpus[i+1].Category != f.Category {
			if err := Alive(ctx, dial, 10*time.Second); err != nil {
				rep.Crashed = true
				rep.Failures++
				rep.Liveness = append(rep.Liveness, "after "+f.Category+": "+err.Error())
				return rep
			}
			rep.Liveness = append(rep.Liveness, "after "+f.Category+": ok")
		}
	}
	return rep
}
