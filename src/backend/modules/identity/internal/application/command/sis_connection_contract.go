package command

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/sis"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// SisContract implements the contract_v1 connection operations of F017 (createSisConnection branch
// contract_v1, updateSisConnection, checkSisConnectionHealth; F017-AC-01, AC-02, AC-04, AC-13, AC-28).
// Secret references are stored but never returned, logged nor audited.
type SisContract struct {
	UoW         write.UnitOfWork
	Connections write.SisConnectionRepository
	Repo        write.SisContractRepository
	Schools     write.SchoolRepository
	Providers   port.SisProviderFactory
	Audit       write.Auditor
	// AllowHTTP accepts http base URLs (EW_SIS_ALLOW_HTTP_BASE_URL, compose dev only); https otherwise.
	AllowHTTP bool
	Clock     func() time.Time
}

func (h SisContract) now() time.Time {
	if h.Clock != nil {
		return h.Clock().UTC()
	}
	return time.Now().UTC()
}

// ContractCreateInput is the contract_v1 branch of SisConnectionCreate.
type ContractCreateInput struct {
	ContractVersion  string
	Config           sis.ContractConfig
	SecretRef        string
	WebhookSecretRef string
	Schedule         string
	Reason           string
}

// ContractPatch is a JSON Merge Patch of updateSisConnection (nil = unchanged).
type ContractPatch struct {
	Config           *sis.ContractConfig
	SecretRef        *string
	WebhookSecretRef *string // "" clears it
	Schedule         *string // "" clears it
	Status           *string // active | paused
	Reason           string
}

func invalidFields(fes []sis.FieldError) error {
	if len(fes) == 0 {
		return nil
	}
	e := problem.Invalid(fes[0].Field, problem.SubInvalid, fes[0].Message)
	for _, f := range fes[1:] {
		e = e.AddField(f.Field, problem.SubInvalid, f.Message)
	}
	return e
}

// ContractView maps a connection row to the API view (flags instead of secret references).
func ContractView(c *write.ContractConnectionRow, schoolCount int) dto.SisConnection {
	updated := c.UpdatedAt
	v := dto.SisConnection{ConnectionID: c.ID, TenantID: c.TenantID, ProviderKind: c.ProviderKind, Status: c.State.Status, SchoolCount: schoolCount,
		CreatedAt: c.CreatedAt, UpdatedAt: &updated, HasSecret: c.SecretRef != "", HasWebhookSecret: c.WebhookSecretRef != "",
		Capabilities: c.Capabilities, ConsecutiveFailedRuns: c.State.FailedRuns, LastHealthAt: c.LastHealthAt, LastSuccessAt: c.LastSuccessAt,
		Version: c.Version}
	if v.Capabilities == nil {
		v.Capabilities = []string{}
	}
	if c.ContractVersion != "" {
		cv := c.ContractVersion
		v.ContractVersion = &cv
	}
	if c.ProviderKind == sis.ProviderContractV1 {
		cfg := c.Config
		v.Config = &cfg
	}
	if c.Schedule != "" {
		s := c.Schedule
		v.Schedule = &s
	}
	if c.State.Reason != "" {
		r := c.State.Reason
		v.StatusReason = &r
	}
	return v
}

// checkSchools verifies that every mapped school is a live school of the tenant scope (422 otherwise).
func (h SisContract) checkSchools(ctx context.Context, cfg sis.ContractConfig) error {
	for i, s := range cfg.Schools {
		if _, err := h.Schools.Get(ctx, s.SchoolID); err != nil {
			var pe *problem.Error
			if errors.As(err, &pe) && pe.Code == problem.NotFound {
				return problem.Invalid("config.schools", problem.SubInvalid, "school "+s.SchoolID.String()+" is not a school of the tenant").
					With("index", i)
			}
			return err
		}
	}
	return nil
}

// Create creates a contract_v1 connection in the target tenant (X-Tenant-Id).
func (h SisContract) Create(ctx context.Context, tenantID uuid.UUID, in ContractCreateInput) (out dto.SisConnection, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.CreateSisConnectionContract")
	defer func() { done(err) }()
	if ctx, _, err = platformTarget(ctx, tenantID); err != nil {
		return out, err
	}
	if err := invalidFields(sis.ValidateContract(in.Config, in.SecretRef, in.WebhookSecretRef, in.ContractVersion, in.Schedule, h.AllowHTTP)); err != nil {
		return out, err
	}
	c := write.ContractConnectionRow{ID: ids.New(), TenantID: tenantID, ProviderKind: sis.ProviderContractV1, ContractVersion: in.ContractVersion,
		Config: in.Config, SecretRef: in.SecretRef, WebhookSecretRef: in.WebhookSecretRef, Schedule: in.Schedule, Capabilities: []string{},
		State: sis.ConnectionState{Status: sis.StatusActive}}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.checkSchools(ctx, in.Config); err != nil {
			return err
		}
		if err := h.Repo.CreateContract(ctx, &c); err != nil {
			return err
		}
		if err := h.Audit.Record(ctx, audit.Event{Action: "sis_connection.created", TargetType: "sis_connection", TargetID: c.ID.String(),
			Result: audit.ResultSuccess, Reason: in.Reason, Metadata: map[string]any{"provider_kind": sis.ProviderContractV1,
				"contract_version": in.ContractVersion, "auth_type": in.Config.AuthType, "school_count": len(in.Config.Schools),
				"has_secret": true, "has_webhook_secret": in.WebhookSecretRef != ""}}); err != nil {
			return err
		}
		out = ContractView(&c, 0)
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// connection resolves the tenant of a connection (System Admin bypass, audited), scopes ctx to it and
// loads the row.
func (h SisContract) connection(ctx context.Context, id uuid.UUID) (context.Context, *write.ContractConnectionRow, error) {
	ctx, _, err := SisConnections{UoW: h.UoW, Connections: h.Connections}.connectionScope(ctx, id)
	if err != nil {
		return ctx, nil, err
	}
	var c *write.ContractConnectionRow
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		c, err = h.Repo.GetContract(ctx, id)
		return err
	})
	return ctx, c, err
}

// healthOutcome is the result of a getHealth call applied to the connection.
type healthOutcome struct {
	health     port.SisHealth
	compatible bool
	err        error // problem returned to the caller after the state is saved
}

// probe calls getHealth and updates c (capabilities, last_health_at, auth_error/contract_error); it never
// changes the database (the caller saves c).
func probe(ctx context.Context, providers port.SisProviderFactory, c *write.ContractConnectionRow, now time.Time) healthOutcome {
	p, err := providers.For(ctx, contractConn(c))
	var hl port.SisHealth
	if err == nil {
		hl, err = p.Health(ctx)
	}
	switch {
	case errors.Is(err, port.ErrSisSecret):
		c.State.MarkAuthError(sis.ReasonSecretUnavailable)
		return healthOutcome{err: problem.New(problem.SisUnavailable, "the credential of the connection cannot be read")}
	case errors.Is(err, port.ErrSisAuth):
		c.State.MarkAuthError(sis.ReasonCredentialRejected)
		return healthOutcome{err: problem.New(problem.SisUnavailable, "the SIS rejected the credential")}
	case err != nil:
		return healthOutcome{err: problem.New(problem.SisUnavailable, "the SIS facade did not answer").Retry()}
	}
	c.LastHealthAt = &now
	c.Capabilities = hl.Capabilities
	out := healthOutcome{health: hl, compatible: sis.CompatibleMajor(c.ContractVersion, hl.ContractVersion)}
	if !out.compatible {
		c.State.MarkContractError(sis.ReasonVersionMismatch) // F017-AC-02
		out.err = problem.New(problem.SisContractVersionUnsupported, "contract_version "+hl.ContractVersion+" of the facade")
	}
	return out
}

func contractConn(c *write.ContractConnectionRow) port.ContractConnection {
	return port.ContractConnection{ID: c.ID, TenantID: c.TenantID, BaseURL: c.Config.BaseURL, AuthType: c.Config.AuthType, TokenURL: c.Config.TokenURL,
		OAuthScope: c.Config.OAuthScope, SecretRef: c.SecretRef}
}

// CheckHealth calls getHealth, stores capabilities and the check time (F017-AC-02).
func (h SisContract) CheckHealth(ctx context.Context, id uuid.UUID) (out dto.SisConnectionHealth, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.CheckSisConnectionHealth")
	defer func() { done(err) }()
	ctx, c, err := h.connection(ctx, id)
	if err != nil {
		return out, err
	}
	if c.ProviderKind != sis.ProviderContractV1 {
		return out, problem.Invalid("connection_id", problem.SubInvalid, "health checks apply to contract_v1 connections")
	}
	before := c.State
	now := h.now()
	res := probe(ctx, h.Providers, c, now)
	if err := h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.Repo.SaveContract(ctx, c); err != nil {
			return err
		}
		return h.auditStatus(ctx, c, before, "health_check")
	}); err != nil {
		return out, err
	}
	if res.err != nil {
		return out, res.err
	}
	cv := res.health.ContractVersion
	status := res.health.Status
	return dto.SisConnectionHealth{Status: status, ContractVersion: &cv, Compatible: res.compatible, Capabilities: c.Capabilities, CheckedAt: now}, nil
}

// auditStatus records sis_connection.status_changed when the status moved (never a secret reference).
func (h SisContract) auditStatus(ctx context.Context, c *write.ContractConnectionRow, before sis.ConnectionState, cause string) error {
	if before.Status == c.State.Status {
		return nil
	}
	return h.Audit.Record(ctx, audit.Event{Action: "sis_connection.status_changed", TargetType: "sis_connection", TargetID: c.ID.String(),
		Result: audit.ResultSuccess, Metadata: map[string]any{"from": before.Status, "to": c.State.Status, "status_reason": c.State.Reason, "cause": cause, "actor": "system:sis-sync:" + c.ID.String()}})
}

// Update applies a merge patch (If-Match = version). Reactivation runs a health check first (F017-AC-13).
func (h SisContract) Update(ctx context.Context, id uuid.UUID, version int64, in ContractPatch) (out dto.SisConnection, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.UpdateSisConnection")
	defer func() { done(err) }()
	ctx, c, err := h.connection(ctx, id)
	if err != nil {
		return out, err
	}
	if c.Version != version {
		return out, problem.New(problem.VersionMismatch, "")
	}
	before := c.State
	changed := []string{}
	if in.Config != nil || in.SecretRef != nil || in.WebhookSecretRef != nil || in.Schedule != nil {
		if c.ProviderKind != sis.ProviderContractV1 {
			return out, problem.Invalid("provider_kind", problem.SubInvalid, "only contract_v1 connections have a configuration")
		}
		if in.Config != nil {
			c.Config, changed = *in.Config, append(changed, "config")
		}
		if in.SecretRef != nil {
			c.SecretRef, changed = *in.SecretRef, append(changed, "secret_ref")
		}
		if in.WebhookSecretRef != nil {
			c.WebhookSecretRef, changed = *in.WebhookSecretRef, append(changed, "webhook_secret_ref")
		}
		if in.Schedule != nil {
			c.Schedule, changed = *in.Schedule, append(changed, "schedule")
		}
		if err := invalidFields(sis.ValidateContract(c.Config, c.SecretRef, c.WebhookSecretRef, c.ContractVersion, c.Schedule, h.AllowHTTP)); err != nil {
			return out, err
		}
	}
	var res healthOutcome
	if in.Status != nil {
		if in.Reason == "" {
			return out, problem.Invalid("reason", problem.SubRequired, "required when status changes")
		}
		switch *in.Status {
		case sis.StatusPaused:
			c.State.Pause()
		case sis.StatusActive:
			if c.ProviderKind == sis.ProviderContractV1 {
				res = probe(ctx, h.Providers, c, h.now())
			}
			if res.err == nil {
				c.State.Activate()
			}
		default:
			return out, problem.Invalid("status", problem.SubInvalid, "must be active or paused")
		}
		changed = append(changed, "status")
	}
	if res.err != nil && c.State == before {
		return out, res.err // facade unavailable: nothing to save
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if res.err == nil {
			if err := h.checkSchools(ctx, c.Config); err != nil {
				return err
			}
		}
		if err := h.Repo.SaveContract(ctx, c); err != nil {
			return err
		}
		if res.err == nil {
			if err := h.Audit.Record(ctx, audit.Event{Action: "sis_connection.updated", TargetType: "sis_connection", TargetID: c.ID.String(),
				Result: audit.ResultSuccess, Reason: in.Reason, Metadata: map[string]any{"changed_fields": changed}}); err != nil {
				return err
			}
		}
		return h.auditStatus(ctx, c, before, "update")
	})
	if err != nil {
		return out, err
	}
	if res.err != nil {
		return out, res.err // the new state (auth_error/contract_error) is saved, the reactivation refused
	}
	n, err := h.schoolCount(ctx, c.ID)
	if err != nil {
		return out, err
	}
	return ContractView(c, n), nil
}

func (h SisContract) schoolCount(ctx context.Context, id uuid.UUID) (n int, err error) {
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		ids, err := h.Repo.ActiveSchoolIDs(ctx, id)
		n = len(ids)
		return err
	})
	return n, err
}
