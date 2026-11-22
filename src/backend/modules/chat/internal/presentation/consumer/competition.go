package consumer

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// Gameplay is the consumer chat.gameplay_channels (F117 T003, T017): party.* and match.* of module competition open, fill
// and close the always-on gameplay channels. Module chat never calls module competition (R29): the event carries
// everything (ids, participants with tenant/school/team).
type Gameplay struct{ H command.Gameplay }

// GameplayOptions returns its options; party.created is schema_version 2 since ADR-0095 (v1 still accepted).
func GameplayOptions(h Gameplay, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "chat", Name: messaging.GameplayChannelsName, Stream: messaging.CompetitionStream,
		Subjects: messaging.Subjects("competition", messaging.GameplayChannelsEvents), MaxSchema: 2, UoW: uow, DLQDB: dlq, JS: js,
		Handler: h.Handle}
}

type participant struct {
	StudentID uuid.UUID `json:"student_id"`
	TenantID  uuid.UUID `json:"tenant_id"`
	SchoolID  uuid.UUID `json:"school_id"`
	TeamNo    *int      `json:"team_no"`
}

func participants(e outbox.Envelope, ps []participant) ([]command.Participant, error) {
	if len(ps) == 0 || len(ps) > 40 {
		return nil, invalid(e.EventType, "participants: 1..40")
	}
	out := make([]command.Participant, 0, len(ps))
	for _, p := range ps {
		if p.StudentID == uuid.Nil || p.TenantID == uuid.Nil || p.TeamNo == nil || *p.TeamNo < 0 || *p.TeamNo > 1 {
			return nil, invalid(e.EventType, "participant student_id/tenant_id/team_no")
		}
		out = append(out, command.Participant{StudentID: p.StudentID, TenantID: p.TenantID, SchoolID: p.SchoolID, TeamNo: *p.TeamNo})
	}
	return out, nil
}

func at(t *time.Time, e outbox.Envelope) time.Time {
	if t != nil && !t.IsZero() {
		return t.UTC()
	}
	return e.OccurredAt.UTC()
}

// Handle implements outbox.Handler.
func (c Gameplay) Handle(ctx context.Context, e outbox.Envelope) error {
	switch e.EventType {
	case "party.created", "party.member_joined", "party.member_left", "party.member_kicked", "party.disbanded", "party.leader_transferred":
		change, err := c.party(e)
		if err != nil {
			return outbox.Permanent(err)
		}
		return classify("", e, true, c.H.ApplyParty(ctx, change))
	case "match.started", "match.completed", "match.aborted":
		change, err := c.match(e)
		if err != nil {
			return outbox.Permanent(err)
		}
		return classify("", e, true, c.H.ApplyMatch(ctx, change))
	}
	return nil
}

func (c Gameplay) party(e outbox.Envelope) (command.PartyChange, error) {
	var p struct {
		PartyID      uuid.UUID  `json:"party_id"`
		TenantID     uuid.UUID  `json:"tenant_id"`
		SchoolID     *uuid.UUID `json:"school_id"`
		LeaderID     uuid.UUID  `json:"leader_id"`
		StudentID    uuid.UUID  `json:"student_id"`
		From         uuid.UUID  `json:"from"`
		To           uuid.UUID  `json:"to"`
		StateVersion *int64     `json:"state_version"`
		CreatedAt    *time.Time `json:"created_at"`
		JoinedAt     *time.Time `json:"joined_at"`
		DisbandedAt  *time.Time `json:"disbanded_at"`
	}
	if err := decode(e, &p); err != nil {
		return command.PartyChange{}, err
	}
	tenant, err := envTenant(e)
	switch {
	case err != nil:
		return command.PartyChange{}, err
	case p.PartyID == uuid.Nil || p.TenantID != tenant || p.StateVersion == nil || *p.StateVersion < 0:
		return command.PartyChange{}, invalid(e.EventType, "party_id/tenant_id/state_version")
	}
	ch := command.PartyChange{TenantID: tenant, PartyID: p.PartyID, SchoolID: p.SchoolID, Version: *p.StateVersion}
	switch e.EventType {
	case "party.created":
		if p.LeaderID == uuid.Nil {
			return ch, invalid(e.EventType, "leader_id")
		}
		ch.StudentID, ch.Join, ch.At = p.LeaderID, true, at(p.CreatedAt, e)
		ch.Leader = true
	case "party.leader_transferred":
		if p.To == uuid.Nil || p.From == uuid.Nil {
			return ch, invalid(e.EventType, "from/to")
		}
		ch.OldLeader, ch.NewLeader, ch.At = p.From, p.To, e.OccurredAt.UTC()
		return ch, nil
	case "party.member_joined":
		ch.StudentID, ch.Join, ch.At = p.StudentID, true, at(p.JoinedAt, e)
	case "party.member_left", "party.member_kicked":
		ch.StudentID, ch.At = p.StudentID, e.OccurredAt.UTC()
	case "party.disbanded":
		ch.Disband, ch.At = true, at(p.DisbandedAt, e)
	}
	if !ch.Disband && ch.StudentID == uuid.Nil {
		return ch, invalid(e.EventType, "student_id")
	}
	return ch, nil
}

func (c Gameplay) match(e outbox.Envelope) (command.MatchChange, error) {
	var p struct {
		MatchID      uuid.UUID     `json:"match_id"`
		Participants []participant `json:"participants"`
		Results      []participant `json:"results"`
		StartedAt    *time.Time    `json:"started_at"`
		EndedAt      *time.Time    `json:"ended_at"`
		AbortedAt    *time.Time    `json:"aborted_at"`
	}
	if err := decode(e, &p); err != nil {
		return command.MatchChange{}, err
	}
	tenant, err := envTenant(e)
	if err != nil {
		return command.MatchChange{}, err
	}
	if p.MatchID == uuid.Nil {
		return command.MatchChange{}, invalid(e.EventType, "match_id")
	}
	list := p.Participants
	when := at(p.StartedAt, e)
	ended := e.EventType != "match.started"
	switch e.EventType {
	case "match.completed":
		list, when = p.Results, at(p.EndedAt, e)
	case "match.aborted":
		when = at(p.AbortedAt, e)
	}
	ps, err := participants(e, list)
	if err != nil {
		return command.MatchChange{}, err
	}
	return command.MatchChange{TenantID: tenant, MatchID: p.MatchID, Participants: ps, Ended: ended, At: when}, nil
}
