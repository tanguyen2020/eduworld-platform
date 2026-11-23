package command

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// AssignSeedQuests assigns platform quests to tenants by tenant code (F072 T002, dev seed only, ADR-0029): an active
// assignment row of infra.tenant_resource_assignments per (quest, tenant), once, with the audit record
// tenant_data.assigned; a tenant code not projected yet into ref.tenants is skipped (the seed runs again after
// make seed-check). Platform scope only. The assignment saga of System Admin (F045) uses the facade operation later.
type AssignSeedQuests struct {
	UoW         write.UnitOfWork
	Assignments write.AssignmentStore
	Audit       write.Auditor
	// ItemType is the resource type assigned ("" = quest; F075 assigns shop_item with the same flow).
	ItemType string
	Now      func() time.Time
}

// SeedAssignment is one quest and the tenant codes it goes to.
type SeedAssignment struct {
	QuestID uuid.UUID
	Tenants []string
}

// Assign returns the rows created and the tenant codes skipped.
func (h AssignSeedQuests) Assign(ctx context.Context, items []SeedAssignment) (created int, skipped []string, err error) {
	if s, ok := tenancy.ScopeFrom(ctx); !ok || !s.PlatformOnly() {
		return 0, nil, problem.New(problem.PermissionDenied, "quests are assigned under a platform scope")
	}
	at := time.Now().UTC()
	if h.Now != nil {
		at = h.Now().UTC()
	}
	by := uuid.Nil
	if p, ok := authz.PrincipalFrom(ctx); ok {
		by = p.UserID
	}
	itemType := h.ItemType
	if itemType == "" {
		itemType = write.ItemQuest
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		created, skipped = 0, nil
		for _, it := range items {
			for _, code := range it.Tenants {
				tenant, err := h.Assignments.TenantByCode(ctx, code)
				if err != nil {
					return err
				}
				if tenant == nil {
					skipped = append(skipped, code)
					continue
				}
				ok, err := h.Assignments.Assign(ctx, itemType, it.QuestID, *tenant, by, at)
				if err != nil || !ok {
					if err != nil {
						return err
					}
					continue
				}
				created++
				if err := h.Audit.Record(ctx, audit.Event{Action: "tenant_data.assigned", TargetType: itemType, TargetID: it.QuestID.String(),
					Result: audit.ResultSuccess, Metadata: map[string]any{"tenant_id": tenant.String(), "source": "seed"}}); err != nil {
					return err
				}
			}
		}
		return nil
	})
	return created, skipped, err
}
