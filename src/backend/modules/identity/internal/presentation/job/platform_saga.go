package job

import (
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
)

// PlatformSagaInterval is the tick of the job platform-saga (F045): due clone/merge saga runs are resumed and
// finished clone jobs closed [Proposed].
const PlatformSagaInterval = 10 * time.Second

// PlatformSaga is the job platform-saga of module identity (profile worker-jobs).
func PlatformSaga(s command.PlatformSagas) RunOnce { return s.RunOnce }
