package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/progression"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/reward"
)

// StudentDirectory reads the projected student of a tenant (ref.students; nil when the tenant does not hold it).
type StudentDirectory interface {
	Student(ctx context.Context, tenant, student uuid.UUID) (*WorldStudent, error)
}

// RewardRules reads the platform reward rules (F074): for an event type, the version of every live rule in effect at
// the event time (the latest version whose effective_from = coalesce(active_from, published_at) ≤ at and not past
// active_to, F074-AC-24).
type RewardRules interface {
	RulesFor(ctx context.Context, eventType string, at time.Time) ([]reward.Rule, error)
}

// CurveRow is the current published version of the progression curve.
type CurveRow struct {
	CurveID   uuid.UUID
	VersionID uuid.UUID
	Version   int
	Curve     progression.Curve
}

// Table is the level table of the version.
func (r CurveRow) Table() progression.Table {
	t := r.Curve.Table(r.Version)
	t.CurveID = r.CurveID.String()
	return t
}

// Curves reads the platform progression curve (Release 1.0: one curve).
type Curves interface {
	// Current is the published version the pointer designates; nil when no curve is published.
	Current(ctx context.Context) (*CurveRow, error)
}

// ProgressionRealtime pushes the progression messages to every connection of a student (AsyncAPI
// ProgressionXpUpdated, ProgressionLevelReached) through Valkey ew:rt:user:<id>; best effort after commit.
type ProgressionRealtime interface {
	XPUpdated(ctx context.Context, student uuid.UUID, payload map[string]any, at time.Time) error
	LevelReached(ctx context.Context, student uuid.UUID, payload map[string]any, at time.Time) error
}
