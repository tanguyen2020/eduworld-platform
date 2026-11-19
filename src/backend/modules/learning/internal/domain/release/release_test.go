package release

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

// F026-AC-01, AC-02, AC-06, AC-08…AC-11, AC-17, AC-19: the release rules.
func TestRules(t *testing.T) {
	item, other := uuid.New(), uuid.New()
	v4 := Version{ID: uuid.New(), ItemID: item, Status: contentmodel.StatusPublished}
	v5 := Version{ID: uuid.New(), ItemID: item, Status: contentmodel.StatusPublished}
	if CheckPublish(Version{Status: contentmodel.StatusApproved}) != nil || !errors.Is(CheckPublish(Version{Status: contentmodel.StatusDraft}), ErrNotApproved) {
		t.Fatal("publish")
	}
	cur := v5.ID
	if err := CheckRollback(item, &cur, v4, "lỗi đề"); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	for name, c := range map[string]struct {
		target Version
		reason string
		want   error
	}{
		"current":   {v5, "x", ErrRollbackTarget},
		"archived":  {Version{ID: uuid.New(), ItemID: item, Status: contentmodel.StatusArchived}, "x", ErrRollbackTarget},
		"approved":  {Version{ID: uuid.New(), ItemID: item, Status: contentmodel.StatusApproved}, "x", ErrRollbackTarget},
		"other":     {Version{ID: uuid.New(), ItemID: other, Status: contentmodel.StatusPublished}, "x", ErrRollbackTarget},
		"no reason": {v4, " ", ErrReasonRequired},
		"long":      {v4, strings.Repeat("a", 501), ErrReasonTooLong},
	} {
		if err := CheckRollback(item, &cur, c.target, c.reason); !errors.Is(err, c.want) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if CheckUnpublish(&cur, "gỡ") != nil || !errors.Is(CheckUnpublish(nil, "gỡ"), ErrNotPublished) || !errors.Is(CheckUnpublish(&cur, ""), ErrReasonRequired) {
		t.Fatal("unpublish")
	}
	now := time.Date(2026, 11, 19, 8, 0, 0, 0, time.UTC)
	if CheckSchedule(now.Add(6*time.Minute), now, 5*time.Minute) != nil || !errors.Is(CheckSchedule(now.Add(time.Minute), now, 5*time.Minute), ErrScheduleTooSoon) {
		t.Fatal("schedule lead")
	}
	if s, err := Cancel(ScheduleScheduled); err != nil || s != ScheduleCancelled {
		t.Fatal("cancel")
	}
	if _, err := Cancel(ScheduleApplied); !errors.Is(err, ErrNotScheduled) {
		t.Fatal("cancel applied")
	}
	if !Due(ScheduleScheduled, now, now) || Due(ScheduleScheduled, now.Add(time.Second), now) || Due(ScheduleCancelled, now, now) {
		t.Fatal("due")
	}
}
