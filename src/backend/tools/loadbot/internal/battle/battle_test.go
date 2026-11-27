package battle

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/gameclient/fakert"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/gameclient/wsclient"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/loadbot/internal/report"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/loadbot/internal/restcall"
)

// The bot queues over REST, accepts the proposal, answers each shown question with a raw response and never a score.
func TestPlayAnswersEveryQuestionWithoutScore(t *testing.T) {
	srv := fakert.New("tk") // test stand-in of the gateway
	defer srv.Close()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/competition/matchmaking/tickets" || r.Header.Get("Idempotency-Key") == "" {
			t.Errorf("request %s %v", r.URL.Path, r.Header)
		}
		w.WriteHeader(http.StatusCreated)
		go func() {
			time.Sleep(50 * time.Millisecond)
			srv.Push(fakert.Out{Type: "matchmaking.proposal.created", Payload: map[string]any{"proposal_id": "p1", "ticket_id": "t1"}})
			for i := 0; i < 3; i++ {
				time.Sleep(30 * time.Millisecond)
				srv.Push(fakert.Out{Type: "battle.question.shown", Payload: map[string]any{"match_id": "m1",
					"question": map[string]any{"question_instance_id": "q" + string(rune('a'+i)), "options": []map[string]any{{"option_id": "o1"}}}}})
			}
			time.Sleep(200 * time.Millisecond)
			srv.Push(fakert.Out{Type: "battle.match.ended", Payload: map[string]any{"match_id": "m1", "status": "completed"}})
		}()
	}))
	defer api.Close()
	ctx := context.Background()
	c, err := wsclient.Dial(ctx, srv.URL(), "tk")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	m := report.New("battle")
	rc := &restcall.Client{HTTP: api.Client(), BaseURL: api.URL + "/api/v1", Token: "tok", TenantID: "t", Metrics: m}
	n, err := Play(ctx, rc, c, m, Options{Mode: "ranked_1v1", SubjectID: "s", Think: func() time.Duration { return 10 * time.Millisecond }, Timeout: 5 * time.Second})
	if err != nil || n != 3 {
		t.Fatalf("answered %d, %v", n, err)
	}
	var accepted bool
	for _, e := range srv.Received() {
		var p map[string]any
		_ = json.Unmarshal(e.Payload, &p)
		if e.Type == "battle.answer.submit" {
			for _, k := range []string{"score", "is_correct", "xp", "rating"} {
				if _, ok := p[k]; ok {
					t.Fatalf("client-authoritative field %s sent (ADR-0016)", k)
				}
			}
		}
		accepted = accepted || (e.Type == "matchmaking.proposal.accept" && p["proposal_id"] == "p1")
	}
	if !accepted {
		t.Fatal("proposal not accepted")
	}
}
