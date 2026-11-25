package command

import (
	"context"

	"github.com/google/uuid"
)

// BossOutcome is the part of boss.encounter_completed the quest engine reads.
type BossOutcome struct {
	TenantID uuid.UUID
	BossID   uuid.UUID
	Defeated bool
	Eligible []uuid.UUID // members who reached min_contribution
}

// ApplyBossDefeated is the consumer step of boss.encounter_completed for the quests (F103-AC-18, QUEST_SYSTEM §3): a
// defeated boss completes the active boss steps of that boss for the eligible members only — a member below
// min_contribution keeps its step active. Runs in the consumer transaction; a replay finds the steps completed.
func (h Quests) ApplyBossDefeated(ctx context.Context, b BossOutcome) error {
	if !b.Defeated {
		return nil
	}
	at := h.now()
	for _, student := range b.Eligible {
		if err := h.Store.LockStudent(ctx, student); err != nil {
			return err
		}
		ps, err := h.Store.ActiveOfStudent(ctx, b.TenantID, student)
		if err != nil {
			return err
		}
		for _, p := range ps {
			v, err := h.version(ctx, p)
			if err != nil {
				return err
			}
			outs := p.BossDefeated(v, b.BossID.String(), at)
			if len(outs) == 0 {
				continue
			}
			push, err := h.record(ctx, p, v, outs, nil, at)
			if err != nil {
				return err
			}
			st := p.StudentID
			h.after(ctx, func(ctx context.Context) { h.push(ctx, st, push, at) })
		}
	}
	return nil
}
