// Package oncall holds the rules of the on-call rotation of the Platform Safety Team for p1 cases (MODERATION §6.3,
// CHILD_SAFETY §11, F124 US3): shifts (primary/secondary, no overlap of one role) and the paging ladder primary →
// secondary → lead → system_admin. Standard library and uuid only.
package oncall

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
)

// Roles of a shift.
const (
	RolePrimary   = "primary"
	RoleSecondary = "secondary"
)

// Levels of the ladder (moderation.oncall_paged.level, chat.moderation_cases.oncall_page_level).
const (
	LevelPrimary     = "primary"
	LevelSecondary   = "secondary"
	LevelLead        = "lead"
	LevelSystemAdmin = "system_admin"
)

// Levels in paging order.
var Levels = []string{LevelPrimary, LevelSecondary, LevelLead, LevelSystemAdmin}

// Roles of a shift.
var Roles = []string{RolePrimary, RoleSecondary}

// ErrInvalid is a shift outside the rules (unknown role, end not after start, longer than oncall_shift_max_hours).
var ErrInvalid = errors.New("oncall: invalid shift")

// Shift is one on-call shift of a platform staff member: [Start, End).
type Shift struct {
	ID        uuid.UUID
	Moderator uuid.UUID
	Role      string
	Start     time.Time
	End       time.Time
}

// Validate checks the role and the window (max = oncall_shift_max_hours).
func (s Shift) Validate(max time.Duration) error {
	switch {
	case !slices.Contains(Roles, s.Role):
		return fmt.Errorf("%w: oncall_role", ErrInvalid)
	case !s.End.After(s.Start):
		return fmt.Errorf("%w: ends_at must be after starts_at", ErrInvalid)
	case max > 0 && s.End.Sub(s.Start) > max:
		return fmt.Errorf("%w: longer than %s", ErrInvalid, max)
	}
	return nil
}

// Overlaps reports two shifts of the same role sharing an instant (adjacent shifts do not overlap).
func (s Shift) Overlaps(o Shift) bool {
	return s.Role == o.Role && s.Start.Before(o.End) && o.Start.Before(s.End)
}

// Covers reports whether t is inside [Start, End).
func (s Shift) Covers(t time.Time) bool { return !t.Before(s.Start) && t.Before(s.End) }

// Ladder is the paging schedule of an unclaimed p1 case: Unclaimed = p1_unclaimed_alert_minutes (secondary at 1×, lead
// at 2×), FirstResponse = the first response of report_sla for p1 (system_admin).
type Ladder struct {
	Unclaimed     time.Duration
	FirstResponse time.Duration
}

// due returns the instant level i becomes due for a case created at.
func (l Ladder) due(i int, created time.Time) time.Time {
	switch Levels[i] {
	case LevelSecondary:
		return created.Add(l.Unclaimed)
	case LevelLead:
		return created.Add(2 * l.Unclaimed)
	case LevelSystemAdmin:
		return created.Add(l.FirstResponse)
	}
	return created
}

// Next returns the level to page now for a case created at whose last paged level is last ("" = none): the highest level
// above last that is due; when it has nobody (has reports false: no current shift of that role), the next level that has
// people (F124-AC-15). Every level is paged at most once and never below last (F124-AC-14).
func (l Ladder) Next(last string, created, now time.Time, has func(level string) bool) (string, bool) {
	start := slices.Index(Levels, last) + 1
	due := -1
	for i := start; i < len(Levels); i++ {
		if !now.Before(l.due(i, created)) {
			due = i
		}
	}
	if due < 0 {
		return "", false
	}
	for i := due; i < len(Levels); i++ {
		if has(Levels[i]) {
			return Levels[i], true
		}
	}
	return "", false
}

// RoleOf returns the shift role paged by level ("" for lead and system_admin, resolved by module identity).
func RoleOf(level string) string {
	switch level {
	case LevelPrimary:
		return RolePrimary
	case LevelSecondary:
		return RoleSecondary
	}
	return ""
}
