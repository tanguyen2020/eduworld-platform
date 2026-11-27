package report

import (
	"bytes"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSummaryAndMetrics(t *testing.T) {
	m := New("ccu_10k")
	for i := 1; i <= 100; i++ {
		m.HTTP("submitActivityAttempt", 200, time.Duration(i)*time.Millisecond)
	}
	m.HTTP("submitActivityAttempt", 503, time.Millisecond)
	m.Login("ok", 2*time.Second)
	m.LoginRetries(3, 0, 1)
	m.WS("chat_send", 40*time.Millisecond, nil)
	m.WS("chat_send", 0, errors.New("x"))
	m.Sent("hub_move")
	m.Connected(1)
	s := m.Summary(time.Unix(0, 0), time.Unix(60, 0))
	if q := s.Latencies["http_submitActivityAttempt"]; q.Count != 101 || q.P95 < 94 || q.P95 > 97 {
		t.Fatalf("quantiles %+v", q)
	}
	if s.Counts["http_submitActivityAttempt_5xx"] != 1 || s.Counts["login_queue_busy"] != 3 || s.Counts["ws_chat_send_failed"] != 1 {
		t.Fatalf("counts %v", s.Counts)
	}
	var b bytes.Buffer
	if err := s.WriteJSON(&b); err != nil || !strings.Contains(b.String(), `"p95_ms"`) {
		t.Fatalf("json %q %v", b.String(), err)
	}
	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	body, _ := io.ReadAll(rec.Body)
	for _, want := range []string{`eduworld_loadbot_http_requests_total{op="submitActivityAttempt",scenario="ccu_10k",status_class="5xx"} 1`,
		`eduworld_loadbot_logins_total{result="queue_busy",scenario="ccu_10k"} 3`, `eduworld_loadbot_ws_connected{scenario="ccu_10k"} 1`} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("metrics lack %s:\n%s", want, body)
		}
	}
}

func TestStatusClass(t *testing.T) {
	if StatusClass(0) != "error" || StatusClass(429) != "4xx" || StatusClass(201) != "2xx" {
		t.Fatal("status class")
	}
}
