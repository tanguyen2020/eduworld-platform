// reconcile_<module>.go: one reconcile.Peer adapter per module facade (F134); the five files differ only by module name.

package wiring

import (
	"context"
	"fmt"
	"time"

	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/reconcile"
)

// LearningReconcileFacade is the part of the learning facade the reconciliation reads.
type LearningReconcileFacade interface {
	ListOutboxEventIds(ctx context.Context, req learningapi.ListOutboxEventIdsRequestObject) (learningapi.ListOutboxEventIdsResponseObject, error)
	ListProcessedEventIds(ctx context.Context, req learningapi.ListProcessedEventIdsRequestObject) (learningapi.ListProcessedEventIdsResponseObject, error)
}

// learningPeer implements reconcile.Peer over the learning facade, through pkg/modcall with the restored module as caller.
type learningPeer struct {
	caller string
	f      LearningReconcileFacade
}

func (p learningPeer) op(name string) modcall.Op {
	return modcall.Op{Caller: p.caller, Callee: "learning", Operation: name, Kind: modcall.Read, Deadline: ReconcileCallDeadline,
		AllowedCallers: learningapi.AllowedCallers[name], ContextRequired: learningapi.ContextRequired[name]}
}

// ListOutboxEventIDs implements reconcile.Peer.
func (p learningPeer) ListOutboxEventIDs(ctx context.Context, from, to time.Time, types []string, cursor string, limit int) ([]reconcile.OutboxEvent, string, error) {
	params := learningapi.ListOutboxEventIdsParams{From: from, To: to, Limit: &limit}
	if types != nil {
		params.EventTypes = &types
	}
	if cursor != "" {
		params.Cursor = &cursor
	}
	resp, err := modcall.Call(ctx, p.op("listOutboxEventIds"), func(ctx context.Context) (learningapi.ListOutboxEventIdsResponseObject, error) {
		return p.f.ListOutboxEventIds(ctx, learningapi.ListOutboxEventIdsRequestObject{Params: params})
	})
	if err != nil {
		return nil, "", err
	}
	page, ok := resp.(learningapi.ListOutboxEventIds200JSONResponse)
	if !ok {
		return nil, "", fmt.Errorf("wiring: learning listOutboxEventIds: unexpected %T", resp)
	}
	out := make([]reconcile.OutboxEvent, 0, len(page.Items))
	for _, it := range page.Items {
		out = append(out, reconcile.OutboxEvent{EventID: it.EventId, EventType: it.EventType, TenantID: it.TenantId, CreatedAt: it.CreatedAt,
			PublishedAt: it.PublishedAt})
	}
	return out, deref(page.NextCursor), nil
}

// ListProcessedEventIDs implements reconcile.Peer.
func (p learningPeer) ListProcessedEventIDs(ctx context.Context, source string, from, to time.Time, cursor string, limit int) ([]reconcile.Consumer, []reconcile.Processed, string, error) {
	params := learningapi.ListProcessedEventIdsParams{SourceModule: learningapi.CommonInternalCommonV1ReconcileModule(source), From: from, To: to, Limit: &limit}
	if cursor != "" {
		params.Cursor = &cursor
	}
	resp, err := modcall.Call(ctx, p.op("listProcessedEventIds"), func(ctx context.Context) (learningapi.ListProcessedEventIdsResponseObject, error) {
		return p.f.ListProcessedEventIds(ctx, learningapi.ListProcessedEventIdsRequestObject{Params: params})
	})
	if err != nil {
		return nil, nil, "", err
	}
	page, ok := resp.(learningapi.ListProcessedEventIds200JSONResponse)
	if !ok {
		return nil, nil, "", fmt.Errorf("wiring: learning listProcessedEventIds: unexpected %T", resp)
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
