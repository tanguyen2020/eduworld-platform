package query

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/policy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Policies are the read use cases of the chat policy levels (F120 US2): getChatPolicy, getEffectiveChatPolicy,
// getPlatformChatPolicy, getCommonTenantSocialOverride. Explicit authorization first (R7, R26); nothing is written.
type Policies struct {
	Read   ReadSession
	Reader read.PolicyReader
	Chat   read.ChatReader
}

// EffectiveView is getEffectiveChatPolicy: the effective rule of one channel type in one school (no catalogue content).
type EffectiveView struct {
	ChannelType string
	Enabled     bool
	Effective   policy.Effective
	Sources     []Source
}

// Source is one level that took part in the effective policy.
type Source struct {
	ScopeType string
	Version   int64
}

func staff(ctx context.Context) (authz.Principal, uuid.UUID, error) {
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || !p.Valid() {
		return authz.Principal{}, uuid.Nil, problem.New(problem.Unauthenticated, "")
	}
	if p.UserType != authz.UserTypeStaff {
		return authz.Principal{}, uuid.Nil, problem.New(problem.PortalAccessStaffOnly, "")
	}
	s, ok := tenancy.ScopeFrom(ctx)
	if !ok {
		return p, uuid.Nil, nil
	}
	id, _ := s.SessionTenantID()
	return p, id, nil
}

// GetLevel serves getChatPolicy (404 when the level has no document, or the school is not one of the tenant).
func (q Policies) GetLevel(ctx context.Context, scopeType string, scopeID uuid.UUID) (out read.PolicyRow, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.GetChatPolicy")
	defer func() { done(err) }()
	p, tenant, err := staff(ctx)
	if err != nil {
		return out, err
	}
	if tenant == uuid.Nil {
		return out, problem.New(problem.TenantHeaderRequired, "")
	}
	var school *uuid.UUID
	switch scopeType {
	case "tenant":
		if scopeID != tenant {
			return out, problem.New(problem.NotFound, "")
		}
		if err := authz.Require(ctx, p, authz.PermTenantSettingsRead, authz.Resource{TenantID: &tenant}); err != nil {
			return out, err
		}
	case "school":
		s := scopeID
		school = &s
	default:
		return out, problem.Invalid("scope_type", problem.SubInvalid, "tenant or school")
	}
	var row *read.PolicyRow
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		if school != nil {
			ok, err := q.Reader.SchoolActive(ctx, tenant, *school)
			if err != nil {
				return err
			}
			if !ok {
				return problem.New(problem.NotFound, "")
			}
		}
		t := tenant
		var err error
		row, err = q.Reader.Level(ctx, &t, scopeType, &scopeID)
		return err
	})
	if err != nil {
		return out, err
	}
	if school != nil { // outside the read-only transaction: the denial is audited
		if err := authz.Require(ctx, p, authz.PermSchoolSettingsRead, authz.Resource{TenantID: &tenant, SchoolID: school}); err != nil {
			return out, err
		}
	}
	if row == nil {
		return out, problem.New(problem.NotFound, "")
	}
	return *row, nil
}

// GetPlatform serves getPlatformChatPolicy (platform.settings.write).
func (q Policies) GetPlatform(ctx context.Context) (out read.PolicyRow, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.GetPlatformChatPolicy")
	defer func() { done(err) }()
	p, _, err := staff(ctx)
	if err != nil {
		return out, err
	}
	if err := authz.Require(ctx, p, authz.PermPlatformSettingsWrite, authz.Resource{}); err != nil {
		return out, err
	}
	var row *read.PolicyRow
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		var err error
		row, err = q.Reader.Level(ctx, nil, "platform", nil)
		return err
	})
	if err != nil {
		return out, err
	}
	if row == nil {
		return out, problem.New(problem.NotFound, "")
	}
	return *row, nil
}

// GetEffective serves getEffectiveChatPolicy (school.settings.read in that school): platform ∩ common defaults ∩ tenant ∩
// school for one channel type, the same for every student of the school (R15).
func (q Policies) GetEffective(ctx context.Context, school uuid.UUID, channelType string) (out EffectiveView, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.GetEffectiveChatPolicy")
	defer func() { done(err) }()
	p, tenant, err := staff(ctx)
	if err != nil {
		return out, err
	}
	if tenant == uuid.Nil {
		return out, problem.New(problem.TenantHeaderRequired, "")
	}
	t := channel.Type(channelType)
	if !t.Valid() {
		return out, problem.Invalid("channel_type", problem.SubInvalid, "a channel type")
	}
	var levels policy.Levels
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		ok, err := q.Reader.SchoolActive(ctx, tenant, school)
		if err != nil {
			return err
		}
		if !ok {
			return problem.New(problem.NotFound, "")
		}
		s := school
		levels, err = q.Chat.PolicyLevels(ctx, tenant, &s)
		return err
	})
	if err != nil {
		return out, err
	}
	if err := authz.Require(ctx, p, authz.PermSchoolSettingsRead, authz.Resource{TenantID: &tenant, SchoolID: &school}); err != nil {
		return out, err
	}
	out.ChannelType = channelType
	e, err := policy.Intersect(channelType, t.AlwaysOn(), levels)
	switch {
	case errors.Is(err, policy.ErrDenied):
		out.Effective = policy.Effective{ChannelType: channelType, AllowedKinds: []policy.Kind{}}
	case err != nil:
		return out, err
	default:
		out.Enabled, out.Effective = true, e
	}
	for _, x := range []struct {
		name string
		d    *policy.Document
	}{{"platform", levels.Platform}, {"common_tenant_defaults", levels.Common}, {"tenant", levels.Tenant}, {"school", levels.School}} {
		if x.d != nil {
			out.Sources = append(out.Sources, Source{ScopeType: x.name, Version: x.d.Version})
		}
	}
	return out, nil
}

// GetCommonOverride serves getCommonTenantSocialOverride (platform role with platform.settings.write; tenant common only).
func (q Policies) GetCommonOverride(ctx context.Context) (out read.CommonOverrideRow, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.GetCommonTenantSocialOverride")
	defer func() { done(err) }()
	p, tenant, err := staff(ctx)
	if err != nil {
		return out, err
	}
	if tenant == uuid.Nil {
		return out, problem.New(problem.TenantHeaderRequired, "")
	}
	if !p.HasPlatformRole() {
		return out, problem.New(problem.PermissionDenied, "")
	}
	if err := authz.Require(ctx, p, authz.PermPlatformSettingsWrite, authz.Resource{TenantID: &tenant}); err != nil {
		return out, err
	}
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		typ, err := q.Reader.TenantType(ctx, tenant)
		if err != nil {
			return err
		}
		if typ != "common" {
			return problem.New(problem.NotFound, "")
		}
		row, err := q.Reader.CommonOverride(ctx, tenant)
		if err != nil {
			return err
		}
		out = read.CommonOverrideRow{TenantID: tenant, Override: policy.DefaultCommonOverride()} // version 0: the defaults
		if row != nil {
			out = *row
		}
		return nil
	})
	return out, err
}
