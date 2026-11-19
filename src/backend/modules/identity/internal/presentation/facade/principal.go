package facade

import (
	"context"
	"errors"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	identityapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Principals serves resolvePrincipal, getPrincipal and checkMemberships (F012 US3).
type Principals struct {
	Resolve     query.ResolvePrincipal
	Memberships query.CheckMemberships
}

func problemBody(status int, code problem.Code) identityapi.CommonInternalCommonV1Problem {
	return identityapi.CommonInternalCommonV1Problem{Type: "about:blank", Title: string(code), Status: status, Code: string(code)}
}

func isCode(err error, code problem.Code) bool {
	var pe *problem.Error
	return errors.As(err, &pe) && pe.Code == code
}

// ResolvePrincipal implements resolvePrincipal: not a member (or unknown user) → typed 404, which the caller
// maps to authz.ErrNotMember (TENANT_MISMATCH, recorded by the principal middleware — F012-AC-12).
func (f Principals) ResolvePrincipal(ctx context.Context, req identityapi.ResolvePrincipalRequestObject) (identityapi.ResolvePrincipalResponseObject, error) {
	if req.Body == nil {
		return nil, problem.Invalid("body", problem.SubRequired, "required")
	}
	in := query.ResolvePrincipalInput{TenantID: req.Body.TenantId, UserID: req.Body.UserId}
	if req.Body.Issuer != nil {
		in.Issuer = *req.Body.Issuer
	}
	if req.Body.Subject != nil {
		in.Subject = *req.Body.Subject
	}
	p, err := f.Resolve.Handle(ctx, in)
	if isCode(err, problem.NotFound) {
		return identityapi.ResolvePrincipal404ApplicationProblemPlusJSONResponse(problemBody(404, problem.NotFound)), nil
	}
	if err != nil {
		return nil, err
	}
	return identityapi.ResolvePrincipal200JSONResponse(toAPI(p)), nil
}

// GetPrincipal implements the deprecated alias getPrincipal (user id + tenant).
func (f Principals) GetPrincipal(ctx context.Context, req identityapi.GetPrincipalRequestObject) (identityapi.GetPrincipalResponseObject, error) {
	uid, tenant := req.UserId, req.Params.TenantId
	p, err := f.Resolve.Handle(ctx, query.ResolvePrincipalInput{UserID: &uid, TenantID: &tenant})
	if isCode(err, problem.NotFound) {
		return identityapi.GetPrincipal404ApplicationProblemPlusJSONResponse{
			CommonInternalCommonV1NotFoundApplicationProblemPlusJSONResponse: identityapi.CommonInternalCommonV1NotFoundApplicationProblemPlusJSONResponse(problemBody(404, problem.NotFound)),
		}, nil
	}
	if err != nil {
		return nil, err
	}
	cache := "private, max-age=60"
	return identityapi.GetPrincipal200JSONResponse{Body: toAPI(p), Headers: identityapi.GetPrincipal200ResponseHeaders{CacheControl: &cache}}, nil
}

// CheckMemberships implements checkMemberships.
func (f Principals) CheckMemberships(ctx context.Context, req identityapi.CheckMembershipsRequestObject) (identityapi.CheckMembershipsResponseObject, error) {
	if req.Body == nil {
		return nil, problem.Invalid("body", problem.SubRequired, "required")
	}
	in := query.CheckMembershipsInput{UserIDs: req.Body.UserIds, SchoolID: req.Body.SchoolId, ClassID: req.Body.ClassId}
	if req.Body.RequireUserType != nil {
		in.RequireUserType = string(*req.Body.RequireUserType)
	}
	tenant, rows, err := f.Memberships.Handle(ctx, in)
	if err != nil {
		return nil, err
	}
	out := identityapi.CheckMemberships200JSONResponse{TenantId: tenant}
	for _, r := range rows {
		item := struct {
			GradeLevel *int                                              `json:"grade_level,omitempty"`
			IsMember   bool                                              `json:"is_member"`
			Status     *identityapi.MembershipCheckResultResultsStatus   `json:"status,omitempty"`
			UserId     openapi_types.UUID                                `json:"user_id"`
			UserType   *identityapi.MembershipCheckResultResultsUserType `json:"user_type,omitempty"`
		}{UserId: r.UserID, IsMember: r.IsMember, GradeLevel: r.GradeLevel}
		if r.Status != "" {
			s := identityapi.MembershipCheckResultResultsStatus(r.Status)
			item.Status = &s
		}
		if r.UserType != "" {
			t := identityapi.MembershipCheckResultResultsUserType(r.UserType)
			item.UserType = &t
		}
		out.Results = append(out.Results, item)
	}
	return out, nil
}

func toAPI(p dto.Principal) identityapi.Principal {
	out := identityapi.Principal{UserId: p.UserID, UserType: identityapi.PrincipalUserType(p.UserType), Status: identityapi.PrincipalStatus(p.Status),
		TenantId: p.TenantID, IsSystemAdmin: p.IsSystemAdmin, Roles: nonNil(p.Roles), Permissions: nonNil(p.Permissions), Version: p.Version,
		GradeLevel: p.GradeLevel, SisConnectionId: p.SisConnectionID, StudentExternalId: p.StudentExternalID, Scopes: []identityapi.Scope{}}
	if p.TenantType != nil {
		tt := identityapi.TenantType(*p.TenantType)
		out.TenantType = &tt
	}
	if len(p.SchoolIDs) > 0 {
		s := append([]uuid.UUID(nil), p.SchoolIDs...)
		out.SchoolIds = &s
	}
	if len(p.ClassIDs) > 0 {
		c := append([]uuid.UUID(nil), p.ClassIDs...)
		out.ClassIds = &c
	}
	for _, s := range p.Scopes {
		out.Scopes = append(out.Scopes, identityapi.Scope{ScopeType: identityapi.ScopeScopeType(s.Type), ScopeId: s.ID})
	}
	grants := make([]identityapi.PrincipalGrant, 0, len(p.Grants))
	for _, g := range p.Grants {
		grants = append(grants, identityapi.PrincipalGrant{RoleCode: g.RoleCode, RoleScopeKind: identityapi.PrincipalGrantRoleScopeKind(g.RoleScopeKind),
			SchoolId: g.SchoolID, SubjectId: g.SubjectID, Permissions: nonNil(g.Permissions)})
	}
	out.Grants = &grants // always present: the caller refuses a Principal without grants (fail closed)
	return out
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
