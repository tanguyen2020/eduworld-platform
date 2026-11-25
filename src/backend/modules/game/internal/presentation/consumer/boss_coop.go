package consumer

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/boss"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/partyquest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// BossMembership is the boss side of a party membership change (command.Bosses).
type BossMembership interface {
	MemberLeft(ctx context.Context, tenant, party, student uuid.UUID, reason string) error
}

// QuestMembership is the co-op quest side of a party membership change (command.PartyQuests).
type QuestMembership interface {
	MemberLeft(ctx context.Context, tenant, party, student uuid.UUID, reason string) error
	Disbanded(ctx context.Context, tenant, party uuid.UUID) error
}

// PartyMembership is the consumer game.boss_party_membership (F103-AC-12, AC-19): party.member_left, party.member_kicked
// (the member leaves the live encounters and the shared quest sessions of the party — contributions, progress and
// learning records stay) and party.disbanded (every member leaves the encounters; failure_conditions party_disbanded
// applies to the quest sessions). Runs in the consumer transaction (TenantScope of the envelope).
type PartyMembership struct {
	Bosses BossMembership
	Quests QuestMembership
}

// PartyMembershipOptions returns the outbox consumer options (tenant events of the competition stream).
func PartyMembershipOptions(h PartyMembership, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "game", Name: messaging.BossPartyMembershipName, Stream: messaging.CompetitionStream,
		Subjects: messaging.CompetitionSubjects(messaging.BossPartyMembershipEvents), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// Handle implements outbox.Handler.
func (c PartyMembership) Handle(ctx context.Context, e outbox.Envelope) error {
	if e.TenantID == nil {
		return reject(messaging.BossPartyMembershipName, invalid(e.EventType, "tenant event expected"))
	}
	var p struct {
		PartyID   uuid.UUID `json:"party_id"`
		StudentID uuid.UUID `json:"student_id"`
	}
	if err := decode(e, &p); err != nil {
		return reject(messaging.BossPartyMembershipName, err)
	}
	if p.PartyID == uuid.Nil {
		return reject(messaging.BossPartyMembershipName, invalid(e.EventType, "party_id is required"))
	}
	tenant := *e.TenantID
	switch e.EventType {
	case "party.disbanded":
		if err := c.Bosses.MemberLeft(ctx, tenant, p.PartyID, uuid.Nil, boss.LeftDisbanded); err != nil {
			return err
		}
		return c.Quests.Disbanded(ctx, tenant, p.PartyID)
	case "party.member_left", "party.member_kicked":
		if p.StudentID == uuid.Nil {
			return reject(messaging.BossPartyMembershipName, invalid(e.EventType, "student_id is required"))
		}
		br, qr := boss.LeftParty, partyquest.LeftParty
		if e.EventType == "party.member_kicked" {
			br, qr = boss.LeftKicked, partyquest.LeftKicked
		}
		if err := c.Bosses.MemberLeft(ctx, tenant, p.PartyID, p.StudentID, br); err != nil {
			return err
		}
		return c.Quests.MemberLeft(ctx, tenant, p.PartyID, p.StudentID, qr)
	}
	return nil
}

// PartyQuestSteps is the co-op quest use case behind the quest events (command.PartyQuests).
type PartyQuestSteps interface {
	StepCompleted(ctx context.Context, tenant, progressID uuid.UUID, stepID string) error
	ActivityGraded(ctx context.Context, g command.Graded) error
}

// PartyQuestOnStep is the consumer game.party_quest_on_step_completed (F103-AC-14, AC-17, AC-20): quest.step_completed of
// a member of a party session shares the frame step with the party, assigns the roles of the interdependent steps that
// became active and closes the session when every member completed.
type PartyQuestOnStep struct{ Quests PartyQuestSteps }

// PartyQuestOnStepOptions returns the outbox consumer options (tenant events of the game stream).
func PartyQuestOnStepOptions(h PartyQuestOnStep, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "game", Name: messaging.PartyQuestStepName, Stream: messaging.GameStream,
		Subjects: messaging.GameSubjects(messaging.PartyQuestStepEvents), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// Handle implements outbox.Handler.
func (c PartyQuestOnStep) Handle(ctx context.Context, e outbox.Envelope) error {
	if e.TenantID == nil {
		return reject(messaging.PartyQuestStepName, invalid(e.EventType, "tenant event expected"))
	}
	var p struct {
		ProgressID uuid.UUID `json:"quest_progress_id"`
		StepID     string    `json:"step_id"`
	}
	if err := decode(e, &p); err != nil {
		return reject(messaging.PartyQuestStepName, err)
	}
	if p.ProgressID == uuid.Nil || p.StepID == "" || len(p.StepID) > 64 {
		return reject(messaging.PartyQuestStepName, invalid(e.EventType, "quest_progress_id and step_id are required"))
	}
	return c.Quests.StepCompleted(ctx, *e.TenantID, p.ProgressID, p.StepID)
}

// PartyQuestOnGraded is the consumer game.party_quest_on_activity_graded (F103-AC-15, AC-16, AC-23): activity.graded of a
// quest step attempt (mode quest, the student's own attempt) feeds the collective target or the interdependent step of
// the party session.
type PartyQuestOnGraded struct{ Quests PartyQuestSteps }

// PartyQuestOnGradedOptions returns the outbox consumer options (tenant events of the learning stream).
func PartyQuestOnGradedOptions(h PartyQuestOnGraded, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "game", Name: messaging.PartyQuestGradedName, Stream: messaging.LearningStream,
		Subjects: messaging.LearningSubjects(messaging.PartyQuestGradedEvents), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// Handle implements outbox.Handler.
func (c PartyQuestOnGraded) Handle(ctx context.Context, e outbox.Envelope) error {
	if e.TenantID == nil {
		return reject(messaging.PartyQuestGradedName, invalid(e.EventType, "tenant event expected"))
	}
	var p struct {
		AttemptID    uuid.UUID `json:"attempt_id"`
		StudentID    uuid.UUID `json:"student_id"`
		Mode         string    `json:"mode"`
		CorrectCount *int      `json:"correct_count"`
		ItemCount    *int      `json:"item_count"`
		Passed       *bool     `json:"passed"`
	}
	if err := decode(e, &p); err != nil {
		return reject(messaging.PartyQuestGradedName, err)
	}
	if p.AttemptID == uuid.Nil || p.StudentID == uuid.Nil {
		return reject(messaging.PartyQuestGradedName, invalid(e.EventType, "attempt_id and student_id are required"))
	}
	if p.Mode != "quest" {
		return nil
	}
	return c.Quests.ActivityGraded(ctx, command.Graded{TenantID: *e.TenantID, AttemptID: p.AttemptID, StudentID: p.StudentID,
		CorrectCount: p.CorrectCount, ItemCount: p.ItemCount, Passed: p.Passed, OccurredAt: e.OccurredAt.UTC()})
}

// BossSteps completes the boss steps of the quests (command.Quests).
type BossSteps interface {
	ApplyBossDefeated(ctx context.Context, b command.BossOutcome) error
}

// QuestOnBoss is the consumer game.quest_on_boss_completed (F103-AC-18): boss.encounter_completed with result defeated
// completes the active boss steps of that boss for the eligible members only.
type QuestOnBoss struct{ Quests BossSteps }

// QuestOnBossOptions returns the outbox consumer options (tenant events of the game stream).
func QuestOnBossOptions(h QuestOnBoss, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "game", Name: messaging.QuestBossName, Stream: messaging.GameStream,
		Subjects: messaging.GameSubjects(messaging.QuestBossEvents), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// Handle implements outbox.Handler.
func (c QuestOnBoss) Handle(ctx context.Context, e outbox.Envelope) error {
	if e.TenantID == nil {
		return reject(messaging.QuestBossName, invalid(e.EventType, "tenant event expected"))
	}
	p, err := bossCompleted(e)
	if err != nil {
		return reject(messaging.QuestBossName, err)
	}
	out := command.BossOutcome{TenantID: *e.TenantID, BossID: p.BossID, Defeated: p.Result == "defeated"}
	for _, m := range p.Contributions {
		if m.Eligible {
			out.Eligible = append(out.Eligible, m.StudentID)
		}
	}
	return c.Quests.ApplyBossDefeated(ctx, out)
}

// bossCompletedPayload is the part of boss.encounter_completed the consumers of module game read.
type bossCompletedPayload struct {
	EncounterID   uuid.UUID        `json:"encounter_id"`
	BossID        uuid.UUID        `json:"boss_id"`
	BossVersionID uuid.UUID        `json:"boss_version_id"`
	SubjectID     *uuid.UUID       `json:"subject_id"`
	Result        string           `json:"result"`
	Contributions []bossMemberPart `json:"contributions"`
	DefeatRewards []map[string]any `json:"defeat_rewards"`
	FirstRewards  []map[string]any `json:"first_defeat_rewards"`
}

type bossMemberPart struct {
	StudentID   uuid.UUID `json:"student_id"`
	Eligible    bool      `json:"eligible"`
	FirstDefeat bool      `json:"first_defeat"`
}

func bossCompleted(e outbox.Envelope) (bossCompletedPayload, error) {
	var p bossCompletedPayload
	if err := decode(e, &p); err != nil {
		return p, err
	}
	if p.EncounterID == uuid.Nil || p.BossID == uuid.Nil || len(p.Contributions) > 8 || len(p.Result) > 64 {
		return p, invalid(e.EventType, "encounter_id, boss_id and at most 8 contributions are required")
	}
	return p, nil
}
