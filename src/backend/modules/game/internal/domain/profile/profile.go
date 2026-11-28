package profile

import (
	"time"

	"github.com/google/uuid"
)

// Nickname change reasons (game.nickname_changes.reason).
const (
	ReasonInitial      = "initial"
	ReasonPlayerChange = "player_change"
	ReasonReset        = "reset"
)

// Profile is the GameProfile aggregate (game.game_profiles): what a student shows in the game. It never holds a
// real name, username, school name or class (R16, ADR-0054).
type Profile struct {
	ID                uuid.UUID // game_profile_id (assigned by the repository when zero)
	StudentID         uuid.UUID
	TenantID          uuid.UUID
	SchoolID          uuid.UUID
	Nickname          *string
	Level             int
	XPTotal           int64
	Avatar            *AvatarConfig
	AvatarHash        *string
	AvatarCreatedAt   *time.Time
	LastAreaID        *string
	Settings          map[string]any
	PreferredLanguage *string
	LastGameLoginAt   *time.Time
	Version           int64
	Deleted           bool
}

// New is the profile EnsureGameProfile creates on the first game session: no avatar, no nickname (setup required).
func New(tenant, student, school uuid.UUID, now time.Time) *Profile {
	t := now.UTC()
	return &Profile{StudentID: student, TenantID: tenant, SchoolID: school, Level: 1, Settings: map[string]any{},
		LastGameLoginAt: &t, Version: 1}
}

// NeedsSetup reports a profile without avatar or nickname: the client opens S02 before the Main Hub (F070-AC-01).
func (p *Profile) NeedsSetup() bool {
	return p.Avatar == nil || p.AvatarCreatedAt == nil || p.Nickname == nil
}

// AvatarChange is the outcome of SetAvatar.
type AvatarChange int

const (
	AvatarUnchanged AvatarChange = iota // same canonical config: no event
	AvatarCreated                       // first avatar: avatar_created_at set once, avatar.created
	AvatarUpdated                       // different config: avatar.updated
)

// SetAvatar stores a validated, normalized config with its hash. avatar_created_at is set only once (F070-AC-03).
func (p *Profile) SetAvatar(c AvatarConfig, hash string, now time.Time) AvatarChange {
	if p.AvatarCreatedAt == nil {
		t := now.UTC()
		p.Avatar, p.AvatarHash, p.AvatarCreatedAt = &c, &hash, &t
		return AvatarCreated
	}
	if p.AvatarHash != nil && *p.AvatarHash == hash {
		return AvatarUnchanged
	}
	p.Avatar, p.AvatarHash = &c, &hash
	return AvatarUpdated
}

// NicknameChange is the outcome of SetNickname.
type NicknameChange struct {
	Old     *string
	New     string
	Reason  string
	Changed bool
}

// SetNickname sets a nickname chosen from a suggestion: the first one is "initial", later ones "player_change"
// (the ones counted by nickname_change_monthly_limit). The same nickname again changes nothing.
func (p *Profile) SetNickname(n string) NicknameChange {
	if p.Nickname != nil && *p.Nickname == n {
		return NicknameChange{Old: p.Nickname, New: n}
	}
	c := NicknameChange{Old: p.Nickname, New: n, Reason: ReasonInitial, Changed: true}
	if p.Nickname != nil {
		c.Reason = ReasonPlayerChange
	}
	v := n
	p.Nickname = &v
	return c
}

// ResetNickname clears the nickname (reset by the school or the Safety Team, F136 US5): setup is required again and the
// next nickname is "initial", not counted by nickname_change_monthly_limit. It returns the old nickname; changed is false
// when there was none.
func (p *Profile) ResetNickname() (old *string, changed bool) {
	if p.Nickname == nil {
		return nil, false
	}
	old, p.Nickname = p.Nickname, nil
	return old, true
}

// Left returns the player changes left this month after changesThisMonth (never negative).
func (n NicknamePolicy) Left(changesThisMonth int) int {
	return max(n.MonthlyLimit-changesThisMonth, 0)
}

// NicknamePolicy is nickname_change_monthly_limit (CONFIG_DEFAULTS, CHARACTER_SYSTEM §5.3): changes by the player
// per calendar month in Location (a school policy may only lower it).
type NicknamePolicy struct {
	MonthlyLimit int
	Location     *time.Location
}

func (n NicknamePolicy) loc() *time.Location {
	if n.Location == nil {
		return time.UTC
	}
	return n.Location
}

// MonthStart is the start of the calendar month of now.
func (n NicknamePolicy) MonthStart(now time.Time) time.Time {
	t := now.In(n.loc())
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, n.loc())
}

// NextMonth is when a player whose limit is reached may change again (start of the next calendar month).
func (n NicknamePolicy) NextMonth(now time.Time) time.Time { return n.MonthStart(now).AddDate(0, 1, 0) }

// Allows reports whether one more player change is allowed after changes this month.
func (n NicknamePolicy) Allows(changesThisMonth int) bool { return changesThisMonth < n.MonthlyLimit }

// Effective is the stricter of the platform limit and a school limit (nil = none): a school may only tighten it.
func (n NicknamePolicy) Effective(school *int) NicknamePolicy {
	if school != nil && *school >= 0 && *school < n.MonthlyLimit {
		n.MonthlyLimit = *school
	}
	return n
}
