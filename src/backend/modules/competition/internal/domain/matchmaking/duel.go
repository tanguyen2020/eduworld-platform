package matchmaking

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// DuelTTL is the life of a pending friend duel (MATCHMAKING §8: 60 s, rule of the owner document).
const DuelTTL = 60 * time.Second

// Duel statuses (competition.duels.status).
const (
	DuelPending   = "pending"
	DuelAccepted  = "accepted"
	DuelDeclined  = "declined"
	DuelExpired   = "expired"
	DuelCancelled = "cancelled"
)

// Errors of the duel.
var (
	ErrDuelGone        = errors.New("matchmaking: duel expired or closed")
	ErrNotDuelOpponent = errors.New("matchmaking: not the opponent of the duel")
)

// Duel is a friend challenge (aggregate of competition.duels): unranked, pending for DuelTTL.
type Duel struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	SchoolID     uuid.UUID
	ChallengerID uuid.UUID
	OpponentID   uuid.UUID
	SubjectID    uuid.UUID
	TopicID      *uuid.UUID
	Status       string
	MatchID      *uuid.UUID
	ExpiresAt    time.Time
	RespondedAt  *time.Time
	CreatedAt    time.Time
}

// NewDuel opens a pending duel at now.
func NewDuel(id, tenant, school, challenger, opponent, subject uuid.UUID, topic *uuid.UUID, now time.Time) Duel {
	return Duel{ID: id, TenantID: tenant, SchoolID: school, ChallengerID: challenger, OpponentID: opponent, SubjectID: subject,
		TopicID: topic, Status: DuelPending, ExpiresAt: now.Add(DuelTTL), CreatedAt: now}
}

// Open reports a pending duel that has not expired at now.
func (d Duel) Open(now time.Time) bool { return d.Status == DuelPending && now.Before(d.ExpiresAt) }

// Accept records the answer of the opponent with the match it starts.
func (d *Duel) Accept(by, match uuid.UUID, now time.Time) error {
	if by != d.OpponentID {
		return ErrNotDuelOpponent
	}
	if !d.Open(now) {
		return ErrDuelGone
	}
	d.Status, d.MatchID, d.RespondedAt = DuelAccepted, &match, &now
	return nil
}

// Decline records a refusal of the opponent.
func (d *Duel) Decline(by uuid.UUID, now time.Time) error {
	if by != d.OpponentID {
		return ErrNotDuelOpponent
	}
	if !d.Open(now) {
		return ErrDuelGone
	}
	d.Status, d.RespondedAt = DuelDeclined, &now
	return nil
}

// Expire closes a pending duel past its expiry; false when nothing changes.
func (d *Duel) Expire(now time.Time) bool {
	if d.Status != DuelPending || now.Before(d.ExpiresAt) {
		return false
	}
	d.Status, d.RespondedAt = DuelExpired, &now
	return true
}

// BandOf is the grade band of a grade (g1_3, g4_5, g6_7, g8_9; "" when unknown): a duel needs the same band (H1 relaxed
// to the band for friend duels, MATCHMAKING §3).
func BandOf(grade int) string {
	switch {
	case grade >= 1 && grade <= 3:
		return "g1_3"
	case grade >= 4 && grade <= 5:
		return "g4_5"
	case grade >= 6 && grade <= 7:
		return "g6_7"
	case grade >= 8 && grade <= 9:
		return "g8_9"
	}
	return ""
}
