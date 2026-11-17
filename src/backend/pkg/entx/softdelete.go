package entx

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// ErrNotInTrash: Restore outside tenancy.Bypass(ctx, ReasonTrash).
var ErrNotInTrash = errors.New("entx: restore requires tenancy.Bypass(ctx, trash)")

// DeletedPayload is the payload of <entity>.deleted events written by SoftDelete.
type DeletedPayload struct {
	ID     string `json:"id"`
	Reason string `json:"reason,omitempty"`
}

// SoftDelete deletes one entity "the only way" (AGENTS R21): it runs del (the module's
// client.<Entity>.DeleteOneID(id).Exec, turned into an update of deleted_at by the mixin hook) with the
// reason, then records audit <entity>.deleted and writes event <entity>.deleted to the outbox — all in
// the current UnitOfWork transaction (F003-AC-04).
func SoftDelete(ctx context.Context, entity string, id uuid.UUID, reason string, del func(ctx context.Context) error) error {
	st, ok := stateFrom(ctx)
	if !ok || st.readOnly {
		return ErrNoTransaction
	}
	if err := del(WithDeleteReason(ctx, reason)); err != nil {
		return err
	}
	action := entity + ".deleted"
	if st.uow.o.Audit == nil || st.uow.o.Events == nil {
		return fmt.Errorf("entx: SoftDelete needs the unit of work's Audit and Events")
	}
	if err := st.uow.o.Audit.Record(ctx, audit.Event{Action: action, TargetType: entity, TargetID: id.String(),
		Result: audit.ResultSuccess, Reason: reason}); err != nil {
		return fmt.Errorf("entx: audit %s: %w", action, err)
	}
	if err := st.uow.o.Events.Write(ctx, action, id.String(), DeletedPayload{ID: id.String(), Reason: reason}); err != nil {
		return fmt.Errorf("entx: event %s: %w", action, err)
	}
	return nil
}

// Restore runs restore (the module clears deleted_at/deleted_by/delete_reason) only under
// tenancy.Bypass(ctx, ReasonTrash), records audit <entity>.restored and writes event <entity>.restored.
func Restore(ctx context.Context, entity string, id uuid.UUID, restore func(ctx context.Context) error) error {
	if r, ok := tenancy.Bypassed(ctx); !ok || r != tenancy.ReasonTrash {
		return ErrNotInTrash
	}
	st, ok := stateFrom(ctx)
	if !ok || st.readOnly {
		return ErrNoTransaction
	}
	if err := restore(ctx); err != nil {
		return err
	}
	action := entity + ".restored"
	if st.uow.o.Audit == nil || st.uow.o.Events == nil {
		return fmt.Errorf("entx: Restore needs the unit of work's Audit and Events")
	}
	if err := st.uow.o.Audit.Record(ctx, audit.Event{Action: action, TargetType: entity, TargetID: id.String(), Result: audit.ResultSuccess}); err != nil {
		return err
	}
	return st.uow.o.Events.Write(ctx, action, id.String(), DeletedPayload{ID: id.String()})
}
