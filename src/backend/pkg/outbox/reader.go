package outbox

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Reader reads the infra tables of one module database for the reconciliation (F134, listOutboxEventIds /
// listProcessedEventIds): ids and metadata only, never the envelope nor the DLQ payload (AGENTS R14). It runs on the
// <module>_platform pool, which may SELECT the reconciliation columns of infra.outbox_events (migration
// infra_reconcile_grants), infra.processed_events and infra.dead_letters; these infra tables have no tenant column
// filter (ADR-0089 §4, like the relay). Read only: no UPDATE/DELETE anywhere.
type Reader struct{ db *sql.DB }

// NewReader returns a Reader over the platform pool of the module.
func NewReader(db *sql.DB) *Reader { return &Reader{db: db} }

// OutboxRow is one outbox event (no envelope).
type OutboxRow struct {
	EventID     uuid.UUID
	EventType   string
	TenantID    *uuid.UUID
	CreatedAt   time.Time
	PublishedAt *time.Time
}

// partitionMargin widens the created_at bounds around the event-id window for partition pruning: created_at is the
// transaction start, the UUIDv7 is taken during the transaction.
const partitionMargin = time.Hour

// Outbox returns up to limit events with fromID ≤ event_id < toID and event_id > after, of the given types (nil =
// every type), ordered by event_id.
func (r *Reader) Outbox(ctx context.Context, from, to time.Time, fromID, toID uuid.UUID, types []string, after uuid.UUID, limit int) ([]OutboxRow, error) {
	var typ any
	if len(types) > 0 {
		typ = types
	}
	rows, err := r.db.QueryContext(ctx, `SELECT event_id, event_type, tenant_id, created_at, published_at
		FROM infra.outbox_events
		WHERE created_at >= $1 AND created_at < $2 AND event_id >= $3 AND event_id < $4 AND event_id > $5
		  AND ($6::text[] IS NULL OR event_type = ANY($6::text[]))
		ORDER BY event_id LIMIT $7`,
		from.Add(-partitionMargin), to.Add(partitionMargin), fromID, toID, after, pgTextArray(typ), limit)
	if err != nil {
		return nil, fmt.Errorf("outbox reader: outbox: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []OutboxRow
	for rows.Next() {
		var o OutboxRow
		var tenant uuid.NullUUID
		var pub sql.NullTime
		if err := rows.Scan(&o.EventID, &o.EventType, &tenant, &o.CreatedAt, &pub); err != nil {
			return nil, fmt.Errorf("outbox reader: outbox scan: %w", err)
		}
		if tenant.Valid {
			t := tenant.UUID
			o.TenantID = &t
		}
		if pub.Valid {
			p := pub.Time.UTC()
			o.PublishedAt = &p
		}
		o.CreatedAt = o.CreatedAt.UTC()
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("outbox reader: outbox rows: %w", err)
	}
	return out, nil
}

// ProcessedRow is one processed_events row (Status "processed") or the first dead_letters row of a (consumer,
// event) (Status "dead_lettered").
type ProcessedRow struct {
	Consumer string
	EventID  uuid.UUID
	At       time.Time
	Status   string
}

// ProcessedKey is the keyset position (consumer, event_id, status) of a page.
type ProcessedKey struct {
	Consumer string
	EventID  uuid.UUID
	Status   string
}

// Processed returns up to limit rows of the consumers with fromID ≤ event_id < toID after the key, ordered by
// (consumer, event_id, status).
func (r *Reader) Processed(ctx context.Context, from time.Time, consumers []string, fromID, toID uuid.UUID, after ProcessedKey, limit int) ([]ProcessedRow, error) {
	if len(consumers) == 0 {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `SELECT consumer, event_id, at, status FROM (
			SELECT consumer, event_id, processed_at AS at, 'processed'::text AS status
			  FROM infra.processed_events
			 WHERE consumer = ANY($1::text[]) AND event_id >= $2 AND event_id < $3
			UNION ALL
			SELECT consumer, event_id, min(created_at) AS at, 'dead_lettered'::text AS status
			  FROM infra.dead_letters
			 WHERE consumer = ANY($1::text[]) AND event_id >= $2 AND event_id < $3 AND created_at >= $4
			 GROUP BY consumer, event_id
		) x
		WHERE (consumer, event_id, status) > ($5::text, $6::uuid, $7::text)
		ORDER BY consumer, event_id, status LIMIT $8`,
		pgTextArray(consumers), fromID, toID, from.Add(-partitionMargin), after.Consumer, after.EventID, after.Status, limit)
	if err != nil {
		return nil, fmt.Errorf("outbox reader: processed: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []ProcessedRow
	for rows.Next() {
		var p ProcessedRow
		if err := rows.Scan(&p.Consumer, &p.EventID, &p.At, &p.Status); err != nil {
			return nil, fmt.Errorf("outbox reader: processed scan: %w", err)
		}
		p.At = p.At.UTC()
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("outbox reader: processed rows: %w", err)
	}
	return out, nil
}

// pgTextArray renders a []string as a PostgreSQL text[] literal (nil stays NULL); values are event types and consumer
// names (no quotes or backslashes by construction), quoted anyway.
func pgTextArray(v any) any {
	ss, ok := v.([]string)
	if !ok || ss == nil {
		return nil
	}
	b := []byte{'{'}
	for i, s := range ss {
		if i > 0 {
			b = append(b, ',')
		}
		b = append(b, '"')
		for _, c := range []byte(s) {
			if c == '"' || c == '\\' {
				b = append(b, '\\')
			}
			b = append(b, c)
		}
		b = append(b, '"')
	}
	return string(append(b, '}'))
}
