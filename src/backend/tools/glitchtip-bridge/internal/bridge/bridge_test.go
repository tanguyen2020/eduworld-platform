package bridge

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

const hookToken = "test-hook-token-0123456789"

// MOCK (AGENTS R17): fakeGlitchTip is an httptest stand-in for the GlitchTip API (issue detail and tags).
type fakeGlitchTip struct {
	mu        sync.Mutex
	tags      map[string][]map[string]any // issue id → tags answer
	failTimes int                         // answer 503 this many times first
	calls     int
	auth      []string
}

func (f *fakeGlitchTip) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.auth = append(f.auth, r.Header.Get("Authorization"))
	if f.failTimes > 0 {
		f.failTimes--
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	m := regexp.MustCompile(`^/api/0/issues/([0-9]+)/(tags/)?$`).FindStringSubmatch(r.URL.Path)
	if m == nil || r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	if m[2] == "tags/" {
		_ = json.NewEncoder(w).Encode(f.tags[m[1]])
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"id": m[1], "title": "detail title", "culprit": "detail culprit",
		"count": "42", "firstSeen": "2026-11-27T01:02:03Z",
		// fields a ticket must never copy
		"metadata": map[string]any{"value": "secret request data"}})
}

// MOCK (AGENTS R17): fakeGitHub is an httptest stand-in for the GitHub REST API (search + create issue).
type fakeGitHub struct {
	mu       sync.Mutex
	issues   []ghIssue
	failPost int // status of POST /issues (0 = 201)
	posts    int
	searches int
}

type ghIssue struct {
	Number int      `json:"number"`
	Title  string   `json:"title"`
	Body   string   `json:"body"`
	Labels []string `json:"labels"`
}

var quoted = regexp.MustCompile(`"([^"]+)"`)

func (f *fakeGitHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.Header.Get("Authorization") != "Bearer gh-token" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/search/issues":
		f.searches++
		q := r.URL.Query().Get("q")
		if !strings.Contains(q, "repo:org/repo") {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		m := quoted.FindStringSubmatch(q)
		items := []map[string]any{}
		for _, is := range f.issues {
			if m != nil && strings.Contains(is.Body, m[1]) {
				items = append(items, map[string]any{"number": is.Number, "body": is.Body})
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"total_count": len(items), "items": items})
	case r.Method == http.MethodPost && r.URL.Path == "/repos/org/repo/issues":
		f.posts++
		if f.failPost != 0 {
			w.WriteHeader(f.failPost)
			return
		}
		var in ghIssue
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		in.Number = len(f.issues) + 1
		f.issues = append(f.issues, in)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"number": in.Number})
	default:
		http.NotFound(w, r)
	}
}

type rig struct {
	gt  *fakeGlitchTip
	gh  *fakeGitHub
	srv *httptest.Server
	b   *Bridge
}

func newRig(t *testing.T, gh *fakeGitHub) *rig {
	t.Helper()
	gt := &fakeGlitchTip{tags: map[string][]map[string]any{
		"101": {tag("module", "identity", 5), tag("service", "core-api", 5), tag("user.email", "student@example.com", 1)},
		"102": {tag("service", "worker", 3)},
		"103": {tag("browser", "x", 1)},
	}}
	if gh == nil {
		gh = &fakeGitHub{}
	}
	gtSrv := httptest.NewServer(gt)
	ghSrv := httptest.NewServer(gh)
	t.Cleanup(gtSrv.Close)
	t.Cleanup(ghSrv.Close)
	tracker := NewGlitchTip(gtSrv.URL, "gt-token", 2*time.Second)
	tracker.c.backoff = time.Millisecond
	tickets := NewGitHub(ghSrv.URL, "org/repo", "gh-token", 2*time.Second)
	tickets.c.backoff = time.Millisecond
	b, err := New(Options{Token: hookToken, Tracker: tracker, Tickets: tickets, Registerer: prometheus.NewRegistry()})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(b)
	t.Cleanup(srv.Close)
	return &rig{gt: gt, gh: gh, srv: srv, b: b}
}

func tag(key, value string, n int) map[string]any {
	return map[string]any{"key": key, "name": key, "topValues": []map[string]any{{"key": key, "value": value, "count": n}}}
}

// payload mirrors the Slack-compatible alert GlitchTip posts to a general webhook.
func payload(issueID, env, title string) string {
	b, _ := json.Marshal(map[string]any{
		"alias": "GlitchTip", "text": "GlitchTip Alert",
		"attachments": []map[string]any{{
			"title": title, "title_link": "https://glitchtip.ops.example/eduworld/issues/" + issueID + "?project=3",
			"text": "modules/identity/internal/application/command.Login",
			"fields": []map[string]any{
				{"title": "Project", "value": "core-api", "short": true},
				{"title": "Environment", "value": env, "short": true},
				{"title": "Release", "value": "backend/v1.4.0", "short": false},
			},
		}},
	})
	return string(b)
}

func (r *rig) post(t *testing.T, token, body string) int {
	t.Helper()
	resp, err := http.Post(r.srv.URL+"/hook/"+token, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	return resp.StatusCode
}

func (r *rig) count(res string) float64 { return testutil.ToFloat64(r.b.total.WithLabelValues(res)) }

// F131-AC-18 (T034): one ticket per GlitchTip issue, labelled by module, with the dedupe marker.
func TestCreatesTicketOnce(t *testing.T) {
	r := newRig(t, nil)
	title := "login failed for student@example.com with Bearer abc.def.ghi"
	if got := r.post(t, hookToken, payload("101", "production", title)); got != http.StatusAccepted {
		t.Fatalf("status %d", got)
	}
	if got := r.post(t, hookToken, payload("101", "production", title)); got != http.StatusAccepted {
		t.Fatalf("repeat status %d", got)
	}
	if len(r.gh.issues) != 1 {
		t.Fatalf("issues = %d, want 1", len(r.gh.issues))
	}
	is := r.gh.issues[0]
	if strings.Join(is.Labels, ",") != "module:identity,source:glitchtip" {
		t.Errorf("labels %v", is.Labels)
	}
	if !strings.Contains(is.Body, "glitchtip-issue:101\n") {
		t.Errorf("body without marker:\n%s", is.Body)
	}
	for _, leak := range []string{"student@example.com", "abc.def.ghi", "secret request data", "project=3"} {
		if strings.Contains(is.Title+is.Body, leak) {
			t.Errorf("ticket leaks %q:\ntitle %s\nbody %s", leak, is.Title, is.Body)
		}
	}
	for _, want := range []string{"https://glitchtip.ops.example/eduworld/issues/101", "Events: 42", "backend/v1.4.0", "command.Login"} {
		if !strings.Contains(is.Body, want) {
			t.Errorf("body misses %q:\n%s", want, is.Body)
		}
	}
	if !strings.HasPrefix(is.Title, "[GlitchTip] login failed for [REDACTED_EMAIL]") {
		t.Errorf("title %q", is.Title)
	}
	if r.count(ResultCreated) != 1 || r.count(ResultDuplicate) != 1 {
		t.Errorf("created=%v duplicate=%v", r.count(ResultCreated), r.count(ResultDuplicate))
	}
	for _, a := range r.gt.auth {
		if a != "Bearer gt-token" {
			t.Errorf("glitchtip auth header %q", a)
		}
	}
}

// A restarted bridge (empty cache) finds the existing ticket by its marker through the search API.
func TestDuplicateFoundBySearchAfterRestart(t *testing.T) {
	gh := &fakeGitHub{issues: []ghIssue{{Number: 7, Title: "old", Body: "x\nglitchtip-issue:101\n"},
		{Number: 8, Title: "other", Body: "glitchtip-issue:1010\n"}}}
	r := newRig(t, gh)
	if got := r.post(t, hookToken, payload("101", "production", "boom")); got != http.StatusAccepted {
		t.Fatalf("status %d", got)
	}
	if gh.posts != 0 || r.count(ResultDuplicate) != 1 {
		t.Fatalf("posts=%d duplicate=%v", gh.posts, r.count(ResultDuplicate))
	}
	// glitchtip-issue:10 must not match the ticket of glitchtip-issue:101 / 1010
	if got := r.post(t, hookToken, payload("10", "production", "boom")); got != http.StatusAccepted {
		t.Fatalf("status %d", got)
	}
	if gh.posts != 1 {
		t.Fatalf("issue 10 should get its own ticket, posts=%d", gh.posts)
	}
}

func TestNonProductionIgnored(t *testing.T) {
	r := newRig(t, nil)
	for _, env := range []string{"staging", "", "local"} {
		if got := r.post(t, hookToken, payload("101", env, "boom")); got != http.StatusAccepted {
			t.Fatalf("%q: status %d", env, got)
		}
	}
	if r.gh.posts != 0 || r.gh.searches != 0 || r.gt.calls != 0 {
		t.Errorf("non-production issue reached an API: posts=%d searches=%d glitchtip=%d", r.gh.posts, r.gh.searches, r.gt.calls)
	}
	if r.count(ResultIgnoredEnv) != 3 {
		t.Errorf("ignored_env = %v", r.count(ResultIgnoredEnv))
	}
}

func TestBadTokenNotFound(t *testing.T) {
	r := newRig(t, nil)
	for _, tok := range []string{"", "wrong", hookToken + "x", strings.ToUpper(hookToken)} {
		if got := r.post(t, tok, payload("101", "production", "boom")); got != http.StatusNotFound {
			t.Errorf("token %q: status %d, want 404", tok, got)
		}
	}
	resp, err := http.Post(r.srv.URL+"/other", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("other path: %d", resp.StatusCode)
	}
	if r.gh.posts != 0 {
		t.Error("ticket created with a bad token")
	}
}

func TestOwnerLabelFallback(t *testing.T) {
	r := newRig(t, nil)
	for _, id := range []string{"101", "102", "103"} {
		if got := r.post(t, hookToken, payload(id, "Production", "boom "+id)); got != http.StatusAccepted {
			t.Fatalf("%s: status %d", id, got)
		}
	}
	want := []string{"module:identity", "service:worker", "service:core-api"} // 103: no tag → GlitchTip project
	for i, is := range r.gh.issues {
		if is.Labels[0] != want[i] {
			t.Errorf("issue %d labels %v, want %s", i, is.Labels, want[i])
		}
	}
	if got := ownerLabel(map[string]string{"module": "Bad Label!"}, ""); got != "service:unknown" {
		t.Errorf("unsafe tag value: %s", got)
	}
}

func TestGitHubErrorAnswers5xxAndCounts(t *testing.T) {
	r := newRig(t, &fakeGitHub{failPost: http.StatusInternalServerError})
	if got := r.post(t, hookToken, payload("101", "production", "boom")); got != http.StatusBadGateway {
		t.Fatalf("status %d, want 502", got)
	}
	if r.gh.posts != 1 {
		t.Errorf("POST retried: %d calls", r.gh.posts)
	}
	if r.count(ResultError) != 1 || r.count(ResultCreated) != 0 {
		t.Errorf("error=%v created=%v", r.count(ResultError), r.count(ResultCreated))
	}
	// the failure is not cached: the next delivery tries again once GitHub is back
	r.gh.mu.Lock()
	r.gh.failPost = 0
	r.gh.mu.Unlock()
	if got := r.post(t, hookToken, payload("101", "production", "boom")); got != http.StatusAccepted {
		t.Fatalf("retry status %d", got)
	}
	if len(r.gh.issues) != 1 {
		t.Errorf("issues = %d", len(r.gh.issues))
	}
}

func TestGlitchTipGetRetried(t *testing.T) {
	r := newRig(t, nil)
	r.gt.failTimes = 2 // two 503 then OK: within the 2 retries of an idempotent GET
	if got := r.post(t, hookToken, payload("101", "production", "boom")); got != http.StatusAccepted {
		t.Fatalf("status %d", got)
	}
	if len(r.gh.issues) != 1 {
		t.Errorf("issues = %d", len(r.gh.issues))
	}
}

func TestInvalidWebhook(t *testing.T) {
	r := newRig(t, nil)
	for _, body := range []string{"not json", `{"attachments":[]}`} {
		if got := r.post(t, hookToken, body); got != http.StatusBadRequest {
			t.Errorf("%q: status %d", body, got)
		}
	}
	if got := r.post(t, hookToken, strings.Replace(payload("101", "production", "x"), "/issues/101", "/projects/1", 1)); got != http.StatusBadGateway {
		t.Errorf("link without issue id: status %d", got)
	}
	resp, err := http.Get(r.srv.URL + "/hook/" + hookToken)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("GET: %d", resp.StatusCode)
	}
}

func TestHasMarker(t *testing.T) {
	for body, want := range map[string]bool{
		"glitchtip-issue:12":                     true,
		"x glitchtip-issue:12\n":                 true,
		"glitchtip-issue:123":                    false,
		"glitchtip-issue:123 glitchtip-issue:12": true,
		"":                                       false,
	} {
		if got := hasMarker(body, "glitchtip-issue:12"); got != want {
			t.Errorf("hasMarker(%q) = %v", body, got)
		}
	}
}
