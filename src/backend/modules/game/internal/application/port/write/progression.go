package write

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Game content item types of the release core added by F074.
const (
	ItemProgressionCurve = "progression_curve"
	ItemRewardRule       = "reward_rule"
)

// Ledger reason codes (gamification.xp_ledger.reason_code, ADR-0049).
const (
	ReasonReward     = "reward"
	ReasonAdjustment = "adjustment"
)

// Reward grant trigger kinds (gamification.reward_grants.trigger_kind).
const (
	GrantEventRule = "event_rule"
	GrantLevelUp   = "level_up"
)

// XpEntry is one row of gamification.xp_ledger (append-only).
type XpEntry struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	StudentID        uuid.UUID
	SchoolID         *uuid.UUID
	Amount           int
	ReasonCode       string
	SourceType       string
	SourceCode       string
	SourceID         uuid.UUID
	ContentVersionID *uuid.UUID
	RuleID           *uuid.UUID
	RuleVersion      *int
	Capped           bool
	IdempotencyKey   string
	At               time.Time
}

// ProgressState is the progression cache of game.game_profiles (written in the ledger transaction).
type ProgressState struct {
	SchoolID     uuid.UUID
	Level        int
	XPTotal      int64
	CurveVersion *int
	TitleCode    *string
	GradeYear    *string
}

// LevelReached is one row of game.level_history.
type LevelReached struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	StudentID    uuid.UUID
	FromLevel    int
	ToLevel      int
	CurveID      uuid.UUID
	CurveVersion int
	TriggerEntry *uuid.UUID
	At           time.Time
}

// RewardGrant is one row of gamification.reward_grants (append-only, idempotent per key).
type RewardGrant struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	StudentID      uuid.UUID
	SchoolID       *uuid.UUID
	RuleVersionID  *uuid.UUID
	TriggerEventID uuid.UUID
	TriggerKind    string
	Level          *int
	CurveID        *uuid.UUID
	Grants         []map[string]any
	IdempotencyKey string
	At             time.Time
}

// RepeatKey identifies the postings that count as repeats of one content version for one source code.
type RepeatKey struct {
	ContentVersionID uuid.UUID
	SourceCode       string
	Since            time.Time
}

// ProfileRef identifies a game profile found by a platform job (curve rollout, reconciliation).
type ProfileRef struct {
	TenantID  uuid.UUID
	StudentID uuid.UUID
}

// ProgressionStore writes the XP ledger, the cap counters, the level history, the reward grants and the progression
// cache of a student inside the transaction of ctx (TenantScope; RLS + explicit tenant_id).
type ProgressionStore interface {
	// LockStudent serializes the progression writes of one student (advisory lock of the transaction).
	LockStudent(ctx context.Context, student uuid.UUID) error
	// Progress returns the progression cache of the live profile (row locked FOR UPDATE); nil without a profile.
	Progress(ctx context.Context, tenant, student uuid.UUID) (*ProgressState, error)
	// SaveProgress writes level, xp_total, curve_version and title_code of the live profile.
	SaveProgress(ctx context.Context, tenant, student uuid.UUID, p ProgressState) error
	// SetGradeYear writes grade_transition_year of the live profile.
	SetGradeYear(ctx context.Context, tenant, student uuid.UUID, year string) error
	// LockCaps creates the missing counters of buckets for day (YYYY-MM-DD, ICT) and returns them locked FOR UPDATE.
	LockCaps(ctx context.Context, tenant, student uuid.UUID, day string, buckets []string) (map[string]int, error)
	// AddCap adds n to the counter of bucket for day.
	AddCap(ctx context.Context, tenant, student uuid.UUID, day, bucket string, n int) error
	// Repeats counts the postings of a content version and source code since k.Since (window) and ever.
	Repeats(ctx context.Context, tenant, student uuid.UUID, k RepeatKey) (inWindow, ever int, err error)
	// InsertEntry appends a ledger row; false when the source key or the idempotency key exists (replay).
	InsertEntry(ctx context.Context, e XpEntry) (bool, error)
	// LedgerTotal is Σ amount of the ledger of a student.
	LedgerTotal(ctx context.Context, tenant, student uuid.UUID) (int64, error)
	// InsertLevel appends a level_history row.
	InsertLevel(ctx context.Context, l LevelReached) error
	// InsertGrant appends a reward grant; false when its idempotency key exists.
	InsertGrant(ctx context.Context, g RewardGrant) (bool, error)
}

// ProgressionScan lists profiles for the platform jobs (PlatformScope, every tenant; each is then handled in its tenant).
type ProgressionScan interface {
	// BehindCurve lists live profiles whose curve_version differs from version (curve-rollout).
	BehindCurve(ctx context.Context, version, limit int) ([]ProfileRef, error)
	// Mismatched lists live profiles whose xp_total differs from Σ ledger (xp-reconcile), after the cursor (tenant,
	// student).
	Mismatched(ctx context.Context, after *ProfileRef, limit int) ([]ProfileMismatch, error)
}

// ProfileMismatch is a profile whose cache differs from its ledger.
type ProfileMismatch struct {
	ProfileRef
	Cached int64
	Ledger int64
}

// GrantContext is the context of one reward grant given to a student.
type GrantContext struct {
	TenantID  uuid.UUID
	StudentID uuid.UUID
	GrantID   uuid.UUID
	At        time.Time
}

// GrantApplierRegistry applies the grants of a RewardGrant by type (F074: xp, title; F075 registers currency, item,
// badge, collection_item). Publishing content with a grant type it does not know is refused (F074-AC-23).
type GrantApplierRegistry interface {
	Has(grantType string) bool
	Apply(ctx context.Context, gc GrantContext, grants []map[string]any) error
}

// LevelRow is one generated row of game.progression_levels.
type LevelRow struct {
	Level      int
	XPRequired *int64
	Cumulative int64
	TitleCode  string
	Rewards    []map[string]any
}

// LevelTableStore writes the level table of a published curve version (PlatformScope, game_platform; GA rows).
type LevelTableStore interface {
	InsertLevels(ctx context.Context, curveVersionID uuid.UUID, levels []LevelRow) error
}
