package health_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/buildinfo"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/health"
)

// fakeNATS is a minimal NATS protocol server (INFO, CONNECT, PING/PONG, PUB/HPUB/SUB swallowed) whose PONGs can be
// suspended to reproduce a paused server: the TCP connection stays open, so the client still reports CONNECTED.
type fakeNATS struct {
	ln     net.Listener
	paused atomic.Bool
}

func startFakeNATS(t *testing.T) *fakeNATS {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeNATS{ln: ln}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go f.serve(c)
		}
	}()
	return f
}

func (f *fakeNATS) url() string { return "nats://" + f.ln.Addr().String() }

func (f *fakeNATS) serve(c net.Conn) {
	defer func() { _ = c.Close() }()
	_, _ = fmt.Fprintf(c, "INFO {\"server_id\":\"fake\",\"version\":\"2.10.0\",\"proto\":1,\"headers\":true,\"max_payload\":1048576}\r\n")
	r := bufio.NewReader(c)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch strings.ToUpper(fields[0]) {
		case "PING":
			if !f.paused.Load() {
				_, _ = io.WriteString(c, "PONG\r\n")
			}
		case "PUB", "HPUB": // payload size is the last field
			n, _ := strconv.Atoi(fields[len(fields)-1])
			if _, err := io.CopyN(io.Discard, r, int64(n+2)); err != nil {
				return
			}
		}
	}
}

func connectFake(t *testing.T, f *fakeNATS) *nats.Conn {
	t.Helper()
	nc, err := nats.Connect(f.url(), nats.NoReconnect())
	if err != nil {
		t.Fatalf("connect fake NATS: %v", err)
	}
	t.Cleanup(nc.Close)
	return nc
}

// The readiness check probes the server (F132 finding: `nats: ok` while NATS was paused for 30 s).
func TestNATSCheckFailsWhenServerPaused(t *testing.T) {
	f := startFakeNATS(t)
	nc := connectFake(t, f)
	if err := health.NATSCheck(nc)(context.Background()); err != nil {
		t.Fatalf("NATSCheck on an answering server = %v, want nil", err)
	}
	f.paused.Store(true)
	if !nc.IsConnected() {
		t.Fatal("client should still report CONNECTED while the server is paused")
	}
	start := time.Now()
	err := health.ProbeNATS(context.Background(), nc, 200*time.Millisecond)
	if err == nil {
		t.Fatal("ProbeNATS on a paused server = nil, want an error")
	}
	if code := health.ErrorCode(err); code != health.CodeTimeout {
		t.Fatalf("error code = %q, want %q", code, health.CodeTimeout)
	}
	if d := time.Since(start); d > time.Second {
		t.Fatalf("probe not bounded by its timeout: took %v", d)
	}
}

// The registry deadline bounds the probe as well (2 s budget of /readyz, nats_readiness_probe_timeout 1 s).
func TestNATSCheckBoundedByReadyz(t *testing.T) {
	f := startFakeNATS(t)
	nc := connectFake(t, f)
	f.paused.Store(true)
	r := health.NewRegistry(buildinfo.Get("core-api", ""))
	r.SetTimeout(300 * time.Millisecond)
	r.Register("nats", health.Readiness, health.NATSCheck(nc))
	rep := r.Evaluate(context.Background(), false)
	if rep.Status != "fail" || rep.Checks["nats"].Status != "fail" || rep.Checks["nats"].ErrorCode != health.CodeTimeout {
		t.Fatalf("readyz with paused NATS = %+v, want nats fail/timeout", rep)
	}
}

func TestNATSCheckNotConnectedAndNil(t *testing.T) {
	f := startFakeNATS(t)
	nc := connectFake(t, f)
	nc.Close()
	if code := health.ErrorCode(health.NATSCheck(nc)(context.Background())); code != health.CodeNotConnected {
		t.Fatalf("closed connection code = %q, want %q", code, health.CodeNotConnected)
	}
	if code := health.ErrorCode(health.NATSCheck(nil)(context.Background())); code != health.CodeNotConfigured {
		t.Fatalf("nil connection code = %q, want %q", code, health.CodeNotConfigured)
	}
}

func TestNATSConnectedGauge(t *testing.T) {
	f := startFakeNATS(t)
	nc := connectFake(t, f)
	g := health.NATSConnectedGauge(nc)
	reg := prometheus.NewRegistry()
	reg.MustRegister(g)
	if v := testutil.ToFloat64(g); v != 1 {
		t.Fatalf("eduworld_nats_connected = %v with an answering server, want 1", v)
	}
	f.paused.Store(true)
	if v := testutil.ToFloat64(g); v != 0 {
		t.Fatalf("eduworld_nats_connected = %v with a paused server, want 0", v)
	}
	if n, err := testutil.GatherAndCount(reg, "eduworld_nats_connected"); err != nil || n != 1 {
		t.Fatalf("eduworld_nats_connected series = %d (%v), want 1", n, err)
	}
	if v := testutil.ToFloat64(health.NATSConnectedGauge(nil)); v != 0 {
		t.Fatalf("eduworld_nats_connected without connection = %v, want 0", v)
	}
}
