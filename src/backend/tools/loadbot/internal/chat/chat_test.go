package chat

import (
	"context"
	"encoding/json"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/gameclient/fakert"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/gameclient/wsclient"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/loadbot/internal/report"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/loadbot/internal/restcall"
)

func TestPickChannelAndSend(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"channel_id":"c0","sendable_kinds":["quick"]},{"channel_id":"c1","sendable_kinds":["quick","text"]}]}`))
	}))
	defer api.Close()
	m := report.New("chat")
	rc := &restcall.Client{HTTP: api.Client(), BaseURL: api.URL, Token: "t", TenantID: "x", Metrics: m}
	ch, err := PickChannel(context.Background(), rc)
	if err != nil || ch != "c1" {
		t.Fatalf("channel %q %v", ch, err)
	}
	srv := fakert.New("tk") // test stand-in of the gateway; every other message fails as NATS down would
	n := 0
	srv.SetReply(func(in fakert.In) []fakert.Out {
		n++
		if n%2 == 0 {
			return []fakert.Out{{Type: "system.command.failed", Ref: in.ID, Payload: map[string]any{"code": "CHAT_TEMPORARILY_UNAVAILABLE"}}}
		}
		return fakert.Ack(in)
	})
	defer srv.Close()
	c, err := wsclient.Dial(context.Background(), srv.URL(), "tk")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer cancel()
	_ = Send(ctx, c, m, ch, 100*time.Millisecond, rand.New(rand.NewPCG(1, 2)))
	s := m.Summary(time.Now(), time.Now())
	if s.Counts["ws_chat_send_acked"] < 2 || s.Counts["error_chat_CHAT_TEMPORARILY_UNAVAILABLE"] < 1 {
		t.Fatalf("counts %v", s.Counts)
	}
	for _, e := range srv.Received() {
		var p map[string]any
		_ = json.Unmarshal(e.Payload, &p)
		if p["channel_id"] != "c1" || p["kind"] != "text" || p["client_message_id"] == "" {
			t.Fatalf("payload %v", p)
		}
	}
}
