package query

import (
	"context"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// QuickCatalog lists the approved quick messages (service.QuickMessages).
type QuickCatalog interface {
	List(ctx context.Context) ([]read.QuickMessageRow, error)
}

// QuickMessages is listQuickMessages (F116 US4, F116-AC-27): the approved bilingual quick messages (vi and en, ADR-0056),
// the same list for every grade (R15) and every channel type; a client renders the text of the viewer's locale and
// sends only the code (no machine translation, NFR-I18N-004).
type QuickMessages struct {
	Catalog QuickCatalog
}

// List returns the quick messages for a student (the game_player guard already refused anyone else).
func (q QuickMessages) List(ctx context.Context, v Viewer, channelType string) (rows []read.QuickMessageRow, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.ListQuickMessages")
	defer func() { done(err) }()
	if v.TenantID == [16]byte{} || v.UserID == [16]byte{} {
		return nil, problem.New(problem.Unauthenticated, "")
	}
	if channelType != "" && !channel.Type(channelType).Valid() {
		return nil, problem.Invalid("channel_type", problem.SubInvalid, "unknown channel type")
	}
	return q.Catalog.List(ctx)
}
