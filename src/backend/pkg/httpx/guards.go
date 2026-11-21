package httpx

import (
	"net/http"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// guards applies portal_staff, game_player and platform_mfa (F002-AC-16..18, API_DESIGN §11). Denials
// are audited as authz.denied with the problem code as reason; MFA_REQUIRED is a step-up prompt and is
// not audited.
func (s *Server) guards(g Guard, next http.Handler) http.Handler {
	if g == GuardNone {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		p, ok := authz.PrincipalFrom(ctx)
		if !ok {
			WriteError(w, r, unauthenticated(""))
			return
		}
		deny := func(code problem.Code, why string) {
			meta := map[string]any{"why": why, "client_id": p.ClientID, "user_type": string(p.UserType)}
			if op, ok := OperationFrom(ctx); ok {
				meta["operation"] = op.ID
			}
			authz.RecordDenied(ctx, string(code), meta)
			WriteError(w, r, problem.New(code, ""))
		}
		if g&GuardPortalStaff != 0 && (p.Channel != authz.ChannelPortal || p.UserType != authz.UserTypeStaff) {
			deny(problem.PortalAccessStaffOnly, "portal_staff")
			return
		}
		if g&GuardGamePlayer != 0 && (p.Channel != authz.ChannelGame || p.UserType != authz.UserTypeStudent) {
			deny(problem.GameAccessStudentOnly, "game_player")
			return
		}
		if g&GuardPlatformMFA != 0 {
			if !p.HasPlatformRole() {
				deny(problem.PermissionDenied, "platform_role")
				return
			}
			if !p.MFA {
				deny(problem.MfaRequired, "mfa") // audited like every other guard denial (F121-AC-10, ADR-0027)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
