package facade

import (
	"context"

	identityapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Nicknames serves screenNicknamePersonalData (x-allowed-callers: game, x-context-required: tenant_scope — F136-AC-12):
// the decision only, never the real name or the school name (R16).
type Nicknames struct {
	Screen query.ScreenNicknamePersonalData
}

// ScreenNicknamePersonalData implements screenNicknamePersonalData: a student outside the scope tenant is a typed 404;
// a read failure is returned (the caller answers 503 and stores nothing, F136-AC-14).
func (f Nicknames) ScreenNicknamePersonalData(ctx context.Context, req identityapi.ScreenNicknamePersonalDataRequestObject) (identityapi.ScreenNicknamePersonalDataResponseObject, error) {
	if req.Body == nil {
		return nil, problem.Invalid("body", problem.SubRequired, "required")
	}
	blocked, err := f.Screen.Handle(ctx, req.StudentId, req.Body.Text)
	if isCode(err, problem.NotFound) {
		return identityapi.ScreenNicknamePersonalData404ApplicationProblemPlusJSONResponse{
			CommonInternalCommonV1NotFoundApplicationProblemPlusJSONResponse: identityapi.CommonInternalCommonV1NotFoundApplicationProblemPlusJSONResponse(problemBody(404, problem.NotFound)),
		}, nil
	}
	if err != nil {
		return nil, err
	}
	d := identityapi.Allowed
	if blocked {
		d = identityapi.Blocked
	}
	return identityapi.ScreenNicknamePersonalData200JSONResponse(identityapi.NicknameScreenResult{Decision: d}), nil
}
