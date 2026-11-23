package config

import (
	"errors"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/club"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// Defaults of the persistent channels and clubs (CONFIG_DEFAULTS: club_max_members / clubs_per_student_max — SOCIAL_SYSTEM
// §6, ADR-0050; club_invitation_ttl [Proposed F120]; persistent_chat_retention_days — CHAT_SYSTEM §2, OQ-09).
const (
	DefaultClubMaxMembers          = 50
	DefaultClubsPerStudentMax      = 5
	DefaultClubInvitationTTL       = 7 * 24 * time.Hour
	DefaultPersistentRetentionDays = 180
	DefaultRetentionInterval       = time.Hour
	DefaultRetentionBatch          = 500
)

// Persistent is the configuration of F120.
type Persistent struct {
	Clubs             club.Limits
	RetentionDays     int           // EW_CHAT_PERSISTENT_RETENTION_DAYS (fallback of chat_retention.persistent_days)
	RetentionInterval time.Duration // EW_CHAT_PERSISTENT_RETENTION_INTERVAL (job persistent-chat-retention)
	RetentionBatch    int           // EW_CHAT_PERSISTENT_RETENTION_BATCH (messages anonymised per transaction)
}

// PersistentFrom reads EW_CHAT_CLUB_MAX_MEMBERS (50), EW_CHAT_CLUBS_PER_STUDENT_MAX (5), EW_CHAT_CLUB_INVITATION_TTL (7 days),
// EW_CHAT_PERSISTENT_RETENTION_DAYS (180), EW_CHAT_PERSISTENT_RETENTION_INTERVAL (1 h), EW_CHAT_PERSISTENT_RETENTION_BATCH (500).
func PersistentFrom(c *config.Config) (Persistent, error) {
	out := Persistent{
		Clubs: club.Limits{MaxMembers: c.Int("EW_CHAT_CLUB_MAX_MEMBERS", DefaultClubMaxMembers),
			PerStudent: c.Int("EW_CHAT_CLUBS_PER_STUDENT_MAX", DefaultClubsPerStudentMax),
			InviteTTL:  c.Duration("EW_CHAT_CLUB_INVITATION_TTL", DefaultClubInvitationTTL)},
		RetentionDays:     c.Int("EW_CHAT_PERSISTENT_RETENTION_DAYS", DefaultPersistentRetentionDays),
		RetentionInterval: c.Duration("EW_CHAT_PERSISTENT_RETENTION_INTERVAL", DefaultRetentionInterval),
		RetentionBatch:    c.Int("EW_CHAT_PERSISTENT_RETENTION_BATCH", DefaultRetentionBatch),
	}
	switch {
	case out.Clubs.MaxMembers < 2 || out.Clubs.MaxMembers > 200:
		return out, errors.New("chat config: EW_CHAT_CLUB_MAX_MEMBERS must be 2..200")
	case out.Clubs.PerStudent < 1:
		return out, errors.New("chat config: EW_CHAT_CLUBS_PER_STUDENT_MAX must be ≥ 1")
	case out.Clubs.InviteTTL < time.Hour:
		return out, errors.New("chat config: EW_CHAT_CLUB_INVITATION_TTL must be ≥ 1h")
	case out.RetentionDays < 1 || out.RetentionDays > 3650:
		return out, errors.New("chat config: EW_CHAT_PERSISTENT_RETENTION_DAYS must be 1..3650")
	case out.RetentionInterval < time.Minute || out.RetentionBatch < 1 || out.RetentionBatch > 5000:
		return out, errors.New("chat config: EW_CHAT_PERSISTENT_RETENTION_INTERVAL ≥ 1m and _BATCH 1..5000")
	}
	return out, nil
}
