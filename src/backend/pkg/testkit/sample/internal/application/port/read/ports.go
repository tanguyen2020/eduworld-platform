// Package read holds the read-context ports of the sample module (ADR-0087).
package read

import (
	"context"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/sample/internal/application/dto"
)

// WidgetReadModel reads widget views (read-only transaction).
type WidgetReadModel interface {
	List(ctx context.Context) ([]dto.WidgetView, error)
}
