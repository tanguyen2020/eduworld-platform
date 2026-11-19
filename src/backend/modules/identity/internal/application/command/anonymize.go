package command

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/saga"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// SagaUserAnonymize is the saga type of the anonymization (F016 US5).
const SagaUserAnonymize = "user_anonymize"

// Triggers of user.anonymized.
const (
	AnonymizeTriggerRequest   = "data_request"
	AnonymizeTriggerRetention = "retention"
)

// DefaultStudentDataRetentionDays is student_data_retention (DATA_PRIVACY §2: one year after the student is
// deactivated, CONFIG_DEFAULTS); the binary passes the configured value.
const DefaultStudentDataRetentionDays = 365

// DataRequestView is DataRequest of the contract (statuses mapped from platform.deletion_requests).
type DataRequestView struct {
	ID            uuid.UUID  `json:"data_request_id"`
	TenantID      uuid.UUID  `json:"tenant_id"`
	RequestType   string     `json:"request_type"`
	SubjectUserID uuid.UUID  `json:"subject_user_id"`
	RequesterKind string     `json:"requester_kind"`
	Reference     *string    `json:"reference,omitempty"`
	Reason        string     `json:"reason"`
	Status        string     `json:"status"`
	DownloadURL   *string    `json:"download_url"`
	CreatedAt     time.Time  `json:"created_at"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
}

// DataRequestInput is DataRequestCreate.
type DataRequestInput struct {
	RequestType   string    `json:"request_type"`
	SubjectUserID uuid.UUID `json:"subject_user_id"`
	RequesterKind string    `json:"requester_kind"`
	Reference     *string   `json:"reference"`
	Reason        string    `json:"reason"`
}

// requestStatus maps the stored status to DataRequest.status.
func requestStatus(s string) string {
	switch s {
	case "pending":
		return "received"
	case "approved":
		return "in_progress"
	case "executed":
		return "completed"
	}
	return s // rejected
}

func dataRequestView(r write.DataRequest) DataRequestView {
	return DataRequestView{ID: r.ID, TenantID: r.TenantID, RequestType: "anonymize", SubjectUserID: r.SubjectID, RequesterKind: r.RequesterKind,
		Reference: r.Reference, Reason: r.Reason, Status: requestStatus(r.Status), CreatedAt: r.CreatedAt, CompletedAt: r.ExecutedAt}
}

// UserAnonymize implements the anonymization (F016 US5, FR-ORG-014, NFR-PRIV-003, ADR-0028 §8, ADR-0070):
// createDataRequest (kind anonymize) and the job retention-anonymizer start the saga user_anonymize
// (anonymizeUserData of every participant, then identity overwrites the PII, appends the erasure log and
// publishes user.anonymized; the Keycloak worker disables and clears the Keycloak user). Nothing is deleted.
type UserAnonymize struct {
	UoW           write.UnitOfWork
	Repo          write.AnonymizeRepository
	Saga          saga.Engine
	Participants  []port.UserDataAnonymizer
	Lifecycle     write.LifecycleRepository
	Events        write.EventPublisher
	Audit         write.Auditor
	RetentionDays int // student_data_retention in days (0 = DefaultStudentDataRetentionDays)
	Clock         func() time.Time
}

func (h UserAnonymize) now() time.Time {
	if h.Clock != nil {
		return h.Clock().UTC()
	}
	return time.Now().UTC()
}

func (h UserAnonymize) platform(ctx context.Context, fn func(ctx context.Context) error) error {
	sc, err := authz.SystemPlatformScope("user-anonymize")
	if err != nil {
		return err
	}
	return h.UoW.Do(tenancy.WithScope(ctx, sc), fn)
}

// anonymizeSubject is the subject of a user_anonymize saga (no PII).
type anonymizeSubject struct {
	UserID      uuid.UUID  `json:"user_id"`
	TenantID    uuid.UUID  `json:"tenant_id"`
	UserType    string     `json:"user_type"`
	From        string     `json:"from_status"`
	ErasureID   uuid.UUID  `json:"erasure_id"`
	RequestID   *uuid.UUID `json:"data_request_id"`
	Trigger     string     `json:"trigger"`
	RequestedBy *uuid.UUID `json:"requested_by"`
}

// facadeReason is UserAnonymizeRequest.reason.
func (s anonymizeSubject) facadeReason() string {
	if s.Trigger == AnonymizeTriggerRetention {
		return "retention_expired"
	}
	return "data_subject_request"
}

// Definition is the user_anonymize saga (also used by the resume job). A step has no compensation: a
// blocked or failed participant leaves the request in progress until the saga is resumed (F016-AC-21).
func (h UserAnonymize) Definition() saga.Definition {
	subj := func(run write.SagaRun) (anonymizeSubject, error) {
		var s anonymizeSubject
		return s, json.Unmarshal(run.Subject, &s)
	}
	def := saga.Definition{Type: SagaUserAnonymize}
	for _, p := range h.Participants {
		p := p
		def.Steps = append(def.Steps, saga.Step{Service: p.Name(), Action: "anonymizeUserData",
			Do: func(ctx context.Context, run write.SagaRun) (saga.Result, error) {
				s, err := subj(run)
				if err != nil {
					return saga.Result{}, problem.New(problem.ValidationFailed, "saga subject").Wrap(err)
				}
				if ctx, err = tenantCtx(ctx, s.TenantID); err != nil {
					return saga.Result{}, err
				}
				r, err := p.Anonymize(ctx, s.UserID, s.ErasureID, s.facadeReason())
				return saga.Result{Outcome: r.Status, Response: map[string]any{"status": r.Status, "blocked_reason": r.BlockedReason}}, err
			}})
	}
	def.Finish = func(ctx context.Context, run write.SagaRun) error {
		s, err := subj(run)
		if err != nil {
			return problem.New(problem.ValidationFailed, "saga subject").Wrap(err)
		}
		return h.platform(ctx, func(ctx context.Context) error { return h.finish(ctx, run, s) })
	}
	return def
}

func (h UserAnonymize) finish(ctx context.Context, run write.SagaRun, s anonymizeSubject) error {
	at := h.now()
	changed, err := h.Repo.AnonymizeUser(ctx, s.UserID, at)
	if err != nil || !changed {
		return err // already anonymized: idempotent
	}
	tenant := s.TenantID
	source := "anonymize"
	if s.Trigger == AnonymizeTriggerRetention {
		source = "retention"
	}
	if err := h.Lifecycle.RecordStatusChange(ctx, write.StatusChange{UserID: s.UserID, TenantID: &tenant, From: s.From, To: "anonymized",
		Source: source, By: s.RequestedBy}); err != nil {
		return err
	}
	modules := map[string]any{"identity": "done"}
	for _, st := range run.Steps {
		modules[st.Service] = st.Status
	}
	basis := "retention_expired:student_data_retention"
	if s.RequestID != nil {
		basis = "data_request:" + s.RequestID.String()
	}
	if err := h.Repo.AppendErasure(ctx, write.ErasureEntry{ID: s.ErasureID, UserID: s.UserID, RequestedBy: s.RequestedBy, LegalBasis: basis,
		Scope: map[string]any{"saga_id": run.ID.String(), "modules": modules, "trigger": s.Trigger}}); err != nil {
		return err
	}
	if s.RequestID != nil {
		if err := h.Repo.FinishRequest(ctx, *s.RequestID, s.ErasureID, at); err != nil {
			return err
		}
	}
	var req any
	if s.RequestID != nil {
		req = s.RequestID.String()
	}
	if err := h.Events.Publish(ctx, write.Event{Type: "user.anonymized", SubjectType: "user", SubjectID: s.UserID.String(), TenantID: &tenant,
		Payload: map[string]any{"user_id": s.UserID, "tenant_id": tenant, "user_type": s.UserType, "trigger": s.Trigger,
			"data_request_id": req, "anonymized_at": at.Format(time.RFC3339)}}); err != nil {
		return err
	}
	meta := map[string]any{"erasure_id": s.ErasureID.String(), "saga_id": run.ID.String(), "trigger": s.Trigger}
	if s.RequestID != nil {
		meta["data_request_id"] = s.RequestID.String()
	}
	return h.Audit.Record(ctx, audit.Event{Action: "user.anonymized", TargetType: "user", TargetID: s.UserID.String(),
		Result: audit.ResultSuccess, Reason: s.Trigger, Metadata: meta})
}

func (h UserAnonymize) start(ctx context.Context, s anonymizeSubject) (saga.Outcome, error) {
	key := fmt.Sprintf("anonymize:%s:%s", s.UserID, s.ErasureID)
	return h.Saga.Start(ctx, h.Definition(), &s.TenantID, s, key, s.RequestedBy)
}

// CreateDataRequest implements createDataRequest (platform.data_request.write, X-Tenant-Id = tenant of the
// subject, F016-AC-22). Only request_type anonymize is served: export is deferred (spec Out of Scope) and
// answers 422. [Proposed] A request recorded by the System Admin (MFA) is approved at once (OQ F016-11);
// the saga starts right away and the response carries its state.
func (h UserAnonymize) CreateDataRequest(ctx context.Context, in DataRequestInput) (out DataRequestView, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.CreateDataRequest")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	if err := authz.Require(ctx, p, authz.PermPlatformDataRequestWrite, authz.Resource{}); err != nil {
		return out, err
	}
	if p.TenantID == nil {
		return out, problem.Invalid("X-Tenant-Id", problem.SubRequired, "the tenant of the subject")
	}
	switch {
	case in.RequestType == "export":
		return out, problem.Invalid("request_type", problem.SubInvalid, "export is not available in this version")
	case in.RequestType != "anonymize":
		return out, problem.Invalid("request_type", problem.SubInvalid, "anonymize")
	case len(strings.TrimSpace(in.Reason)) < 10:
		return out, problem.Invalid("reason", problem.SubInvalid, "10..1000 characters")
	}
	tenant := *p.TenantID
	by := p.UserID
	var subject anonymizeSubject
	var req write.DataRequest
	err = h.platform(ctx, func(ctx context.Context) error {
		sub, err := h.Repo.Subject(ctx, in.SubjectUserID)
		if err != nil {
			return err
		}
		if sub == nil || sub.TenantID == nil || *sub.TenantID != tenant || sub.UserType != string(authz.UserTypeStudent) {
			return problem.New(problem.NotFound, "") // never reveal a user of another tenant
		}
		if sub.Status == "anonymized" {
			return problem.New(problem.Conflict, "already anonymized")
		}
		req = write.DataRequest{ID: uuid.Must(uuid.NewV7()), SubjectID: in.SubjectUserID, TenantID: tenant, Status: "approved",
			Reason: strings.TrimSpace(in.Reason), RequesterKind: in.RequesterKind, Reference: in.Reference, RequestedBy: by, ApprovedBy: &by,
			CreatedAt: h.now()}
		if err := h.Repo.CreateRequest(ctx, req); err != nil {
			if strings.Contains(err.Error(), "deletion_requests_open_uq") {
				return problem.New(problem.Conflict, "an open request exists for this user")
			}
			return err
		}
		id := req.ID
		subject = anonymizeSubject{UserID: in.SubjectUserID, TenantID: tenant, UserType: sub.UserType, From: sub.Status,
			ErasureID: uuid.Must(uuid.NewV7()), RequestID: &id, Trigger: AnonymizeTriggerRequest, RequestedBy: &by}
		return h.Audit.Record(ctx, audit.Event{Action: "data_request.created", TargetType: "user", TargetID: in.SubjectUserID.String(),
			Result: audit.ResultSuccess, Reason: "anonymize", Metadata: map[string]any{"data_request_id": req.ID.String(),
				"requester_kind": in.RequesterKind}})
	})
	if err != nil {
		return out, err
	}
	o, err := h.start(ctx, subject)
	if err != nil {
		return dataRequestView(req), nil // recorded; the saga-retry job resumes it
	}
	if o.Status == saga.StatusSucceeded {
		req.Status = "executed"
		at := h.now()
		req.ExecutedAt = &at
	}
	return dataRequestView(req), nil
}

// dataRequestReader checks platform.data_request.write for listDataRequests/getDataRequest and returns the
// tenants the System Admin chose (nil = every tenant).
func dataRequestReader(ctx context.Context) ([]uuid.UUID, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	if err := authz.Require(ctx, p, authz.PermPlatformDataRequestWrite, authz.Resource{}); err != nil {
		return nil, err
	}
	if p.TenantID != nil {
		return []uuid.UUID{*p.TenantID}, nil
	}
	if s, ok := tenancy.ScopeFrom(ctx); ok {
		if cs, ok := s.(tenancy.CrossTenantScope); ok {
			return cs.TenantIDs(), nil
		}
	}
	return nil, nil
}

// ListDataRequests implements listDataRequests.
func (h UserAnonymize) ListDataRequests(ctx context.Context, tenantIDs []uuid.UUID, status string, limit int) (out []DataRequestView, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.ListDataRequests")
	defer func() { done(err) }()
	tenants, err := dataRequestReader(ctx)
	if err != nil {
		return nil, err
	}
	if tenants == nil && len(tenantIDs) > 0 { // System Admin filter (tenant_ids)
		if len(tenantIDs) > 100 {
			return nil, problem.Invalid("tenant_ids", problem.SubInvalid, "at most 100 tenants")
		}
		tenants = tenantIDs
	}
	stored := map[string]string{"received": "pending", "in_progress": "approved", "completed": "executed", "rejected": "rejected", "": ""}[status]
	out = []DataRequestView{}
	err = h.platform(ctx, func(ctx context.Context) error {
		rows, err := h.Repo.ListRequests(ctx, tenants, stored, page.Limit(limit))
		for _, r := range rows {
			out = append(out, dataRequestView(r))
		}
		return err
	})
	return out, err
}

// GetDataRequest implements getDataRequest (404 outside the chosen tenants).
func (h UserAnonymize) GetDataRequest(ctx context.Context, id uuid.UUID) (out DataRequestView, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.GetDataRequest")
	defer func() { done(err) }()
	tenants, err := dataRequestReader(ctx)
	if err != nil {
		return out, err
	}
	err = h.platform(ctx, func(ctx context.Context) error {
		r, err := h.Repo.Request(ctx, id)
		if err != nil {
			return err
		}
		if r == nil || (tenants != nil && !containsID(tenants, r.TenantID)) {
			return problem.New(problem.NotFound, "")
		}
		out = dataRequestView(*r)
		return nil
	})
	return out, err
}

func containsID(ids []uuid.UUID, id uuid.UUID) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// RunRetention is the job retention-anonymizer (NFR-PRIV-003, DATA_PRIVACY §2 student_data_retention):
// students deactivated longer than the retention are anonymized; returns how many sagas succeeded.
func (h UserAnonymize) RunRetention(ctx context.Context) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "identity.job.RetentionAnonymizer")
	defer func() { done(err) }()
	days := h.RetentionDays
	if days <= 0 {
		days = DefaultStudentDataRetentionDays
	}
	var due []write.AnonymizeSubject
	if err := h.platform(ctx, func(ctx context.Context) error {
		due, err = h.Repo.RetentionDue(ctx, h.now().AddDate(0, 0, -days), 100)
		return err
	}); err != nil {
		return 0, err
	}
	for _, s := range due {
		o, err := h.start(ctx, anonymizeSubject{UserID: s.UserID, TenantID: *s.TenantID, UserType: s.UserType, From: s.Status,
			ErasureID: uuid.NewSHA1(uuid.NameSpaceOID, []byte("retention:"+s.UserID.String())), Trigger: AnonymizeTriggerRetention})
		if err != nil {
			return n, err
		}
		if o.Status == saga.StatusSucceeded {
			n++
		}
	}
	return n, nil
}
