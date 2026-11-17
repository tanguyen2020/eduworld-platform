package entx

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// ErrNoTransaction: data access outside UnitOfWork.Do / ReadSession.Read (F003-AC-06).
var ErrNoTransaction = errors.New("entx: no transaction in context (use UnitOfWork.Do or ReadSession.Read)")

// EventWriter writes events in the current transaction; implemented by outbox.Writer.
type EventWriter interface {
	Write(ctx context.Context, eventType, partitionKey string, payload any) error
}

// DBs are the pools of one module database: App (<module>_app, RLS), Platform (<module>_platform,
// BYPASSRLS) and Read (replica or primary; EW_DB_READ_URL_FILE [Proposed]).
type DBs struct {
	App      *sql.DB
	Platform *sql.DB
	Read     *sql.DB
}

// Options configure UnitOfWork.
type Options struct {
	Module string
	DBs    DBs
	// Events and Audit are used by SoftDelete inside the transaction (outbox writer and audit recorder).
	Events EventWriter
	Audit  audit.Recorder
}

// UnitOfWork is the write-context transaction (ADR-0087): one transaction on the primary with
// app.tenant_id set, shared through ctx by repositories, the outbox writer and idempotency.
type UnitOfWork struct{ o Options }

// NewUnitOfWork validates the options.
func NewUnitOfWork(o Options) (*UnitOfWork, error) {
	if o.Module == "" || o.DBs.App == nil || o.DBs.Platform == nil {
		return nil, errors.New("entx: unit of work needs a module and the app/platform pools")
	}
	return &UnitOfWork{o: o}, nil
}

type txState struct {
	tx           *sql.Tx
	readOnly     bool
	uow          *UnitOfWork
	beforeCommit []func(ctx context.Context, tx *sql.Tx) error
	afterCommit  []func(ctx context.Context)
}

type txKey struct{}

func stateFrom(ctx context.Context) (*txState, bool) {
	s, ok := ctx.Value(txKey{}).(*txState)
	return s, ok && s != nil
}

// Tx returns the SQL transaction of ctx (outbox, idempotency, explicit SQL with tenant + deleted_at).
func Tx(ctx context.Context) (*sql.Tx, bool) {
	s, ok := stateFrom(ctx)
	if !ok {
		return nil, false
	}
	return s.tx, true
}

// InWriteTx reports a UnitOfWork transaction in ctx.
func InWriteTx(ctx context.Context) bool {
	s, ok := stateFrom(ctx)
	return ok && !s.readOnly
}

// BeforeCommit runs fn inside the transaction just before commit (idempotency record); only in Do.
func BeforeCommit(ctx context.Context, fn func(ctx context.Context, tx *sql.Tx) error) error {
	s, ok := stateFrom(ctx)
	if !ok || s.readOnly {
		return ErrNoTransaction
	}
	s.beforeCommit = append(s.beforeCommit, fn)
	return nil
}

// AfterCommit runs fn after a successful commit (best effort: cache invalidation, never business writes).
func AfterCommit(ctx context.Context, fn func(ctx context.Context)) error {
	s, ok := stateFrom(ctx)
	if !ok {
		return ErrNoTransaction
	}
	s.afterCommit = append(s.afterCommit, fn)
	return nil
}

// Driver returns the ent driver bound to the transaction of ctx; without one every statement fails
// with ErrNoTransaction. Modules build their client per call: ent.NewClient(ent.Driver(entx.Driver(ctx))).
func Driver(ctx context.Context) dialect.Driver {
	if tx, ok := Tx(ctx); ok {
		return entsql.NewDriver(dialect.Postgres, entsql.Conn{ExecQuerier: tx})
	}
	return entsql.NewDriver(dialect.Postgres, entsql.Conn{ExecQuerier: noTx{}})
}

type noTx struct{}

func (noTx) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	return nil, ErrNoTransaction
}
func (noTx) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	return nil, ErrNoTransaction
}

// pool picks the role for the scope (BACKEND_DESIGN §5.1): TenantScope → <module>_app with
// app.tenant_id; CrossTenantScope, PlatformScope and Bypass → <module>_platform (BYPASSRLS, the
// interceptor keeps explicit tenant conditions). No scope → <module>_app without a tenant (global
// tables only; RLS returns nothing for tenant tables).
func pool(ctx context.Context, d DBs, read bool) (*sql.DB, string, tenancy.Scope) {
	app := d.App
	if read && d.Read != nil {
		app = d.Read
	}
	if _, bypassed := tenancy.Bypassed(ctx); bypassed {
		return d.Platform, "", nil
	}
	s, ok := tenancy.ScopeFrom(ctx)
	if !ok {
		return app, "", nil
	}
	if id, single := s.SessionTenantID(); single {
		return app, id.String(), s
	}
	return d.Platform, "", s
}

// Do runs fn in a write transaction (commit on nil, rollback on error or panic). A nested Do reuses
// the outer transaction.
func (u *UnitOfWork) Do(ctx context.Context, fn func(ctx context.Context) error) (err error) {
	if s, ok := stateFrom(ctx); ok {
		if s.readOnly {
			return errors.New("entx: UnitOfWork.Do inside a ReadSession")
		}
		return fn(ctx)
	}
	return u.run(tenancy.WithModule(ctx, u.o.Module), u.o.DBs, false, fn)
}

// InTenantTx is Do with an explicit scope (consumers, jobs).
func (u *UnitOfWork) InTenantTx(ctx context.Context, s tenancy.Scope, fn func(ctx context.Context) error) error {
	return u.Do(tenancy.WithScope(ctx, s), fn)
}

func (u *UnitOfWork) run(ctx context.Context, d DBs, readOnly bool, fn func(ctx context.Context) error) (err error) {
	db, tenant, scope := pool(ctx, d, readOnly)
	if cs, ok := scope.(tenancy.CrossTenantScope); ok {
		mode := tenancy.ModeWrite
		if readOnly {
			mode = tenancy.ModeRead
		}
		if err := tenancy.RecordCrossTenantScope(ctx, cs, mode); err != nil {
			return err
		}
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: readOnly})
	if err != nil {
		return fmt.Errorf("entx: begin: %w", err)
	}
	st := &txState{tx: tx, readOnly: readOnly, uow: u}
	ctx = context.WithValue(ctx, txKey{}, st)
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if tenant != "" {
		if _, err = tx.ExecContext(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenant); err != nil {
			return fmt.Errorf("entx: set tenant: %w", err)
		}
	}
	if err = fn(ctx); err != nil {
		return err
	}
	for _, b := range st.beforeCommit {
		if err = b(ctx, tx); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("entx: commit: %w", err)
	}
	for _, a := range st.afterCommit {
		a(ctx)
	}
	return nil
}

// ReadSession is the read-context transaction (ADR-0087): READ ONLY, read pool when configured.
type ReadSession struct {
	module string
	dbs    DBs
}

// NewReadSession builds a read session; Platform is needed for cross-tenant/platform reads.
func NewReadSession(module string, d DBs) (*ReadSession, error) {
	if module == "" || (d.App == nil && d.Read == nil) || d.Platform == nil {
		return nil, errors.New("entx: read session needs a module, an app or read pool and the platform pool")
	}
	if d.App == nil {
		d.App = d.Read
	}
	return &ReadSession{module: module, dbs: d}, nil
}

// Read runs fn in a READ ONLY transaction with the scope of ctx; any write fails.
func (r *ReadSession) Read(ctx context.Context, fn func(ctx context.Context) error) error {
	if s, ok := stateFrom(ctx); ok && s.readOnly {
		return fn(ctx)
	}
	u := &UnitOfWork{o: Options{Module: r.module, DBs: r.dbs}}
	return u.run(tenancy.WithModule(ctx, r.module), r.dbs, true, fn)
}

// InTenantRead is Read with an explicit scope.
func (r *ReadSession) InTenantRead(ctx context.Context, s tenancy.Scope, fn func(ctx context.Context) error) error {
	return r.Read(tenancy.WithScope(ctx, s), fn)
}
