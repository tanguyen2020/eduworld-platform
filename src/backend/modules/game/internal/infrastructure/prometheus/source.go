// Package prometheus is the adapter of the Prometheus HTTP API for the Live dashboard (F080-AC-17, DASHBOARD_DATA §2:
// live-ops numbers never come from the database): instant queries of the fixed catalog of query.KPICatalog only, with a
// timeout; URL from EW_PROMETHEUS_URL (R3).
package prometheus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
)

// Source implements port.MetricsSource.
type Source struct {
	BaseURL string
	Client  *http.Client
	Timeout time.Duration
}

var _ port.MetricsSource = Source{}

type response struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  [2]any            `json:"value"`
		} `json:"result"`
	} `json:"data"`
	Error string `json:"error"`
}

// Query implements port.MetricsSource: GET /api/v1/query; any transport error, non-2xx status or error body is
// port.ErrMetricsUnavailable (the caller answers 503, never fake zeros).
func (s Source) Query(ctx context.Context, promql string) ([]port.Sample, error) {
	timeout := s.Timeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	client := s.Client
	if client == nil {
		client = http.DefaultClient
	}
	u := strings.TrimRight(s.BaseURL, "/") + "/api/v1/query?" + url.Values{"query": {promql}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", port.ErrMetricsUnavailable, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", port.ErrMetricsUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("%w: status %d", port.ErrMetricsUnavailable, resp.StatusCode)
	}
	var r response
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil || r.Status != "success" {
		return nil, fmt.Errorf("%w: %s", port.ErrMetricsUnavailable, r.Error)
	}
	out := make([]port.Sample, 0, len(r.Data.Result))
	for _, x := range r.Data.Result {
		str, _ := x.Value[1].(string)
		v, err := strconv.ParseFloat(str, 64)
		if err != nil {
			continue
		}
		out = append(out, port.Sample{Labels: x.Metric, Value: v})
	}
	return out, nil
}
