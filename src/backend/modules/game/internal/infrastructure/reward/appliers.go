// Package reward holds the grant appliers of the Reward Engine (F074 T010): one applier per grant type, registered in a
// write.GrantApplierRegistry. F074 registers xp and title; F075 adds currency, item, badge and collection_item. Content
// naming a grant type without an applier is refused at publish (F074-AC-23).
package reward

import (
	"context"
	"fmt"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
)

// Applier applies one grant of its type for a student inside the grant transaction.
type Applier interface {
	Type() string
	Apply(ctx context.Context, gc write.GrantContext, grant map[string]any) error
}

// Registry implements write.GrantApplierRegistry.
type Registry struct{ appliers map[string]Applier }

var _ write.GrantApplierRegistry = (*Registry)(nil)

// NewRegistry returns a registry of appliers (the F074 default: XP and Title).
func NewRegistry(as ...Applier) *Registry {
	r := &Registry{appliers: map[string]Applier{}}
	for _, a := range as {
		r.appliers[a.Type()] = a
	}
	return r
}

// Default is the registry of F074 (xp, title).
func Default() *Registry { return NewRegistry(XP{}, Title{}) }

// Has implements write.GrantApplierRegistry.
func (r *Registry) Has(grantType string) bool {
	_, ok := r.appliers[grantType]
	return ok
}

// Apply implements write.GrantApplierRegistry: every grant through the applier of its type; an unknown type fails the
// transaction (never silently skipped).
func (r *Registry) Apply(ctx context.Context, gc write.GrantContext, grants []map[string]any) error {
	for _, g := range grants {
		t, _ := g["type"].(string)
		a, ok := r.appliers[t]
		if !ok {
			return fmt.Errorf("reward: no grant applier for %q", t)
		}
		if err := a.Apply(ctx, gc, g); err != nil {
			return err
		}
	}
	return nil
}

// XP is the applier of xp grants: the Reward Engine posts the XP of a rule in xp_ledger itself (with its caps), so the
// grant record is the only state (no second posting).
type XP struct{}

// Type implements Applier.
func (XP) Type() string { return "xp" }

// Apply implements Applier.
func (XP) Apply(context.Context, write.GrantContext, map[string]any) error { return nil }

// Title is the applier of title grants: a title is shown from the level ranges of the curve; a granted title is kept in
// gamification.reward_grants (the grant row is its record, no other table in Release 1.0 — PROGRESSION_SYSTEM §4.3).
type Title struct{}

// Type implements Applier.
func (Title) Type() string { return "title" }

// Apply implements Applier: the title code must be present.
func (Title) Apply(_ context.Context, _ write.GrantContext, g map[string]any) error {
	if c, _ := g["title_code"].(string); c == "" {
		return fmt.Errorf("reward: title grant without title_code")
	}
	return nil
}
