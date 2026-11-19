package write

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent"
	entconn "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/sisconnection"
	entconnschool "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/sisconnectionschool"
	entrun "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/syncrun"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/sis"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// SisContractRepo implements write.SisContractRepository (F017).
type SisContractRepo struct{}

var _ write.SisContractRepository = SisContractRepo{}

func configMap(c sis.ContractConfig) (map[string]any, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	return m, json.Unmarshal(b, &m)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func contractRow(r *ent.SisConnection) (*write.ContractConnectionRow, error) {
	out := &write.ContractConnectionRow{ID: r.ID, TenantID: r.TenantID, ProviderKind: r.ProviderKind, ContractVersion: deref(r.ContractVersion),
		SecretRef: deref(r.SecretRef), WebhookSecretRef: deref(r.WebhookSecretRef), Schedule: deref(r.Schedule), Capabilities: r.Capabilities,
		State:          sis.ConnectionState{Status: r.Status, Reason: deref(r.StatusReason), FailedRuns: int(r.ConsecutiveFailedRuns)},
		DeltaWatermark: deref(r.DeltaWatermark), LastHealthAt: r.LastHealthAt, LastSuccessAt: r.LastSuccessAt, CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt, Version: r.Version}
	if out.Capabilities == nil {
		out.Capabilities = []string{}
	}
	if r.Config != nil {
		b, err := json.Marshal(r.Config)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(b, &out.Config); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// CreateContract inserts a contract_v1 connection (tenant explicit).
func (SisContractRepo) CreateContract(ctx context.Context, c *write.ContractConnectionRow) error {
	cfg, err := configMap(c.Config)
	if err != nil {
		return err
	}
	q := client(ctx).SisConnection.Create().SetID(c.ID).SetTenantID(c.TenantID).SetProviderKind(sis.ProviderContractV1).
		SetContractVersion(c.ContractVersion).SetConfig(cfg).SetSecretRef(c.SecretRef).SetStatus(c.State.Status).
		SetCapabilities(c.Capabilities).SetConsecutiveFailedRuns(0)
	if c.WebhookSecretRef != "" {
		q = q.SetWebhookSecretRef(c.WebhookSecretRef)
	}
	if c.Schedule != "" {
		q = q.SetSchedule(c.Schedule)
	}
	row, err := q.Save(ctx)
	if err != nil {
		return err
	}
	c.CreatedAt, c.UpdatedAt, c.Version = row.CreatedAt, row.UpdatedAt, row.Version
	return nil
}

// GetContract loads a connection of the scope.
func (SisContractRepo) GetContract(ctx context.Context, id uuid.UUID) (*write.ContractConnectionRow, error) {
	row, err := client(ctx).SisConnection.Get(ctx, id)
	if ent.IsNotFound(err) {
		return nil, problem.New(problem.NotFound, "")
	}
	if err != nil {
		return nil, err
	}
	return contractRow(row)
}

func optRef(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// SaveContract updates the mutable columns with optimistic concurrency.
func (SisContractRepo) SaveContract(ctx context.Context, c *write.ContractConnectionRow) error {
	u := client(ctx).SisConnection.Update().Where(entconn.ID(c.ID), entconn.Version(c.Version)).
		SetStatus(c.State.Status).SetNillableStatusReason(optRef(c.State.Reason)).SetConsecutiveFailedRuns(int32(c.State.FailedRuns)).
		SetCapabilities(c.Capabilities).SetNillableLastHealthAt(c.LastHealthAt).SetNillableLastSuccessAt(c.LastSuccessAt).AddVersion(1)
	if c.State.Reason == "" {
		u = u.ClearStatusReason()
	}
	if c.ProviderKind == sis.ProviderContractV1 {
		cfg, err := configMap(c.Config)
		if err != nil {
			return err
		}
		u = u.SetConfig(cfg).SetContractVersion(c.ContractVersion).SetSecretRef(c.SecretRef)
		if c.WebhookSecretRef == "" {
			u = u.ClearWebhookSecretRef()
		} else {
			u = u.SetWebhookSecretRef(c.WebhookSecretRef)
		}
		if c.Schedule == "" {
			u = u.ClearSchedule()
		} else {
			u = u.SetSchedule(c.Schedule)
		}
		if c.DeltaWatermark == "" {
			u = u.ClearDeltaWatermark()
		} else {
			u = u.SetDeltaWatermark(c.DeltaWatermark)
		}
	}
	n, err := u.Save(ctx)
	if err != nil {
		return err
	}
	if n == 0 {
		return problem.New(problem.VersionMismatch, "the connection changed concurrently")
	}
	c.Version++
	return nil
}

// PendingPullRun reports a queued or running pull run started after since.
func (SisContractRepo) PendingPullRun(ctx context.Context, conn uuid.UUID, since time.Time) (bool, error) {
	// A lookup run of login sync (F011 ApplyLookup: mode delta, trigger manual, no requester) is not a pull run.
	return client(ctx).SyncRun.Query().Where(entrun.SisConnectionID(conn), entrun.StatusIn("queued", "running"),
		entrun.ModeIn(sis.ModeFull, sis.ModeDelta, sis.ModeWebhook), entrun.StartedAtGT(since),
		entrun.Or(entrun.TriggerNEQ(sis.TriggerManual), entrun.RequestedByNotNil())).Exist(ctx)
}

// GetRun loads a run (nil when absent).
func (SisContractRepo) GetRun(ctx context.Context, id uuid.UUID) (*write.SyncRun, error) {
	r, err := client(ctx).SyncRun.Get(ctx, id)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &write.SyncRun{ID: r.ID, TenantID: r.TenantID, ConnectionID: r.SisConnectionID, SchoolID: r.SchoolID, Mode: r.Mode, DryRun: r.DryRun,
		Status: r.Status, FailureCode: r.FailureCode, Stats: r.Stats, StartedAt: r.StartedAt, FinishedAt: r.FinishedAt, Trigger: r.Trigger,
		RequestedBy: r.RequestedBy, DeliveryID: r.DeliveryID}, nil
}

// StartRun moves a queued run to running.
func (SisContractRepo) StartRun(ctx context.Context, id uuid.UUID) error {
	n, err := client(ctx).SyncRun.Update().Where(entrun.ID(id), entrun.Status("queued")).SetStatus("running").AddVersion(1).Save(ctx)
	if err != nil {
		return err
	}
	if n == 0 {
		return problem.New(problem.Conflict, "the run is not queued")
	}
	return nil
}

// ActiveContractConnections lists active contract_v1 connections (runs under a platform_job bypass).
func (SisContractRepo) ActiveContractConnections(ctx context.Context) ([]write.ContractConnectionRow, error) {
	rows, err := client(ctx).SisConnection.Query().Where(entconn.ProviderKind(sis.ProviderContractV1), entconn.Status(sis.StatusActive)).
		Order(ent.Asc(entconn.FieldID)).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]write.ContractConnectionRow, 0, len(rows))
	for _, r := range rows {
		c, err := contractRow(r)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, nil
}

// ActiveSchoolIDs returns the active schools of the connection.
func (SisContractRepo) ActiveSchoolIDs(ctx context.Context, conn uuid.UUID) (map[uuid.UUID]bool, error) {
	rows, err := client(ctx).SisConnectionSchool.Query().Where(entconnschool.SisConnectionID(conn), entconnschool.Active(true)).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]bool, len(rows))
	for _, r := range rows {
		out[r.SchoolID] = true
	}
	return out, nil
}

// TenantOfAny returns the tenant of a connection (runs under a bypass).
func (r SisContractRepo) TenantOfAny(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	return SisConnectionRepo{}.TenantOf(ctx, id)
}
