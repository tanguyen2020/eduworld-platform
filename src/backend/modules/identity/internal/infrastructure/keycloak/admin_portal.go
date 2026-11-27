package keycloak

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// Admin API calls of the portal account setup and password reset (ADR-0091 §3, F012 US8), on the same
// svc-identity client, timeouts, retry policy and circuit breaker as the rest of the adapter.

var _ port.PortalAccountAdmin = (*Admin)(nil)

type loginUserRep struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Enabled  bool   `json:"enabled"`
}

// FindUserByLogin implements port.PortalAccountAdmin: exact username, or exact email when login has "@".
func (a *Admin) FindUserByLogin(ctx context.Context, login string) (port.KCUser, error) {
	login = strings.TrimSpace(login)
	if !strings.Contains(login, "@") {
		return a.FindUserByUsername(ctx, login)
	}
	var users []loginUserRep
	q := url.Values{"email": {login}, "exact": {"true"}}
	if _, err := a.call(ctx, "find_user_email", http.MethodGet, "/users?"+q.Encode(), nil, &users, true); err != nil {
		return port.KCUser{}, err
	}
	var found []loginUserRep
	for _, u := range users {
		if strings.EqualFold(u.Email, login) {
			found = append(found, u)
		}
	}
	if len(found) != 1 { // none, or ambiguous (duplicate emails are not allowed for staff, ADR-0069)
		return port.KCUser{}, port.ErrKCNotFound
	}
	return port.KCUser{ID: found[0].ID, Username: found[0].Username, Enabled: found[0].Enabled}, nil
}

// RequiredActions implements port.PortalAccountAdmin.
func (a *Admin) RequiredActions(ctx context.Context, userID string) ([]string, error) {
	var u struct {
		RequiredActions []string `json:"requiredActions"`
	}
	if _, err := a.call(ctx, "get_user", http.MethodGet, "/users/"+url.PathEscape(userID), nil, &u, true); err != nil {
		return nil, err
	}
	return u.RequiredActions, nil
}

// ResetPassword implements port.PortalAccountAdmin: PUT reset-password with temporary=false (idempotent,
// retried). 400 is the realm password policy (invalidPassword*Message) → port.ErrKCPasswordPolicy. The JSON
// payload is built without intermediate strings of the password and zeroed afterwards (best effort).
func (a *Admin) ResetPassword(ctx context.Context, userID string, password []byte) error {
	payload := []byte(`{"type":"password","temporary":false,"value":`)
	payload = appendJSONString(payload, password)
	payload = append(payload, '}')
	defer clear(payload)
	res, err := a.callRaw(ctx, "reset_password", http.MethodPut, "/users/"+url.PathEscape(userID)+"/reset-password", payload, true)
	if err != nil {
		return err
	}
	switch {
	case res.status < 300:
		return nil
	case res.status == http.StatusBadRequest:
		return port.ErrKCPasswordPolicy
	default:
		return fmt.Errorf("keycloak reset_password: status %d: %w", res.status, statusErr(res.status))
	}
}

// RemoveRequiredAction implements port.PortalAccountAdmin (GET then PUT of the representation: idempotent).
func (a *Admin) RemoveRequiredAction(ctx context.Context, userID, action string) error {
	var cur map[string]any
	path := "/users/" + url.PathEscape(userID)
	if _, err := a.call(ctx, "get_user", http.MethodGet, path, nil, &cur, true); err != nil {
		return err
	}
	list, _ := cur["requiredActions"].([]any)
	kept := make([]any, 0, len(list))
	for _, v := range list {
		if s, _ := v.(string); s != action {
			kept = append(kept, v)
		}
	}
	if len(kept) == len(list) {
		return nil
	}
	cur["requiredActions"] = kept
	_, err := a.call(ctx, "remove_required_action", http.MethodPut, path, cur, nil, true)
	return err
}

// ExecuteActionsEmail implements port.PortalAccountAdmin. It sends an email, so it is never retried; the
// lifespan of the action token is the realm setting (F004).
func (a *Admin) ExecuteActionsEmail(ctx context.Context, userID string, actions ...string) error {
	if len(actions) == 0 {
		return errors.New("keycloak: execute-actions-email needs an action")
	}
	_, err := a.call(ctx, "execute_actions_email", http.MethodPut, "/users/"+url.PathEscape(userID)+"/execute-actions-email", actions, nil, false)
	return err
}

// callRaw is call for requests whose 4xx answers the caller interprets (password policy): transport errors,
// 401 (token refresh), 429 and 5xx are handled like call; other statuses are returned as a result.
func (a *Admin) callRaw(ctx context.Context, op, method, path string, payload []byte, idempotent bool) (result, error) {
	ctx, span := otel.Tracer("eduworld/identity/keycloak").Start(ctx, "keycloak."+op)
	defer span.End()
	exec := a.once
	if idempotent {
		exec = a.retry
	}
	refreshed := false
	res, err := exec.WithContext(ctx).Get(func() (result, error) {
		tok, err := a.accessToken(ctx, false)
		if err != nil {
			return result{}, err
		}
		res, err := a.do(ctx, method, path, tok, payload, payload != nil)
		if err != nil {
			return result{}, err
		}
		if res.status == http.StatusUnauthorized && !refreshed {
			refreshed = true
			if _, err := a.accessToken(ctx, true); err != nil {
				return result{}, err
			}
			return result{}, errTransient
		}
		if res.status == http.StatusTooManyRequests || res.status >= 500 {
			return result{}, fmt.Errorf("keycloak %s: status %d: %w", op, res.status, errTransient)
		}
		return res, nil
	})
	observability.ObserveRequest("identity", observability.IntegrationKeycloakAdmin, err, port.ErrKCNotFound, port.ErrKCConflict)
	if err != nil {
		span.SetStatus(codes.Error, op)
		span.SetAttributes(attribute.String("error.kind", errKind(err)))
		orgSync.WithLabelValues(op, errKind(err)).Inc()
		return result{}, err
	}
	kind := "ok"
	if res.status >= 300 {
		kind = "rejected"
	}
	orgSync.WithLabelValues(op, kind).Inc()
	span.SetAttributes(attribute.Int("http.response.status_code", res.status))
	return res, nil
}

// appendJSONString appends s as a JSON string literal (RFC 8259 escaping; invalid UTF-8 is passed through
// for Keycloak to reject).
func appendJSONString(b, s []byte) []byte {
	b = append(b, '"')
	for _, c := range s {
		switch {
		case c == '"' || c == '\\':
			b = append(b, '\\', c)
		case c < 0x20:
			b = append(b, '\\', 'u', '0', '0', hexDigits[c>>4], hexDigits[c&15])
		default:
			b = append(b, c)
		}
	}
	return append(b, '"')
}
