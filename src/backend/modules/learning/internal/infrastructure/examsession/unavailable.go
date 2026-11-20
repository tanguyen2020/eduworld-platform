// Package examsession holds the adapters of port.ExamSessionCreator.
package examsession

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
)

// Unavailable is the default adapter of port.ExamSessionCreator until F042 (not a mock: a real adapter answering a
// typed error, F041 Assumptions): graded assignments on a blueprint are refused at creation (F041-AC-09).
type Unavailable struct{}

var _ port.ExamSessionCreator = Unavailable{}

// Available implements port.ExamSessionCreator.
func (Unavailable) Available() bool { return false }

// Create implements port.ExamSessionCreator.
func (Unavailable) Create(context.Context, port.ExamSessionRequest) (uuid.UUID, error) {
	return uuid.Nil, port.ErrExamSessionUnavailable
}

// Cancel implements port.ExamSessionCreator.
func (Unavailable) Cancel(context.Context, uuid.UUID, string) error {
	return port.ErrExamSessionUnavailable
}
