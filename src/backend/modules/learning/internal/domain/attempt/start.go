package attempt

import (
	"time"

	"github.com/google/uuid"
)

// Start is what a new attempt pins (ACTIVITY_ENGINE §6.4): the source version and the item versions resolved at start,
// the seed of the presentation, the display language and the server start time.
type Start struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	SchoolID         uuid.UUID
	StudentID        uuid.UUID
	Mode             Mode
	ContextType      string
	ContextID        uuid.UUID
	ActivityID       *uuid.UUID
	ActivityType     string
	SourceKind       string
	SourceVersionID  uuid.UUID
	Seed             int64
	Now              time.Time
	TimeLimitSeconds *int
	// ActivityFeedback is feedback.timing of the activity config ("" = default of the mode).
	ActivityFeedback string
	DisplayLanguage  string
	RecommendationID *uuid.UUID
	IdempotencyKey   string
	Items            []Item
}

// New starts an attempt in_progress (F030-AC-01): deadline_at = start + time limit when the activity is timed;
// objective_ids is the union of the item objectives (snapshot).
func New(s Start) Attempt {
	a := Attempt{ID: s.ID, TenantID: s.TenantID, SchoolID: s.SchoolID, StudentID: s.StudentID, Mode: s.Mode, ContextType: s.ContextType,
		ContextID: s.ContextID, ActivityID: s.ActivityID, ActivityType: s.ActivityType, SourceKind: s.SourceKind,
		SourceVersionID: s.SourceVersionID, Status: StatusInProgress, Seed: s.Seed, StartedAt: s.Now, LastActivityAt: s.Now,
		Feedback: ResolveFeedback(s.Mode, s.ActivityFeedback), DisplayLanguage: s.DisplayLanguage, RecommendationID: s.RecommendationID,
		IdempotencyKey: s.IdempotencyKey, Items: s.Items}
	if a.DisplayLanguage != "en" {
		a.DisplayLanguage = "vi"
	}
	if s.TimeLimitSeconds != nil && *s.TimeLimitSeconds > 0 {
		d := s.Now.Add(time.Duration(*s.TimeLimitSeconds) * time.Second)
		a.DeadlineAt = &d
	}
	seen := map[uuid.UUID]bool{}
	for _, it := range s.Items {
		for _, o := range it.ObjectiveIDs {
			if !seen[o] && len(a.ObjectiveIDs) < 50 {
				seen[o] = true
				a.ObjectiveIDs = append(a.ObjectiveIDs, o)
			}
		}
	}
	return a
}
