package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/policy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Events and audit actions of the chat policy levels (F120 US2; ids, versions and the policy documents only — no message).
const (
	EventChatPolicyUpdated      = "chat_policy.updated"
	AuditChatPolicyUpdated      = "chat_policy.updated"
	AuditCommonOverrideUpdated  = "chat_policy.common_tenant_social_updated"
	policyTimezone              = "Asia/Ho_Chi_Minh"
	maxPolicyReason             = 500
	minPolicyReason             = 10
	policyScopePlatform         = "platform"
	policyScopeTenant           = "tenant"
	policyScopeSchool           = "school"
	commonTenantType            = "common"
	unitTenantType              = "unit"
	maxPolicyDocumentBytes      = 65536
	policyVersionMismatchDetail = "the policy changed meanwhile: reload it"
)

// PolicyInput is the body of putChatPolicy / putPlatformChatPolicy (ChatPolicyPut).
type PolicyInput struct {
	Rules      json.RawMessage
	GlobalRate *policy.Rate
	WorldScope string
	Preset     string
	Reason     string
}

// CommonOverrideInput is the body of putCommonTenantSocialOverride.
type CommonOverrideInput struct {
	Override policy.CommonOverride
	Reason   string
}

// PolicyAdmin are the write use cases of the chat policy levels (F120 US2): putChatPolicy (tenant, school),
// putPlatformChatPolicy and putCommonTenantSocialOverride. Every write: explicit authorization, the always-on rule, the
// JSON schema v5 (no grade condition), "only tighten" against the levels above and the curfew, a new version with
// If-Match, audit before/after and chat_policy.updated in the same transaction; the cached effective policies of the
// tenant (or of every tenant) change at once after the commit (generation bump).
type PolicyAdmin struct {
	UoW         write.UnitOfWork
	AfterCommit write.AfterCommit
	Store       write.PolicyStore
	Schema      port.PolicySchema
	Events      write.EventPublisher
	Audit       write.Auditor
	Gens        port.PolicyGenerations // nil: cache TTL only
	// Curfew returns the effective curfew of a tenant/school (service.Facts.Curfew).
	Curfew func(ctx context.Context, tenant uuid.UUID, school *uuid.UUID) (policy.Curfew, error)
	// PlatformCurfew is the curfew of a platform write (EW_PLATFORM_CURFEW until identity projects platform_curfew).
	PlatformCurfew policy.Curfew
	// Rejected counts a refused write by problem code (eduworld_chat_policy_rejected_total); nil: none.
	Rejected func(code string)
	Now      func() time.Time
}

func (h PolicyAdmin) count(err error) {
	var pe *problem.Error
	if h.Rejected == nil || !errors.As(err, &pe) {
		return
	}
	switch pe.Code {
	case problem.ChatPolicyRelaxNotAllowed, problem.ChatPolicyAlwaysOn, problem.ValidationFailed:
		h.Rejected(string(pe.Code))
	}
}

func (h PolicyAdmin) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

// PolicyResult is the stored document of a level.
type PolicyResult struct{ Row read.PolicyRow }

func staffPrincipal(ctx context.Context) (authz.Principal, error) {
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || !p.Valid() {
		return authz.Principal{}, problem.New(problem.Unauthenticated, "")
	}
	if p.UserType != authz.UserTypeStaff {
		return authz.Principal{}, problem.New(problem.PortalAccessStaffOnly, "")
	}
	return p, nil
}

func sessionTenant(ctx context.Context) (uuid.UUID, error) {
	if s, ok := tenancy.ScopeFrom(ctx); ok {
		if id, ok := s.SessionTenantID(); ok {
			return id, nil
		}
	}
	return uuid.Nil, problem.New(problem.TenantHeaderRequired, "")
}

func checkReason(reason string) (string, error) {
	r := strings.TrimSpace(reason)
	if n := utf8.RuneCountInString(r); n < minPolicyReason || n > maxPolicyReason {
		return "", problem.Invalid("reason", problem.SubInvalid, "10..500 characters")
	}
	return r, nil
}

// document builds and checks the chat-policy v5 document of a level: grade keys → VALIDATION_FAILED (R15), always-on →
// CHAT_POLICY_ALWAYS_ON, schema → VALIDATION_FAILED with the failing pointers, preset outside a school → VALIDATION_FAILED.
//
// The keys of the stored document that ChatPolicyPut does not carry (moderation_switches, sensitive_term_policy,
// quick_message_set_id, notes of the platform seed) are kept from base, the current version.
func (h PolicyAdmin) document(scopeType string, scopeID *uuid.UUID, id uuid.UUID, version int64, in PolicyInput,
	base json.RawMessage) (json.RawMessage, policy.Document, error) {
	if policy.HasGradeKey(in.Rules) {
		return nil, policy.Document{}, problem.Invalid("rules", problem.SubInvalid, "no grade condition in a chat policy (one rule set for every grade)")
	}
	var rules []json.RawMessage
	if err := json.Unmarshal(in.Rules, &rules); err != nil || len(rules) == 0 {
		return nil, policy.Document{}, problem.Invalid("rules", problem.SubInvalid, "a non-empty array of rules")
	}
	doc := map[string]any{}
	if len(base) > 0 {
		_ = json.Unmarshal(base, &doc) // a stored document is a JSON object (CHECK chat_policies_rules_chk)
	}
	for _, k := range []string{"global_rate", "world_scope", "chat_policy_preset"} {
		delete(doc, k)
	}
	for k, v := range map[string]any{"schema_version": 5, "policy_id": id, "scope_type": scopeType, "scope_id": scopeID,
		"version": version, "rules": rules} {
		doc[k] = v
	}
	if _, ok := doc["timezone"]; !ok {
		doc["timezone"] = policyTimezone
	}
	if scopeID == nil {
		doc["scope_id"] = nil
	}
	if in.GlobalRate != nil {
		doc["global_rate"] = in.GlobalRate
	}
	if in.WorldScope != "" {
		doc["world_scope"] = in.WorldScope
	}
	if in.Preset != "" {
		if scopeType != policyScopeSchool {
			return nil, policy.Document{}, problem.Invalid("chat_policy_preset", problem.SubInvalid, "a school-level setting for the whole school")
		}
		doc["chat_policy_preset"] = in.Preset
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, policy.Document{}, err
	}
	if len(raw) > maxPolicyDocumentBytes {
		return nil, policy.Document{}, problem.Invalid("rules", problem.SubInvalid, "policy document over 64 KiB")
	}
	if err := CheckPolicyDocument(raw); err != nil {
		return nil, policy.Document{}, err
	}
	bad, err := h.Schema.Validate(raw)
	if err != nil {
		return nil, policy.Document{}, fmt.Errorf("chat: policy schema: %w", err)
	}
	if len(bad) > 0 {
		pe := problem.Invalid("rules", problem.SubInvalid, "not a chat-policy.schema.json v5 rule set")
		return nil, policy.Document{}, pe.With("pointers", bad)
	}
	d, err := policy.Parse(raw)
	if err != nil {
		return nil, policy.Document{}, problem.Invalid("rules", problem.SubInvalid, "not a ChatPolicy document")
	}
	return raw, d, nil
}

func relaxProblem(err error) error {
	var re policy.RelaxError
	switch {
	case errors.As(err, &re):
		return problem.New(problem.ChatPolicyRelaxNotAllowed, "").With("channel_type", re.ChannelType).With("field", re.Field)
	case errors.Is(err, policy.ErrPresetScope):
		return problem.Invalid("chat_policy_preset", problem.SubInvalid, "a school-level setting for the whole school")
	case errors.Is(err, policy.ErrHours):
		return problem.Invalid("rules", problem.SubInvalid, "allowed_hours windows {days, from, to}")
	}
	return err
}

// PutLevel implements putChatPolicy for scope tenant (tenant.settings.write) or school (school.settings.write in that
// school). ifMatch is the version the caller read (0: no document yet).
func (h PolicyAdmin) PutLevel(ctx context.Context, scopeType string, scopeID uuid.UUID, ifMatch int64, in PolicyInput) (res PolicyResult, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.PutChatPolicy")
	defer func() { h.count(err); done(err) }()
	p, err := staffPrincipal(ctx)
	if err != nil {
		return PolicyResult{}, err
	}
	tenant, err := sessionTenant(ctx)
	if err != nil {
		return PolicyResult{}, err
	}
	reason, err := checkReason(in.Reason)
	if err != nil {
		return PolicyResult{}, err
	}
	var school *uuid.UUID
	switch scopeType {
	case policyScopeTenant:
		if scopeID != tenant {
			return PolicyResult{}, notFound()
		}
		if err := authz.Require(ctx, p, authz.PermTenantSettingsWrite, authz.Resource{TenantID: &tenant}); err != nil {
			return PolicyResult{}, err
		}
	case policyScopeSchool:
		s := scopeID
		school = &s
	default:
		return PolicyResult{}, problem.Invalid("scope_type", problem.SubInvalid, "tenant or school")
	}
	if school != nil {
		// A school of another tenant (or disabled) is 404 before any permission check; another school of the tenant is 403
		// (outside the grant), audited outside the write transaction.
		var ok bool
		if err := h.UoW.Do(ctx, func(ctx context.Context) error {
			var err error
			ok, err = h.Store.SchoolActive(ctx, tenant, *school)
			return err
		}); err != nil {
			return PolicyResult{}, err
		}
		if !ok {
			return PolicyResult{}, notFound()
		}
		if err := authz.Require(ctx, p, authz.PermSchoolSettingsWrite, authz.Resource{TenantID: &tenant, SchoolID: school}); err != nil {
			return PolicyResult{}, err
		}
	}
	curfew, err := h.curfew(ctx, tenant, school)
	if err != nil {
		return PolicyResult{}, err
	}
	now := h.now()
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		t := tenant
		sid := scopeID
		if err := h.Store.Lock(ctx, &t, scopeType, &sid); err != nil {
			return err
		}
		cur, err := h.Store.Level(ctx, &t, scopeType, &sid)
		if err != nil {
			return err
		}
		id, version := ids.New(), int64(1)
		var base json.RawMessage
		if cur != nil {
			id, version, base = cur.ID, cur.Version+1, cur.Rules
		}
		if (cur == nil && ifMatch != 0) || (cur != nil && cur.Version != ifMatch) {
			return problem.New(problem.VersionMismatch, policyVersionMismatchDetail)
		}
		raw, doc, err := h.document(scopeType, &sid, id, version, in, base)
		if err != nil {
			return err
		}
		upper, unit, err := h.upper(ctx, tenant, scopeType)
		if err != nil {
			return err
		}
		if err := policy.ValidateLevel(policy.Lower{Doc: doc, Upper: upper, Unit: unit, Curfew: curfew}); err != nil {
			return relaxProblem(err)
		}
		row := read.PolicyRow{ID: id, TenantID: &t, ScopeType: scopeType, ScopeID: &sid, SchoolID: school, Rules: raw, Version: version,
			Reason: &reason, UpdatedBy: &p.UserID, UpdatedAt: now}
		if err := h.save(ctx, cur, row); err != nil {
			return err
		}
		res = PolicyResult{Row: row}
		return h.recorded(ctx, cur, row, doc, p.UserID, school)
	})
	if err != nil {
		return PolicyResult{}, err
	}
	return res, nil
}

// PutPlatform implements putPlatformChatPolicy (System Admin, platform.settings.write; PlatformScope → role chat_platform).
func (h PolicyAdmin) PutPlatform(ctx context.Context, ifMatch int64, in PolicyInput) (res PolicyResult, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.PutPlatformChatPolicy")
	defer func() { h.count(err); done(err) }()
	p, err := staffPrincipal(ctx)
	if err != nil {
		return PolicyResult{}, err
	}
	if err := authz.Require(ctx, p, authz.PermPlatformSettingsWrite, authz.Resource{}); err != nil {
		return PolicyResult{}, err
	}
	reason, err := checkReason(in.Reason)
	if err != nil {
		return PolicyResult{}, err
	}
	now := h.now()
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.Store.Lock(ctx, nil, policyScopePlatform, nil); err != nil {
			return err
		}
		cur, err := h.Store.Level(ctx, nil, policyScopePlatform, nil)
		if err != nil {
			return err
		}
		if cur == nil {
			return problem.New(problem.Unavailable, "platform chat policy not seeded").Retry()
		}
		if cur.Version != ifMatch {
			return problem.New(problem.VersionMismatch, policyVersionMismatchDetail)
		}
		raw, doc, err := h.document(policyScopePlatform, nil, cur.ID, cur.Version+1, in, cur.Rules)
		if err != nil {
			return err
		}
		if err := policy.ValidatePlatformHours(doc, h.PlatformCurfew); err != nil {
			return relaxProblem(err)
		}
		row := read.PolicyRow{ID: cur.ID, ScopeType: policyScopePlatform, Rules: raw, Version: cur.Version + 1, Reason: &reason,
			UpdatedBy: &p.UserID, UpdatedAt: now}
		if err := h.save(ctx, cur, row); err != nil {
			return err
		}
		res = PolicyResult{Row: row}
		return h.recorded(ctx, cur, row, doc, p.UserID, nil)
	})
	if err != nil {
		return PolicyResult{}, err
	}
	return res, nil
}

func (h PolicyAdmin) curfew(ctx context.Context, tenant uuid.UUID, school *uuid.UUID) (policy.Curfew, error) {
	if h.Curfew == nil {
		return h.PlatformCurfew, nil
	}
	c, err := h.Curfew(ctx, tenant, school)
	if err != nil {
		return nil, problem.New(problem.Unavailable, "").Retry().Wrap(err)
	}
	return c, nil
}

// upper returns the levels above a tenant (platform, common defaults) or a school (also the tenant level).
func (h PolicyAdmin) upper(ctx context.Context, tenant uuid.UUID, scopeType string) (policy.Levels, bool, error) {
	var l policy.Levels
	pl, err := h.Store.Level(ctx, nil, policyScopePlatform, nil)
	if err != nil {
		return l, false, err
	}
	if pl == nil {
		return l, false, problem.New(problem.Unavailable, "platform chat policy not seeded").Retry()
	}
	d, err := policy.Parse(pl.Rules)
	if err != nil {
		return l, false, err
	}
	l.Platform = &d
	typ, err := h.Store.TenantType(ctx, tenant)
	if err != nil {
		return l, false, err
	}
	if typ == commonTenantType {
		o, err := h.Store.CommonOverride(ctx, tenant)
		if err != nil {
			return l, false, err
		}
		var ov policy.CommonOverride
		if o != nil {
			ov = o.Override
		}
		l.Common = policy.CommonDefaults(ov)
	}
	if scopeType == policyScopeSchool {
		t := tenant
		tl, err := h.Store.Level(ctx, &t, policyScopeTenant, &t)
		if err != nil {
			return l, false, err
		}
		if tl != nil {
			td, err := policy.Parse(tl.Rules)
			if err != nil {
				return l, false, err
			}
			l.Tenant = &td
		}
	}
	return l, typ == unitTenantType, nil
}

func (h PolicyAdmin) save(ctx context.Context, cur *read.PolicyRow, row read.PolicyRow) error {
	if cur == nil {
		return h.Store.InsertLevel(ctx, row)
	}
	ok, err := h.Store.UpdateLevel(ctx, row, cur.Version)
	if err != nil {
		return err
	}
	if !ok {
		return problem.New(problem.VersionMismatch, policyVersionMismatchDetail)
	}
	return nil
}

// changedFields lists the top-level keys whose value changed (event changed_fields; no values).
func changedFields(before, after json.RawMessage) []string {
	var b, a map[string]json.RawMessage
	_ = json.Unmarshal(before, &b)
	_ = json.Unmarshal(after, &a)
	var out []string
	for _, k := range []string{"rules", "global_rate", "world_scope", "chat_policy_preset"} {
		if string(b[k]) != string(a[k]) {
			out = append(out, k)
		}
	}
	if len(out) == 0 {
		out = []string{"reason"}
	}
	return out
}

// recorded writes the audit before/after and the event of a saved level, and bumps the cache generation after commit.
func (h PolicyAdmin) recorded(ctx context.Context, cur *read.PolicyRow, row read.PolicyRow, doc policy.Document, actor uuid.UUID,
	school *uuid.UUID) error {
	var before json.RawMessage
	beforeVersion := int64(0)
	if cur != nil {
		before, beforeVersion = cur.Rules, cur.Version
	}
	fields := changedFields(before, row.Rules)
	meta := map[string]any{"scope_type": row.ScopeType, "before_version": beforeVersion, "after_version": row.Version,
		"changed_fields": fields, "after": row.Rules}
	if before != nil {
		meta["before"] = before
	}
	ev := audit.Event{Action: AuditChatPolicyUpdated, TargetType: "chat_policy", TargetID: row.ID.String(), Result: audit.ResultSuccess,
		Reason: *row.Reason, Metadata: meta}
	if school != nil {
		ev.SchoolID = school.String()
	}
	if err := h.Audit.Record(ctx, ev); err != nil {
		return err
	}
	payload := map[string]any{"policy_id": row.ID, "scope_type": row.ScopeType, "policy_version": row.Version,
		"changed_fields": fields, "actor_id": actor, "occurred_at": row.UpdatedAt}
	if row.TenantID != nil {
		payload["tenant_id"] = *row.TenantID
	}
	if row.ScopeID != nil {
		payload["scope_id"] = *row.ScopeID
	}
	if doc.Preset != "" {
		payload["preset"] = doc.Preset
	}
	e := write.Event{Type: EventChatPolicyUpdated, SubjectType: "chat_policy", SubjectID: row.ID.String(),
		PartitionKey: row.ID.String(), OccurredAt: row.UpdatedAt, Payload: payload}
	if row.TenantID != nil {
		t := *row.TenantID
		e.TenantID = &t
	} else {
		e.Platform = true
	}
	if err := h.Events.Publish(ctx, e); err != nil {
		return err
	}
	return h.bump(ctx, row.TenantID)
}

func (h PolicyAdmin) bump(ctx context.Context, tenant *uuid.UUID) error {
	if h.Gens == nil || h.AfterCommit == nil {
		return nil
	}
	return h.AfterCommit(ctx, func(ctx context.Context) { _ = h.Gens.Bump(ctx, tenant) })
}

// CommonOverrideResult is the stored override of a tenant common.
type CommonOverrideResult struct{ Row read.CommonOverrideRow }

// PutCommonOverride implements putCommonTenantSocialOverride (F120-AC-28): System Admin only (platform.settings.write),
// the tenant of X-Tenant-Id must be a tenant common (404 otherwise), reason required, audit; effective at once on every
// instance (generation bump). A tenant role never reaches it (internal gateway + platform role guard, then authz here).
func (h PolicyAdmin) PutCommonOverride(ctx context.Context, in CommonOverrideInput) (res CommonOverrideResult, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.PutCommonTenantSocialOverride")
	defer func() { done(err) }()
	p, err := staffPrincipal(ctx)
	if err != nil {
		return CommonOverrideResult{}, err
	}
	tenant, err := sessionTenant(ctx)
	if err != nil {
		return CommonOverrideResult{}, err
	}
	if !p.HasPlatformRole() {
		authz.RecordDenied(ctx, string(problem.PermissionDenied), map[string]any{"why": "platform_role", "operation": "putCommonTenantSocialOverride"})
		return CommonOverrideResult{}, problem.New(problem.PermissionDenied, "")
	}
	if err := authz.Require(ctx, p, authz.PermPlatformSettingsWrite, authz.Resource{TenantID: &tenant}); err != nil {
		return CommonOverrideResult{}, err
	}
	reason, err := checkReason(in.Reason)
	if err != nil {
		return CommonOverrideResult{}, err
	}
	now := h.now()
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		typ, err := h.Store.TenantType(ctx, tenant)
		if err != nil {
			return err
		}
		if typ != commonTenantType {
			return notFound()
		}
		before, err := h.Store.CommonOverride(ctx, tenant)
		if err != nil {
			return err
		}
		row, err := h.Store.SaveCommonOverride(ctx, read.CommonOverrideRow{TenantID: tenant, Override: in.Override, Reason: reason,
			UpdatedBy: &p.UserID, UpdatedAt: now})
		if err != nil {
			return err
		}
		meta := map[string]any{"after": in.Override, "version": row.Version}
		if before != nil {
			meta["before"] = before.Override
		}
		if err := h.Audit.Record(ctx, audit.Event{Action: AuditCommonOverrideUpdated, TargetType: "tenant", TargetID: tenant.String(),
			Result: audit.ResultSuccess, Reason: reason, Metadata: meta}); err != nil {
			return err
		}
		res = CommonOverrideResult{Row: row}
		t := tenant
		return h.bump(ctx, &t)
	})
	if err != nil {
		return CommonOverrideResult{}, err
	}
	return res, nil
}
