package devseed

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"

	"github.com/google/uuid"
)

// PortalLogin signs in through POST /auth/portal/login (module identity, ADR-0091/ADR-0094: password only) and returns
// the access token. The password is never logged.
func PortalLogin(ctx context.Context, hc *http.Client, base, username, password string) (string, error) {
	var out struct {
		AccessToken string `json:"access_token"`
	}
	body := map[string]any{"username": username, "password": password, "device_id": uuid.NewString(), "locale": "vi-VN"}
	if _, err := Do(ctx, hc, base, "", http.MethodPost, "/auth/portal/login", nil, body, &out); err != nil {
		return "", fmt.Errorf("portal login %s: %w", username, err)
	}
	if out.AccessToken == "" {
		return "", fmt.Errorf("portal login %s: no access token", username)
	}
	return out.AccessToken, nil
}

// AdminAssigner assigns through the System Admin API of the internal gateway (assignToTenants, ADR-0029): the demo
// tenants are looked up by code (listTenants); only the tenants that do not have the resource yet
// (listTenantAssignments, every page) are sent, so a second run writes nothing (assignToTenants records
// tenant_data.assigned for each tenant it is given).
type AdminAssigner struct {
	HTTP    *http.Client
	BaseURL string // internal gateway, ends with /api/v1
	Token   string // System Admin
	Codes   []string
	tenants []string
}

// Tenants resolves the demo tenant codes (all must exist and be active).
func (a *AdminAssigner) Tenants(ctx context.Context) ([]string, error) {
	if a.tenants != nil {
		return a.tenants, nil
	}
	for _, code := range a.Codes {
		var p page
		if _, err := Do(ctx, a.HTTP, a.BaseURL, a.Token, http.MethodGet, "/admin/tenants?limit=100&q="+url.QueryEscape(code), nil, nil, &p); err != nil {
			return nil, err
		}
		i := slices.IndexFunc(p.Data, func(t map[string]any) bool { return str(t, "code") == code && str(t, "status") == "active" })
		if i < 0 {
			return nil, fmt.Errorf("tenant %s not found or not active: run make seed-identity first", code)
		}
		a.tenants = append(a.tenants, str(p.Data[i], "tenant_id"))
	}
	return a.tenants, nil
}

// Assign implements Assigner.
func (a *AdminAssigner) Assign(ctx context.Context, resourceType string, id uuid.UUID) error {
	tenants, err := a.Tenants(ctx)
	if err != nil {
		return err
	}
	missing, err := MissingTenants(ctx, a.HTTP, a.BaseURL, a.Token, resourceType, id.String(), tenants)
	if err != nil || len(missing) == 0 {
		return err
	}
	body := map[string]any{"resource_type": resourceType, "resource_id": id.String(), "tenant_ids": missing}
	_, err = Do(ctx, a.HTTP, a.BaseURL, a.Token, http.MethodPost, "/admin/tenant-assignments", nil, body, nil)
	return err
}

// MissingTenants returns the tenants of want that do not have the resource yet (listTenantAssignments of the System Admin
// API, every page), in the order of want.
func MissingTenants(ctx context.Context, hc *http.Client, base, token, resourceType, id string, want []string) ([]string, error) {
	have := map[string]bool{}
	cursor := ""
	for range 100 {
		q := "/admin/tenant-assignments?limit=100&resource_type=" + url.QueryEscape(resourceType) + "&resource_id=" + url.QueryEscape(id)
		if cursor != "" {
			q += "&cursor=" + url.QueryEscape(cursor)
		}
		var p struct {
			Data []map[string]any `json:"data"`
			Page struct {
				NextCursor *string `json:"next_cursor"`
				HasMore    bool    `json:"has_more"`
			} `json:"page"`
		}
		if _, err := Do(ctx, hc, base, token, http.MethodGet, q, nil, nil, &p); err != nil {
			return nil, err
		}
		for _, a := range p.Data {
			have[str(a, "tenant_id")] = true
		}
		if !p.Page.HasMore || p.Page.NextCursor == nil || *p.Page.NextCursor == "" {
			out := make([]string, 0, len(want))
			for _, t := range want {
				if !have[t] {
					out = append(out, t)
				}
			}
			return out, nil
		}
		cursor = *p.Page.NextCursor
	}
	return nil, fmt.Errorf("tenant assignments of %s %s: too many pages", resourceType, id)
}
