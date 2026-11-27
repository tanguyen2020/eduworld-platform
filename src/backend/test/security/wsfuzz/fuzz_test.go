package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/envguard"
)

const testMaxFrame = 1024

// fakeGateway is a STAND-IN realtime gateway for the fuzzer tests only (AGENTS R17): it mimics the answers of
// modules/realtime (welcome, system.command.failed VALIDATION_FAILED/UNKNOWN_TYPE, read limit 4×ws_max_frame,
// 4429 on flood). It proves the fuzzer classifies answers, not that the real gateway resists — the nightly
// staging job does.
type fakeGateway struct {
	crashOnMalformed bool // stop serving after the first malformed frame (the crash the fuzzer must detect)
	answerInternal   bool // answer INTERNAL to an unknown type
	dropNoClose      bool // drop TCP without a close frame on oversize
	down             atomic.Bool
}

func (g *fakeGateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if g.down.Load() {
		http.Error(w, "down", http.StatusBadGateway)
		return
	}
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{Subprotocol}})
	if err != nil {
		return
	}
	defer func() { _ = c.CloseNow() }()
	c.SetReadLimit(4 * testMaxFrame)
	ctx := r.Context()
	send := func(typ string, payload any) {
		b, _ := json.Marshal(map[string]any{"v": 1, "type": typ, "seq": 1, "ts": 0, "payload": payload})
		_ = c.Write(ctx, websocket.MessageText, b)
	}
	send("system.session.welcome", map[string]any{})
	frames := 0
	for {
		mt, b, err := c.Read(ctx)
		if err != nil {
			if g.dropNoClose && websocket.CloseStatus(err) == -1 {
				return // CloseNow without a close frame
			}
			return
		}
		frames++
		if frames > 60 {
			_ = c.Close(4429, "rate_limited")
			return
		}
		if len(b) > testMaxFrame {
			if g.dropNoClose {
				return
			}
			send("system.command.failed", map[string]any{"code": "VALIDATION_FAILED"})
			continue
		}
		if mt == websocket.MessageBinary {
			_ = c.Close(websocket.StatusUnsupportedData, "binary")
			return
		}
		var env struct {
			V    int    `json:"v"`
			Type string `json:"type"`
		}
		if json.Unmarshal(b, &env) != nil || env.V != 1 || env.Type == "" {
			if g.crashOnMalformed {
				g.down.Store(true)
				return
			}
			send("system.command.failed", map[string]any{"code": "VALIDATION_FAILED"})
			continue
		}
		switch {
		case env.Type == "system.heartbeat.ping":
			send("system.heartbeat.pong", map[string]any{})
		case strings.HasPrefix(env.Type, "chat.") || strings.HasPrefix(env.Type, "battle."):
			send("system.command.failed", map[string]any{"code": "VALIDATION_FAILED"})
		default:
			code := "UNKNOWN_TYPE"
			if g.answerInternal {
				code = "INTERNAL"
			}
			send("system.command.failed", map[string]any{"code": code})
		}
	}
}

func dialer(t *testing.T, g *fakeGateway) Dialer {
	t.Helper()
	srv := httptest.NewServer(g)
	t.Cleanup(srv.Close)
	ws := "ws" + strings.TrimPrefix(srv.URL, "http")
	return TicketDialer(ws, "", func(context.Context) (string, error) { return "t", nil })
}

// F132-AC-18 (T025): the corpus covers the four categories, the oversize frames exceed ws_max_frame.
func TestCorpus(t *testing.T) {
	cats := map[string]int{}
	for _, f := range Corpus(testMaxFrame, 100) {
		cats[f.Category]++
		if f.Category == "oversize" && len(f.Data) <= testMaxFrame {
			t.Fatalf("%s is not above ws_max_frame", f.Name)
		}
		if f.Category == "flood" && f.Repeat != 100 {
			t.Fatalf("flood repeat %d", f.Repeat)
		}
	}
	for _, c := range []string{"malformed", "unknown_type", "oversize", "flood"} {
		if cats[c] == 0 {
			t.Fatalf("no %s frame", c)
		}
	}
}

func TestRunAgainstResilientGateway(t *testing.T) {
	rep := Run(context.Background(), dialer(t, &fakeGateway{}), Corpus(testMaxFrame, 100), 500*time.Millisecond)
	if rep.Crashed || rep.Failures != 0 {
		t.Fatalf("crashed=%v failures=%d %+v", rep.Crashed, rep.Failures, rep.Outcomes)
	}
	var flood Outcome
	for _, o := range rep.Outcomes {
		if o.Category == "flood" {
			flood = o
		}
	}
	if flood.Answer != "closed" || flood.CloseCode != 4429 {
		t.Fatalf("flood outcome %+v", flood)
	}
	if len(rep.Liveness) != 4 {
		t.Fatalf("liveness %v", rep.Liveness)
	}
}

// A gateway that stops accepting connections after a malformed frame is reported as crashed.
func TestRunDetectsCrash(t *testing.T) {
	rep := Run(context.Background(), dialer(t, &fakeGateway{crashOnMalformed: true}), Corpus(testMaxFrame, 100), 300*time.Millisecond)
	if !rep.Crashed || rep.Failures == 0 {
		t.Fatalf("crash not detected: %+v", rep)
	}
}

func TestRunDetectsInternalAndDroppedConnections(t *testing.T) {
	rep := Run(context.Background(), dialer(t, &fakeGateway{answerInternal: true}), Corpus(testMaxFrame, 10), 300*time.Millisecond)
	if rep.Failures == 0 {
		t.Fatal("INTERNAL answer not reported")
	}
	rep = Run(context.Background(), dialer(t, &fakeGateway{dropNoClose: true}), Corpus(testMaxFrame, 10), 300*time.Millisecond)
	bad := false
	for _, o := range rep.Outcomes {
		if o.Category == "oversize" && !o.OK {
			bad = true
		}
	}
	if !bad {
		t.Fatalf("drop without close frame not reported: %+v", rep.Outcomes)
	}
}

func TestSumMetric(t *testing.T) {
	text := "# HELP eduworld_ws_connections x\n# TYPE eduworld_ws_connections gauge\neduworld_ws_connections{node=\"a\"} 3\neduworld_ws_connections{node=\"b\"} 4\neduworld_ws_connections_total 99\n"
	v, err := sumMetric(strings.NewReader(text), "eduworld_ws_connections")
	if err != nil || v != 7 {
		t.Fatalf("sum = %v, %v", v, err)
	}
	if _, err := sumMetric(strings.NewReader("other 1\n"), "eduworld_ws_connections"); err == nil {
		t.Fatal("missing metric accepted")
	}
}

// F132-AC-07: refuses production before any request.
func TestRefusesProduction(t *testing.T) {
	t.Setenv(envguard.Var, "production")
	err := run(context.Background(), nil, func(k string) string {
		if k == "EW_BASE_URL" {
			return "http://127.0.0.1:1/api/v1"
		}
		return ""
	})
	if !errors.Is(err, envguard.ErrRefused) {
		t.Fatal(err)
	}
}
