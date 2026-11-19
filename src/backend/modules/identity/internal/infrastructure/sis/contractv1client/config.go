package contractv1client

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// Config is the worker configuration of contract_v1 connections (F017 T002). Nothing is hardcoded (AGENTS R3):
//
//   - EW_SIS_SECRETS_DIR: directory of the secrets referenced by secret_ref/webhook_secret_ref
//     (/run/secrets in staging/production, deploy/compose/secrets locally);
//   - EW_SIS_EGRESS_PROXY_URL: fixed egress proxy towards the facades (static IP allowlisted by schools,
//     SIS_INTEGRATION §9); empty = direct;
//   - EW_SIS_ALLOW_HTTP_BASE_URL: accept http base URLs (compose dev with sis-mock only; default false).
type Config struct {
	SecretsDir       string
	EgressProxy      *url.URL
	AllowHTTPBaseURL bool
}

// ConfigFrom reads the variables; a malformed proxy URL is reported by name only.
func ConfigFrom(c *config.Config) (Config, error) {
	if err := c.Require("EW_SIS_SECRETS_DIR"); err != nil {
		return Config{}, err
	}
	out := Config{SecretsDir: c.String("EW_SIS_SECRETS_DIR"), AllowHTTPBaseURL: c.Bool("EW_SIS_ALLOW_HTTP_BASE_URL", false)}
	if raw := c.String("EW_SIS_EGRESS_PROXY_URL"); raw != "" {
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" {
			return Config{}, errors.New("contractv1client: EW_SIS_EGRESS_PROXY_URL must be an absolute URL")
		}
		out.EgressProxy = u
	}
	return out, nil
}

// Transport returns the base transport of the factory (egress proxy when configured).
func (c Config) Transport() *http.Transport {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if c.EgressProxy != nil {
		tr.Proxy = http.ProxyURL(c.EgressProxy)
	}
	return tr
}
