package natsx_test

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/natsx"
)

func TestErrorReason(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{nats.ErrNoResponders, natsx.ReasonNoResponders},
		{fmt.Errorf("wrap: %w", nats.ErrNoResponders), natsx.ReasonNoResponders},
		{nats.ErrTimeout, natsx.ReasonTimeout},
		{context.DeadlineExceeded, natsx.ReasonTimeout},
		{nats.ErrConnectionClosed, natsx.ReasonUnavailable},
		{nats.ErrConnectionReconnecting, natsx.ReasonUnavailable},
		{nats.ErrConnectionDraining, natsx.ReasonUnavailable},
		{nats.ErrDisconnected, natsx.ReasonUnavailable},
		{nats.ErrNoServers, natsx.ReasonUnavailable},
		{nats.ErrInvalidConnection, natsx.ReasonUnavailable},
		{errors.New("boom"), natsx.ReasonError},
	}
	for _, c := range cases {
		if got := natsx.ErrorReason(c.err); got != c.want {
			t.Errorf("ErrorReason(%v) = %q, want %q", c.err, got, c.want)
		}
	}
}

// silentNATS accepts connections and answers PINGs but never replies to a request (a core-api that does not answer
// in time).
func silentNATS(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer func() { _ = c.Close() }()
				_, _ = io.WriteString(c, "INFO {\"server_id\":\"fake\",\"version\":\"2.10.0\",\"proto\":1,\"headers\":true,\"max_payload\":1048576}\r\n")
				r := bufio.NewReader(c)
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					f := strings.Fields(line)
					if len(f) == 0 {
						continue
					}
					switch strings.ToUpper(f[0]) {
					case "PING":
						_, _ = io.WriteString(c, "PONG\r\n")
					case "PUB", "HPUB":
						n, _ := strconv.Atoi(f[len(f)-1])
						if _, err := io.CopyN(io.Discard, r, int64(n+2)); err != nil {
							return
						}
					}
				}
			}(c)
		}
	}()
	return "nats://" + ln.Addr().String()
}

// Every request is timed (denominator of NatsRequestTimeouts) and a request without a reply counts as a timeout.
func TestRequestRecordsTimeout(t *testing.T) {
	nc, err := nats.Connect(silentNATS(t), nats.NoReconnect())
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	natsx.InitRequestMetrics("testmod_timeout")
	before := testutil.ToFloat64(natsx.RequestErrors().WithLabelValues("testmod_timeout", natsx.ReasonTimeout))
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	_, err = natsx.Request(ctx, nc, "testmod_timeout", "battle", nats.NewMsg("ew.cmd.testmod_timeout.battle.answer"))
	if err == nil {
		t.Fatal("request without reply = nil error")
	}
	if got := testutil.ToFloat64(natsx.RequestErrors().WithLabelValues("testmod_timeout", natsx.ReasonTimeout)) - before; got != 1 {
		t.Fatalf("eduworld_nats_request_errors_total{reason=timeout} += %v, want 1", got)
	}
	if n := testutil.CollectAndCount(natsx.RequestDuration(), "eduworld_nats_request_duration_seconds"); n == 0 {
		t.Fatal("eduworld_nats_request_duration_seconds not observed")
	}
	// InitRequestMetrics exposes every reason at 0.
	if v := testutil.ToFloat64(natsx.RequestErrors().WithLabelValues("testmod_timeout", natsx.ReasonNoResponders)); v != 0 {
		t.Fatalf("no_responders = %v, want 0", v)
	}
}

// A closed connection (NATS cluster unreachable) counts as unavailable; a caller cancellation is not a NATS error.
func TestRequestUnavailableAndCancelled(t *testing.T) {
	nc, err := nats.Connect(silentNATS(t), nats.NoReconnect())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	before := testutil.ToFloat64(natsx.RequestErrors().WithLabelValues("testmod_cancel", natsx.ReasonError)) +
		testutil.ToFloat64(natsx.RequestErrors().WithLabelValues("testmod_cancel", natsx.ReasonTimeout))
	if _, err := natsx.Request(ctx, nc, "testmod_cancel", "hub", nats.NewMsg("ew.cmd.testmod_cancel.hub.x")); err == nil {
		t.Fatal("cancelled request = nil error")
	}
	after := testutil.ToFloat64(natsx.RequestErrors().WithLabelValues("testmod_cancel", natsx.ReasonError)) +
		testutil.ToFloat64(natsx.RequestErrors().WithLabelValues("testmod_cancel", natsx.ReasonTimeout))
	if after != before {
		t.Fatalf("a caller cancellation was counted as a NATS error")
	}

	nc.Close()
	ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second)
	defer cancel2()
	if _, err := natsx.Request(ctx2, nc, "testmod_cancel", "hub", nats.NewMsg("ew.cmd.testmod_cancel.hub.x")); err == nil {
		t.Fatal("request on a closed connection = nil error")
	}
	if v := testutil.ToFloat64(natsx.RequestErrors().WithLabelValues("testmod_cancel", natsx.ReasonUnavailable)); v != 1 {
		t.Fatalf("unavailable = %v, want 1", v)
	}
}
