package facade

import (
	"context"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/reconcile"
)

// ListOutboxEventIds implements api.StrictServerInterface (x-allowed-callers: every module with a database, F134-AC-02):
// ids and metadata of infra.outbox_events of this module for `worker reconcile`, never the envelope.
func (f Facade) ListOutboxEventIds(ctx context.Context, req api.ListOutboxEventIdsRequestObject) (api.ListOutboxEventIdsResponseObject, error) {
	if f.Ops == nil {
		return nil, problem.New(problem.Unavailable, "reconciliation not loaded in this binary").Retry()
	}
	p := req.Params
	q := reconcile.OutboxQuery{From: time.Time(p.From), To: time.Time(p.To)}
	if p.EventTypes != nil {
		q.EventTypes = *p.EventTypes
	}
	if p.Cursor != nil {
		q.Cursor = *p.Cursor
	}
	if p.Limit != nil {
		q.Limit = *p.Limit
	}
	events, next, err := f.Ops.ListOutbox(ctx, q)
	if err != nil {
		return nil, err
	}
	out := api.ListOutboxEventIds200JSONResponse{Items: make([]api.CommonInternalCommonV1OutboxEventId, 0, len(events))}
	for _, e := range events {
		out.Items = append(out.Items, api.CommonInternalCommonV1OutboxEventId{EventId: e.EventID, EventType: e.EventType, TenantId: e.TenantID,
			CreatedAt: e.CreatedAt, PublishedAt: e.PublishedAt})
	}
	if next != "" {
		out.NextCursor = &next
	}
	return out, nil
}

// ListProcessedEventIds implements api.StrictServerInterface (x-allowed-callers: every module with a database,
// F134-AC-01): the consumers of this module for source_module and their processed / dead-lettered event ids.
func (f Facade) ListProcessedEventIds(ctx context.Context, req api.ListProcessedEventIdsRequestObject) (api.ListProcessedEventIdsResponseObject, error) {
	if f.Ops == nil {
		return nil, problem.New(problem.Unavailable, "reconciliation not loaded in this binary").Retry()
	}
	p := req.Params
	q := reconcile.ProcessedQuery{SourceModule: string(p.SourceModule), From: time.Time(p.From), To: time.Time(p.To)}
	if p.Cursor != nil {
		q.Cursor = *p.Cursor
	}
	if p.Limit != nil {
		q.Limit = *p.Limit
	}
	consumers, rows, next, err := f.Ops.ListProcessed(ctx, q)
	if err != nil {
		return nil, err
	}
	out := api.ListProcessedEventIds200JSONResponse{Consumers: make([]api.CommonInternalCommonV1ReconcileConsumer, 0, len(consumers)),
		Items: make([]api.CommonInternalCommonV1ProcessedEventId, 0, len(rows))}
	for _, c := range consumers {
		out.Consumers = append(out.Consumers, api.CommonInternalCommonV1ReconcileConsumer{Consumer: c.Name, EventTypes: c.EventTypes})
	}
	for _, r := range rows {
		st := api.CommonInternalCommonV1ProcessedEventIdStatus("processed")
		if r.DeadLettered {
			st = "dead_lettered"
		}
		out.Items = append(out.Items, api.CommonInternalCommonV1ProcessedEventId{Consumer: r.Consumer, EventId: r.EventID, ProcessedAt: r.At, Status: st})
	}
	if next != "" {
		out.NextCursor = &next
	}
	return out, nil
}
