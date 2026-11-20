package write

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/videolesson"
)

// BranchChoice is one row of assessment.video_branch_choices (one choice per branch point of an attempt, F065-AC-25).
type BranchChoice struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	SchoolID        uuid.UUID
	AttemptID       uuid.UUID
	BranchPointID   string
	OptionID        string
	TargetSegmentID string
	ChosenAt        time.Time
}

// IntegritySignal is one row of assessment.integrity_signals (ANTI_CHEAT §6: a signal, never an automatic penalty).
type IntegritySignal struct {
	ID         uuid.UUID
	TenantID   uuid.UUID
	SchoolID   uuid.UUID
	StudentID  uuid.UUID
	AttemptID  uuid.UUID
	SignalType string
	Score      float64
	Evidence   json.RawMessage
	CreatedAt  time.Time
}

// SignalVideoProgressImplausible is the signal of watched ranges growing faster than real time × the fastest rate.
const SignalVideoProgressImplausible = "video_progress_implausible"

// SignalAudioProgressImplausible is the same signal for an audio lesson (F066-AC-04).
const SignalAudioProgressImplausible = "audio_progress_implausible"

// MediaProgressStore persists the progress of the media attempts (video_lesson F065, audio_lesson F066) in the
// transaction of ctx (TenantScope; RLS T).
type MediaProgressStore interface {
	// Progress locks and returns the progress row of an attempt (nil when none).
	Progress(ctx context.Context, attemptID uuid.UUID) (*videolesson.Progress, error)
	// SaveProgress inserts or updates the progress row of an attempt.
	SaveProgress(ctx context.Context, p videolesson.Progress) error
	// BranchChoices returns the choices of an attempt in choice order.
	BranchChoices(ctx context.Context, attemptID uuid.UUID) ([]BranchChoice, error)
	// InsertBranchChoice records a choice; a second choice at the same branch point is problem CONFLICT.
	InsertBranchChoice(ctx context.Context, c BranchChoice) error
	// InsertIntegritySignal appends a signal.
	InsertIntegritySignal(ctx context.Context, s IntegritySignal) error
}
