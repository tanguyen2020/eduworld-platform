package port

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/trash"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
)

// ResourceHandler is what one resource type of module learning plugs into the shared foundation of F020:
// trash, restore, assignment and clone (ADR-0028, ADR-0029). F021 (curriculum), F022 (question) and F024
// (question set, lesson, blueprint) register theirs; F020 itself ships none (tests use a fixture handler).
// Every method runs inside the transaction and with the scope of ctx chosen by the use case.
type ResourceHandler interface {
	// ResourceType is the type handled.
	ResourceType() contentmodel.ResourceType
	// WritePermission is the permission a tenant user needs to delete items of this type.
	WritePermission() authz.Permission
	// Owners returns the owner of each id visible to the scope of ctx; invisible or deleted ids are absent.
	Owners(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]contentmodel.Owner, error)
	// SoftDelete soft-deletes one live item visible to the scope (deleted_at/by/reason through the entx hook)
	// and returns its owner and title: problem NotFound when invisible, contentmodel.ErrItemPublished when
	// the item has a current release.
	SoftDelete(ctx context.Context, id uuid.UUID, reason string) (Deleted, error)
	// ListDeleted lists soft-deleted items (ctx carries tenancy.Bypass(trash)), newest first, with the
	// unique-key conflict of each entry.
	ListDeleted(ctx context.Context, f TrashFilter) ([]trash.Entry, error)
	// Restore clears deleted_* of one item (ctx carries tenancy.Bypass(trash)): trash.ErrNotDeleted when the
	// item is not in the trash, trash.ErrConflict when a live item holds its unique key.
	Restore(ctx context.Context, id uuid.UUID) (trash.Entry, error)
	// Clone deep-copies one item of the source tenant into the target tenant (ctx carries
	// tenancy.Bypass(tenant_clone)): new ids, owner tenant = target, latest version as a new draft,
	// cloned_from_* set; references to platform data are kept; student data is never copied (R22).
	// ErrNotInSourceTenant when the item does not belong to the source tenant.
	Clone(ctx context.Context, in CloneInput) (uuid.UUID, error)
}

// ErrNotInSourceTenant: a clone source that is not a live item of the source tenant (F020-AC-29).
var ErrNotInSourceTenant = errors.New("port: resource does not belong to the source tenant")

// Deleted describes a soft-deleted item.
type Deleted struct {
	Owner contentmodel.Owner
	Title string
}

// TrashFilter selects deleted items.
type TrashFilter struct {
	TenantIDs   []uuid.UUID // empty = every tenant and platform items
	PlatformToo bool        // include platform items (tenant_id NULL) when TenantIDs is not empty
	AfterAt     *time.Time  // keyset: deleted_at of the last row of the previous page
	AfterID     uuid.UUID
	Limit       int
}

// CloneInput is one element of a clone step.
type CloneInput struct {
	JobID        uuid.UUID
	SourceID     uuid.UUID
	SourceTenant uuid.UUID
	TargetTenant uuid.UUID
	Actor        uuid.UUID
	At           time.Time
	IDMap        map[uuid.UUID]uuid.UUID // ids cloned by earlier steps (cross-module references)
}

// Registry holds the handlers by resource type. Modules register at composition time; lookups are safe for
// concurrent use.
type Registry struct {
	mu       sync.RWMutex
	handlers map[contentmodel.ResourceType]ResourceHandler
}

// NewRegistry registers handlers; a type registered twice is an error.
func NewRegistry(hs ...ResourceHandler) (*Registry, error) {
	r := &Registry{handlers: map[contentmodel.ResourceType]ResourceHandler{}}
	for _, h := range hs {
		if err := r.Register(h); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// Register adds h.
func (r *Registry) Register(h ResourceHandler) error {
	if h == nil {
		return errors.New("port: nil resource handler")
	}
	rt := h.ResourceType()
	if _, err := contentmodel.ParseResourceType(string(rt)); err != nil {
		return fmt.Errorf("port: register %q: %w", rt, err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.handlers == nil {
		r.handlers = map[contentmodel.ResourceType]ResourceHandler{}
	}
	if _, dup := r.handlers[rt]; dup {
		return fmt.Errorf("port: resource type %q registered twice", rt)
	}
	r.handlers[rt] = h
	return nil
}

// Get returns the handler of rt.
func (r *Registry) Get(rt contentmodel.ResourceType) (ResourceHandler, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.handlers[rt]
	return h, ok
}

// Types returns the registered types, sorted.
func (r *Registry) Types() []contentmodel.ResourceType {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]contentmodel.ResourceType, 0, len(r.handlers))
	for t := range r.handlers {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
