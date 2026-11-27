package attempt

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/loadbot/internal/report"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/loadbot/internal/restcall"
)

// TEST STAND-IN of the learning attempt API: start returns two choice items, responses and submit answer 200.
func TestRunAnswersAndSubmitsWithIdempotencyKey(t *testing.T) {
	var mu sync.Mutex
	var keys []string
	var bodies []map[string]any
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		var b map[string]any
		_ = json.NewDecoder(r.Body).Decode(&b)
		bodies = append(bodies, b)
		switch {
		case strings.HasSuffix(r.URL.Path, "/attempts"):
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"attempt_id":"a1","mode":"practice","status":"in_progress","started_at":"x","items":[
				{"position":1,"question_version_id":"q1","type":"single_choice","render_payload":{"options":[{"option_id":"o1"}]}},
				{"position":2,"question_version_id":"q2","type":"single_choice","render_payload":{"options":[{"id":"o2"}]}}]}`))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer api.Close()
	m := report.New("attempt")
	rc := &restcall.Client{HTTP: api.Client(), BaseURL: api.URL, Token: "t", TenantID: "x", Metrics: m}
	res, err := Run(context.Background(), rc, m, Options{ActivityID: "act", Think: func() time.Duration { return 0 }})
	if err != nil || res.Answered != 2 || !res.Submitted {
		t.Fatalf("res %+v %v", res, err)
	}
	if len(keys) != 4 || keys[3] == "" || keys[3] == keys[2] {
		t.Fatalf("idempotency keys %v", keys)
	}
	for _, b := range bodies {
		for _, k := range []string{"score", "is_correct", "xp"} {
			if _, ok := b[k]; ok {
				t.Fatalf("client-authoritative field %s (ADR-0016)", k)
			}
		}
	}
	if resp := bodies[2]["response"].(map[string]any); resp["option_id"] != "o2" {
		t.Fatalf("response %v", resp)
	}
}
