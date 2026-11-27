package command

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/gameconfig"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// ActionContentRestored is the audit action of a restore from the trash (ADMIN_PORTAL audit catalog, ADR-0028).
const ActionContentRestored = "content.restored"

// RestoredPayload is the payload of <resource_type>.restored (specs/events/payloads/game/resource.restored.v1.schema.json).
type RestoredPayload struct {
	ResourceType string     `json:"resource_type"`
	ResourceID   uuid.UUID  `json:"resource_id"`
	TenantID     *uuid.UUID `json:"tenant_id"`
	RestoredBy   *uuid.UUID `json:"restored_by"`
	Reason       string     `json:"reason,omitempty"`
}

// RestoreTrashInput is the request of facade restoreTrashItem.
type RestoreTrashInput struct {
	ResourceType string    `json:"resource_type"`
	ID           uuid.UUID `json:"resource_id"`
	Reason       string    `json:"reason"` // 3–1000
}

// RestoreTrashView is the result of a restore (stored for the Idempotency-Key replay).
type RestoreTrashView struct {
	ResourceType string    `json:"resource_type"`
	ResourceID   uuid.UUID `json:"resource_id"`
	RestoredAt   time.Time `json:"restored_at"`
}

// RestoreTrash is the use case behind facade restoreTrashItem of module game (ADR-0028, ADMIN_PORTAL §3.8, caller
// module identity): System Admin (platform.trash.restore) restores a soft-deleted console item; game resources are
// platform content, so a tenant user (tenant.trash.restore) never finds one (404). The restore runs under
// tenancy.Bypass(trash) (audit cross_tenant.accessed, mode write), records audit content.restored and writes
// <resource_type>.restored (platform event) in the same transaction; a live item holding the key is 409 CONFLICT.
type RestoreTrash struct {
	UoW    write.UnitOfWork
	Store  write.TrashStore
	Events write.EventPublisher
	Audit  write.Auditor
	Now    func() time.Time
}

// Restore runs the command.
func (h RestoreTrash) Restore(ctx context.Context, in RestoreTrashInput) (out RestoreTrashView, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.RestoreTrashItem")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	if p.TenantID != nil {
		if err := authz.Require(ctx, p, authz.PermTenantTrashRestore, authz.Resource{}); err != nil {
			return out, err
		}
		return out, problem.New(problem.NotFound, "") // platform content is not in a tenant trash
	}
	if err := authz.Require(ctx, p, authz.PermPlatformTrashRestore, authz.Resource{}); err != nil {
		return out, err
	}
	if !p.IsPlatform() {
		return out, problem.New(problem.PermissionDenied, "")
	}
	if !gameconfig.IsTrashType(in.ResourceType) {
		return out, problem.Invalid("resource_type", problem.SubInvalid, "is not a resource type of module game")
	}
	if n := utf8.RuneCountInString(strings.TrimSpace(in.Reason)); n < 3 || utf8.RuneCountInString(in.Reason) > 1000 {
		return out, problem.Invalid("reason", problem.SubInvalid, "must be 3–1000 characters")
	}
	bctx, err := tenancy.Bypass(tenancy.WithModule(ctx, "game"), tenancy.ReasonTrash, tenancy.WithMode(tenancy.ModeWrite))
	if err != nil {
		return out, err
	}
	now := time.Now().UTC()
	if h.Now != nil {
		now = h.Now().UTC()
	}
	err = h.UoW.Do(bctx, func(ctx context.Context) error {
		switch err := h.Store.RestoreConfig(ctx, in.ResourceType, in.ID, p.UserID, now); {
		case errors.Is(err, gameconfig.ErrNotInTrash):
			return problem.New(problem.NotFound, "the item is not in the trash")
		case errors.Is(err, gameconfig.ErrRestoreConflict):
			return problem.New(problem.Conflict, "a live item already uses the key of this one")
		case err != nil:
			return err
		}
		if err := h.Audit.Record(ctx, audit.Event{Action: ActionContentRestored, TargetType: in.ResourceType, TargetID: in.ID.String(),
			Result: audit.ResultSuccess, Reason: in.Reason, Metadata: map[string]any{"resource_type": in.ResourceType}}); err != nil {
			return err
		}
		by := p.UserID
		if err := h.Events.Publish(ctx, write.Event{Type: in.ResourceType + ".restored", SubjectType: in.ResourceType, SubjectID: in.ID.String(),
			PartitionKey: in.ID.String(), Platform: true, OccurredAt: now,
			Payload: RestoredPayload{ResourceType: in.ResourceType, ResourceID: in.ID, RestoredBy: &by, Reason: in.Reason}}); err != nil {
			return err
		}
		out = RestoreTrashView{ResourceType: in.ResourceType, ResourceID: in.ID, RestoredAt: now}
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}
