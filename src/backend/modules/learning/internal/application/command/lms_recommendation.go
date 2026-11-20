package command

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/recommendation"
)

// SnapshotSource adds data of another feature to the snapshot of the rule engine (F040 R6, F041 R1, F042 R5), so their
// rules register without changing the engine nor the loader of F032 (F032-AC-33).
type SnapshotSource interface {
	Fill(ctx context.Context, studentID uuid.UUID, s *recommendation.Snapshot) error
}

// UpcomingWindow is how far ahead R6 looks in the LMS schedule (two weeks) [Proposed].
const UpcomingWindow = 14 * 24 * time.Hour

// LMSUpcoming is the SnapshotSource of F040: the lessons scheduled for the student's courses in the next two weeks.
type LMSUpcoming struct {
	Store write.LMSStore
}

// Fill implements SnapshotSource.
func (u LMSUpcoming) Fill(ctx context.Context, studentID uuid.UUID, s *recommendation.Snapshot) error {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		loc = time.UTC
	}
	from := s.Now.In(loc)
	items, err := u.Store.UpcomingLessons(ctx, studentID, from.Format(time.DateOnly), from.Add(UpcomingWindow).Format(time.DateOnly))
	if err != nil {
		return err
	}
	for _, it := range items {
		s.Upcoming = append(s.Upcoming, recommendation.Upcoming{ContentType: it.ContentType, ContentID: it.ContentID, SubjectID: it.SubjectID,
			Title: it.Title, ObjectiveIDs: it.ObjectiveIDs, Date: it.Date})
	}
	return nil
}
