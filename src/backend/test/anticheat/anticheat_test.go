package anticheat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/envguard"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/gameclient/fakert"
)

// standIn is a TEST STAND-IN of the public API and realtime gateway (AGENTS R17): it implements just enough of the
// contract for the harness logic to be exercised, honestly or as a cheating server (broken = the rule under test is
// violated). It proves the scenarios detect a violation; it proves nothing about the platform, which only a run on
// staging (make anticheat ENV=staging) does.
type standIn struct {
	t      *testing.T
	api    *httptest.Server
	rt     *fakert.Server
	broken map[string]bool

	mu            sync.Mutex
	attempts      map[string]*fakeAttempt
	keys          map[string][]byte // Idempotency-Key → stored submit response
	completed     int
	xp            []xpEntry
	sent          map[string]string // ws envelope id → message id
	progressReads int
	nextID        int
}

type fakeAttempt struct {
	submitted   bool
	clientScore float64
	correct     int
}

func newStandIn(t *testing.T, broken ...string) *standIn {
	s := &standIn{t: t, broken: map[string]bool{}, attempts: map[string]*fakeAttempt{}, keys: map[string][]byte{}, sent: map[string]string{}}
	for _, b := range broken {
		s.broken[b] = true
	}
	s.rt = fakert.New("tk-1")
	s.rt.SetReply(s.reply)
	s.api = httptest.NewServer(http.HandlerFunc(s.serve))
	t.Cleanup(func() { s.api.Close(); s.rt.Close() })
	return s
}

func (s *standIn) id(prefix string) string {
	s.nextID++
	return fmt.Sprintf("%s-%032d", prefix, s.nextID)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *standIn) serve(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := strings.TrimPrefix(r.URL.Path, "/api/v1")
	switch {
	case p == "/auth/game/login":
		writeJSON(w, 200, map[string]any{"tokens": map[string]any{"access_token": "tok"},
			"bootstrap": map[string]any{"tenant_id": "t-1", "student": map[string]any{"student_id": "s-1", "grade_level": 5}}})
	case p == "/game/sessions":
		writeJSON(w, 201, map[string]any{"game_session_id": "gs-1"})
	case strings.HasSuffix(p, "/realtime-tickets"):
		writeJSON(w, 201, map[string]any{"ticket": "tk-1", "url": s.rt.URL()})
	case strings.HasPrefix(p, "/learning/activities/") && strings.HasSuffix(p, "/attempts"):
		id := s.id("att")
		s.attempts[id] = &fakeAttempt{}
		writeJSON(w, 201, map[string]any{"attempt_id": id, "status": "in_progress", "items": []any{
			map[string]any{"position": 1, "question_version_id": "q1", "render_payload": map[string]any{"options": []any{map[string]any{"id": "a"}}}},
			map[string]any{"position": 2, "question_version_id": "q2", "render_payload": map[string]any{"options": []any{map[string]any{"id": "b"}}}},
		}})
	case strings.HasSuffix(p, "/responses"):
		a := s.attempts[strings.Split(p, "/")[3]]
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if sc, ok := body["score"].(float64); ok && s.broken["client_score"] {
			a.clientScore = sc // a cheating server trusts the client value
		}
		if body["position"].(float64) == 1 {
			a.correct++ // the server's answer key: option a of item 1 is right, item 2 is wrong
		}
		writeJSON(w, 200, map[string]any{"accepted": true})
	case strings.HasSuffix(p, ":submit"):
		id := strings.TrimSuffix(strings.Split(p, "/")[3], ":submit")
		a := s.attempts[id]
		key := r.Header.Get("Idempotency-Key")
		if b, ok := s.keys[key]; ok && !s.broken["idempotency"] {
			w.Header().Set("Idempotent-Replayed", "true")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = w.Write(b)
			return
		}
		if a.submitted && !s.broken["double_submit"] {
			writeJSON(w, 409, map[string]any{"code": "ATTEMPT_ALREADY_SUBMITTED"})
			return
		}
		a.submitted = true
		s.completed++
		decay := []int{100, 50, 25, 0, 0}
		amount := 0
		if s.completed <= len(decay) {
			amount = decay[s.completed-1] / 10
		}
		if s.broken["xp_farm"] {
			amount = 10
		}
		if amount > 0 {
			s.xp = append(s.xp, xpEntry{Amount: amount, CreatedAt: time.Now()})
		}
		b, _ := json.Marshal(map[string]any{"attempt_id": id, "status": "submitted", "submitted_at": time.Now().Format(time.RFC3339Nano)})
		s.keys[key] = b
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write(b)
	case strings.HasSuffix(p, "/review"):
		a := s.attempts[strings.Split(p, "/")[3]]
		score := float64(a.correct)
		if a.clientScore != 0 {
			score = a.clientScore
		}
		writeJSON(w, 200, map[string]any{"attempt_id": "x", "items": []any{}, "score": score, "max_score": 2})
	case p == "/learning/progress/me":
		s.progressReads++
		version := 0
		if s.broken["record_changed"] {
			version = s.progressReads // a cheating server lets the cheat signal touch the learning record
		}
		writeJSON(w, 200, map[string]any{"objectives": []any{}, "version": version})
	case p == "/game/progression/xp-history":
		writeJSON(w, 200, map[string]any{"data": s.xp, "page": map[string]any{}})
	default:
		writeJSON(w, 404, map[string]any{"code": "NOT_FOUND"})
	}
}

func (s *standIn) reply(in fakert.In) []fakert.Out {
	ack := fakert.Out{Type: "system.command.acked", Ref: in.ID, Payload: map[string]any{"result": map[string]any{}}}
	switch in.Type {
	case "hub.instance.join":
		return []fakert.Out{ack, {Type: "hub.instance.assigned", Payload: map[string]any{"room_id": "hub:main_hub:01", "spawn": []float64{0, 0, 0}, "max_speed_mps": 5}}}
	case "hub.player.move":
		if s.broken["speed"] {
			return nil
		}
		return []fakert.Out{{Type: "hub.player.corrected", Payload: map[string]any{"room_id": "hub:main_hub:01", "client_seq": 1, "p": []float64{0, 0, 0}, "reason": "speed"}}}
	case "chat.message.send":
		var p struct {
			ClientMessageID string `json:"client_message_id"`
		}
		_ = json.Unmarshal(in.Payload, &p)
		s.mu.Lock()
		defer s.mu.Unlock()
		if prev, ok := s.sent[in.ID]; ok && !s.broken["ws_replay"] {
			return []fakert.Out{{Type: "system.command.acked", Ref: in.ID, Payload: map[string]any{"result": json.RawMessage(prev)}}}
		}
		res, _ := json.Marshal(map[string]any{"message_id": s.id("msg"), "client_message_id": p.ClientMessageID})
		s.sent[in.ID] = string(res)
		return []fakert.Out{{Type: "system.command.acked", Ref: in.ID, Payload: map[string]any{"result": json.RawMessage(res)}}}
	}
	return fakert.Ack(in)
}

func (s *standIn) env(t *testing.T) *Env {
	t.Helper()
	cfg := Config{Env: "local", BaseURL: s.api.URL + "/api/v1", Password: "pw", ClientVersion: "1.0.0",
		Fixtures: Fixtures{Primary: Student{Username: "hs.qa.1", DeviceID: "d-1"}, PracticeActivityID: "act-1", ChatChannelID: "ch-1"}}
	cfg.Fixtures.defaults()
	e := NewEnv(cfg, s.api.Client())
	e.Sleep = func(time.Duration) {}
	metric := 0.0
	e.Probe = func(context.Context, string) (float64, error) { // stand-in Prometheus: grows with each correction
		metric++
		if s.broken["metric"] {
			return 1, nil
		}
		return metric, nil
	}
	t.Cleanup(e.Close)
	return e
}

var unit = map[string]func(context.Context, *Env) error{
	"AC-CMP-201": IdempotentSubmit, "AC-CMP-202": SecondSubmitConflict, "AC-CMP-203": ClientScoreIgnored,
	"AC-CMP-208": SpeedHack, "AC-CMP-209": XPDecay, "AC-CMP-210": WSReplay,
}

// The harness passes against a server that follows ANTI_CHEAT §2–§4 (stand-in).
func TestScenariosPassAgainstHonestStandIn(t *testing.T) {
	for id, fn := range unit {
		t.Run(id, func(t *testing.T) {
			s := newStandIn(t)
			if err := fn(context.Background(), s.env(t)); err != nil {
				t.Fatalf("%s against an honest server: %v", id, err)
			}
		})
	}
}

// Each scenario fails against a server breaking the rule it checks (the harness is not vacuous).
func TestScenariosCatchCheatingServer(t *testing.T) {
	for id, broken := range map[string]string{
		"AC-CMP-201": "idempotency", "AC-CMP-202": "double_submit", "AC-CMP-203": "client_score",
		"AC-CMP-208": "speed", "AC-CMP-209": "xp_farm", "AC-CMP-210": "ws_replay",
	} {
		t.Run(id+"/"+broken, func(t *testing.T) {
			s := newStandIn(t, broken)
			err := unit[id](context.Background(), s.env(t))
			if err == nil || errors.Is(err, ErrSkip) {
				t.Fatalf("%s against a server breaking %s: err = %v, want a failure", id, broken, err)
			}
		})
	}
	t.Run("AC-CMP-208/record_changed", func(t *testing.T) {
		s := newStandIn(t, "record_changed")
		if err := SpeedHack(context.Background(), s.env(t)); err == nil || !strings.Contains(err.Error(), "learning record") {
			t.Fatalf("a learning record changed by a cheat signal must fail, got %v", err)
		}
	})
	t.Run("AC-CMP-208/metric", func(t *testing.T) {
		s := newStandIn(t, "metric")
		if err := SpeedHack(context.Background(), s.env(t)); err == nil || !strings.Contains(err.Error(), "eduworld_rt_speed_violations_total") {
			t.Fatalf("metric not increasing must fail, got %v", err)
		}
	})
}

// Scenarios without their fixture are skipped, never passed (SC-005 counts only passed scenarios).
func TestMissingFixturesSkip(t *testing.T) {
	s := newStandIn(t)
	e := s.env(t)
	for _, sc := range []Scenario{Scenarios[3], Scenarios[4], Scenarios[5], Scenarios[6]} {
		r := RunOne(context.Background(), e, sc, 10*time.Second)
		if r.Status != Skipped {
			t.Fatalf("%s without fixture: %s (%s), want skipped", sc.ID, r.Status, r.Reason)
		}
	}
}

func TestScenarioCatalogue(t *testing.T) {
	if len(Scenarios) != 10 {
		t.Fatalf("%d scenarios, ANTI_CHEAT §11 has 10", len(Scenarios))
	}
	seen := map[string]bool{}
	for _, s := range Scenarios {
		if seen[s.ID] || s.Run == nil || !strings.HasPrefix(s.AC, "F132-AC-2") {
			t.Fatalf("bad scenario %+v", s)
		}
		seen[s.ID] = true
	}
}

func TestSummaryAndMarkdown(t *testing.T) {
	var rs []Result
	for _, s := range Scenarios {
		rs = append(rs, Result{ID: s.ID, AC: s.AC, Title: s.Title, Status: Passed})
	}
	if p, n, ok := Summary(rs); !ok || p != 10 || n != 10 {
		t.Fatalf("10 passed: %d/%d ok=%v", p, n, ok)
	}
	rs[4].Status, rs[4].Reason = Skipped, "anticheat: skipped: battle_subject_id missing"
	if _, _, ok := Summary(rs); ok {
		t.Fatal("a skipped scenario must not give 10/10")
	}
	md := Markdown("staging", time.Date(2026, 11, 27, 8, 0, 0, 0, time.UTC), rs)
	if !strings.Contains(md, "KHÔNG ĐẠT") || !strings.Contains(md, "9/10") {
		t.Fatalf("markdown verdict:\n%s", md)
	}
	if _, err := JSON("staging", time.Now(), rs); err != nil {
		t.Fatal(err)
	}
}

func TestCheckDecay(t *testing.T) {
	d := []int{100, 50, 25, 0, 0}
	if err := checkDecay([]int{20, 10, 5, 0, 0}, d); err != nil {
		t.Fatal(err)
	}
	if err := checkDecay([]int{21, 10, 5, 0, 0}, d); err != nil { // ±1 XP rounding
		t.Fatal(err)
	}
	if err := checkDecay([]int{20, 20, 20, 20, 20}, d); err == nil {
		t.Fatal("no decay accepted")
	}
	if err := checkDecay([]int{0, 0, 0, 0, 0}, d); !errors.Is(err, ErrSkip) {
		t.Fatalf("already farmed today must skip, got %v", err)
	}
}

func TestCompareScores(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	if err := compareScores(f(1), f(1), f(2)); err != nil {
		t.Fatal(err)
	}
	if err := compareScores(f(999), f(1), f(2)); err == nil {
		t.Fatal("client score accepted")
	}
	if err := compareScores(nil, f(1), nil); err == nil {
		t.Fatal("missing score accepted")
	}
}

// F132-AC-07: the runner refuses production and needs its configuration.
func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	fx := filepath.Join(dir, "fixtures.yaml")
	if err := os.WriteFile(fx, []byte("primary: {username: hs.qa.1}\npractice_activity_id: a\nxp_decay: [100, 50, 25, 0, 0]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{envguard.Var: "production", "EW_API_BASE_URL": "http://x/api/v1", "EW_ANTICHEAT_PASSWORD": "pw", "EW_ANTICHEAT_FIXTURES": fx}
	get := func(k string) string { return env[k] }
	if _, err := LoadConfig(get); !errors.Is(err, envguard.ErrRefused) {
		t.Fatalf("production: %v, want refused", err)
	}
	env[envguard.Var] = "staging"
	c, err := LoadConfig(get)
	if err != nil {
		t.Fatal(err)
	}
	if c.Fixtures.Primary.Username != "hs.qa.1" || c.Fixtures.HubArea != "main_hub" || c.ClientVersion == "" {
		t.Fatalf("config %+v", c)
	}
	delete(env, "EW_ANTICHEAT_PASSWORD")
	if _, err := LoadConfig(get); err == nil {
		t.Fatal("missing password accepted")
	}
}

func TestRedactPath(t *testing.T) {
	if got := redactPath("/learning/activity-attempts/0193b0a0-0000-7000-8000-000000000c01:submit"); got != "/learning/activity-attempts/{id}" {
		t.Fatalf("redactPath = %q", got)
	}
}
