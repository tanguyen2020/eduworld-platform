package wsclient

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/gameclient/fakert"
)

func TestDialRequestAndHeartbeat(t *testing.T) {
	srv := fakert.New("tk")
	srv.SetHeartbeat(1)
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, err := Dial(ctx, srv.URL(), "tk")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	if _, rtt, err := c.Request(ctx, "hub.instance.join", map[string]any{"area_id": "main_hub"}, 2*time.Second); err != nil || rtt < 0 {
		t.Fatalf("request: %v rtt %v", err, rtt)
	}
	time.Sleep(1300 * time.Millisecond)
	var ping bool
	for _, in := range srv.Received() {
		if in.V != 1 || len(in.ID) < 10 {
			t.Fatalf("envelope %+v", in)
		}
		ping = ping || in.Type == "system.heartbeat.ping"
	}
	if !ping {
		t.Fatal("no heartbeat ping after heartbeat_s")
	}
}

func TestFailedCommandCarriesCode(t *testing.T) {
	srv := fakert.New("tk")
	srv.SetReply(func(in fakert.In) []fakert.Out {
		return []fakert.Out{{Type: "system.command.failed", Ref: in.ID, Payload: map[string]any{"code": "CHAT_TEMPORARILY_UNAVAILABLE"}}}
	})
	defer srv.Close()
	ctx := context.Background()
	c, err := Dial(ctx, srv.URL(), "tk")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	_, _, err = c.Request(ctx, "chat.message.send", map[string]any{}, time.Second)
	var fe *FailedError
	if !errors.As(err, &fe) || fe.Code != "CHAT_TEMPORARILY_UNAVAILABLE" || !errors.Is(err, ErrFailed) {
		t.Fatalf("err = %v", err)
	}
}

func TestBadTicketFails(t *testing.T) {
	srv := fakert.New("tk")
	defer srv.Close()
	if _, err := Dial(context.Background(), srv.URL(), "wrong"); err == nil {
		t.Fatal("dial with a wrong ticket succeeded")
	}
}
