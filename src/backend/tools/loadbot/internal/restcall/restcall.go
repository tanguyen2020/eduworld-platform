// Package restcall sends the authenticated REST calls of a bot (Bearer token, X-Tenant-Id, Idempotency-Key UUIDv7
// on writes) and records them in the run metrics.
package restcall

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/gameclient/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/loadbot/internal/report"
)

// Client is one bot's REST identity.
type Client struct {
	HTTP     *http.Client
	BaseURL  string // …/api/v1
	Token    string
	TenantID string
	Metrics  *report.Metrics
}

// Do calls method path; idempotent writes get a fresh Idempotency-Key unless key is set. It returns the status and
// body (≤ 1 MiB); status 0 = transport error.
func (c *Client) Do(ctx context.Context, op, method, path string, body any, key string) (int, []byte, error) {
	var rd io.Reader = http.NoBody
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, rd)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("X-Tenant-Id", c.TenantID)
	req.Header.Set("Accept-Language", "vi")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if method != http.MethodGet {
		if key == "" {
			key = ids.V7(time.Now())
		}
		req.Header.Set("Idempotency-Key", key)
	}
	t0 := time.Now()
	resp, err := c.HTTP.Do(req)
	if err != nil {
		if c.Metrics != nil {
			c.Metrics.HTTP(op, 0, time.Since(t0))
		}
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if c.Metrics != nil {
		c.Metrics.HTTP(op, resp.StatusCode, time.Since(t0))
	}
	return resp.StatusCode, b, err
}
