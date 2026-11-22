package command

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/saga"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// SagaUserAnonymize is the saga type of the anonymization (F016 US5).
const SagaUserAnonymize = "user_anonymize"

// Triggers of user.anonymized.
const (
	AnonymizeTriggerRequest   = "data_request" // sagas recorded before the intake left Release 1 (owner 2026-11-22)
	AnonymizeTriggerRetention = "retention"
)

// DefaultStudentDataRetentionDays is student_data_retention (DATA_PRIVACY §2: one year after the student is
// deactivated, CONFIG_DEFAULTS); the binary passes the configured value.
const DefaultStudentDataRetentionDays = 365

// UserAnonymize implements the anonymization (F016 US5, FR-ORG-014, NFR-PRIV-003, ADR-0028 §8, ADR-0070):
// the job retention-anonymizer starts the saga user_anonymize (the portal data-request intake createDataRequest is out of
// Release 1, owner 2026-11-22; a saga recorded earlier with a request id still finishes it)
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
