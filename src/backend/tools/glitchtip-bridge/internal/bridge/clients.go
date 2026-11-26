package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// maxAPIBody bounds one answer of GlitchTip or GitHub.
const maxAPIBody = 4 << 20

// apiClient is a JSON client with a bounded timeout and a bearer token. Only GET is retried (idempotent, R29);
// a POST is sent once.
type apiClient struct {
	base    string
	token   string
	headers map[string]string
	http    *http.Client
	retries int           // extra attempts of a GET on a network error, 429 or 5xx
	backoff time.Duration // multiplied by the attempt number
}

// StatusError is a non-2xx answer; it never carries the response body (it may echo request data).
type StatusError struct {
	Method, Path string
	Status       int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("%s %s: status %d", e.Method, e.Path, e.Status)
}

func (c *apiClient) do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	u := c.base + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	var payload []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode %s: %w", path, err)
		}
		payload = b
	}
	attempts := 1
	if method == http.MethodGet {
		attempts += c.retries
	}
	var last error
	for i := range attempts {
		if i > 0 {
			select {
			case <-ctx.Done():
				return fmt.Errorf("%s %s: %w", method, path, ctx.Err())
			case <-time.After(c.backoff * time.Duration(i)):
			}
		}
		retry, err := c.once(ctx, method, u, path, payload, out)
		if err == nil {
			return nil
		}
		last = err
		if !retry {
			break
		}
	}
	return last
}

func (c *apiClient) once(ctx context.Context, method, u, path string, payload []byte, out any) (retry bool, err error) {
	var rd io.Reader
	if payload != nil {
		rd = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rd)
	if err != nil {
		return false, fmt.Errorf("request %s: %w", path, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return ctx.Err() == nil, fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		return resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500,
			&StatusError{Method: method, Path: path, Status: resp.StatusCode}
	}
	if out == nil {
		return false, nil
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxAPIBody)).Decode(out); err != nil {
		return false, fmt.Errorf("decode %s: %w", path, err)
	}
	return false, nil
}

// GlitchTip reads issues through the Sentry-compatible API of GlitchTip (read-only token).
type GlitchTip struct{ c *apiClient }

// NewGlitchTip returns a GlitchTip API client for base (e.g. http://glitchtip:8000).
func NewGlitchTip(base, token string, timeout time.Duration) *GlitchTip {
	return &GlitchTip{c: &apiClient{base: strings.TrimRight(base, "/"), token: token,
		http: &http.Client{Timeout: timeout}, retries: 2, backoff: 300 * time.Millisecond}}
}

// IssueDetail is the subset of a GlitchTip issue copied into a ticket: no event payload, no request data.
type IssueDetail struct {
	Title     string
	Culprit   string
	Count     string
	FirstSeen string
}

// Issue returns the detail of issue id (GET /api/0/issues/{id}/).
func (g *GlitchTip) Issue(ctx context.Context, id string) (IssueDetail, error) {
	var raw struct {
		Title     string          `json:"title"`
		Culprit   string          `json:"culprit"`
		Count     json.RawMessage `json:"count"` // string in the Sentry API, number in some versions
		FirstSeen string          `json:"firstSeen"`
	}
	if err := g.c.do(ctx, http.MethodGet, "/api/0/issues/"+url.PathEscape(id)+"/", nil, nil, &raw); err != nil {
		return IssueDetail{}, err
	}
	return IssueDetail{Title: raw.Title, Culprit: raw.Culprit, Count: strings.Trim(string(raw.Count), `"`), FirstSeen: raw.FirstSeen}, nil
}

// Tags returns the most frequent value of every tag of issue id (GET /api/0/issues/{id}/tags/).
func (g *GlitchTip) Tags(ctx context.Context, id string) (map[string]string, error) {
	var raw []struct {
		Key       string `json:"key"`
		TopValues []struct {
			Value string `json:"value"`
			Count int    `json:"count"`
		} `json:"topValues"`
	}
	if err := g.c.do(ctx, http.MethodGet, "/api/0/issues/"+url.PathEscape(id)+"/tags/", nil, nil, &raw); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(raw))
	for _, t := range raw {
		best, n := "", -1
		for _, v := range t.TopValues {
			if v.Count > n {
				best, n = v.Value, v.Count
			}
		}
		if best != "" {
			out[t.Key] = best
		}
	}
	return out, nil
}

// GitHub creates and searches issues of one repository (fine-grained token: issues read/write).
type GitHub struct {
	c    *apiClient
	repo string // owner/name
}

// NewGitHub returns a GitHub REST client for repo (owner/name) at base (https://api.github.com).
func NewGitHub(base, repo, token string, timeout time.Duration) *GitHub {
	return &GitHub{repo: repo, c: &apiClient{base: strings.TrimRight(base, "/"), token: token,
		headers: map[string]string{"Accept": "application/vnd.github+json", "X-GitHub-Api-Version": "2022-11-28"},
		http:    &http.Client{Timeout: timeout}, retries: 2, backoff: 500 * time.Millisecond}}
}

// FindByMarker returns the number of an issue (open or closed) whose body contains marker, or 0.
func (g *GitHub) FindByMarker(ctx context.Context, marker string) (int, error) {
	var raw struct {
		Items []struct {
			Number      int    `json:"number"`
			Body        string `json:"body"`
			PullRequest any    `json:"pull_request"`
		} `json:"items"`
	}
	q := url.Values{"q": {fmt.Sprintf("repo:%s is:issue in:body %q", g.repo, marker)}, "per_page": {"10"}}
	if err := g.c.do(ctx, http.MethodGet, "/search/issues", q, nil, &raw); err != nil {
		return 0, err
	}
	for _, it := range raw.Items {
		if it.PullRequest == nil && hasMarker(it.Body, marker) {
			return it.Number, nil
		}
	}
	return 0, nil
}

// CreateIssue creates an issue and returns its number. It is never retried: a lost answer would otherwise
// create a second ticket (the next webhook finds the first one by its marker).
func (g *GitHub) CreateIssue(ctx context.Context, title, body string, labels []string) (int, error) {
	var out struct {
		Number int `json:"number"`
	}
	in := map[string]any{"title": title, "body": body, "labels": labels}
	if err := g.c.do(ctx, http.MethodPost, "/repos/"+g.repo+"/issues", nil, in, &out); err != nil {
		return 0, err
	}
	if out.Number == 0 {
		return 0, errors.New("github: create issue answered without a number")
	}
	return out.Number, nil
}

// hasMarker reports whether body contains marker as a whole token (glitchtip-issue:12 ≠ glitchtip-issue:123).
func hasMarker(body, marker string) bool {
	for i := 0; ; {
		j := strings.Index(body[i:], marker)
		if j < 0 {
			return false
		}
		end := i + j + len(marker)
		if end == len(body) || !isDigit(body[end]) {
			return true
		}
		i = end
	}
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }
