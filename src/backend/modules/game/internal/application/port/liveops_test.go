package port

import (
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/liveops"
)

// SchoolEvent.status (SchoolEventStatus, F081 edge case "Tạm dừng bởi EduWorld"): a suspended run stays suspended (the
// portal shows it and offers no edit/cancel); an archived run shows the end the school knows.
func TestSchoolStatus(t *testing.T) {
	at := time.Now()
	for _, c := range []struct {
		run  *EventRunRow
		want string
	}{
		{nil, "draft"},
		{&EventRunRow{Status: liveops.StatusScheduled}, "scheduled"},
		{&EventRunRow{Status: liveops.StatusLive}, "live"},
		{&EventRunRow{Status: liveops.StatusSuspended}, "suspended"},
		{&EventRunRow{Status: liveops.StatusEnded}, "ended"},
		{&EventRunRow{Status: liveops.StatusCancelled}, "cancelled"},
		{&EventRunRow{Status: liveops.StatusArchived}, "ended"},
		{&EventRunRow{Status: liveops.StatusArchived, CancelledAt: &at}, "cancelled"},
	} {
		if got := SchoolStatus(c.run); got != c.want {
			t.Errorf("%+v: %s, want %s", c.run, got, c.want)
		}
	}
}
