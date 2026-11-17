// Package idempotency stores the results of idempotent operations in infra.idempotency_records of the
// module database (API_DESIGN §6, ADR-0089 §4, F003 US3): the record is INSERTed once, inside the
// UnitOfWork transaction of the business change, with the serialized response; a rollback leaves no
// record; a concurrent duplicate waits on the primary key and then replays.
package idempotency

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/clock"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// DefaultTTL is idempotency_record_ttl (CONFIG_DEFAULTS).
const DefaultTTL = 24 * time.Hour

var (
	// ErrDuplicate: another transaction committed the same key first (replay it).
	ErrDuplicate = errors.New("idempotency: duplicate key committed concurrently")
	// ErrInProgress: the concurrent transaction did not finish within lock_timeout.
	ErrInProgress = errors.New("idempotency: concurrent request still in progress")
)

// ParseKey validates Idempotency-Key: a UUIDv7 whose timestamp is within ttl of now (F003-AC-37). The
// day of that timestamp (UTC) is key_issued_on, so a retry after midnight hits the same primary key.
func ParseKey(raw string, now time.Time, ttl time.Duration) (uuid.UUID, time.Time, error) {
	k, err := ids.ParseV7(raw)
	if err != nil {
		return uuid.Nil, time.Time{}, problem.New(problem.BadRequest, "Idempotency-Key must be a UUIDv7")
	}
	at, _ := ids.Time(k)
	if d := now.Sub(at); d > ttl || d < -ttl {
		return uuid.Nil, time.Time{}, problem.New(problem.BadRequest, "Idempotency-Key timestamp is too far from the server time")
	}
	y, m, d := at.UTC().Date()
	return k, time.Date(y, m, d, 0, 0, 0, 0, time.UTC), nil
}

// Hash is SHA-256 of the canonical JSON of body (key order and whitespace do not matter); a non-JSON
// body is hashed as is.
func Hash(body []byte) [32]byte {
	var v any
	if len(body) > 0 && json.Unmarshal(body, &v) == nil {
		if canon, err := json.Marshal(v); err == nil {
			return sha256.Sum256(canon)
		}
	}
	return sha256.Sum256(body)
}

// Key identifies a record.
type Key struct {
	UserID   uuid.UUID
	Method   string
	Route    string // route template, never the concrete path
	Key      uuid.UUID
	IssuedOn time.Time
}

// Record is a stored response.
type Record struct {
	Hash   [32]byte
	Status int
	Body   []byte
}

// Store reads records with the <module>_app pool (outside the business transaction).
type Store struct {
	db    *sql.DB
	ttl   time.Duration
	clock clock.Clock
}

// NewStore builds a store; ttl 0 means DefaultTTL.
func NewStore(db *sql.DB, ttl time.Duration, clk clock.Clock) *Store {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	if clk == nil {
		clk = clock.System()
	}
	return &Store{db: db, ttl: ttl, clock: clk}
}

// TTL returns idempotency_record_ttl.
func (s *Store) TTL() time.Duration { return s.ttl }

// Clock returns the store clock.
func (s *Store) Clock() clock.Clock { return s.clock }

// Lookup returns the unexpired record of k (expired records are ignored, never deleted — F003-AC-15).
func (s *Store) Lookup(ctx context.Context, k Key) (*Record, error) {
	var r Record
	var hash []byte
	var body []byte
	err := s.db.QueryRowContext(ctx, `SELECT request_hash, response_status, response_body FROM infra.idempotency_records
		WHERE user_id = $1 AND route_template = $2 AND idem_key = $3 AND key_issued_on = $4 AND expires_at > $5`,
		k.UserID, k.Route, k.Key, k.IssuedOn, s.clock.Now()).Scan(&hash, &r.Status, &body)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("idempotency: lookup: %w", err)
	}
	copy(r.Hash[:], hash)
	r.Body = body
	return &r, nil
}

// Pending is set in the context by the middleware for a request whose key has no record yet.
type Pending struct {
	Key           Key
	Hash          [32]byte
	SuccessStatus int
	store         *Store
	recorded      bool
}

type pendingKey struct{}

// WithPending stores p in ctx.
func WithPending(ctx context.Context, p *Pending) context.Context {
	return context.WithValue(ctx, pendingKey{}, p)
}

// PendingFrom returns the pending idempotent request of ctx.
func PendingFrom(ctx context.Context) (*Pending, bool) {
	p, ok := ctx.Value(pendingKey{}).(*Pending)
	return p, ok && p != nil
}

// NewPending builds the pending record of a request.
func (s *Store) NewPending(k Key, hash [32]byte, successStatus int) *Pending {
	return &Pending{Key: k, Hash: hash, SuccessStatus: successStatus, store: s}
}

// Recorded reports whether SetResult registered the record in the transaction.
func (p *Pending) Recorded() bool { return p.recorded }

// SetResult must be called by the command inside UnitOfWork.Do with the response body of the operation
// (the JSON the presentation writes with SuccessStatus). The record is INSERTed just before commit in the
// same transaction; without an idempotent request in ctx it does nothing.
func SetResult(ctx context.Context, result any) error {
	p, ok := PendingFrom(ctx)
	if !ok {
		return nil
	}
	body, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("idempotency: response: %w", err)
	}
	if len(body) > 65536 {
		return fmt.Errorf("idempotency: response larger than 64 KiB")
	}
	if err := entx.BeforeCommit(ctx, func(ctx context.Context, tx *sql.Tx) error { return p.insert(ctx, tx, body) }); err != nil {
		return err
	}
	p.recorded = true
	return nil
}

func (p *Pending) insert(ctx context.Context, tx *sql.Tx, body []byte) error {
	if _, err := tx.ExecContext(ctx, `SET LOCAL lock_timeout = '5s'`); err != nil {
		return err
	}
	now := p.store.clock.Now()
	var jsonBody any
	if len(body) > 0 && string(body) != "null" {
		jsonBody = body
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO infra.idempotency_records
		(user_id, method, route_template, idem_key, key_issued_on, request_hash, response_status, response_body, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		p.Key.UserID, p.Key.Method, p.Key.Route, p.Key.Key, p.Key.IssuedOn, p.Hash[:], p.SuccessStatus, jsonBody, now, now.Add(p.store.ttl))
	var pgErr *pgconn.PgError
	switch {
	case errors.As(err, &pgErr) && pgErr.Code == "23505":
		return problem.New(problem.Conflict, "idempotent request committed concurrently").Wrap(ErrDuplicate)
	case errors.As(err, &pgErr) && pgErr.Code == "55P03":
		return problem.New(problem.IdempotencyInProgress, "").Wrap(ErrInProgress)
	case err != nil:
		return fmt.Errorf("idempotency: insert: %w", err)
	}
	return nil
}
