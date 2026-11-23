package read

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ProgressionProfile is the progression cache of a student (game.game_profiles).
type ProgressionProfile struct {
	Level        int
	XPTotal      int64
	CurveVersion *int
}

// XPHistoryRow is one ledger posting as the student sees it: no source_id, rule or key (F074-AC-18).
type XPHistoryRow struct {
	ID         uuid.UUID
	Amount     int
	SourceType string
	SourceCode string
	Capped     bool
	CreatedAt  time.Time
}

// XPHistoryCursor is the position after the last returned posting (created_at DESC, xp_entry_id DESC).
type XPHistoryCursor struct {
	At time.Time
	ID uuid.UUID
}

// ProgressionLevel is one row of game.progression_levels.
type ProgressionLevel struct {
	Level      int
	XPRequired *int64
	Cumulative int64
	TitleCode  string
	Rewards    []map[string]any
}

// ProgressionReadModel reads the progression of a student and the level table (READ ONLY session, explicit tenant).
type ProgressionReadModel interface {
	Profile(ctx context.Context, tenant, student uuid.UUID) (*ProgressionProfile, error)
	// DailyGranted is the XP granted on day (YYYY-MM-DD, ICT) in bucket.
	DailyGranted(ctx context.Context, tenant, student uuid.UUID, day, bucket string) (int, error)
	// XPHistory lists postings newest first after the cursor (limit+1 rows to detect more).
	XPHistory(ctx context.Context, tenant, student uuid.UUID, after *XPHistoryCursor, limit int) ([]XPHistoryRow, error)
	// Levels is the level table of a curve version (by level).
	Levels(ctx context.Context, curveVersionID uuid.UUID) ([]ProgressionLevel, error)
}
