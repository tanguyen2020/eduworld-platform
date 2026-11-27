// reconcile_<module>.go: one reconcile.Peer adapter per module facade (F134); the five files differ only by module name.

package wiring

import (
	"context"
	"fmt"
	"time"

	gameapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/reconcile"
)

// GameReconcileFacade is the part of the game facade the reconciliation reads.
type GameReconcileFacade interface {
	ListOutboxEventIds(ctx context.Context, req gameapi.ListOutboxEventIdsRequestObject) (gameapi.ListOutboxEventIdsResponseObject, error)
	ListProcessedEventIds(ctx context.Context, req gameapi.ListProcessedEventIdsRequestObject) (gameapi.ListProcessedEventIdsResponseObject, error)
}

// gamePeer implements reconcile.Peer over the game facade, through pkg/modcall with the restored module as caller.
type gamePeer struct {
	caller string
	f      GameReconcileFacade
}

func (p gamePeer) op(name string) modcall.Op {
	return modcall.Op{Caller: p.caller, Callee: "game", Operation: name, Kind: modcall.Read, Deadline: ReconcileCallDeadline,
		AllowedCallers: gameapi.AllowedCallers[name], ContextRequired: gameapi.ContextRequired[name]}
}

// ListOutboxEventIDs implements reconcile.Peer.
func (p gamePeer) ListOutboxEventIDs(ctx context.Context, from, to time.Time, types []string, cursor string, limit int) ([]reconcile.OutboxEvent, string, error) {
	params := gameapi.ListOutboxEventIdsParams{From: from, To: to, Limit: &limit}
	if types != nil {
		params.EventTypes = &types
	}
	if cursor != "" {
		params.Cursor = &cursor
	}
	resp, err := modcall.Call(ctx, p.op("listOutboxEventIds"), func(ctx context.Context) (gameapi.ListOutboxEventIdsResponseObject, error) {
		return p.f.ListOutboxEventIds(ctx, gameapi.ListOutboxEventIdsRequestObject{Params: params})
	})
	if err != nil {
		return nil, "", err
	}
	page, ok := resp.(gameapi.ListOutboxEventIds200JSONResponse)
	if !ok {
		return nil, "", fmt.Errorf("wiring: game listOutboxEventIds: unexpected %T", resp)
	}
	out := make([]reconcile.OutboxEvent, 0, len(page.Items))
	for _, it := range page.Items {
		out = append(out, reconcile.OutboxEvent{EventID: it.EventId, EventType: it.EventType, TenantID: it.TenantId, CreatedAt: it.CreatedAt,
			PublishedAt: it.PublishedAt})
	}
	return out, deref(page.NextCursor), nil
}

// ListProcessedEventIDs implements reconcile.Peer.
func (p gamePeer) ListProcessedEventIDs(ctx context.Context, source string, from, to time.Time, cursor string, limit int) ([]reconcile.Consumer, []reconcile.Processed, string, error) {
	params := gameapi.ListProcessedEventIdsParams{SourceModule: gameapi.CommonInternalCommonV1ReconcileModule(source), From: from, To: to, Limit: &limit}
	if cursor != "" {
		params.Cursor = &cursor
	}
	resp, err := modcall.Call(ctx, p.op("listProcessedEventIds"), func(ctx context.Context) (gameapi.ListProcessedEventIdsResponseObject, error) {
		return p.f.ListProcessedEventIds(ctx, gameapi.ListProcessedEventIdsRequestObject{Params: params})
	})
	if err != nil {
		return nil, nil, "", err
	}
	page, ok := resp.(gameapi.ListProcessedEventIds200JSONResponse)
	if !ok {
		return nil, nil, "", fmt.Errorf("wiring: game listProcessedEventIds: unexpected %T", resp)
	}
	consumers := make([]reconcile.Consumer, 0, len(page.Consumers))
	for _, c := range page.Consumers {
		consumers = append(consumers, reconcile.Consumer{Name: c.Consumer, EventTypes: c.EventTypes})
	}
	rows := make([]reconcile.Processed, 0, len(page.Items))
	for _, it := range page.Items {
		rows = append(rows, reconcile.Processed{Consumer: it.Consumer, EventID: it.EventId, At: it.ProcessedAt, DeadLettered: it.Status == "dead_lettered"})
	}
	return consumers, rows, deref(page.NextCursor), nil
}
